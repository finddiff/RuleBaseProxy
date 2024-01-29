package rules

import (
	"net"

	C "github.com/finddiff/RuleBaseProxy/constant"
)

type IPCIDROption func(*IPCIDR)

func WithIPCIDRSourceIP(b bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.isSourceIP = b
	}
}

func WithIPCIDRNoResolve(noResolve bool) IPCIDROption {
	return func(i *IPCIDR) {
		i.noResolveIP = noResolve
	}
}

func WithDstPort(port string) IPCIDROption {
	return func(i *IPCIDR) {
		i.isWithPort = true
		i.port = port
	}
}

type IPCIDR struct {
	ipnet       *net.IPNet
	adapter     string
	isSourceIP  bool
	noResolveIP bool
	isWithPort  bool
	port        string
}

func (i *IPCIDR) RuleType() C.RuleType {
	if i.isWithPort {
		return C.DstIPPort
	}
	if i.isSourceIP {
		return C.SrcIPCIDR
	}
	return C.IPCIDR
}

func (i *IPCIDR) Match(metadata *C.Metadata) bool {
	ip := metadata.DstIP
	if i.isSourceIP {
		ip = metadata.SrcIP
	}
	if i.isWithPort {
		return ip != nil && i.ipnet.Contains(ip) && metadata.DstPort == i.port
	} else {
		return ip != nil && i.ipnet.Contains(ip)
	}
}

func (i *IPCIDR) Adapter() string {
	return i.adapter
}

func (i *IPCIDR) Payload() string {
	if i.isWithPort {
		return i.ipnet.String() + " port: " + i.port
	}
	return i.ipnet.String()
}

func (i *IPCIDR) ShouldResolveIP() bool {
	return !i.noResolveIP
}

func NewIPCIDR(s string, adapter string, opts ...IPCIDROption) (*IPCIDR, error) {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		return nil, errPayload
	}

	ipcidr := &IPCIDR{
		ipnet:   ipnet,
		adapter: adapter,
	}

	for _, o := range opts {
		o(ipcidr)
	}

	return ipcidr, nil
}
