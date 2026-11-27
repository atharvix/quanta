# Security & Governance Model — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Core Principle:** Multi-tenant isolation, least privilege, maker-checker governance, and live approval-time financial re-verification.

---

## 1. Trust Boundaries & Tenant Isolation

```
[Authenticated User (Global Identity)]
        |  (Active Organization Context: org_id)
        v
+--------------------------------------------------------+
| Go Core (Trusted Authority)                            |
|  1. Verify OrganizationMembership(user_id, org_id)      |
|  2. Enforce Org-Scoped RBAC Role                       |
|  3. Enforce Maker-Checker & Live Financial Invariants  |
+--------------------------------------------------------+
     |                                    |
     | Trusted ACID DB                    | Untrusted Compute Boundary
     v                                    v
[Native PostgreSQL]               [Python Intelligence Service]
(Org-Scoped Queries)              (Stateless, No DB Access, Advisory Only)
```

- **Confirmed Decision (Tenant-Scoped Authorization):** Every request targeting a `TreasuryEntity`, `TreasuryAccount`, `OptimizationRun`, `AllocationProposal`, or `AuditEvent` must verify that the authenticated `User` holds an `ACTIVE` `OrganizationMembership` in the target resource's `Organization`. A user belonging to multiple organizations cannot cross-read or cross-mutate data between them.

---

## 2. Authentication & Session Model

- **Proposed Decision:** Global user accounts (`users`) authenticate via password hashing (`argon2id` or `bcrypt`) and receive signed tokens/sessions. Requests carry an active `organization_id` context (selected via header, path, or org-scoped token exchange) validated on every request against `organization_memberships`.
- **Confirmed Decision:** Credentials, tokens, and secrets are never stored in plain text or logged in `audit_events`.

---

## 3. Organization-Scoped RBAC Matrix (Proposed)

| Capability (Within Active `Organization`) | `Viewer / Auditor` | `Treasury_Analyst` | `Treasury_Approver` | `Admin` |
| :--- | :---: | :---: | :---: | :---: |
| View organization treasuries, positions, runs, proposals | Yes | Yes | Yes | Yes |
| View organization `audit_events` | Yes | Scoped | Yes | Yes |
| Trigger forecasts, risk simulations, optimizations | No | Yes | Yes | No |
| Create/submit `AllocationProposal` (`DRAFT` → `PENDING_APPROVAL`) | No | Yes *(Maker)* | No | No |
| Approve / Reject `PENDING_APPROVAL` proposal | No | No | Yes *(Checker)* | No |
| Propose / approve policy & constraint changes | No | Propose | Approve | No |
| Manage `OrganizationMembership` & user roles in org | No | No | No | Yes |

*(A user may hold `Treasury_Analyst` in Organization A and `Viewer` in Organization B; permissions are always resolved from `OrganizationMembership` for the active organization.)*

---

## 4. Maker-Checker & Approval-Time Verification

- **Proposed Decision:** Enforce **Four-Eyes Separation** (`proposal.submitted_by != decision.approver_id`).
- **Confirmed Decision (Approval-Time Re-Verification):** Every approval executes in a PostgreSQL transaction that:
  1. Verifies active organization membership and approver role.
  2. Verifies proposal status is `PENDING_APPROVAL` and `now() < expires_at`.
  3. Locks affected `treasury_accounts` and checks whether underlying balances, policies, constraints, or counterparty limits changed since submission.
  4. Re-validates all hard financial invariants against **current authoritative state and active policies**—blocking stale or non-compliant approvals cleanly.
  5. Writes immutable `approval_decisions` and `audit_events` atomically.
- **Unresolved Business Decisions:** Exact monetary/percentage thresholds requiring dual approvers, proposal expiry duration, and whether local development environments allow a flagged maker-checker bypass.

---

## 5. Auditability & Tamper-Evidence

- **Confirmed Decision:** Append-only `audit_events` record `organization_id`, `actor_id`, `actor_role`, `action`, `entity_type`, `entity_id`, `correlation_id`, `metadata`, and UTC timestamp inside the same DB transaction as financial mutations.
- **Proposed Decision:** Revoke `UPDATE` and `DELETE` on `audit_events`, `ledger_entries`, `approval_decisions`, and `fx_rate_snapshots` for the Go application DB user.

---

## 6. Unresolved Questions

1. Should active `organization_id` be passed via an HTTP path prefix (`/api/v1/orgs/{orgId}/...`), a header (`X-Organization-ID`), or an org-scoped JWT claim?
2. What internal authentication mechanism (e.g., localhost shared HMAC secret) should protect Go ↔ Python internal endpoints?
3. Should local development seed two distinct users (`analyst` + `approver`) by default to exercise maker-checker without bypasses?
