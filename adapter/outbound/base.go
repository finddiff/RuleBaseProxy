package outbound

import (
	"encoding/json"
	"errors"
	"net"
	"time"

	C "github.com/finddiff/RuleBaseProxy/constant"
)

type Base struct {
	name string
	addr string
	tp   C.AdapterType
	udp  bool
}

// Name implements C.ProxyAdapter
func (b *Base) Name() string {
	return b.name
}

// Type implements C.ProxyAdapter
func (b *Base) Type() C.AdapterType {
	return b.tp
}

// StreamConn implements C.ProxyAdapter
func (b *Base) StreamConn(c net.Conn, metadata *C.Metadata) (net.Conn, error) {
	return c, errors.New("no support")
}

// DialUDP implements C.ProxyAdapter
func (b *Base) DialUDP(metadata *C.Metadata) (C.PacketConn, error) {
	return nil, errors.New("no support")
}

// SupportUDP implements C.ProxyAdapter
func (b *Base) SupportUDP() bool {
	return b.udp
}

// MarshalJSON implements C.ProxyAdapter
func (b *Base) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string{
		"type": b.Type().String(),
	})
}

// Addr implements C.ProxyAdapter
func (b *Base) Addr() string {
	return b.addr
}

// Unwrap implements C.ProxyAdapter
func (b *Base) Unwrap(metadata *C.Metadata) C.Proxy {
	return nil
}

func NewBase(name string, addr string, tp C.AdapterType, udp bool) *Base {
	return &Base{name, addr, tp, udp}
}

type conn struct {
	net.Conn
	chain      C.Chain
	raddr      *net.UDPAddr
	lastUpdate time.Time
	canRaw     bool
}

func (c *conn) RawConn() net.Conn {
	if c.canRaw {
		return c.Conn
	} else {
		return nil
	}
}

// Chains implements C.Connection
func (c *conn) Chains() C.Chain {
	return c.chain
}

// AppendToChains implements C.Connection
func (c *conn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

// RUDPAddr implements C.Connection
func (c *conn) Raddr() *net.UDPAddr {
	return c.raddr
}

func (c *conn) SetRaddr(addr *net.UDPAddr) {
	c.raddr = addr
}

func (pc *conn) NeedUpdateDeadline(timeout time.Duration) bool {
	// 允许极小概率的并发读写冲突，在高频 UDP 下完全可以接受
	return time.Since(pc.lastUpdate) > (timeout / 3)
}

func (pc *conn) UpdateLastUpdate() {
	pc.lastUpdate = time.Now()
}

func (pc *conn) GetLastUpdate() time.Time {
	return pc.lastUpdate
}

func NewConn(c net.Conn, a C.ProxyAdapter) C.Conn {
	//canRaw := false
	//switch a.Type() {
	//case C.Direct:
	//	canRaw = true
	//case C.Http:
	//	canRaw = true
	//case C.Socks5:
	//	canRaw = true // Socks5 可以直接使用 RawConn
	//default:
	//	canRaw = false
	//}
	return &conn{c, []string{a.Name()}, nil, time.Now(), true}
}

type packetConn struct {
	net.PacketConn
	chain      C.Chain
	raddr      *net.UDPAddr
	lastUpdate time.Time
}

// Chains implements C.Connection
func (c *packetConn) Chains() C.Chain {
	return c.chain
}

// AppendToChains implements C.Connection
func (c *packetConn) AppendToChains(a C.ProxyAdapter) {
	c.chain = append(c.chain, a.Name())
}

func (c *packetConn) Raddr() *net.UDPAddr {
	return c.raddr
}

func (c *packetConn) SetRaddr(addr *net.UDPAddr) {
	c.raddr = addr
}

func (pc *packetConn) NeedUpdateDeadline(timeout time.Duration) bool {
	// 允许极小概率的并发读写冲突，在高频 UDP 下完全可以接受
	return time.Since(pc.lastUpdate) > (timeout / 3)
}

func (pc *packetConn) UpdateLastUpdate() {
	pc.lastUpdate = time.Now()
}

func (pc *packetConn) GetLastUpdate() time.Time {
	return pc.lastUpdate
}

func newPacketConn(pc net.PacketConn, a C.ProxyAdapter) C.PacketConn {
	return &packetConn{pc, []string{a.Name()}, nil, time.Now()}
}
