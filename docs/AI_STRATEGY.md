# AI, Risk & Optimization Research Strategy — QUANTA

> **Document Status:** Phase 0 Draft (Revision 1) — Pending Review  
> **Important:** All models, solvers, and libraries listed below are **research candidates**, not final commitments. **No models or AI/ML libraries are installed or downloaded in Phase 0.**

---

## 1. Guiding Guardrails (Confirmed Decisions)

1. **Non-Authoritative AI:** Neither forecasting models, risk simulators, optimization solvers, quantum prototypes, nor LLMs are authoritative for balances, ledger state, or financial approvals.
2. **Clear Separation — Statistical Model Validation vs. Go Financial Verification:**
   - **Python Statistical / Model Validation:** Python owns forecasting backtests, risk-model evaluation, calibration checks, input data validation, model provenance, and model-version tracking.
   - **Go Financial Verification:** Go owns deterministic capital conservation, liquidity floors, counterparty limits, decimal precision, allocation validity, and financial invariants. Go **never** claims to independently prove the statistical correctness of Python predictions.
3. **LLMs Never Compute Financial Math:** Language models may summarize verified reports or retrieve internal documentation, but **must never** calculate balances, risk metrics, or allocations.
4. **No Assumed Quantum Advantage:** Quantum and quantum-inspired solvers must be benchmarked against classical baselines on identical seeded instances.
5. **Synthetic Data First:** Initial experiments use reproducible, seeded synthetic treasury datasets.

---

## 2. Candidate Research Tracks

### 2.1 Cash-Flow Forecasting Candidates
- **Seasonal Naive** *(Mandatory Baseline Floor)*
- **ETS** *(Classical Exponential Smoothing)*
- **ARIMA / SARIMAX** *(Classical Autoregressive + Exogenous)*
- **XGBoost** *(Gradient Boosted Trees)*
- **LightGBM** *(Gradient Boosted Trees / Quantiles)*
- **Temporal Fusion Transformer (TFT)** *(Deep Learning Multi-Horizon)*
- **Chronos** *(Foundation Time-Series Model)*
- **TimesFM** *(Foundation Time-Series Model)*

### 2.2 Risk Modeling & Anomaly Detection Candidates
- **Monte Carlo Simulation**
- **Historical and Parametric VaR**
- **CVaR (Expected Shortfall)**
- **Stress Testing**
- **Isolation Forest** *(Unsupervised anomaly detection on synthetic cash flows)*

### 2.3 Classical Capital Optimization Candidates
- **Linear Programming (LP)**
- **Quadratic Programming (QP)**
- **MILP** *(Mixed-Integer Linear Programming)*
- **NSGA-II** *(Multi-objective evolutionary Pareto exploration)*
- **HiGHS** *(Open-source LP/QP/MILP solver)*
- **OR-Tools** *(Optimization modeling & solver suite)*
- **Pyomo** *(Algebraic modeling language)*
- **CVXPY** *(Disciplined convex programming)*

### 2.4 Quantum & Quantum-Inspired Research Candidates
- **QUBO** *(Quadratic Unconstrained Binary Optimization formulation)*
- **QAOA** *(Quantum Approximate Optimization Algorithm in simulation)*
- **Quantum-Inspired Optimization** *(Classical Ising/annealing heuristics)*
- **PennyLane**, **Qiskit**, **D-Wave Ocean** *(Research SDK candidates)*

### 2.5 Language Intelligence Candidates
- **Qwen**, **Llama**, **GPT** *(Non-authoritative narrative explanation)*
- **BGE**, **E5** *(Dense embeddings for policy/report retrieval)*

---

## 3. Model Validation & Benchmarking Protocol (Proposed)

- **Forecasting Validation (Python):** Out-of-time rolling backtests reporting MAE, RMSE, and quantile pinball/coverage calibration vs. **Seasonal Naive** and **ETS**.
- **Risk Validation (Python):** VaR exceedance backtesting, CVaR tail stability, and deterministic stress scenario coverage.
- **Optimization Benchmarking (Python):** Side-by-side comparison of heuristic/quantum candidates vs. **HiGHS / MILP** exact baselines on objective gap, feasibility rate, and runtime; reported using the unified `OptimizationRun` status model (`COMPLETED`, `INFEASIBLE`, `SOLVER_FAILED`, `VERIFICATION_FAILED`, `CANCELLED`).
- **Provenance Tracking:** Every Python response includes `model_name`, `model_version`, `seed`, `validation_summary`, and `input_checks_passed`.

---

## 4. Decision Summary & Unresolved Questions

| Item | Classification | Notes |
| :--- | :--- | :--- |
| Candidate-only status in Phase 0; zero installs/downloads | **Confirmed** | Documentation phase only |
| Python owns statistical/model validation; Go owns financial verification | **Confirmed** | Go does not prove statistical accuracy |
| LLMs non-authoritative; quantum advantage never assumed | **Confirmed** | Empirical benchmarking required |
| Rolling backtest & exact-baseline benchmarking protocol | **Proposed** | Implemented in Phases 4–6 |

**Unresolved Questions:**
1. Which classical optimization stack (`CVXPY + HiGHS`, `OR-Tools`, or `Pyomo + HiGHS`) will be chosen as the primary Phase 5 baseline?
2. Which QAOA/QUBO SDK (`Qiskit`, `PennyLane`, or `D-Wave Ocean`) will be prototyped first in Phase 6?
3. Will language intelligence remain strictly local/offline (open-weight models) or support optional hosted APIs?
