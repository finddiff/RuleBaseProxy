package tunnel

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/finddiff/RuleBaseProxy/common/pool"
	"github.com/finddiff/RuleBaseProxy/component/resolver"
	C "github.com/finddiff/RuleBaseProxy/constant"
)

func handleUDPToRemote(packet C.UDPPacket, pc C.PacketConn, metadata *C.Metadata, key string) error {
	defer packet.Drop()

	// local resolve UDP dns
	if !metadata.Resolved() {
		ip, err := resolver.ResolveIP(metadata.Host)
		if err != nil {
			return err
		}
		if metadata.DstIP == nil {
			metadata.DstIP = ip
		}
	}

	addr := metadata.UDPAddr()
	if addr == nil {
		return errors.New("udp addr invalid")
	}

	if _, err := pc.WriteTo(packet.Data(), addr); err != nil {
		natTable.Delete(key)
		pc.Close()
		return err
	}
	// reset timeout
	//pc.SetReadDeadline(time.Now().Add(udpTimeout))
	pc.SetDeadline(time.Now().Add(udpTimeout))

	return nil
}

func handleUDPToLocal(packet C.UDPPacket, pc net.PacketConn, key string, fAddr net.Addr) {
	buf := pool.Get(pool.RelayBufferSize)
	defer pool.Put(buf)
	defer natTable.Delete(key)
	defer pc.Close()

	for {
		//pc.SetReadDeadline(time.Now().Add(udpTimeout))
		pc.SetDeadline(time.Now().Add(udpTimeout))
		n, from, err := pc.ReadFrom(buf)
		if err != nil {
			return
		}

		if fAddr != nil {
			from = fAddr
		}

		_, err = packet.WriteBack(buf[:n], from)
		if err != nil {
			return
		}
	}
}

type IdleTimeoutConn struct {
	Conn    net.Conn
	Timeout time.Duration
}

func handleSocket(ctx C.ConnContext, outbound net.Conn) {
	relay(ctx.Conn(), outbound)
}

type ReadOnlyReader struct {
	io.Reader
}

type WriteOnlyWriter struct {
	io.Writer
}

// relay copies between left and right bidirectionally.
func relay(leftConn, rightConn net.Conn) {
	ch := make(chan error)

	go func() {
		// Wrapping to avoid using *net.TCPConn.(ReadFrom)
		// See also https://github.com/Dreamacro/clash/pull/1209
		_, err := io.Copy(WriteOnlyWriter{Writer: leftConn}, ReadOnlyReader{Reader: rightConn})
		leftConn.SetReadDeadline(time.Now())
		leftConn.Close()
		ch <- err
	}()

	io.Copy(WriteOnlyWriter{Writer: rightConn}, ReadOnlyReader{Reader: leftConn})
	rightConn.SetReadDeadline(time.Now())
	rightConn.Close()
	<-ch
}

//func relay(leftConn, rightConn net.Conn) {
//	//defer rightConn.Close()
//	//log.Infoln("relay leftConn:%v, rightConn:%v", leftConn, rightConn)
//	//clientDoneCh, serverDoneCh := make(chan struct{}), make(chan struct{})
//	//go relayCopy(leftConn, rightConn, serverDoneCh)
//	//go relayCopy(rightConn, leftConn, clientDoneCh)
//	//wait(clientDoneCh, serverDoneCh, leftConn, rightConn)
//	ch := make(chan error)
//
//	go func() {
//		buf := pool.Get(pool.RelayBufferSize)
//		// Wrapping to avoid using *net.TCPConn.(ReadFrom)
//		// See also https://github.com/finddiff/RuleBaseProxy/pull/1209
//		_, err := io.CopyBuffer(N.WriteOnlyWriter{Writer: leftConn}, N.ReadOnlyReader{Reader: rightConn}, buf)
//		pool.Put(buf)
//		leftConn.SetReadDeadline(time.Now())
//		ch <- err
//	}()
//
//	buf := pool.Get(pool.RelayBufferSize)
//	io.CopyBuffer(N.WriteOnlyWriter{Writer: rightConn}, N.ReadOnlyReader{Reader: leftConn}, buf)
//	pool.Put(buf)
//	rightConn.SetReadDeadline(time.Now())
//	<-ch
//}

//func relayCopy(det net.Conn, src net.Conn, done chan struct{}) {
//	buf := pool.Get(pool.RelayBufferSize)
//	_, _ = io.CopyBuffer(det, src, buf)
//	pool.Put(buf)
//	det.SetReadDeadline(time.Now())
//	//log.Infoln("relayCopy src:%v,  done:%v", src, done)
//	close(done)
//}
//
//func wait(clientDoneCh chan struct{}, serverDoneCh chan struct{}, leftConn, rightConn net.Conn) {
//	select {
//	case <-clientDoneCh:
//		//log.Infoln("wait rightConn.close:%v", rightConn)
//		rightConn.Close()
//	case <-serverDoneCh:
//		//log.Infoln("wait leftConn.close:%v", leftConn)
//		leftConn.Close()
//	}
//}
