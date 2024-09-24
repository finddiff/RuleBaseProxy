package outbound

import (
	"context"
	"errors"
	"github.com/dgraph-io/ristretto"
	"github.com/finddiff/RuleBaseProxy/component/dialer"
	"github.com/finddiff/RuleBaseProxy/component/resolver"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel"
	"net"
)

var (
	dirCache, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 28, // maximum cost of cache (256MB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
)

type Direct struct {
	*Base
}

// DialContext implements C.ProxyAdapter
func (d *Direct) DialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	if metadata.Type == C.WINTUN {
		return nil, errors.New("Tun mode cannot DialContext by Direct")
	}
	var c net.Conn
	c = nil
	var err error

	MultiDomain := tunnel.InSeIP(metadata.DstIP.String()) || tunnel.InSeDomain(metadata.Host)

	log.Debugln("direct DialContext DstAddr %s:%s, infokey:%s, AddrType:%v, MultiDomain:%v", metadata.DstAddr(), metadata.DstPort, metadata.InfoKey(), metadata.AddrType, MultiDomain)
	if !(metadata.Type.String() == "HTTP" || metadata.Type.String() == "HTTP Connect" || metadata.Type.String() == "Socks4" || metadata.Type.String() == "Socks5") {
		if MultiDomain {
			if metadata.DstIP.To4() != nil {
				metadata.AddrType = C.AtypIPv4
			} else {
				metadata.AddrType = C.AtypIPv6
			}
			log.Debugln("direct handleTCPConn dial by ip infokey:%s", metadata.InfoKey())
		} else {
			if metadata.Host != "" {
				metadata.AddrType = C.AtypDomainName
				//metadata.DstIP = nil
				log.Debugln("direct DialContext dial by domain infokey:%s", metadata.InfoKey())
			} else {
				log.Debugln("direct DialContext dial by defaule infokey:%s", metadata.InfoKey())
			}
		}
	}

	if metadata.AddrType != C.AtypDomainName {
		c, err = dialer.DialContextHost(ctx, "tcp", metadata.DstAddr(), metadata.DstPort)
	} else {
		c, err = dialer.DialContextHost(ctx, "tcp", metadata.String(), metadata.DstPort)
	}

	if err != nil {
		if c != nil {
			c.Close()
		}
		return nil, err
	}

	if c == nil {
		return nil, errors.New("DialContext c is nil")
	}

	tcpKeepAlive(c)
	return NewConn(c, d), nil
}

// DialContext implements C.ProxyAdapter
func (d *Direct) ipDialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	infoKey := metadata.InfoKey()
	log.Debugln("direct DialContext infoKey:%s", infoKey)
	var c net.Conn
	c = nil
	var err error
	if metadata.Host == "" {
		c, err = dialer.ExDialContext(ctx, "tcp", metadata.DstIP, metadata.DstPort)
	} else {
		if metadata.DstIP == nil {
			c, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(metadata.String(), metadata.DstPort))
		} else {
			if resolver.IsExistFakeIP(metadata.DstIP) {
				c, err = dialer.DialContext(ctx, "tcp", net.JoinHostPort(metadata.String(), metadata.DstPort))
				//if c != nil {
				//	dirCache.SetWithTTL(infoKey, c.RemoteAddr().String(), 1, time.Minute*60*4)
				//}
			} else {
				c, err = dialer.ExDialContext(ctx, "tcp", metadata.DstIP, metadata.DstPort)
			}
		}
	}

	if err != nil {
		if c != nil {
			c.Close()
		}
		return nil, err
	}

	if c == nil {
		return nil, errors.New("DialContext c is nil")
	}

	tcpKeepAlive(c)
	//log.Infoln("direct DialContext dail %s %s:%s<-->%s", metadata.Host, metadata.DstIP.String(), metadata.DstPort, c.RemoteAddr().String())
	return NewConn(c, d), nil
}

// DialContext implements C.ProxyAdapter
func (d *Direct) orgDialContext(ctx context.Context, metadata *C.Metadata) (C.Conn, error) {
	c, err := dialer.DialContext(ctx, "tcp", metadata.RemoteAddress())
	if err != nil {
		return nil, err
	}
	tcpKeepAlive(c)
	return NewConn(c, d), nil
}

// DialUDP implements C.ProxyAdapter
func (d *Direct) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	if metadata.Type == C.WINTUN {
		return nil, errors.New("Tun mode cannot dailUDP by Direct")
	}
	pc, err := dialer.ListenPacket(context.Background(), "udp", "")
	if err != nil {
		return nil, err
	}
	return newPacketConn(&directPacketConn{pc}, d), nil
}

type directPacketConn struct {
	net.PacketConn
}

func NewDirect() *Direct {
	return &Direct{
		Base: &Base{
			name: "DIRECT",
			tp:   C.Direct,
			udp:  true,
		},
	}
}
