"""Unit tests for classical optimization algorithms."""

import pytest
import numpy as np

from app.classical.exact import ExactSolver
from app.classical.greedy import GreedySolver
from app.classical.simulated_annealing import SimulatedAnnealingSolver
from app.classical.continuous import ContinuousSLSQPSolver
from app.classical.milp import MILPSolver


def test_exact_solver_optimal(sample_model):
    solver = ExactSolver()
    result = solver.solve(sample_model)

    assert result.status == "optimal"
    assert result.feasible is True
    assert len(result.constraint_violations) == 0
    assert result.objective_value > 0.0
    assert len(result.allocation) == sample_model.n


def test_greedy_solver(sample_model):
    solver = GreedySolver()
    result = solver.solve(sample_model)

    assert result.feasible is True
    assert result.status == "feasible"
    assert result.objective_value > 0.0


def test_simulated_annealing_deterministic(sample_model):
    solver1 = SimulatedAnnealingSolver(seed=42)
    res1 = solver1.solve(sample_model, seed=42)

    solver2 = SimulatedAnnealingSolver(seed=42)
    res2 = solver2.solve(sample_model, seed=42)

    assert res1.feasible is True
    assert res1.objective_value == pytest.approx(res2.objective_value, rel=1e-5)
    assert res1.allocation == res2.allocation


def test_continuous_slsqp_solver(sample_model):
    solver = ContinuousSLSQPSolver()
    result = solver.solve(sample_model)

    assert result.feasible is True
    assert result.metadata["is_continuous"] is True
    assert result.portfolio_return > 0.0
    # Continuous weights sum should not exceed 1.0 + tolerance
    total_wt = sum(result.allocation.values())
    assert total_wt <= 1.0 + 1e-4


def test_milp_solver(sample_model):
    solver = MILPSolver()
    result = solver.solve(sample_model)

    assert result.feasible is True
    assert result.status == "optimal"
    assert result.objective_value > 0.0

