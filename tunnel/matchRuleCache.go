package tunnel

import (
	"fmt"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel/statistic"
	//HS "github.com/cornelk/hashmap"
	//"github.com/dgraph-io/ristretto"
	"github.com/hashicorp/golang-lru/v2"
	//"golang.org/x/sync/syncmap"
	"net"
)

var (
	Cm, _    = lru.New[string, any](1024 * 1024)
	Dm, _    = lru.New[string, any](1024 * 1024)
	Dm_se, _ = lru.New[string, any](1024 * 1024)
	//Dm_end_time, _ = lru.New[string, any](1024 * 1024)
)

func DnsPreCache(domain string, ip string, remote net.Addr, ttl uint32) {
	if value, ok := Dm.Get(ip); ok && value.(string) == domain {
		Dm.Add(ip, domain)
		log.Debugln("DnsPreCache Dm cache ip:%v, domain%s", ip, domain)
	} else {
		Dm_se.Add(ip, domain)
		log.Debugln("DnsPreCache Dm_se cache ip:%v, domain%s", ip, domain)
		//Dm_end_time.Add(ip, time.Now().Add(time.Second*time.Duration(ttl)))
	}

}

func setMatchHashMap(key string, value interface{}) {
	Cm.Add(key, value)
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
