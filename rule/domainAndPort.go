package rules

import (
	C "github.com/finddiff/RuleBaseProxy/constant"
	"strconv"
	"strings"
)

type DomainAndPort struct {
	domain            string
	adapter           string
	port              string
	isSource          bool
	multiDomainDialip bool
}

func (d *DomainAndPort) RuleType() C.RuleType {
	if d.isSource {
		return C.DomainSrcPort
	} else {
		return C.DomainDstPort
	}
}

func (d *DomainAndPort) Match(metadata *C.Metadata) bool {
	//log.Debugln("metadata:%v MatchPort:%v MatchDomain:%v", metadata, d.MatchPort(metadata), d.MatchDomain(metadata))
	return d.MatchPort(metadata) && d.MatchDomain(metadata)
}

func (d *DomainAndPort) MatchDomain(metadata *C.Metadata) bool {
	if metadata.AddrType != C.AtypDomainName {
		return false
	}
	domain := metadata.Host
	return strings.HasSuffix(domain, "."+d.domain) || domain == d.domain
}

func (d *DomainAndPort) MatchPort(metadata *C.Metadata) bool {
	if d.isSource {
		return metadata.SrcPort == d.port
	}
	return metadata.DstPort == d.port
}

func (d *DomainAndPort) Adapter() string {
	return d.adapter
}

func (d *DomainAndPort) Payload() string {
	return d.domain + "," + d.port
}

func (d *DomainAndPort) ShouldResolveIP() bool {
	return false
}

func (d *DomainAndPort) MultiDomainDialIP() bool {
	return d.multiDomainDialip
}

func NewDomainAndPort(domain string, adapter string, port string, isSource bool, multiDomainDialip bool) *DomainAndPort {
	_, err := strconv.Atoi(port)
	if err != nil {
		return nil
	}
	return &DomainAndPort{
		domain:            strings.ToLower(domain),
		adapter:           adapter,
		port:              port,
		isSource:          isSource,
		multiDomainDialip: multiDomainDialip,
	}
}
