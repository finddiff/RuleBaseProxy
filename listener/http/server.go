package http

import (
	"github.com/finddiff/RuleBaseProxy/log"
	"net"
	"time"

	"github.com/finddiff/RuleBaseProxy/common/cache"
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
	return NewWithAuthenticate(addr, in, true)
}

func NewWithAuthenticate(addr string, in chan<- C.ConnContext, authenticate bool) (*Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	var c *cache.Cache
	if authenticate {
		c = cache.New(time.Second * 30)
	}

	hl := &Listener{
		listener: l,
		addr:     addr,
	}
	go func() {
		for {
			conn, err := hl.listener.Accept()
			if err != nil {
				if hl.closed {
					break
				}
				continue
			}

			if tcpConn, ok := conn.(*net.TCPConn); ok {
				// 1. 禁用 Nagle 算法，消除 40ms 延迟等待
				log.Debugln("HandleConn set NoDelay=true")
				tcpConn.SetNoDelay(true)
			} else {
				log.Debugln("HandleConn set NoDelay=false")
			}

			go HandleConn(conn, in, c)
		}
	}()

	return hl, nil
}
