package tunnel

import (
	"fmt"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel/statistic"
	"time"

	//HS "github.com/cornelk/hashmap"
	//"github.com/dgraph-io/ristretto"
	"github.com/hashicorp/golang-lru/v2"
	//"golang.org/x/sync/syncmap"
	"net"
)

var (
	Cm, _           = lru.New[string, any](1024 * 1024)
	Dm, _           = lru.New[string, string](1024 * 1024)
	Dm_se, _        = lru.New[string, string](1024 * 1024)
	Dm_se_domain, _ = lru.New[string, string](1024 * 1024)
	Dm_ttl, _       = lru.New[string, time.Time](1024 * 1024 * 4)
)

func DnsPreCache(domain string, ip string, remote net.Addr, ttl uint32) {
	//最小保存1s
	if ttl < 2 {
		ttl = 2
	}
	now_time := time.Now()
	end_time := now_time.Add(time.Second * time.Duration(ttl))
	ip_se_key := ip + "_se"

	//更新域名超时时间
	Dm_ttl.Add(domain, end_time)

	//存在不一致的缓存，移动到二级缓存
	if se_domain, ok := Dm.Get(ip); ok && se_domain != domain {
		Dm_se.Add(ip, se_domain)
		log.Debugln("DnsPreCache ip:%s has MultiDomain: %s,%s", ip, domain, se_domain)
		Dm_se_domain.Add(se_domain, "")
		if ip_end_time, ok := Dm_ttl.Get(ip); ok {
			Dm_ttl.Add(ip_se_key, ip_end_time)
		} else {
			Dm_ttl.Add(ip_se_key, end_time)
		}
	}

	//更新一级缓存
	Dm.Add(ip, domain)
	Dm_ttl.Add(ip, end_time)

	if se_end_time, ok := Dm_ttl.Get(ip_se_key); ok {
		if se_end_time.Before(now_time) {
			if end_domain, ok := Dm_se.Get(ip); ok && domain != end_domain {
				if domain_time_out, ok := Dm_ttl.Get(end_domain); ok && domain_time_out.Before(now_time) {
					Dm_se_domain.Remove(end_domain)
				}
			}

			Dm_se.Remove(ip)
			Dm_ttl.Remove(ip_se_key)
		}
	}
}

func InSeIP(ip string) bool {
	if Dm_se.Contains(ip) {
		//ip_se_key := ip + "_se"
		//if end_time, ok := Dm_ttl.Get(ip_se_key); ok {
		//	if end_time.Before(time.Now()) {
		//		Dm_se.Remove(ip)
		//		Dm_ttl.Remove(ip_se_key)
		//		return false
		//	}
		//}
		return true
	} else {
		return false
	}
}

func InSeDomain(domain string) bool {
	if Dm_se_domain.Contains(domain) {
		//if end_time, ok := Dm_ttl.Get(domain); ok {
		//	if end_time.Before(time.Now()) {
		//		Dm_se_domain.Remove(domain)
		//		Dm_ttl.Remove(domain)
		//		return false
		//	}
		//}
		return true
	} else {
		return false
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
