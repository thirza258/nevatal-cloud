package edge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/nevatal/cloud/internal/domainroute"
)

const dialTimeout = 5 * time.Second

var errTargetNotAllowed = errors.New("edge: target address not allowed by EDGE_TARGET_CIDRS/EDGE_DENY_IPS")

type routeKey struct{}

func routeFrom(ctx context.Context) *route {
	rt, _ := ctx.Value(routeKey{}).(*route)
	return rt
}

// NewTransport returns the transport the edge proxies with: plain HTTP to
// the VM, a 5 second dial timeout, a keep-alive pool and no overall request
// timeout (long polling, uploads, WebSockets). It never uses an environment
// proxy, and its dialer refuses any address the policy does not allow, as
// a second line of defense behind the routing table.
func NewTransport(policy domainroute.Policy) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   dialTimeout,
		KeepAlive: 30 * time.Second,
		Control: func(_, address string, _ syscall.RawConn) error {
			target, err := netip.ParseAddrPort(address)
			if err != nil || !policy.Allows(target.Addr()) {
				return errTargetNotAllowed
			}
			return nil
		},
	}
	return &http.Transport{
		Proxy:                 nil,
		DialContext:           dialer.DialContext,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   32,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}

func (s *Server) newProxy(transport http.RoundTripper) *httputil.ReverseProxy {
	return &httputil.ReverseProxy{
		Rewrite:        s.rewrite,
		Transport:      transport,
		ModifyResponse: s.injectAds,
		ErrorHandler:   s.proxyError,
		ErrorLog:       s.logger,
	}
}

// rewrite sends the request to the VM. The public Host is kept, and the
// forwarding headers are rebuilt from what Cloudflare and cloudflared
// established: anything a client sent itself under X-Forwarded-*, Cf-*,
// Forwarded or X-Real-Ip is dropped. (ReverseProxy has already removed
// hop-by-hop headers, Forwarded and X-Forwarded-For/Host/Proto.)
func (s *Server) rewrite(pr *httputil.ProxyRequest) {
	rt := routeFrom(pr.In.Context())
	pr.SetURL(&url.URL{Scheme: "http", Host: rt.target.String()})
	pr.Out.Host = rt.host

	stripForwardingHeaders(pr.Out.Header)

	// Let Go's transport negotiate gzip and decompress it transparently, so
	// the response body ModifyResponse sees is HTML we can inject into. (The
	// edge re-compresses nothing; the compressed hops are browser/Cloudflare
	// to edge and Go transport's gzip to the VM.)
	pr.Out.Header.Del("Accept-Encoding")

	client := clientIP(pr.In)
	pr.Out.Header.Set("X-Forwarded-For", client)
	pr.Out.Header.Set("X-Real-Ip", client)
	pr.Out.Header.Set("X-Forwarded-Host", rt.host)
	pr.Out.Header.Set("X-Forwarded-Proto", forwardedProto(pr.In))

	// Request metadata set by Cloudflare that apps commonly read.
	if ip, ok := cfConnectingIP(pr.In); ok {
		pr.Out.Header.Set("Cf-Connecting-Ip", ip)
	}
	for _, name := range []string{"Cf-Ray", "Cf-Ipcountry", "Cf-Visitor"} {
		if v := pr.In.Header.Get(name); v != "" {
			pr.Out.Header.Set(name, v)
		}
	}
}

// stripForwardingHeaders removes every header that could make an app
// believe something about the client or the path the request took.
func stripForwardingHeaders(h http.Header) {
	for name := range h {
		if isForwardingHeader(name) {
			delete(h, name)
		}
	}
}

func isForwardingHeader(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "cf-") {
		return true
	}
	switch lower {
	case "forwarded", "x-real-ip", "true-client-ip", "x-client-ip", "x-cluster-client-ip", "x-original-forwarded-for":
		return true
	}
	return false
}

// clientIP is the visitor address: Cf-Connecting-Ip (set by Cloudflare),
// otherwise the address of the peer that connected to the edge.
func clientIP(r *http.Request) string {
	if ip, ok := cfConnectingIP(r); ok {
		return ip
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

func cfConnectingIP(r *http.Request) (string, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")))
	if err != nil || addr.Zone() != "" {
		return "", false
	}
	return addr.Unmap().String(), true
}

// forwardedProto is "https" when the visitor used HTTPS: Cloudflare says so
// in Cf-Visitor ({"scheme":"https"}) and cloudflared in X-Forwarded-Proto.
func forwardedProto(r *http.Request) string {
	if r.TLS != nil || strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return "https"
	}
	var visitor struct {
		Scheme string `json:"scheme"`
	}
	if raw := r.Header.Get("Cf-Visitor"); raw != "" && json.Unmarshal([]byte(raw), &visitor) == nil && strings.EqualFold(visitor.Scheme, "https") {
		return "https"
	}
	return "http"
}

// proxyError answers when the VM did not accept or answer the request. The
// page names the port but never the VM address.
func (s *Server) proxyError(w http.ResponseWriter, r *http.Request, err error) {
	rt := routeFrom(r.Context())
	if !errors.Is(err, context.Canceled) {
		s.logger.Printf("edge: %s: upstream error: %v", rt.host, err)
	}
	writePage(w, http.StatusBadGateway, pageNotResponding, rt.host, rt.port)
}

// serveHealth reports the routing table: 200 once a snapshot was loaded
// ("stale" when the last reload is older than 30 seconds), 503 before.
func (s *Server) serveHealth(w http.ResponseWriter) {
	snap := s.current()
	body := struct {
		LastRefresh *time.Time `json:"last_refresh"`
		Live        int        `json:"live"`
		Routes      int        `json:"routes"`
		Status      string     `json:"status"`
	}{Live: snap.live, Routes: len(snap.routes), Status: "starting"}
	code := http.StatusServiceUnavailable
	if !snap.loadedAt.IsZero() {
		loaded := snap.loadedAt.UTC()
		body.LastRefresh = &loaded
		body.Status = "ok"
		if time.Since(snap.loadedAt) > 30*time.Second {
			body.Status = "stale"
		}
		code = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

// statusRecorder captures the response status for the access log. It
// exposes the wrapped writer through Unwrap (http.ResponseController uses it
// for Flush) and implements Hijack so WebSocket upgrades keep working.
type statusRecorder struct {
	http.ResponseWriter
	code int
}

func (r *statusRecorder) WriteHeader(code int) {
	if r.code == 0 && code >= 200 {
		r.code = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	conn, brw, err := http.NewResponseController(r.ResponseWriter).Hijack()
	if err == nil && r.code == 0 {
		r.code = http.StatusSwitchingProtocols
	}
	return conn, brw, err
}

func (r *statusRecorder) status() int {
	if r.code == 0 {
		return http.StatusOK
	}
	return r.code
}
