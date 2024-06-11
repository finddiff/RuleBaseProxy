package nat

import (
	C "github.com/finddiff/RuleBaseProxy/constant"
	"sync"
	"time"
)

type Table struct {
	mapping sync.Map
	TimeMap sync.Map
}

func (t *Table) Set(key string, pc C.PacketConn) {
	t.mapping.Store(key, pc)
}

func (t *Table) Get(key string) C.PacketConn {
	item, exist := t.mapping.Load(key)
	if !exist {
		return nil
	}
	return item.(C.PacketConn)
}

func (t *Table) GetOrCreateLock(key string) (*sync.Cond, bool) {
	item, loaded := t.mapping.LoadOrStore(key, sync.NewCond(&sync.Mutex{}))
	return item.(*sync.Cond), loaded
}

func (t *Table) Delete(key string) {
	t.mapping.Delete(key)
}

func (t *Table) SetEndTime(key string, endtime time.Time) {
	t.TimeMap.Store(key, endtime)
}

func (t *Table) GetEndTime(key string, endtime time.Time) time.Time {
	item, exist := t.TimeMap.Load(key)
	if !exist {
		return time.Now().Add(60 * time.Second)
	}
	return item.(time.Time)
}

func (t *Table) DeleteEndTime(key string) {
	t.TimeMap.Delete(key)
}

// New return *Cache
func New() *Table {
	return &Table{}
}
