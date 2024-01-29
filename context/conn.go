package context

import (
	"github.com/finddiff/RuleBaseProxy/tunnel/statistic"
	"net"

	C "github.com/finddiff/RuleBaseProxy/constant"

	"github.com/gofrs/uuid"
)

type ConnContext struct {
	id       uuid.UUID
	metadata *C.Metadata
	conn     net.Conn
	tcp      C.Conn
	udp      C.PacketConn
}

func NewConnContext(conn net.Conn, metadata *C.Metadata) *ConnContext {
	id, _ := uuid.NewV4()
	conCont := &ConnContext{
		id:       id,
		metadata: metadata,
		conn:     conn,
	}

	if metadata.NetWork == C.TCP {
		tcpTrack := statistic.NewTCPTracker(nil, statistic.DefaultManager, metadata, nil, nil)
		statistic.DefaultManager.Join(tcpTrack)
		conCont.tcp = tcpTrack
	}

	return conCont
	//return &ConnContext{
	//	id:       id,
	//	metadata: metadata,
	//	conn:     conn,
	//}
}

// ID implement C.ConnContext ID
func (c *ConnContext) ID() uuid.UUID {
	return c.id
}

// Metadata implement C.ConnContext Metadata
func (c *ConnContext) Metadata() *C.Metadata {
	return c.metadata
}

// Conn implement C.ConnContext Conn
func (c *ConnContext) Conn() net.Conn {
	return c.conn
}

// Tracker implement C.ConnContext tcp
func (c *ConnContext) Tracker() C.Conn {
	return c.tcp
}
