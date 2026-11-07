package edge

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

func testAds() AdsOptions {
	ads, ok := normalizeAds(AdsOptions{SiteKey: "site_pub_edge_test"})
	if !ok {
		panic("test ads config should be valid")
	}
	return ads
}

func adServer() *Server {
	return &Server{ads: testAds(), adsOn: true}
}

func adResponse(method, contentType, body string) *http.Response {
	req, _ := http.NewRequest(method, "http://app.nevatal.id/page", nil)
	header := http.Header{}
	if contentType != "" {
		header.Set("Content-Type", contentType)
	}
	header.Set("ETag", `"upstream"`)
	header.Set("Last-Modified", "Mon, 02 Nov 2026 10:00:00 GMT")
	return &http.Response{
		StatusCode:    http.StatusOK,
		Request:       req,
		Header:        header,
		Body:          io.NopCloser(strings.NewReader(body)),
		ContentLength: int64(len(body)),
	}
}

func readResponse(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

func TestInjectsSnippetBeforeBodyEnd(t *testing.T) {
	s := adServer()
	page := "<!doctype html><html><head><title>x</title></head><body><p>hello</p></body></html>"
	resp := adResponse(http.MethodGet, "text/html; charset=utf-8", page)
	if err := s.injectAds(resp); err != nil {
		t.Fatalf("injectAds: %v", err)
	}
	body := readResponse(t, resp)
	for _, want := range []string{
		`data-site-key="site_pub_edge_test"`,
		`data-adcontrol-placement="edge-banner"`,
		adInjectMarker,
		`src="https://marketing.nevatal.id/sdk/v1/adcontrol.js"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body does not contain %q:\n%s", want, body)
		}
	}
	if strings.Index(body, adInjectMarker) > strings.Index(body, "</body>") {
		t.Errorf("snippet was injected after </body>: %s", body)
	}
	if resp.Header.Get("ETag") != "" || resp.Header.Get("Last-Modified") != "" {
		t.Errorf("stale validators survived injection: %v", resp.Header)
	}
	if got := resp.Header.Get("Content-Length"); got != "" && got != strconv.Itoa(len(body)) {
		t.Errorf("Content-Length = %q, want %d", got, len(body))
	}
	if resp.ContentLength != int64(len(body)) {
		t.Errorf("ContentLength = %d, want %d", resp.ContentLength, len(body))
	}
}

func TestInjectsAtEndWhenBodyTagMissing(t *testing.T) {
	s := adServer()
	resp := adResponse(http.MethodGet, "text/html", "<html><body><p>no closer")
	if err := s.injectAds(resp); err != nil {
		t.Fatalf("injectAds: %v", err)
	}
	body := readResponse(t, resp)
	if !strings.Contains(body, "no closer") || !strings.HasSuffix(strings.TrimSpace(body), "</div>") {
		t.Fatalf("snippet not appended at end: %s", body)
	}
}

func TestInjectionSkipsNonHTML(t *testing.T) {
	s := adServer()
	cases := map[string]struct {
		method      string
		contentType string
		body        string
		encoded     bool
	}{
		"json":     {http.MethodGet, "application/json", `{"ok":true}`, false},
		"no type":  {http.MethodGet, "", "<html><body>x</body></html>", false},
		"post":     {http.MethodPost, "text/html", "<html><body>x</body></html>", false},
		"encoded":  {http.MethodGet, "text/html", "<html><body>x</body></html>", true},
		"fragment": {http.MethodGet, "text/html", "<div>fragment</div>", false},
	}
	for name, tc := range cases {
		resp := adResponse(tc.method, tc.contentType, tc.body)
		if tc.encoded {
			resp.Header.Set("Content-Encoding", "gzip")
		}
		if err := s.injectAds(resp); err != nil {
			t.Fatalf("%s: injectAds: %v", name, err)
		}
		if got := readResponse(t, resp); got != tc.body {
			t.Errorf("%s: body changed: %q", name, got)
		}
	}
}

func TestInjectionIsIdempotent(t *testing.T) {
	s := adServer()
	page := "<html><body><!-- " + adInjectMarker + " --></body></html>"
	resp := adResponse(http.MethodGet, "text/html", page)
	if err := s.injectAds(resp); err != nil {
		t.Fatalf("injectAds: %v", err)
	}
	if got := readResponse(t, resp); got != page {
		t.Fatalf("already-injected page changed: %q", got)
	}
}

func TestInjectionSkipsOversizedPages(t *testing.T) {
	s := adServer()
	page := "<html><body>" + strings.Repeat("x", maxAdInjectBytes) + "</body></html>"
	resp := adResponse(http.MethodGet, "text/html", page)
	if err := s.injectAds(resp); err != nil {
		t.Fatalf("injectAds: %v", err)
	}
	if got := readResponse(t, resp); got != page {
		t.Fatalf("oversized page changed (len %d vs %d)", len(got), len(page))
	}
}

func TestInjectionDisabledWithoutSiteKey(t *testing.T) {
	ads, ok := normalizeAds(AdsOptions{})
	if ok {
		t.Fatal("empty site key must disable injection")
	}
	if ads.SiteKey != "" {
		t.Fatalf("unexpected site key %q", ads.SiteKey)
	}
	if ads.SDKURL != DefaultAdSDKURL || ads.PlacementKey != DefaultAdPlacementKey {
		t.Fatalf("defaults not applied: %+v", ads)
	}

	s := &Server{ads: ads} // adsOn defaults to false
	page := "<html><body>x</body></html>"
	resp := adResponse(http.MethodGet, "text/html", page)
	if err := s.injectAds(resp); err != nil {
		t.Fatalf("injectAds: %v", err)
	}
	if got := readResponse(t, resp); got != page {
		t.Fatalf("disabled injection changed the page: %q", got)
	}
}

func TestNormalizeAdsRejectsUnsafeValues(t *testing.T) {
	for name, opts := range map[string]AdsOptions{
		"http sdk":      {SiteKey: "site_pub_x", SDKURL: "http://marketing.nevatal.id/sdk.js"},
		"bad site key":  {SiteKey: `site_pub_x"onload=`},
		"bad placement": {SiteKey: "site_pub_x", PlacementKey: `x"><script>`},
		"userinfo":      {SiteKey: "site_pub_x", SDKURL: "https://user:pass@marketing.nevatal.id/sdk.js"},
	} {
		if _, ok := normalizeAds(opts); ok {
			t.Errorf("%s: expected invalid config to be rejected", name)
		}
	}
}
