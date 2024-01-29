package dns

import (
	"bytes"
	"encoding/gob"
	"github.com/finddiff/RuleBaseProxy/Persistence"
	"github.com/finddiff/RuleBaseProxy/common/cache"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel"
	"net"

	C "github.com/finddiff/RuleBaseProxy/constant"
	D "github.com/miekg/dns"
	//"github.com/xujiajun/nutsdb"
	nutsdb "github.com/finddiff/nutsDBMD"
	"sync"
	"time"
)

// const MapDomainIPs string = "mapDomain-IPs"
// const MapDomainIPttl string = "mapDomainIP-ttl"
const MapIPDomain string = "mapIP-Domain"
const MapDomainDnsMsg string = "mapDomain-DnsMsg"
const MaxDnsMsgAge = 3 * 24 * 3600

type DnsMap struct {
	ipstr   string
	domain  string
	ttl     uint32
	raddr   net.Addr
	mapping *cache.LruCache
}

type DnsMsgMap struct {
	key   string
	value D.Msg
}

var (
	saveMapQueue = make(chan DnsMap, 500)
	saveDnsQueue = make(chan DnsMsgMap, 500)
	mu           sync.Mutex
	startED      = false
	//db           *sql.DB
	db *nutsdb.DB
	//SplitChat = "$"
)

func DnsMapAdd(dnsMap DnsMap) {
	select {
	case saveMapQueue <- dnsMap:
	default:
		log.Debugln("DnsMapAdd is block!")
	}
	//saveMapQueue <- dnsMap
}

func handleDnsMap(dnsMap DnsMap) {
	if C.UNSaveDNSDB {
		return
	}
	if dnsMap.ipstr == "" {
		return
	}

	dnsMap.ttl += 30
	tunnel.DnsPreCache(dnsMap.domain, dnsMap.ipstr, dnsMap.raddr, dnsMap.ttl)

	err := db.Update(func(tx *nutsdb.Tx) error {
		//add new to maps
		log.Debugln("DnsMapAdd add new to maps ip:%s| host:%s| expire Time:%v| ttl:%d|", dnsMap.ipstr, dnsMap.domain, time.Second*time.Duration(0), dnsMap.ttl)
		err := tx.Put(MapIPDomain, []byte(dnsMap.ipstr), []byte(dnsMap.domain), 0)
		if err != nil {
			log.Errorln("tx.Put(MapDomainIPs, key, []byte(val), 0) %v", err)
		}
		return nil
	})
	if err != nil {
		log.Errorln("DnsMapAdd db.Update(func(tx *nutsdb.Tx) error  %v", err)
	}
	//go tunnel.DnsPreCache(dnsMap.domain, dnsMap.ipstr, dnsMap.raddr)
}

func IPDomainMapOnEvict(key interface{}, value interface{}) {
	err := db.Update(func(tx *nutsdb.Tx) error {
		_ = tx.Delete(MapIPDomain, []byte(key.(string)))
		return nil
	})
	if err != nil {
		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
	}
}

func loadToIPDomainMap(mapping *cache.LruCache) {
	err := db.View(func(tx *nutsdb.Tx) error {
		//db.Merge()
		entries, _ := tx.GetAll(MapIPDomain)
		for _, entry := range entries {
			ip := string(entry.Key)
			domainStr := string(entry.Value)
			log.Infoln("loadToIPDomainMap SetWithExpire ip:%s| host:%s| expire Time:%v| ttl:%d|", ip, domainStr, time.Second*time.Duration(3), 3)
			mapping.SetWithExpire(ip, domainStr, time.Now().Add(time.Second*time.Duration(3)))
			tunnel.Dm.Set(ip, domainStr, 0)
		}
		return nil
	})
	if err != nil {
		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
	}

	mapping.SetOnEvict(IPDomainMapOnEvict)
}

func DnsMsgAdd(dnsMsg DnsMsgMap) {
	select {
	case saveDnsQueue <- dnsMsg:
	default:
		log.Debugln("DnsMsgAdd is block!")
	}
	//saveMapQueue <- dnsMap
}

func DnsMsg2Byte(p interface{}) (rb []byte, err error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err = enc.Encode(p)
	if err != nil {
		log.Errorln("Struct2Byte gob err:%v", err)
	}
	return buf.Bytes(), err
}

func Byte2DnsMsg(buf []byte) (dnsMsg D.Msg, err error) {
	enc := gob.NewDecoder(bytes.NewReader(buf))
	err = enc.Decode(&dnsMsg)
	if err != nil {
		log.Errorln("Byte2DnsMsg gob err:%v", err)
		//return dnsMsg, err
	}
	return dnsMsg, err
}

func handleDnsMsg() {
	for dnsMsg := range saveDnsQueue {
		if C.UNSaveDNSDB {
			return
		}

		value, err := DnsMsg2Byte(dnsMsg.value)
		if err != nil {
			continue
		}
		err = db.Update(func(tx *nutsdb.Tx) error {
			log.Debugln("handleDnsMsg tx.Put(MapDomainDnsMsg, []byte(dnsMsg.key:%v)", dnsMsg.key)
			_ = tx.Put(MapDomainDnsMsg, []byte(dnsMsg.key), value, MaxDnsMsgAge)
			return nil
		})
		if err != nil {
			log.Errorln("handleDnsMsg db.Update(func(tx *nutsdb.Tx) error:%v", err)
		}
	}
}

func DnsMapOnEvict(key interface{}, value interface{}) {
	err := db.Update(func(tx *nutsdb.Tx) error {
		_ = tx.Delete(MapDomainDnsMsg, []byte(key.(string)))
		return nil
	})
	if err != nil {
		log.Errorln("DnsMapOnEvict db.Update(func(tx *nutsdb.Tx) error %v", err)
	}
}

func loadToDnsMap(resolver *Resolver) {
	err := db.Update(func(tx *nutsdb.Tx) error {
		//db.Merge()
		entries, _ := tx.GetAll(MapDomainDnsMsg)
		for _, entry := range entries {
			//fmt.Println(string(entry.Key), string(entry.Value))
			//log.Debugln("loadToDnsMap entry.Key:%v entry.Value:%v", entry.Key, entry.Value)
			domainStr := string(entry.Key)
			dnsMsg, err := Byte2DnsMsg(entry.Value)
			if err != nil {
				continue
			}
			log.Infoln("loadToDnsMap SetWithExpire domainStr:%s| dnsMsg:%s| expire Time:%v| ttl:%d|", domainStr, dnsMsg, time.Second*time.Duration(3), 3)
			resolver.lruCache.SetWithExpire(domainStr, &dnsMsg, time.Now().Add(time.Second*time.Duration(3)))
		}
		return nil
	})
	if err != nil {
		log.Errorln("db.Update(func(tx *nutsdb.Tx) error %v", err)
	}

	resolver.lruCache.SetOnEvict(DnsMapOnEvict)
}

func loadNDSCache(resolver *Resolver, mapper *ResolverEnhancer) {
	if startED {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	startED = true

	if C.UNSaveDNSDB {
		go handleDnsMsg()
		go processDnsMap()
		return
	}

	gob.Register(&D.A{})
	gob.Register(&D.AAAA{})
	gob.Register(&D.PTR{})
	gob.Register(&D.SOA{})
	gob.Register(&D.CNAME{})
	gob.Register(&D.OPT{})
	gob.Register(&D.TXT{})
	gob.Register(&D.EDNS0_PADDING{})
	gob.Register(&D.SVCB{})
	gob.Register(&D.HTTPS{})
	gob.Register(&D.SVCBAlpn{})
	gob.Register(&D.SVCBLocal{})
	gob.Register(&D.SVCBIPv4Hint{})
	gob.Register(&D.SVCBIPv6Hint{})

	db = Persistence.DB

	loadToIPDomainMap(mapper.mapping)
	loadToDnsMap(resolver)

	go handleDnsMsg()
	go processDnsMap()
}

func processDnsMap() {
	defer db.Close()
	queue := saveMapQueue

	for dnsMap := range queue {
		go handleDnsMap(dnsMap)
	}
}
