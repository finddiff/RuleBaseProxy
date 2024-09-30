package executor

import (
	"fmt"
	"io/ioutil"
	"os"
	"sync"

	"github.com/finddiff/RuleBaseProxy/adapter"
	"github.com/finddiff/RuleBaseProxy/adapter/outboundgroup"
	"github.com/finddiff/RuleBaseProxy/component/auth"
	"github.com/finddiff/RuleBaseProxy/component/dialer"
	"github.com/finddiff/RuleBaseProxy/component/iface"
	"github.com/finddiff/RuleBaseProxy/component/profile"
	"github.com/finddiff/RuleBaseProxy/component/profile/cachefile"
	"github.com/finddiff/RuleBaseProxy/component/resolver"
	"github.com/finddiff/RuleBaseProxy/component/trie"
	"github.com/finddiff/RuleBaseProxy/config"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/constant/provider"
	"github.com/finddiff/RuleBaseProxy/dns"
	P "github.com/finddiff/RuleBaseProxy/listener"
	authStore "github.com/finddiff/RuleBaseProxy/listener/auth"
	"github.com/finddiff/RuleBaseProxy/log"
	"github.com/finddiff/RuleBaseProxy/tunnel"
)

var (
	mux sync.Mutex
)

func readConfig(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", path)
	}

	return data, err
}

// Parse config with default config path
func Parse() (*config.Config, error) {
	return ParseWithPath(C.Path.Config())
}

// ParseWithPath parse config with custom config path
func ParseWithPath(path string) (*config.Config, error) {
	buf, err := readConfig(path)
	if err != nil {
		return nil, err
	}

	return ParseWithBytes(buf)
}

// ParseWithBytes config with buffer
func ParseWithBytes(buf []byte) (*config.Config, error) {
	return config.Parse(buf)
}

// ApplyConfig dispatch configure to all parts
func ApplyConfig(cfg *config.Config, force bool) {
	mux.Lock()
	defer mux.Unlock()

	updateUsers(cfg.Users)
	updateProxies(cfg.Proxies, cfg.Providers)
	updateRules(cfg.Rules)
	updateHosts(cfg.Hosts)
	updateProfile(cfg)
	updateGeneral(cfg.General, force)
	updateDNS(cfg.DNS)
	updateExperimental(cfg)
}

func GetGeneral() *config.General {
	ports := P.GetPorts()
	authenticator := []string{}
	if auth := authStore.Authenticator(); auth != nil {
		authenticator = auth.Users()
	}

	general := &config.General{
		Inbound: config.Inbound{
			Port:           ports.Port,
			SocksPort:      ports.SocksPort,
			RedirPort:      ports.RedirPort,
			TProxyPort:     ports.TProxyPort,
			MixedPort:      ports.MixedPort,
			Authentication: authenticator,
			AllowLan:       P.AllowLan(),
			BindAddress:    P.BindAddress(),
		},
		Mode:     tunnel.Mode(),
		LogLevel: log.Level(),
		IPv6:     !resolver.DisableIPv6,
	}

	return general
}

func updateExperimental(c *config.Config) {}

func updateDNS(c *config.DNS) {
	if !c.Enable {
		resolver.DefaultResolver = nil
		resolver.DefaultHostMapper = nil
		dns.ReCreateServer("", nil, nil)
		return
	}

	cfg := dns.Config{
		Main:         c.NameServer,
		Fallback:     c.Fallback,
		IPv6:         c.IPv6,
		EnhancedMode: c.EnhancedMode,
		Pool:         c.FakeIPRange,
		Hosts:        c.Hosts,
		FallbackFilter: dns.FallbackFilter{
			GeoIP:     c.FallbackFilter.GeoIP,
			GeoIPCode: c.FallbackFilter.GeoIPCode,
			IPCIDR:    c.FallbackFilter.IPCIDR,
			Domain:    c.FallbackFilter.Domain,
		},
		Default: c.DefaultNameserver,
		Policy:  c.NameServerPolicy,
	}

	r := dns.NewResolver(cfg)
	m := dns.NewEnhancer(cfg)

	// reuse cache of old host mapper
	if old := resolver.DefaultHostMapper; old != nil {
		m.PatchFrom(old.(*dns.ResolverEnhancer))
	}

	resolver.DefaultResolver = r
	resolver.DefaultHostMapper = m

	dns.ADGurdRules = []C.Rule{}
	dns.ADGurdCache.Purge()
	for _, file := range c.ADGuard {
		dns.AdgurdFile2Rule(file)
	}

	if err := dns.ReCreateServer(c.Listen, r, m); err != nil {
		log.Errorln("Start DNS server error: %s", err.Error())
		return
	}

	if c.Listen != "" {
		log.Infoln("DNS server listening at: %s", c.Listen)
	}
}

func updateHosts(tree *trie.DomainTrie) {
	resolver.DefaultHosts = tree
}

func updateProxies(proxies map[string]C.Proxy, providers map[string]provider.ProxyProvider) {
	tunnel.UpdateProxies(proxies, providers)
}

func updateRules(rules []C.Rule) {
	rules = tunnel.LoadRule(rules)
	tunnel.UpdateRules(rules)
}

func updateGeneral(general *config.General, force bool) {
	log.SetLevel(general.LogLevel)
	tunnel.SetMode(general.Mode)
	resolver.DisableIPv6 = !general.IPv6

	if general.Interface != "" {
		dialer.DefaultOptions = []dialer.Option{dialer.WithInterface(general.Interface)}
	} else {
		dialer.DefaultOptions = nil
	}

	iface.FlushCache()

	if !force {
		return
	}

	allowLan := general.AllowLan
	P.SetAllowLan(allowLan)

	bindAddress := general.BindAddress
	P.SetBindAddress(bindAddress)

	if general.Controller.ExternalCMD != "" {
		log.Infoln("ExternalCMD: %s", general.Controller.ExternalCMD)
		if err := P.PreCmd(general.Controller.ExternalCMD); err != nil {
			log.Errorln("ExternalCMD error: %s", err.Error())
		}
	}

	tcpIn := tunnel.TCPIn()
	udpIn := tunnel.UDPIn()

	if err := P.ReCreateHTTP(general.Port, tcpIn); err != nil {
		log.Errorln("Start HTTP server error: %s", err.Error())
	}

	if err := P.ReCreateSocks(general.SocksPort, tcpIn, udpIn); err != nil {
		log.Errorln("Start SOCKS server error: %s", err.Error())
	}

	if err := P.ReCreateRedir(general.RedirPort, tcpIn, udpIn); err != nil {
		log.Errorln("Start Redir server error: %s", err.Error())
	}

	if err := P.ReCreateTProxy(general.TProxyPort, tcpIn, udpIn); err != nil {
		log.Errorln("Start TProxy server error: %s", err.Error())
	}

	if err := P.ReCreateMixed(general.MixedPort, tcpIn, udpIn); err != nil {
		log.Errorln("Start Mixed(http and socks) server error: %s", err.Error())
	}

	if err := P.ReCreateTun(general.TunDevice, general.TUNPreUp, general.TUNPostUp, tcpIn, udpIn); err != nil {
		log.Errorln("Start Tun server error: %s", err.Error())
	}
}

func updateUsers(users []auth.AuthUser) {
	if len(users) > 0 {
		C.UserName = users[0].User
		C.UserPass = users[0].Pass
	}
	authenticator := auth.NewAuthenticator(users)
	authStore.SetAuthenticator(authenticator)
	if authenticator != nil {
		log.Infoln("Authentication of local server updated")
	}
}

func updateProfile(cfg *config.Config) {
	profileCfg := cfg.Profile

	profile.StoreSelected.Store(profileCfg.StoreSelected)
	if profileCfg.StoreSelected {
		patchSelectGroup(cfg.Proxies)
	}
}

func patchSelectGroup(proxies map[string]C.Proxy) {
	mapping := cachefile.Cache().SelectedMap()
	if mapping == nil {
		return
	}

	for name, proxy := range proxies {
		outbound, ok := proxy.(*adapter.Proxy)
		if !ok {
			continue
		}

		selector, ok := outbound.ProxyAdapter.(*outboundgroup.Selector)
		if !ok {
			continue
		}

		selected, exist := mapping[name]
		if !exist {
			continue
		}

		selector.Set(selected)
	}
}
