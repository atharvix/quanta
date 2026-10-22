"""Mathematical optimization model, constraint evaluation, and scoring logic."""

from __future__ import annotations
from typing import Any, Dict, List, Optional, Tuple
import numpy as np

from app.models.financial_problem import FinancialProblem, Asset


class OptimizationModel:
    """Mathematical formulation of the capital allocation optimization problem.
    
    Variables:
        Binary: x_i in {0, 1} indicates full discrete selection of asset i.
        Continuous: w_i in [0, 1] indicates fractional allocation of deployable capital to asset i.
    
    Objective:
        Maximize: sum(mu_i * x_i) - lambda * x^T Sigma x
        Equivalently, Minimize: - sum(mu_i * x_i) + lambda * x^T Sigma x
        where lambda = 1.0 - risk_tolerance, and Sigma is diagonal (risk_i^2) or explicit covariance.
    
    Hard Constraints:
        1. Capital Budget:
           sum(c_i * x_i) <= deployable_capital
           where deployable_capital = capital - mandatory_debt_reserve.
        2. Minimum Liquidity:
           Unspent cash has liquidity 1.0. Asset i has liquidity l_i.
           Total Liquid Capital = (C_deploy - sum(c_i * x_i)) + sum(l_i * c_i * x_i)
                                = C_deploy - sum((1 - l_i) * c_i * x_i)
           Requirement: Total Liquid Capital >= minimum_liquidity * C_deploy
           Linear form: sum((1 - l_i) * c_i * x_i) <= (1 - minimum_liquidity) * C_deploy
        3. Maximum Asset Exposure:
           c_i * x_i <= max_exposure_pct * C_deploy for all i.
        4. Group / Sector Caps (optional):
           sum_{i in group_g}(c_i * x_i) <= group_cap_g * C_deploy.
    """

    def __init__(
        self,
        problem: FinancialProblem,
        candidate_subset: Optional[List[str]] = None,
        planning_horizon_months: int = 12,
    ):
        self.raw_problem = problem
        self.planning_horizon_months = planning_horizon_months

        # Filter candidate assets if subset is provided
        if candidate_subset:
            subset_set = set(candidate_subset)
            self.assets: List[Asset] = [
                a for a in problem.assets if a.name in subset_set or a.id in subset_set
            ]
        else:
            self.assets = list(problem.assets)

        self.n = len(self.assets)
        self.asset_names = [a.name for a in self.assets]
        self.asset_ids = [a.id or a.name for a in self.assets]

        # Calculate mandatory liability reserve due within planning horizon
        self.mandatory_debt_reserve = 0.0
        for l in problem.liabilities:
            # Check mandatory reserve flag or maturity within planning horizon
            is_due = (l.maturity <= self.planning_horizon_months) if l.maturity is not None else False
            if l.is_mandatory_reserve or is_due:
                self.mandatory_debt_reserve += l.amount

        # Deployable capital after setting aside debt reserve
        self.deployable_capital = max(0.0, problem.capital - self.mandatory_debt_reserve)
        self.total_capital = problem.capital

        # Risk aversion parameter lambda: higher risk_tolerance -> lower lambda
        self.risk_tolerance = float(problem.risk_tolerance)
        self.risk_aversion = float(1.0 - self.risk_tolerance)

        # Minimum required portfolio liquidity ratio
        self.min_liquidity_ratio = float(problem.minimum_liquidity)

        # Maximum exposure fraction per individual asset
        self.max_exposure_pct = problem.max_exposure_pct
        for c in problem.constraints:
            if c.constraint_type in ("maximum_asset_exposure", "max_exposure"):
                thresh = c.parameters.get("threshold") or c.parameters.get("max_fraction")
                if thresh is not None:
                    self.max_exposure_pct = float(thresh)

        # Group caps from problem or constraints
        self.group_caps: Dict[str, float] = dict(problem.group_caps or {})
        for c in problem.constraints:
            if c.constraint_type in ("maximum_asset_type_exposure", "group_cap"):
                grp = c.parameters.get("asset_type") or c.parameters.get("group")
                cap = c.parameters.get("max_fraction") or c.parameters.get("threshold")
                if grp and cap is not None:
                    self.group_caps[str(grp).lower()] = float(cap)

        # Vector representations
        # Asset cost in Rs Cr (if asset.cost is a tiny fraction < 0.05, treat as transaction cost and allocate step_size or min_investment;
        # if asset.cost >= 0.05, it represents lump-sum asset ticket size in Rs Cr)
        costs = []
        for a in self.assets:
            if a.cost > 0.05:
                costs.append(float(a.cost))
            elif a.min_investment and a.min_investment > 0.05:
                costs.append(float(a.min_investment))
            else:
                # Default discrete cost unit = step_size or 5.0 Cr
                step = problem.step_size if problem.step_size and problem.step_size > 0.05 else 5.0
                costs.append(float(step))
        self.c = np.array(costs, dtype=np.float64)

        # Expected return mu
        self.mu = np.array([float(a.expected_return) for a in self.assets], dtype=np.float64)

        # Risk vector (standard deviation / volatility)
        self.risk_vec = np.array([float(a.risk) for a in self.assets], dtype=np.float64)

        # Liquidity coefficient vector l_i in [0, 1]
        self.liquidity_vec = np.array([float(a.liquidity) for a in self.assets], dtype=np.float64)

        # Covariance matrix Sigma (n x n)
        if problem.covariance_matrix and len(problem.covariance_matrix) == self.n:
            self.Sigma = np.array(problem.covariance_matrix, dtype=np.float64)
        else:
            # Default diagonal covariance matrix: Sigma_ii = risk_i^2
            self.Sigma = np.diag(self.risk_vec ** 2)

    def is_problem_infeasible(self) -> Tuple[bool, List[str]]:
        """Pre-check whether problem parameters are inherently infeasible."""
        reasons = []
        if self.n == 0:
            reasons.append("No assets available in candidate pool")
        if self.deployable_capital <= 0.0:
            reasons.append(
                f"Mandatory debt reserve (Rs {self.mandatory_debt_reserve:.2f} Cr) "
                f"exceeds or equals total capital (Rs {self.total_capital:.2f} Cr)"
            )
        if self.min_liquidity_ratio > 1.0:
            reasons.append(f"Minimum liquidity requirement ({self.min_liquidity_ratio*100:.1f}%) cannot exceed 100%")
        return (len(reasons) > 0, reasons)

    def evaluate_allocation(self, x: np.ndarray) -> Dict[str, Any]:
        """Compute all financial metrics for an allocation vector x (binary or continuous)."""
        x = np.asarray(x, dtype=np.float64)
        capital_used = float(np.dot(self.c, x))
        unspent_cash = max(0.0, self.deployable_capital - capital_used)

        # Portfolio return: weighted return of selected assets + risk-free return on unspent cash
        # (assume unspent cash earns base risk-free return of 3%)
        r_cash = 0.03
        if self.deployable_capital > 0:
            weighted_return = float((np.dot(self.mu, self.c * x) + r_cash * unspent_cash) / self.deployable_capital)
        else:
            weighted_return = 0.0

        # Portfolio risk: quadratic variance x^T Sigma x
        if self.deployable_capital > 0:
            normalized_w = (self.c * x) / self.deployable_capital
            portfolio_risk = float(np.sqrt(max(0.0, np.dot(normalized_w, np.dot(self.Sigma, normalized_w)))))
        else:
            portfolio_risk = 0.0

        # Total liquidity: unspent cash (100% liquid) + liquid component of assets
        liquid_assets = float(np.dot(self.liquidity_vec, self.c * x))
        total_liquid_capital = unspent_cash + liquid_assets
        liquidity_ratio = float(total_liquid_capital / self.deployable_capital) if self.deployable_capital > 0 else 1.0

        # Objective function score:
        # Expected Return - lambda * Risk^2 (Mean-Variance Utility)
        # Scaled to maintain numerical stability
        raw_return = float(np.dot(self.mu, x))
        raw_risk_penalty = float(self.risk_aversion * np.dot(x, np.dot(self.Sigma, x)))
        objective_value = raw_return - raw_risk_penalty

        return {
            "capital_used": capital_used,
            "unspent_cash": unspent_cash,
            "portfolio_return": weighted_return,
            "portfolio_risk": portfolio_risk,
            "liquidity": liquidity_ratio,
            "total_liquid_capital": total_liquid_capital,
            "objective_value": objective_value,
        }

    def score(self, x: np.ndarray) -> float:
        """Return the objective value for allocation x."""
        return float(self.evaluate_allocation(x)["objective_value"])

    def violation_report(self, x: np.ndarray, tolerance: float = 1e-4) -> List[str]:
        """Check all constraints and return human-readable violation descriptions."""
        x = np.asarray(x, dtype=np.float64)
        violations: List[str] = []

        # 1. Budget constraint: sum(c_i * x_i) <= C_deploy
        total_spent = float(np.dot(self.c, x))
        if total_spent > (self.deployable_capital + tolerance):
            deficit = total_spent - self.deployable_capital
            violations.append(
                f"Capital budget exceeded by Rs {deficit:.2f} Cr "
                f"(allocated Rs {total_spent:.2f} Cr vs deployable limit Rs {self.deployable_capital:.2f} Cr)"
            )

        # 2. Negative cash or overspent check
        if total_spent < -tolerance or np.any(x < -tolerance):
            violations.append("Negative allocation or negative cash detected")

        # 3. Minimum liquidity constraint:
        # Total Liquid Capital >= min_liquidity * C_deploy
        metrics = self.evaluate_allocation(x)
        actual_liq = metrics["liquidity"]
        if actual_liq < (self.min_liquidity_ratio - tolerance):
            deficit = (self.min_liquidity_ratio - actual_liq) * 100
            violations.append(
                f"Minimum liquidity deficit: achieved {actual_liq*100:.1f}% "
                f"vs required floor {self.min_liquidity_ratio*100:.1f}% (deficit {deficit:.1f}%)"
            )

        # 4. Maximum individual asset exposure
        if self.max_exposure_pct is not None and self.deployable_capital > 0:
            max_allowed = self.max_exposure_pct * self.deployable_capital
            for i, a in enumerate(self.assets):
                asset_spent = float(self.c[i] * x[i])
                if asset_spent > (max_allowed + tolerance):
                    violations.append(
                        f"Asset '{a.name}' exposure of Rs {asset_spent:.2f} Cr "
                        f"exceeds limit Rs {max_allowed:.2f} Cr ({self.max_exposure_pct*100:.1f}%)"
                    )

        # 5. Asset class / group caps
        if self.group_caps and self.deployable_capital > 0:
            for grp_name, cap_frac in self.group_caps.items():
                max_grp_allowed = cap_frac * self.deployable_capital
                grp_spent = sum(
                    float(self.c[i] * x[i])
                    for i, a in enumerate(self.assets)
                    if (a.group and a.group.lower() == grp_name.lower())
                    or (a.asset_type and a.asset_type.lower() == grp_name.lower())
                )
                if grp_spent > (max_grp_allowed + tolerance):
                    violations.append(
                        f"Asset class '{grp_name}' total Rs {grp_spent:.2f} Cr "
                        f"exceeds cap Rs {max_grp_allowed:.2f} Cr ({cap_frac*100:.1f}%)"
                    )

        return violations

    def is_feasible(self, x: np.ndarray, tolerance: float = 1e-4) -> bool:
        """Check if allocation x strictly satisfies all hard constraints."""
        return len(self.violation_report(x, tolerance=tolerance)) == 0

