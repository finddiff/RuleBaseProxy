package dns

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"

	"github.com/finddiff/RuleBaseProxy/component/dialer"
	"github.com/finddiff/RuleBaseProxy/component/resolver"

	D "github.com/miekg/dns"
)

type client struct {
	*D.Client
	r    *Resolver
	port string
	host string
}

func (c *client) Exchange(m *D.Msg) (*D.Msg, error) {
	return c.ExchangeContext(context.Background(), m)
}

func (c *client) ExchangeContext(ctx context.Context, m *D.Msg) (*D.Msg, error) {
	var (
		ip  net.IP
		err error
	)
	if c.r == nil {
		// a default ip dns
		if ip = net.ParseIP(c.host); ip == nil {
			return nil, fmt.Errorf("dns %s not a valid ip", c.host)
		}
	} else {
		if ip, err = resolver.ResolveIPWithResolver(c.host, c.r); err != nil {
			return nil, fmt.Errorf("use default dns resolve failed: %w", err)
		}
	}

	network := "udp"
	if strings.HasPrefix(c.Client.Net, "tcp") {
		network = "tcp"
	}

	conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), c.port))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// miekg/dns ExchangeContext doesn't respond to context cancel.
	// this is a workaround
	type result struct {
		msg *D.Msg
		err error
	}
	ch := make(chan result, 1)
	go func() {
		if strings.HasSuffix(c.Client.Net, "tls") {
			conn = tls.Client(conn, c.Client.TLSConfig)
		}

		msg, _, err := c.Client.ExchangeWithConn(m, &D.Conn{
			Conn:         conn,
			UDPSize:      c.Client.UDPSize,
			TsigSecret:   c.Client.TsigSecret,
			TsigProvider: c.Client.TsigProvider,
		})

		ch <- result{msg, err}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case ret := <-ch:
		return ret.msg, ret.err
	}
}