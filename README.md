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

## Phase 0 Status

This repository is at **Phase 0 (Architecture and Requirements — Revision 1)** and contains Markdown documentation only. No application code, SQL migrations, dependencies, or models have been created or installed. Phase 1 begins only after explicit review and approval.
