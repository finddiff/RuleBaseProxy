package statistic

import (
	"net"
	"time"

	C "github.com/finddiff/RuleBaseProxy/constant"

	"github.com/gofrs/uuid"
)

type tracker interface {
	ID() string
	Close() error
	TrackerInfo() *trackerInfo
}

type trackerInfo struct {
	UUID          uuid.UUID   `json:"id"`
	Metadata      *C.Metadata `json:"metadata"`
	UploadTotal   int64       `json:"upload"`
	TempUpload    int64
	DownloadTotal int64 `json:"download"`
	TempDownload  int64
	//UploadTotal   *atomic.Int64 `json:"upload"`
	//DownloadTotal *atomic.Int64 `json:"download"`
	Start       time.Time `json:"start"`
	Chain       C.Chain   `json:"chains"`
	Rule        string    `json:"rule"`
	RulePayload string    `json:"rulePayload"`
}

type tcpTracker struct {
	C.Conn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (tt *tcpTracker) ID() string {
	return tt.UUID.String()
}

func (tt *tcpTracker) TrackerInfo() *trackerInfo {
	return tt.trackerInfo
}

func (tt *tcpTracker) Read(b []byte) (int, error) {
	n, err := tt.Conn.Read(b)
	download := int64(n)
	//tt.manager.PushDownloaded(download)
	//tt.DownloadTotal.Add(download)
	tt.DownloadTotal += download
	//tt.TempDownload += download
	return n, err
}

func (tt *tcpTracker) Write(b []byte) (int, error) {
	n, err := tt.Conn.Write(b)
	upload := int64(n)
	//tt.manager.PushUploaded(upload)
	//tt.UploadTotal.Add(upload)
	tt.UploadTotal += upload
	//tt.TempUpload += upload
	return n, err
}

func (tt *tcpTracker) Close() error {
	go func() {
		// 增加延迟时间，在3秒后关闭，在web端能观察到异常关闭情况。
		if time.Now().Sub(tt.Start) < time.Duration(3)*time.Second {
			time.Sleep(time.Duration(3) * time.Second)
		}
		tt.manager.Leave(tt)
	}()

	if tt.Conn == nil {
		return nil
	}
	err := tt.Conn.Close()
	return err
}

func Conn2TCPTracker(conn C.Conn) *tcpTracker {
	if conn == nil {
		return nil
	}
	return conn.(*tcpTracker)
}

func NewTCPTracker(conn C.Conn, manager *Manager, metadata *C.Metadata, rule C.Rule, proxy C.Proxy) *tcpTracker {
	uuid, _ := uuid.NewV4()

	t := &tcpTracker{
		Conn:    conn,
		manager: manager,
		trackerInfo: &trackerInfo{
			UUID:     uuid,
			Start:    time.Now(),
			Metadata: metadata,
			//Chain:         conn.Chains(),
			Rule: "",
			//UploadTotal:   atomic.NewInt64(0),
			//DownloadTotal: atomic.NewInt64(0),

			UploadTotal:   0,
			TempUpload:    0,
			DownloadTotal: 0,
			TempDownload:  0,
		},
	}

	if conn != nil {
		t.Chain = conn.Chains()
	} else {
		if proxy != nil {
			t.Chain = []string{"DISP", proxy.Name()}
		} else {
			t.Chain = []string{"INIT"}
		}

	}

	if rule != nil {
		t.trackerInfo.Rule = rule.RuleType().String()
		t.trackerInfo.RulePayload = rule.Payload()
	}

	manager.Join(t)
	return t
}

type udpTracker struct {
	C.PacketConn `json:"-"`
	*trackerInfo
	manager *Manager
}

func (ut *udpTracker) ID() string {
	return ut.UUID.String()
}

func (ut *udpTracker) TrackerInfo() *trackerInfo {
	return ut.trackerInfo
}

func (ut *udpTracker) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := ut.PacketConn.ReadFrom(b)
	download := int64(n)
	//ut.manager.PushDownloaded(download)
	//ut.DownloadTotal.Add(download)
	ut.DownloadTotal += download
	//ut.TempDownload += download
	return n, addr, err
}

func (ut *udpTracker) WriteTo(b []byte, addr net.Addr) (int, error) {
	n, err := ut.PacketConn.WriteTo(b, addr)
	upload := int64(n)
	//ut.manager.PushUploaded(upload)
	//ut.UploadTotal.Add(upload)
	ut.UploadTotal += upload
	//ut.TempUpload += upload
	return n, err
}

func (ut *udpTracker) Close() error {
	go func() {
		// 增加延迟时间，在3秒后关闭，在web端能观察到异常关闭情况。
		if time.Now().Sub(ut.Start) < time.Duration(3)*time.Second {
			time.Sleep(time.Duration(3) * time.Second)
		}
		ut.manager.Leave(ut)
	}()

	if ut.PacketConn == nil {
		return nil
	}
	err := ut.PacketConn.Close()
	return err
}

func NewUDPTracker(conn C.PacketConn, manager *Manager, metadata *C.Metadata, rule C.Rule, proxy C.Proxy) *udpTracker {
	uuid, _ := uuid.NewV4()

	ut := &udpTracker{
		PacketConn: conn,
		manager:    manager,
		trackerInfo: &trackerInfo{
			UUID:     uuid,
			Start:    time.Now(),
			Metadata: metadata,
			//Chain:         conn.Chains(),
			Rule: "",
			//UploadTotal:   atomic.NewInt64(0),
			//DownloadTotal: atomic.NewInt64(0),

			UploadTotal:   0,
			TempUpload:    0,
			DownloadTotal: 0,
			TempDownload:  0,
		},
	}

	if conn != nil {
		ut.Chain = conn.Chains()
	} else {
		if proxy != nil {
			ut.Chain = []string{"PRELOAD", proxy.Name()}
		} else {
			ut.Chain = []string{"INIT"}
		}
	}

	if rule != nil {
		ut.trackerInfo.Rule = rule.RuleType().String()
		ut.trackerInfo.RulePayload = rule.Payload()
	}

	manager.Join(ut)
	return ut
}
