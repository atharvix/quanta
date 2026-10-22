"""Unit tests for Pydantic domain models and financial problem schemas."""

import pytest
from app.models.financial_problem import FinancialProblem, Asset, Liability, Constraint
from app.models.optimization_model import OptimizationModel


def test_asset_creation_and_defaults():
    asset = Asset(
        name="Test Bond",
        expected_return=0.08,
        risk=0.10,
        liquidity=0.90,
    )
    assert asset.name == "Test Bond"
    assert asset.id == "Test Bond"
    assert asset.cost == 0.0
    assert asset.expected_return == 0.08


def test_liability_aliases():
    # Test normalization from principal and maturity_months
    lib1 = Liability(name="Term Loan", principal=20.0, maturity_months=18, interest_rate=0.08)
    assert lib1.amount == 20.0
    assert lib1.maturity == 18

    # Test normalization from amount and maturity
    lib2 = Liability(name="Term Loan 2", amount=15.0, maturity=12, interest_rate=0.07)
    assert lib2.principal == 15.0
    assert lib2.maturity_months == 12


def test_financial_problem_validation():
    problem = FinancialProblem(
        capital=100.0,
        assets=[
            Asset(name="Asset A", expected_return=0.10, risk=0.20, liquidity=0.80),
        ],
        liabilities=[],
        constraints=[],
        risk_tolerance=0.60,
        minimum_liquidity=0.20,
    )
    assert problem.capital == 100.0
    assert problem.risk_tolerance == 0.60
    assert problem.minimum_liquidity == 0.20
    assert problem.objective == "maximize_risk_adjusted_return"


def test_optimization_model_candidate_subset(sample_problem):
    # Filter to only two assets
    subset = ["Gov Bond 1", "Corp Bond 2"]
    model = OptimizationModel(sample_problem, candidate_subset=subset)
    assert model.n == 2
    assert model.asset_names == subset

