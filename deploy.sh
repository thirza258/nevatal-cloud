#!/usr/bin/env bash
#
# Nevatal Cloud Deployment & Database Migration Script
#
# Usage:
#   ./deploy.sh                # Apply migrations and deploy full stack
#   ./deploy.sh --migrate-only # Apply database migrations only
#   ./deploy.sh --skip-migrate # Deploy services without running migrations
#   ./deploy.sh --rollback     # Roll back the most recent migration step
#   ./deploy.sh --status       # Display current migration version & service status
#   ./deploy.sh --help         # Show this help text

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

# Flags
RUN_MIGRATE=1
MIGRATE_ONLY=0
ROLLBACK=0
CHECK_STATUS=0
REBUILD=1

# Colors
C_RESET='\033[0m'
C_BOLD='\033[1m'
C_BLUE='\033[1;34m'
C_GREEN='\033[1;32m'
C_YELLOW='\033[1;33m'
C_RED='\033[1;31m'
C_CYAN='\033[1;36m'

log()     { printf "${C_BLUE}==>${C_RESET} ${C_BOLD}%s${C_RESET}\n" "$*"; }
success() { printf "${C_GREEN}✓${C_RESET} %s\n" "$*"; }
warn()    { printf "${C_YELLOW}[warn]${C_RESET} %s\n" "$*" >&2; }
die()     { printf "${C_RED}[error]${C_RESET} %s\n" "$*" >&2; exit 1; }

usage() {
  cat <<EOF
Nevatal Cloud Deployment Script

Usage:
  ./deploy.sh [OPTIONS]

Options:
  --migrate-only     Apply database migrations and exit (do not start web stack)
  --skip-migrate     Deploy/restart services without applying migrations
  --rollback         Roll back 1 database migration step
  --status           Check current migration version and container health
  --no-build         Skip rebuilding Docker container images
  -h, --help         Show this help message
EOF
  exit 0
}

# Parse CLI arguments
for arg in "$@"; do
  case "$arg" in
    --migrate-only)
      MIGRATE_ONLY=1
      RUN_MIGRATE=1
      ;;
    --skip-migrate)
      RUN_MIGRATE=0
      ;;
    --rollback)
      ROLLBACK=1
      RUN_MIGRATE=0
      ;;
    --status)
      CHECK_STATUS=1
      RUN_MIGRATE=0
      ;;
    --no-build)
      REBUILD=0
      ;;
    -h|--help)
      usage
      ;;
    *)
      die "Unknown option: $arg (run ./deploy.sh --help for options)"
      ;;
  esac
done

# Load environment configuration
ENV_FILE="$ROOT_DIR/.env"
if [ -f "$ENV_FILE" ]; then
  log "Loading configuration from $ENV_FILE"
  # Export non-comment lines
  set -a
  # shellcheck disable=SC1090
  source "$ENV_FILE" 2>/dev/null || true
  set +a
else
  warn ".env file not found; using existing environment variables or defaults."
fi

# Configuration defaults
PORT="${PORT:-8090}"
DATABASE_URL_HOST="${DATABASE_URL:-postgres://postgres:postgrespassword@localhost:5435/nevatal_cloud?sslmode=disable}"
DATABASE_URL_DOCKER="postgres://postgres:postgrespassword@postgres:5432/nevatal_cloud?sslmode=disable"

# Determine Docker Compose command
if docker compose version >/dev/null 2>&1; then
  COMPOSE_CMD="docker compose"
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE_CMD="docker-compose"
else
  die "Neither 'docker compose' nor 'docker-compose' was found on this system."
fi

# Function: Wait for PostgreSQL container to become ready
wait_for_postgres() {
  log "Waiting for PostgreSQL database to be healthy..."
  local retries=30
  local count=0
  until docker exec nevatal-cloud-postgres pg_isready -U postgres -d nevatal_cloud >/dev/null 2>&1; do
    sleep 1
    count=$((count + 1))
    if [ "$count" -ge "$retries" ]; then
      die "Timed out waiting for PostgreSQL container 'nevatal-cloud-postgres' to become ready."
    fi
  done
  success "PostgreSQL is ready and accepting connections."
}

# Function: Run migrations using go-migrate
apply_migrations() {
  log "Applying database migrations using go-migrate..."

  if command -v go >/dev/null 2>&1; then
    log "Executing migrations via host Go runtime..."
    (cd "$ROOT_DIR/backend" && go run ./cmd/migrate --database "$DATABASE_URL_HOST" up)
  elif [ -x "$ROOT_DIR/backend/bin/nevatal-migrate" ]; then
    log "Executing migrations via pre-built nevatal-migrate binary..."
    "$ROOT_DIR/backend/bin/nevatal-migrate" --database "$DATABASE_URL_HOST" up
  else
    log "Executing migrations via Docker container runner..."
    $COMPOSE_CMD run --rm -e DATABASE_URL="$DATABASE_URL_DOCKER" migrate /app/nevatal-migrate up
  fi
  success "Database migrations successfully applied."
}

# Function: Rollback migration
rollback_migrations() {
  log "Rolling back 1 migration step..."
  if command -v go >/dev/null 2>&1; then
    (cd "$ROOT_DIR/backend" && go run ./cmd/migrate --database "$DATABASE_URL_HOST" down 1)
  elif [ -x "$ROOT_DIR/backend/bin/nevatal-migrate" ]; then
    "$ROOT_DIR/backend/bin/nevatal-migrate" --database "$DATABASE_URL_HOST" down 1
  else
    $COMPOSE_CMD run --rm -e DATABASE_URL="$DATABASE_URL_DOCKER" migrate /app/nevatal-migrate down 1
  fi
  success "Rollback completed."
}

# Function: Check status
show_status() {
  log "Checking Nevatal Cloud status..."
  if command -v go >/dev/null 2>&1; then
    (cd "$ROOT_DIR/backend" && go run ./cmd/migrate --database "$DATABASE_URL_HOST" status)
  elif [ -x "$ROOT_DIR/backend/bin/nevatal-migrate" ]; then
    "$ROOT_DIR/backend/bin/nevatal-migrate" --database "$DATABASE_URL_HOST" status
  fi
  echo ""
  log "Container Status:"
  $COMPOSE_CMD ps
  exit 0
}

# Handle --status
if [ "$CHECK_STATUS" -eq 1 ]; then
  show_status
fi

# Step 1: Ensure database service is up
log "Ensuring database & cache infrastructure services are running..."
$COMPOSE_CMD up -d postgres redis
wait_for_postgres

# Step 2: Handle rollback if requested
if [ "$ROLLBACK" -eq 1 ]; then
  rollback_migrations
  exit 0
fi

# Step 3: Run migrations if enabled
if [ "$RUN_MIGRATE" -eq 1 ]; then
  apply_migrations
fi

# If migrate-only requested, exit here
if [ "$MIGRATE_ONLY" -eq 1 ]; then
  log "Migration-only run requested. Exiting without deploying frontend/backend."
  exit 0
fi

# Step 4: Build backend binaries locally if Go compiler is present
if command -v go >/dev/null 2>&1; then
  log "Compiling backend Go binaries (nevatal-api & nevatal-migrate)..."
  mkdir -p "$ROOT_DIR/backend/bin"
  (
    cd "$ROOT_DIR/backend"
    CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/nevatal-api ./cmd/api
    CGO_ENABLED=0 go build -ldflags="-w -s" -o bin/nevatal-migrate ./cmd/migrate
  )
  success "Backend binaries compiled successfully."
fi

# Step 5: Start or restart full container stack
log "Deploying container stack with Docker Compose..."
if [ "$REBUILD" -eq 1 ]; then
  $COMPOSE_CMD up -d --build backend frontend
else
  $COMPOSE_CMD up -d backend frontend
fi

# Step 6: Verify deployment health
log "Verifying backend API health..."
HEALTH_URL="http://localhost:${PORT}/health"
RETRIES=20
WAIT_COUNT=0
HEALTHY=0

until [ "$WAIT_COUNT" -ge "$RETRIES" ]; do
  if curl -sf "$HEALTH_URL" >/dev/null 2>&1; then
    HEALTHY=1
    break
  fi
  sleep 1
  WAIT_COUNT=$((WAIT_COUNT + 1))
done

if [ "$HEALTHY" -eq 1 ]; then
  success "Backend health check passed at $HEALTH_URL"
else
  warn "Health check did not respond within ${RETRIES}s. Inspect container logs with: $COMPOSE_CMD logs backend"
fi

# Step 7: Display deployment summary
printf "\n"
printf "${C_CYAN}================================================================${C_RESET}\n"
printf "${C_GREEN}${C_BOLD}🚀 Nevatal Cloud Deployment Successful!${C_RESET}\n"
printf "${C_CYAN}================================================================${C_RESET}\n"
printf "  • ${C_BOLD}Web Control Plane:${C_RESET} http://localhost:3080\n"
printf "  • ${C_BOLD}Backend API:${C_RESET}       http://localhost:%s\n" "$PORT"
printf "  • ${C_BOLD}Health Endpoint:${C_RESET}   http://localhost:%s/health\n" "$PORT"
printf "  • ${C_BOLD}PostgreSQL Port:${C_RESET}   5435 (container: 5432)\n"
printf "  • ${C_BOLD}Redis Port:${C_RESET}        6385 (container: 6379)\n"
printf "${C_CYAN}================================================================${C_RESET}\n"
