package dns

import (
	C "github.com/finddiff/RuleBaseProxy/constant"
	"net"
	"strings"
	"time"

	"github.com/finddiff/RuleBaseProxy/common/cache"
	"github.com/finddiff/RuleBaseProxy/component/fakeip"
	"github.com/finddiff/RuleBaseProxy/component/trie"
	"github.com/finddiff/RuleBaseProxy/context"
	"github.com/finddiff/RuleBaseProxy/log"

	D "github.com/miekg/dns"
)

type handler func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error)
type middleware func(next handler) handler

func withHosts(hosts *trie.DomainTrie, ipv6 bool) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			if !isIPRequest(q) {
				return next(ctx, r)
			}

			record := hosts.Search(strings.TrimRight(q.Name, "."))
			if record == nil {
				return next(ctx, r)
			}

			ip := record.Data.(net.IP)
			msg := r.Copy()

			if v4 := ip.To4(); v4 != nil && q.Qtype == D.TypeA {
				rr := &D.A{}
				rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
				rr.A = v4

				msg.Answer = []D.RR{rr}
			} else if v6 := ip.To16(); ipv6 && v6 != nil && q.Qtype == D.TypeAAAA {
				rr := &D.AAAA{}
				rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeAAAA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
				rr.AAAA = v6

				msg.Answer = []D.RR{rr}
			} else {
				return next(ctx, r)
			}

			ctx.SetType(context.DNSTypeHost)
			msg.SetRcode(r, D.RcodeSuccess)
			msg.Authoritative = true
			msg.RecursionAvailable = true

			return msg, nil
		}
	}
}

func withMapping(mapping *cache.LruCache) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			if !isIPRequest(q) {
				return next(ctx, r)
			}

			msg, err := next(ctx, r)
			if err != nil {
				return nil, err
			}

			host := strings.TrimRight(q.Name, ".")
			all_eq := false

			if C.FilterCacheIP {
				// 透明代理模式，尽量保证一个ip 对应一个domain
				//new_Answer := make([]D.RR, 0)
				//pass_ip_count := 0
				//last_domain := ""
				//all_eq = true
				//for _, ans := range msg.Answer { // 对于存在缓存中的ip进行过滤
				//	var ip net.IP
				//
				//	switch a := ans.(type) {
				//	case *D.A:
				//		ip = a.A
				//	case *D.AAAA:
				//		ip = a.AAAA
				//	default:
				//		new_Answer = append(new_Answer, ans)
				//		continue
				//	}
				//
				//	if cache_domain, ok := mapping.Get(ip.String()); ok && cache_domain.(string) != host {
				//		// 过滤掉已经使用的ip - domain 映射
				//		if last_domain == "" {
				//			last_domain = cache_domain.(string)
				//		} else {
				//			if last_domain != cache_domain {
				//				all_eq = false
				//			}
				//		}
				//
				//		log.Debugln("FilterCacheIP ip:%s, host:%s, cache_domain:%s", ip.String(), host, cache_domain)
				//	} else {
				//		new_Answer = append(new_Answer, ans)
				//		pass_ip_count += 1
				//	}
				//}
				//
				//if last_domain == "" {
				//	all_eq = false
				//}
				//
				////如果过滤后，没有可以的ip 那就不过滤了
				//if pass_ip_count > 0 {
				//	msg.Answer = new_Answer
				//}
				//
				//log.Debugln("FilterCacheIP host:%s, all_eq:%v, pass_ip_count:%v", host, all_eq, pass_ip_count)
			}

			for _, ans := range msg.Answer {
				var ip net.IP
				var ttl uint32

				switch a := ans.(type) {
				case *D.A:
					ip = a.A
					ttl = a.Hdr.Ttl
				case *D.AAAA:
					ip = a.AAAA
					ttl = a.Hdr.Ttl
				default:
					continue
				}

				// 全部等价，就不用记录新值了
				if all_eq {
					continue
				}

				mapping.SetWithExpire(ip.String(), host, time.Now().Add(time.Second*time.Duration(ttl)))
				DnsMapAdd(DnsMap{
					ipstr:  ip.String(),
					domain: host,
					ttl:    ttl,
					raddr:  ctx.RemoteAddr(),
				})
			}

			return msg, nil
		}
	}
}

func withFakeIP(fakePool *fakeip.Pool) middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]

			host := strings.TrimRight(q.Name, ".")
			if fakePool.LookupHost(host) {
				return next(ctx, r)
			}

			switch q.Qtype {
			case D.TypeAAAA, D.TypeSVCB, D.TypeHTTPS:
				return handleMsgWithEmptyAnswer(r), nil
			}

			if q.Qtype != D.TypeA {
				return next(ctx, r)
			}

			rr := &D.A{}
			rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			ip := fakePool.Lookup(host)
			rr.A = ip
			msg := r.Copy()
			msg.Answer = []D.RR{rr}

			ctx.SetType(context.DNSTypeFakeIP)
			setMsgTTL(msg, 1)
			msg.SetRcode(r, D.RcodeSuccess)
			msg.Authoritative = true
			msg.RecursionAvailable = true

			return msg, nil
		}
	}
}

func withResolver(resolver *Resolver) handler {
	return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
		ctx.SetType(context.DNSTypeRaw)
		q := r.Question[0]

		// return a empty AAAA msg when ipv6 disabled
		if !resolver.ipv6 && q.Qtype == D.TypeAAAA {
			return handleMsgWithEmptyAnswer(r), nil
		}

		msg, err := resolver.Exchange(r)
		if err != nil {
			log.Debugln("[DNS Server] Exchange %s failed: %v", q.String(), err)
			return msg, err
		}

		log.Debugln("[DNS Server] Exchange %s msg: %s", q.String(), strings.Replace(msg.String(), "\n", "@n@", -1))
		msg.SetRcode(r, msg.Rcode)
		msg.Authoritative = true

		return msg, nil
	}
}

func compose(middlewares []middleware, endpoint handler) handler {
	length := len(middlewares)
	h := endpoint
	for i := length - 1; i >= 0; i-- {
		middleware := middlewares[i]
		h = middleware(h)
	}

	return h
}

func newHandler(resolver *Resolver, mapper *ResolverEnhancer) handler {
	middlewares := []middleware{}

	if resolver.hosts != nil {
		middlewares = append(middlewares, withHosts(resolver.hosts, resolver.ipv6))
	}

	if mapper.mode == FAKEIP {
		middlewares = append(middlewares, withFakeIP(mapper.fakePool))
	}

	if mapper.mode != NORMAL {
		middlewares = append(middlewares, withMapping(mapper.mapping))
	}

	return compose(middlewares, withResolver(resolver))
}
