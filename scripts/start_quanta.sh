#!/usr/bin/env bash
set -euo pipefail

# Starts all 4 QUANTA services natively without Docker and tracks PIDs in .logs/.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$REPO_ROOT/.logs"

mkdir -p "$LOG_DIR"

log_info() { printf "[QUANTA] %s\n" "$1"; }
log_ok()   { printf "[OK]     %s\n" "$1"; }
log_warn() { printf "[WARN]   %s\n" "$1"; }
log_err()  { printf "[ERROR]  %s\n" "$1" >&2; }

# 1. Load .env variables into process environment
if [[ -f "$REPO_ROOT/.env" ]]; then
  log_info "Loading environment from $REPO_ROOT/.env"
  set -a
  # shellcheck disable=SC1091
  source "$REPO_ROOT/.env"
  set +a
else
  log_warn "Root .env not found; relying on existing environment variables."
fi

PG_HOST="${POSTGRES_HOST:-localhost}"
PG_PORT="${POSTGRES_PORT:-5432}"

# 2. Check PostgreSQL reachability on localhost:5432
check_port() {
  local host="$1"
  local port="$2"
  if command -v pg_isready >/dev/null 2>&1 && [[ "$port" == "$PG_PORT" ]]; then
    pg_isready -h "$host" -p "$port" >/dev/null 2>&1 && return 0
  fi
  if command -v nc >/dev/null 2>&1; then
    nc -z "$host" "$port" >/dev/null 2>&1 && return 0
  fi
  if (echo >"/dev/tcp/$host/$port") >/dev/null 2>&1; then
    return 0
  fi
  return 1
}

log_info "Checking PostgreSQL reachability on ${PG_HOST}:${PG_PORT}..."
if ! check_port "$PG_HOST" "$PG_PORT"; then
  log_err "PostgreSQL is NOT reachable on ${PG_HOST}:${PG_PORT}."
  log_warn "Start your local PostgreSQL server and run: psql -U postgres -f scripts/init_db.sql"
  exit 1
fi
log_ok "PostgreSQL is reachable on ${PG_HOST}:${PG_PORT}."

# Optional SQL migrations if psql is available and DATABASE_URL is set
if command -v psql >/dev/null 2>&1 && [[ -n "${DATABASE_URL:-}" ]]; then
  log_info "Applying SQL migrations via psql..."
  for sql_file in \
    "$REPO_ROOT/apps/backend/migrations/000_bootstrap.sql" \
    "$REPO_ROOT/apps/backend/migrations/001_step5_tables.sql" \
    "$REPO_ROOT/apps/backend/migrations/002_users.sql" \
    "$REPO_ROOT/apps/backend/migrations/003_treasury_owner.sql" \
    "$REPO_ROOT/apps/backend/migrations/004_treasury_assets_liabilities.sql" \
    "$REPO_ROOT/apps/backend/migrations/005_audit_logs_entity_id_text.sql"
  do
    if [[ -f "$sql_file" ]]; then
      psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -f "$sql_file" >/dev/null 2>&1 || \
        log_warn "Migration step $(basename "$sql_file") warned (Go backend also auto-migrates on connect)."
    fi
  done
  log_ok "SQL migrations checked."
fi

# 3. Resolve Python, Go, npm
PYTHON_BIN=""
if command -v python3 >/dev/null 2>&1; then
  PYTHON_BIN="$(command -v python3)"
elif command -v python >/dev/null 2>&1; then
  PYTHON_BIN="$(command -v python)"
else
  log_err "python3/python executable not found on PATH."
  exit 1
fi

command -v go >/dev/null 2>&1 || { log_err "go executable not found on PATH."; exit 1; }
command -v npm >/dev/null 2>&1 || { log_err "npm executable not found on PATH."; exit 1; }

if [[ ! -d "$REPO_ROOT/apps/frontend/node_modules" ]]; then
  log_info "Installing frontend dependencies in apps/frontend..."
  (cd "$REPO_ROOT/apps/frontend" && npm ci)
fi

# 4. Launch services natively in background with PID & log tracking
start_service() {
  local key="$1"
  local workdir="$2"
  shift 2
  log_info "Starting $key..."
  (
    cd "$workdir"
    nohup "$@" >"$LOG_DIR/${key}.log" 2>&1 &
    echo $! >"$LOG_DIR/${key}.pid"
  )
}

start_service "ai-engine" "$REPO_ROOT/services/ai-engine" \
  "$PYTHON_BIN" -m uvicorn app.main:app --host 0.0.0.0 --port 8000

start_service "optimization-engine" "$REPO_ROOT/services/optimization-engine" \
  "$PYTHON_BIN" -m uvicorn app.main:app --host 0.0.0.0 --port 8001

start_service "backend" "$REPO_ROOT/apps/backend" \
  go run ./cmd/server

start_service "frontend" "$REPO_ROOT/apps/frontend" \
  npm run dev -- --port 3000

# 5. Poll health endpoints
wait_for_http() {
  local name="$1"
  local url="$2"
  local timeout="${3:-90}"
  local elapsed=0
  while (( elapsed < timeout )); do
    if curl -fsS --max-time 2 "$url" >/dev/null 2>&1; then
      log_ok "$name is healthy ($url)"
      return 0
    fi
    sleep 2
    elapsed=$((elapsed + 2))
  done
  log_warn "$name did not report healthy within ${timeout}s (check $LOG_DIR)"
  return 1
}

log_info "Waiting for service health checks..."
wait_for_http "AI Decision Engine" "http://localhost:8000/health" 90 || true
wait_for_http "Optimization Engine" "http://localhost:8001/health" 90 || true
wait_for_http "Go Backend Gateway" "http://localhost:8080/api/v1/health" 90 || true

if check_port "localhost" 3000; then
  log_ok "SvelteKit Frontend dev server is listening (http://localhost:3000)"
else
  sleep 3
  if check_port "localhost" 3000; then
    log_ok "SvelteKit Frontend dev server is listening (http://localhost:3000)"
  else
    log_warn "Frontend on port 3000 still starting (check $LOG_DIR/frontend.log)"
  fi
fi

cat <<EOF

QUANTA Native Services Started:
  - Frontend Dashboard : http://localhost:3000
  - Go API Gateway     : http://localhost:8080/api/v1/health
  - AI Decision Engine : http://localhost:8000/docs
  - Optimization Lab   : http://localhost:8001/docs
  - Logs & PIDs        : $LOG_DIR

Stop services cleanly with: ./scripts/stop_quanta.sh
EOF
