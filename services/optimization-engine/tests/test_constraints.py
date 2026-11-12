"""Unit tests for constraint evaluation, feasibility checks, and scoring."""

import numpy as np
import pytest
from app.models.optimization_model import OptimizationModel
from app.models.financial_problem import FinancialProblem, Asset, Liability, Constraint


def test_feasibility_and_scoring(sample_model):
    # Empty allocation (holding 100% cash)
    x_zero = np.zeros(sample_model.n)
    assert sample_model.is_feasible(x_zero)
    metrics_zero = sample_model.evaluate_allocation(x_zero)
    assert metrics_zero["liquidity"] == 1.0
    assert metrics_zero["capital_used"] == 0.0

    # Feasible allocation: select 2 safe bonds
    # costs: 20 + 25 = 45 <= 85 deployable
    x_valid = np.array([1.0, 1.0, 0.0, 0.0])
    assert sample_model.is_feasible(x_valid)
    assert len(sample_model.violation_report(x_valid)) == 0

    metrics_valid = sample_model.evaluate_allocation(x_valid)
    assert metrics_valid["capital_used"] == 45.0
    assert metrics_valid["objective_value"] > 0.0


def test_budget_violation(sample_model):
    # Force budget overflow: allocate all assets (costs: 20 + 25 + 25 + 15 = 85 <= 85)
    # Let's create an overloaded model where budget is 30
    problem = FinancialProblem(
        capital=30.0,
        assets=[
            Asset(name="A", cost=20.0, expected_return=0.10, risk=0.1, liquidity=0.9),
            Asset(name="B", cost=20.0, expected_return=0.12, risk=0.2, liquidity=0.8),
        ],
        liabilities=[],
        constraints=[],
        risk_tolerance=0.5,
        minimum_liquidity=0.2,
    )
    model = OptimizationModel(problem)
    x_over = np.array([1.0, 1.0])  # Cost = 40 > 30
    assert not model.is_feasible(x_over)
    violations = model.violation_report(x_over)
    assert any("Capital budget exceeded" in v for v in violations)


def test_minimum_liquidity_violation():
    # Asset with zero liquidity
    problem = FinancialProblem(
        capital=100.0,
        assets=[
            Asset(name="Illiquid Real Estate", cost=80.0, expected_return=0.15, risk=0.1, liquidity=0.0),
        ],
        liabilities=[],
        constraints=[
            Constraint(id="c_liq", constraint_type="minimum_liquidity", parameters={"threshold": 0.50})
        ],
        risk_tolerance=0.5,
        minimum_liquidity=0.50,
    )
    model = OptimizationModel(problem)
    x = np.array([1.0])  # Liquid capital = 20 Cr unspent cash / 100 = 20% < 50%
    assert not model.is_feasible(x)
    violations = model.violation_report(x)
    assert any("Minimum liquidity deficit" in v for v in violations)

