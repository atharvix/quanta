# System Architecture — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Architecture Style:** Go Modular Monolith + Separate Python Intelligence Service + Native PostgreSQL on Windows.

---

## 1. High-Level Component Architecture

```
+-----------------------------------------------------------------------+
|                Future Frontend (Proposed: TypeScript + Svelte)        |
|                     Localhost UI (Human Review & Lab)                 |
+-----------------------------------------------------------------------+
                                    |
                                    | HTTP / REST JSON (Localhost)
                                    v
+-----------------------------------------------------------------------+
|                  Go Core Backend (Modular Monolith)                   |
|  Tenant Auth · Ledger · Financial Verification · Approvals · Audit    |
|                                                                       |
|  +--------------+  +---------------+  +------------+  +------------+  |
|  | Org & RBAC   |  |  Ledger & FX  |  | Financial  |  |  Approval  |  |
|  |   Module     |  |  Projections  |  |  Verifier  |  |  Workflow  |  |
|  +--------------+  +---------------+  +------------+  +------------+  |
|  +--------------+  +---------------+  +------------+                  |
|  | Counterparty |  |  Orchestrator |  |   Audit    |                  |
|  | & Policy     |  | (Timeouts/ID) |  |   Trail    |                  |
|  +--------------+  +---------------+  +------------+                  |
+-----------------------------------------------------------------------+
         |                                           |
         | SQL (ACID Transactions)                   | Explicit JSON Contract
         | Authoritative State                       | (Untrusted Advisory Compute)
         v                                           v
+---------------------------+       +-----------------------------------+
|   Native PostgreSQL       |       |   Python Intelligence Service     |
|   (Windows Service)       |       |   (Stateless / Non-Authoritative) |
|                           |       |                                   |
| - Organizations & Users   |       | - Forecasting & Backtesting       |
| - Append-Only Ledger      |       | - Risk Sim & Model Validation     |
| - Account Projections     |       | - Classical & QUBO/QAOA Solvers   |
| - Proposals & Audit Logs  |       | - Non-Authoritative LLM Narrator  |
+---------------------------+       +-----------------------------------+
```

---

## 2. Responsibility Separation: Financial Verification vs. Model Validation

**Confirmed Architectural Rule:** Go and Python perform strictly distinct forms of validation:

### A. Go-Side Financial Verification (Deterministic & Authoritative)
Go treats every Python allocation response as an **untrusted candidate** and deterministically verifies:
1. **Capital Conservation:** `sum(quantized_allocations) + unallocated_cash == investable_capital`.
2. **Liquidity & Reserve Floors:** Operating liquidity and emergency reserve invariants remain satisfied.
3. **Counterparty Constraints:** `current_exposure + proposed_delta <= max_exposure_limit` for every counterparty.
4. **Decimal Precision & Residual Accounting:** Exact decimal quantization (`NUMERIC` / Banker's Rounding) with zero unallocated penny drift.
5. **Allocation Validity & Invariants:** Non-negative amounts, active candidate IDs, individual bounds, and currency consistency.
- If any check fails, Go transitions the `OptimizationRun` to **`VERIFICATION_FAILED`** and blocks `AllocationProposal` creation.

### B. Python-Side Statistical & Model Validation (Research & Provenance)
Python is responsible for empirical model quality and statistical checks:
1. **Forecasting Backtests:** Out-of-time rolling evaluation against baselines (`Seasonal Naive`, `ETS`).
2. **Risk-Model Evaluation:** VaR exceedance tests, CVaR stability, and stress coverage.
3. **Calibration:** Quantile/interval coverage checks.
4. **Model Provenance & Version Tracking:** Emitting `model_name`, `model_version`, `seed`, and training-window metadata.
5. **Input Data Validation:** Checking time-series completeness, variance bounds, and matrix conditioning prior to fitting.
- **Confirmed Boundary:** Go **does not** and **cannot** independently prove the statistical correctness or predictive accuracy of a Python model's forecasts or risk estimates.

---

## 3. Reliability, Degradation & Native Windows Runtime

- **Confirmed Decision:** All components run natively on Windows (Go binary, Python venv, native PostgreSQL Windows service—no Docker, Docker Compose, Kubernetes, or Java).
- **Liveness vs. Readiness:** Go `/health` reports process liveness (`200`); `/api/v1/health` checks PostgreSQL (critical) and Python (degraded compute indicator).
- **Python Outage Resilience:** When Python is down or times out, all Go/PostgreSQL treasury reads, ledger views, audit queries, and rejections continue operating normally; only Python-backed compute endpoints fail cleanly with `503` or `SOLVER_FAILED` without mutating balances.
- **Correlation & Timeouts:** Every Go → Python call enforces `context.WithTimeout` and propagates `X-Correlation-ID`.

---

## 4. Decision Summary & Unresolved Questions

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Go modular monolith + separate Python service + native Windows PostgreSQL | **Confirmed** | See ADRs 001–004 |
| Strict split: Go financial verification vs. Python statistical validation | **Confirmed** | Go never claims statistical proof |
| Core treasury reads remain available during Python outage | **Confirmed** | Clean degradation on compute paths |
| Sync HTTP/JSON for fast solves + async polling for heavy runs | **Proposed** | Finalize threshold in Phase 4/5 |

**Unresolved Questions:**
1. Which specific compute operations should default to async `202 Accepted` execution in Phase 5?
2. Should Python run as a single FastAPI worker or spawn bounded subprocesses for CPU-heavy MILP/QAOA benchmarks?
