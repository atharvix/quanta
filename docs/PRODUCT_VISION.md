# Product Vision — QUANTA

> **Document Status:** Phase 0 Draft — Pending Review  
> **Legend:** Every section distinguishes **Confirmed Decisions**, **Proposed Decisions**, and **Unresolved Questions**.

---

## 1. Vision Statement

**Confirmed Decision:**  
QUANTA is an enterprise-grade corporate treasury and capital optimization platform designed for **corporate treasury departments**. Its purpose is to give treasury teams a single, auditable system to monitor liquidity, forecast cash positions, quantify downside risk, evaluate capital allocation trade-offs, and produce explainable, constraint-checked recommendations for human approval.

## 2. Operating Philosophy: Semi-Autonomous Treasury Intelligence

**Confirmed Decision:**  
QUANTA operates strictly as a **semi-autonomous** system:

1. **Continuous Analytical Loop:** QUANTA monitors treasury balances, runs cash-flow forecasts, executes risk simulations, and computes candidate capital allocations.
2. **Human-in-the-Loop Gate:** Every consequential financial action (e.g., approving an allocation plan, releasing emergency reserves, modifying policy constraints, or marking an action for execution) requires explicit human approval.
3. **No Autonomous Execution:** QUANTA **never** initiates autonomous money transfers, wire instructions, or live market trades.

```
[Treasury Snapshot] --> [Forecast & Risk] --> [Constrained Optimization] --> [Recommendation]
        ^                                                                           |
        |                                                                           v
[Audit & Ledger Update] <-- [Human Approval Gate] <-- [Constraint & Policy Verification (Go)]
```

## 3. Target Customer & Personas

**Confirmed Decision:**  
Initial target customer: **Corporate treasury departments** managing multi-account liquidity, operating obligations, emergency buffers, and deployable capital.

**Proposed Personas:**
- **Treasury Analyst:** Configures scenarios, runs forecasts/simulations/optimizations, inspects constraint trade-offs, and submits allocation proposals.
- **Treasury Approver (Treasurer / CFO Delegate):** Reviews proposals, risk metrics, and constraint compliance; approves or rejects consequential actions.
- **System / Security Administrator:** Manages user identities, role assignments, and system configuration (without bypassing maker-checker financial controls).
- **Auditor (Read-Only):** Inspects immutable decision histories, approval records, model provenance, and balance snapshots.

## 4. Core Capabilities

**Confirmed Decisions:**
1. **Monitor:** Track authoritative account balances, available balances, operating liquidity, emergency reserves, and counterparty exposures.
2. **Forecast:** Produce probabilistic and deterministic cash-flow projections over configurable planning horizons using synthetic data initially.
3. **Simulate:** Run stress tests and tail-risk analyses (e.g., VaR, CVaR, Monte Carlo, anomaly detection) against liquidity and solvency thresholds.
4. **Analyze & Explain:** Quantify risk-return trade-offs and constraint bottlenecks; optionally provide natural-language summaries via non-authoritative language intelligence.
5. **Optimize:** Formulate constrained capital allocation problems and solve them classically while benchmarking quantum/quantum-inspired research formulations against classical baselines.
6. **Recommend & Govern:** Present ranked, constraint-verified recommendations through a formal approval workflow with end-to-end auditability.

## 5. Initial Data Strategy

**Confirmed Decision:**  
Initial development will use **realistic synthetic financial data** generated deterministically with explicit assumptions. No external banking APIs, market data vendors, specific fiat currencies, or real-world investment instruments are assumed or hardcoded in Phase 0.

## 6. Explicit Non-Goals

**Confirmed Decisions:**
- **No autonomous funds movement or trade execution.**
- **No invented regulatory compliance certifications** (compliance rules are modeled as explicit, configurable policy constraints rather than hardcoded jurisdictional statutes).
- **No LLM-computed financial math** (LLMs never compute balances, risk metrics, or allocations).
- **No assumed quantum advantage** (quantum methods are research candidates evaluated strictly against classical solvers).
- **No container/cluster infrastructure** (no Docker, Docker Compose, or Kubernetes).

---

## 7. Decision Summary

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Corporate treasury focus & semi-autonomous model | **Confirmed** | Human approval required for consequential actions |
| No autonomous transfers or trade execution | **Confirmed** | Recommendation & governance system only |
| Synthetic data for initial development | **Confirmed** | Explicit, reproducible synthetic generators in later phases |
| Four-persona RBAC split (Analyst, Approver, Admin, Auditor) | **Proposed** | Subject to refinement in Security Model review |

## 8. Unresolved Questions

1. Will initial synthetic scenarios model a single corporate entity or a parent treasury with multiple subsidiaries/cost centers?
2. What planning horizons (e.g., daily intraday, 30-day cash, 90-day working capital, 12-month strategic) are highest priority for Phase 1–2?
3. What specific approval thresholds (e.g., dual-approver above a capital percentage) should govern consequential recommendations?
