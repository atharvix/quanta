"""Synthetic financial instance generator for reproducible benchmarks."""

from __future__ import annotations
from typing import List, Optional
import numpy as np

from app.models.financial_problem import FinancialProblem, Asset, Liability, Constraint


def generate_synthetic_problem(
    n_assets: int = 6,
    difficulty: str = "balanced",
    seed: int = 42,
    capital: float = 100.0,
) -> FinancialProblem:
    """Generate a reproducible, realistic synthetic corporate treasury problem."""
    rng = np.random.RandomState(seed)

    # Asset classes catalogue
    asset_types = ["gov_bond", "corp_bond", "equity", "real_estate", "commodity", "cash_buffer"]
    asset_names_catalog = [
        "Sovereign 10Y G-Sec",
        "AAA Corporate Bond",
        "Large-Cap Bluechip Equity",
        "Commercial Infrastructure Trust",
        "Gold / Commodity Hedge",
        "91-Day Treasury Bills",
        "Green Energy Infrastructure Fund",
        "Short-Duration Liquid Mutual Fund",
        "High-Yield Corporate Debentures",
        "Mid-Cap Growth Equity Fund",
        "State Development Loans (SDL)",
        "Overnight Reverse Repo",
        "Commercial Paper (CP)",
        "Inflation-Indexed Bonds (IIB)",
        "Export Credit Financing Facility",
        "Structured Real Estate Yield Debt",
        "Private Debt Special Opportunities",
        "Multilateral Development Bank Bond",
    ]

    assets: List[Asset] = []
    for i in range(n_assets):
        name = asset_names_catalog[i] if i < len(asset_names_catalog) else f"Asset_{i+1}"
        atype = asset_types[i % len(asset_types)]

        if atype == "cash_buffer":
            exp_ret = float(rng.uniform(0.025, 0.040))
            risk = float(rng.uniform(0.005, 0.020))
            liq = float(rng.uniform(0.95, 1.00))
            cost = float(rng.uniform(5.0, 15.0))
        elif atype == "gov_bond":
            exp_ret = float(rng.uniform(0.065, 0.078))
            risk = float(rng.uniform(0.03, 0.06))
            liq = float(rng.uniform(0.85, 0.95))
            cost = float(rng.uniform(10.0, 25.0))
        elif atype == "corp_bond":
            exp_ret = float(rng.uniform(0.080, 0.098))
            risk = float(rng.uniform(0.08, 0.16))
            liq = float(rng.uniform(0.65, 0.85))
            cost = float(rng.uniform(10.0, 20.0))
        elif atype == "equity":
            exp_ret = float(rng.uniform(0.120, 0.165))
            risk = float(rng.uniform(0.25, 0.45))
            liq = float(rng.uniform(0.70, 0.85))
            cost = float(rng.uniform(10.0, 25.0))
        elif atype == "real_estate":
            exp_ret = float(rng.uniform(0.100, 0.130))
            risk = float(rng.uniform(0.15, 0.28))
            liq = float(rng.uniform(0.20, 0.45))
            cost = float(rng.uniform(15.0, 30.0))
        else:  # commodity / other
            exp_ret = float(rng.uniform(0.075, 0.110))
            risk = float(rng.uniform(0.18, 0.32))
            liq = float(rng.uniform(0.50, 0.75))
            cost = float(rng.uniform(8.0, 18.0))

        assets.append(
            Asset(
                id=f"asset_{i+1}",
                name=name,
                asset_type=atype,
                cost=cost,
                expected_return=round(exp_ret, 4),
                risk=round(risk, 4),
                liquidity=round(liq, 4),
                volatility=round(risk * 1.1, 4),
            )
        )

    # Liabilities
    liabilities = [
        Liability(
            id="debt_senior_1",
            name="Senior Term Loan Tranche A",
            amount=15.0,  # Rs 15 Cr
            interest_rate=0.085,
            maturity=12,
            priority=1,
            is_mandatory_reserve=True,
        )
    ]

    # Constraints depending on difficulty profile
    if difficulty == "loose_budget":
        risk_tol = 0.70
        min_liq = 0.15
        max_exp = 0.50
    elif difficulty == "tight_liquidity":
        risk_tol = 0.50
        min_liq = 0.45
        max_exp = 0.35
    elif difficulty == "high_risk_aversion":
        risk_tol = 0.15
        min_liq = 0.30
        max_exp = 0.30
    else:  # "balanced"
        risk_tol = 0.50
        min_liq = 0.25
        max_exp = 0.40

    constraints = [
        Constraint(
            id="c_min_liquidity",
            constraint_type="minimum_liquidity",
            description=f"Maintain minimum {min_liq*100:.0f}% portfolio liquidity",
            parameters={"threshold": min_liq},
        ),
        Constraint(
            id="c_max_exposure",
            constraint_type="maximum_asset_exposure",
            description=f"Cap individual asset exposure at {max_exp*100:.0f}%",
            parameters={"max_fraction": max_exp},
        ),
    ]

    return FinancialProblem(
        capital=capital,
        step_size=5.0,
        assets=assets,
        liabilities=liabilities,
        constraints=constraints,
        risk_tolerance=risk_tol,
        minimum_liquidity=min_liq,
        max_exposure_pct=max_exp,
        objective="maximize_risk_adjusted_return",
        market_state="normal",
    )

