package tunnel

import (
	"net"
	"sync/atomic"
	"time"
)

type IdleConn struct {
	net.Conn
	timeout     time.Duration
	lastUpdated atomic.Int64 // 存储 Unix 纳秒时间戳
}

func NewIdleConn(conn net.Conn, timeout time.Duration) *IdleConn {
	c := &IdleConn{
		Conn:    conn,
		timeout: timeout,
	}
	// 初始化设置第一次 Deadline
	now := time.Now()
	c.lastUpdated.Store(now.UnixNano())
	//_ = conn.SetDeadline(now.Add(timeout))
	return c
}

func (c *IdleConn) RawConn() net.Conn {
	return c.Conn
}

func (c *IdleConn) StartDeadline() {
	_ = c.Conn.SetDeadline(time.Now().Add(c.timeout))
}

func (c *IdleConn) updateDeadline() {
	nowNano := time.Now().UnixNano()
	lastNano := c.lastUpdated.Load()

	// 阈值检查：超过 timeout/4 时才尝试更新
	if nowNano-lastNano > int64(c.timeout/4) {
		// 使用 CAS 确保只有一个 Goroutine 能触发系统调用更新 Deadline
		if c.lastUpdated.CompareAndSwap(lastNano, nowNano) {
			_ = c.Conn.SetDeadline(time.Now().Add(c.timeout))
		}
	}
}

func (c *IdleConn) Read(b []byte) (n int, err error) {
	n, err = c.Conn.Read(b)
	if err == nil {
		c.updateDeadline()
	}
	return
}

func (c *IdleConn) Write(b []byte) (n int, err error) {
	n, err = c.Conn.Write(b)
	if err == nil {
		c.updateDeadline()
	}
	return
}
