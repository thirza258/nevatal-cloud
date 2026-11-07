package edge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/repository"
)

// fakeSource serves a mutable snapshot and counts loads.
type fakeSource struct {
	mu    sync.Mutex
	snap  repository.EdgeSnapshot
	err   error
	loads int
}

func (f *fakeSource) LoadEdgeSnapshot(context.Context) (repository.EdgeSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	return f.snap, f.err
}

func (f *fakeSource) set(mut func(*fakeSource)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	mut(f)
}

func (f *fakeSource) loadCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loads
}

func strp(s string) *string { return &s }

func liveRoute(host, ip string, port int) repository.EdgeRoute {
	return repository.EdgeRoute{
		Hostname: host, TargetPort: port, Enabled: true, OwnerStatus: "active",
		InstanceFound: true, InstanceStatus: strp("running"), PrivateIP: strp(ip),
	}
}

// testEdge is an edge in front of one real backend. The VM address
// 10.10.42.20:8080 is mapped to the backend's loopback listener by the test
// transport (the production policy never allows loopback targets).
type testEdge struct {
	t       *testing.T
	source  *fakeSource
	server  *Server
	front   *httptest.Server
	backend *httptest.Server

	mu   sync.Mutex
	seen *http.Request // last request the backend received
}

func newTestEdge(t *testing.T) *testEdge {
	t.Helper()
	te := &testEdge{t: t}

	te.backend = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			echoWebSocket(t, w, r)
			return
		}
		te.mu.Lock()
		te.seen = r.Clone(context.Background())
		te.mu.Unlock()
		w.Header().Set("X-Backend", "yes")
		if r.URL.Path == "/page" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<!doctype html><html><body><p>hello page</p></body></html>")
			return
		}
		fmt.Fprintf(w, "hello from %s%s", r.Host, r.URL.RequestURI())
	}))
	t.Cleanup(te.backend.Close)

	vmAddrs := map[string]string{"10.10.42.20:8080": te.backend.Listener.Addr().String()}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			real, ok := vmAddrs[addr]
			if !ok {
				return nil, &net.OpError{Op: "dial", Net: network, Err: errors.New("connection refused")}
			}
			var d net.Dialer
			return d.DialContext(ctx, network, real)
		},
	}

	te.source = &fakeSource{snap: repository.EdgeSnapshot{
		Routes: []repository.EdgeRoute{
			liveRoute("app.nevatal.id", "10.10.42.20", 8080),
			liveRoute("dead.nevatal.id", "10.10.42.40", 9),
			liveRoute("outside.nevatal.id", "192.168.1.5", 8080),
			liveRoute("denied.nevatal.id", "10.10.42.1", 8080),
			liveRoute("dup.nevatal.id", "10.10.42.30", 8080),
			liveRoute("loop.nevatal.id", "127.0.0.1", 8080),
			func() repository.EdgeRoute {
				r := liveRoute("stopped.nevatal.id", "10.10.42.21", 8080)
				r.InstanceStatus = strp("stopped")
				return r
			}(),
			func() repository.EdgeRoute {
				r := liveRoute("off.nevatal.id", "10.10.42.22", 8080)
				r.Enabled = false
				return r
			}(),
			func() repository.EdgeRoute {
				r := liveRoute("blocked.nevatal.id", "10.10.42.23", 8080)
				r.AdminDisabled = true
				return r
			}(),
			{Hostname: "gone.nevatal.id", TargetPort: 8080, Enabled: true, OwnerStatus: "active"},
		},
		PrivateIPs: []string{"10.10.42.20", "10.10.42.21", "10.10.42.30", "10.10.42.30", "10.10.42.40"},
	}}

	te.server = NewServer(Options{
		Root: "nevatal.id",
		Policy: domainroute.Policy{
			TargetCIDRs: []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24")},
			DenyIPs:     []netip.Addr{netip.MustParseAddr("10.10.42.1")},
		},
		Source:          te.source,
		Transport:       transport,
		RefreshInterval: time.Hour, // tests reload explicitly
		Logger:          log.New(io.Discard, "", 0),
		Ads:             AdsOptions{SiteKey: "site_pub_edge_test"},
	})
	if err := te.server.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	te.front = httptest.NewServer(te.server)
	t.Cleanup(te.front.Close)
	return te
}

func (te *testEdge) do(host, path string, header map[string]string) (*http.Response, string) {
	te.t.Helper()
	req, err := http.NewRequest(http.MethodGet, te.front.URL+path, nil)
	if err != nil {
		te.t.Fatal(err)
	}
	req.Host = host
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		te.t.Fatalf("GET %s%s: %v", host, path, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, string(body)
}

func (te *testEdge) lastSeen() *http.Request {
	te.mu.Lock()
	defer te.mu.Unlock()
	return te.seen
}

func expectPage(t *testing.T, resp *http.Response, body string, status int, contains string) {
	t.Helper()
	if resp.StatusCode != status {
		t.Fatalf("status = %d, want %d (body %q)", resp.StatusCode, status, body)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q, want no-store", got)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("Content-Type = %q, want an HTML page", resp.Header.Get("Content-Type"))
	}
	if !strings.Contains(body, contains) {
		t.Fatalf("page does not contain %q:\n%s", contains, body)
	}
	if strings.Contains(body, "10.10.42.") || strings.Contains(body, "192.168.") || strings.Contains(body, "127.0.0.1") {
		t.Fatalf("page reveals an internal address:\n%s", body)
	}
	if resp.Header.Get("X-Backend") != "" {
		t.Fatalf("an edge page must not come from the backend")
	}
}

func TestUnknownAndForeignHostsAre404(t *testing.T) {
	te := newTestEdge(t)
	for _, host := range []string{
		"nope.nevatal.id",        // unknown label
		"app.example.com",        // another root
		"app.nevatal.id.evil.io", // root only as a substring
		"evilnevatal.id",         // shares the suffix without the dot
		"a.app.nevatal.id",       // nested subdomain
		"nevatal.id",             // the apex is not an app
		"under_score.nevatal.id",
		"off.nevatal.id",     // disabled by its owner
		"blocked.nevatal.id", // disabled by an administrator
		"gone.nevatal.id",    // instance deleted
	} {
		t.Run(host, func(t *testing.T) {
			resp, body := te.do(host, "/", nil)
			expectPage(t, resp, body, http.StatusNotFound, "No app here")
		})
	}
}

func TestStoppedInstanceIs503(t *testing.T) {
	te := newTestEdge(t)
	resp, body := te.do("stopped.nevatal.id", "/", nil)
	expectPage(t, resp, body, http.StatusServiceUnavailable, "not running")
}

func TestUnroutableAddressesAre502(t *testing.T) {
	te := newTestEdge(t)
	for _, host := range []string{"outside.nevatal.id", "denied.nevatal.id", "dup.nevatal.id", "loop.nevatal.id"} {
		t.Run(host, func(t *testing.T) {
			resp, body := te.do(host, "/", nil)
			expectPage(t, resp, body, http.StatusBadGateway, "be reached")
		})
	}
}

func TestUpstreamNotRespondingIs502WithPort(t *testing.T) {
	te := newTestEdge(t)
	resp, body := te.do("dead.nevatal.id", "/", nil)
	expectPage(t, resp, body, http.StatusBadGateway, "did not respond on port 9")
}

func TestProxiesWithPublicHostAndRebuiltForwardingHeaders(t *testing.T) {
	te := newTestEdge(t)
	resp, body := te.do("APP.Nevatal.ID.:443", "/api/items?page=2", map[string]string{
		"Cf-Connecting-Ip":                   "203.0.113.9",
		"Cf-Visitor":                         `{"scheme":"https"}`,
		"Cf-Ray":                             "8f1e2d3c4b5a6978-SIN",
		"X-Forwarded-For":                    "6.6.6.6",
		"X-Forwarded-Host":                   "evil.example",
		"X-Forwarded-User":                   "admin",
		"X-Real-Ip":                          "6.6.6.6",
		"Forwarded":                          "for=6.6.6.6",
		"Cf-Access-Authenticated-User-Email": "ceo@nevatal.id",
		"Authorization":                      "Bearer app-token",
	})
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Backend") != "yes" {
		t.Fatalf("expected the backend response, got %d %q", resp.StatusCode, body)
	}
	if body != "hello from app.nevatal.id/api/items?page=2" {
		t.Fatalf("unexpected body %q", body)
	}

	seen := te.lastSeen()
	want := map[string]string{
		"X-Forwarded-For":                    "203.0.113.9",
		"X-Real-Ip":                          "203.0.113.9",
		"X-Forwarded-Proto":                  "https",
		"X-Forwarded-Host":                   "app.nevatal.id",
		"Cf-Connecting-Ip":                   "203.0.113.9",
		"Cf-Ray":                             "8f1e2d3c4b5a6978-SIN",
		"X-Forwarded-User":                   "",
		"Forwarded":                          "",
		"Cf-Access-Authenticated-User-Email": "",
		"Authorization":                      "Bearer app-token",
	}
	for name, value := range want {
		if got := seen.Header.Get(name); got != value {
			t.Errorf("backend saw %s = %q, want %q", name, got, value)
		}
	}
	if got := seen.Header.Values("X-Forwarded-For"); len(got) != 1 {
		t.Errorf("X-Forwarded-For must be a single value, got %q", got)
	}
}

func TestInjectsAdControlIntoProxiedHTML(t *testing.T) {
	te := newTestEdge(t)
	resp, body := te.do("app.nevatal.id", "/page", nil)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-Backend") != "yes" {
		t.Fatalf("expected the backend page, got %d %q", resp.StatusCode, body)
	}
	for _, want := range []string{
		`data-site-key="site_pub_edge_test"`,
		`data-adcontrol-placement="edge-banner"`,
		`src="https://marketing.nevatal.id/sdk/v1/adcontrol.js"`,
		adInjectMarker,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("proxied page does not contain %q:\n%s", want, body)
		}
	}
	if strings.Index(body, adInjectMarker) > strings.Index(body, "</body>") {
		t.Errorf("snippet was injected after </body>: %s", body)
	}
	if got := resp.Header.Get("Content-Length"); got != "" && got != fmt.Sprint(len(body)) {
		t.Errorf("Content-Length = %q, want %d", got, len(body))
	}
}

func TestForwardingHeadersWithoutCloudflare(t *testing.T) {
	te := newTestEdge(t)
	te.do("app.nevatal.id", "/", map[string]string{"X-Forwarded-For": "6.6.6.6"})
	seen := te.lastSeen()
	if got := seen.Header.Get("X-Forwarded-For"); got != "127.0.0.1" {
		t.Fatalf("without Cf-Connecting-Ip the peer address is used, got %q", got)
	}
	if got := seen.Header.Get("X-Forwarded-Proto"); got != "http" {
		t.Fatalf("plain request must be http, got %q", got)
	}

	// cloudflared reports the visitor scheme in X-Forwarded-Proto.
	te.do("app.nevatal.id", "/", map[string]string{"X-Forwarded-Proto": "https"})
	if got := te.lastSeen().Header.Get("X-Forwarded-Proto"); got != "https" {
		t.Fatalf("X-Forwarded-Proto=https from cloudflared must be kept, got %q", got)
	}

	// An invalid Cf-Connecting-Ip is ignored.
	te.do("app.nevatal.id", "/", map[string]string{"Cf-Connecting-Ip": "not-an-ip"})
	if got := te.lastSeen().Header.Get("X-Forwarded-For"); got != "127.0.0.1" {
		t.Fatalf("invalid Cf-Connecting-Ip must fall back to the peer, got %q", got)
	}
}

func TestWebSocketUpgrade(t *testing.T) {
	te := newTestEdge(t)
	conn, err := net.Dial("tcp", te.front.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	fmt.Fprintf(conn, "GET /socket HTTP/1.1\r\nHost: app.nevatal.id\r\nConnection: Upgrade\r\nUpgrade: websocket\r\n"+
		"Sec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n")
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Fatalf("reading upgrade response: %v", err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("status = %d, want 101", resp.StatusCode)
	}
	if _, err := conn.Write([]byte("ping\n")); err != nil {
		t.Fatal(err)
	}
	line, err := br.ReadString('\n')
	if err != nil || line != "echo: ping\n" {
		t.Fatalf("echo over the upgraded connection = %q, %v", line, err)
	}
}

// echoWebSocket is a minimal upgrade handler: it switches protocols and
// echoes lines.
func echoWebSocket(t *testing.T, w http.ResponseWriter, r *http.Request) {
	conn, brw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		t.Errorf("backend hijack: %v", err)
		return
	}
	defer conn.Close()
	fmt.Fprintf(brw, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n")
	_ = brw.Flush()
	for {
		line, err := brw.ReadString('\n')
		if err != nil {
			return
		}
		fmt.Fprintf(brw, "echo: %s", line)
		_ = brw.Flush()
	}
}

func TestHealth(t *testing.T) {
	te := newTestEdge(t)
	resp, body := te.do("localhost:8088", HealthPath, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health status = %d (%s)", resp.StatusCode, body)
	}
	var health struct {
		Status      string     `json:"status"`
		Routes      int        `json:"routes"`
		Live        int        `json:"live"`
		LastRefresh *time.Time `json:"last_refresh"`
	}
	if err := json.Unmarshal([]byte(body), &health); err != nil {
		t.Fatalf("health is not JSON: %v (%s)", err, body)
	}
	if health.Status != "ok" || health.Routes != 10 || health.Live != 2 || health.LastRefresh == nil {
		t.Fatalf("unexpected health %+v", health)
	}

	// On an app hostname the path belongs to the app.
	resp, body = te.do("app.nevatal.id", HealthPath, nil)
	if resp.Header.Get("X-Backend") != "yes" || !strings.Contains(body, HealthPath) {
		t.Fatalf("health path on an app host must be proxied, got %d %q", resp.StatusCode, body)
	}
}

func TestHealthBeforeFirstLoad(t *testing.T) {
	s := NewServer(Options{Root: "nevatal.id", Source: &fakeSource{}, Logger: log.New(io.Discard, "", 0)})
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8088"+HealthPath, nil))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), `"starting"`) {
		t.Fatalf("health before the first load = %d %s", rec.Code, rec.Body.String())
	}
}

func TestUnknownHostTriggersRateLimitedReload(t *testing.T) {
	te := newTestEdge(t)
	te.source.set(func(f *fakeSource) {
		f.snap.Routes = append(f.snap.Routes, liveRoute("fresh.nevatal.id", "10.10.42.20", 8080))
	})
	before := te.source.loadCount()

	resp, _ := te.do("fresh.nevatal.id", "/", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("a domain claimed after the last tick must be served at once, got %d", resp.StatusCode)
	}
	if got := te.source.loadCount() - before; got != 1 {
		t.Fatalf("expected exactly one reload, got %d", got)
	}

	// More unknown hosts within the same second do not reload again.
	for i := 0; i < 5; i++ {
		te.do(fmt.Sprintf("random%d.nevatal.id", i), "/", nil)
	}
	if got := te.source.loadCount() - before; got != 1 {
		t.Fatalf("reloads after misses must be limited to one per second, got %d", got)
	}
	// Hosts outside the root never reload.
	te.server.lastMiss = time.Time{}
	te.do("app.example.com", "/", nil)
	if got := te.source.loadCount() - before; got != 1 {
		t.Fatalf("foreign hosts must not trigger a reload, got %d", got)
	}
}

func TestFailedReloadKeepsLastSnapshot(t *testing.T) {
	te := newTestEdge(t)
	te.source.set(func(f *fakeSource) {
		f.err = errors.New("database is down")
		f.snap = repository.EdgeSnapshot{}
	})
	if err := te.server.Refresh(context.Background()); err == nil {
		t.Fatalf("Refresh must report the source error")
	}
	if resp, _ := te.do("app.nevatal.id", "/", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("the last good snapshot must keep serving, got %d", resp.StatusCode)
	}
}

func TestTransportRefusesAddressesOutsideThePolicy(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer backend.Close()

	transport := NewTransport(domainroute.Policy{TargetCIDRs: []netip.Prefix{netip.MustParsePrefix("10.10.42.0/24")}})
	req, _ := http.NewRequest(http.MethodGet, backend.URL, nil)
	_, err := transport.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("dialing a loopback backend must be refused, got %v", err)
	}
}
