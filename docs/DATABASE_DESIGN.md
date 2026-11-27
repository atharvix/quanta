# Database Design — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Database Engine:** Native PostgreSQL on Windows (No Docker / No Containers).

---

## 1. Design Principles

1. **Confirmed Decision:** PostgreSQL is owned exclusively by the Go modular monolith. Python never connects directly to PostgreSQL to read or mutate financial state.
2. **Confirmed Decision:** Multi-tenancy is anchored at `organizations`. Every tenant-owned table carries `organization_id` (directly or via FK chain) and queries enforce tenant isolation.
3. **Confirmed Decision:** Monetary values use fixed-point `NUMERIC`—never binary floating-point (`REAL`, `DOUBLE PRECISION`, `FLOAT`).
4. **Confirmed Decision:** `ledger_entries`, `approval_decisions`, `audit_events`, and `fx_rate_snapshots` are **append-only**; historical entries are never destructively updated or deleted.

---

## 2. Logical Entity-Relationship Diagram

```
organizations (id, name, status, created_at)
  |
  +---> organization_memberships (id, organization_id, user_id, role, status) <--- users (id, email, ...)
  |
  +---> fx_rate_snapshots (id, organization_id, from_currency, to_currency, rate, effective_at, provenance)
  |
  +---> treasury_entities (id, organization_id, name, base_currency, is_synthetic)
          |
          +---> liquidity_policies (id, treasury_entity_id, version, min_operating_liquidity, min_emergency_reserve)
          +---> counterparties (id, treasury_entity_id, name, max_exposure_limit, current_exposure)
          +---> financial_constraints (id, treasury_entity_id, version, constraint_type, severity, parameters)
          |
          +---> treasury_accounts (id, treasury_entity_id, currency_code, ledger_balance, encumbered_balance, version)
          |       |
          |       +---> ledger_entries (id, account_id, entry_type, delta_amount, balance_after, reversed_entry_id)
          |
          +---> optimization_runs (id, treasury_entity_id, status, problem_snapshot, raw_response, fx_snapshot_ids)
                  |
                  +---> allocation_proposals (id, run_id, status, snapshot_versions, expires_at, supersedes_proposal_id)
                          |
                          +---> approval_decisions (id, proposal_id, approver_id, decision, rationale, live_snapshot)
```

---

## 3. Conceptual Table Specifications

> Note: No SQL migrations are created in Phase 0. Types below specify the Phase 2 target schema.

### 3.1 Multi-Tenancy & Identity
- `organizations`: `id (UUID PK)`, `name`, `status ('ACTIVE' | 'SUSPENDED')`, `created_at (TIMESTAMPTZ)`.
- `users`: `id (UUID PK)`, `email (UNIQUE)`, `password_hash`, `full_name`, `is_active (BOOLEAN)`, `created_at (TIMESTAMPTZ)`.
- `organization_memberships`: `id (UUID PK)`, `organization_id (UUID FK)`, `user_id (UUID FK)`, `role ('ADMIN' | 'TREASURY_APPROVER' | 'TREASURY_ANALYST' | 'VIEWER')`, `status ('ACTIVE' | 'SUSPENDED')`, `UNIQUE(organization_id, user_id)`.

### 3.2 Treasury, Currency & FX Snapshots
- `treasury_entities`: `id (UUID PK)`, `organization_id (UUID FK)`, `name`, `base_currency (VARCHAR(10))`, `is_synthetic (BOOLEAN)`, `created_at (TIMESTAMPTZ)`.
- `fx_rate_snapshots` *(append-only)*: `id (UUID PK)`, `organization_id (UUID FK)`, `from_currency (VARCHAR(10))`, `to_currency (VARCHAR(10))`, `rate NUMERIC(20, 8) CHECK (rate > 0)`, `effective_at (TIMESTAMPTZ)`, `source_provenance (VARCHAR(120))`, `is_synthetic (BOOLEAN)`, `created_at (TIMESTAMPTZ)`.
- `liquidity_policies`: `id (UUID PK)`, `treasury_entity_id (UUID FK)`, `version (BIGINT)`, `min_operating_liquidity NUMERIC(20, 4) CHECK (>= 0)`, `min_emergency_reserve NUMERIC(20, 4) CHECK (>= 0)`, `is_active (BOOLEAN)`, `created_by (UUID FK)`, `created_at (TIMESTAMPTZ)`.

### 3.3 Accounts & Append-Only Ledger Projections
- `treasury_accounts`: `id (UUID PK)`, `treasury_entity_id (UUID FK)`, `account_code`, `currency_code (VARCHAR(10))`, `ledger_balance NUMERIC(20, 4)`, `encumbered_balance NUMERIC(20, 4)`, `version (BIGINT)`, `updated_at (TIMESTAMPTZ)`.
  - **Check Constraints:** `ledger_balance >= 0`, `encumbered_balance >= 0`, `encumbered_balance <= ledger_balance`.
- `ledger_entries` *(append-only)*: `id (UUID PK)`, `account_id (UUID FK)`, `entry_type ('CREDIT' | 'DEBIT' | 'REVERSAL' | 'HOLD_PLACED' | 'HOLD_RELEASED' | 'HOLD_SETTLED')`, `currency_code (VARCHAR(10))`, `delta_amount NUMERIC(20, 4)`, `balance_after NUMERIC(20, 4)`, `encumbered_after NUMERIC(20, 4)`, `reversed_entry_id (UUID NULLABLE FK)`, `reference_type`, `reference_id (UUID)`, `correlation_id`, `created_at (TIMESTAMPTZ)`.
  - **Projection Coupling:** Every `INSERT` into `ledger_entries` occurs in the same PostgreSQL ACID transaction as the corresponding `UPDATE` to `treasury_accounts.ledger_balance`, `encumbered_balance`, and `version = version + 1`.

### 3.4 Counterparties, Constraints, Optimization & Governance
- `counterparties`: `id (UUID PK)`, `treasury_entity_id (UUID FK)`, `name`, `currency_code (VARCHAR(10))`, `max_exposure_limit NUMERIC(20, 4) CHECK (>= 0)`, `current_exposure NUMERIC(20, 4) CHECK (>= 0)`, `version (BIGINT)`.
- `financial_constraints`: `id (UUID PK)`, `treasury_entity_id (UUID FK)`, `version (BIGINT)`, `constraint_type`, `severity ('HARD' | 'SOFT')`, `parameters (JSONB)`, `is_active (BOOLEAN)`.
- `optimization_runs`: `id (UUID PK)`, `treasury_entity_id (UUID FK)`, `requested_by (UUID FK)`, `correlation_id`, `solver_category` (`'classical' | 'quantum_research' | 'hybrid_benchmark'`), `status` (`'COMPLETED' | 'INFEASIBLE' | 'SOLVER_FAILED' | 'VERIFICATION_FAILED' | 'CANCELLED'`), `feasible (BOOLEAN)`, `problem_snapshot (JSONB)`, `raw_response (JSONB)`, `fx_snapshot_ids (JSONB)`, `runtime_ms (INTEGER)`, `created_at (TIMESTAMPTZ)`.
- `allocation_proposals`: `id (UUID PK)`, `optimization_run_id (UUID FK)`, `treasury_entity_id (UUID FK)`, `supersedes_proposal_id (UUID NULLABLE FK)`, `status` (`'DRAFT' | 'PENDING_APPROVAL' | 'APPROVED' | 'REJECTED' | 'EXPIRED'`), `investable_capital NUMERIC(20, 4)`, `total_allocated NUMERIC(20, 4)`, `residual_cash NUMERIC(20, 4)`, `verified_allocations (JSONB)`, `snapshot_account_versions (JSONB)`, `snapshot_policy_version (BIGINT)`, `submitted_by (UUID FK)`, `submitted_at (TIMESTAMPTZ)`, `expires_at (TIMESTAMPTZ)`, `version (BIGINT)`.
- `approval_decisions` *(append-only)*: `id (UUID PK)`, `proposal_id (UUID FK)`, `approver_id (UUID FK)`, `decision` (`'APPROVE' | 'REJECT'`), `rationale (TEXT)`, `live_balance_snapshot (JSONB)`, `decided_at (TIMESTAMPTZ)`.
- `audit_events` *(append-only)*: `id (UUID PK)`, `organization_id (UUID NULLABLE FK)`, `actor_id (UUID NULLABLE)`, `actor_role`, `action`, `entity_type`, `entity_id`, `correlation_id`, `metadata (JSONB)`, `created_at (TIMESTAMPTZ)`.

---

## 4. Transactional Integrity & Privilege Hardening (Proposed)

- **Atomic Locking:** Proposals and approvals lock affected `treasury_accounts` rows (`SELECT ... FOR UPDATE`) and verify optimistic `version` tokens before committing projections or transitions.
- **Database Role Hardening:** Revoke `UPDATE` and `DELETE` on `ledger_entries`, `approval_decisions`, `audit_events`, and `fx_rate_snapshots` for the Go application DB user.

---

## 5. Decision Summary & Unresolved Questions

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Multi-tenant `organizations` + `organization_memberships` schema | **Confirmed** | Supports multi-org users |
| Native Windows PostgreSQL owned solely by Go | **Confirmed** | See ADR-004 |
| Append-only `ledger_entries` + atomic account projections | **Confirmed** | Reversals only; no destructive updates |
| Unified 5-status `optimization_runs` check constraint | **Confirmed** | Matches domain & API contracts |
| `NUMERIC(20, 4)` balances, `NUMERIC(20, 8)` FX, role hardening | **Proposed** | Finalize before Phase 2 |

**Unresolved Questions:**
1. Should `treasury_entities` enforce a single currency across all child accounts initially via DB trigger/constraint, or in Go domain validation?
2. Should `audit_events` partition by month once retention grows in later enterprise hardening (Phase 7)?
3. Which Go SQL migration tool (embedded SQL runner vs. `goose`/`golang-migrate`) will be adopted in Phase 2?
