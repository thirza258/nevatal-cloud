package edge

import (
	"bytes"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// AdsOptions configures the AdControl snippet that nevatal-edge injects into
// HTML pages proxied to customer VMs. An empty SiteKey disables injection.
//
// The public site key is safe to expose. Its AdControl site must list every
// hostname the edge serves (for example *.nevatal.app) in allowed_domains.
type AdsOptions struct {
	// SDKURL is the absolute HTTPS URL of adcontrol.js.
	SDKURL string
	// SiteKey is the public site key (site_pub_...).
	SiteKey string
	// PlacementKey is the placement the injected container addresses.
	PlacementKey string
}

const (
	// DefaultAdSDKURL is the production AdControl browser SDK.
	DefaultAdSDKURL = "https://marketing.nevatal.id/sdk/v1/adcontrol.js"
	// DefaultAdPlacementKey is the placement created for edge-injected ads.
	DefaultAdPlacementKey = "edge-banner"

	// adInjectMarker makes injection idempotent when an upstream app already
	// contains the snippet.
	adInjectMarker = "adcontrol-edge"
	// maxAdInjectBytes bounds the page buffered for injection. Larger pages
	// stream through untouched.
	maxAdInjectBytes = 4 << 20
)

var (
	adKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,63}$`)
)

// normalizeAds applies defaults and reports whether injection is usable.
func normalizeAds(opts AdsOptions) (AdsOptions, bool) {
	opts.SDKURL = strings.TrimSpace(opts.SDKURL)
	opts.SiteKey = strings.TrimSpace(opts.SiteKey)
	opts.PlacementKey = strings.TrimSpace(opts.PlacementKey)

	if opts.SDKURL == "" {
		opts.SDKURL = DefaultAdSDKURL
	}
	if opts.PlacementKey == "" {
		opts.PlacementKey = DefaultAdPlacementKey
	}
	if opts.SiteKey == "" {
		return opts, false
	}
	if !adKeyPattern.MatchString(opts.SiteKey) || !adKeyPattern.MatchString(opts.PlacementKey) {
		return opts, false
	}
	u, err := url.Parse(opts.SDKURL)
	if err != nil || !strings.EqualFold(u.Scheme, "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
		return opts, false
	}
	return opts, true
}

// adSnippet is the markup appended before </body>.
func adSnippet(opts AdsOptions) string {
	return "\n<!-- " + adInjectMarker + " -->\n" +
		`<script async src="` + html.EscapeString(opts.SDKURL) + `" data-site-key="` + html.EscapeString(opts.SiteKey) + `"></script>` + "\n" +
		`<div data-adcontrol-placement="` + html.EscapeString(opts.PlacementKey) + `" style="max-width:728px;margin:1.25rem auto;padding:0 16px;box-sizing:border-box"></div>` + "\n"
}

// injectAds adds the AdControl loader and placement container to proxied HTML.
// It is fail-open: responses it cannot handle pass through untouched.
func (s *Server) injectAds(resp *http.Response) error {
	if !s.adsOn || resp.Request == nil || resp.Request.Method != http.MethodGet {
		return nil
	}
	if resp.StatusCode != http.StatusOK || resp.Body == nil {
		return nil
	}
	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if !strings.HasPrefix(contentType, "text/html") || resp.Header.Get("Content-Encoding") != "" {
		return nil
	}

	original := resp.Body
	buf, err := io.ReadAll(io.LimitReader(original, maxAdInjectBytes+1))
	if err != nil {
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), original))
		return nil
	}
	if len(buf) > maxAdInjectBytes {
		// Too large to buffer: restore the consumed prefix and stream it all.
		resp.Body = io.NopCloser(io.MultiReader(bytes.NewReader(buf), original))
		return nil
	}
	_ = original.Close()

	lower := bytes.ToLower(buf)
	if bytes.Contains(lower, []byte(adInjectMarker)) || !bytes.Contains(lower, []byte("<html")) {
		resp.Body = io.NopCloser(bytes.NewReader(buf))
		resp.ContentLength = int64(len(buf))
		return nil
	}

	snippet := adSnippet(s.ads)
	var out bytes.Buffer
	out.Grow(len(buf) + len(snippet))
	if i := bytes.LastIndex(lower, []byte("</body>")); i >= 0 {
		out.Write(buf[:i])
		out.WriteString(snippet)
		out.Write(buf[i:])
	} else {
		out.Write(buf)
		out.WriteString(snippet)
	}

	body := out.Bytes()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", strconv.Itoa(len(body)))
	// The injected bytes invalidate upstream validators; a 304 would serve
	// the page without the ad or with a stale ETag.
	resp.Header.Del("ETag")
	resp.Header.Del("Last-Modified")
	return nil
}
