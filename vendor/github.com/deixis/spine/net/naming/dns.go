package naming

// Based on gRPC naming pkg
// https://raw.githubusercontent.com/grpc/grpc-go/master/naming/dns_resolver.go

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/deixis/spine/log"
)

const (
	defaultPort = "443"
	defaultFreq = time.Minute * 30
	defaultSRV  = "spine"
)

var (
	lookupHost = net.DefaultResolver.LookupHost
	lookupSRV  = net.DefaultResolver.LookupSRV

	errMissingAddr = errors.New("missing address")
)

// DNS creates a DNS Resolver that can resolve DNS names, and create watchers
// that poll the DNS server using the frequency set by freq or the default
// frequency defined by defaultFreq.
func DNS(ctx context.Context, freq ...time.Duration) Resolver {
	if len(freq) > 0 {
		return &dnsResolver{ctx: ctx, freq: freq[0]}
	}
	return &dnsResolver{ctx: ctx, freq: defaultFreq}
}

func buildDNS(ctx context.Context, uri *url.URL) (Watcher, error) {
	target := strings.TrimPrefix(uri.Path, "/")

	fp := uri.Query().Get("freq")
	if fp == "" {
		return DNS(ctx).Resolve(target)
	}

	f, err := strconv.ParseInt(fp, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing DNS update frequency")
	}
	return DNS(ctx, time.Second*time.Duration(f)).Resolve(target)
}

// dnsResolver handles name resolution for names following the DNS scheme
type dnsResolver struct {
	// ctx is the context on which the DNS resolver runs
	ctx context.Context
	// frequency of polling the DNS server that the watchers created by this resolver will use.
	freq time.Duration
}

// Resolve creates a watcher that watches the name resolution of the target.
func (r *dnsResolver) Resolve(target string) (Watcher, error) {
	host, port, err := parseTarget(target)
	if err != nil {
		return nil, err
	}

	if net.ParseIP(host) != nil {
		ipWatcher := &ipWatcher{
			updateChan: make(chan *Update, 1),
		}
		host, _ = formatIP(host)
		ipWatcher.updateChan <- &Update{Op: Add, Addr: host + ":" + port}
		return ipWatcher, nil
	}

	ctx, cancel := context.WithCancel(r.ctx)
	return &dnsWatcher{
		r:      r,
		host:   host,
		port:   port,
		ctx:    ctx,
		cancel: cancel,
		logger: log.FromContext(ctx),
		t:      time.NewTimer(0),
	}, nil
}

// dnsWatcher watches for the name resolution update for a specific target
type dnsWatcher struct {
	r    *dnsResolver
	host string
	port string
	// The latest resolved address set
	curAddrs map[string]*Update
	ctx      context.Context
	cancel   context.CancelFunc
	logger   log.Logger
	t        *time.Timer
}

// ipWatcher watches for the name resolution update for an IP address.
type ipWatcher struct {
	updateChan chan *Update
}

// Next returns the adrress resolution Update for the target. For IP address,
// the resolution is itself, thus polling name server is unncessary. Therefore,
// Next() will return an Update the first time it is called, and will be blocked
// for all following calls as no Update exisits until watcher is closed.
func (i *ipWatcher) Next() ([]*Update, error) {
	u, ok := <-i.updateChan
	if !ok {
		return nil, ErrWatcherClosed
	}
	return []*Update{u}, nil
}

// Close closes the ipWatcher.
func (i *ipWatcher) Close() error {
	close(i.updateChan)
	return nil
}

// compileUpdate compares the old resolved addresses and newly resolved addresses,
// and generates an update list
func (w *dnsWatcher) compileUpdate(newAddrs map[string]*Update) []*Update {
	var res []*Update
	for a, u := range w.curAddrs {
		if _, ok := newAddrs[a]; !ok {
			u.Op = Delete
			res = append(res, u)
		}
	}
	for a, u := range newAddrs {
		if _, ok := w.curAddrs[a]; !ok {
			res = append(res, u)
		}
	}
	return res
}

func (w *dnsWatcher) lookupSRV() map[string]*Update {
	newAddrs := make(map[string]*Update)
	_, srvs, err := lookupSRV(w.ctx, defaultSRV, "tcp", w.host)
	if err != nil {
		w.logger.Trace("naming.dns.srv.fail", "Failed dns SRV record lookup",
			log.Error(err),
		)
		return nil
	}
	for _, s := range srvs {
		lbAddrs, err := lookupHost(w.ctx, s.Target)
		if err != nil {
			w.logger.Warning(
				"naming.dns.srv.fail",
				"Failed load balancer address dns lookup",
				log.Error(err),
			)
			continue
		}
		for _, a := range lbAddrs {
			a, ok := formatIP(a)
			if !ok {
				w.logger.Warning("naming.dns.srv.err", "Failed IP parsing",
					log.Error(err),
				)
				continue
			}
			addr := a + ":" + strconv.Itoa(int(s.Port))
			newAddrs[addr] = &Update{Addr: addr}
		}
	}
	return newAddrs
}

func (w *dnsWatcher) lookupHost() map[string]*Update {
	newAddrs := make(map[string]*Update)
	addrs, err := lookupHost(w.ctx, w.host)
	if err != nil {
		w.logger.Trace("naming.dns.a.fail", "Failed dns A record lookup",
			log.Error(err),
		)
		return nil
	}
	for _, a := range addrs {
		a, ok := formatIP(a)
		if !ok {
			w.logger.Warning("naming.dns.a.err", "Failed IP parsing",
				log.Error(err),
			)
			continue
		}
		addr := a + ":" + w.port
		newAddrs[addr] = &Update{Addr: addr}
	}
	return newAddrs
}

func (w *dnsWatcher) lookup() []*Update {
	newAddrs := w.lookupSRV()
	if newAddrs == nil {
		// If failed to get any balancer address (either no corresponding SRV for the
		// target, or caused by failure during resolution/parsing of the balancer target),
		// return any A record info available.
		newAddrs = w.lookupHost()
	}
	result := w.compileUpdate(newAddrs)
	w.curAddrs = newAddrs
	return result
}

// Next returns the resolved address update(delta) for the target. If there's no
// change, it will sleep for 30 mins and try to resolve again after that.
func (w *dnsWatcher) Next() ([]*Update, error) {
	for {
		select {
		case <-w.ctx.Done():
			return nil, ErrWatcherClosed
		case <-w.t.C:
		}
		result := w.lookup()
		// Next lookup should happen after an interval defined by w.r.freq.
		w.t.Reset(w.r.freq)
		if len(result) > 0 {
			return result, nil
		}
	}
}

func (w *dnsWatcher) Close() error {
	w.cancel()
	return nil
}

// formatIP returns ok = false if addr is not a valid textual representation of an IP address.
// If addr is an IPv4 address, return the addr and ok = true.
// If addr is an IPv6 address, return the addr enclosed in square brackets and ok = true.
func formatIP(addr string) (addrIP string, ok bool) {
	ip := net.ParseIP(addr)
	if ip == nil {
		return "", false
	}
	if ip.To4() != nil {
		return addr, true
	}
	return "[" + addr + "]", true
}

// parseTarget takes the user input target string, returns formatted host and port info.
// If target doesn't specify a port, set the port to be the defaultPort.
// If target is in IPv6 format and host-name is enclosed in sqarue brackets, brackets
// are strippd when setting the host.
// examples:
// target: "www.google.com" returns host: "www.google.com", port: "443"
// target: "ipv4-host:80" returns host: "ipv4-host", port: "80"
// target: "[ipv6-host]" returns host: "ipv6-host", port: "443"
// target: ":80" returns host: "localhost", port: "80"
// target: ":" returns host: "localhost", port: "443"
func parseTarget(target string) (host, port string, err error) {
	if target == "" {
		return "", "", errMissingAddr
	}

	if ip := net.ParseIP(target); ip != nil {
		// target is an IPv4 or IPv6(without brackets) address
		return target, defaultPort, nil
	}
	if host, port, err := net.SplitHostPort(target); err == nil {
		// target has port, i.e ipv4-host:port, [ipv6-host]:port, host-name:port
		if host == "" {
			// Keep consistent with net.Dial(): If the host is empty, as in ":80", the local system is assumed.
			host = "localhost"
		}
		if port == "" {
			// If the port field is empty(target ends with colon), e.g. "[::1]:", defaultPort is used.
			port = defaultPort
		}
		return host, port, nil
	}
	if host, port, err := net.SplitHostPort(target + ":" + defaultPort); err == nil {
		// target doesn't have port
		return host, port, nil
	}
	return "", "", fmt.Errorf("invalid target address %v", target)
}
