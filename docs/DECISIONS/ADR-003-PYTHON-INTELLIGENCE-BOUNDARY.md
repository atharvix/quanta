# ADR-003: Separate Python Intelligence Boundary

- **Status:** Confirmed (Boundary & Responsibility Split) / Proposed (Localhost HTTP+JSON Transport)
- **Date:** Phase 0 (Revision 1 — 2026-11-27)
- **Deciders:** QUANTA Architecture Team

---

## Context
QUANTA requires statistical forecasting, Monte Carlo/VaR/CVaR risk simulation, mathematical optimization, quantum/quantum-inspired benchmarking, and optional non-authoritative language explanations. Python is the natural ecosystem for these workloads, while Go owns authoritative financial state.

## Decision
Isolate intelligence and optimization in a **separate, stateless Python service** communicating with Go over an **explicit, versioned contract** (`contract_version`, correlation IDs, strict timeouts, and decimal strings for monetary values).

## Boundary & Validation Split (Confirmed)
1. **Python is Non-Authoritative:** Python never owns system-of-record state and never writes directly to PostgreSQL financial tables.
2. **Two Distinct Validation Layers:**
   - **Python performs Statistical & Model Validation:** Backtesting, calibration, input data sanity checks, model provenance, and model-version tracking.
   - **Go performs Financial Verification:** Deterministic verification of capital conservation, liquidity floors, counterparty limits, decimal precision, and allocation validity. Go **does not** independently prove the statistical correctness of Python's predictive models.
3. **LLMs Are Strictly Advisory:** Language models never calculate balances, risk metrics, or allocations.
4. **Fail-Safe Degradation:** If Python is unavailable, times out, or fails, Go core treasury reads and non-compute governance continue working; compute endpoints fail cleanly without mutating financial state.

## Trade-offs & Mitigations
- **Trade-off:** Serialization overhead and process boundary between Go and Python.
  - **Mitigation:** Self-contained problem snapshots over `127.0.0.1` with explicit decimal string encoding and bounded timeouts.
- **Trade-off:** Separate Go and Python processes on Windows.
  - **Mitigation:** Native PowerShell startup/stop automation in later implementation phases (no Docker required).

## Alternatives Considered
- **Embedding Python via CGo:** Rejected due to GIL/memory fragility and Windows build complexity.
- **Implementing all optimization/ML in Go:** Rejected because mature forecasting, convex optimization, and quantum research libraries live in Python.
