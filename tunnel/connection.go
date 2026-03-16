package tunnel

import (
	"errors"
	"github.com/finddiff/RuleBaseProxy/component/resolver"
	"github.com/finddiff/RuleBaseProxy/log"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	N "github.com/finddiff/RuleBaseProxy/common/net"
	"github.com/finddiff/RuleBaseProxy/common/pool"
	C "github.com/finddiff/RuleBaseProxy/constant"
)

var (
	currentConns int64
	maxConns     int64 = 5000 // 熔断阈值
	// 复用缓冲区，减少 GC 压力
	// 128KB 是针对高带宽视频流优化的尺寸
	bufPool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 128*1024)
		},
	}
	zeroCopy bool = true
)

func GetMaxConns() int64 {
	return maxConns
}

func SetMaxConns(max int64) {
	maxConns = max
}

func GetCurrentConns() int64 {
	return currentConns
}

func SetZeroCopy(enable bool) {
	zeroCopy = enable
}

func GetZeroCopy() bool {
	return zeroCopy
}

func handleUDPToRemote(packet C.UDPPacket, pc C.PacketConn, metadata *C.Metadata, key string) error {
	defer packet.Drop()

	//addr := pc.Raddr()
	//if addr == nil {
	//	return errors.New("udp addr invalid")
	//}

	if !metadata.Resolved() {
		log.Infoln("handleUDPToRemote need to resolve ip for host:%s", metadata.Host)
		ip, err := resolver.ResolveIP(metadata.Host)
		if err != nil {
			return err
		}
		if metadata.DstIP == nil {
			metadata.DstIP = ip
		}
	}

	addr := metadata.UDPAddr()
	if addr == nil {
		return errors.New("udp addr invalid")
	}

	//if addr.String() != metadata.UDPAddr().String() {
	//	log.Errorln("handleUDPToRemote addr:%s not equal metadata.UDPAddr:%s", addr.String(), metadata.UDPAddr().String())
	//}

	timeOut := udpTimeout

	if addr.Port == 53 {
		timeOut = 5 * time.Second
	}

	if _, err := pc.WriteTo(packet.Data(), addr); err != nil {
		natTable.Delete(key)
		pc.Close()
		//log.Debugln("handleUDPToRemote WriteTo:%v err:%v", addr, err)
		return err
	}
	// reset timeout
	//log.Debugln("handleUDPToRemote SetReadDeadline WriteTo:%v timeOut:%v", addr, timeOut)
	if pc.NeedUpdateDeadline(timeOut) {
		pc.SetReadDeadline(time.Now().Add(timeOut))
		pc.UpdateLastUpdate()
	}
	return nil
}

func handleUDPToLocal(packet C.UDPPacket, pc C.PacketConn, key string, fAddr net.Addr) {
	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	defer natTable.Delete(key)
	defer pc.Close()

	count := atomic.AddInt64(&currentConns, 1)
	defer atomic.AddInt64(&currentConns, -1) // 确保任何路径退出都会减少计数

	if count > maxConns {
		log.Warnln("[Breaker] Concurrent connections (%d) exceed max (%d), rejecting handleUDPToLocal.", count, maxConns)
		return
	}

	timeOut := udpTimeout
	//if fAddr != nil && fAddr.(*net.UDPAddr).Port == 53 {
	//	timeOut = 5 * time.Second
	//}
	pc.SetReadDeadline(time.Now().Add(timeOut))
	pc.UpdateLastUpdate()

	for {
		if pc.NeedUpdateDeadline(timeOut) {
			pc.SetReadDeadline(time.Now().Add(timeOut))
			pc.UpdateLastUpdate()
		}
		n, from, err := pc.ReadFrom(buf)

		if err != nil {
			return
		}

		if fAddr != nil {
			from = fAddr
		}

		_, err = packet.WriteBack(buf[:n], from)
		if err != nil {
			return
		}
	}
}

type IdleTimeoutConn struct {
	Conn    net.Conn
	Timeout time.Duration
}

//func handleSocket(ctx C.ConnContext, outbound net.Conn) {
//	relay(ctx.Conn(), outbound)
//}

type ReadOnlyReader struct {
	io.Reader
}

type WriteOnlyWriter struct {
	io.Writer
}

func setKeepAlive(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		tcp.SetKeepAlive(true)
		// 30s 是一个平衡点：既能绕过大部分防火墙的空闲回收，
		// 又不会因为探测太频繁而消耗过多移动端电量。
		tcp.SetKeepAlivePeriod(30 * time.Second)
	}
}

func noZeroCopyRelay(left, right net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	copyFunc := func(dst, src net.Conn) {
		defer wg.Done()
		defer dst.Close()
		defer src.Close()

		buf := bufPool.Get().([]byte)
		defer bufPool.Put(buf)
		_, _ = io.CopyBuffer(dst, src, buf)

		dst.SetDeadline(time.Now())
	}

	go copyFunc(right, left)
	copyFunc(left, right)

	wg.Wait()
}

func relay(left, right net.Conn) {
	// 1. 兜底释放：如果 copyFunc 没跑起来，这里确保连接不泄露
	// 如果已经执行了 copyFunc 里的 Close，重复 Close 是安全的
	defer left.Close()
	defer right.Close()

	count := atomic.AddInt64(&currentConns, 1)
	defer atomic.AddInt64(&currentConns, -1) // 确保任何路径退出都会减少计数

	if count > maxConns {
		log.Warnln("[Breaker] Concurrent connections (%d) exceed max (%d), rejecting relay.", count, maxConns)
		return
	}

	setKeepAlive(left)
	setKeepAlive(right)

	idleTimeout := 5 * time.Minute
	leftConn := NewIdleConn(left, idleTimeout)
	rightConn := NewIdleConn(right, idleTimeout)

	if !zeroCopy {
		noZeroCopyRelay(leftConn, rightConn)
		return
	}

	// 1. 预处理：排空所有缓冲区，并获取最底层的原始连接
	orgLeft := drainAndUnwrap(leftConn, rightConn)
	orgRight := drainAndUnwrap(rightConn, leftConn)

	// 2. 此时 left 和 right 已经是被“剥干净”的原始连接（或者是无法剥离的特殊连接）
	// 尝试获取原始 TCP 指针以触发 Splice
	rawLeft, leftIsTCP := orgLeft.(*net.TCPConn)
	rawRight, rightIsTCP := orgRight.(*net.TCPConn)

	var wg sync.WaitGroup
	wg.Add(2)

	log.Debugln("drainAndUnwrap leftConn:%v rightConn:%v leftIsTCP:%v rawLeft:%v rightIsTCP:%v rawRight:%v", leftConn, rightConn, leftIsTCP, rawLeft, rightIsTCP, rawRight)

	copyFunc := func(warpDst, warpSrc *IdleConn, dst, src net.Conn, dstIsTCP, srcIsTCP bool) {
		defer wg.Done()
		defer warpDst.Close()
		defer warpSrc.Close()
		if zeroCopy && dstIsTCP && srcIsTCP {
			trackSrc, isSrc := findTrack(warpSrc)
			trackDst, isDst := findTrack(warpDst)
			if isSrc {
				trackSrc.SetModeZeroCopy(true)
			}
			if isDst {
				trackDst.SetModeZeroCopy(true)
			}
			// 【快车道】进入内核 Splice，直接在内核转发
			//write, _ := io.Copy(dst, src)
			for {
				write, err := io.CopyN(dst, src, 1024*1024)
				if isSrc {
					trackSrc.SetDownloadTotal(write)
				}
				if isDst {
					trackDst.SetUploadTotal(write)
				}

				if err != nil {
					log.Debugln("copyFunc CopyN err:%v", err)
					break
				}
			}
		} else {
			// 【慢车道】用户态拷贝
			warpDst.StartDeadline()
			buf := bufPool.Get().([]byte)
			defer bufPool.Put(buf)
			_, _ = io.CopyBuffer(warpDst, warpSrc, buf)
		}
		//warpDst.SetDeadline(time.Now())
	}

	log.Debugln("drainAndUnwrap leftIsTCP:%v rightIsTCP:%v", leftIsTCP, rightIsTCP)

	go copyFunc(rightConn, leftConn, rawRight, rawLeft, rightIsTCP, leftIsTCP)
	go copyFunc(leftConn, rightConn, rawLeft, rawRight, leftIsTCP, rightIsTCP)

	wg.Wait()
}

// Wrapper 定义了获取底层连接的通用接口
type Wrapper interface {
	RawConn() net.Conn
}

// drainAndUnwrap: 核心工具函数
func drainAndUnwrap(conn net.Conn, target net.Conn) net.Conn {
	curr := conn

	for {
		log.Debugln("drainAndUnwrap conn:%v layer: type=%T", conn, curr)
		// 特殊处理：带缓冲的包装
		if bc, ok := curr.(*N.BufferedConn); ok {
			if n := bc.Buffered(); n > 0 {
				// 将协议识别阶段预读的数据先推给目标
				// 注意：这里 target 也可以做一次简单的 unwrap 以提高写入效率
				buf := make([]byte, n)
				_, _ = bc.Reader().Read(buf)
				_, _ = target.Write(buf)
			}
			curr = bc.RawConn()
			continue
		}

		// 普通处理：纯统计或超时包装
		if w, ok := curr.(Wrapper); ok {
			curr = w.RawConn()
			continue
		}

		break
	}
	return curr
}

type connTracker interface {
	SetDownloadTotal(download int64)
	SetUploadTotal(upload int64)
	SetModeZeroCopy(modeZeroCopy bool)
}

func findTrack(conn net.Conn) (connTracker, bool) {
	curr := conn
	log.Debugln("findTrack call conn:%v layer: type=%T", conn, curr)
	for {
		//log.Infoln("updateTrack conn:%v layer: type=%T download:%d upload:%d", conn, curr, download, upload)
		// 特殊处理：带缓冲的包装
		if bc, ok := curr.(connTracker); ok {
			return bc, true
		}

		// 普通处理：纯统计或超时包装
		if w, ok := curr.(Wrapper); ok {
			curr = w.RawConn()
			continue
		}

		break
	}
	return nil, false
}
