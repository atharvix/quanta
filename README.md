# QUANTA — Autonomous Financial Treasury & Capital Optimization Platform

[![Python](https://img.shields.io/badge/Python-3.12%2B-blue.svg)](https://www.python.org/)
[![FastAPI](https://img.shields.io/badge/FastAPI-0.115%2B-green.svg)](https://fastapi.tiangolo.com/)
[![License](https://img.shields.io/badge/License-Academic%20Research-purple.svg)]()

> **Enterprise-Oriented AI Financial Decision Intelligence Platform**  
> QUANTA evaluates corporate balance sheets, investment opportunities, liabilities, liquidity requirements, and risk constraints to synthesize optimal, deterministic, and explainable capital allocation strategies.

---

## Team & Module Ownership

| Member | Role & Module | Responsibilities |
| :--- | :--- | :--- |
| **Atharv (Team Lead)** | **AI Decision Engine** | Search Algorithms, CSP, Bayesian Risk Reasoning, Game Theory (Minimax/Alpha-Beta), MDP & RL, Intelligent Agent Coordinator |
| **Parv** | **Optimization Engine** | Mathematical Programming, QUBO Formulations, QAOA, Quantum Simulation & Classical-vs-Quantum Benchmarking |
| **Aryan** | **Backend & Platform** | Go REST API, PostgreSQL, Authentication, Audit Logs, SvelteKit Frontend & Cloud/Docker Deployment |

---

## Monorepo Architecture

```
quanta/
├── apps/
│   ├── frontend/             # SvelteKit + TypeScript Dashboard (Aryan)
│   └── backend/              # Go REST API & PostgreSQL Service (Aryan)
├── services/
│   ├── ai-engine/            # Python AI Decision Engine (Atharv)
│   └── optimization-engine/  # Mathematical & Quantum Optimization (Parv)
├── shared/
│   └── schemas/              # Common JSON / OpenAPI Schema Contracts
├── infrastructure/
│   ├── docker/               # Dockerfiles & production compose configs
│   └── postgres/             # Database initialization schemas & migrations
├── docs/                     # Architectural, AI Engine & Integration specs
├── tests/                    # Integration & end-to-end tests
└── scripts/                  # Development & utility scripts
```

---

## Academic Scope: Classical AI Curriculum Integration

QUANTA implements core principles of Artificial Intelligence directly mapped to treasury optimization:

1. **Problem Formulation & State Spaces (Unit I)**: Immutable `FinancialState` space modeling balance sheet dynamics under discrete capital allocation actions.
2. **Search Algorithms (Unit II)**:
   - **Uninformed**: Breadth-First Search (BFS), Depth-First Search (DFS).
   - **Informed & Heuristic**: Systematic Generate-and-Test, Steepest-Ascent Hill Climbing, Greedy Best-First Search, and $A^*$ Search using an admissible risk-adjusted heuristic $f(n) = g(n) + h(n)$.
3. **Game Trees & Constraint Satisfaction (Unit III)**:
   - **Adversarial Search**: Minimax and Alpha-Beta Pruning modeling capital strategies against worst-case macroeconomic shock adversaries.
   - **Constraint Satisfaction (CSP)**: Backtracking search with Forward Checking and Minimum Remaining Values (MRV) over portfolio limits, liquidity floors, and debt reserves.
4. **Markov Decision Processes & RL (Unit IV)**: Multi-period treasury rebalancing MDP with state transitions across market regimes (Bull, Bear, Volatile, Stagnant).
5. **Intelligent Agents & Reasoning (Unit V)**: Multi-agent cooperative architecture (`TreasuryAgent`, `RiskAgent`, `ComplianceAgent`, `DecisionAgent`) and Bayesian risk reasoning.

---

## Quickstart

### Prerequisites
- Python 3.12+ (or 3.13)
- Docker & Docker Compose (optional for full platform)

### 1. Run the AI Engine Tests
```bash
cd services/ai-engine
pytest -v
```

### 2. Run the Synthetic ₹100 Cr Benchmark Example
```bash
python services/ai-engine/examples/basic_capital_allocation.py
```

### 3. Launch the FastAPI AI Engine
```bash
cd services/ai-engine
uvicorn app.main:app --host 0.0.0.0 --port 8000 --reload
```
Interactive API documentation will be available at: `http://localhost:8000/docs`

---

## Optimization Engine (Member 2 — Parv Sharma)

The Optimization Engine (`services/optimization-engine/`, port 8001) provides mathematical programming (LP/QP), Quadratic Unconstrained Binary Optimization (QUBO) formulations, and Quantum Approximate Optimization Algorithm (QAOA) statevector simulation.

### 1. Run Optimization Engine Tests
```bash
cd services/optimization-engine
pytest -v
```

### 2. Run the Synthetic ₹100 Cr Optimization Demo
```bash
python services/optimization-engine/examples/basic_optimization_demo.py
```

### 3. Launch the FastAPI Optimization Engine Service
```bash
cd services/optimization-engine
uvicorn app.main:app --host 0.0.0.0 --port 8001 --reload
```
Interactive API documentation is available at: `http://localhost:8001/docs`

### 4. Run the Full Classical vs QAOA Benchmark Suite
```bash
python -c "import sys; sys.path.insert(0, 'services/optimization-engine'); from app.benchmarks.reporter import generate_benchmark_and_reports; generate_benchmark_and_reports()"
```

---

## Disclaimer
QUANTA is an academic research prototype and simulated decision intelligence framework. It does not provide certified financial advice or execute live transactions with real financial institutions.

