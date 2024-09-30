package dns

import (
	"bufio"
	"errors"
	C "github.com/finddiff/RuleBaseProxy/constant"
	R "github.com/finddiff/RuleBaseProxy/rule"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/finddiff/RuleBaseProxy/common/sockopt"
	"github.com/finddiff/RuleBaseProxy/context"
	"github.com/finddiff/RuleBaseProxy/log"

	D "github.com/miekg/dns"
)

var (
	address string
	server  = &Server{}

	dnsDefaultTTL uint32 = 600
)

type Server struct {
	*D.Server
	handler handler
}

// ServeDNS implement D.Handler ServeDNS
func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	msg, err := handlerWithContext(s.handler, r, w.RemoteAddr())
	if err != nil {
		D.HandleFailed(w, r)
		return
	}
	msg.Compress = true
	w.WriteMsg(msg)
}

func handlerWithContext(handler handler, msg *D.Msg, raddr net.Addr) (*D.Msg, error) {
	if len(msg.Question) == 0 {
		return nil, errors.New("at least one question is required")
	}

	ctx := context.NewDNSContext(msg, raddr)
	return handler(ctx, msg)
}

func (s *Server) setHandler(handler handler) {
	s.handler = handler
}

func AdgurdFile2Rule(file string) {
	//打开文件
	fi, err := os.Open(C.Path.HomeDir() + "/" + file)
	if err != nil {
		return
	}
	defer fi.Close()

	if re, err := regexp.Compile("\\|\\|.+\\^"); err == nil {
		buf := bufio.NewScanner(fi)
		for {
			if !buf.Scan() {
				break //文件读完了,退出for
			}
			line := buf.Text() //获取每一行

			if st := re.FindString(line); st != "" {
				st = strings.Replace(st, "||", "", -1)
				st = strings.Replace(st, "^", "", -1)
				if rule, err := R.ParseRule("DOMAIN-SUFFIX", st, "ADGURD_MATCH", []string{""}); err == nil {
					ADGurdRules = append(ADGurdRules, rule)
				}
			}
		}
	}
}

func ReCreateServer(addr string, resolver *Resolver, mapper *ResolverEnhancer) error {
	if addr == address && resolver != nil {
		handler := newHandler(resolver, mapper)
		server.setHandler(handler)
		return nil
	}

	if server.Server != nil {
		server.Shutdown()
		server = &Server{}
		address = ""
	}

	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return nil
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	p, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	err = sockopt.UDPReuseaddr(p)
	if err != nil {
		log.Warnln("Failed to Reuse UDP Address: %s", err)
	}

	address = addr
	handler := newHandler(resolver, mapper)
	server = &Server{handler: handler}
	server.Server = &D.Server{Addr: addr, PacketConn: p, Handler: server}
	loadNDSCache(resolver, mapper)

	go func() {
		server.ActivateAndServe()
	}()
	return nil
}
