package rules

import (
	"strings"

	C "github.com/finddiff/RuleBaseProxy/constant"
)

type DomainSuffix struct {
	suffix            string
	adapter           string
	multiDomainDialip bool
}

func (ds *DomainSuffix) RuleType() C.RuleType {
	return C.DomainSuffix
}

func (ds *DomainSuffix) Match(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}
	domain := metadata.Host
	return strings.HasSuffix(domain, "."+ds.suffix) || domain == ds.suffix
}

func (ds *DomainSuffix) Adapter() string {
	return ds.adapter
}

func (ds *DomainSuffix) Payload() string {
	return ds.suffix
}

func (ds *DomainSuffix) ShouldResolveIP() bool {
	return false
}

func (d *DomainSuffix) MultiDomainDialIP() bool {
	return d.multiDomainDialip
}

func NewDomainSuffix(suffix string, adapter string, multiDomainDialip bool) *DomainSuffix {
	return &DomainSuffix{
		suffix:            strings.ToLower(suffix),
		adapter:           adapter,
		multiDomainDialip: multiDomainDialip,
	}
}
