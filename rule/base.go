package rules

import (
	"errors"
)

var (
	errPayload = errors.New("payload error")

	noResolve         = "no-resolve"
	multiDomainDailIP = "dial-ip"
)

func HasNoResolve(params []string) bool {
	for _, p := range params {
		if p == noResolve {
			return true
		}
	}
	return false
}

func HasMultiDomainDailIP(params []string) bool {
	for _, p := range params {
		if p == multiDomainDailIP {
			return true
		}
	}
	return false
}
