# AdControl on customer sites (nevatal-edge)

Nevatal Cloud is free. To fund it, `nevatal-edge` can add one labelled
[AdControl](https://marketing.nevatal.id) placement to the HTML pages it proxies
to customer VMs.

The feature is **off unless `ADCONTROL_SITE_KEY` is set**. It never touches the
control plane (`cloud.nevatal.id`), API responses, non-HTML content, or the
edge's own error pages.

## How it works

For a proxied response, the edge injects this markup immediately before
`</body>` (or appends it when the document has no closing body tag):

```html
<!-- adcontrol-edge -->
<script async src="https://marketing.nevatal.id/sdk/v1/adcontrol.js"
        data-site-key="site_pub_…"></script>
<div data-adcontrol-placement="edge-banner"
     style="max-width:728px;margin:1.25rem auto;padding:0 16px;box-sizing:border-box"></div>
```

The AdControl SDK loads its own stylesheet, fetches the placement
configuration from the ad origin, renders an accessible banner with an `Ad`
badge, and reports impressions/clicks through the signed click redirects.

Injection rules (`backend/internal/edge/ads.go`):

| Response | Injected? |
| --- | --- |
| `GET`, status `200`, `Content-Type: text/html…`, full document (`<html`) | Yes, up to 4 MiB |
| Already contains the `adcontrol-edge` marker | No (idempotent) |
| Non-HTML, compressed (`Content-Encoding` set), `HEAD`/`POST`, other status | No |
| Larger than 4 MiB | No (streams through untouched) |
| Page with a strict `Content-Security-Policy` that blocks the ad origin | Script is blocked by the browser; the page still works |

The edge removes `ETag` and `Last-Modified` from rewritten pages so a cached
validator cannot serve a version without the ad. The request's
`Accept-Encoding` is dropped, letting Go's transport negotiate gzip and
decompress it, so the injected body is plain HTML.

## AdControl setup

1. Create a site in AdControl whose `allowed_domains` cover every hostname the
   edge serves, e.g. `*.nevatal.app` (the wildcard form is supported). The
   public site key is safe to expose.
2. Create a placement with key `edge-banner` (format `responsive_banner`,
   `fallback_behavior: hide`) and at least one published, active campaign.
3. Put the site key in the edge environment and restart the edge.

This deployment already has:

| Object | Value |
| --- | --- |
| AdControl site | `Nevatal Cloud Customer Apps` — `ae2db5f2-3859-d2e1-68ac-1f83e3b0720b` |
| Public site key | `site_pub_51725c05b8678a68421e5585` |
| Allowed domains | `*.nevatal.id`, `*.nevatal.app` |
| Placement | `edge-banner` — `0b98e0ab-ec99-ee44-252c-383c6dd0aa77` |
| Campaign | `Nevatal Cloud — free VPS promo` (house, active) |

If `PUBLIC_DOMAIN_ROOT` is a different registrable domain, add a matching
wildcard to the site's allowed domains in **Websites → Edit**.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `ADCONTROL_SDK_URL` | `https://marketing.nevatal.id/sdk/v1/adcontrol.js` | HTTPS URL of the browser SDK |
| `ADCONTROL_SITE_KEY` | empty | Public site key; empty disables injection |
| `ADCONTROL_PLACEMENT_KEY` | `edge-banner` | Placement addressed by the injected container |

The same variables are wired into the `edge` service in
[`docker-compose.yml`](../docker-compose.yml).

## Enable and verify

The edge itself needs `PUBLIC_DOMAIN_ROOT` and `EDGE_TARGET_CIDRS`; see
[PROXMOX_INSTALL.md section 20](PROXMOX_INSTALL.md#20-cloudflare-tunnel-and-railway-style-subdomains-nevatalid).

```bash
# .env (production)
COMPOSE_PROFILES=edge,tunnel
PUBLIC_DOMAIN_ROOT=nevatal.app          # or the tenant domain you use
ADCONTROL_SITE_KEY=site_pub_51725c05b8678a68421e5585
ADCONTROL_PLACEMENT_KEY=edge-banner

./deploy.sh --mode production
```

Verify through the edge (replace the hostname with one of your published VMs):

```bash
curl -sS https://myapp.nevatal.app/ | grep -o 'data-site-key="[^"]*"'
curl -sS https://myapp.nevatal.app/ | grep -o 'data-adcontrol-placement="[^"]*"'
```

Check the AdControl delivery API with the same origin:

```bash
curl -sS 'https://marketing.nevatal.id/delivery/v1/sites/site_pub_51725c05b8678a68421e5585/config' \
  -H 'Origin: https://myapp.nevatal.app'
```

An empty `placements` object means no active campaign, the site is paused, or
the origin is not in `allowed_domains`.

Disable injection by unsetting `ADCONTROL_SITE_KEY` and restarting the edge; no
code change is needed.

## Disclosure

The Terms of Service (section 4, **Published Sites & Advertising**) tells
customers that pages published through Nevatal Domains may carry a labelled
AdControl advertisement. Keep that clause in sync if the behaviour changes.

The AdControl origin must stay reachable over HTTPS from customers' browsers.
If it is down, the SDK fails open and the rest of the page is unaffected.
