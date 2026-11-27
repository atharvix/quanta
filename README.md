# QUANTA

**Semi-Autonomous Corporate Treasury & Capital Optimization Platform**

> **Current Project Status:** Phase 0 — Architecture, Financial Correctness & Design Documentation (**Pending Review & Approval**)  
> **Operating Mode:** Semi-Autonomous (Human approval required for all consequential financial actions; no autonomous transfers or trades)

---

## Project Description

QUANTA is an enterprise-grade corporate treasury and capital optimization platform designed for **corporate treasury departments**. It monitors multi-account treasury positions, forecasts cash-flow trajectories, simulates downside risk, optimizes constrained capital allocation, and recommends verifiable strategies for human review.

Initial development uses **realistic synthetic financial data** without assuming external banking APIs, market data feeds, or regulatory certifications.

---

## Technology Direction

| Layer | Technology / Role | Status |
| :--- | :--- | :--- |
| **Core Backend** | **Go** modular monolith owning tenant isolation, authoritative ledger/balances, Go-side financial verification, approval workflows, and auditability | **Confirmed** |
| **Intelligence & Optimization** | Separate **Python** intelligence service for forecasting, risk simulation, statistical model validation, classical/quantum-research optimization, and non-authoritative narrative assistance | **Confirmed** |
| **Database** | **PostgreSQL** installed natively on Windows | **Confirmed** |
| **Frontend (Phase 8)** | **TypeScript and Svelte** | **Proposed** |
| **VCS & Environment** | **Git**, **Windows**, and **PowerShell** | **Confirmed** |

**Explicitly Excluded:** Docker, Docker Compose, Kubernetes, Java / Spring Boot, premature microservices, and unnecessary infrastructure.

---

## Development Principles

1. **Financial Correctness First:** Never use ordinary binary floating-point types (`float32`/`float64`/`REAL`) for authoritative monetary storage or ledger arithmetic. Use fixed-point `NUMERIC` in PostgreSQL, exact decimals in Go, and decimal strings across Go ↔ Python contracts.
2. **Append-Only Truth:** Financial history (`ledger_entries`, `approval_decisions`, `audit_events`, `fx_rate_snapshots`) is never destructively modified; corrections append offsetting reversals.
3. **Currency Isolation:** Never aggregate balances across different currencies without an explicit `FXRateSnapshot` (rate, timestamp, provenance).
4. **Clear Validation Split:** Go deterministically verifies financial invariants (conservation, liquidity floors, counterparty limits, precision); Python performs statistical/model validation (backtests, calibration, provenance).
5. **Non-Authoritative AI & Empirical Quantum Discipline:** LLMs never compute financial metrics; quantum/quantum-inspired methods are always benchmarked against classical baselines.
6. **Phased Discipline:** Phase 0 produces documentation only—no application code, migrations, or dependencies are created until Phase 0 is approved.

---

## Documentation Index

### Architecture & Design (`docs/`)
- [Product Vision](docs/PRODUCT_VISION.md) — Mission, semi-autonomous model, personas, and non-goals.
- [Requirements](docs/REQUIREMENTS.md) — Multi-tenancy, ledger, verification, reliability, and operational requirements.
- [Domain Model](docs/DOMAIN_MODEL.md) — Organization hierarchy, currency/FX rules, ledger projections, unified optimization statuses, and proposal lifecycle.
- [System Architecture](docs/SYSTEM_ARCHITECTURE.md) — Go modular monolith, Python boundary, financial vs. model validation split, and failure modes.
- [Database Design](docs/DATABASE_DESIGN.md) — Native PostgreSQL schema, `NUMERIC` precision, append-only tables, and locking.
- [API Design](docs/API_DESIGN.md) — Org-scoped REST API, liveness/readiness, outage degradation, and Go ↔ Python contracts.
- [Security Model](docs/SECURITY_MODEL.md) — Multi-org membership RBAC, maker-checker governance, approval-time re-verification, and auditability.
- [AI Strategy](docs/AI_STRATEGY.md) — Candidate forecasting, risk, optimization, quantum research, and LLM approaches.
- [Development Roadmap](docs/DEVELOPMENT_ROADMAP.md) — 9-phase delivery plan (Phase 0 through Phase 8) with deliverables and exit criteria.
- [Learning Log](docs/LEARNING_LOG.md) — Engineering principles, trade-offs, and Phase 0 review lessons.

### Architecture Decision Records (`docs/DECISIONS/`)
- [ADR-001: Core Backend Language (Go)](docs/DECISIONS/ADR-001-CORE-BACKEND-LANGUAGE.md)
- [ADR-002: Modular Monolith Architecture](docs/DECISIONS/ADR-002-MODULAR-MONOLITH.md)
- [ADR-003: Python Intelligence Boundary](docs/DECISIONS/ADR-003-PYTHON-INTELLIGENCE-BOUNDARY.md)
- [ADR-004: Native PostgreSQL Database Strategy](docs/DECISIONS/ADR-004-DATABASE-STRATEGY.md)

---

## Setup & Execution (Milestone 1 & Milestone 2)

> **Current Status:** Milestone 2 (PostgreSQL Schema, Ledger & Multi-Tenancy) implemented in `apps/backend`. PostgreSQL remains the sole intended system of record (no SQLite, no Docker). On restricted lab machines without Administrator privileges, PostgreSQL installation and live integration verification (`QUANTA_TEST_DATABASE_URL`) are deferred to the developer's personal laptop.

### 1. Native PostgreSQL Setup (Personal Windows Laptop)
Install PostgreSQL 16 natively (requires Windows Administrator authorization):
```powershell
winget install --exact --id PostgreSQL.PostgreSQL.16
```
Configure the `quanta` role and `quanta_db` database without committing secrets:
```powershell
psql -U postgres -h 127.0.0.1 -p 5432 -c "CREATE ROLE quanta WITH LOGIN PASSWORD '<YOUR_LOCAL_PASSWORD>';"
psql -U postgres -h 127.0.0.1 -p 5432 -c "CREATE DATABASE quanta_db OWNER quanta;"
```

### 2. Run & Test the Go Backend (`apps/backend`)
Copy `.env.example` to `.env` (gitignored) and set `DATABASE_URL` and a 32+ character `JWT_SECRET`, then run:
```powershell
cd apps\backend
gofmt -l .
go vet ./...
go build ./...
go test -v ./...
```
To start the HTTP server locally (requires `DATABASE_URL` and `JWT_SECRET` in environment):
```powershell
$env:DATABASE_URL="postgres://quanta:<YOUR_LOCAL_PASSWORD>@127.0.0.1:5432/quanta_db?sslmode=disable"
$env:JWT_SECRET="<YOUR_32_PLUS_CHAR_LOCAL_SECRET>"
$env:JWT_ISSUER="quanta-backend"
$env:JWT_AUDIENCE="quanta-api"
go run ./cmd/server
```
### 3. Run & Test the Python Intelligence Service (`services/intelligence`)
```powershell
cd services\intelligence
python -m pytest -v
python -m uvicorn app.main:app --host 127.0.0.1 --port 8001
```

To run live PostgreSQL integration tests once PostgreSQL is installed on your personal laptop:
```powershell
$env:QUANTA_TEST_DATABASE_URL="postgres://quanta:<YOUR_LOCAL_PASSWORD>@127.0.0.1:5432/quanta_db?sslmode=disable"
go test -v ./internal/repository/postgres/...
```
Verify liveness (`GET http://127.0.0.1:8080/health`) and readiness (`GET http://127.0.0.1:8080/api/v1/health`).
