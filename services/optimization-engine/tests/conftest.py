"""Pytest configuration and shared fixtures for Optimization Engine tests."""

import os
import sys
import pytest

# Ensure services/optimization-engine is on sys.path
engine_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
if engine_root not in sys.path:
    sys.path.insert(0, engine_root)

from app.models.financial_problem import FinancialProblem, Asset, Liability, Constraint
from app.models.optimization_model import OptimizationModel


@pytest.fixture
def sample_problem() -> FinancialProblem:
    """Fixture providing a small, well-defined corporate treasury problem."""
    assets = [
        Asset(id="bond_1", name="Gov Bond 1", cost=20.0, expected_return=0.07, risk=0.05, liquidity=0.95),
        Asset(id="bond_2", name="Corp Bond 2", cost=25.0, expected_return=0.09, risk=0.12, liquidity=0.80),
        Asset(id="equity_1", name="Equity 1", cost=25.0, expected_return=0.14, risk=0.35, liquidity=0.75),
        Asset(id="cash_1", name="T-Bills", cost=15.0, expected_return=0.04, risk=0.01, liquidity=1.00),
    ]
    liabilities = [
        Liability(id="debt_1", name="Senior Debt", amount=15.0, interest_rate=0.08, maturity=12, is_mandatory_reserve=True)
    ]
    constraints = [
        Constraint(
            id="c_min_liq",
            constraint_type="minimum_liquidity",
            parameters={"threshold": 0.25},
        ),
        Constraint(
            id="c_max_exp",
            constraint_type="maximum_asset_exposure",
            parameters={"max_fraction": 0.50},
        ),
    ]
    return FinancialProblem(
        capital=100.0,
        assets=assets,
        liabilities=liabilities,
        constraints=constraints,
        risk_tolerance=0.50,
        minimum_liquidity=0.25,
        objective="maximize_risk_adjusted_return",
    )


@pytest.fixture
def sample_model(sample_problem) -> OptimizationModel:
    return OptimizationModel(sample_problem)

