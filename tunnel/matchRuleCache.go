package tunnel

import (
	"fmt"
	"time"

	//HS "github.com/cornelk/hashmap"
	"github.com/dgraph-io/ristretto"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel/statistic"
	//"golang.org/x/sync/syncmap"
	"net"
)

var (
	//Cm *concurrent_map.ConcurrentMap
	//Cm     = CMAP.New()
	//Cm = CC.New(CC.Configure().MaxSize(1024 * 128).ItemsToPrune(500).Buckets(1024 * 128 / 64))
	//Dm = CC.New(CC.Configure().MaxSize(1024 * 128).ItemsToPrune(500).Buckets(1024 * 128 / 64))
	Cm, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 28, // maximum cost of cache (256MB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	Dm, _ = ristretto.NewCache(&ristretto.Config{
		NumCounters: 1e7,     // number of keys to track frequency of (10M).
		MaxCost:     1 << 28, // maximum cost of cache (256MB).
		BufferItems: 64,      // number of keys per Get buffer.
	})
	//Am = &HS.HashMap{}
	//Bm = syncmap.Map{}
	//Am.Set("amount", 123)
	//TimeCm = CMAP.New()
	//RulChan = make(chan string, 10000)
	//Cm = concurrent_map.CreateConcurrentMap(1024)
)

func DnsPreCache(domain string, ip string, remote net.Addr, ttl uint32) {
	//log.Debugln("DnsPreCache remote:%s ", remote.String())
	//addr, err := net.ResolveTCPAddr("tcp", remote.String())
	//if err != nil {
	//	log.Debugln("DnsPreCache ResolveTCPAddr remote:%s err:%v", remote.String(), err)
	//	return
	//}
	//log.Debugln("DnsPreCache addr:%s not cache", addr.IP.String())
	//if addr.IP.String() == "127.0.0.1" {
	//	log.Debugln("DnsPreCache addr:%s not cache", addr.IP.String())
	//	return
	//}
	//metadata := &C.Metadata{
	//	NetWork:  C.TCP,
	//	Type:     C.SOCKS5,
	//	SrcIP:    addr.IP,
	//	DstIP:    net.ParseIP(ip),
	//	SrcPort:  "",
	//	DstPort:  "80",
	//	AddrType: C.AtypDomainName,
	//	Host:     domain,
	//}
	//adapter, hashRule, err := matchHashMap(metadata)
	//metadata.SrcPort = ""
	//metadata.DstPort = "443"
	//adapter, hashRule, err = matchHashMap(metadata)
	//if dur, ok := Dm.GetTTL(ip); ok && dur.Seconds() > 10 {
	//	return
	//}
	Dm.SetWithTTL(ip, domain, 1, time.Minute*60*24)
	//Dm.SetWithTTL(ip, domain, 1, time.Duration(ttl)*time.Second)
	log.Debugln("DnsPreCache cache ip:%v, domain%s", ip, domain)
	//Dm.Set(ip, domain, time.Hour*24)
	//TimeCm.Set(domain, 0)
	//log.Debugln("DnsPreCache call return domain:%s,adapter:%s,hashRule:%v,err:%v", domain, adapter.Name(), hashRule, err)
}

func setMatchHashMap(key string, value interface{}) {
	//Cm.Set(key, value, time.Minute*60*24)
	Cm.SetWithTTL(key, value, 1, time.Minute*60*24)
	//Bm.Store(key, value)
	//Am.Set(key, value)
	//Cm.Set(key, value)
	//TimeCm.Set(key, 0)
}

func CloseRuleMatchCon(rule C.Rule) {
	snapshot := statistic.DefaultManager.Snapshot()
	for _, c := range snapshot.Connections {
		if rule.Match(c.TrackerInfo().Metadata) {
			log.Debugln("CloseRuleMatchCon Rule:%v, Connect:%v", rule, c.TrackerInfo().Metadata)
			c.Close()
		}
	}
}

func GetnatPC(key string) C.PacketConn {
	pc := natTable.Get(key)
	if pc != nil {
		//handleUDPToRemote(packet, pc, metadata)
		return pc
	}

	return nil
}

func matchHashMap(metadata *C.Metadata) (adapter C.Proxy, hashRule C.Rule, err error) {
	domainStr := fmt.Sprintf("%v:%v %v %v", metadata, metadata.DstPort, metadata.SrcIP, metadata.NetWork)

	log.Debugln("matchHashMap Cm.Get(domainStr) domainStr=%s", domainStr)
	if hashValue, ok := Cm.Get(domainStr); ok && hashValue != nil {
		//hashValue := item.Value()
		//hashValue := item
		switch hashValue.(type) {
		case C.Rule:
			hashRule := hashValue.(C.Rule)
			adapter, ok := proxies[hashValue.(C.Rule).Adapter()]
			if hashRule.Match(metadata) && ok {
				if metadata.NetWork == C.TCP {
					log.Debugln("matchHashMap match domainStr=%s adapter=%v, hashRule=%v, err=%v ", domainStr, adapter, hashRule, err)
					return adapter, hashRule, nil
				}
				if metadata.NetWork == C.UDP && adapter.SupportUDP() {
					log.Debugln("matchHashMap match domainStr=%s adapter=%v, hashRule=%v, err=%v ", domainStr, adapter, hashRule, err)
					return adapter, hashRule, nil
				}
			}
		}
	}

	proxyStr := fmt.Sprintf("%v:%v %v %v %v", metadata, metadata.DstPort, metadata.SrcIP, metadata.SrcPort, metadata.NetWork)
	log.Debugln("matchHashMap Cm.Get(proxyStr) proxyStr=%s", proxyStr)
	if hashValue, ok := Cm.Get(proxyStr); ok && hashValue != nil {
		//hashValue := item.Value()
		//hashValue := item
		switch hashValue.(type) {
		case C.Proxy:
			log.Debugln("matchHashMap match proxyStr=%s adapter=%v, hashRule=%v, err=%v ", proxyStr, adapter, hashRule, err)
			return hashValue.(C.Proxy), nil, nil
			//if proxy, ok := proxies[hashValue.(string)]; ok {
			//	return proxy, nil, nil
			//}
		}
	}

	log.Debugln("matchHashMap match(metadata) metadata=%v", metadata)
	adapter, hashRule, err = match(metadata)
	if hashRule != nil {
		setMatchHashMap(domainStr, hashRule)
	} else {
		setMatchHashMap(proxyStr, adapter)
	}

	log.Debugln("last match domainStr=%s adapter=%v, hashRule=%v, err=%v ", domainStr, adapter, hashRule, err)
	return
}
