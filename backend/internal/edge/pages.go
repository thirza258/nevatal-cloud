package edge

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
)

// The pages the edge answers with itself. They are small, self-contained
// (no external resources), never cached and never mention the VM address.

type pageKind int

const (
	pageNotFound      pageKind = iota // unknown or disabled hostname
	pageStopped                       // instance not running
	pageUnroutable                    // no usable private address (missing, not allowed, shared)
	pageNotResponding                 // proxying failed
)

func pageText(kind pageKind, host string, port int) (title, message string) {
	switch kind {
	case pageStopped:
		return "This app is not running",
			"The server behind " + host + " is stopped right now. If this is your app, start its instance from the Nevatal Cloud dashboard."
	case pageUnroutable:
		return "This app can't be reached",
			"Nevatal Cloud can't route traffic to the server behind " + host + " right now. If this is your app, check the domain status in the Nevatal Cloud dashboard."
	case pageNotResponding:
		return "This app is not responding",
			"The app did not respond on port " + strconv.Itoa(port) + ". If this is your app, make sure it is running and listening on 0.0.0.0:" + strconv.Itoa(port) + " inside the server."
	default:
		if host == "" {
			return "No app here", "There is nothing published at this address."
		}
		return "No app here", "There is nothing published at " + host + "."
	}
}

const pageTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>%[1]d · %[2]s · Nevatal Cloud</title>
<style>
:root{color-scheme:light dark;--bg:#f6f7fb;--card:#fff;--text:#111827;--muted:#4b5563;--accent:#4f46e5}
@media (prefers-color-scheme:dark){:root{--bg:#0b0d14;--card:#151826;--text:#e5e7eb;--muted:#9ca3af;--accent:#818cf8}}
*{box-sizing:border-box}
body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;padding:16px;background:var(--bg);color:var(--text);font:16px/1.55 system-ui,-apple-system,"Segoe UI",Roboto,sans-serif}
main{max-width:480px;width:100%%;background:var(--card);border-radius:16px;padding:32px;box-shadow:0 10px 30px rgba(0,0,0,.08)}
.code{font-weight:700;color:var(--accent);letter-spacing:.08em}
h1{margin:.25em 0 .4em;font-size:1.5rem;line-height:1.25}
p{margin:0;color:var(--muted);overflow-wrap:anywhere}
footer{margin-top:24px;font-size:.85rem;color:var(--muted)}
</style>
</head>
<body>
<main>
<div class="code">%[1]d</div>
<h1>%[2]s</h1>
<p>%[3]s</p>
<footer>Served by Nevatal Cloud</footer>
</main>
</body>
</html>
`

// writePage answers with one of the edge's own pages.
func writePage(w http.ResponseWriter, status int, kind pageKind, host string, port int) {
	title, message := pageText(kind, host, port)
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; frame-ancestors 'none'")
	h.Set("Referrer-Policy", "no-referrer")
	w.WriteHeader(status)
	fmt.Fprintf(w, pageTemplate, status, html.EscapeString(title), html.EscapeString(message))
}
