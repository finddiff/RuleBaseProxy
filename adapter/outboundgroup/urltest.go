package outboundgroup

import (
	"context"
	"encoding/json"
	"github.com/finddiff/RuleBaseProxy/adapter/outbound"
	"github.com/finddiff/RuleBaseProxy/common/singledo"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/constant/provider"
	"time"
)

type urlTestOption func(*URLTest)

func urlTestWithTolerance(tolerance uint16) urlTestOption {
	return func(u *URLTest) {
		u.tolerance = tolerance
	}
}

type URLTest struct {
	*outbound.Base
	tolerance  uint16
	disableUDP bool
	fastNode   C.Proxy
	//atfastNode atomic.UnsafePointer
	single     *singledo.Single
	fastSingle *singledo.Single
	providers  []provider.ProxyProvider
}

func (u *URLTest) Now() string {
	return u.fast(false).Name()
}

// DialContext implements C.ProxyAdapter
func (u *URLTest) DialContext(ctx context.Context, metadata *C.Metadata) (c C.Conn, err error) {
	c, err = u.fast(true).DialContext(ctx, metadata)
	if err == nil {
		c.AppendToChains(u)
	}
	return c, err
}

// DialUDP implements C.ProxyAdapter
func (u *URLTest) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := u.fast(true).DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(u)
	}
	return pc, err
}

// Unwrap implements C.ProxyAdapter
func (u *URLTest) Unwrap(metadata *C.Metadata) C.Proxy {
	return u.fast(true)
}

func (u *URLTest) proxies(touch bool) []C.Proxy {
	elm, _, _ := u.single.Do(func() (interface{}, error) {
		return getProvidersProxies(u.providers, touch), nil
	})

	return elm.([]C.Proxy)
}

func (u *URLTest) fast(touch bool) C.Proxy {
	//return *((*C.Proxy)(u.atfastNode.Load()))
	return u.fastNode
}

// SupportUDP implements C.ProxyAdapter
func (u *URLTest) SupportUDP() bool {
	if u.disableUDP {
		return false
	}

	return u.fast(false).SupportUDP()
}

// MarshalJSON implements C.ProxyAdapter
func (u *URLTest) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range u.proxies(false) {
		all = append(all, proxy.Name())
	}
	return json.Marshal(map[string]interface{}{
		"type": u.Type().String(),
		"now":  u.Now(),
		"all":  all,
	})
}

func parseURLTestOption(config map[string]interface{}) []urlTestOption {
	opts := []urlTestOption{}

	// tolerance
	if elm, ok := config["tolerance"]; ok {
		if tolerance, ok := elm.(int); ok {
			opts = append(opts, urlTestWithTolerance(uint16(tolerance)))
		}
	}

	return opts
}

func NewURLTest(commonOptions *GroupCommonOption, providers []provider.ProxyProvider, options ...urlTestOption) *URLTest {
	fastNode := providers[0].Proxies()[0]
	urlTest := &URLTest{
		Base:       outbound.NewBase(commonOptions.Name, "", C.URLTest, false),
		single:     singledo.NewSingle(defaultGetProxiesDuration),
		fastSingle: singledo.NewSingle(time.Second * 10),
		fastNode:   fastNode,
		providers:  providers,
		disableUDP: commonOptions.DisableUDP,
	}

	//urlTest.atfastNode.Store(unsafe.Pointer(&fastNode))

	for _, option := range options {
		option(urlTest)
	}

	//定时更新最小延迟节点
	go func() {
		ticker := time.NewTicker(time.Second * 15)
		defer ticker.Stop()
		for range ticker.C {
			proxies := urlTest.proxies(true)
			fast := proxies[0]
			min := fast.LastDelay()

			for _, proxy := range proxies[1:] {
				if !proxy.Alive() {
					continue
				}

				delay := proxy.LastDelay()
				if delay < min {
					fast = proxy
					min = delay
				}
			}

			urlTest.fastNode = fast
			//urlTest.atfastNode.Store(unsafe.Pointer(&fast))
		}
	}()

	return urlTest
}
