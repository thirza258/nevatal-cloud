#!/usr/bin/env bash
# Build the API from BACKEND_DIR, start it on a fresh demo database and record
# endpoint shapes. Usage: capture_shapes.sh <backend_dir> <out.json>
set -euo pipefail
SP="$(cd "$(dirname "$0")" && pwd)"
BACKEND_DIR="$1"; OUT="$2"
BIN="$SP/shape-api"
(cd "$BACKEND_DIR" && go build -o "$BIN" ./cmd/api)
docker exec nevatal-smoke-pg psql -U postgres -q -c "DROP DATABASE IF EXISTS nevatal_shape" -c "CREATE DATABASE nevatal_shape"
(exec env -i PATH="$PATH" HOME="$HOME" APP_ENV=development DEMO_MODE=true \
  DATABASE_URL="postgres://postgres:smokepw@127.0.0.1:55433/nevatal_shape?sslmode=disable" \
  PORT=18091 STORAGE_PATH="$SP/storage-shape" "$BIN" >"$SP/shape-api.log" 2>&1) &
API_PID=$!
for _ in $(seq 1 40); do curl -sf http://127.0.0.1:18091/health >/dev/null 2>&1 && break; sleep 0.5; done
python3 "$SP/shape_snapshot.py" http://127.0.0.1:18091 "$OUT" || { kill "$API_PID" 2>/dev/null; exit 1; }
kill "$API_PID" 2>/dev/null || true
wait "$API_PID" 2>/dev/null || true
