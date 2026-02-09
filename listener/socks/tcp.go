package socks

import (
	"net"
	"time"

	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	N "github.com/finddiff/RuleBaseProxy/common/net"
	C "github.com/finddiff/RuleBaseProxy/constant"
	authStore "github.com/finddiff/RuleBaseProxy/listener/auth"
	"github.com/finddiff/RuleBaseProxy/transport/socks4"
	"github.com/finddiff/RuleBaseProxy/transport/socks5"
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

	sl := &Listener{
		listener: l,
		addr:     addr,
	}
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				if sl.closed {
					break
				}
				continue
			}
			go handleSocks(c, in)
		}
	}()

	return sl, nil
}

func handleSocks(conn net.Conn, in chan<- C.ConnContext) {
	bufConn := N.NewBufferedConn(conn)
	head, err := bufConn.Peek(1)
	if err != nil {
		conn.Close()
		return
	}

	switch head[0] {
	case socks4.Version:
		HandleSocks4(bufConn, in)
	case socks5.Version:
		HandleSocks5(bufConn, in)
	default:
		conn.Close()
	}
}

func HandleSocks4(conn net.Conn, in chan<- C.ConnContext) {
	addr, _, err := socks4.ServerHandshake(conn, authStore.Authenticator())
	if err != nil {
		conn.Close()
		return
	}
	if c, ok := conn.(*net.TCPConn); ok {
		c.SetKeepAlive(true)
	}
	in <- inbound.NewSocket(socks5.ParseAddr(addr), conn, C.SOCKS4)
}

func waitUDPControlConn(conn net.Conn, idleTimeout time.Duration) {
	defer conn.Close()

	// 1. 设置读取超时
	// 如果 idleTimeout 内没有任何数据流进（包括 FIN 包），Read 将返回 timeout 错误
	if idleTimeout > 0 {
		_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
	}

	// 2. 使用小缓冲区进行读取，减少内存占用
	// 不需要 io.Copy 那么大的缓冲区（默认 32KB），UDP 控制连接通常没有数据流
	buf := make([]byte, 128)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			// 如果是超时、EOF 或连接关闭，则退出循环
			return
		}
		// 如果客户端竟然发了数据（不符合协议预期），可以重置超时时间
		if idleTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
		}
	}
}

func HandleSocks5(conn net.Conn, in chan<- C.ConnContext) {
	target, command, err := socks5.ServerHandshake(conn, authStore.Authenticator())
	if err != nil {
		conn.Close()
		return
	}
	if c, ok := conn.(*net.TCPConn); ok {
		c.SetKeepAlive(true)
		c.SetKeepAlivePeriod(30 * time.Second)
	}
	if command == socks5.CmdUDPAssociate {
		defer conn.Close()
		waitUDPControlConn(conn, 3*time.Minute)
		//io.Copy(io.Discard, conn)
		return
	}
	in <- inbound.NewSocket(target, conn, C.SOCKS5)
}
