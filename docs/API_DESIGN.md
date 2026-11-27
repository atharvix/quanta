# API & Contract Design — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Scope:** Tenant-Scoped External REST API (Go Gateway), Reliability Semantics, and Explicit Go ↔ Python Contracts.

---

## 1. API Boundary & Tenant Context

- **Confirmed Decision:** External clients interact solely with the **Go Modular Monolith**. Python intelligence endpoints are internal-only on `127.0.0.1`.
- **Confirmed Decision (Tenant Isolation):** Every protected business endpoint validates the authenticated user's `OrganizationMembership` for the target `Organization` (`orgId`) before accessing any `TreasuryEntity`, `TreasuryAccount`, `OptimizationRun`, `AllocationProposal`, or `AuditEvent`.
- **Proposed Routing Convention:** Scope tenant resources explicitly under `/api/v1/orgs/{orgId}/...` (or via `X-Organization-ID` header—see Security Model unresolved question).

---

## 2. Service Reliability, Health & Error Semantics

### 2.1 Liveness vs. Readiness Probes
- **`GET /health` (Liveness):** Returns `200 OK` whenever the Go process is alive and accepting HTTP connections. **Never** fails due to a Python intelligence outage.
- **`GET /api/v1/health` (Readiness):** Checks PostgreSQL connectivity (critical: returns `503 Service Unavailable` if PostgreSQL is unreachable) and reports Python intelligence health (`UP` / `DOWN`) as a non-fatal dependency status (`"status": "ok"` when DB is up and Python is up; `"status": "degraded"` when DB is up and Python is down).

### 2.2 Availability During Python Outage (Confirmed Rule)
A Python intelligence outage **must never** block core Go/PostgreSQL operations:
- **Remains Available (`200` / `201`):** Account & balance reads, ledger history, counterparty/constraint reads and updates, proposal reads, proposal rejections, and audit log queries.
- **Degrades Cleanly (`503 Service Unavailable`):** Intelligence computation triggers (`forecast`, `risk/evaluate`, `optimize`) fail fast with structured diagnostics without mutating treasury balances.

### 2.3 Synchronous vs. Asynchronous Compute & Timeouts (Proposed)
- **Synchronous Mode:** Bounded short-running requests (e.g., deterministic LP/QP, small risk checks) execute within strict Go `context.WithTimeout` bounds (e.g., 10–15s). On timeout, Go cancels the Python request and persists `OptimizationRun.status = SOLVER_FAILED`.
- **Asynchronous Mode:** Heavy Monte Carlo sweeps, multi-model forecast backtests, or QAOA benchmarks return `202 Accepted` with `{ "run_id": "...", "status": "PENDING" }` and poll `GET /api/v1/orgs/{orgId}/optimization-runs/{runId}` (exact threshold unresolved).

### 2.4 Correlation ID & Structured Error Envelope (Proposed)
- Every inbound request accepts or generates `X-Correlation-ID`, echoes it in response headers, propagates it to Python, and writes it to `audit_events`.
- Canonical error JSON:
```json
{
  "error": {
    "code": "UPSTREAM_UNAVAILABLE",
    "message": "Python intelligence service is unreachable",
    "correlation_id": "uuid",
    "details": []
  }
}
```

---

## 3. External REST API Surface (Go Core — Proposed)

### 3.1 Auth, Organizations & Treasury Positions
- `POST /api/v1/auth/login` / `GET /api/v1/auth/me` (returns user profile + active organization memberships)
- `GET /api/v1/orgs/{orgId}/treasuries`
- `GET /api/v1/orgs/{orgId}/treasuries/{entityId}/accounts`
- `GET /api/v1/orgs/{orgId}/accounts/{accountId}` & `GET /api/v1/orgs/{orgId}/accounts/{accountId}/ledger`
- `POST /api/v1/orgs/{orgId}/accounts/{accountId}/credit` | `.../debit` | `.../holds` | `.../holds/release` | `.../holds/settle` | `.../reconcile`
- `POST /api/v1/orgs/{orgId}/ledger-entries/{entryId}/reverse`
- `GET/POST /api/v1/orgs/{orgId}/treasuries/{entityId}/liquidity-policies` & `PUT .../liquidity-policies/{policyId}` & `POST .../liquidity-policies/validate`
- `GET/POST /api/v1/orgs/{orgId}/treasuries/{entityId}/counterparties` & `PUT .../counterparties/{counterpartyId}` & `POST .../counterparties/{counterpartyId}/validate-exposure`

#### Milestone 3 Request/Response Examples
**Login (`POST /api/v1/auth/login`):**
```json
{
  "email": "analyst@quanta.local",
  "password": "<LOCAL_SYNTHETIC_ANALYST_PASSWORD>"
}
```
Response (`200 OK`):
```json
{
  "access_token": "<JWT_HS256_TOKEN>",
  "token_type": "Bearer",
  "expires_at": "2026-11-28T01:55:00Z",
  "user": { "id": "uuid", "email": "analyst@quanta.local", "full_name": "Alex Analyst", "is_active": true },
  "memberships": [{ "id": "uuid", "organization_id": "uuid", "role": "TREASURY_ANALYST", "status": "ACTIVE" }]
}
```

**Credit Account (`POST /api/v1/orgs/{orgId}/accounts/{accountId}/credit`):**
```json
{
  "amount": "1000.0000",
  "reference_type": "DEMO_SEED"
}
```
Response (`201 Created`):
```json
{
  "entry": { "id": "uuid", "entry_type": "CREDIT", "currency_code": "USD", "delta_amount": "1000.0000", "balance_after": "1000.0000", "account_version": 2 },
  "account": { "id": "uuid", "ledger_balance": "1000.0000", "encumbered_balance": "0.0000", "available_balance": "1000.0000", "version": 2 }
}
```

**Validate Liquidity Policy (`POST /api/v1/orgs/{orgId}/treasuries/{entityId}/liquidity-policies/validate`):**
```json
{
  "total_available": "1000.0000",
  "proposed_allocation": "750.0000"
}
```
Response (`422 Unprocessable Entity` when breaching `min_operating_liquidity + min_emergency_reserve`):
```json
{
  "error": {
    "code": "LIQUIDITY_POLICY_VIOLATION",
    "message": "allocation violates liquidity policy floors",
    "correlation_id": "uuid",
    "details": { "required_floor": "300.0000", "post_allocation_cash": "250.0000", "investable_capacity": "700.0000" }
  }
}
```

> **Local Synthetic Credentials Note:** Never commit plaintext passwords or JWT secrets. Seeded synthetic analyst (`analyst@quanta.local`, role `TREASURY_ANALYST`) and approver (`approver@quanta.local`, role `TREASURY_APPROVER`) fixtures are hashed via `bcrypt` at test/seed time using local-only environment variables or test fixtures.

### 3.2 Intelligence, Proposals & Governance
- `POST /api/v1/orgs/{orgId}/treasuries/{id}/forecast`
- `POST /api/v1/orgs/{orgId}/treasuries/{id}/risk/evaluate`
- `POST /api/v1/orgs/{orgId}/treasuries/{id}/optimize` — Runs solver(s), runs Go financial verification, persists `OptimizationRun` (`COMPLETED | INFEASIBLE | SOLVER_FAILED | VERIFICATION_FAILED | CANCELLED`). Only **`COMPLETED`** runs allow creating a `DRAFT` `AllocationProposal`.
- `POST /api/v1/orgs/{orgId}/proposals/{id}/submit` (`DRAFT` → `PENDING_APPROVAL`)
- `POST /api/v1/orgs/{orgId}/proposals/{id}/approve` (Live re-verification → `APPROVED`)
- `POST /api/v1/orgs/{orgId}/proposals/{id}/reject` (`PENDING_APPROVAL` → `REJECTED`, terminal)
- `GET /api/v1/orgs/{orgId}/audit-events`

---

## 4. Explicit Go ↔ Python Internal Contracts

**Confirmed Decision:** All monetary amounts cross the Go ↔ Python boundary as **exact decimal strings** alongside `contract_version: "1.0"`, `correlation_id`, and `base_currency`.

### 4.1 Optimization Request (`POST /internal/v1/optimize`)
```json
{
  "contract_version": "1.0",
  "correlation_id": "uuid",
  "treasury_id": "uuid",
  "base_currency": "CURR",
  "investable_capital": "750000.0000",
  "operating_liquidity_floor": "150000.0000",
  "emergency_reserve_floor": "100000.0000",
  "risk_tolerance": "0.500000",
  "candidates": [],
  "counterparty_limits": [],
  "solver_mode": "classical_and_quantum_benchmark",
  "timeout_ms": 15000
}
```

### 4.2 Optimization Response (Consistent Status Model)
Python returns one of `COMPLETED` (solver found feasible candidate), `INFEASIBLE` (no feasible allocation exists), `SOLVER_FAILED` (crash/timeout/non-convergence), or `CANCELLED`. Go then performs independent financial verification and may downgrade a `COMPLETED` Python response to **`VERIFICATION_FAILED`** if invariants fail.

```json
{
  "contract_version": "1.0",
  "correlation_id": "uuid",
  "status": "COMPLETED",
  "feasible": true,
  "solver_used": "highs_lp",
  "allocations": {
    "bucket_a": "250000.0000"
  },
  "unallocated_cash": "500000.0000",
  "metrics": {
    "expected_return": "0.052000",
    "portfolio_risk": "0.021000",
    "post_allocation_liquidity": "0.960000"
  },
  "model_validation": {
    "model_version": "0.1.0",
    "input_checks_passed": true
  },
  "benchmark_comparison": [],
  "constraint_violations": [],
  "warnings": [],
  "runtime_ms": 18
}
```

---

## 5. Decision Summary & Unresolved Questions

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Separate `/health` (liveness) vs `/api/v1/health` (readiness) | **Confirmed** | Python outage must not block Go reads |
| Unified 5-status optimization response model | **Confirmed** | Matches Domain Model & DB Design |
| Decimal strings across Go ↔ Python wire | **Confirmed** | Prevents float drift |
| `/api/v1/orgs/{orgId}/...` routing & `202` async pattern | **Proposed** | Finalize in Phase 1/5 |

**Unresolved Questions:**
1. Should `organization_id` live in the URL path (`/orgs/{orgId}/...`), `X-Organization-ID` header, or JWT claim?
2. Which exact runtime threshold separates synchronous HTTP compute from asynchronous `202 Accepted` polling?
