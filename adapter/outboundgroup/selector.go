package outboundgroup

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/finddiff/RuleBaseProxy/adapter/outbound"
	"github.com/finddiff/RuleBaseProxy/common/singledo"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/constant/provider"
)

type Selector struct {
	*outbound.Base
	disableUDP bool
	single     *singledo.Single
	selected   string
	sedProxy   C.Proxy
	//seProxy    atomic.UnsafePointer
	providers []provider.ProxyProvider
}

// DialContext implements C.ProxyAdapter
func (s *Selector) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := s.selectedProxy(true).DialContext(ctx, metadata)
	if err == nil {
		c.AppendToChains(s)
	}
	return c, err
}

// DialUDP implements C.ProxyAdapter
func (s *Selector) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	pc, err := s.selectedProxy(true).DialUDP(metadata)
	if err == nil {
		pc.AppendToChains(s)
	}
	return pc, err
}

// SupportUDP implements C.ProxyAdapter
func (s *Selector) SupportUDP() bool {
	if s.disableUDP {
		return false
	}

	return s.selectedProxy(false).SupportUDP()
}

// MarshalJSON implements C.ProxyAdapter
func (s *Selector) MarshalJSON() ([]byte, error) {
	var all []string
	for _, proxy := range getProvidersProxies(s.providers, false) {
		all = append(all, proxy.Name())
	}

	return json.Marshal(map[string]interface{}{
		"type": s.Type().String(),
		"now":  s.Now(),
		"all":  all,
	})
}

func (s *Selector) Now() string {
	return s.selectedProxy(false).Name()
}

func (s *Selector) Set(name string) error {
	for _, proxy := range getProvidersProxies(s.providers, false) {
		if proxy.Name() == name {
			s.selected = name
			//s.seProxy.Store(unsafe.Pointer(&proxy))
			s.sedProxy = proxy
			s.single.Reset()
			return nil
		}
	}

	return errors.New("proxy not exist")
}

// Unwrap implements C.ProxyAdapter
func (s *Selector) Unwrap(metadata *C.Metadata) C.Proxy {
	return s.selectedProxy(true)
}

func (s *Selector) selectedProxy(touch bool) C.Proxy {
	//return *((*C.Proxy)(s.seProxy.Load()))
	return s.sedProxy
}

func NewSelector(options *GroupCommonOption, providers []provider.ProxyProvider) *Selector {
	selector := &Selector{
		Base:      outbound.NewBase(options.Name, "", C.Selector, false),
		single:    singledo.NewSingle(defaultGetProxiesDuration),
		providers: providers,
		//selected:   selected,
		//seProxy:    proxy,
		disableUDP: options.DisableUDP,
	}
	selector.selected = providers[0].Proxies()[0].Name()
	selector.sedProxy = providers[0].Proxies()[0]
	//selector.seProxy.Store(unsafe.Pointer(&(providers[0].Proxies()[0])))
	return selector
}
