package constant

import "golang.org/x/net/proxy"

var (
	DnsProxyString = ""
	TunProxyString = ""
	UserName       = ""
	UserPass       = ""
	FilterCacheIP  = false
	//SockProxy      *outbound.Socks5
	SockDialer proxy.Dialer = nil
)
