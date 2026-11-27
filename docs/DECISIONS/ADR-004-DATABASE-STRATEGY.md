# ADR-004: Native PostgreSQL Database Strategy

- **Status:** Confirmed (Native PostgreSQL) / Proposed (Specific Scale & Role Hardening)
- **Date:** Phase 0 (2026-11-27)
- **Deciders:** QUANTA Architecture Team

---

## Context
QUANTA requires an ACID-compliant relational database capable of exact fixed-point decimal arithmetic (`NUMERIC`), strict check constraints, JSONB storage for versioned problem/response snapshots, and append-only auditability—installed cleanly in a Windows + PowerShell environment without Docker.

## Decision
Use **PostgreSQL installed natively on Windows** as the single authoritative database, accessed exclusively by the Go modular monolith.

## Rationale
1. **Exact Decimal Arithmetic:** PostgreSQL `NUMERIC(p, s)` stores monetary quantities without binary floating-point rounding error.
2. **ACID Transactions & Row Locking:** Native support for `SELECT ... FOR UPDATE`, check constraints (`encumbered_balance <= ledger_balance`), and transactional audit logging.
3. **Zero Container Dependency:** Native Windows PostgreSQL runs directly as a Windows service (`localhost:5432`), aligning with the strict exclusion of Docker, Docker Compose, and Kubernetes.
4. **Rich Relational + JSONB Hybrid:** Structured relational tables enforce financial invariants while `JSONB` stores reproducible optimization snapshots and diagnostic metadata.

## Trade-offs & Mitigations
- **Trade-off:** Requires local PostgreSQL installation and initial database/role setup on developer Windows machines.
  - **Mitigation:** Document native Windows setup clearly and provide deterministic bootstrap SQL in Phase 1.
- **Trade-off:** Single-database bottleneck if write volume grows massively.
  - **Mitigation:** Corporate treasury ledger and approval workloads are high-value, moderate-TPS transactions well within a single PostgreSQL instance.

## Alternatives Considered
- **Dockerized PostgreSQL:** Explicitly rejected (Docker/Compose excluded).
- **SQLite:** Rejected due to weaker concurrent write locking, lack of native fixed-point `NUMERIC` enforcement semantics at PostgreSQL's level, and less realistic enterprise parity.
- **Separate databases per module:** Rejected to preserve single-transaction ACID integrity (see [ADR-002](ADR-002-MODULAR-MONOLITH.md)).
