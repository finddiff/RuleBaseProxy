package dns

import (
	//C "github.com/finddiff/RuleBaseProxy/constant"
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

func withPreHandle() middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			q := r.Question[0]
			ctx.Host = strings.TrimRight(q.Name, ".")
			return next(ctx, r)
		}
	}
}

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
			} else if v6 := ip.To16(); v4 == nil && ipv6 && v6 != nil && q.Qtype == D.TypeAAAA {
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

func withADGurd() middleware {
	return func(next handler) handler {
		return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
			//q := r.Question[0]
			//host := strings.TrimRight(q.Name, ".")
			if ADGurdMatch(ctx.Host) {
				log.Debugln("%s from %s ADGurdMatch block", ctx.Host, ctx.RemoteAddr().String())
				return r, nil
			} else {
				return next(ctx, r)
			}
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

			//host := strings.TrimRight(q.Name, ".")
			all_eq := false

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
				//if all_eq {
				//	continue
				//}
				if host, ok := mapping.Get(ip.String()); ok {
					if host.(string) == ctx.Host {
						all_eq = true
					}
				}
				log.Debugln("ResolverEnhancer mapping.SetWithExpire [%s] => [%s]", ip.String(), ctx.Host)
				mapping.SetWithExpire(ip.String(), ctx.Host, time.Now().Add(time.Second*time.Duration(ttl)))

				if all_eq {
					all_eq = false
					continue
				}

				DnsMapAdd(DnsMap{
					ipstr:  ip.String(),
					domain: ctx.Host,
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

			//host := strings.TrimRight(q.Name, ".")
			if fakePool.LookupHost(ctx.Host) {
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
			ip := fakePool.Lookup(ctx.Host)
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

func dualQuery(resolver *Resolver, r *D.Msg) (msg *D.Msg, err error) {
	// 同时查询IPv4和IPv6
	// 1. 创建AAAA查询消息
	aaaaMsg := r.Copy()
	aaaaMsg.Question[0].Qtype = D.TypeAAAA

	// 2. 并发执行两个查询
	type result struct {
		msg *D.Msg
		err error
	}

	resultCh := make(chan result, 2)

	// 查询A记录（IPv4）
	go func() {
		msg, err := resolver.Exchange(r)
		resultCh <- result{msg: msg, err: err}
	}()

	// 查询AAAA记录（IPv6）
	go func() {
		msg, err := resolver.Exchange(aaaaMsg)
		resultCh <- result{msg: msg, err: err}
	}()

	// 3. 收集结果并合并
	var aResult, aaaaResult result
	for i := 0; i < 2; i++ {
		res := <-resultCh
		if res.msg != nil && len(res.msg.Question) > 0 {
			if res.msg.Question[0].Qtype == D.TypeA {
				aResult = res
			} else {
				aaaaResult = res
			}
		}
	}

	log.Debugln("[DNS Server] Merged query:%s  aResult:%v ", r.Question[0].String(), aResult.msg)
	log.Debugln("[DNS Server] Merged query:%s  aaaaResult:%v ", r.Question[0].String(), aaaaResult.msg)

	// 4. 合并结果
	var finalMsg *D.Msg
	if aResult.err == nil && aResult.msg != nil && aResult.msg.Rcode == D.RcodeSuccess {
		finalMsg = aResult.msg
		// 如果有AAAA结果且有效，合并到最终消息中
		if aaaaResult.err == nil && aaaaResult.msg != nil && len(aaaaResult.msg.Answer) > 0 {
			finalMsg.Answer = append(finalMsg.Answer, aaaaResult.msg.Answer...)
			// 合并其他部分（Authority和Additional记录）
			finalMsg.Ns = append(finalMsg.Ns, aaaaResult.msg.Ns...)
			finalMsg.Extra = append(finalMsg.Extra, aaaaResult.msg.Extra...)
		}
	} else if aaaaResult.err == nil && aaaaResult.msg != nil && aaaaResult.msg.Rcode == D.RcodeSuccess {
		// 如果只有AAAA结果有效，使用它
		finalMsg = aaaaResult.msg
	} else {
		// 如果都失败，返回第一个错误
		finalMsg = aResult.msg
	}
	if finalMsg == nil {
		log.Errorln("[DNS Server] Merged dualQuery failed: both A and AAAA queries failed")
		return handleMsgWithEmptyAnswer(r), nil
	}
	finalMsg.SetRcode(r, finalMsg.Rcode)
	finalMsg.Authoritative = true
	finalMsg.RecursionAvailable = true

	log.Debugln("[DNS Server] Merged IPv4 and IPv6 results for %s", r.Question[0].String())
	return finalMsg, nil
}

func withResolver(resolver *Resolver) handler {
	return func(ctx *context.DNSContext, r *D.Msg) (*D.Msg, error) {
		ctx.SetType(context.DNSTypeRaw)
		q := r.Question[0]

		// return a empty AAAA msg when ipv6 disabled
		if !resolver.ipv6 && q.Qtype == D.TypeAAAA {
			return handleMsgWithEmptyAnswer(r), nil
		}

		//if q.Qtype == D.TypeAAAA {
		//	return handleMsgWithEmptyAnswer(r), nil
		//}

		// 检查是否需要同时查询IPv4和IPv6
		needDualQuery := resolver.ipv6 && q.Qtype == D.TypeA
		if needDualQuery {
			return dualQuery(resolver, r)
		}

		msg, err := resolver.Exchange(r)
		if err != nil {
			log.Debugln("[DNS Server] Exchange %s failed: %v", q.String(), err)
			return msg, err
		}

		log.Debugln("[DNS Server] Exchange %s query-type:%s, query-msg: %s,  response-msg: %s", q.String(), D.Type(r.Question[0].Qtype).String(), strings.Replace(r.String(), "\n", "@n@", -1), strings.Replace(msg.String(), "\n", "@n@", -1))
		msg.SetRcode(r, msg.Rcode)
		msg.Authoritative = true
		msg.RecursionAvailable = true

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
	middlewares := []middleware{withPreHandle()}

	if resolver.hosts != nil {
		middlewares = append(middlewares, withHosts(resolver.hosts, resolver.ipv6))
	}

	if len(ADGurdRules) > 0 {
		middlewares = append(middlewares, withADGurd())
	}

	if mapper.mode == FAKEIP {
		middlewares = append(middlewares, withFakeIP(mapper.fakePool))
	}

	if mapper.mode != NORMAL {
		middlewares = append(middlewares, withMapping(mapper.mapping))
	}

	return compose(middlewares, withResolver(resolver))
}
