# Learning & Engineering Log — QUANTA

> **Purpose:** Record architectural lessons, financial systems engineering trade-offs, and review findings across phases so design rationale remains transparent.

---

## Phase 0 — Architecture & Technical Review Lessons

### 1. Multi-Tenancy Needs Explicit Membership Modeling
- Coupling users 1:1 to a tenant organization breaks real corporate workflows where treasurers, auditors, or group analysts operate across multiple entities or organizations.
- **Takeaway:** Separate global `User` identity from `Organization` via `OrganizationMembership` with org-scoped roles, and enforce tenant-scoped authorization on every query.

### 2. Currency Aggregation Requires Explicit FX Provenance
- Summing raw numbers across accounts denominated in different currencies is a silent correctness bug.
- **Takeaway:** Store account balances in original currency, never aggregate cross-currency without an explicit timestamped `FXRateSnapshot`, and start Phases 2–3 single-currency-per-treasury while preserving the multi-currency schema path.

### 3. Append-Only Ledger Events vs. Fast-Read Balance Projections
- Relying solely on mutable account balance columns loses auditability, while scanning an unbounded ledger on every read hurts transactional locking.
- **Takeaway:** Keep `ledger_entries` strictly append-only (reversals only, never updates/deletes), update `ledger_balance` and `encumbered_balance` atomically in the same ACID transaction, and run deterministic reconciliation checks.

### 4. Financial Verification != Statistical Model Validation
- A Go backend can deterministically prove capital conservation, liquidity floors, counterparty limits, and decimal rounding—but it **cannot** prove whether a Python forecasting or risk model is statistically well-calibrated.
- **Takeaway:** Keep deterministic financial verification in Go and statistical validation (backtests, calibration, provenance) in Python, using one unified 5-state `OptimizationRun` status model (`COMPLETED`, `INFEASIBLE`, `SOLVER_FAILED`, `VERIFICATION_FAILED`, `CANCELLED`).

### 5. Live Re-Verification & Graceful Degradation
- Proposals can go stale if balances or policies change between submission and approval; meanwhile, a Python solver outage should never take down core Go/PostgreSQL treasury reads.
- **Takeaway:** Always re-verify live state inside the approval transaction, and isolate Python outages cleanly from Go liveness and read paths.
