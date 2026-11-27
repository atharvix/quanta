# Domain Model & Financial Correctness Specification — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Purpose:** Define multi-tenant organization hierarchy, authoritative treasury entities, currency/FX rules, append-only ledger projections, unified optimization statuses, and approval lifecycles.

---

## 1. Multi-Tenancy & Organization Model

QUANTA models multi-tenancy explicitly so organizations remain strictly isolated while allowing users to collaborate across multiple organizations without assuming a 1:1 user-to-organization binding.

```
+------------------+       1..*       +--------------------------+
|       User       |<---------------->|  OrganizationMembership  |
|  (Global Auth)   |                  |  (org_id, user_id, role) |
+------------------+                  +--------------------------+
                                                   | *..1
                                                   v
                                      +--------------------------+
                                      |       Organization       |
                                      |    (Tenant Boundary)     |
                                      +--------------------------+
                                                   | 1..*
                                                   v
                                      +--------------------------+
                                      |      TreasuryEntity      |
                                      |  (base_currency, scope)  |
                                      +--------------------------+
                                                   | 1..*
                                                   v
                                      +--------------------------+
                                      |     TreasuryAccount      |
                                      | (currency_code, balance) |
                                      +--------------------------+
```

### Entity Distinction (Proposed Organizational Hierarchy)
1. **Organization:** The top-level tenant boundary. Owns `TreasuryEntity` records, policies, counterparties, audit scope, and membership rosters.
2. **User:** A global authentication principal (`id`, `email`, `full_name`, `is_active`). A single `User` may belong to **zero, one, or multiple** organizations.
3. **OrganizationMembership:** The explicit association between a `User` and an `Organization`, carrying an organization-scoped RBAC `role` (`Admin`, `Treasury_Approver`, `Treasury_Analyst`, `Viewer/Auditor`) and `status` (`ACTIVE`, `SUSPENDED`).
4. **TreasuryEntity:** A corporate treasury unit belonging strictly to **one** `Organization`, defining a `base_currency` for reporting, `LiquidityPolicy`, `FinancialConstraint`s, and `Counterparty` limits.
5. **TreasuryAccount:** A cash or holding account belonging strictly to **one** `TreasuryEntity` (and transitively one `Organization`), denominated in an explicit `currency_code`.

- **Confirmed Decision:** All treasury data, optimization runs, proposals, and audit queries are strictly **tenant-scoped** to an `Organization`. A user can only act on a `TreasuryEntity` if they hold an `ACTIVE` `OrganizationMembership` in that entity's owning `Organization` with the required role.

---

## 2. Currency Model & Financial Aggregation

**Confirmed Invariant:** A treasury position **must never** aggregate amounts across different currencies without an explicit conversion mechanism.

### 2.1 Currency Concepts
- **Account Currency (`TreasuryAccount.currency_code`):** The native currency in which an individual account's `ledger_balance`, `encumbered_balance`, and `ledger_entries` are recorded.
- **Treasury Base Currency (`TreasuryEntity.base_currency`):** The reporting currency used for entity-level consolidated views and cross-account capital optimization.
- **Original-Currency Balances:** Exact monetary balances stored and maintained in each account's `currency_code`.
- **Reporting-Currency Balances:** Derived amounts converted from `currency_code` into `TreasuryEntity.base_currency` using an explicit `FXRateSnapshot`.
- **FXRateSnapshot:** An immutable record capturing `from_currency`, `to_currency`, `rate`, `effective_at` (timestamp), `source_provenance` (e.g., synthetic snapshot identifier), and `is_synthetic`.

### 2.2 Conversion Rules & Reproducibility (Proposed)
- **Conversion Precision & Rounding:** Multiply original-currency `NUMERIC` balance by `FXRateSnapshot.rate` (`NUMERIC(20, 8)` proposed), then quantize to reporting precision using **Banker's Rounding (Round Half to Even)**, recording any conversion rounding residual explicitly.
- **Historical Reproducibility:** Every consolidated position, `OptimizationRun`, `AllocationProposal`, and `ApprovalDecision` involving cross-currency aggregation must reference the exact `fx_rate_snapshot_id`(s) used so historical reports reproduce deterministically.
- **Phased Adoption Recommendation (Proposed):** Start Phase 2–3 with a **single-currency-per-treasury** enforcement rule (all accounts in a `TreasuryEntity` share `base_currency`, and Go rejects cross-currency aggregation), while preserving `currency_code`, `base_currency`, and the `FXRateSnapshot` model so multi-currency support can be enabled cleanly in a later phase without schema rework.
- *(No FX vendors or specific supported currencies are assumed in Phase 0.)*

---

## 3. Ledger Model & Account Balance Projections

QUANTA separates immutable financial history from fast-read account projections inside Go-managed PostgreSQL ACID transactions:

- **`ledger_entries` (Authoritative Append-Only Event Log):** Records every financial movement (`CREDIT`, `DEBIT`, `REVERSAL`, `HOLD_PLACED`, `HOLD_RELEASED`, `HOLD_SETTLED`) with `delta_amount`, `currency_code`, `balance_after`, `correlation_id`, and optional `reversed_entry_id`.
- **`treasury_accounts.ledger_balance` (Materialized Projection):** Reflects the exact sum of committed `CREDIT`, `DEBIT`, `REVERSAL`, and `HOLD_SETTLED` ledger entries for that account.
- **`treasury_accounts.encumbered_balance` (Materialized Projection):** Reflects the net sum of active holds (`HOLD_PLACED` minus `HOLD_RELEASED` and `HOLD_SETTLED`).
- **`available_balance` (Derived Invariant):** `ledger_balance - encumbered_balance`. Always satisfies `0 <= encumbered_balance <= ledger_balance`.

### 3.1 Atomic Updates, Reversals, Holds & Reconciliation
1. **Atomic Projection Update:** Within a single PostgreSQL transaction, Go locks the `treasury_accounts` row (`SELECT ... FOR UPDATE` / optimistic `version` check), inserts the `ledger_entries` row(s), updates `ledger_balance` / `encumbered_balance`, and asserts `0 <= encumbered_balance <= ledger_balance`.
2. **No Destructive Modification:** Historical `ledger_entries` are **never** updated or deleted. Errors are corrected exclusively by appending a `REVERSAL` entry referencing `reversed_entry_id`.
3. **Hold Lifecycle:** Submitting/approving an allocation or earmark places a hold (`HOLD_PLACED`, increasing `encumbered_balance`); cancelling or completing releases (`HOLD_RELEASED`) or settles (`HOLD_SETTLED`, reducing both `encumbered_balance` and `ledger_balance`).
4. **Reconciliation Behavior:** Go provides a deterministic reconciliation check comparing `treasury_accounts.ledger_balance` and `encumbered_balance` against `SUM(ledger_entries)` by entry type. Any mismatch triggers a critical `AuditEvent` and blocks approvals on that account until resolved.

---

## 4. Monetary Precision, Rounding & Liquidity Partitioning

### 4.1 Precision & Rounding Rules
- **Confirmed Decision:** Ordinary floating-point types (`float32`, `float64`, `REAL`, `DOUBLE PRECISION`) **must never** be used for authoritative monetary storage or ledger arithmetic.
- **Proposed Decision:** Store monetary balances as `NUMERIC(20, 4)` and rates/ratios as `NUMERIC(12, 6)` in PostgreSQL; use exact decimal arithmetic in Go; serialize money as decimal strings across Go ↔ Python JSON; apply **Banker's Rounding (Round Half to Even)** when quantizing continuous weights, assigning any sub-unit residual explicitly to `unallocated_cash` so capital conservation holds to the last digit.

### 4.2 Liquidity Partitioning & Counterparty Exposure

| Concept | Formal Definition | Testable Invariant |
| :--- | :--- | :--- |
| **Account Balance (`ledger_balance`)** | Materialized ledger total in account currency. | `ledger_balance >= 0` |
| **Encumbered Balance (`encumbered_balance`)** | Active holds/reservations in account currency. | `0 <= encumbered_balance <= ledger_balance` |
| **Available Balance (`available_balance`)** | `ledger_balance - encumbered_balance`. | `available_balance >= 0` |
| **Operating Liquidity ($L_{\text{op}}$)** | Ring-fenced liquid funds for near-term operations. | `L_op >= policy.min_operating_liquidity` |
| **Emergency Reserves ($R_{\text{em}}$)** | Crisis buffer excluded from deployment unless explicitly released by approval. | `R_em >= policy.min_emergency_reserve` |
| **Investable Capital ($C_{\text{inv}}$)** | `max(0, total_available_in_base_currency - L_op - R_em)` | `sum(allocations) + unallocated_cash == C_inv` |
| **Counterparty Exposure** | Aggregated exposure across active holdings and pending approved commitments per `Counterparty`. | `exposure_after_proposal <= counterparty.max_exposure_limit` |

---

## 5. Unified Optimization Status Model

All QUANTA components (`OptimizationRun`, PostgreSQL schema, Go verifier, and REST APIs) use **one canonical status enum**:

| Status | Computation Completed? | Allocation Feasible? | May Create `AllocationProposal`? | Meaning |
| :--- | :---: | :---: | :---: | :--- |
| **`COMPLETED`** | Yes | Yes (`feasible = true`) | **Yes** | Python solver succeeded **and** Go independent financial verification passed all invariants. |
| **`INFEASIBLE`** | Yes | No (`feasible = false`) | **No** | No allocation satisfies all hard constraints simultaneously; conflicting constraints are reported. |
| **`SOLVER_FAILED`** | No | No | **No** | Solver timed out, crashed, or failed to converge numerically. |
| **`VERIFICATION_FAILED`** | Yes (in Python) | No | **No** | Python returned a candidate, but Go financial verification rejected it (e.g., conservation, precision, or constraint violation). |
| **`CANCELLED`** | Stopped early | No | **No** | Caller or system cancelled execution before completion. |

**Confirmed Rule:** An `AllocationProposal` may be created **only** from an `OptimizationRun` with status **`COMPLETED`**.

---

## 6. Proposal & Approval Lifecycle

```
 [OptimizationRun: COMPLETED]
              |
              v
          +-------+   Analyst Submits     +------------------+
          | DRAFT |---------------------->| PENDING_APPROVAL |
          +-------+   (Capture Snapshots) +------------------+
                                            |      |       |
                   Approver Approves        |      |       | Expiry / Balance Drift
                   (Re-verify Live State)   v      |       v
                                      +----------+ |  +---------+
                                      | APPROVED | |  | EXPIRED |
                                      +----------+ |  +---------+
                                                   v
                                              +----------+
                                              | REJECTED | (Terminal; new proposal
                                              +----------+  required to resubmit)
```

### Lifecycle Governance Rules
1. **Balance & Policy Snapshot Versioning:** Upon transition to `PENDING_APPROVAL`, the proposal records `account_versions` (and `policy_version` / `fx_rate_snapshot_id`s).
2. **Stale Proposal Detection:** If any referenced account's `version` changes or `LiquidityPolicy` / `FinancialConstraint` / `Counterparty` limits change after submission, the proposal is marked stale and **must** pass live re-verification before approval can succeed.
3. **Policy Changes After Submission:** Approval-time verification always evaluates against **currently active** policies and constraints—never stale submission-time rules. If a new constraint invalidates the proposal, approval is blocked cleanly.
4. **Expiry Rules:** Proposals carry an `expires_at` timestamp; expired proposals transition to `EXPIRED` and cannot be approved.
5. **Rejection & Resubmission:** `REJECTED` is terminal for that `AllocationProposal` (preserving its `ApprovalDecision` and rationale). Resubmission creates a **new** `AllocationProposal` linked via optional `supersedes_proposal_id` (after re-running or re-verifying against current state).
6. **Maker-Checker (Proposed):** `submitted_by != approver_id` enforced on approval.
7. **Approval-Time Re-Verification (Confirmed):** Approval executes inside a PostgreSQL transaction that locks relevant accounts, re-verifies `available_balance`, liquidity floors, and counterparty caps against live state, records `ApprovalDecision` + `AuditEvent`, and never triggers autonomous money movement.

---

## 7. Decision Summary & Unresolved Questions

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Multi-tenant `Organization` boundary & multi-org `User` memberships | **Confirmed** | Tenant-scoped authorization required |
| No cross-currency aggregation without explicit FX snapshot | **Confirmed** | Single-currency-first implementation recommended |
| Append-only `ledger_entries` with atomic balance projections | **Confirmed** | No destructive edits; reversals only |
| Canonical 5-state `OptimizationRun` model (`COMPLETED` only -> proposal) | **Confirmed** | Distinguishes computation vs. feasibility |
| Approval-time live re-verification of balances & policies | **Confirmed** | Prevents stale-state or post-policy-change approvals |
| `NUMERIC(20, 4)`, Banker's Rounding, single-currency Phase 2 start | **Proposed** | To be finalized before Phase 2 DDL |

**Unresolved Questions:**
1. Can a user hold different roles across different organizations, and how is the active `organization_id` selected in API requests (path parameter, header, or scoped token)?
2. Which specific currency codes and minor-unit scales will synthetic scenarios support initially, and in which phase should multi-currency `FXRateSnapshot` conversion be activated?
3. What exact approval expiry duration (`expires_at`) and monetary/percentage approval thresholds (e.g., dual approval) should apply?
4. When underlying account balances change after submission, should minor inflows keep a proposal valid on re-verification or automatically invalidate it?
