# QUANTA — Autonomous Financial Treasury & Capital Optimization Platform

[![Python](https://img.shields.io/badge/Python-3.12%2B-blue.svg)](https://www.python.org/)
[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8.svg)](https://go.dev/)
[![FastAPI](https://img.shields.io/badge/FastAPI-0.115%2B-green.svg)](https://fastapi.tiangolo.com/)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-Svelte%205-FF3E00.svg)](https://svelte.dev/)
[![License](https://img.shields.io/badge/License-Academic%20Research-purple.svg)]()

> **Enterprise-Oriented AI Financial Decision Intelligence & Optimization Platform**
> QUANTA evaluates corporate balance sheets, investment opportunities, liabilities, liquidity requirements, and risk constraints to synthesize optimal, deterministic, and explainable capital allocation strategies across heuristic AI search, convex mathematical programming, and QUBO/QAOA quantum simulation.

---

## Team & Module Ownership

| Member | Role & Module | Responsibilities |
| :--- | :--- | :--- |
| **Atharv (Team Lead)** | **AI Decision Engine (`services/ai-engine/`)** | Search Algorithms (BFS, DFS, Best-First, Hill Climbing, A*), CSP, Bayesian Risk Reasoning, Game Theory (Minimax/Alpha-Beta), MDP & Multi-Agent Council |
| **Parv** | **Optimization Engine (`services/optimization-engine/`)** | Mathematical Programming (Exact, Greedy, SA, SciPy SLSQP, MILP), QUBO Formulations, QAOA Statevector Simulation & Classical-vs-Quantum Benchmarking |
| **Aryan** | **Backend & Platform (`apps/backend/`, `apps/frontend/`)** | Go REST API Gateway, PostgreSQL, JWT Auth/RBAC, Audit Logs, SvelteKit Dashboard, Optimization Lab & Orchestration |

---

## Monorepo Architecture

```text
quanta/
├── apps/
│   ├── frontend/             # SvelteKit 5 + TypeScript Dashboard (Port 3000)
│   └── backend/              # Go Chi API Gateway & PostgreSQL Migrations (Port 8080)
├── services/
│   ├── ai-engine/            # Python FastAPI AI Decision Engine (Port 8000)
│   └── optimization-engine/  # Python FastAPI Optimization & QAOA Engine (Port 8001)
├── shared/
│   └── schemas/              # Canonical JSON Schema Contracts
├── scripts/                  # Polyglot Local Lifecycle & Smoke Test Suite
├── infrastructure/           # PostgreSQL init schema & optional container configs
├── docs/                     # Architectural, AI Engine & Integration specs
├── quanta.bat                # One-click Windows local launcher
└── docker-compose.yml        # Optional container orchestration
```

---

## Quickstart: Native Local Execution (Primary — No Docker Required)

### Prerequisites
- **Python 3.12+** (`services/ai-engine` & `services/optimization-engine`)
- **Go 1.24+** (`apps/backend`)
- **Node.js 20+ / npm** (`apps/frontend`)
- **PostgreSQL 15/16+** running locally on `localhost:5432`

### 1. Initialize Local Dependencies & Database
```powershell
# Python engines
pip install -r services/ai-engine/requirements.txt
pip install -r services/optimization-engine/requirements.txt

# SvelteKit frontend
cd apps/frontend && npm install && cd ../..

# Initialize PostgreSQL role (`quanta`), database (`quanta_db`), and core tables
psql -U postgres -f scripts/init_db.sql
```

Root `.env` and `apps/frontend/.env` are preconfigured for native localhost execution (`localhost:5432`, `8000`, `8001`, `8080`, `3000`).

### 2. Start All Services Natively
- **Windows (Double-click or PowerShell):**
  ```powershell
  .\quanta.bat
  # or directly:
  powershell -ExecutionPolicy Bypass -File .\scripts\start_quanta.ps1
  ```
- **macOS / Linux / Git Bash / WSL:**
  ```bash
  ./scripts/start_quanta.sh
  ```

Once healthy, all four native endpoints are available:
- **Web Dashboard & Optimization Lab:** `http://localhost:3000`
- **Go API Gateway:** `http://localhost:8080/api/v1/health`
- **AI Decision Engine (Swagger):** `http://localhost:8000/docs`
- **Optimization & QAOA Engine (Swagger):** `http://localhost:8001/docs`

### 3. Verify End-to-End Live Flow & Stop Services
```powershell
# Run end-to-end API smoke test (health checks, JWT auth, optimization compare, stress test)
powershell -ExecutionPolicy Bypass -File .\scripts\smoke_test.ps1

# Cleanly stop all native local services (ports 3000, 8080, 8000, 8001)
powershell -ExecutionPolicy Bypass -File .\scripts\stop_quanta.ps1
# or on POSIX shells:
./scripts/stop_quanta.sh
```

---

## Running Unit Test Suites Individually

```powershell
# AI Decision Engine (49 tests)
cd services/ai-engine && pytest -v

# Optimization & QAOA Engine (28 tests)
cd services/optimization-engine && pytest -v

# Go Backend Gateway & Services
cd apps/backend && go test ./...

# SvelteKit Frontend
cd apps/frontend && npm run check && npm test
```

---

## Optional: Docker Compose Execution

If you prefer containers instead of running natively, an optional [docker-compose.yml](file:///c:/Users/Lab/personal/quanta/docker-compose.yml) configuration is maintained at the repository root:

```bash
docker compose up --build
```

---

## Disclaimer
QUANTA is an academic research prototype and simulated decision intelligence framework. All quantum optimization workflows run on deterministic classical CPU statevector simulation; no quantum hardware execution or financial advice is claimed.
