# ADR-002: Go Modular Monolith Architecture

- **Status:** Confirmed
- **Date:** Phase 0 (2026-11-27)
- **Deciders:** QUANTA Architecture Team

---

## Context
Treasury operations involve tightly coupled invariants: approving an allocation proposal requires locking balances, verifying operating liquidity and emergency reserve floors, checking counterparty exposure limits, updating proposal state, and recording an immutable audit event.

## Decision
The Go core backend will be structured as a **Modular Monolith** rather than a suite of microservices.

## Rationale
1. **Single ACID Transaction Boundary:** Balance checks, encumbrance updates, approval state transitions, and audit log insertions commit atomically in PostgreSQL without distributed two-phase commit or saga complexity.
2. **Strong Internal Boundaries Without Network Overhead:** Modules (`treasury`, `ledger`, `constraints`, `exposure`, `governance`, `audit`, `auth`) interact via clean Go interfaces in-process.
3. **Operational Simplicity on Windows:** One Go process + one Python process + native PostgreSQL—no service mesh, message broker, or container orchestration required.

## Trade-offs & Mitigations
- **Trade-off:** Discipline is required so internal modules do not reach directly into another module's internal database tables.
  - **Mitigation:** Enforce module boundaries through package structure and service interfaces; cross-module writes flow through explicit module methods or transactional unit-of-work interfaces.
- **Trade-off:** Core backend scales as a single unit.
  - **Mitigation:** Heavy numerical/ML workloads are already isolated in the separate Python intelligence service.

## Alternatives Considered
- **Microservices per domain (Ledger service, Approval service, Audit service):** Rejected as premature microservices creating unnecessary network latency, distributed failure modes, and operational complexity.
