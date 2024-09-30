package context

import (
	"github.com/gofrs/uuid"
	"github.com/miekg/dns"
	"net"
)

const (
	DNSTypeHost   = "host"
	DNSTypeFakeIP = "fakeip"
	DNSTypeRaw    = "raw"
)

type DNSContext struct {
	id    uuid.UUID
	msg   *dns.Msg
	tp    string
	raddr net.Addr
	Host  string
}

func NewDNSContext(msg *dns.Msg, raddr net.Addr) *DNSContext {
	id, _ := uuid.NewV4()
	return &DNSContext{
		id:    id,
		msg:   msg,
		raddr: raddr,
	}
}

// ID implement C.PlainContext ID
func (c *DNSContext) ID() uuid.UUID {
	return c.id
}

// SetType set type of response
func (c *DNSContext) SetType(tp string) {
	c.tp = tp
}

// Type return type of response
func (c *DNSContext) Type() string {
	return c.tp
}

// RemoteAddr return remot net.Addr of response
func (c *DNSContext) RemoteAddr() net.Addr {
	return c.raddr
}
