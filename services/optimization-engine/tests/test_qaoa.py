"""Unit tests for QAOA statevector simulator."""

import pytest
import numpy as np

from app.models.financial_problem import FinancialProblem, Asset
from app.models.optimization_model import OptimizationModel
from app.classical.exact import ExactSolver
from app.quantum.simulator import QAOAStatevectorSimulator


def test_qaoa_solution_and_approximation_ratio(sample_model):
    # Solve exact
    exact_solver = ExactSolver()
    exact_res = exact_solver.solve(sample_model)

    sim = QAOAStatevectorSimulator()
    result = sim.solve(
        sample_model,
        p=2,
        seed=42,
        shots=1024,
        exact_optimum_score=exact_res.objective_value,
        use_slack=False,
    )

    assert result.feasible is True
    assert result.method == "qaoa"
    assert result.metadata["simulation"] is True
    assert result.metadata["p"] == 2
    # p=2 should achieve a high approximation ratio (>= 0.85)
    assert result.metadata["approximation_ratio"] is not None
    assert result.metadata["approximation_ratio"] >= 0.85


def test_qaoa_qubit_hard_cap():
    sim = QAOAStatevectorSimulator(max_qubits=5)
    # 6 assets with max_qubits=5 must raise ValueError
    problem = FinancialProblem(
        capital=100.0,
        assets=[
            Asset(name=f"A_{i}", expected_return=0.1, risk=0.1, liquidity=0.9)
            for i in range(6)
        ],
        liabilities=[],
        constraints=[],
    )
    model = OptimizationModel(problem)

    with pytest.raises(ValueError, match="exceeds the hard simulator cap"):
        sim.solve(model, p=1, use_slack=False)


def test_statevector_unitarity():
    sim = QAOAStatevectorSimulator()
    n_qubits = 3
    dim = 1 << n_qubits
    C_diag = np.arange(dim, dtype=np.float64)

    gammas = np.array([0.5, 1.2])
    betas = np.array([0.3, 0.8])

    psi = sim.simulate_circuit(gammas, betas, C_diag, n_qubits)
    norm = np.sum(np.abs(psi) ** 2)
    assert norm == pytest.approx(1.0, abs=1e-6)

