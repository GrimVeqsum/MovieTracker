package httptransport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ForwardingPolicy trusts only explicitly configured immediate proxy peers.
// An empty policy is appropriate for a gateway exposed directly in development.
type ForwardingPolicy struct{ trusted []netip.Prefix }

type forwardingKey struct{}
type forwardingInfo struct{ ip, proto, host string }

func NewForwardingPolicy(cidrs []string) (*ForwardingPolicy, error) {
	p := &ForwardingPolicy{}
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", cidr, err)
		}
		p.trusted = append(p.trusted, prefix.Masked())
	}
	return p, nil
}

func (p *ForwardingPolicy) trusts(ip netip.Addr) bool {
	if p != nil {
		for _, prefix := range p.trusted {
			if prefix.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func (p *ForwardingPolicy) resolve(r *http.Request) forwardingInfo {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, _ := netip.ParseAddr(host)
	peer = peer.Unmap()
	info := forwardingInfo{ip: peer.String(), proto: "http", host: r.Host}
	if !peer.IsValid() {
		info.ip = "unknown"
	}
	if r.TLS != nil {
		info.proto = "https"
	}
	if !p.trusts(peer) {
		return info
	}
	// Walk from the immediate peer toward the client. Never select a
	// user-controlled leftmost address across an untrusted hop.
	chain := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	if len(chain) <= 32 {
		addresses := make([]netip.Addr, 0, len(chain))
		valid := true
		for _, part := range chain {
			ip, err := netip.ParseAddr(strings.TrimSpace(part))
			if err != nil {
				valid = false
				break
			}
			addresses = append(addresses, ip.Unmap())
		}
		if valid {
			current := peer
			for i := len(addresses) - 1; i >= 0 && p.trusts(current); i-- {
				current = addresses[i]
			}
			info.ip = current.String()
		}
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if len(r.Header.Values("X-Forwarded-Proto")) == 1 && (proto == "https" || proto == "http") {
		info.proto = proto
	}
	// Caddy preserves Host. X-Forwarded-Host supplied by a client is never used.
	return info
}

func requestForwarding(r *http.Request) forwardingInfo {
	if info, ok := r.Context().Value(forwardingKey{}).(forwardingInfo); ok {
		return info
	}
	return (*ForwardingPolicy)(nil).resolve(r)
}

func (p *ForwardingPolicy) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		info := p.resolve(r)
		r = r.WithContext(context.WithValue(r.Context(), forwardingKey{}, info))
		for _, name := range []string{"Forwarded", "X-Real-IP", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host"} {
			r.Header.Del(name)
		}
		// Bound request bodies before forwarding, including chunked requests.
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}
