package tunnel

import (
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	"github.com/finddiff/RuleBaseProxy/component/nat"
	"github.com/finddiff/RuleBaseProxy/component/resolver"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/constant/provider"
	"github.com/finddiff/RuleBaseProxy/context"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel/statistic"
)

var (
	tcpQueue  = make(chan C.ConnContext, 200)
	udpQueue  = make(chan *inbound.PacketAdapter, 200)
	natTable  = nat.New()
	rules     []C.Rule
	proxies   = make(map[string]C.Proxy)
	providers map[string]provider.ProxyProvider
	configMux sync.RWMutex

	// Outbound Rule
	mode = Rule

	// default timeout for UDP session
	udpTimeout = 60 * time.Second
)

func init() {
	go process()
}

// TCPIn return fan-in queue
func TCPIn() chan<- C.ConnContext {
	return tcpQueue
}

// UDPIn return fan-in udp queue
func UDPIn() chan<- *inbound.PacketAdapter {
	return udpQueue
}

// Rules return all rules
func Rules() []C.Rule {
	return rules
}

// UpdateRules handle update rules
func UpdateRules(newRules []C.Rule) {
	configMux.Lock()
	rules = newRules
	Cm.Clear()
	configMux.Unlock()
}

// Proxies return all proxies
func Proxies() map[string]C.Proxy {
	return proxies
}

// Providers return all compatible providers
func Providers() map[string]provider.ProxyProvider {
	return providers
}

// UpdateProxies handle update proxies
func UpdateProxies(newProxies map[string]C.Proxy, newProviders map[string]provider.ProxyProvider) {
	configMux.Lock()
	proxies = newProxies
	providers = newProviders
	configMux.Unlock()
}

// Mode return current mode
func Mode() TunnelMode {
	return mode
}

// SetMode change the mode of tunnel
func SetMode(m TunnelMode) {
	mode = m
}

// processUDP starts a loop to handle udp packet
func processUDP() {
	queue := udpQueue
	for conn := range queue {
		handleUDPConn(conn)
	}
}

func processTCP() {
	queue := tcpQueue
	for conn := range queue {
		go handleTCPConn(conn)
	}
}

func process() {
	numUDPWorkers := 4
	if runtime.NumCPU() > numUDPWorkers {
		numUDPWorkers = runtime.NumCPU()
	}
	for i := 0; i < numUDPWorkers; i++ {
		go processUDP()
		go processTCP()
	}

	//tun.tunprocess()

	//queue := tcpQueue
	//for conn := range queue {
	//	go handleTCPConn(conn)
	//}
}

func needLookupIP(metadata *C.Metadata) bool {
	return resolver.MappingEnabled() && metadata.Host == "" && metadata.DstIP != nil
}

func preHandleMetadata(metadata *C.Metadata) error {
	// handle IP string on host
	if ip := net.ParseIP(metadata.Host); ip != nil {
		metadata.DstIP = ip
		metadata.Host = ""
		if ip.To4() != nil {
			metadata.AddrType = C.AtypIPv4
		} else {
			metadata.AddrType = C.AtypIPv6
		}
	}

	//log.Infoln("preHandleMetadata after ParseIP infokey:%s", metadata.InfoKey())

	// preprocess enhanced-mode metadata
	if needLookupIP(metadata) {
		host, exist := resolver.FindHostByIP(metadata.DstIP)
		if !exist {
			item, ok := Dm.Get(metadata.DstIP.String())
			if ok && item != nil {
				exist = true
				host = item.(string)
			}
		}
		//log.Debugln("preHandleMetadata after needLookupIP infokey:%s", metadata.InfoKey())

		if exist {
			metadata.Host = host
			metadata.AddrType = C.AtypDomainName
			if resolver.FakeIPEnabled() {
				metadata.DstIP = nil
				//log.Debugln("preHandleMetadata after resolver.FakeIPEnabled infokey:%s", metadata.InfoKey())
			} else if node := resolver.DefaultHosts.Search(host); node != nil {
				// redir-host should lookup the hosts
				//metadata.DstIP = node.Data.(net.IP)
				if metadata.DstIP == nil {
					metadata.DstIP = node.Data.(net.IP)
					//log.Debugln("preHandleMetadata after resolver.DefaultHosts infokey:%s", metadata.InfoKey())
				}
			}

			//log.Debugln("preHandleMetadata after exist infokey:%s", metadata.InfoKey())
		} else if resolver.IsFakeIP(metadata.DstIP) {
			return fmt.Errorf("fake DNS record %s missing", metadata.DstIP)
		}
	}

	return nil
}

//
//func afterHandleMetadata(metadata *C.Metadata) error {
//	if resolver.FakeIPEnabled() {
//		return nil
//	}
//	// handle IP string on host
//	if metadata.DstIP != nil {
//		if metadata.DstIP.To4() != nil {
//			metadata.AddrType = C.AtypIPv4
//		} else {
//			metadata.AddrType = C.AtypIPv6
//		}
//	}
//
//	return nil
//}

func resolveMetadata(ctx C.PlainContext, metadata *C.Metadata) (proxy C.Proxy, rule C.Rule, err error) {
	switch mode {
	case Direct:
		proxy = proxies["DIRECT"]
	case Global:
		proxy = proxies["GLOBAL"]
	// Rule
	default:
		//proxy, rule, err = match(metadata)
		proxy, rule, err = matchHashMap(metadata)
	}
	return
}

func handleUDPConn(packet *inbound.PacketAdapter) {
	metadata := packet.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}

	// make a fAddr if request ip is fakeip
	var fAddr net.Addr
	if resolver.IsExistFakeIP(metadata.DstIP) {
		fAddr = metadata.UDPAddr()
	}

	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}

	key := packet.LocalAddr().String()

	handle := func() bool {
		pc := natTable.Get(key)
		if pc != nil {
			handleUDPToRemote(packet, pc, metadata)
			return true
		}
		return false
	}

	if handle() {
		return
	}

	lockKey := key + "-lock"
	cond, loaded := natTable.GetOrCreateLock(lockKey)

	go func() {
		if loaded {
			cond.L.Lock()
			cond.Wait()
			handle()
			cond.L.Unlock()
			return
		}

		defer func() {
			natTable.Delete(lockKey)
			cond.Broadcast()
		}()

		ctx := context.NewPacketConnContext(metadata)
		proxy, rule, err := resolveMetadata(ctx, metadata)
		if err != nil {
			log.Warnln("[UDP] Parse metadata failed: %s", err.Error())
			return
		}

		//if err := afterHandleMetadata(metadata); err != nil {
		//	log.Debugln("[Metadata afterHandle] error: %s", err)
		//	return
		//}

		pc := statistic.NewUDPTracker(nil, statistic.DefaultManager, metadata, rule, proxy)
		statistic.DefaultManager.Join(pc)
		rawPc, err := proxy.DialUDP(metadata)
		if err != nil {
			if rule == nil {
				log.Warnln("[UDP] dial %s to %s error: %s", proxy.Name(), metadata.RemoteAddress(), err.Error())
			} else {
				log.Warnln("[UDP] dial %s (match %s/%s) to %s error: %s", proxy.Name(), rule.RuleType().String(), rule.Payload(), metadata.RemoteAddress(), err.Error())
			}
			pc.Chain = []string{err.Error(), "ERROR", proxy.Name()}
			//time.Sleep(time.Duration(3) * time.Second)
			pc.Close()
			return
		}
		ctx.InjectPacketConn(rawPc)
		pc.PacketConn = rawPc
		pc.Chain = rawPc.Chains()
		//statistic.DefaultManager.Join(pc)
		//pc := statistic.NewUDPTracker(rawPc, statistic.DefaultManager, metadata, rule)

		switch true {
		case rule != nil:
			log.Infoln("[UDP] %s --> %s match %s(%s) using %s", metadata.SourceAddress(), metadata.RemoteAddress(), rule.RuleType().String(), rule.Payload(), rawPc.Chains().String())
		case mode == Global:
			log.Infoln("[UDP] %s --> %s using GLOBAL", metadata.SourceAddress(), metadata.RemoteAddress())
		case mode == Direct:
			log.Infoln("[UDP] %s --> %s using DIRECT", metadata.SourceAddress(), metadata.RemoteAddress())
		default:
			log.Infoln("[UDP] %s --> %s doesn't match any rule using DIRECT", metadata.SourceAddress(), metadata.RemoteAddress())
		}

		go handleUDPToLocal(packet.UDPPacket, pc, key, fAddr)

		natTable.Set(key, pc)
		handle()
	}()
}

func handleTCPConn(ctx C.ConnContext) {
	defer ctx.Conn().Close()
	tcpTrack := statistic.Conn2TCPTracker(ctx.Tracker())
	defer tcpTrack.Close()
	if tcpTrack == nil {
		tcpTrack = statistic.NewTCPTracker(nil, statistic.DefaultManager, ctx.Metadata(), nil, nil)
	}
	tcpTrack.Chain = []string{"DISP", "ERROR"}

	metadata := ctx.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}
	//log.Infoln("handleTCPConn infokey:%s", metadata.InfoKey())

	tcpTrack.Chain = []string{"PREH", "ERROR"}
	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}
	//log.Infoln("handleTCPConn after preHandleMetadata infokey:%s", metadata.InfoKey())

	tcpTrack.Chain = []string{"MDNS", "ERROR"}
	proxy, rule, err := resolveMetadata(ctx, metadata)
	if err != nil {
		log.Warnln("[Metadata] parse failed: %s", err.Error())
		return
	}
	//log.Infoln("handleTCPConn after resolveMetadata infokey:%s", metadata.InfoKey())

	//tcpTrack := statistic.NewTCPTracker(nil, statistic.DefaultManager, metadata, rule, proxy)
	//statistic.DefaultManager.Join(tcpTrack)
	//tcpTrack := (ctx.Tracker()).(*tcpTracker)
	//if err := afterHandleMetadata(metadata); err != nil {
	//	log.Debugln("[Metadata afterHandle] error: %s", err)
	//	return
	//}

	tcpTrack.Chain = []string{"DAIL", "ERROR"}
	remoteConn, err := proxy.Dial(metadata)
	if err != nil {
		if rule == nil {
			log.Warnln("[TCP] dial %s to %s error: %s", proxy.Name(), metadata.RemoteAddress(), err.Error())
		} else {
			log.Warnln("[TCP] dial %s (match %s/%s) to %s error: %s", proxy.Name(), rule.RuleType().String(), rule.Payload(), metadata.RemoteAddress(), err.Error())
		}
		tcpTrack.Chain = []string{err.Error(), "ERROR", proxy.Name()}
		//time.Sleep(time.Duration(3) * time.Second)
		defer tcpTrack.Close()
		return
	}
	tcpTrack.Chain = []string{"RECO", "ERROR"}
	tcpTrack.Conn = remoteConn
	tcpTrack.Chain = remoteConn.Chains()
	remoteConn = tcpTrack
	//statistic.DefaultManager.Join(tcpTrack)
	//remoteConn = statistic.NewTCPTracker(remoteConn, statistic.DefaultManager, metadata, rule)
	defer remoteConn.Close()

	//log.Infoln("handleTCPConn after proxy.Dial infokey%s", metadata.InfoKey())

	switch true {
	case rule != nil:
		log.Infoln("[TCP] %s --> %s match %s(%s) using %s", metadata.SourceAddress(), metadata.RemoteAddress(), rule.RuleType().String(), rule.Payload(), remoteConn.Chains().String())
	case mode == Global:
		log.Infoln("[TCP] %s --> %s using GLOBAL", metadata.SourceAddress(), metadata.RemoteAddress())
	case mode == Direct:
		log.Infoln("[TCP] %s --> %s using DIRECT", metadata.SourceAddress(), metadata.RemoteAddress())
	default:
		log.Infoln("[TCP] %s --> %s doesn't match any rule using DIRECT", metadata.SourceAddress(), metadata.RemoteAddress())
	}

	handleSocket(ctx, remoteConn)
}

func shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return rule.ShouldResolveIP() && metadata.Host != "" && metadata.DstIP == nil
}

func match(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	configMux.RLock()
	defer configMux.RUnlock()

	var resolved bool

	if node := resolver.DefaultHosts.Search(metadata.Host); node != nil {
		ip := node.Data.(net.IP)
		if metadata.DstIP == nil {
			metadata.DstIP = ip
		}
		resolved = true
	}

	for _, rule := range rules {
		if !resolved && shouldResolveIP(rule, metadata) {
			ip, err := resolver.ResolveIP(metadata.Host)
			if err != nil {
				log.Debugln("[DNS] resolve %s error: %s", metadata.Host, err.Error())
			} else {
				log.Debugln("[DNS] %s --> %s", metadata.Host, ip.String())
				if metadata.DstIP == nil {
					metadata.DstIP = ip
				}
			}
			resolved = true
		}

		if rule.Match(metadata) {
			adapter, ok := proxies[rule.Adapter()]
			if !ok {
				continue
			}

			if metadata.NetWork == C.UDP && !adapter.SupportUDP() {
				log.Debugln("%s UDP is not supported", adapter.Name())
				continue
			}
			return adapter, rule, nil
		}
	}

	return proxies["DIRECT"], nil, nil
}
