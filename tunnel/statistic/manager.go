package statistic

import (
	"sync"
	"time"
)

var DefaultManager *Manager

func init() {
	DefaultManager = &Manager{
		//uploadTemp:    atomic.NewInt64(0),
		//downloadTemp:  atomic.NewInt64(0),
		//uploadBlip:    atomic.NewInt64(0),
		//downloadBlip:  atomic.NewInt64(0),
		//uploadTotal:   atomic.NewInt64(0),
		//downloadTotal: atomic.NewInt64(0),

		uploadTemp:    0,
		downloadTemp:  0,
		uploadBlip:    0,
		downloadBlip:  0,
		uploadTotal:   0,
		downloadTotal: 0,
	}

	go DefaultManager.handle()
	go DefaultManager.end_errors_conn()
}

type Manager struct {
	connections sync.Map
	//uploadTemp    *atomic.Int64
	//downloadTemp  *atomic.Int64
	//uploadBlip    *atomic.Int64
	//downloadBlip  *atomic.Int64
	//uploadTotal   *atomic.Int64
	//downloadTotal *atomic.Int64

	uploadTemp    int64
	downloadTemp  int64
	uploadBlip    int64
	downloadBlip  int64
	uploadTotal   int64
	downloadTotal int64
}

func (m *Manager) Join(c tracker) {
	m.connections.Store(c.ID(), c)
}

func (m *Manager) Leave(c tracker) {
	m.connections.Delete(c.ID())
}

func (m *Manager) PushUploaded(size int64) {
	//m.uploadTemp.Add(size)
	//m.uploadTotal.Add(size)
}

func (m *Manager) PushDownloaded(size int64) {
	//m.downloadTemp.Add(size)
	//m.downloadTotal.Add(size)
}

func (m *Manager) Now() (up int64, down int64) {
	//return m.uploadBlip.Load(), m.downloadBlip.Load()
	return m.uploadBlip, m.downloadBlip
}

func (m *Manager) Snapshot() *Snapshot {
	connections := []tracker{}
	m.connections.Range(func(key, value interface{}) bool {
		connections = append(connections, value.(tracker))
		return true
	})

	//return &Snapshot{
	//	UploadTotal:   m.uploadTotal.Load(),
	//	DownloadTotal: m.downloadTotal.Load(),
	//	Connections:   connections,
	//}

	return &Snapshot{
		UploadTotal:   m.uploadTotal,
		DownloadTotal: m.downloadTotal,
		Connections:   connections,
	}
}

func (m *Manager) ResetStatistic() {
	m.uploadTemp = 0
	m.uploadBlip = 0
	m.uploadTotal = 0
	m.downloadTemp = 0
	m.downloadBlip = 0
	m.downloadTotal = 0
}

func (m *Manager) handle() {
	ticker := time.NewTicker(time.Second)

	for range ticker.C {
		//m.uploadBlip.Store(m.uploadTemp.Load())
		//m.uploadTemp.Store(0)
		//m.downloadBlip.Store(m.downloadTemp.Load())
		//m.downloadTemp.Store(0)

		allTempUpload := int64(0)
		upload := int64(0)
		allTemDownLoad := int64(0)
		download := int64(0)
		m.connections.Range(func(key, value interface{}) bool {
			tinfo := value.(tracker).TrackerInfo()
			//upload = tinfo.UploadTotal.Load()
			upload = tinfo.UploadTotal - tinfo.TempUpload
			allTempUpload += upload
			tinfo.TempUpload += upload

			//download = tinfo.DownloadTotal.Load()
			download = tinfo.DownloadTotal - tinfo.TempDownload
			allTemDownLoad += download
			tinfo.TempDownload += download
			return true
		})

		//m.uploadBlip.Store(allTempUpload)
		//m.downloadBlip.Store(allTemDownLoad)
		//m.uploadTotal.Store(m.uploadTotal.Load() + allTempUpload)
		//m.downloadTotal.Store(m.downloadTotal.Load() + allTemDownLoad)

		m.uploadBlip = allTempUpload
		m.downloadBlip = allTemDownLoad
		m.uploadTotal += allTempUpload
		m.downloadTotal += allTemDownLoad
	}
}

func (m *Manager) end_errors_conn() {
	ticker := time.NewTicker(time.Second * 3)

	for range ticker.C {
		m.connections.Range(func(key, value interface{}) bool {
			tinfo := value.(tracker).TrackerInfo()
			if tinfo.Chain[len(tinfo.Chain)-1] == "ERROR" {
				//对于 出现error的连接回出现两次调用Close，第一次设置回收标志MarkGC，第二次进行关闭 回收时间最短3s 最长6s
				err := value.(tracker).Close()
				if err != nil {
					return true
				}
			}
			return true
		})
	}
}

type Snapshot struct {
	DownloadTotal int64     `json:"downloadTotal"`
	UploadTotal   int64     `json:"uploadTotal"`
	Connections   []tracker `json:"connections"`
}
