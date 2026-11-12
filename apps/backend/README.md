# QUANTA Backend Service

High-performance Go backend service for the QUANTA AI-Powered Autonomous Balance Sheet Optimization and Treasury Management Platform.

---

## 1. Technical Stack

- **Language**: Go 1.27+
- **HTTP Routing**: `github.com/go-chi/chi/v5`
- **Database Driver & Pool**: `github.com/jackc/pgx/v5/pgxpool`
- **Database**: PostgreSQL 16
- **Architecture**: Layered Clean Architecture (cmd, config, domain, database, clients, repository, service, handlers, router)

---

## 2. Environment Variables

| Variable | Description | Default Value |
| :--- | :--- | :--- |
| `PORT` | HTTP server listening port | `8080` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable` |
| `AI_ENGINE_URL` | Member 1 AI Decision Engine base URL | `http://localhost:8000` |
| `OPTIMIZATION_ENGINE_URL` | Member 2 Optimization Engine base URL | `http://localhost:8001` |
| `JWT_SECRET` | HMAC-SHA256 secret key for signing JWT tokens | `dev-only-change-me-please` (logs WARN on startup if default) |
| `JWT_TTL` | JWT token validity lifespan | `1h` |
| `CORS_ALLOWED_ORIGINS` | Comma-separated whitelist of allowed CORS origins | `http://localhost:3000` |

> Note: `DATABASE_URL` credentials are automatically masked (e.g. `postgres://quanta_admin:*****@...`) in server logs to prevent credential leakage.

---

## 3. Database Schema & Idempotent Migrations

Database tables beyond baseline `init.sql` are managed via embedded SQL migration files under `apps/backend/migrations/`.
- Embedded into the Go binary at compile time via `//go:embed *.sql` (`migrations.Files`).
- Automatically executed when the database connection pool initializes on startup, as well as upon successful reconnection via the background retry worker.
- Migrations are tracked inside the `schema_migrations` table (`version VARCHAR(100) PRIMARY KEY, applied_at TIMESTAMPTZ DEFAULT now()`).
- All statements are fully idempotent (`CREATE TABLE IF NOT EXISTS`, `CREATE INDEX IF NOT EXISTS`) and wrapped in an atomic database transaction. Subsequent restarts check `schema_migrations` and skip already applied migrations without error.

### Tables Added in Step 6 (`002_users.sql`):
4. `users`:
   - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - `email VARCHAR(255) UNIQUE NOT NULL`
   - `password_hash TEXT NOT NULL`
   - `full_name VARCHAR(255)`
   - `role VARCHAR(20) NOT NULL DEFAULT 'viewer' CHECK (role IN ('admin','analyst','viewer'))`
   - `created_at TIMESTAMPTZ DEFAULT now()`
   - Indexes: `idx_users_email` on `(email)`, `idx_users_role` on `(role)`

### Tables Added in Step 5 (`001_step5_tables.sql`):
1. `financial_problems`:
   - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - `treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE`
   - `algorithm VARCHAR(50)`
   - `objective VARCHAR(100)`
   - `market_state VARCHAR(50)`
   - `problem_json JSONB NOT NULL`
   - `strategy_log_id UUID REFERENCES strategy_execution_logs(id) ON DELETE SET NULL`
   - `created_at TIMESTAMPTZ DEFAULT now()`
   - Index: `idx_financial_problems_treasury_id` on `(treasury_id)`

2. `orchestration_runs`:
   - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - `treasury_id UUID NOT NULL REFERENCES corporate_treasuries(id) ON DELETE CASCADE`
   - `status VARCHAR(30) NOT NULL` (e.g., `completed`, `failed`)
   - `ai_status VARCHAR(30)` (e.g., `completed`, `failed`)
   - `optimization_status VARCHAR(30)` (currently `not_available`)
   - `strategy_log_id UUID REFERENCES strategy_execution_logs(id) ON DELETE SET NULL`
   - `response_json JSONB`
   - `error_message TEXT`
   - `duration_ms NUMERIC(10,2)`
   - `created_at TIMESTAMPTZ DEFAULT now()`
   - Index: `idx_orchestration_runs_treasury_id` on `(treasury_id)`

3. `audit_logs`:
   - `id UUID PRIMARY KEY DEFAULT uuid_generate_v4()`
   - `action VARCHAR(100) NOT NULL`
   - `entity_type VARCHAR(50) NOT NULL`
   - `entity_id UUID`
   - `actor VARCHAR(100) NOT NULL DEFAULT 'system'`
   - `metadata JSONB`
   - `created_at TIMESTAMPTZ DEFAULT now()`
   - Indexes:
     - `idx_audit_logs_created_at_desc` on `(created_at DESC)`
     - `idx_audit_logs_entity_action` on `(entity_type, action)`

---

## 4. Implemented API Endpoints

### Authentication & User Endpoints (Step 6)
- `POST /api/v1/auth/register` - Register a new user account with email, password (min 8 chars), and full name. First registered user bootstraps as `admin`, subsequent users default to `viewer`.
- `POST /api/v1/auth/login` - Authenticate with email/password; returns JWT HS256 token and user profile. Timing-leak protected via dummy bcrypt comparisons for unknown emails.
- `GET /api/v1/auth/me` - Authenticated user identity and role (requires `Authorization: Bearer <token>`).
- `GET /api/v1/users` - Admin-only user list. `password_hash` is never included.
- `PATCH /api/v1/users/{id}/role` - Admin-only role modification (`admin`, `analyst`, `viewer`). Protected against demoting the sole remaining admin.

### Roles & Permissions Matrix (Step 6)

| Endpoint | Method | Public | Viewer | Analyst | Admin | Unauthenticated / Insufficient Role |
| :--- | :--- | :---: | :---: | :---: | :---: | :--- |
| `GET /` | GET | Yes | Yes | Yes | Yes | 200 OK |
| `GET /health` | GET | Yes | Yes | Yes | Yes | 200 OK |
| `GET /api/v1/health` | GET | Yes | Yes | Yes | Yes | 200 OK (503 if DB down) |
| `POST /api/v1/auth/register` | POST | Yes | Yes | Yes | Yes | 201 Created / 409 Conflict |
| `POST /api/v1/auth/login` | POST | Yes | Yes | Yes | Yes | 200 OK / 401 Invalid Credentials |
| `GET /api/v1/auth/me` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/treasuries` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/treasuries/{id}` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/treasuries/{id}/strategies` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/strategies/{id}` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/financial-problems` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/financial-problems/{id}` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `GET /api/v1/orchestrate/runs/{id}` | GET | No | Yes | Yes | Yes | 401 Unauthorized |
| `POST /api/v1/treasuries` | POST | No | No (403) | Yes | Yes | 401 (no token) / 403 (viewer) |
| `POST /api/v1/treasuries/{id}/strategies/generate` | POST | No | No (403) | Yes | Yes | 401 (no token) / 403 (viewer) |
| `POST /api/v1/financial-problems` | POST | No | No (403) | Yes | Yes | 401 (no token) / 403 (viewer) |
| `POST /api/v1/orchestrate/allocation` | POST | No | No (403) | Yes | Yes | 401 (no token) / 403 (viewer) |
| `GET /api/v1/audit-logs` | GET | No | No (403) | No (403) | Yes | 401 (no token) / 403 (viewer/analyst) |
| `GET /api/v1/users` | GET | No | No (403) | No (403) | Yes | 401 (no token) / 403 (viewer/analyst) |
| `PATCH /api/v1/users/{id}/role` | PATCH | No | No (403) | No (403) | Yes | 401 (no token) / 403 (viewer/analyst) |

### System & Health Endpoints
- `GET /` - Service identity and version (`{"service":"quanta-backend","version":"0.1.0"}`).
- `GET /health` - Lightweight container liveness check (`200 OK` `{"status":"ok"}`).
- `GET /api/v1/health` - Readiness probe checking active PostgreSQL connectivity and reporting AI engine upstream dependency status (`200 OK` `{"status":"ok","database":"up","dependencies":{"ai_engine":"up"}}` or `503 Service Unavailable` if database is down).

### Corporate Treasury Endpoints (Step 2)
- `GET /api/v1/treasuries?limit=&offset=` - Paginated list of corporate treasuries (`200 OK` `{"data":[...],"limit":N,"offset":N}`).
- `GET /api/v1/treasuries/{id}` - Retrieve a corporate treasury by its canonical UUID.
- `POST /api/v1/treasuries` - Onboard a new corporate treasury balance sheet entity.
  - Generates audit log `treasury.created`.

### Strategy Synthesis & Execution Logs (Step 4)
- `POST /api/v1/treasuries/{id}/strategies/generate` - Synthesizes an optimal capital allocation strategy using Member 1 AI Decision Engine and persists to `strategy_execution_logs`.
  - Automatically records audit log `strategy.generated`.
- `GET /api/v1/treasuries/{id}/strategies?limit=&offset=` - Paginated list of strategy execution logs for a corporate treasury.
- `GET /api/v1/strategies/{id}` - Retrieve a specific strategy execution log by its canonical UUID.

### Canonical Financial Problems (Step 5)
- `POST /api/v1/financial-problems` - Ingests a complete financial problem definition, validates schema rules, runs AI strategy generation, stores the problem with linked strategy, and writes an audit log.
  - Automatically records audit log `financial_problem.created`.
- `GET /api/v1/financial-problems?limit=&offset=&treasury_id=` - List financial problems with optional treasury filtering and pagination (newest first).
- `GET /api/v1/financial-problems/{id}` - Retrieve a canonical financial problem by ID.

### Multi-Engine Allocation Orchestration (Step 5)
- `POST /api/v1/orchestrate/allocation` - Master orchestration endpoint for multi-engine capital allocation:
  - Validates treasury and applies `ResolveStepSize` guard (default `capital / 20`, max 200 search steps).
  - Invokes AI Engine `POST /api/v1/strategy/generate` for primary strategy synthesis.
  - If `include_comparison: true`, simultaneously executes AI Engine `POST /api/v1/search/compare` across all search algorithms (BFS, DFS, UCS, A*).
  - Handles Optimization Engine availability gracefully: returns `optimization: {"status":"not_available","reason":"optimization engine not yet integrated"}` without making bogus network calls.
  - Persists orchestration lifecycle run to `orchestration_runs` (`status: completed` or `failed`).
  - Records audit log (`orchestration.completed` on success, or `orchestration.failed` on upstream failure).
  - On upstream failure (e.g. AI Engine stopped), records failed run, logs audit failure, and returns `502 Bad Gateway` (`{"error":"upstream AI decision engine unavailable"}`).
- `GET /api/v1/orchestrate/runs/{id}` - Retrieve orchestration run history and full response JSON by run ID.

### Audit Logs (Step 5)
- `GET /api/v1/audit-logs?limit=&offset=&entity_type=&action=` - Paginated and filtered query of immutable audit logs, ordered newest first.
  - Supports filtering by `action` (e.g., `strategy.generated`, `orchestration.completed`, `orchestration.failed`, `financial_problem.created`, `treasury.created`) and `entity_type`.
  - Default limit: 20, max limit: 100. Parameterized SQL queries ensure injection safety.
  - Best-Effort Guarantee: Audit write failures log warnings but never abort or fail the primary business transaction.

---

## 5. Capital Units and Scale Consistency

Corporate treasury capital is denominated in Crores (Cr) or normalized currency units (e.g. `100.0` Cr).
- **Recommended Test Treasury**:
  - `name`: `Demo Treasury`
  - `total_capital`: `100.0`
  - `risk_tolerance`: `0.50`
  - `min_liquidity_ratio`: `0.20`
- **Scale Consistency Rule**: Liabilities, asset costs, and constraint thresholds must be scaled consistently with total capital. For a 100 Cr treasury, senior debt principal of 15.0 Cr represents a realistic 15% debt reserve.
- **Search Complexity**: AI search algorithms evaluate allocations by discrete increments of `step_size`. Setting `step_size` too small relative to capital produces a massive combinatorial tree. Defaulting to `capital / 20` ensures fast, reliable convergence across all treasury balance sheet sizes.

---

## 6. Resilience & Fault Isolation

- **Database Resilience**: The HTTP server starts even if PostgreSQL is initially offline, serving liveness requests in degraded mode. A background worker retries every 5 seconds until reachable (`[INFO] database connected after retry`).
- **Idempotent Embedded Migrations**: Migrations run safely inside a transaction upon connection without modifying existing tables or duplicating records.
- **Upstream Engine Fault Isolation**: Calls to AI Engine use a 60-second context timeout. If unreachable, the backend captures the failure in `orchestration_runs` with status `failed`, writes an `orchestration.failed` audit record, and safely returns `502 Bad Gateway`.
- **Best-Effort Audit Logging**: Audit persistence runs non-blockingly so unexpected logging failures never interrupt user-facing operations.

---

## 7. Developer & Verification Commands (Windows CMD)

### Unit Tests
```cmd
cd apps\backend
go test -v ./...
```

### Integration Tests (Against Live PostgreSQL)
```cmd
cd apps\backend
set DATABASE_URL=postgres://quanta_admin:quanta_secure_dev_password@localhost:5432/quanta_db?sslmode=disable
go test -v -tags=integration ./...
```

### Static Analysis, Linting & Build
```cmd
cd apps\backend
gofmt -l .
go vet ./...
go build ./...
```

---

## 8. Docker Commands (Windows CMD)

### Build and Launch Full Core Stack
```cmd
docker compose up -d --build postgres ai-engine backend
```

### Check Container Status
```cmd
docker compose ps
```

### View Backend Logs & Migrations
```cmd
docker compose logs backend
```

### Restart Backend Container
```cmd
docker compose restart backend
```

---

## 9. End-to-End Verification Curl Commands (Windows CMD)

### User Registration (Bootstrap Admin & Viewer) (Windows CMD)
```cmd
curl.exe -i -s -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"email\":\"admin@quanta.local\",\"password\":\"password123\",\"full_name\":\"Quanta Admin\"}"
curl.exe -i -s -X POST http://localhost:8080/api/v1/auth/register -H "Content-Type: application/json" -d "{\"email\":\"viewer@quanta.local\",\"password\":\"password123\",\"full_name\":\"Quanta Viewer\"}"
```

### User Login & Token Extraction (Windows CMD)
```cmd
curl.exe -i -s -X POST http://localhost:8080/api/v1/auth/login -H "Content-Type: application/json" -d "{\"email\":\"admin@quanta.local\",\"password\":\"password123\"}"
```

### Authenticated Profile Inspection (Windows CMD)
```cmd
curl.exe -i -s http://localhost:8080/api/v1/auth/me -H "Authorization: Bearer <TOKEN>"
```

### Role Demotion/Promotion (Admin Only) (Windows CMD)
```cmd
curl.exe -i -s -X PATCH http://localhost:8080/api/v1/users/<USER_ID>/role -H "Authorization: Bearer <ADMIN_TOKEN>" -H "Content-Type: application/json" -d "{\"role\":\"analyst\"}"
```

### CORS Preflight Request (Windows CMD)
```cmd
curl.exe -i -X OPTIONS -H "Origin: http://localhost:3000" -H "Access-Control-Request-Method: POST" -H "Access-Control-Request-Headers: authorization,content-type" http://localhost:8080/api/v1/treasuries
```

### System Health
```cmd
curl.exe -i http://localhost:8080/api/v1/health
```

### Submit Canonical Financial Problem (Step 5)
```cmd
curl.exe -i -s -X POST http://localhost:8080/api/v1/financial-problems -H "Content-Type: application/json" -d @apps/backend/testdata/sample_financial_problem_request.json
```

### List Financial Problems
```cmd
curl.exe -i -s http://localhost:8080/api/v1/financial-problems
```

### Get Financial Problem by ID
```cmd
curl.exe -i -s http://localhost:8080/api/v1/financial-problems/<problem_id>
```

### Orchestrate Capital Allocation (Without Comparison)
```cmd
curl.exe -i -s -X POST http://localhost:8080/api/v1/orchestrate/allocation -H "Content-Type: application/json" -d @apps/backend/testdata/sample_orchestration_request.json
```

### Orchestrate Capital Allocation (With Multi-Search Comparison)
```cmd
curl.exe -i -s -X POST http://localhost:8080/api/v1/orchestrate/allocation -H "Content-Type: application/json" -d @apps/backend/testdata/sample_orchestration_compare_request.json
```

### Get Orchestration Run by ID
```cmd
curl.exe -i -s http://localhost:8080/api/v1/orchestrate/runs/<run_id>
```

### Query Audit Logs (All Recent)
```cmd
curl.exe -i -s "http://localhost:8080/api/v1/audit-logs?limit=10"
```

### Query Audit Logs by Action
```cmd
curl.exe -i -s "http://localhost:8080/api/v1/audit-logs?action=strategy.generated"
curl.exe -i -s "http://localhost:8080/api/v1/audit-logs?action=orchestration.completed"
curl.exe -i -s "http://localhost:8080/api/v1/audit-logs?action=orchestration.failed"
```

### Inspect Database Directly via psql (Windows CMD)
```cmd
docker compose exec postgres psql -U quanta_admin -d quanta_db -c "select action, entity_type, created_at from audit_logs order by created_at desc limit 10;"
docker compose exec postgres psql -U quanta_admin -d quanta_db -c "select id, status, ai_status, optimization_status, duration_ms from orchestration_runs order by created_at desc limit 5;"
docker compose exec postgres psql -U quanta_admin -d quanta_db -c "select id, algorithm, objective, created_at from financial_problems order by created_at desc limit 5;"
docker compose exec postgres psql -U quanta_admin -d quanta_db -c "select version, applied_at from schema_migrations;"
```

---

## 10. Integration Contract & Next Steps

- **Optimization Engine (Member 2)**: Currently, the optimization engine service (`services/optimization-engine`) has no exposed HTTP endpoints. The backend adheres cleanly to the contract by reporting `optimization_status: "not_available"` and `optimization: {"status":"not_available","reason":"optimization engine not yet integrated"}`. An explicit integration hook is reserved in `apps/backend/internal/clients/optimization_client.go` to connect once Member 2 deploys their service.
