# QUANTA — Cross-Module Integration Contract

This document defines the strict data exchange contracts between:
- **Atharv's AI Engine** (`services/ai-engine`, port 8000)
- **Parv's Optimization Engine** (`services/optimization-engine`, port 8001)
- **Aryan's Go Backend** (`apps/backend`, port 8080)

---

## 1. Unified `FinancialProblem` Payload

All solvers (Search, CSP, Classical LP, QUBO, QAOA) accept the same canonical input contract defined in `shared/schemas/financial_problem.json`:

```json
{
  "capital": 100.0,
  "step_size": 5.0,
  "assets": [
    {
      "id": "gov_bond",
      "name": "Government Bonds",
      "asset_type": "gov_bond",
      "cost": 0.001,
      "expected_return": 0.07,
      "risk": 0.05,
      "liquidity": 0.95,
      "volatility": 0.03
    }
  ],
  "liabilities": [
    {
      "id": "loan_1",
      "name": "Senior Term Debt",
      "principal": 15.0,
      "interest_rate": 0.085,
      "maturity_months": 12,
      "priority": 1,
      "is_mandatory_reserve": true
    }
  ],
  "constraints": [
    {
      "id": "c_min_liq",
      "constraint_type": "minimum_liquidity",
      "description": "Maintain minimum 20% liquidity",
      "parameters": { "threshold": 0.20 }
    }
  ],
  "risk_tolerance": 0.50,
  "minimum_liquidity": 0.20,
  "objective": "maximize_risk_adjusted_return",
  "market_state": "normal"
}
```

---

## 2. AI Engine REST API Endpoints (Atharv — Port 8000)

### 2.1 Strategy Synthesis
- **Endpoint**: `POST /api/v1/strategy/generate`
- **Request Body**:
  ```json
  {
    "problem": { ... },
    "algorithm": "astar"  // "bfs", "dfs", "generate_and_test", "hill_climbing", "best_first", "astar"
  }
  ```
- **Response**:
  ```json
  {
    "strategy": {
      "actions": [...],
      "total_allocated": 85.0,
      "expected_return": 0.0815,
      "risk_score": 0.1189,
      "liquidity_ratio": 0.92,
      "objective_value": 0.2854,
      "is_compliant": true,
      "explanation": "Allocated via A* Search with expected return 8.15% and risk 0.1189.",
      "metadata": { ... }
    },
    "search_result": { ... },
    "risk_breakdown": { ... },
    "constraint_audit": [ ... ]
  }
  ```

### 2.2 Algorithmic Benchmarking
- **Endpoint**: `POST /api/v1/search/compare`
- **Request Body**: `FinancialProblem`
- **Response**: Comparative results object executing BFS, DFS, Generate-and-Test, Hill Climbing, Best First, and A* side-by-side.

### 2.3 Multi-Agent Council Deliberation
- **Endpoint**: `POST /api/v1/agents/evaluate`
- **Request Body**: `FinancialProblem`
- **Response**: Deliberation summary with individual scores and opinions from `TreasuryAgent`, `RiskAgent`, `ComplianceAgent`, and `DecisionAgent`.

### 2.4 Adversarial Stress Testing
- **Endpoint**: `POST /api/v1/stress-test/adversarial`
- **Request Body**: `FinancialProblem`
- **Response**: Alpha-Beta minimax game result against worst-case macroeconomic shock adversaries.

---

## 3. Optimization Engine Integration Contract (Parv — Port 8001)

`services/optimization-engine` exposes the following FastAPI endpoints:
- `GET /health`: Service health check (`status`, `service`, `environment`, `version`, `port`).
- `POST /api/v1/optimization/classical` (alias `POST /api/v1/classical/optimize`): Executes `exact`, `greedy`, `sa`/`simulated_annealing`, `slsqp`/`continuous`, or `milp`.
- `POST /api/v1/quantum/qubo`: Formulates binary allocation as a QUBO matrix $Q$ where objective $= x^T Q x + \text{offset}$, with optional slack variables.
- `POST /api/v1/optimization/quantum` (alias `POST /api/v1/quantum/qaoa`): Executes QAOA circuit simulation on the CPU statevector simulator (capped by `MAX_QUBITS_CAP`) and returns quantum sample distribution vs classical ground truth.
- `POST /api/v1/optimization/compare`: Benchmarks Exact, Greedy, Simulated Annealing, Continuous SLSQP, and QAOA side-by-side.
- `POST /api/v1/optimization/benchmark`: Generates a synthetic instance of size `n_assets` and runs a comparative benchmark.

---

## 4. Go Backend Orchestration Contract (Aryan — Port 8080)

Aryan's Go API gateway handles:
1. Validating incoming user inputs against `shared/schemas/financial_problem.json`.
2. Calling Atharv's AI Engine (`POST http://ai-engine:8000/api/v1/strategy/generate`).
3. Calling Parv's Optimization Engine (`POST http://optimization-engine:8001/api/v1/quantum/qaoa`).
4. Writing audit logs and strategy execution metrics to PostgreSQL (`strategy_execution_logs` table).
5. Returning the unified response to the SvelteKit frontend dashboard.
