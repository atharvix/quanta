"""Unit and correctness tests for QUBO formulation and Ising conversion."""

import pytest
import numpy as np

from app.models.financial_problem import FinancialProblem, Asset
from app.models.optimization_model import OptimizationModel
from app.qubo.builder import QUBOBuilder
from app.qubo.ising import qubo_to_ising, ising_to_qubo
from app.classical.exact import ExactSolver


def test_qubo_ising_roundtrip():
    # Random QUBO matrix
    rng = np.random.RandomState(42)
    Q = rng.randn(4, 4)
    Q = np.triu(Q, k=1)
    np.fill_diagonal(Q, rng.randn(4))
    offset = 12.5

    ising = qubo_to_ising(Q, offset)
    assert ising.num_spins == 4

    # Test on all 2^4 spin configurations
    for idx in range(16):
        x = np.array([(idx >> j) & 1 for j in range(4)], dtype=np.float64)
        z = 1.0 - 2.0 * x

        qubo_energy = float(np.dot(x, np.dot(Q, x)) + offset)
        ising_energy = ising.energy(z)
        assert qubo_energy == pytest.approx(ising_energy, rel=1e-5)

    # Reverse conversion
    Q_rec, offset_rec = ising_to_qubo(ising)
    assert np.allclose(Q, Q_rec, atol=1e-5)
    assert offset == pytest.approx(offset_rec, rel=1e-5)


def test_qubo_energy_equivalence_and_global_optimum():
    """Mandatory test:
    
    For small n, enumerate all bitstrings.
    Every feasible x must have QUBO energy equal to -objective (plus a known constant).
    The QUBO global minimum must equal the exact classical optimum.
    """
    problem = FinancialProblem(
        capital=30.0,
        assets=[
            Asset(name="Asset_1", cost=10.0, expected_return=0.08, risk=0.04, liquidity=0.90),
            Asset(name="Asset_2", cost=15.0, expected_return=0.12, risk=0.10, liquidity=0.80),
            Asset(name="Asset_3", cost=20.0, expected_return=0.15, risk=0.20, liquidity=0.70),
        ],
        liabilities=[],
        constraints=[],
        risk_tolerance=0.50,
        minimum_liquidity=0.20,
    )
    model = OptimizationModel(problem)

    exact_solver = ExactSolver()
    exact_res = exact_solver.solve(model)
    assert exact_res.feasible is True
    exact_opt_obj = exact_res.objective_value
    exact_opt_x = np.array([exact_res.allocation[name] for name in model.asset_names])

    # Build QUBO with slack variables
    builder = QUBOBuilder(discretization_unit=1.0, penalty_multiplier=10.0)
    qubo = builder.build(model, use_slack=True)

    total_dim = 1 << qubo.total_qubits
    best_qubo_energy = float("inf")
    best_qubo_y = None

    # Enumerate all 2^total_qubits states
    for idx in range(total_dim):
        y = np.array([(idx >> j) & 1 for j in range(qubo.total_qubits)], dtype=np.float64)
        e = qubo.energy(y)

        if e < best_qubo_energy:
            best_qubo_energy = e
            best_qubo_y = y

    # Extract asset decision bits from QUBO global minimum
    qubo_asset_bits = best_qubo_y[:model.n]

    # Verify 1: The QUBO global minimum must equal the exact classical optimum
    assert np.array_equal(qubo_asset_bits, exact_opt_x)

    # Verify 2: For every feasible x, there exists a slack assignment such that
    # QUBO energy equals -objective(x)
    n_asset_states = 1 << model.n
    k_slack = qubo.n_slack

    for a_idx in range(n_asset_states):
        x = np.array([(a_idx >> j) & 1 for j in range(model.n)], dtype=np.float64)
        if model.is_feasible(x):
            obj = model.score(x)
            # Find the minimum energy over slack configurations for this feasible x
            min_e_for_x = float("inf")
            for s_idx in range(1 << k_slack):
                s = np.array([(s_idx >> j) & 1 for j in range(k_slack)], dtype=np.float64)
                y_combo = np.concatenate([x, s])
                e_val = qubo.energy(y_combo)
                if e_val < min_e_for_x:
                    min_e_for_x = e_val

            # Penalties should be 0, so min_e_for_x == -objective
            assert min_e_for_x == pytest.approx(-obj, rel=1e-3, abs=1e-3)

