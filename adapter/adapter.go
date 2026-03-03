package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/finddiff/RuleBaseProxy/common/queue"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"

	"go.uber.org/atomic"
)

var (
	ErrRateLimit = errors.New("rate limit exceeded due to continuous failures")
)

type Proxy struct {
	C.ProxyAdapter
	history *queue.Queue
	alive   *atomic.Bool

	// 新增字段
	lastSuccessTime *atomic.Uint64 // 存储 UnixNano 格式的时间戳
	sem             chan struct{}  // 用于限流的信号量（容量为3）
}

// Alive implements C.Proxy
func (p *Proxy) Alive() bool {
	return p.alive.Load()
}

// Dial implements C.Proxy
func (p *Proxy) Dial(metadata *C.Metadata) (C.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), C.DefaultTCPTimeout)
	defer cancel()
	return p.DialContext(ctx, metadata)
}

// DialContext implements C.ProxyAdapter
func (p *Proxy) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	now := time.Now()
	lastSuccess := time.Unix(0, int64(p.lastSuccessTime.Load()))

	// 规则：如果最近5秒内没有成功记录
	isFailingStrictly := now.Sub(lastSuccess) > C.DefaultTCPTimeout

	if isFailingStrictly {
		// 尝试非阻塞地获取令牌
		select {
		case p.sem <- struct{}{}:
			// 拿到令牌，记得释放
			defer func() { <-p.sem }()
		default:
			// 令牌已满，直接返回限流错误
			return nil, ErrRateLimit
		}
	}

	conn, err := p.ProxyAdapter.DialContext(ctx, metadata)
	if err != nil {
		p.alive.Store(false)
	} else {
		p.alive.Store(true)
		// 拨号成功，更新最后成功时间
		p.lastSuccessTime.Store(uint64(time.Now().UnixNano()))
	}
	return conn, err
}

// DelayHistory implements C.Proxy
func (p *Proxy) DelayHistory() []C.DelayHistory {
	queue := p.history.Copy()
	histories := []C.DelayHistory{}
	for _, item := range queue {
		histories = append(histories, item.(C.DelayHistory))
	}
	return histories
}

// LastDelay return last history record. if proxy is not alive, return the max value of uint16.
// implements C.Proxy
func (p *Proxy) LastDelay() (delay uint16) {
	var max uint16 = 0xffff
	if !p.alive.Load() {
		return max
	}

	last := p.history.Last()
	if last == nil {
		return max
	}
	history := last.(C.DelayHistory)
	if history.Delay == 0 {
		return max
	}
	return history.Delay
}

// MarshalJSON implements C.ProxyAdapter
func (p *Proxy) MarshalJSON() ([]byte, error) {
	inner, err := p.ProxyAdapter.MarshalJSON()
	if err != nil {
		return inner, err
	}

	mapping := map[string]interface{}{}
	json.Unmarshal(inner, &mapping)
	mapping["history"] = p.DelayHistory()
	mapping["name"] = p.Name()
	mapping["udp"] = p.SupportUDP()
	return json.Marshal(mapping)
}

// URLTest get the delay for the specified URL
// implements C.Proxy
// URLTest 改进版：支持预热以消除首次连接复用的延迟波动
func (p *Proxy) URLTest(ctx context.Context, url string) (t uint16, err error) {
	defer func() {
		p.alive.Store(err == nil)
		record := C.DelayHistory{Time: time.Now()}
		if err == nil {
			record.Delay = t
		}
		p.history.Put(record)
		if p.history.Len() > 10 {
			p.history.Pop()
		}
	}()

	// 1. 预解析 metadata，确保 DialContext 知道要去哪里
	addr, err := urlToMetadata(url)
	if err != nil {
		return 0, err
	}

	// 2. 建立长连接隧道
	// 注意：这是通过代理服务器建立的原始 TCP/加密隧道
	//instance, err := p.DialContext(ctx, &addr)
	//if err != nil {
	//	return 0, err
	//}
	//defer instance.Close()

	// 3. 构建 Transport：关键在于只给这一个 instance
	// 我们使用单连接池模式，并允许 KeepAlive
	transport := &http.Transport{
		// 关键：将拨号逻辑直接绑定到代理对象的 DialContext
		DialContext: func(c context.Context, network, address string) (net.Conn, error) {
			// 这里强制使用我们解析好的 addr，确保它走代理节点
			return p.DialContext(c, &addr)
		},
		DisableKeepAlives: false, // 必须开启以复用隧道
		MaxIdleConns:      1,
		IdleConnTimeout:   10 * time.Second,
	}
	client := http.Client{Transport: transport, Timeout: 5 * time.Second}
	defer client.CloseIdleConnections()

	var totalDelay int64
	var validCount int

	for i := 0; i < 3; i++ {
		// 在循环内定义局部变量，避免闭包竞争
		var start, firstByte time.Time
		var isReused bool // 核心：增加复用标识

		trace := &httptrace.ClientTrace{
			GotConn: func(info httptrace.GotConnInfo) {
				isReused = info.Reused // 捕获连接是否来自连接池
			},
			WroteRequest:         func(_ httptrace.WroteRequestInfo) { start = time.Now() },
			GotFirstResponseByte: func() { firstByte = time.Now() },
		}

		req, _ := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
		req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))

		// 强制设置 Connection: keep-alive 确保不关闭底层 instance
		req.Header.Set("Connection", "Keep-Alive")

		resp, err := client.Do(req)
		if err != nil || !isReused {
			log.Debugln("URLTest proxy:%s, Iteration %d failed: %v, isReused: %v", p.Name(), i, err, isReused)
			continue
		}
		// 必须完全读取 Body 才能复用连接
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// 逻辑控制：i=0 往往包含代理隧道的首次协议处理开销，跳过它取纯净延迟
		if i > 0 && !start.IsZero() && !firstByte.IsZero() {
			delay := firstByte.Sub(start).Milliseconds()
			if delay > 0 {
				totalDelay += delay
				validCount++
			}
		}
	}
	log.Debugln("URLTest proxy:%s, url:%s, validCount: %d, totalDelay: %dms", p.Name(), url, validCount, totalDelay)

	if validCount == 0 {
		return 0, fmt.Errorf("all probes failed or invalid")
	}

	t = uint16(totalDelay / int64(validCount))
	return t, nil
}

func (p *Proxy) URLTestOrg(ctx context.Context, url string) (t uint16, err error) {
	defer func() {
		p.alive.Store(err == nil)
		record := C.DelayHistory{Time: time.Now()}
		if err == nil {
			record.Delay = t
		}
		p.history.Put(record)
		if p.history.Len() > 10 {
			p.history.Pop()
		}
	}()

	addr, err := urlToMetadata(url)
	if err != nil {
		return
	}

	start := time.Now()
	instance, err := p.DialContext(ctx, &addr)
	if err != nil {
		return
	}
	defer instance.Close()

	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return
	}
	req = req.WithContext(ctx)

	transport := &http.Transport{
		Dial: func(string, string) (net.Conn, error) {
			return instance, nil
		},
		// from http.DefaultTransport
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	client := http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer client.CloseIdleConnections()

	resp, err := client.Do(req)
	if err != nil {
		return
	}
	resp.Body.Close()
	t = uint16(time.Since(start) / time.Millisecond)
	log.Debugln("URLTest proxy:%s, url:%s, delay: %d", p.Name(), url, t)
	return
}

func NewProxy(adapter C.ProxyAdapter) *Proxy {
	return &Proxy{
		adapter,
		queue.New(10),
		atomic.NewBool(true),
		atomic.NewUint64(uint64(time.Now().UnixNano())),
		make(chan struct{}, 3), // 初始化信号量，容量为3
	}
}

func urlToMetadata(rawURL string) (addr C.Metadata, err error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return
	}

	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		default:
			err = fmt.Errorf("%s scheme not Support", rawURL)
			return
		}
	}

	addr = C.Metadata{
		AddrType: C.AtypDomainName,
		Host:     u.Hostname(),
		DstIP:    nil,
		DstPort:  port,
	}
	return
}
