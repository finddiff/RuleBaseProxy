package forward

import (
	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	"github.com/finddiff/RuleBaseProxy/common/pool"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/context"
	log "github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel"
	"github.com/xjasonlyu/tun2socks/v2/core/adapter"
	tunTunnel "github.com/xjasonlyu/tun2socks/v2/tunnel"
	"net"
	"strconv"
)

var (
	_     adapter.TransportHandler = (*Tunnel)(nil)
	TcpIn chan<- C.ConnContext
	UdpIn chan<- *inbound.PacketAdapter
)

type Tunnel struct{}

func (*Tunnel) nHandleTCP(conn adapter.TCPConn) {
	tunTunnel.TCPIn() <- conn
}

func (*Tunnel) nHandleUDP(conn adapter.UDPConn) {
	tunTunnel.UDPIn() <- conn
}

func (*Tunnel) HandleTCP(conn adapter.TCPConn) {
	//log.Infoln("tunHandleTCPConn %v", conn)
	//conn.LocalAddr().String()
	//lhost, lport, _ := net.SplitHostPort(conn.LocalAddr().String())
	//lip := net.ParseIP(lhost)
	//rhost, rport, _ := net.SplitHostPort(conn.RemoteAddr().String())
	//rip := net.ParseIP(rhost)

	id := conn.ID()
	metadata := &C.Metadata{
		AddrType: C.AtypIPv4,
		NetWork:  C.TCP,
		Type:     C.WINTUN,
		SrcIP:    net.IP(id.RemoteAddress.AsSlice()),
		SrcPort:  strconv.Itoa(int(id.RemotePort)),
		DstIP:    net.IP(id.LocalAddress.AsSlice()),
		DstPort:  strconv.Itoa(int(id.LocalPort)),
	}
	if metadata.DstIP.To4() == nil {
		metadata.AddrType = C.AtypIPv6
	}

	log.Debugln("tunHandleTCPConn metadata:%v", metadata)
	TcpIn <- context.NewConnContext(conn, metadata)
}

type packet struct {
	pc    net.PacketConn
	rAddr net.Addr
	//payload []byte
	bufRef []byte
}

func (c *packet) Data() []byte {
	return c.bufRef
}

// WriteBack write UDP packet with source(ip, port) = `addr`
func (c *packet) WriteBack(b []byte, addr net.Addr) (n int, err error) {
	log.Debugln("to Tunnel WriteBack:%v", addr)
	return c.pc.WriteTo(b, nil)
}

// LocalAddr returns the source IP/Port of UDP Packet
func (c *packet) LocalAddr() net.Addr {
	return c.rAddr
}

func (c *packet) Drop() {
	pool.Put(c.bufRef)
}

func (*Tunnel) HandleUDP(conn adapter.UDPConn) {
	go func() {
		id := conn.ID()
		metadata := &C.Metadata{
			AddrType: C.AtypIPv4,
			NetWork:  C.UDP,
			Type:     C.WINTUN,
			SrcIP:    net.IP(id.RemoteAddress.AsSlice()),
			SrcPort:  strconv.Itoa(int(id.RemotePort)),
			DstIP:    net.IP(id.LocalAddress.AsSlice()),
			DstPort:  strconv.Itoa(int(id.LocalPort)),
		}

		if metadata.DstIP.To4() == nil {
			metadata.AddrType = C.AtypIPv6
		}

		log.Debugln("tunHandleUDPConn %v", metadata)
		var pc C.PacketConn = nil
		raddr := &net.UDPAddr{
			IP:   metadata.DstIP,
			Port: int(id.RemotePort),
		}

		for {
			buf := pool.Get(pool.MaxSegmentSize)
			n, remoteAddr, err := conn.ReadFrom(buf)
			if err != nil {
				pool.Put(buf)
				conn.Close()
				return
			}
			if pc == nil {
				pc = tunnel.GetnatPC(remoteAddr.String())
			}

			if pc != nil {
				if _, err = pc.WriteTo(buf[:n], raddr); err != nil {
					pool.Put(buf)
					conn.Close()
					return
				}
				continue
			} else {
				pack := &inbound.PacketAdapter{
					UDPPacket: &packet{
						pc:     conn,
						rAddr:  remoteAddr,
						bufRef: buf[:n],
					},
				}
				pack.SetMetadata(metadata)
				UdpIn <- pack
			}
		}
	}()
}
