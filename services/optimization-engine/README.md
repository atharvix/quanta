# Optimization Engine (Parv - Member 2)

This directory houses the Mathematical Programming, QUBO Formulation, and Quantum Simulation Engine for QUANTA.

## Implemented Capabilities & Modules
- **Classical Solvers (`app/classical/`)**:
  - `ExactSolver` (`exact.py`): Exhaustive $2^N$ combinatorial search for ground-truth optimality.
  - `GreedySolver` (`greedy.py`): Sharpe-like risk-adjusted heuristic baseline.
  - `SimulatedAnnealingSolver` (`simulated_annealing.py`): Seeded Metropolis-Hastings discrete search.
  - `ContinuousSLSQPSolver` (`continuous.py`): SciPy SLSQP continuous nonlinear optimization.
  - `MILPSolver` (`milp.py`): SciPy HiGHS mixed-integer linear programming.
- **QUBO & Ising Formulation (`app/qubo/`)**:
  - `QUBOBuilder` (`builder.py`) & `ising.py`: Constructs QUBO matrix $Q$, offset, penalty weights, optional binary slack variables for liquidity constraints, and Ising Hamiltonian mappings.
- **Quantum Simulation (`app/quantum/`)**:
  - `QAOAStatevectorSimulator` (`simulator.py`): CPU statevector QAOA simulator ($p$-layer cost/mixer evolution, classical parameter optimization, shot sampling, and exact ground-truth approximation ratio).
- **Benchmarking (`app/benchmarks/`, `benchmarks/results/`)**:
  - Synthetic problem generator, multi-solver runner, and matplotlib report plots (`feasibility_rate_by_method.png`, `objective_gap_vs_exact.png`, `qaoa_approx_ratio_vs_p.png`, `runtime_vs_n_scaling.png`).

## REST API Endpoints (Port `8001`)
- `GET /health` — Service health & version check
- `POST /api/v1/optimization/classical` (alias: `POST /api/v1/classical/optimize`) — Run `exact`, `greedy`, `sa`/`simulated_annealing`, `slsqp`/`continuous`, or `milp`
- `POST /api/v1/optimization/quantum` (alias: `POST /api/v1/quantum/qaoa`) — Run QAOA statevector simulation (enforces `MAX_QUBITS_CAP`)
- `POST /api/v1/quantum/qubo` — Formulate `FinancialProblem` as a QUBO matrix $Q$ and slack variable mapping
- `POST /api/v1/optimization/compare` — Compare Exact, Greedy, SA, SLSQP, and QAOA side-by-side
- `POST /api/v1/optimization/benchmark` — Generate synthetic instance of size `n_assets` and benchmark all solvers

## Environment Variables
| Variable | Default | Description |
| :--- | :--- | :--- |
| `OPTIMIZATION_ENGINE_HOST` | `0.0.0.0` | Bind host |
| `OPTIMIZATION_ENGINE_PORT` | `8001` | Service port |
| `ENVIRONMENT` | `development` | Runtime environment (`development` / `production`) |
| `LOG_LEVEL` | `INFO` | Python logging level |
| `MAX_QUBITS_CAP` | `20` | Hard cap on simulator qubit count |
| `DEFAULT_DISCRETIZATION_UNIT` | `1.0` | ₹ Cr granularity for slack variables |
| `DEFAULT_PENALTY_MULTIPLIER` | `10.0` | QUBO constraint penalty multiplier |
| `DEFAULT_QAOA_SHOTS` | `2048` | Default QAOA measurement shots |

## Setup, Running & Testing

```bash
cd services/optimization-engine
pip install -r requirements.txt

# Start FastAPI service on port 8001
uvicorn app.main:app --host 0.0.0.0 --port 8001 --reload

# Run tests (28 unit + API tests)
python -m pytest -v

# Run ₹100 Cr optimization demo
python examples/basic_optimization_demo.py
```

Interactive OpenAPI docs are available at `http://localhost:8001/docs`.
