# System Requirements — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Scope:** Multi-Tenancy, Currency & Ledger Correctness, Verification Separation, Governance, Reliability, and Operational Requirements.

---

## 1. Functional Requirements

### 1.1 Multi-Tenancy & Organization Access
- **FR-01 (Organizations & Multi-Org Users):** The platform shall distinguish `Organization`, `User`, `OrganizationMembership`, `TreasuryEntity`, and `TreasuryAccount`, allowing a `User` to hold memberships in one or more organizations with organization-scoped roles.
- **FR-02 (Tenant Isolation):** Every treasury read, mutation, optimization, proposal, approval, and audit query shall be authorized strictly within the active `Organization` scope.

### 1.2 Ledger, Currency & Balance Projections
- **FR-03 (Append-Only Ledger & Projections):** The Go core shall record all monetary movements, reversals, and holds in append-only `ledger_entries` and update `treasury_accounts.ledger_balance` and `encumbered_balance` atomically in PostgreSQL transactions.
- **FR-04 (Reconciliation & Reversals):** Historical `ledger_entries` shall never be modified or deleted. Corrections shall use offsetting `REVERSAL` entries, and Go shall support deterministic reconciliation comparing projections against `SUM(ledger_entries)`.
- **FR-05 (Currency Isolation & FX Snapshots):** The system shall never aggregate balances across different currencies without an explicit `FXRateSnapshot` (rate, timestamp, provenance). Initial Phase 2–3 implementation may restrict each `TreasuryEntity` to a single currency while preserving multi-currency schema support.
- **FR-06 (Liquidity Partitioning & Exposure):** Go shall compute `available_balance`, enforce **Operating Liquidity** and **Emergency Reserves**, derive **Investable Capital**, and track **Counterparty Exposure** caps.

### 1.3 Intelligence, Verification & Governance
- **FR-07 (Forecasting & Risk):** Python shall generate forecasts with uncertainty intervals and risk evaluations (VaR, CVaR, Monte Carlo, stress tests, anomaly detection) from synthetic treasury data, reporting model provenance and calibration/backtest metadata.
- **FR-08 (Constrained Optimization & Unified Statuses):** `OptimizationRun` results shall use the unified status model (`COMPLETED`, `INFEASIBLE`, `SOLVER_FAILED`, `VERIFICATION_FAILED`, `CANCELLED`). Only `COMPLETED` runs may produce an `AllocationProposal`.
- **FR-09 (Financial Verification vs. Model Validation):** Go shall independently perform **deterministic financial verification** (capital conservation, liquidity floors, counterparty limits, decimal precision, valid allocations). Python shall perform **statistical/model validation** (backtests, calibration, input sanity, provenance). Go does not claim to prove statistical model correctness.
- **FR-10 (Approval Lifecycle & Re-Verification):** Proposals follow `DRAFT` → `PENDING_APPROVAL` → `APPROVED` / `REJECTED` / `EXPIRED`. Approval always revalidates live authoritative balances, encumbrances, counterparty exposure, and currently active policies inside an ACID transaction. Rejected proposals are terminal; resubmission creates a new proposal.
- **FR-11 (Non-Authoritative Language Explanations):** Optional LLM narratives may only summarize verified metrics and must never compute balances, risk metrics, or allocations.

---

## 2. Non-Functional & Reliability Requirements

### 2.1 Financial Precision & Safety
- **NFR-01 (No Binary Floats for Money):** Authoritative monetary storage and arithmetic shall never use binary floats (`float32`/`float64`/`REAL`/`DOUBLE PRECISION`).
- **NFR-02 (Explicit Rounding):** Rounding mode and residual cash assignment shall be deterministic and unit-tested.

### 2.2 Availability, Resilience & Observability
- **NFR-03 (Liveness vs. Readiness):** Go shall expose separate liveness (`/health`) and readiness (`/api/v1/health`) checks distinguishing PostgreSQL availability (critical) from Python intelligence availability (degraded compute capability).
- **NFR-04 (Graceful Degradation on Python Outage):** A Python outage shall **not** block basic treasury reads, ledger views, audit inspection, or proposal rejections; only intelligence computation endpoints degrade cleanly (`503` / `SOLVER_FAILED`).
- **NFR-05 (Timeouts, Correlation & Structured Errors):** Every Go → Python call shall enforce strict `context` timeouts, propagate `X-Correlation-ID`, and return structured error payloads without mutating treasury state on failure.
- **NFR-06 (Native Windows Runtime):** Go, Python, native PostgreSQL, and future TypeScript/Svelte UI shall run natively on Windows without Docker, Docker Compose, Kubernetes, or Java.

---

## 3. Decision Summary & Unresolved Questions

| Requirement Area | Classification | Notes |
| :--- | :--- | :--- |
| Multi-org user memberships & strict org-scoped isolation | **Confirmed** | Cross-tenant isolation required |
| Append-only ledger + atomic projections + no cross-currency float math | **Confirmed** | Core correctness invariants |
| Unified 5-status optimization model (`COMPLETED` only -> proposal) | **Confirmed** | Consistent across all layers |
| Separation of Go financial verification vs. Python model validation | **Confirmed** | Clear responsibility boundary |
| Maker-checker separation & single-currency-first rollout | **Proposed** | Needs confirmation before Phase 2/3 |

**Unresolved Questions:**
1. Which currency codes and minor-unit scales will initial synthetic datasets use?
2. What exact `expires_at` timeout and monetary/percentage approval thresholds should govern proposals?
3. Which intelligence calls should switch from synchronous HTTP to asynchronous `202 Accepted` job execution in Phase 5?
