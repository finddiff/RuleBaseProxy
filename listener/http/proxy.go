package http

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/finddiff/RuleBaseProxy/adapter/inbound"
	"github.com/finddiff/RuleBaseProxy/common/cache"
	N "github.com/finddiff/RuleBaseProxy/common/net"
	C "github.com/finddiff/RuleBaseProxy/constant"
	authStore "github.com/finddiff/RuleBaseProxy/listener/auth"
	"github.com/finddiff/RuleBaseProxy/log"
)

func HandleConn(c net.Conn, in chan<- C.ConnContext, cache *cache.Cache) {
	client := newClient(c.RemoteAddr(), in)
	defer client.CloseIdleConnections()

	conn := N.NewBufferedConn(c)

	keepAlive := true
	trusted := cache == nil // disable authenticate if cache is nil

	for keepAlive {
		request, err := ReadRequest(conn.Reader())
		if err != nil {
			break
		}

		request.RemoteAddr = conn.RemoteAddr().String()

		keepAlive = strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive"

		var resp *http.Response

		if !trusted {
			resp = authenticate(request, cache)

			trusted = resp == nil
		}

		if trusted {
			if request.Method == http.MethodConnect {
				//resp = responseWith(200)
				//resp.Status = "Connection established"
				//resp.ContentLength = -1
				//
				//if resp.Write(conn) != nil {
				//	break // close connection
				//}

				// Manual writing to support CONNECT for http 1.0 (workaround for uplay client)
				if _, err = fmt.Fprintf(conn, "HTTP/%d.%d %03d %s\r\n\r\n", request.ProtoMajor, request.ProtoMinor, http.StatusOK, "Connection established"); err != nil {
					break // close connection
				}

				in <- inbound.NewHTTPS(request, conn)

				return // hijack connection
			}

			host := request.Header.Get("Host")
			if host != "" {
				request.Host = host
			}

			request.RequestURI = ""

			RemoveHopByHopHeaders(request.Header)
			RemoveExtraHTTPHostPort(request)

			if request.URL.Scheme == "" || request.URL.Host == "" {
				resp = responseWith(http.StatusBadRequest)
			} else {
				resp, err = client.Do(request)
				if err != nil {
					resp = responseWith(http.StatusBadGateway)
				}
			}
		}

		RemoveHopByHopHeaders(resp.Header)

		if keepAlive {
			resp.Header.Set("Proxy-Connection", "keep-alive")
			resp.Header.Set("Connection", "keep-alive")
			resp.Header.Set("Keep-Alive", "timeout=4")
		}

		resp.Close = !keepAlive

		err = resp.Write(conn)
		if err != nil {
			break // close connection
		}
	}

	conn.Close()
}

func authenticate(request *http.Request, cache *cache.Cache) *http.Response {
	authenticator := authStore.Authenticator()
	if authenticator != nil {
		credential := ParseBasicProxyAuthorization(request)
		if credential == "" {
			resp := responseWith(http.StatusProxyAuthRequired)
			resp.Header.Set("Proxy-Authenticate", "Basic")
			return resp
		}

		var authed interface{}
		if authed = cache.Get(credential); authed == nil {
			user, pass, err := DecodeBasicProxyAuthorization(credential)
			authed = err == nil && authenticator.Verify(user, pass)
			cache.Put(credential, authed, time.Minute)
		}
		if !authed.(bool) {
			log.Infoln("Auth failed from %s", request.RemoteAddr)

			return responseWith(http.StatusForbidden)
		}
	}

	return nil
}

func responseWith(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{},
	}
}
