package rules

import (
	"strings"

	C "github.com/finddiff/RuleBaseProxy/constant"
)

type DomainKeyword struct {
	keyword           string
	adapter           string
	multiDomainDialip bool
}

func (dk *DomainKeyword) RuleType() C.RuleType {
	return C.DomainKeyword
}

func (dk *DomainKeyword) Match(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}
	domain := metadata.Host
	return strings.Contains(domain, dk.keyword)
}

func (dk *DomainKeyword) Adapter() string {
	return dk.adapter
}

func (dk *DomainKeyword) Payload() string {
	return dk.keyword
}

func (dk *DomainKeyword) ShouldResolveIP() bool {
	return false
}

func (d *DomainKeyword) MultiDomainDialIP() bool {
	return d.multiDomainDialip
}

func NewDomainKeyword(keyword string, adapter string, multiDomainDialip bool) *DomainKeyword {
	return &DomainKeyword{
		keyword:           strings.ToLower(keyword),
		adapter:           adapter,
		multiDomainDialip: multiDomainDialip,
	}
}
