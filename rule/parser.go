package rules

import (
	"fmt"

	C "github.com/finddiff/RuleBaseProxy/constant"
)

func ParseRule(tp, payload, target string, params []string) (C.Rule, error) {
	var (
		parseErr error
		parsed   C.Rule
	)

	multiDomainDialip := HasMultiDomainDailIP(params)

	switch tp {
	case "DOMAIN":
		parsed = NewDomain(payload, target, multiDomainDialip)
	case "DOMAIN-SUFFIX":
		parsed = NewDomainSuffix(payload, target, multiDomainDialip)
	case "DOMAIN-KEYWORD":
		parsed = NewDomainKeyword(payload, target, multiDomainDialip)
	case "GEOIP":
		noResolve := HasNoResolve(params)
		parsed = NewGEOIP(payload, target, noResolve, multiDomainDialip)
	case "IP-CIDR", "IP-CIDR6":
		noResolve := HasNoResolve(params)
		parsed, parseErr = NewIPCIDR(payload, target, multiDomainDialip, WithIPCIDRNoResolve(noResolve))
	case "SRC-IP-CIDR":
		parsed, parseErr = NewIPCIDR(payload, target, multiDomainDialip, WithIPCIDRSourceIP(true), WithIPCIDRNoResolve(true))
	case "DST-IP-PORT":
		parsed, parseErr = NewIPCIDR(payload, target, multiDomainDialip, WithDstPort(params[0]), WithIPCIDRNoResolve(true))
	case "SRC-PORT":
		parsed, parseErr = NewPort(payload, target, true, multiDomainDialip)
	case "DST-PORT":
		parsed, parseErr = NewPort(payload, target, false, multiDomainDialip)
	case "PROCESS-NAME":
		parsed, parseErr = NewProcess(payload, target, multiDomainDialip)
	case "MATCH":
		parsed = NewMatch(target, multiDomainDialip)
	case "ALLIP":
		parsed = NewAllIP(target, multiDomainDialip)
	case "DOMAIN-SRC-PORT":
		parsed = NewDomainAndPort(payload, target, params[0], true, multiDomainDialip)
	case "DOMAIN-DST-PORT":
		parsed = NewDomainAndPort(payload, target, params[0], false, multiDomainDialip)
	default:
		parseErr = fmt.Errorf("unsupported rule type %s", tp)
	}

	return parsed, parseErr
}
