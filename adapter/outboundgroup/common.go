package outboundgroup

import (
	"time"

	C "github.com/finddiff/clashWithCache/constant"
	"github.com/finddiff/clashWithCache/constant/provider"
)

const (
	defaultGetProxiesDuration = time.Second * 5
)

func getProvidersProxies(providers []provider.ProxyProvider, touch bool) []C.Proxy {
	proxies := []C.Proxy{}
	for _, provider := range providers {
		if touch {
			proxies = append(proxies, provider.ProxiesWithTouch()...)
		} else {
			proxies = append(proxies, provider.Proxies()...)
		}
	}
	return proxies
}
