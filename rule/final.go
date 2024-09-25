package rules

import (
	C "github.com/finddiff/RuleBaseProxy/constant"
)

type Match struct {
	adapter           string
	multiDomainDialip bool
}

func (f *Match) RuleType() C.RuleType {
	return C.MATCH
}

func (f *Match) Match(metadata *C.Metadata) bool {
	return true
}

func (f *Match) Adapter() string {
	return f.adapter
}

func (f *Match) Payload() string {
	return ""
}

func (f *Match) ShouldResolveIP() bool {
	return false
}

func (d *Match) MultiDomainDialIP() bool {
	return d.multiDomainDialip
}

func NewMatch(adapter string, multiDomainDialip bool) *Match {
	return &Match{
		adapter:           adapter,
		multiDomainDialip: multiDomainDialip,
	}
}
