#!/usr/bin/env bash
# Backend smoke test: demo mode, production mode, demo->production DB reuse, invalid prod config.
set -u
SP="$(cd "$(dirname "$0")" && pwd)"
BIN="${BIN:-$SP/nevatal-api}"
# Fresh databases every run (scenario C deliberately mutates the demo DB).
docker exec nevatal-smoke-pg psql -U postgres -q -c "DROP DATABASE IF EXISTS nevatal_demo" -c "CREATE DATABASE nevatal_demo" \
  -c "DROP DATABASE IF EXISTS nevatal_prod" -c "CREATE DATABASE nevatal_prod" >/dev/null
API="http://127.0.0.1:18090"
PG="postgres://postgres:smokepw@127.0.0.1:55433"
SECRET="0123456789abcdef0123456789abcdef0123456789abcdef"
PASS=0; FAIL=0
ok()   { PASS=$((PASS+1)); echo "  PASS $*"; }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL $*"; }
check(){ if eval "$2"; then ok "$1"; else bad "$1"; fi; }

start_api() { # env assignments as args
  # exec chain keeps $! equal to the API process itself, so stop_api kills only what we started.
  (cd "$SP" && exec env -i PATH="$PATH" HOME="$HOME" "$@" PORT=18090 STORAGE_PATH="$SP/storage" "$BIN" >"$SP/api.log" 2>&1) &
  API_PID=$!
  for _ in $(seq 1 40); do curl -sf "$API/health" >/dev/null 2>&1 && return 0; sleep 0.5; done
  echo "API did not start:"; tail -20 "$SP/api.log"; return 1
}
stop_api() { if [ -n "${API_PID:-}" ]; then kill "$API_PID" >/dev/null 2>&1; wait "$API_PID" 2>/dev/null; API_PID=""; fi; sleep 1; }
j() { python3 -c "import sys,json; d=json.load(sys.stdin); print(eval(sys.argv[1]))" "$1"; }
login() { curl -s -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"$1\",\"password\":\"$2\"}"; }

echo "== A. Demo deployment (APP_ENV=production, DEMO_MODE=true)"
start_api APP_ENV=production DEMO_MODE=true JWT_SECRET="$SECRET" APP_BASE_URL=https://demo.nevatal.id \
  DATABASE_URL="$PG/nevatal_demo?sslmode=disable" PROXMOX_MOCK=false PROXMOX_HOST=10.0.0.2 PROXMOX_TOKEN_ID='x@pve!t' PROXMOX_TOKEN_SECRET=s || exit 1
check "forced mock warning logged" "grep -q 'DEMO_MODE=true forces PROXMOX_MOCK=true' '$SP/api.log'"
CFG=$(curl -s "$API/api/v1/system/config")
check "system config demo_mode=true" "[ \"\$(echo '$CFG' | j 'd[\"data\"][\"demo_mode\"]')\" = True ]"
check "system config proxmox_mock=true" "[ \"\$(echo '$CFG' | j 'd[\"data\"][\"proxmox_mock\"]')\" = True ]"
TOK=$(login demo@nevatal.id DemoPass123! | j 'd["data"]["token"]')
check "demo customer login" "[ -n '$TOK' ]"
ATOK=$(login admin@nevatal.id AdminPass123! | j 'd["data"]["token"]')
check "demo admin login" "[ -n '$ATOK' ]"
INST=$(curl -s "$API/api/v1/instances" -H "Authorization: Bearer $TOK" | j 'd["data"][0]["id"]')
check "demo instance exists" "[ -n '$INST' ]"
OV=$(curl -s "$API/api/v1/instances/$INST/docker?stats=true" -H "Authorization: Bearer $TOK")
check "docker overview available" "[ \"\$(echo '$OV' | j 'd[\"data\"][\"available\"]')\" = True ]"
NC=$(echo "$OV" | j 'len(d["data"]["containers"])'); CNAME=$(echo "$OV" | j 'd["data"]["containers"][0]["name"]')
check "docker overview lists containers ($NC)" "[ '$NC' -ge 3 ]"
check "docker stats merged" "[ -n \"\$(echo '$OV' | j 'd[\"data\"][\"containers\"][0].get(\"cpu_percent\",\"\")')\" ]"
check "docker version present" "[ \"\$(echo '$OV' | j 'd[\"data\"][\"docker_version\"]')\" = 27.3.1 ]"
LG=$(curl -s "$API/api/v1/instances/$INST/docker/containers/$CNAME/logs?tail=25&since=24h" -H "Authorization: Bearer $TOK")
check "docker logs 1..25 lines" "n=\$(echo '$LG' | j 'len(d[\"data\"][\"lines\"])'); [ \"\$n\" -ge 1 ] && [ \"\$n\" -le 25 ]"
check "logs reject flag-like container (-v)" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/instances/$INST/docker/containers/-v/logs' -H 'Authorization: Bearer $TOK')\" = 400 ]"
check "logs reject tail=99999" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/instances/$INST/docker/containers/$CNAME/logs?tail=99999' -H 'Authorization: Bearer $TOK')\" = 400 ]"
check "logs reject since=1d" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/instances/$INST/docker/containers/$CNAME/logs?since=1d' -H 'Authorization: Bearer $TOK')\" = 400 ]"
check "logs unknown container 404" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/instances/$INST/docker/containers/nope/logs' -H 'Authorization: Bearer $TOK')\" = 404 ]"
check "admin docker overview" "[ \"\$(curl -s '$API/api/v1/admin/instances/$INST/docker' -H 'Authorization: Bearer $ATOK' | j 'd[\"data\"][\"available\"]')\" = True ]"
check "customer blocked from admin docker route" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/admin/instances/$INST/docker' -H 'Authorization: Bearer $TOK')\" = 403 ]"
REG=$(curl -s -X POST "$API/api/v1/auth/register" -H 'Content-Type: application/json' -d '{"email":"other@example.com","password":"Password123","display_name":"Other"}' | j 'd["data"]["token"]')
check "other tenant cannot read demo VM docker (404)" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' '$API/api/v1/instances/$INST/docker' -H 'Authorization: Bearer $REG')\" = 404 ]"
check "demo mock google oauth works" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/google' -H 'Content-Type: application/json' -d '{}')\" = 200 ]"
check "CORS: allowed origin echoed" "curl -s -D - -o /dev/null -H 'Origin: https://demo.nevatal.id' '$API/api/v1/system/config' | grep -qi '^access-control-allow-origin: https://demo.nevatal.id'"
check "CORS: foreign origin not echoed" "! curl -s -D - -o /dev/null -H 'Origin: https://evil.example' '$API/api/v1/system/config' | grep -qi '^access-control-allow-origin'"
check "secure SameSite cookie on login" "curl -s -D - -o /dev/null -X POST '$API/api/v1/auth/login' -H 'Content-Type: application/json' -d '{\"email\":\"demo@nevatal.id\",\"password\":\"DemoPass123!\"}' | grep -i '^set-cookie' | grep -qi 'secure' "
CK=$(curl -s -D - -o /dev/null -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' -d '{"email":"demo@nevatal.id","password":"DemoPass123!"}' | grep -i '^set-cookie' | head -1 | sed -E 's/^[Ss]et-[Cc]ookie: ([^;]*).*/\1/' | tr -d '\r')
CK_GET=$(curl -s -o /dev/null -w '%{http_code}' -H "Cookie: $CK" "$API/api/v1/auth/me")
CK_POST=$(curl -s -o /dev/null -w '%{http_code}' -X POST -H "Cookie: $CK" -H 'Content-Type: text/plain' --data '{"name":"csrf","public_key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl csrf"}' "$API/api/v1/ssh-keys")
check "session cookie uses __Host- prefix" "[ \"\${CK%%=*}\" = __Host-nevatal_token ]"
check "cookie-only GET allowed" "[ $CK_GET = 200 ]"
check "cookie-only POST rejected (sibling-subdomain CSRF)" "[ $CK_POST = 401 ]"
check "health not in access log" "! grep -q 'GET      \"/health\"' '$SP/api.log'"
stop_api

echo "== B. Production deployment (fresh DB, DEMO_MODE=false)"
start_api APP_ENV=production DEMO_MODE=false JWT_SECRET="$SECRET" APP_BASE_URL=https://cloud.nevatal.id \
  DATABASE_URL="$PG/nevatal_prod?sslmode=disable" ADMIN_EMAIL=ops@nevatal.id ADMIN_PASSWORD='S3cure-admin-pass' PROXMOX_MOCK=true || exit 1
check "system config demo_mode=false" "[ \"\$(curl -s '$API/api/v1/system/config' | j 'd[\"data\"][\"demo_mode\"]')\" = False ]"
check "no demo customer seeded" "[ \"\$(login demo@nevatal.id DemoPass123! | j 'd[\"error\"][\"code\"]')\" = INVALID_CREDENTIALS ]"
check "no demo admin seeded" "[ \"\$(login admin@nevatal.id AdminPass123! | j 'd[\"error\"][\"code\"]')\" = INVALID_CREDENTIALS ]"
PTOK=$(login ops@nevatal.id 'S3cure-admin-pass' | j 'd["data"]["token"]')
check "bootstrap admin login" "[ -n '$PTOK' ] && [ \"\$(curl -s '$API/api/v1/auth/me' -H \"Authorization: Bearer $PTOK\" | j 'd[\"data\"][\"role\"]')\" = admin ]"
check "OAuth mock_email takeover blocked (503)" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/google' -H 'Content-Type: application/json' -d '{\"mock_email\":\"ops@nevatal.id\"}')\" = 503 ]"
check "GitHub mock blocked (503)" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/github' -H 'Content-Type: application/json' -d '{\"mock_email\":\"ops@nevatal.id\"}')\" = 503 ]"
check "WordPress SSO without key blocked" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/wordpress' -H 'Content-Type: application/json' -d '{\"email\":\"ops@nevatal.id\"}')\" = 503 ]"
check "WordPress webhook disabled without key" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/webhooks/wordpress' -H 'Content-Type: application/json' -d '{\"action\":\"ping\"}')\" = 503 ]"
check "catalog templates still seeded" "[ \"\$(curl -s '$API/api/v1/templates' | j 'len(d[\"data\"])')\" -ge 3 ]"
stop_api

echo "== B2. Production with WordPress key: SSO cannot sign in admins"
start_api APP_ENV=production DEMO_MODE=false JWT_SECRET="$SECRET" APP_BASE_URL=https://cloud.nevatal.id \
  DATABASE_URL="$PG/nevatal_prod?sslmode=disable" ADMIN_EMAIL=ops@nevatal.id ADMIN_PASSWORD='S3cure-admin-pass' PROXMOX_MOCK=true WORDPRESS_API_KEY=wpkey-1234567890 || exit 1
check "WP SSO wrong key 401" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/wordpress' -H 'X-Nevatal-Key: nope' -H 'Content-Type: application/json' -d '{\"email\":\"x@example.com\"}')\" = 401 ]"
check "WP SSO for admin refused (422)" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/wordpress' -H 'X-Nevatal-Key: wpkey-1234567890' -H 'Content-Type: application/json' -d '{\"email\":\"ops@nevatal.id\"}')\" = 422 ]"
check "WP SSO for customer works" "[ \"\$(curl -s -o /dev/null -w '%{http_code}' -X POST '$API/api/v1/auth/oauth/wordpress' -H 'X-Nevatal-Key: wpkey-1234567890' -H 'Content-Type: application/json' -d '{\"email\":\"wp-customer@example.com\"}')\" = 200 ]"
stop_api

echo "== C. Production reusing the demo database"
start_api APP_ENV=production DEMO_MODE=false JWT_SECRET="$SECRET" APP_BASE_URL=https://cloud.nevatal.id \
  DATABASE_URL="$PG/nevatal_demo?sslmode=disable" ADMIN_EMAIL=ops@nevatal.id ADMIN_PASSWORD='S3cure-admin-pass' PROXMOX_MOCK=true || exit 1
check "demo admin suspended" "login admin@nevatal.id AdminPass123! | grep -q suspended"
check "demo customer suspended" "login demo@nevatal.id DemoPass123! | grep -q suspended"
check "suspension logged" "grep -q 'SECURITY: suspended admin@nevatal.id' '$SP/api.log'"
stop_api

echo "== C2. Production where ADMIN_EMAIL is the old demo admin: password rotated"
start_api APP_ENV=production DEMO_MODE=false JWT_SECRET="$SECRET" APP_BASE_URL=https://cloud.nevatal.id \
  DATABASE_URL="$PG/nevatal_demo?sslmode=disable" ADMIN_EMAIL=admin@nevatal.id ADMIN_PASSWORD='Rotated-admin-pass-1' PROXMOX_MOCK=true || exit 1
check "public password no longer works" "! login admin@nevatal.id AdminPass123! | grep -q token"
check "rotated password works" "login admin@nevatal.id 'Rotated-admin-pass-1' | grep -q token"
stop_api

echo "== D. Unsafe production config refuses to start"
(cd "$SP" && env -i PATH="$PATH" APP_ENV=production DEMO_MODE=false DATABASE_URL="$PG/nevatal_prod?sslmode=disable" PORT=18090 "$BIN" >"$SP/api-bad.log" 2>&1)
check "exit non-zero + explains" "grep -q 'Refusing to start' '$SP/api-bad.log' && grep -q 'JWT_SECRET is required' '$SP/api-bad.log' && grep -q 'ADMIN_EMAIL is required' '$SP/api-bad.log'"

echo "== Graceful shutdown"
start_api APP_ENV=development DATABASE_URL="$PG/nevatal_demo?sslmode=disable" || exit 1
kill -TERM "$API_PID"; wait "$API_PID" 2>/dev/null; API_PID=""; sleep 1
check "shutdown completes cleanly" "grep -q 'Shutdown complete' '$SP/api.log'"

echo "RESULT: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
