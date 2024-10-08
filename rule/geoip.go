package rules

import (
	"strings"

	"github.com/finddiff/RuleBaseProxy/component/mmdb"
	C "github.com/finddiff/RuleBaseProxy/constant"
)

type GEOIP struct {
	country           string
	adapter           string
	noResolveIP       bool
	multiDomainDialip bool
}

func (g *GEOIP) RuleType() C.RuleType {
	return C.GEOIP
}

func (g *GEOIP) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if ip == nil {
		return false
	}

	if strings.EqualFold(g.country, "LAN") {
		return ip.IsPrivate()
	}
	record, _ := mmdb.Instance().Country(ip)
	return strings.EqualFold(record.Country.IsoCode, g.country)
}

func (g *GEOIP) Adapter() string {
	return g.adapter
}

func (g *GEOIP) Payload() string {
	return g.country
}

func (g *GEOIP) ShouldResolveIP() bool {
	return !g.noResolveIP
}

func (d *GEOIP) MultiDomainDialIP() bool {
	return d.multiDomainDialip
}

func NewGEOIP(country string, adapter string, noResolveIP bool, multiDomainDialip bool) *GEOIP {
	geoip := &GEOIP{
		country:           country,
		adapter:           adapter,
		noResolveIP:       noResolveIP,
		multiDomainDialip: multiDomainDialip,
	}

	return geoip
}
