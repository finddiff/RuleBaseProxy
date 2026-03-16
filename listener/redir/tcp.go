package redir

import (
	"github.com/finddiff/RuleBaseProxy/log"
	"net"

	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	C "github.com/finddiff/RuleBaseProxy/constant"
)

type Listener struct {
	listener net.Listener
	addr     string
	closed   bool
}

// RawAddress implements C.Listener
func (l *Listener) RawAddress() string {
	return l.addr
}

// Address implements C.Listener
func (l *Listener) Address() string {
	return l.listener.Addr().String()
}

// Close implements C.Listener
func (l *Listener) Close() error {
	l.closed = true
	return l.listener.Close()
}

func New(addr string, in chan<- C.ConnContext) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	rl := &Listener{
		listener: l,
		addr:     addr,
	}

	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if rl.closed {
					break
				}
				continue
			}
			if tcpConn, ok := c.(*net.TCPConn); ok {
				// 1. 禁用 Nagle 算法，消除 40ms 延迟等待
				log.Debugln("handleRedir set NoDelay=true")
				tcpConn.SetNoDelay(true)
			} else {
				log.Debugln("handleRedir set NoDelay=false")
			}
			go handleRedir(c, in)
		}
	}()

	return rl, nil
}

func handleRedir(conn net.Conn, in chan<- C.ConnContext) {
	target, err := parserPacket(conn)
	if err != nil {
		conn.Close()
		return
	}
	conn.(*net.TCPConn).SetKeepAlive(true)
	in <- inbound.NewSocket(target, conn, C.REDIR)
}
