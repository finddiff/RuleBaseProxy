package dns

import (
	"bytes"
	"context"
	C "github.com/finddiff/clashWithCache/constant"
	"github.com/finddiff/clashWithCache/log"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/finddiff/clashWithCache/component/dialer"
	"github.com/finddiff/clashWithCache/component/resolver"

	D "github.com/miekg/dns"
)

const (
	// dotMimeType is the DoH mimetype that should be used.
	dotMimeType = "application/dns-message"
)

type dohClient struct {
	url       string
	transport *http.Transport
}

func (dc *dohClient) Exchange(m *D.Msg) (msg *D.Msg, err error) {
	return dc.ExchangeContext(context.Background(), m)
}

func (dc *dohClient) ExchangeContext(ctx context.Context, m *D.Msg) (msg *D.Msg, err error) {
	req, err := dc.newRequest(m)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	return dc.doRequest(req)
}

// newRequest returns a new DoH request given a dns.Msg.
func (dc *dohClient) newRequest(m *D.Msg) (*http.Request, error) {
	buf, err := m.Pack()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, dc.url, bytes.NewReader(buf))
	if err != nil {
		return req, err
	}

	req.Header.Set("content-type", dotMimeType)
	req.Header.Set("accept", dotMimeType)
	return req, nil
}

func (dc *dohClient) doRequest(req *http.Request) (msg *D.Msg, err error) {
	client := &http.Client{Transport: dc.transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	msg = &D.Msg{}
	err = msg.Unpack(buf)
	log.Debugln("[DNS Server] dohClient doRequest %s msg: %s", dc.url, strings.Replace(msg.String(), "\n", "@n@", -1))
	return msg, err
}

func newDoHClient(dohurl string, r *Resolver) *dohClient {
	if len(C.DnsProxyString) > 0 {
		proxyUrl, _ := url.Parse(C.DnsProxyString)
		return &dohClient{
			url: dohurl,
			transport: &http.Transport{
				Proxy:             http.ProxyURL(proxyUrl),
				ForceAttemptHTTP2: true,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					host, port, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}

					ip, err := resolver.ResolveIPWithResolver(host, r)
					if err != nil {
						return nil, err
					}

					return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
				},
			},
		}
	} else {
		return &dohClient{
			url: dohurl,
			transport: &http.Transport{
				ForceAttemptHTTP2: true,
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					host, port, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}

					ip, err := resolver.ResolveIPWithResolver(host, r)
					if err != nil {
						return nil, err
					}

					return dialer.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
				},
			},
		}
	}
}
