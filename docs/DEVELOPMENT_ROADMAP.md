# Development Roadmap — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Rule:** Implementation begins **only** after Phase 0 documentation is reviewed and approved.

---

## Phase 0 — Architecture and Requirements
**Objective:** Establish a clean repository, multi-tenant domain model, financial correctness invariants, Go/Python boundaries, native PostgreSQL design, and ADRs.
- **Deliverables:** `README.md`, `docs/*.md`, and `docs/DECISIONS/ADR-001..004.md`.
- **Exit Criteria:** Explicit human review and approval of all Phase 0 documents; zero application code or dependencies introduced during Phase 0.

---

## Phase 1 — Go Backend Foundation
**Objective:** Bootstrap the Go modular monolith skeleton, HTTP server, configuration, observability, and reliability primitives on Windows.
- **Deliverables:**
  - Fresh Git repository initialization and `go.mod` setup.
  - Modular package layout (`org`, `treasury`, `ledger`, `constraints`, `governance`, `audit`, `auth`, `verifier`).
  - Liveness (`GET /health`) and readiness (`GET /api/v1/health`) endpoints.
  - `X-Correlation-ID` propagation middleware, structured logging, and canonical JSON error envelope.
  - Exact decimal arithmetic utility package (zero `float64` in monetary types) with unit tests.
- **Exit Criteria:** Go service builds natively on Windows, passes decimal/middleware unit tests, and responds cleanly to liveness/readiness checks.

---

## Phase 2 — PostgreSQL and Financial Domain
**Objective:** Stand up native Windows PostgreSQL, versioned schema migrations, multi-tenant organization tables, and append-only ledger persistence.
- **Deliverables:**
  - Native Windows PostgreSQL bootstrap SQL and versioned Go-managed migrations (`organizations`, `users`, `organization_memberships`, `treasury_entities`, `treasury_accounts`, `ledger_entries`, `fx_rate_snapshots`, `liquidity_policies`, `counterparties`, `financial_constraints`, `audit_events`).
  - Atomic ledger entry insertion + `treasury_accounts` balance/encumbrance projection updates inside ACID transactions.
  - Deterministic reversal entry (`REVERSAL`) and hold lifecycle (`HOLD_PLACED`, `HOLD_RELEASED`, `HOLD_SETTLED`) implementation.
  - Ledger-to-projection reconciliation check.
- **Exit Criteria:** Integration tests against native PostgreSQL pass, proving tenant isolation, zero float drift, atomic projection updates, reversal-only corrections, and reconciliation invariant checks.

---

## Phase 3 — Core Treasury Functionality
**Objective:** Implement full Go business workflows for treasury positions, single-currency enforcement, constraints, approval governance, and synthetic data seeding.
- **Deliverables:**
  - Multi-organization user authentication and org-scoped RBAC enforcement (`OrganizationMembership`).
  - Treasury position computation (`ledger_balance`, `encumbered_balance`, `available_balance`, `operating_liquidity`, `emergency_reserves`, `investable_capital`) with strict single-currency-per-treasury guard (preserving multi-currency `FXRateSnapshot` path).
  - Counterparty exposure tracking and hard/soft `FinancialConstraint` management.
  - Proposal & approval state machine (`DRAFT` → `PENDING_APPROVAL` → `APPROVED` / `REJECTED` / `EXPIRED`) with maker-checker enforcement and live approval-time re-verification.
  - Seeded, deterministic synthetic treasury data generator in Go.
- **Exit Criteria:** End-to-end Go API tests verify org-scoped RBAC, stale-proposal blocking, policy-change re-verification, and transactional audit logging with no Python dependency required for core reads/governance.

---

## Phase 4 — Python Intelligence Foundation
**Objective:** Establish the separate Python intelligence service, versioned Go ↔ Python contract, input validation, and forecasting baselines.
- **Deliverables:**
  - Isolated Python virtual environment on Windows and service entrypoint on `127.0.0.1`.
  - Versioned contract (`contract_version: "1.0"`) with decimal-string monetary serialization, correlation IDs, and bounded Go timeouts.
  - Input data validation, model provenance metadata (`model_name`, `model_version`, `seed`), and rolling out-of-time backtest harness.
  - Classical forecasting baselines (`Seasonal Naive`, `ETS`, `ARIMA/SARIMAX`) on synthetic cash-flow series.
  - Graceful degradation tests in Go confirming core treasury reads remain available when Python is stopped or timed out.
- **Exit Criteria:** Go ↔ Python contract tests pass, classical forecast baselines beat/validate against Seasonal Naive in backtests, and Python outage tests confirm zero core Go disruption.

---

## Phase 5 — Risk and Optimization
**Objective:** Implement risk analytics, classical constrained optimization, unified `OptimizationRun` statuses, and Go-side independent financial verification.
- **Deliverables:**
  - Risk simulation module (Monte Carlo, Historical/Parametric VaR, CVaR, stress testing) with statistical backtesting/calibration checks in Python.
  - Classical optimization baseline(s) (LP/QP/MILP via selected candidate such as HiGHS/OR-Tools/CVXPY/Pyomo).
  - Go independent financial verifier enforcing capital conservation, liquidity floors, counterparty limits, and Banker's Rounding residual assignment.
  - Unified 5-status `OptimizationRun` persistence (`COMPLETED`, `INFEASIBLE`, `SOLVER_FAILED`, `VERIFICATION_FAILED`, `CANCELLED`), permitting `AllocationProposal` creation **only** on `COMPLETED`.
  - Optional async `202 Accepted` job polling for long-running sweeps.
- **Exit Criteria:** Tests prove Go catches invalid/infeasible Python outputs (`VERIFICATION_FAILED` / `INFEASIBLE`), handles timeouts (`SOLVER_FAILED`), and preserves exact decimal capital conservation.

---

## Phase 6 — Decision Intelligence
**Objective:** Evaluate advanced ML forecasting, anomaly detection, quantum/quantum-inspired optimization benchmarks, and non-authoritative language explanations.
- **Deliverables:**
  - ML & foundation forecasting benchmarks (e.g., `XGBoost`, `LightGBM`, `TFT`, `Chronos`, `TimesFM`) compared against Phase 4 classical baselines.
  - Cash-flow anomaly detection prototype (`Isolation Forest`).
  - Quantum research track (`QUBO`, `QAOA`, quantum-inspired heuristics via `Qiskit` / `PennyLane` / `D-Wave Ocean`) benchmarked head-to-head against Phase 5 classical baselines.
  - Optional non-authoritative LLM narrator (`Qwen` / `Llama` / `GPT`) and policy retrieval (`BGE` / `E5`) with numeric grounding verification against Go-verified payloads.
- **Exit Criteria:** Reproducible benchmark reports document empirical accuracy, objective gap, feasibility rate, and runtime trade-offs; zero LLM-computed numbers reach authoritative state.

---

## Phase 7 — Enterprise Hardening
**Objective:** Harden security, concurrency, database privileges, multi-currency FX snapshot workflows, and operational automation for native Windows deployment.
- **Deliverables:**
  - Database role privilege hardening (revoking `UPDATE`/`DELETE` on append-only tables).
  - Multi-currency `FXRateSnapshot` conversion activation and historical reproducibility tests.
  - Concurrency/load testing on optimistic locking and `SELECT ... FOR UPDATE` approval paths.
  - Native Windows PowerShell lifecycle, backup, and smoke-test automation scripts (without Docker/K8s).
- **Exit Criteria:** Security review, concurrency stress tests, FX reproducibility checks, and native PowerShell smoke suite pass cleanly.

---

## Phase 8 — Frontend Integration
**Objective:** Build the human-in-the-loop web dashboard (Proposed: TypeScript + Svelte) consuming Go `/api/v1/*` APIs.
- **Deliverables (Proposed):**
  - Multi-organization switcher and authenticated session flow.
  - Treasury position dashboard (`Original` vs `Reporting` currency views, `Operating Liquidity`, `Emergency Reserves`, `Investable Capital`).
  - Proposal & approval queue with live re-verification feedback, diff view, and mandatory rejection rationale.
  - Forecasting, risk stress-test, and Classical-vs-Quantum benchmark comparison views.
- **Exit Criteria:** End-to-end browser verification against the native Windows Go + Python + PostgreSQL stack passes.
