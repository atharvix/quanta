# Backend Service (Aryan - Member 3)

This directory houses the Go REST API and database integration layer for the QUANTA Autonomous Financial Treasury & Capital Optimization Platform.

---

## 1. Technology Stack
- **Language**: Go 1.27+
- **HTTP Routing**: `github.com/go-chi/chi/v5`
- **Database Driver & Pool**: `github.com/jackc/pgx/v5/pgxpool`
- **Database**: PostgreSQL 16
- **Architecture**: Layered Clean Architecture (cmd, config, domain, database, repository, service, handlers, router)

---

## 2. Environment Variables

| Variable | Description | Default Value |
| :--- | :--- | :--- |
| `PORT` | HTTP server listening port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable` |
| `AI_ENGINE_URL` | Member 1 AI Decision Engine base URL | `http://localhost:8000` |
| `OPTIMIZATION_ENGINE_URL` | Member 2 Optimization Engine base URL | `http://localhost:8001` |

> Note: `DATABASE_URL` credentials are automatically masked (e.g. `postgres://quanta_admin:*****@...`) in server logs to prevent credential leakage.

---

## 3. Implemented API Endpoints

### System & Health Endpoints
- `GET /` — Service identity and version (`{"service":"quanta-backend","version":"0.1.0"}`).
- `GET /health` — Lightweight container liveness check (`200 OK` `{"status":"ok"}`).
- `GET /api/v1/health` — Readiness probe checking active PostgreSQL connectivity (`200 OK` `{"status":"ok","database":"up"}` or `503 Service Unavailable` `{"database":"down","status":"degraded"}`).

### Corporate Treasury Endpoints (Step 2)
- `GET /api/v1/treasuries?limit=&offset=` — Paginated list of corporate treasuries (`200 OK` `{"data":[...],"limit":N,"offset":N}`).
  - Validates `limit` (default: 20, max: 100) and `offset` (must be >= 0).
  - Returns `503 Service Unavailable` if database is disconnected.
- `GET /api/v1/treasuries/{id}` — Retrieve a corporate treasury by its canonical UUID.
  - Returns `400 Bad Request` if UUID format is invalid.
  - Returns `404 Not Found` if UUID does not exist.
  - Returns `503 Service Unavailable` if database is down.
- `POST /api/v1/treasuries` — Onboard a new corporate treasury balance sheet entity.
  - Restricts payload body size to 1 MB.
  - Disallows unknown JSON fields (`DisallowUnknownFields`).
  - Returns `201 Created` with persisted entity and generated UUID.
  - Returns `400 Bad Request` with field-level details `{"error":"validation failed","details":[...]}` on constraint violations.

---

## 4. Resilience & Background Reconnection
- The HTTP server starts even if PostgreSQL is initially offline, serving liveness requests in degraded mode.
- A background worker ticker retries connecting to PostgreSQL every 5 seconds until reachable.
- Once PostgreSQL recovers, the pool is populated and logged: `[INFO] database connected after retry`.

---

## 5. Developer Commands

### Run Unit Tests
```bash
go test -v ./...
```

### Run Integration Tests (Requires Running PostgreSQL)
```cmd
set DATABASE_URL=postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable
go test -v -tags=integration ./...
```

### Static Analysis & Formatting
```bash
gofmt -l .
go vet ./...
go build ./...
```

### Start Server Locally
```bash
go run ./cmd/server
```

---

## 6. Docker Containerization & Compose Orchestration

### Multi-Stage Build
The backend uses a multi-stage Dockerfile located at `apps/backend/Dockerfile`:
- **Builder Stage**: `golang:1.27-alpine` installs dependencies and compiles a statically linked, CGO-disabled binary (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`).
- **Runtime Stage**: `alpine:latest` runs the binary as an unprivileged user (`appuser`, UID 10001) with enterprise certificates and timezone data.
- **Container Health Check**: Evaluates liveness every 10 seconds via `wget -qO- http://localhost:8080/health`.

### Docker Commands (Windows CMD)

#### Build Backend Image
```cmd
docker compose build backend
```

#### Launch Core Stack (PostgreSQL, AI Engine, Backend)
```cmd
docker compose up -d postgres ai-engine backend
```

#### Check Container Status & Health
```cmd
docker compose ps
```

#### View Service Logs
```cmd
docker compose logs backend
docker compose logs --tail=30 ai-engine
```

#### Verify Inter-Container Networking (Backend -> AI Engine)
```cmd
docker compose exec backend wget -qO- http://ai-engine:8000/health
```

#### Stop Services
```cmd
docker compose stop postgres ai-engine backend
```
