# ADR-001: Core Backend Language — Go

- **Status:** Confirmed
- **Date:** Phase 0 (2026-11-27)
- **Deciders:** QUANTA Architecture Team

---

## Context
QUANTA requires an authoritative core backend responsible for treasury account balances, exact monetary arithmetic, constraint verification, human approval state machines, RBAC enforcement, PostgreSQL ACID transactions, and orchestrating calls to a separate Python intelligence service.

## Decision
**Go** is selected as the core backend language for QUANTA.

## Rationale
1. **Explicit Error Handling & Predictable Control Flow:** Financial workflows benefit from explicit error propagation (`if err != nil`) rather than hidden exception paths, making failure handling around transactions, timeouts, and solver verification transparent.
2. **Strong Static Typing & Concurrency:** Go provides compile-time type safety, fast compilation, and lightweight concurrency (`context.Context` cancellation/timeouts) ideal for bounding upstream Python solver calls.
3. **Native Windows Deployment:** Go compiles to a single native Windows executable with zero JVM or container overhead.
4. **Clear Separation from Java/Spring Boot:** Explicitly avoids heavy enterprise JVM frameworks while maintaining clean modular structure.

## Trade-offs & Mitigations
- **Trade-off:** Go's standard library does not include built-in arbitrary-precision `decimal` types (unlike SQL `NUMERIC`).
  - **Mitigation:** Mandate an explicit decimal library or scaled integer minor-unit package in Phase 1; strictly ban `float64` in authoritative monetary structs via code review and tests.
- **Trade-off:** Go has a smaller numerical optimization/ML ecosystem than Python.
  - **Mitigation:** Delegate forecasting, risk simulation, and optimization to the dedicated Python intelligence service (see [ADR-003](ADR-003-PYTHON-INTELLIGENCE-BOUNDARY.md)).

## Alternatives Considered
- **Java / Spring Boot:** Explicitly rejected (heavy runtime, excluded by technology direction).
- **Python for everything:** Rejected for core ledger/governance due to dynamic typing and weaker compile-time guarantees on financial invariants.
