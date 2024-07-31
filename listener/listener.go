package proxy

import (
	"fmt"
	"github.com/finddiff/RuleBaseProxy/listener/tun"
	"github.com/finddiff/RuleBaseProxy/listener/tun/forward"
	"net"
	"strconv"
	"sync"

	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	C "github.com/finddiff/RuleBaseProxy/constant"
	"github.com/finddiff/RuleBaseProxy/listener/http"
	"github.com/finddiff/RuleBaseProxy/listener/mixed"
	"github.com/finddiff/RuleBaseProxy/listener/redir"
	"github.com/finddiff/RuleBaseProxy/listener/socks"
	"github.com/finddiff/RuleBaseProxy/listener/tproxy"
	"github.com/finddiff/RuleBaseProxy/log"
)

var (
	allowLan    = false
	bindAddress = "*"

	socksListener     *socks.Listener
	socksUDPListener  *socks.UDPListener
	httpListener      *http.Listener
	redirListener     *redir.Listener
	redirUDPListener  *tproxy.UDPListener
	tproxyListener    *tproxy.Listener
	tproxyUDPListener *tproxy.UDPListener
	mixedListener     *mixed.Listener
	mixedUDPLister    *socks.UDPListener

	// lock for recreate function
	socksMux  sync.Mutex
	httpMux   sync.Mutex
	redirMux  sync.Mutex
	tproxyMux sync.Mutex
	mixedMux  sync.Mutex
)

type Ports struct {
	Port       int `json:"port"`
	SocksPort  int `json:"socks-port"`
	RedirPort  int `json:"redir-port"`
	TProxyPort int `json:"tproxy-port"`
	MixedPort  int `json:"mixed-port"`
}

func AllowLan() bool {
	return allowLan
}

func BindAddress() string {
	return bindAddress
}

func SetAllowLan(al bool) {
	allowLan = al
}

func SetBindAddress(host string) {
	bindAddress = host
}

func PreCmd(cmd string) error {
	return tun.ExecCommand(cmd)
}

func ReCreateHTTP(port int, tcpIn chan<- C.ConnContext) error {
	httpMux.Lock()
	defer httpMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if httpListener != nil {
		if httpListener.RawAddress() == addr {
			return nil
		}
		httpListener.Close()
		httpListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	httpListener, err = http.New(addr, tcpIn)
	if err != nil {
		return err
	}

	C.DnsProxyString = "http://127.0.0.1:" + strconv.Itoa(port)
	log.Infoln("HTTP proxy listening at: %s", httpListener.Address())
	return nil
}

func ReCreateSocks(port int, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) error {
	socksMux.Lock()
	defer socksMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	shouldTCPIgnore := false
	shouldUDPIgnore := false

	if socksListener != nil {
		if socksListener.RawAddress() != addr {
			socksListener.Close()
			socksListener = nil
		} else {
			shouldTCPIgnore = true
		}
	}

	if socksUDPListener != nil {
		if socksUDPListener.RawAddress() != addr {
			socksUDPListener.Close()
			socksUDPListener = nil
		} else {
			shouldUDPIgnore = true
		}
	}

	if shouldTCPIgnore && shouldUDPIgnore {
		return nil
	}

	if portIsZero(addr) {
		return nil
	}

	tcpListener, err := socks.New(addr, tcpIn)
	if err != nil {
		return err
	}

	udpListener, err := socks.NewUDP(addr, udpIn)
	if err != nil {
		tcpListener.Close()
		return err
	}

	socksListener = tcpListener
	socksUDPListener = udpListener

	C.TunProxyString = "socks5://127.0.0.1:" + strconv.Itoa(port)
	log.Infoln("SOCKS proxy listening at: %s", socksListener.Address())
	return nil
}

func ReCreateRedir(port int, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) error {
	redirMux.Lock()
	defer redirMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if redirListener != nil {
		if redirListener.RawAddress() == addr {
			return nil
		}
		redirListener.Close()
		redirListener = nil
	}

	if redirUDPListener != nil {
		if redirUDPListener.RawAddress() == addr {
			return nil
		}
		redirUDPListener.Close()
		redirUDPListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	redirListener, err = redir.New(addr, tcpIn)
	if err != nil {
		return err
	}

	redirUDPListener, err = tproxy.NewUDP(addr, udpIn)
	if err != nil {
		log.Warnln("Failed to start Redir UDP Listener: %s", err)
	}

	log.Infoln("Redirect proxy listening at: %s", redirListener.Address())
	return nil
}

func ReCreateTProxy(port int, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) error {
	tproxyMux.Lock()
	defer tproxyMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	if tproxyListener != nil {
		if tproxyListener.RawAddress() == addr {
			return nil
		}
		tproxyListener.Close()
		tproxyListener = nil
	}

	if tproxyUDPListener != nil {
		if tproxyUDPListener.RawAddress() == addr {
			return nil
		}
		tproxyUDPListener.Close()
		tproxyUDPListener = nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	tproxyListener, err = tproxy.New(addr, tcpIn)
	if err != nil {
		return err
	}

	tproxyUDPListener, err = tproxy.NewUDP(addr, udpIn)
	if err != nil {
		log.Warnln("Failed to start TProxy UDP Listener: %s", err)
	}

	log.Infoln("TProxy server listening at: %s", tproxyListener.Address())
	return nil
}

func ReCreateMixed(port int, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) error {
	mixedMux.Lock()
	defer mixedMux.Unlock()

	addr := genAddr(bindAddress, port, allowLan)

	shouldTCPIgnore := false
	shouldUDPIgnore := false

	if mixedListener != nil {
		if mixedListener.RawAddress() != addr {
			mixedListener.Close()
			mixedListener = nil
		} else {
			shouldTCPIgnore = true
		}
	}
	if mixedUDPLister != nil {
		if mixedUDPLister.RawAddress() != addr {
			mixedUDPLister.Close()
			mixedUDPLister = nil
		} else {
			shouldUDPIgnore = true
		}
	}

	if shouldTCPIgnore && shouldUDPIgnore {
		return nil
	}

	if portIsZero(addr) {
		return nil
	}

	var err error
	mixedListener, err = mixed.New(addr, tcpIn)
	if err != nil {
		return err
	}

	mixedUDPLister, err = socks.NewUDP(addr, udpIn)
	if err != nil {
		mixedListener.Close()
		return err
	}
	C.DnsProxyString = "http://127.0.0.1:" + strconv.Itoa(port)
	C.TunProxyString = "socks5://127.0.0.1:" + strconv.Itoa(port)
	log.Infoln("Mixed(http+socks) proxy listening at: %s", mixedListener.Address())
	return nil
}

func ReCreateTun(TunDevice string, TUNPreUp, TUNPostUp []string, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) error {
	//tun.Insert()
	//maxprocs.Set(maxprocs.Logger(func(string, ...any) {}))
	if TunDevice == "" {
		return nil
	}

	tun.Stop()
	key := new(tun.Key)

	key.Mark = 0
	key.MTU = 0
	key.UDPTimeout = 0
	key.Device = TunDevice
	key.Interface = ""
	key.LogLevel = "debug"
	key.RestAPI = ""
	key.TCPSendBufferSize = ""
	key.TCPReceiveBufferSize = ""
	key.TCPModerateReceiveBuffer = true
	key.TUNPreUp = TUNPreUp
	key.TUNPostUp = TUNPostUp
	key.Proxy = C.TunProxyString

	log.Infoln("TunDevice:%s , TUNPreUp:%s, TUNPostUp:%s", TunDevice, TUNPreUp, TUNPostUp)

	forward.TcpIn = tcpIn
	forward.UdpIn = udpIn
	tun.Insert(key)
	tun.Start()
	return nil
}

// GetPorts return the ports of proxy servers
func GetPorts() *Ports {
	ports := &Ports{}

	if httpListener != nil {
		_, portStr, _ := net.SplitHostPort(httpListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.Port = port
	}

	if socksListener != nil {
		_, portStr, _ := net.SplitHostPort(socksListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.SocksPort = port
	}

	if redirListener != nil {
		_, portStr, _ := net.SplitHostPort(redirListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.RedirPort = port
	}

	if tproxyListener != nil {
		_, portStr, _ := net.SplitHostPort(tproxyListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.TProxyPort = port
	}

	if mixedListener != nil {
		_, portStr, _ := net.SplitHostPort(mixedListener.Address())
		port, _ := strconv.Atoi(portStr)
		ports.MixedPort = port
	}

	return ports
}

func portIsZero(addr string) bool {
	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return true
	}
	return false
}

func genAddr(host string, port int, allowLan bool) string {
	if allowLan {
		if host == "*" {
			return fmt.Sprintf(":%d", port)
		}
		return fmt.Sprintf("%s:%d", host, port)
	}

	return fmt.Sprintf("127.0.0.1:%d", port)
}
