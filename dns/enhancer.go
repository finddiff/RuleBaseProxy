package dns

import (
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/hashicorp/golang-lru/v2"
	"net"

	"github.com/finddiff/RuleBaseProxy/common/cache"
	"github.com/finddiff/RuleBaseProxy/component/fakeip"
)

var (
	ADGurdRules    []C.Rule
	ADGurdCache, _ = lru.New[string, bool](1024 * 1024 * 4)
)

func ADGurdMatch(domain string) bool {
	if value, ok := ADGurdCache.Get(domain); ok {
		return value
	}
	for _, rule := range ADGurdRules {
		if rule.Match(&C.Metadata{
			AddrType: C.AtypDomainName,
			Host:     domain,
		}) {
			ADGurdCache.Add(domain, true)
			return true
		}
	}
	ADGurdCache.Add(domain, false)

	return false
}

type ResolverEnhancer struct {
	mode     EnhancedMode
	fakePool *fakeip.Pool
	mapping  *cache.LruCache
}

func (h *ResolverEnhancer) FakeIPEnabled() bool {
	return h.mode == FAKEIP
}

func (h *ResolverEnhancer) MappingEnabled() bool {
	return h.mode == FAKEIP || h.mode == MAPPING
}

func (h *ResolverEnhancer) IsExistFakeIP(ip net.IP) bool {
	if !h.FakeIPEnabled() {
		return false
	}

	if pool := h.fakePool; pool != nil {
		return pool.Exist(ip)
	}

	return false
}

func (h *ResolverEnhancer) IsFakeIP(ip net.IP) bool {
	if !h.FakeIPEnabled() {
		return false
	}

	if pool := h.fakePool; pool != nil {
		return pool.IPNet().Contains(ip) && !pool.Gateway().Equal(ip)
	}

	return false
}

func (h *ResolverEnhancer) FindHostByIP(ip net.IP) (string, bool) {
	if pool := h.fakePool; pool != nil {
		if host, existed := pool.LookBack(ip); existed {
			return host, true
		}
	}

	if mapping := h.mapping; mapping != nil {
		if host, existed := h.mapping.Get(ip.String()); existed {
			return host.(string), true
		}
	}

	return "", false
}

func (h *ResolverEnhancer) PatchFrom(o *ResolverEnhancer) {
	if h.mapping != nil && o.mapping != nil {
		o.mapping.CloneTo(h.mapping)
	}

	if h.fakePool != nil && o.fakePool != nil {
		h.fakePool.PatchFrom(o.fakePool)
	}
}

func NewEnhancer(cfg Config) *ResolverEnhancer {
	var fakePool *fakeip.Pool
	var mapping *cache.LruCache

	if cfg.EnhancedMode != NORMAL {
		fakePool = cfg.Pool
		mapping = cache.NewLRUCache(cache.WithSize(4096), cache.WithStale(true))
		if cfg.EnhancedMode == FAKEIP {
			cfg.EnhancedMode = MAPPING
		}
	}

	return &ResolverEnhancer{
		mode:     cfg.EnhancedMode,
		fakePool: fakePool,
		mapping:  mapping,
	}
}
