// Package edge is nevatal-edge, the reverse proxy behind Nevatal Domains:
//
//	browser -> Cloudflare (wildcard DNS, TLS) -> Cloudflare Tunnel -> cloudflared
//	        -> nevatal-edge -> customer VM private IP:port
//
// The edge only reads the database. It keeps a snapshot of the active
// domains, reloaded every 2 seconds (and, at most once per second, when a
// request names a hostname it does not know yet), and decides with
// domainroute.Policy (the function behind the API's live_status) which
// hostnames it may proxy and where to.
package edge

import (
	"context"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// HealthPath answers the edge's health probe on any host outside the public
// root (localhost, the container name, an IP), so it never shadows a path of
// a customer app.
const HealthPath = "/__edge/health"

const (
	defaultRefreshInterval = 2 * time.Second
	missRefreshInterval    = time.Second
	refreshTimeout         = 5 * time.Second
	missRefreshTimeout     = 2 * time.Second
)

// Source loads the routing data (repository.DomainRepository in production).
type Source interface {
	LoadEdgeSnapshot(ctx context.Context) (repository.EdgeSnapshot, error)
}

// Options configure a Server.
type Options struct {
	// Root is PUBLIC_DOMAIN_ROOT (lowercase, no leading or trailing dot).
	// Only <label>.<Root> hosts are proxied.
	Root string
	// Policy holds EDGE_TARGET_CIDRS and EDGE_DENY_IPS.
	Policy domainroute.Policy
	Source Source
	// Transport sends proxied requests; nil means NewTransport(Policy).
	Transport http.RoundTripper
	// RefreshInterval between background reloads; 0 means 2 seconds.
	RefreshInterval time.Duration
	// Logger receives the access log and errors; nil means log.Default().
	Logger *log.Logger
}

// route is one active hostname and what the edge does with it.
type route struct {
	host   string
	status domainroute.Status
	port   int
	target netip.AddrPort // set when status is StatusLive
}

type snapshot struct {
	routes   map[string]*route
	live     int
	loadedAt time.Time
}

// Server is the nevatal-edge HTTP handler.
type Server struct {
	root     string
	suffix   string
	policy   domainroute.Policy
	source   Source
	proxy    *httputil.ReverseProxy
	interval time.Duration
	logger   *log.Logger

	snap atomic.Pointer[snapshot]

	refreshMu sync.Mutex // one reload at a time
	missMu    sync.Mutex // single-flight reloads triggered by unknown hosts
	lastMiss  time.Time
}

// NewServer returns an edge with an empty routing table; call Refresh before
// serving and Run to keep it current.
func NewServer(opts Options) *Server {
	s := &Server{
		root:     opts.Root,
		suffix:   "." + opts.Root,
		policy:   opts.Policy,
		source:   opts.Source,
		interval: opts.RefreshInterval,
		logger:   opts.Logger,
	}
	if s.interval <= 0 {
		s.interval = defaultRefreshInterval
	}
	if s.logger == nil {
		s.logger = log.Default()
	}
	transport := opts.Transport
	if transport == nil {
		transport = NewTransport(opts.Policy)
	}
	s.proxy = s.newProxy(transport)
	s.snap.Store(&snapshot{routes: map[string]*route{}})
	return s
}

// Refresh reloads the routing snapshot. On error the previous snapshot stays
// in use.
func (s *Server) Refresh(ctx context.Context) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, refreshTimeout)
	defer cancel()
	data, err := s.source.LoadEdgeSnapshot(ctx)
	if err != nil {
		return err
	}
	s.snap.Store(s.build(data, time.Now()))
	return nil
}

// Run reloads the snapshot every refresh interval until ctx is done. Errors
// are logged when the reload starts failing and when it recovers.
func (s *Server) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	failing := false
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		err := s.Refresh(ctx)
		switch {
		case err != nil && !failing:
			failing = true
			s.logger.Printf("edge: route refresh failed, serving the last snapshot: %v", err)
		case err == nil && failing:
			failing = false
			s.logger.Printf("edge: route refresh recovered")
		}
	}
}

// build evaluates every active domain with the shared routing policy.
func (s *Server) build(data repository.EdgeSnapshot, now time.Time) *snapshot {
	ipUsers := domainroute.CountAddrs(data.PrivateIPs)
	snap := &snapshot{routes: make(map[string]*route, len(data.Routes)), loadedAt: now}
	for _, r := range data.Routes {
		target := domainroute.Target{
			Enabled:       r.Enabled,
			AdminDisabled: r.AdminDisabled,
			OwnerActive:   r.OwnerStatus == models.UserStatusActive,
			InstanceFound: r.InstanceFound,
		}
		if r.InstanceStatus != nil {
			target.InstanceStatus = *r.InstanceStatus
		}
		if r.PrivateIP != nil {
			target.PrivateIP = *r.PrivateIP
		}
		status, addr := s.policy.Evaluate(target, ipUsers)
		rt := &route{host: strings.ToLower(r.Hostname), status: status, port: r.TargetPort}
		if status == domainroute.StatusLive {
			if r.TargetPort < 1 || r.TargetPort > 65535 {
				rt.status = domainroute.StatusIPNotRoutable
			} else {
				rt.target = netip.AddrPortFrom(addr, uint16(r.TargetPort))
				snap.live++
			}
		}
		snap.routes[rt.host] = rt
	}
	return snap
}

func (s *Server) current() *snapshot {
	return s.snap.Load()
}

// lookup returns the route of host. An unknown host triggers one reload so
// a domain claimed a moment ago works without waiting for the next tick.
func (s *Server) lookup(host string) *route {
	if rt := s.current().routes[host]; rt != nil {
		return rt
	}
	s.refreshAfterMiss()
	return s.current().routes[host]
}

// refreshAfterMiss reloads at most once per second, single-flight: requests
// that arrive during a reload wait for it instead of starting their own, so
// random hostnames cannot hammer the database.
func (s *Server) refreshAfterMiss() {
	s.missMu.Lock()
	defer s.missMu.Unlock()
	if time.Since(s.lastMiss) < missRefreshInterval {
		return
	}
	s.lastMiss = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), missRefreshTimeout)
	defer cancel()
	if err := s.Refresh(ctx); err != nil {
		s.logger.Printf("edge: route refresh after unknown host failed: %v", err)
	}
}

// ServeHTTP routes one request by its Host header.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := normalizeHost(r.Host)
	label, public := s.publicLabel(host)

	if !public && r.URL.Path == HealthPath {
		s.serveHealth(w)
		return
	}

	start := time.Now()
	rec := &statusRecorder{ResponseWriter: w}
	defer func() {
		logHost := host
		if logHost == "" {
			logHost = "-"
		}
		s.logger.Printf("%s %s %s %d %s", r.Method, logHost, r.URL.EscapedPath(), rec.status(), time.Since(start).Round(time.Millisecond))
	}()

	if label == "" {
		writePage(rec, http.StatusNotFound, pageNotFound, host, 0)
		return
	}
	rt := s.lookup(host)
	if rt == nil {
		writePage(rec, http.StatusNotFound, pageNotFound, host, 0)
		return
	}
	switch rt.status {
	case domainroute.StatusLive:
		s.proxy.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), routeKey{}, rt)))
	case domainroute.StatusDisabled, domainroute.StatusInstanceMissing:
		writePage(rec, http.StatusNotFound, pageNotFound, host, 0)
	case domainroute.StatusInstanceStopped:
		writePage(rec, http.StatusServiceUnavailable, pageStopped, host, 0)
	default: // no_ip, ip_not_routable, ip_conflict
		writePage(rec, http.StatusBadGateway, pageUnroutable, host, 0)
	}
}

// publicLabel splits host into the app label when host is <label>.<root>.
// public reports whether host is the root or anything under it; label is
// "" unless host is exactly one valid DNS label under the root.
func (s *Server) publicLabel(host string) (label string, public bool) {
	if host == s.root {
		return "", true
	}
	prefix, ok := strings.CutSuffix(host, s.suffix)
	if !ok {
		return "", false
	}
	if !isDNSLabel(prefix) {
		return "", true
	}
	return prefix, true
}

// normalizeHost lower-cases a Host header and strips the port and a
// trailing dot.
func normalizeHost(hostport string) string {
	host := strings.ToLower(strings.TrimSpace(hostport))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

// isDNSLabel reports whether s is one LDH label (1-63 characters, no
// leading or trailing hyphen).
func isDNSLabel(s string) bool {
	if len(s) == 0 || len(s) > 63 || s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
			return false
		}
	}
	return true
}
