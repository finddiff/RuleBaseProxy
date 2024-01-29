package dialer

import (
	"context"
	"errors"
	"net"
	"time"

	"github.com/finddiff/RuleBaseProxy/component/resolver"
)

func DialContext(ctx context.Context, network, address string, options ...Option) (net.Conn, error) {
	//log.Debugln("DialContext network:%s address:%s", network, address)
	switch network {
	case "tcp4", "tcp6", "udp4", "udp6":
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}

		var ip net.IP
		switch network {
		case "tcp4", "udp4":
			ip, err = resolver.ResolveIPv4(host)
		default:
			ip, err = resolver.ResolveIPv6(host)
		}
		if err != nil {
			return nil, err
		}

		return dialContext(ctx, network, ip, port, options)
	case "tcp", "udp":
		return dualStackDialContext(ctx, network, address, options)
	default:
		return nil, errors.New("network invalid")
	}
}

func ListenPacket(ctx context.Context, network, address string, options ...Option) (net.PacketConn, error) {
	cfg := &config{}

	if !cfg.skipDefault {
		for _, o := range DefaultOptions {
			o(cfg)
		}
	}

	for _, o := range options {
		o(cfg)
	}

	lc := &net.ListenConfig{}
	if cfg.interfaceName != "" {
		addr, err := bindIfaceToListenConfig(cfg.interfaceName, lc, network, address)
		if err != nil {
			return nil, err
		}
		address = addr
	}
	if cfg.addrReuse {
		addrReuseToListenConfig(lc)
	}

	return lc.ListenPacket(ctx, network, address)
}

func ExDialContext(ctx context.Context, network string, destination net.IP, port string, options ...Option) (net.Conn, error) {
	return dialContext(ctx, network, destination, port, options)
}

func DialContextHost(ctx context.Context, network string, host string, port string, options ...Option) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, error := dialer.DialContext(ctx, network, net.JoinHostPort(host, port))
	return conn, error
}

func dialContext(ctx context.Context, network string, destination net.IP, port string, options []Option) (net.Conn, error) {
	opt := &config{}

	if !opt.skipDefault {
		for _, o := range DefaultOptions {
			o(opt)
		}
	}

	for _, o := range options {
		o(opt)
	}
	//timeoutkey := fmt.Sprintf("%s %s", network, net.JoinHostPort(destination.String(), port))
	//timeout := 6 * time.Second
	//if item, ok := statistic.ManagerTimeout.Get(timeoutkey); ok && item != nil {
	//	timeout = item.(time.Duration) + time.Second
	//	//log.Debugln("dialContext load from cache timeout=%v", timeout)
	//}
	//log.Debugln("dialContext timeout=%v timeoutkey=%v", timeout, timeoutkey)
	//startDail := time.Now()
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	//endDail := time.Now()
	//duration := endDail.Sub(startDail)
	if opt.interfaceName != "" {
		if err := bindIfaceToDialer(opt.interfaceName, dialer, network, destination); err != nil {
			return nil, err
		}
	}

	conn, error := dialer.DialContext(ctx, network, net.JoinHostPort(destination.String(), port))
	//if error != nil && strings.Contains(error.Error(), "timeout") {
	//	timeout += time.Second
	//	if timeout > 6*time.Second {
	//		timeout = 6 * time.Second
	//	}
	//} else {
	//	timeout = (timeout + duration) / 2
	//}
	//log.Debugln("dialContext modify timeout=%v", timeout)
	//statistic.ManagerTimeout.SetWithTTL(timeoutkey, timeout, 1, time.Minute*60*24)
	return conn, error
}

func dualStackDialContext(ctx context.Context, network, address string, options []Option) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	returned := make(chan struct{})
	defer close(returned)

	type dialResult struct {
		net.Conn
		error
		resolved bool
		ipv6     bool
		done     bool
	}
	results := make(chan dialResult)
	var primary, fallback dialResult

	startRacer := func(ctx context.Context, network, host string, ipv6 bool) {
		result := dialResult{ipv6: ipv6, done: true}
		defer func() {
			select {
			case results <- result:
			case <-returned:
				if result.Conn != nil {
					result.Conn.Close()
				}
			}
		}()

		var ip net.IP
		if ipv6 {
			ip, result.error = resolver.ResolveIPv6(host)
		} else {
			ip, result.error = resolver.ResolveIPv4(host)
		}
		if result.error != nil {
			return
		}
		result.resolved = true

		result.Conn, result.error = dialContext(ctx, network, ip, port, options)
	}

	go startRacer(ctx, network+"4", host, false)
	go startRacer(ctx, network+"6", host, true)

	for res := range results {
		if res.error == nil {
			return res.Conn, nil
		}

		if !res.ipv6 {
			primary = res
		} else {
			fallback = res
		}

		if primary.done && fallback.done {
			if primary.resolved {
				return nil, primary.error
			} else if fallback.resolved {
				return nil, fallback.error
			} else {
				return nil, primary.error
			}
		}
	}

	return nil, errors.New("never touched")
}
