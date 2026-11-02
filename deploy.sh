#!/usr/bin/env bash
#
# Nevatal Cloud - deployment script for Docker Compose hosts (e.g. a Proxmox VM).
# Needs only Docker Engine + the Compose v2 plugin (>= 2.24); everything is
# built and migrated inside containers.
#
#   cp deploy/env/demo.env.example .env          # or production.env.example
#   ./deploy.sh --mode demo                       # or --mode production
#
# Run ./deploy.sh --help for all options. Full guide: docs/PROXMOX_INSTALL.md

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

ENV_FILE="$ROOT_DIR/.env"
MIN_COMPOSE_VERSION="2.24.0"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-180}"

# Public defaults the backend rejects in production (internal/config/config.go).
DEV_JWT_SECRET="nevatal-cloud-super-secret-key-production-ready-32-chars"
LEGACY_WORDPRESS_KEY="wp_nevatal_cloud_api_key_secret_2026"
DEMO_ADMIN_PASSWORD='AdminPass123!'
DEMO_USER_PASSWORD='DemoPass123!'
MIN_JWT_SECRET_LENGTH=32
MIN_ADMIN_PASSWORD_LENGTH=12

MODE=""
ACTION="deploy"
BUILD=1
RUN_MIGRATE=1
ASSUME_YES=0

# ------------------------------------------------------------------ output ---
if [ -t 1 ]; then
  C_RESET=$'\033[0m'; C_BOLD=$'\033[1m'; C_BLUE=$'\033[1;34m'; C_GREEN=$'\033[1;32m'
  C_YELLOW=$'\033[1;33m'; C_RED=$'\033[1;31m'; C_CYAN=$'\033[1;36m'
else
  C_RESET=""; C_BOLD=""; C_BLUE=""; C_GREEN=""; C_YELLOW=""; C_RED=""; C_CYAN=""
fi

log()     { printf '%s==>%s %s%s%s\n' "$C_BLUE" "$C_RESET" "$C_BOLD" "$*" "$C_RESET"; }
success() { printf '%sOK%s  %s\n' "$C_GREEN" "$C_RESET" "$*"; }
warn()    { printf '%s[warn]%s %s\n' "$C_YELLOW" "$C_RESET" "$*" >&2; }
die()     { printf '%s[error]%s %s\n' "$C_RED" "$C_RESET" "$*" >&2; exit 1; }

usage() {
  cat <<'EOF'
Nevatal Cloud deployment script

Usage:
  ./deploy.sh [--mode demo|production] [OPTIONS]

The mode defaults to NEVATAL_MODE in .env. Create .env from a template first:
  cp deploy/env/demo.env.example .env         # public demo, simulated hypervisor
  cp deploy/env/production.env.example .env   # real Proxmox VE

Optional services come from COMPOSE_PROFILES in .env (comma-separated):
  tls      Caddy with Let's Encrypt (inbound 80/443)
  tunnel   cloudflared, Cloudflare Tunnel (no inbound ports; CLOUDFLARE_TUNNEL_TOKEN)
  edge     subdomain edge for https://<name>.PUBLIC_DOMAIN_ROOT (production only)
Services whose profile is removed are stopped and removed on the next deploy.

Actions (default: validate .env, build, migrate, start, health-check):
  --migrate-only     Apply pending database migrations and exit
  --rollback         Roll back ONE migration (stops the API first; asks to confirm)
  --status           Show containers, schema version, health, runtime mode,
                     edge and tunnel state
  --reset-demo       Demo mode only: delete the demo database + files, redeploy
                     (asks to confirm; use --yes for cron)

Options:
  --mode MODE        demo | production (development = plain docker-compose.yml)
  --no-build         Do not build images (use the local/pulled images)
  --skip-migrate     Do not run the migrate container before starting
                     (the API still migrates itself at start-up)
  -y, --yes          Do not ask for confirmation (--rollback, --reset-demo)
  -h, --help         Show this help

Examples:
  ./deploy.sh --mode production
  ./deploy.sh --status
  15 3 * * * cd /opt/nevatal-cloud && ./deploy.sh --reset-demo --yes --no-build
EOF
}

set_action() {
  if [ "$ACTION" != "deploy" ] && [ "$ACTION" != "$1" ]; then
    die "Options --$ACTION and --$1 cannot be combined."
  fi
  ACTION="$1"
}

while [ $# -gt 0 ]; do
  case "$1" in
    --mode)
      [ $# -ge 2 ] || die "--mode needs a value: demo or production"
      MODE="$2"
      shift
      ;;
    --mode=*) MODE="${1#*=}" ;;
    --migrate-only) set_action migrate-only ;;
    --rollback) set_action rollback ;;
    --status) set_action status ;;
    --reset-demo) set_action reset-demo ;;
    --skip-migrate) RUN_MIGRATE=0 ;;
    --no-build) BUILD=0 ;;
    -y|--yes) ASSUME_YES=1 ;;
    -h|--help) usage; exit 0 ;;
    *) die "Unknown option: $1 (run ./deploy.sh --help)" ;;
  esac
  shift
done

case "$MODE" in
  ""|demo|production|development) ;;
  *) die "Invalid --mode '$MODE' (expected demo or production)" ;;
esac

# --------------------------------------------------------------- .env I/O ---
# Read KEY from .env WITHOUT executing it (values may contain characters that
# are special to the shell). Mirrors Docker Compose: last assignment wins,
# surrounding quotes are removed, " #" starts a comment in unquoted values.
dotenv_get() {
  awk -v key="$1" -v sq="'" '
    /^[[:space:]]*(#|$)/ { next }
    {
      line = $0
      sub(/^[[:space:]]*export[[:space:]]+/, "", line)
      eq = index(line, "=")
      if (eq == 0) next
      name = substr(line, 1, eq - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
      if (name != key) next
      val = substr(line, eq + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
      first = substr(val, 1, 1)
      last = substr(val, length(val), 1)
      if (length(val) >= 2 && first == last && (first == "\"" || first == sq)) {
        val = substr(val, 2, length(val) - 2)
      } else {
        c = index(val, " #")
        if (c > 0) { val = substr(val, 1, c - 1); gsub(/[[:space:]]+$/, "", val) }
      }
      found = val
    }
    END { printf "%s", found }
  ' "$ENV_FILE"
}

# Effective value as Docker Compose sees it: shell environment first, then
# .env, then the given default.
cfg() {
  local key="$1" default="${2:-}" value
  if [ -n "${!key+x}" ]; then
    value="${!key}"
  else
    value="$(dotenv_get "$key")"
  fi
  printf '%s' "${value:-$default}"
}

is_true() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|t|true|y|yes|on) return 0 ;;
    *) return 1 ;;
  esac
}

confirm() {
  local prompt="$1" answer
  [ "$ASSUME_YES" -eq 1 ] && return 0
  if [ ! -t 0 ]; then
    die "$prompt -- refusing without a terminal; re-run with --yes to confirm."
  fi
  printf '%s%s%s [y/N] ' "$C_YELLOW" "$prompt" "$C_RESET"
  read -r answer
  case "$answer" in
    y|Y|yes|YES) return 0 ;;
    *) die "Aborted." ;;
  esac
}

version_ge() { # version_ge CURRENT REQUIRED
  [ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | sed -n 1p)" = "$2" ]
}

# ------------------------------------------------------------ load config ---
if [ ! -f "$ENV_FILE" ]; then
  if [ -n "$MODE" ] && [ "$MODE" != "development" ]; then
    die ".env not found. Create it from the $MODE template, fill in every CHANGE_ME, then re-run:
      cp deploy/env/${MODE}.env.example .env
      ./deploy.sh --mode ${MODE}"
  fi
  die ".env not found. Create it from a template, fill in every CHANGE_ME, then re-run:
      cp deploy/env/demo.env.example .env         && ./deploy.sh --mode demo
      cp deploy/env/production.env.example .env   && ./deploy.sh --mode production"
fi

ENV_MODE="$(cfg NEVATAL_MODE)"
ENV_COMPOSE_FILE="$(cfg COMPOSE_FILE)"
if [ -z "$ENV_MODE" ]; then
  case "$ENV_COMPOSE_FILE" in
    *deploy/compose/production.yml*) ENV_MODE="production" ;;
    *deploy/compose/demo.yml*) ENV_MODE="demo" ;;
    *) ENV_MODE="development" ;;
  esac
fi
case "$ENV_MODE" in
  demo|production|development) ;;
  *) die ".env declares NEVATAL_MODE=$ENV_MODE (expected demo or production)" ;;
esac
if [ -n "$MODE" ] && [ "$MODE" != "$ENV_MODE" ]; then
  die "--mode $MODE does not match .env (NEVATAL_MODE=$ENV_MODE). Use the matching template:
      cp deploy/env/${MODE}.env.example .env
  (a fresh .env also selects a separate COMPOSE_PROJECT_NAME, i.e. a fresh database)."
fi
MODE="$ENV_MODE"

case "$MODE" in
  demo) EXPECTED_COMPOSE_FILE="docker-compose.yml:deploy/compose/demo.yml" ;;
  production) EXPECTED_COMPOSE_FILE="docker-compose.yml:deploy/compose/production.yml" ;;
  *) EXPECTED_COMPOSE_FILE="docker-compose.yml" ;;
esac
if [ -z "$ENV_COMPOSE_FILE" ]; then
  if [ "$MODE" != "development" ]; then
    die "COMPOSE_FILE is missing from .env. Add this line so every docker compose command uses the $MODE override:
      COMPOSE_FILE=$EXPECTED_COMPOSE_FILE"
  fi
  COMPOSE_FILE="$EXPECTED_COMPOSE_FILE"
else
  case "$ENV_COMPOSE_FILE" in
    "$EXPECTED_COMPOSE_FILE"|"$EXPECTED_COMPOSE_FILE":*) COMPOSE_FILE="$ENV_COMPOSE_FILE" ;;
    *) die "COMPOSE_FILE=$ENV_COMPOSE_FILE does not match mode '$MODE' (expected it to start with $EXPECTED_COMPOSE_FILE)." ;;
  esac
fi
export COMPOSE_FILE

PROJECT="$(cfg COMPOSE_PROJECT_NAME nevatal-cloud)"
PROFILES="$(cfg COMPOSE_PROFILES)"
HTTP_PORT="$(cfg HTTP_PORT 3080)"
HTTP_BIND="$(cfg HTTP_BIND 0.0.0.0)"
EXTERNAL_DATABASE_URL="$(cfg EXTERNAL_DATABASE_URL)"
case "$MODE" in
  production) REPLICAS="$(cfg BACKEND_REPLICAS 2)" ;;
  *) REPLICAS="$(cfg BACKEND_REPLICAS 1)" ;;
esac

has_profile() {
  case ",${PROFILES// /}," in
    *",$1,"*) return 0 ;;
    *) return 1 ;;
  esac
}

compose_file_has() { # compose_file_has deploy/compose/<file>.yml
  case ":$COMPOSE_FILE:" in
    *":$1:"*) return 0 ;;
    *) return 1 ;;
  esac
}

lowercase() { printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]'; }

# Optional services that only run with their profile (see COMPOSE_PROFILES).
OPTIONAL_SERVICES="caddy edge cloudflared redis"
# Container port of the edge and cloudflared's in-container metrics address
# (fixed in docker-compose.yml).
EDGE_CONTAINER_PORT=8088
CLOUDFLARED_METRICS_ADDR="127.0.0.1:20241"
EDGE_PUBLISH_FILE="deploy/compose/edge-publish.yml"

# --------------------------------------------------------------- validate ---
ERRORS=()
error() { ERRORS+=("$*"); }

# Comma-separated list -> one trimmed, non-empty item per line.
split_list() {
  printf '%s\n' "${1:-}" | tr ',' '\n' | sed 's/^[[:space:]]*//; s/[[:space:]]*$//' | grep -v '^$' || true
}

list_contains() { # list_contains "a,b,c" b
  case ",$(printf '%s' "$1" | tr -d '[:space:]')," in
    *",$2,"*) return 0 ;;
    *) return 1 ;;
  esac
}

is_ipv4() {
  local IFS=. octet
  # No leading zeros (Go's net.ParseIP rejects them as well).
  [[ "${1:-}" =~ ^(0|[1-9][0-9]{0,2})(\.(0|[1-9][0-9]{0,2})){3}$ ]] || return 1
  for octet in $1; do
    [ "$((10#$octet))" -le 255 ] || return 1
  done
}

is_ip() { # IPv4, or something that looks like an IPv6 address
  is_ipv4 "$1" || [[ "$1" == *:* && "$1" =~ ^[0-9A-Fa-f:.]+$ ]]
}

is_cidr() {
  local addr="${1%/*}" prefix="${1##*/}"
  [ "$addr" != "$1" ] && [[ "$prefix" =~ ^[0-9]{1,3}$ ]] || return 1
  if is_ipv4 "$addr"; then
    [ "$((10#$prefix))" -le 32 ]
  else
    is_ip "$addr" && [ "$((10#$prefix))" -le 128 ]
  fi
}

ipv4_to_int() {
  local IFS=.
  # shellcheck disable=SC2086 # split the dotted quad on purpose
  set -- $1
  printf '%s' "$(( (10#$1 << 24) + (10#$2 << 16) + (10#$3 << 8) + 10#$4 ))"
}

ipv4_in_cidrs() { # ipv4_in_cidrs IP "cidr,cidr" (IPv4 entries only)
  local ip="$1" cidr net prefix mask
  is_ipv4 "$ip" || return 1
  while IFS= read -r cidr; do
    net="${cidr%/*}"
    prefix="${cidr##*/}"
    if ! is_cidr "$cidr" || ! is_ipv4 "$net"; then continue; fi
    prefix=$((10#$prefix))
    mask=$(( prefix == 0 ? 0 : (0xFFFFFFFF << (32 - prefix)) & 0xFFFFFFFF ))
    if [ $(( $(ipv4_to_int "$ip") & mask )) -eq $(( $(ipv4_to_int "$net") & mask )) ]; then
      return 0
    fi
  done < <(split_list "$2")
  return 1
}

# Profiles "tunnel" and "edge-publish": checked in every mode.
validate_connectors() {
  local value
  if has_profile tunnel; then
    value="$(cfg CLOUDFLARE_TUNNEL_TOKEN)"
    if [ -z "$value" ]; then
      error "COMPOSE_PROFILES contains 'tunnel' but CLOUDFLARE_TUNNEL_TOKEN is empty (Cloudflare Zero Trust -> Networks -> Tunnels -> your tunnel: the value after --token)."
    elif [[ "$value" != eyJ* ]] || [[ "$value" =~ [[:space:]] ]]; then
      warn "CLOUDFLARE_TUNNEL_TOKEN does not look like a tunnel token (one base64 string starting with 'eyJ'). Paste only the value after --token."
    fi
    value="$(cfg CLOUDFLARED_METRICS_PORT 20241)"
    if ! [[ "$value" =~ ^[0-9]+$ ]]; then
      error "CLOUDFLARED_METRICS_PORT must be a port number (got '$value')."
    fi
    if [ "$MODE" != "development" ] && [ "$HTTP_BIND" != "127.0.0.1" ]; then
      warn "Tunnel profile enabled but HTTP_BIND=$HTTP_BIND: plain HTTP on port $HTTP_PORT bypasses the tunnel. Set HTTP_BIND=127.0.0.1 unless another proxy needs it."
    fi
    if has_profile tls; then
      warn "Both 'tls' (Caddy, inbound 80/443) and 'tunnel' are enabled. A tunnel needs no inbound port: drop 'tls' unless Caddy also serves other clients."
    fi
  fi

  if compose_file_has "$EDGE_PUBLISH_FILE"; then
    value="$(cfg EDGE_PORT_HOST)"
    if ! [[ "$value" =~ ^[0-9]+$ ]]; then
      error "COMPOSE_FILE includes $EDGE_PUBLISH_FILE: set EDGE_PORT_HOST to the host port for the edge, e.g. 8088 (got '$value')."
    fi
    case "$(cfg EDGE_BIND 127.0.0.1)" in
      0.0.0.0|"::"|"[::]")
        warn "EDGE_BIND=$(cfg EDGE_BIND) publishes the edge (plain HTTP) on every interface. Bind it to the address the connector uses and allow only the connector in the firewall." ;;
    esac
    if ! has_profile edge; then
      warn "$EDGE_PUBLISH_FILE is in COMPOSE_FILE but the 'edge' profile is not enabled: nothing is published."
    fi
  fi
}

# Railway-style subdomains (production only; demo forces them off).
validate_subdomains() {
  local raw root cidrs deny item prefix key value app_host
  raw="$(cfg PUBLIC_DOMAIN_ROOT)"
  # Normalized like the backend does: lowercase, no leading/trailing dots.
  root="$(lowercase "$raw" | sed 's/^[.[:space:]]*//; s/[.[:space:]]*$//')"
  if [ -z "$root" ]; then
    if has_profile edge; then
      error "The 'edge' profile is enabled but PUBLIC_DOMAIN_ROOT is empty: nevatal-edge refuses to start without it. Set PUBLIC_DOMAIN_ROOT or remove 'edge' from COMPOSE_PROFILES."
    fi
    return 0
  fi

  if ! [[ "$root" =~ ^([a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$ ]] || [[ "${root##*.}" =~ ^[0-9]+$ ]]; then
    error "PUBLIC_DOMAIN_ROOT must be a bare domain such as nevatal.app (no scheme, no '*.', no port; got '$raw')."
  fi

  cidrs="$(cfg EDGE_TARGET_CIDRS)"
  if [ -z "$cidrs" ]; then
    error "EDGE_TARGET_CIDRS is required when PUBLIC_DOMAIN_ROOT is set: the customer VM subnet(s), e.g. 10.20.0.0/24. VM addresses come from the guest agent, which customers control."
  fi
  while IFS= read -r item; do
    if is_ip "$item"; then
      continue # a single address is allowed (backend: /32 or /128)
    fi
    if ! is_cidr "$item"; then
      error "EDGE_TARGET_CIDRS: '$item' is not a CIDR such as 10.20.0.0/24 (or a single IP)."
      continue
    fi
    prefix=$((10#${item##*/}))
    if [ "$prefix" -eq 0 ]; then
      error "EDGE_TARGET_CIDRS must not contain $item: list only the customer VM subnet(s)."
    elif is_ipv4 "${item%/*}" && [ "$prefix" -lt 16 ]; then
      warn "EDGE_TARGET_CIDRS contains $item: a /$prefix is very broad. List only the customer VM subnet(s), never management or infrastructure networks."
    fi
  done < <(split_list "$cidrs")

  deny="$(cfg EDGE_DENY_IPS)"
  if [ -z "$deny" ]; then
    warn "EDGE_DENY_IPS is empty: list the Proxmox host, the gateway(s) and the control-plane/infrastructure VMs that the edge must never reach."
  fi
  while IFS= read -r item; do
    is_ip "$item" || error "EDGE_DENY_IPS: '$item' is not an IP address."
  done < <(split_list "$deny")

  # The backend adds PROXMOX_HOST (when it is an IP) to the deny list itself.
  # The control plane's own VM is not known to it: warn when HTTP_BIND is a
  # customer-network address that is not denied.
  if is_ipv4 "$HTTP_BIND" && ipv4_in_cidrs "$HTTP_BIND" "$cidrs" && ! list_contains "$deny" "$HTTP_BIND"; then
    warn "HTTP_BIND $HTTP_BIND (this VM) is inside EDGE_TARGET_CIDRS: add it to EDGE_DENY_IPS so tenant names can never point at the control plane."
  fi

  while IFS= read -r item; do
    if ! [[ "$item" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]]; then
      warn "RESERVED_SUBDOMAINS: '$item' is not a single lowercase DNS label (write 'cloud', not 'cloud.<domain>')."
    fi
  done < <(split_list "$(cfg RESERVED_SUBDOMAINS)")

  for key in MAX_DOMAINS_PER_USER DOMAIN_QUARANTINE_DAYS; do
    value="$(cfg "$key")"
    if [ -n "$value" ] && ! [[ "$value" =~ ^[0-9]+$ ]]; then
      error "$key must be a whole number (got '$value')."
    fi
  done

  # The control plane under the tenant domain = same-site with tenant apps.
  app_host="$(cfg APP_BASE_URL)"
  app_host="${app_host#*://}"
  app_host="$(lowercase "${app_host%%[/:?#]*}")"
  if [ "$app_host" = "$root" ] || [[ "$app_host" == *".$root" ]]; then
    warn "The control plane ($app_host) is inside PUBLIC_DOMAIN_ROOT ($root), so tenant apps are same-site with it. Recommended: a separate registrable domain for tenant apps, e.g. PUBLIC_DOMAIN_ROOT=nevatal.app (docs/PROXMOX_INSTALL.md, section 20)."
  fi

  if ! has_profile edge; then
    warn "PUBLIC_DOMAIN_ROOT is set but the 'edge' profile is not enabled: names can be claimed, but this stack does not serve them. Add 'edge' to COMPOSE_PROFILES unless the edge runs on another VM."
  elif ! has_profile tunnel && ! compose_file_has "$EDGE_PUBLISH_FILE"; then
    warn "The edge is reachable only inside this compose project: enable the 'tunnel' profile, or append $EDGE_PUBLISH_FILE to COMPOSE_FILE for a connector outside this stack."
  fi
}

validate_env() {
  local key value lines

  # Placeholders left from the template (comment lines are ignored).
  lines="$(awk '/^[[:space:]]*#/ { next } /CHANGE_ME/ { split($0, kv, "="); gsub(/^[[:space:]]*(export[[:space:]]+)?|[[:space:]]+$/, "", kv[1]); print kv[1] }' "$ENV_FILE" | sort -u | tr '\n' ' ')"
  if [ -n "$lines" ]; then
    error "Replace the CHANGE_ME placeholders in .env: ${lines% }"
  fi

  # Compose interpolates "$" inside .env values; single-quoted values are literal.
  lines="$(awk -v sq="'" '
    /^[[:space:]]*(#|$)/ { next }
    {
      eq = index($0, "="); if (eq == 0) next
      name = substr($0, 1, eq - 1); val = substr($0, eq + 1)
      gsub(/^[[:space:]]*(export[[:space:]]+)?|[[:space:]]+$/, "", name)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
      if (index(val, "$") > 0 && !(substr(val, 1, 1) == sq && substr(val, length(val), 1) == sq)) print name
    }' "$ENV_FILE" | sort -u | tr '\n' ' ')"
  if [ -n "$lines" ]; then
    warn "These .env values contain '\$', which Docker Compose treats as a variable reference: ${lines% }"
    warn "Use hex secrets (openssl rand -hex 32) or wrap the value in single quotes."
  fi

  if ! [[ "$REPLICAS" =~ ^[1-9][0-9]*$ ]]; then
    error "BACKEND_REPLICAS must be a positive integer (got '$REPLICAS')."
  fi
  if ! [[ "$HTTP_PORT" =~ ^[0-9]+$ ]]; then
    error "HTTP_PORT must be a port number (got '$HTTP_PORT')."
  fi

  validate_connectors

  [ "$MODE" = "development" ] && return 0

  # Demo and production both run with APP_ENV=production (set by the override),
  # so the backend's production rules apply to both.
  value="$(cfg JWT_SECRET)"
  if [ -z "$value" ]; then
    error "JWT_SECRET is required (generate one with: openssl rand -hex 32)."
  elif [ "$value" = "$DEV_JWT_SECRET" ]; then
    error "JWT_SECRET is the public development default; generate a new one with: openssl rand -hex 32"
  elif [ "${#value}" -lt "$MIN_JWT_SECRET_LENGTH" ]; then
    error "JWT_SECRET must be at least $MIN_JWT_SECRET_LENGTH characters (openssl rand -hex 32)."
  fi

  value="$(cfg APP_BASE_URL)"
  if ! [[ "$value" =~ ^https?://[^/[:space:]]+ ]]; then
    error "APP_BASE_URL must be an absolute URL such as https://cloud.example.com (got '$value')."
  elif [ "$MODE" = "production" ] && [[ "$value" == http://* ]]; then
    warn "APP_BASE_URL uses http://: session cookies will not be marked Secure. Terminate TLS in front of Nevatal Cloud."
  fi

  value="$(cfg CORS_ALLOWED_ORIGINS)"
  case ",${value// /}," in
    *",*,"*) error "CORS_ALLOWED_ORIGINS must list explicit origins (\"*\" is not allowed in production)." ;;
  esac

  if [ -z "$EXTERNAL_DATABASE_URL" ]; then
    value="$(cfg POSTGRES_PASSWORD)"
    if [ -z "$value" ]; then
      error "POSTGRES_PASSWORD is required (openssl rand -hex 32)."
    elif ! [[ "$value" =~ ^[A-Za-z0-9._~-]+$ ]]; then
      error "POSTGRES_PASSWORD may only contain A-Z a-z 0-9 . _ ~ - (it is embedded in DATABASE_URL). Use: openssl rand -hex 32"
    fi
  fi

  if has_profile tls; then
    if [ -z "$(cfg DOMAIN)" ]; then
      error "COMPOSE_PROFILES contains 'tls' but DOMAIN is empty (Caddy needs the public hostname)."
    fi
    if [ "$HTTP_BIND" != "127.0.0.1" ]; then
      warn "TLS profile enabled but HTTP_BIND=$HTTP_BIND: plain HTTP on port $HTTP_PORT bypasses Caddy. Set HTTP_BIND=127.0.0.1."
    fi
  fi

  if [ "$MODE" = "demo" ]; then
    if [ "$REPLICAS" != "1" ]; then
      error "Demo mode runs exactly one backend replica (the simulated hypervisor lives in process memory). Set BACKEND_REPLICAS=1."
    fi
    if [ -n "$(cfg DEMO_MODE)" ] && ! is_true "$(cfg DEMO_MODE)"; then
      warn "DEMO_MODE=$(cfg DEMO_MODE) in .env is ignored: deploy/compose/demo.yml always sets DEMO_MODE=true."
    fi
    if [ -n "$(cfg PROXMOX_TOKEN_SECRET)" ]; then
      warn "PROXMOX_TOKEN_SECRET is set but demo mode never contacts Proxmox (PROXMOX_MOCK is forced to true). Remove it from a public demo host."
    fi
    if has_profile edge; then
      error "The 'edge' profile is production-only: remove it from COMPOSE_PROFILES (demo mode turns Railway-style subdomains off)."
    fi
    if [ -n "$(cfg PUBLIC_DOMAIN_ROOT)" ]; then
      warn "PUBLIC_DOMAIN_ROOT is ignored in demo mode (deploy/compose/demo.yml forces it empty). Leave it empty."
    fi
    return 0
  fi

  # ---- production (DEMO_MODE=false) ----
  value="$(cfg ADMIN_EMAIL)"
  if [ -z "$value" ] || [[ "$value" != *@* ]]; then
    error "ADMIN_EMAIL is required in production (bootstrap administrator)."
  fi
  value="$(cfg ADMIN_PASSWORD)"
  if [ "${#value}" -lt "$MIN_ADMIN_PASSWORD_LENGTH" ]; then
    error "ADMIN_PASSWORD is required in production and must be at least $MIN_ADMIN_PASSWORD_LENGTH characters."
  elif [ "$value" = "$DEMO_ADMIN_PASSWORD" ] || [ "$value" = "$DEMO_USER_PASSWORD" ]; then
    error "ADMIN_PASSWORD must not be one of the public demo passwords."
  fi

  if [ -z "$EXTERNAL_DATABASE_URL" ]; then
    value="$(cfg POSTGRES_PASSWORD)"
    case "$value" in
      postgres|postgrespassword|password|changeme|nevatal|nevatal_cloud)
        error "POSTGRES_PASSWORD is a well-known default; generate one with: openssl rand -hex 32" ;;
    esac
    if [ -n "$value" ] && [ "${#value}" -lt 16 ]; then
      warn "POSTGRES_PASSWORD is shorter than 16 characters."
    fi
  fi

  value="$(cfg PROXMOX_MOCK false)"
  if is_true "$value"; then
    warn "PROXMOX_MOCK=true in production: the SIMULATED hypervisor is used and no real VM will be created."
  else
    for key in PROXMOX_HOST PROXMOX_TOKEN_ID PROXMOX_TOKEN_SECRET; do
      if [ -z "$(cfg "$key")" ]; then
        error "PROXMOX_MOCK=false requires PROXMOX_HOST, PROXMOX_TOKEN_ID and PROXMOX_TOKEN_SECRET ($key is empty)."
        break
      fi
    done
    value="$(cfg PROXMOX_TOKEN_ID)"
    if [ -n "$value" ] && ! [[ "$value" =~ ^[^@!]+@[^@!]+![^@!]+$ ]]; then
      warn "PROXMOX_TOKEN_ID '$value' does not look like USER@REALM!TOKENID."
    fi
    if ! is_true "$(cfg PROXMOX_VERIFY_TLS false)"; then
      warn "PROXMOX_VERIFY_TLS=false: the Proxmox API certificate is not verified. Keep the API on a trusted network."
    fi
  fi

  if [ "$(cfg WORDPRESS_API_KEY)" = "$LEGACY_WORDPRESS_KEY" ]; then
    warn "WORDPRESS_API_KEY is the old public default: the backend disables the WordPress integration until you set a new key."
  fi

  validate_subdomains

  if [ -n "$(find "$ENV_FILE" -perm -0004 2>/dev/null)" ]; then
    warn ".env is world-readable and holds secrets: chmod 600 .env"
  fi
}

report_validation() {
  local e
  if [ "${#ERRORS[@]}" -gt 0 ]; then
    printf '%s[error]%s .env is not valid for %s mode:\n' "$C_RED" "$C_RESET" "$MODE" >&2
    for e in "${ERRORS[@]}"; do
      printf '  - %s\n' "$e" >&2
    done
    exit 1
  fi
  success ".env passes the $MODE checks"
}

# ----------------------------------------------------------------- docker ---
compose() { docker compose "$@"; }

check_docker() {
  local version
  command -v docker >/dev/null 2>&1 || die "Docker is not installed. See docs/PROXMOX_INSTALL.md (Install Docker Engine)."
  docker info >/dev/null 2>&1 || die "Cannot reach the Docker daemon. Is it running, and is $(id -un) in the 'docker' group (or run with sudo)?"
  docker compose version >/dev/null 2>&1 || die "The Docker Compose v2 plugin is missing (apt install docker-compose-plugin)."
  version="$(docker compose version --short 2>/dev/null || true)"
  version="${version#v}"
  [ -n "$version" ] || die "Could not determine the Docker Compose version."
  if ! version_ge "$version" "$MIN_COMPOSE_VERSION"; then
    die "Docker Compose $version is too old: >= $MIN_COMPOSE_VERSION is required (the overrides use !reset)."
  fi
}

check_other_stacks() {
  local others
  others="$(docker ps --filter "label=com.docker.compose.project.working_dir=$ROOT_DIR" \
    --format '{{.Label "com.docker.compose.project"}}' | sort -u | grep -vx "$PROJECT" || true)"
  if [ -n "$others" ]; then
    die "Another Nevatal stack from this directory is running (project: $(echo "$others" | tr '\n' ' ')).
  It would collide on ports. Stop it first, e.g.: docker compose -p ${others%%$'\n'*} down"
  fi
}

service_enabled() {
  # Capture first: grep -q exiting early would SIGPIPE compose under pipefail.
  local services
  services="$(compose config --services 2>/dev/null || true)"
  grep -qx "$1" <<<"$services"
}

# `up --remove-orphans` keeps the containers of services whose profile was
# removed from COMPOSE_PROFILES (they are still defined in the compose files),
# so a disabled tunnel or edge would keep serving traffic. Remove them.
remove_disabled_services() {
  local services svc ids
  services="$(compose config --services 2>/dev/null || true)"
  for svc in $OPTIONAL_SERVICES; do
    grep -qx "$svc" <<<"$services" && continue
    ids="$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT" --filter "label=com.docker.compose.service=$svc")"
    [ -n "$ids" ] || continue
    log "Profile of '$svc' is not enabled: stopping and removing its container(s)..."
    # shellcheck disable=SC2086 # one id per word
    compose rm -sf "$svc" >/dev/null 2>&1 || docker rm -f $ids >/dev/null
  done
}

# Recent logs of the services that matter for a failed start.
dump_logs() {
  local svc services="backend frontend"
  for svc in edge cloudflared; do
    if service_enabled "$svc"; then services="$services $svc"; fi
  done
  # shellcheck disable=SC2086 # one service per word
  compose logs --tail 40 $services >&2 || true
}

edge_healthy() {
  compose exec -T edge wget -qO /dev/null "http://127.0.0.1:$EDGE_CONTAINER_PORT/__edge/health" >/dev/null 2>&1
}

tunnel_ready() {
  compose exec -T cloudflared cloudflared tunnel --metrics "$CLOUDFLARED_METRICS_ADDR" ready >/dev/null 2>&1
}

# Edge + tunnel state, for --status and after a deploy.
show_edge_and_tunnel() {
  local root
  root="$(cfg PUBLIC_DOMAIN_ROOT)"
  [ "$MODE" = "demo" ] && root=""
  if [ -n "$root" ]; then
    log "Subdomains: https://<name>.$root   targets: $(cfg EDGE_TARGET_CIDRS '(none!)')   deny: $(cfg EDGE_DENY_IPS '(none)')"
  else
    log "Subdomains: off (PUBLIC_DOMAIN_ROOT is empty$([ "$MODE" = "demo" ] && printf ' / demo mode'))"
  fi
  if service_enabled edge; then
    if edge_healthy; then
      success "edge: /__edge/health OK"
    else
      warn "edge: /__edge/health is not answering. Inspect: docker compose logs edge"
    fi
    if compose_file_has "$EDGE_PUBLISH_FILE"; then
      printf '      published on %s:%s\n' "$(cfg EDGE_BIND 127.0.0.1)" "$(cfg EDGE_PORT_HOST '?')"
    fi
  elif [ -n "$root" ]; then
    warn "edge: not enabled on this host (COMPOSE_PROFILES has no 'edge')."
  fi
  if service_enabled cloudflared; then
    if tunnel_ready; then
      success "cloudflared: connected to Cloudflare (/ready); metrics on 127.0.0.1:$(cfg CLOUDFLARED_METRICS_PORT 20241)"
    else
      warn "cloudflared: tunnel not ready. Check CLOUDFLARE_TUNNEL_TOKEN and outbound access to Cloudflare (port 7844 TCP/UDP): docker compose logs cloudflared"
    fi
  fi
}

wait_for_postgres() {
  local i
  log "Waiting for PostgreSQL to accept connections..."
  for i in $(seq 1 60); do
    # shellcheck disable=SC2016 # expanded by the shell inside the container
    if compose exec -T postgres sh -c 'pg_isready -q -U "$POSTGRES_USER" -d "$POSTGRES_DB"' >/dev/null 2>&1; then
      success "PostgreSQL is ready"
      return 0
    fi
    [ "$i" -eq 1 ] || printf '.'
    sleep 2
  done
  printf '\n'
  compose logs --tail 30 postgres >&2 || true
  die "PostgreSQL did not become ready within 120s."
}

start_database() {
  if service_enabled postgres; then
    log "Starting PostgreSQL..."
    compose up -d postgres
    wait_for_postgres
  else
    log "Bundled PostgreSQL disabled (external database)."
  fi
}

build_images() {
  # Stamp the git version when available; otherwise keep the version compiled
  # into the source (internal/config.Version).
  APP_VERSION="${APP_VERSION:-$(git -C "$ROOT_DIR" describe --tags --always --dirty 2>/dev/null || true)}"
  export APP_VERSION
  log "Building images (version ${APP_VERSION:-from source})..."
  compose build backend frontend
}

run_migrations() {
  log "Applying database migrations..."
  compose run --rm migrate
  success "Database schema is up to date"
}

# HTTP GET from this host: curl, wget, or from inside the frontend container.
http_get() {
  local path="$1" host="$HTTP_BIND"
  case "$host" in ""|0.0.0.0|"::"|"[::]") host="127.0.0.1" ;; esac
  if command -v curl >/dev/null 2>&1; then
    curl -fsS --max-time 5 "http://$host:$HTTP_PORT$path"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- -T 5 "http://$host:$HTTP_PORT$path"
  else
    compose exec -T frontend wget -qO- -T 5 "http://127.0.0.1:8080$path"
  fi
}

wait_for_health() {
  local waited=0
  log "Waiting for /health through the frontend (port $HTTP_PORT)..."
  until http_get /health >/dev/null 2>&1; do
    if [ "$waited" -ge "$HEALTH_TIMEOUT" ]; then
      compose ps >&2 || true
      dump_logs
      die "No healthy response from /health after ${HEALTH_TIMEOUT}s. Inspect: docker compose logs backend frontend"
    fi
    sleep 3
    waited=$((waited + 3))
  done
  success "API healthy through nginx"
}

json_field() { # json_field KEY <<< JSON  (flat scalar fields only)
  grep -o "\"$1\"[[:space:]]*:[[:space:]]*\"\{0,1\}[^,\"}]*" | sed -n '1s/.*:[[:space:]]*"\{0,1\}//p' || true
}

check_runtime_mode() {
  local body env demo mock version domains expected
  body="$(http_get /api/v1/system/config 2>/dev/null || true)"
  if [ -z "$body" ]; then
    warn "Could not read /api/v1/system/config to confirm the runtime mode."
    return 0
  fi
  env="$(printf '%s' "$body" | json_field environment)"
  demo="$(printf '%s' "$body" | json_field demo_mode)"
  mock="$(printf '%s' "$body" | json_field proxmox_mock)"
  version="$(printf '%s' "$body" | json_field version)"
  # domains.root: empty while Railway-style subdomains are off.
  domains="$(printf '%s' "$body" | json_field root)"
  log "Runtime: environment=${env:-?} demo_mode=${demo:-?} proxmox_mock=${mock:-?} subdomains=${domains:-off} version=${version:-?}"
  case "$MODE" in
    demo)
      [ "$demo" = "true" ] || warn "Expected demo_mode=true in demo mode."
      [ "$mock" = "true" ] || warn "Expected proxmox_mock=true in demo mode."
      [ -z "$domains" ] || warn "Expected subdomains to be off in demo mode (got $domains)."
      ;;
    production)
      [ "$demo" = "false" ] || warn "Expected demo_mode=false in production mode!"
      if [ "$mock" = "true" ]; then
        warn "The API is using the SIMULATED hypervisor (proxmox_mock=true). Set PROXMOX_MOCK=false and the Proxmox token in .env."
      fi
      expected="$(lowercase "$(cfg PUBLIC_DOMAIN_ROOT)" | sed 's/^[.[:space:]]*//; s/[.[:space:]]*$//')"
      if [ "$domains" != "$expected" ]; then
        warn "Subdomains at runtime: '${domains:-off}', .env PUBLIC_DOMAIN_ROOT: '${expected:-empty}'. Check the API log (docker compose logs backend)."
      fi
      ;;
  esac
}

print_summary() {
  local url bind_note
  url="$(cfg APP_BASE_URL "http://localhost:$HTTP_PORT")"
  bind_note="$HTTP_BIND:$HTTP_PORT"
  printf '\n%s================================================================%s\n' "$C_CYAN" "$C_RESET"
  printf '%sNevatal Cloud is up (%s mode)%s\n' "$C_GREEN$C_BOLD" "$MODE" "$C_RESET"
  printf '%s================================================================%s\n' "$C_CYAN" "$C_RESET"
  printf '  Public URL:        %s\n' "$url"
  printf '  nginx (HTTP):      %s\n' "$bind_note"
  if has_profile tls; then
    printf '  TLS (Caddy):       https://%s\n' "$(cfg DOMAIN)"
  fi
  if service_enabled cloudflared; then
    printf '  Cloudflare Tunnel: connected (metrics: http://127.0.0.1:%s/metrics)\n' "$(cfg CLOUDFLARED_METRICS_PORT 20241)"
  fi
  if service_enabled edge; then
    if [ "$MODE" = "production" ] && [ -n "$(cfg PUBLIC_DOMAIN_ROOT)" ]; then
      printf '  Subdomains (edge): https://<name>.%s -> %s\n' "$(cfg PUBLIC_DOMAIN_ROOT)" "$(cfg EDGE_TARGET_CIDRS)"
    else
      printf '  Subdomains (edge): running, feature off (PUBLIC_DOMAIN_ROOT empty)\n'
    fi
  fi
  printf '  Compose project:   %s   (COMPOSE_FILE=%s)\n' "$PROJECT" "$COMPOSE_FILE"
  printf '  Profiles:          %s\n' "${PROFILES:-(none)}"
  printf '  Backend replicas:  %s\n' "$REPLICAS"
  printf '  Logs:              docker compose logs -f backend\n'
  printf '  Status:            ./deploy.sh --status\n'
  if [ "$MODE" = "demo" ]; then
    printf '\n  %sDemo accounts are PUBLIC:%s admin@nevatal.id / AdminPass123!, demo@nevatal.id / DemoPass123!\n' "$C_YELLOW" "$C_RESET"
    printf '  Nightly reset (cron): cd %s && ./deploy.sh --reset-demo --yes --no-build\n' "$ROOT_DIR"
  fi
  printf '%s================================================================%s\n' "$C_CYAN" "$C_RESET"
}

deploy_stack() {
  check_other_stacks
  if [ "$BUILD" -eq 1 ]; then
    build_images
  fi
  start_database
  if [ "$RUN_MIGRATE" -eq 1 ]; then
    run_migrations
  fi
  remove_disabled_services
  log "Starting the stack (backend replicas: $REPLICAS, profiles: ${PROFILES:-none})..."
  if ! compose up -d --remove-orphans --scale "backend=$REPLICAS" --wait --wait-timeout 300; then
    compose ps >&2 || true
    dump_logs
    die "Some services did not become healthy. Inspect: docker compose logs backend (and edge / cloudflared when enabled)"
  fi
  wait_for_health
  check_runtime_mode
  print_summary
}

show_status() {
  log "Mode: $MODE   project: $PROJECT   profiles: ${PROFILES:-none}   COMPOSE_FILE=$COMPOSE_FILE"
  compose ps
  printf '\n'
  log "Database schema version:"
  compose run --rm --no-deps migrate /app/nevatal-migrate version || warn "Could not read the schema version (is the database running?)."
  printf '\n'
  if http_get /health >/dev/null 2>&1; then
    success "/health OK through nginx on port $HTTP_PORT"
    check_runtime_mode
  else
    warn "/health is not answering on port $HTTP_PORT."
  fi
  printf '\n'
  show_edge_and_tunnel
}

rollback() {
  local stopped
  warn "Rolling back the most recent migration can DELETE data. Take a backup first:"
  warn "  docker compose exec -T postgres pg_dump -U <user> -Fc <db> > backup.dump"
  confirm "Stop the API and roll back ONE database migration?"
  log "Stopping the API so it cannot re-apply the migration..."
  compose stop backend || true
  # The edge reads the routing tables that a migration may change.
  if service_enabled edge; then
    compose stop edge || true
    stopped="API and the edge are"
  else
    stopped="API is"
  fi
  start_database
  compose run --rm migrate /app/nevatal-migrate down 1
  success "Rolled back one migration. The $stopped STOPPED."
  printf '%s\n' \
    "Next steps:" \
    "  * Check out the previous release (git checkout <tag>) and run: ./deploy.sh --skip-migrate" \
    "  * Starting the CURRENT release again re-applies the migration (the API migrates at start-up)."
}

reset_demo() {
  local volume ids
  [ "$MODE" = "demo" ] || die "--reset-demo only works in demo mode (.env declares NEVATAL_MODE=$MODE)."
  confirm "Delete ALL demo data (database + uploaded files) of project '$PROJECT' and redeploy?"
  check_other_stacks
  log "Stopping the demo stack..."
  compose down --remove-orphans
  # Only the data volumes: caddy_data keeps the TLS certificate (Let's Encrypt
  # rate-limits re-issuing it).
  for volume in pgdata storagedata; do
    ids="$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT" --filter "label=com.docker.compose.volume=$volume")"
    if [ -n "$ids" ]; then
      # shellcheck disable=SC2086 # one id per word
      docker volume rm $ids >/dev/null
      success "Removed volume $volume"
    fi
  done
  deploy_stack
}

acquire_lock() {
  local lock
  lock="${TMPDIR:-/tmp}/nevatal-deploy-$(id -u).lock"
  if command -v flock >/dev/null 2>&1; then
    exec 9>"$lock"
    flock -n 9 || die "Another ./deploy.sh is already running (lock: $lock)."
  fi
}

# ------------------------------------------------------------------- main ---
log "Nevatal Cloud deploy: mode=$MODE action=$ACTION project=$PROJECT"
if [ "$ACTION" = "deploy" ] || [ "$ACTION" = "reset-demo" ]; then
  validate_env
  report_validation
fi
check_docker
acquire_lock

case "$ACTION" in
  deploy) deploy_stack ;;
  migrate-only)
    start_database
    run_migrations
    ;;
  rollback) rollback ;;
  status) show_status ;;
  reset-demo) reset_demo ;;
esac
