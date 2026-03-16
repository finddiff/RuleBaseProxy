package tunnel

import (
	"fmt"
	"golang.org/x/sync/singleflight"
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

	// singleflight.Group for UDP session
	udpSF singleflight.Group
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
	//Cm.Clear()
	Cm.Purge()
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

	//go processNotUserUDP()

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

	// preprocess enhanced-mode metadata
	if needLookupIP(metadata) {
		host, exist := resolver.FindHostByIP(metadata.DstIP)
		if !exist {
			if item, ok := Dm.Get(metadata.DstIP.String()); ok && item != "" {
				exist = true
				host = item
			}
		}

		if exist {
			metadata.Host = host
			if !(metadata.Type.String() == "HTTP" || metadata.Type.String() == "HTTP Connect" || metadata.Type.String() == "Socks4" || metadata.Type.String() == "Socks5") {
				metadata.AddrType = C.AtypDomainName
			}
			if resolver.FakeIPEnabled() {
				metadata.DstIP = nil
				//log.Debugln("preHandleMetadata after resolver.FakeIPEnabled infokey:%s", metadata.InfoKey())
			} else if node := resolver.DefaultHosts.Search(host); node != nil {
				// redir-host should lookup the hosts
				//metadata.DstIP = node.Data.(net.IP)
				if metadata.DstIP == nil {
					metadata.DstIP = node.Data.([]net.IP)[0]
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

	// 1. 先尝试直接从 natTable 获取已存在的连接
	if pc := natTable.Get(key); pc != nil {
		handleUDPToRemote(packet, pc, metadata, key)
		return
	}

	// 2. 使用 singleflight 确保只有一个协程在 Dial
	// 这样不需要手动管理 sync.Cond 和 LockKey
	go func() {
		_, err, _ := udpSF.Do(key, func() (interface{}, error) {
			// 双重检查，防止在进入 Do 之前连接已建立
			if pc := natTable.Get(key); pc != nil {
				return pc, nil
			}

			//if !metadata.Resolved() {
			//	ip, err := resolver.ResolveIP(metadata.Host)
			//	if err != nil {
			//		return nil, err
			//	}
			//	if metadata.DstIP == nil {
			//		metadata.DstIP = ip
			//	}
			//}
			//
			//addr := metadata.UDPAddr()
			//if addr == nil {
			//	return nil, errors.New("udp addr invalid")
			//}

			// 执行创建连接的逻辑
			ctx := context.NewPacketConnContext(metadata)
			proxy, rule, err := resolveMetadata(ctx, metadata)
			if err != nil {
				log.Warnln("[UDP] Parse metadata failed: %s", err.Error())
				return nil, err
			}

			//拨号前先加入跟踪器，确保在拨号出现问题时，已经在管理器内进行跟踪
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
				pc.Close()
				return nil, err
			}
			pc.PacketConn = rawPc
			pc.Chain = rawPc.Chains()
			//pc.SetRaddr(addr)
			ctx.InjectPacketConn(rawPc)

			// 启动读取协程
			go handleUDPToLocal(packet.UDPPacket, pc, key, fAddr)
			natTable.Set(key, pc)

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

			return pc, nil
		})

		if err != nil {
			log.Warnln("[UDP] setup connection failed: %v", err)
			return
		}

		// 3. 成功后，再次调用 handle
		if pc := natTable.Get(key); pc != nil {
			handleUDPToRemote(packet, pc, metadata, key)
		}
	}()
}

func handleTCPConn(ctx C.ConnContext) {
	defer ctx.Conn().Close()
	tcpTrack := statistic.Conn2TCPTracker(ctx.Tracker())

	startTcpConn := time.Since(tcpTrack.Start).Milliseconds()

	if tcpTrack == nil {
		tcpTrack = statistic.NewTCPTracker(nil, statistic.DefaultManager, ctx.Metadata(), nil, nil)
	}
	defer tcpTrack.Close()
	tcpTrack.Chain = []string{"DISP", "TRACE"}

	metadata := ctx.Metadata()
	if !metadata.Valid() {
		log.Warnln("[Metadata] not valid: %#v", metadata)
		return
	}
	//log.Infoln("handleTCPConn infokey:%s", metadata.InfoKey())

	tcpTrack.Chain = []string{"PREH", "TRACE"}
	if err := preHandleMetadata(metadata); err != nil {
		log.Debugln("[Metadata PreHandle] error: %s", err)
		return
	}
	//log.Infoln("handleTCPConn after preHandleMetadata infokey:%s", metadata.InfoKey())

	tcpTrack.Chain = []string{"MDNS", "TRACE"}
	proxy, rule, err := resolveMetadata(ctx, metadata)
	if err != nil {
		log.Warnln("[Metadata] parse failed: %s", err.Error())
		return
	}

	if rule != nil {
		tcpTrack.TrackerInfo().Rule = rule.RuleType().String()
		tcpTrack.TrackerInfo().RulePayload = rule.Payload()
		tcpTrack.Chain = []string{rule.RuleType().String(), rule.Payload(), proxy.Name(), "DAIL", "TRACE"}
	} else {
		tcpTrack.Chain = []string{proxy.Name(), "DAIL", "TRACE"}
	}

	org_DstIP := metadata.DstIP
	org_AddrType := metadata.AddrType
	//MultiDomain := InSeIP(metadata.DstIP.String()) || InSeDomain(metadata.Host)
	if rule != nil && rule.MultiDomainDialIP() {
		log.Debugln("tunnel handleTCPConn DstAddr %s:%s, infokey:%s, AddrType:%v", metadata.DstAddr(), metadata.DstPort, metadata.InfoKey(), metadata.AddrType)
		if !(metadata.Type.String() == "HTTP" || metadata.Type.String() == "HTTP Connect" || metadata.Type.String() == "Socks4" || metadata.Type.String() == "Socks5") {
			if metadata.DstIP.To4() != nil {
				metadata.AddrType = C.AtypIPv4
			} else {
				metadata.AddrType = C.AtypIPv6
			}
		}
	}

	Dial_type := "Dial-Unkown"
	if metadata.AddrType == C.AtypIPv4 || metadata.AddrType == C.AtypIPv6 {
		Dial_type = "Dial-IP"
	}
	if metadata.AddrType == C.AtypDomainName {
		Dial_type = "Dial-Domain"
	}

	log.Debugln("proxy(%v).Dial metadata NetWork:%v Type:%v SrcIP:%v DstIP:%v SrcPort:%v DstPort:%v AddrType:%v Host:%v", proxy.Name(), metadata.NetWork, metadata.Type, metadata.SrcIP, metadata.DstIP, metadata.SrcPort, metadata.DstPort, metadata.AddrType, metadata.Host)
	//tcpTrack.Chain = []string{proxy.Name(), "DAIL", "ERROR"}
	startDial := time.Since(tcpTrack.Start).Milliseconds()
	remoteConn, err := proxy.Dial(metadata)

	metadata.AddrType = org_AddrType
	metadata.DstIP = org_DstIP

	if err != nil {
		if rule == nil {
			log.Warnln("[TCP] dial %s to %s error: %s", proxy.Name(), metadata.RemoteAddress(), err.Error())
			tcpTrack.Chain = []string{proxy.Name(), err.Error(), "ERROR"}
		} else {
			log.Warnln("[TCP] dial %s (match %s/%s) to %s error: %s", proxy.Name(), rule.RuleType().String(), rule.Payload(), metadata.RemoteAddress(), err.Error())
			tcpTrack.Chain = []string{proxy.Name(), rule.RuleType().String(), rule.Payload(), err.Error(), "ERROR"}
		}

		if time.Now().Sub(tcpTrack.Start) < time.Duration(3)*time.Second {
			time.Sleep(time.Duration(3) * time.Second)
		}
		//time.Sleep(time.Duration(3) * time.Second)
		//defer tcpTrack.Close()
		return
	}

	tcpTrack.Conn = remoteConn
	tcpTrack.Chain = append(remoteConn.Chains(), Dial_type)
	remoteConn = tcpTrack
	defer remoteConn.Close()

	//log.Infoln("handleTCPConn after proxy.Dial infokey%s", metadata.InfoKey())

	switch true {
	case rule != nil:
		log.Infoln("[TCP] %s --> %s match %s(%s) using %s, startTcpConn:%dms, startDial:%dms, cost:%dms", metadata.SourceAddress(), metadata.RemoteAddress(), rule.RuleType().String(), rule.Payload(), remoteConn.Chains().String(), startTcpConn, startDial, time.Since(tcpTrack.Start).Milliseconds())
	case mode == Global:
		log.Infoln("[TCP] %s --> %s using GLOBAL, startTcpConn:%dms, startDial:%dms, cost:%dms", metadata.SourceAddress(), metadata.RemoteAddress(), startTcpConn, startDial, time.Since(tcpTrack.Start).Milliseconds())
	case mode == Direct:
		log.Infoln("[TCP] %s --> %s using DIRECT, startTcpConn:%dms, startDial:%dms, cost:%dms", metadata.SourceAddress(), metadata.RemoteAddress(), startTcpConn, startDial, time.Since(tcpTrack.Start).Milliseconds())
	default:
		log.Infoln("[TCP] %s --> %s doesn't match any rule using DIRECT, startTcpConn:%dms, startDial:%dms, cost:%dms", metadata.SourceAddress(), metadata.RemoteAddress(), startTcpConn, startDial, time.Since(tcpTrack.Start).Milliseconds())
	}

	//handleSocket(ctx, remoteConn)
	relay(ctx.Conn(), remoteConn)
}

func shouldResolveIP(rule C.Rule, metadata *C.Metadata) bool {
	return rule.ShouldResolveIP() && metadata.Host != "" && metadata.DstIP == nil
}

func match(metadata *C.Metadata) (C.Proxy, C.Rule, error) {
	configMux.RLock()
	defer configMux.RUnlock()

	var resolved bool

	if node := resolver.DefaultHosts.Search(metadata.Host); node != nil {
		ipList := node.Data.([]net.IP)
		if metadata.DstIP == nil {
			metadata.DstIP = ipList[0]
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
