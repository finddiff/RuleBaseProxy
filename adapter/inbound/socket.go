package inbound

import (
	"github.com/finddiff/clashWithCache/log"
	"net"

	C "github.com/finddiff/clashWithCache/constant"
	"github.com/finddiff/clashWithCache/context"
	"github.com/finddiff/clashWithCache/transport/socks5"
)

// NewSocket receive TCP inbound and return ConnContext
func NewSocket(target socks5.Addr, conn net.Conn, source C.Type) *context.ConnContext {
	metadata := parseSocksAddr(target)
	log.Debugln("direct NewSocket infokey:%s", metadata.InfoKey())
	metadata.NetWork = C.TCP
	metadata.Type = source
	if ip, port, err := parseAddr(conn.RemoteAddr().String()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = port
	}

	return context.NewConnContext(conn, metadata)
}
