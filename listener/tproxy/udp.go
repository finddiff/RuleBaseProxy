package tproxy

import (
	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	"github.com/finddiff/RuleBaseProxy/common/pool"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/transport/socks5"
	"net"
)

type UDPListener struct {
	packetConn net.PacketConn
	addr       string
	closed     bool
}

// RawAddress implements C.Listener
func (l *UDPListener) RawAddress() string {
	return l.addr
}

// Address implements C.Listener
func (l *UDPListener) Address() string {
	return l.packetConn.LocalAddr().String()
}

// Close implements C.Listener
func (l *UDPListener) Close() error {
	l.closed = true
	return l.packetConn.Close()
}

func NewUDP(addr string, in chan<- *inbound.PacketAdapter) (*UDPListener, error) {
	l, err := net.ListenPacket("udp", addr)
	if err != nil {
		return nil, err
	}

	rl := &UDPListener{
		packetConn: l,
		addr:       addr,
	}

	c := l.(*net.UDPConn)

	rc, err := c.SyscallConn()
	if err != nil {
		return nil, err
	}
	//fmt.Println("tproxy listener udp type:", reflect.TypeOf(rc).String()) *net.rawConn
	err = setsockopt(rc, addr)
	if err != nil {
		return nil, err
	}

	go func() {
		oob := make([]byte, 1024)
		for {
			buf := pool.Get(pool.RelayBufferSize)
			n, oobn, _, lAddr, err := c.ReadMsgUDP(buf, oob)
			if err != nil {
				pool.Put(buf)
				if rl.closed {
					break
				}
				continue
			}

			rAddr, err := getOrigDst(oob, oobn)
			if err != nil {
				continue
			}
			handlePacketConn(l, in, buf[:n], lAddr, rAddr)
		}
	}()

	return rl, nil
}

func handlePacketConn(pc net.PacketConn, in chan<- *inbound.PacketAdapter, buf []byte, lAddr *net.UDPAddr, rAddr *net.UDPAddr) {
	target := socks5.ParseAddrToSocksAddr(rAddr)
	pkt := &packet{
		lAddr: lAddr,
		buf:   buf,
	}
	select {
	case in <- inbound.NewPacket(target, pkt, C.TPROXY):
	default:
	}
}
