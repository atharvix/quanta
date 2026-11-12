"""SciPy-based continuous portfolio optimization using Sequential Least Squares Programming (SLSQP)."""

from __future__ import annotations
import time
from typing import Dict, List
import numpy as np
from scipy.optimize import minimize

from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import OptimizationResult


class ContinuousSLSQPSolver:
    """Continuous portfolio weight optimizer solving the Markowitz mean-variance problem."""

    def solve(self, model: OptimizationModel) -> OptimizationResult:
        start_time = time.perf_counter()
        n = model.n

        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="slsqp",
                status="infeasible",
                allocation={name: 0.0 for name in model.asset_names},
                objective_value=0.0,
                portfolio_return=0.0,
                portfolio_risk=0.0,
                liquidity=1.0,
                capital_used=0.0,
                feasible=False,
                constraint_violations=reasons,
                runtime_ms=elapsed_ms,
                metadata={"is_continuous": True},
            )

        # Objective function for continuous weights w in [0, 1]^n
        # Maximize: w^T mu - lambda * w^T Sigma w
        # Minimize: - w^T mu + lambda * w^T Sigma w
        def objective(w: np.ndarray) -> float:
            return float(-np.dot(model.mu, w) + model.risk_aversion * np.dot(w, np.dot(model.Sigma, w)))

        def obj_grad(w: np.ndarray) -> np.ndarray:
            return -model.mu + 2.0 * model.risk_aversion * np.dot(model.Sigma, w)

        # Bounds: 0 <= w_i <= max_exposure
        max_exp = model.max_exposure_pct if model.max_exposure_pct is not None else 1.0
        bounds = [(0.0, max_exp) for _ in range(n)]

        constraints: List[Dict] = []

        # 1. Budget constraint: sum(w_i) <= 1.0 -> 1.0 - sum(w_i) >= 0
        constraints.append({
            "type": "ineq",
            "fun": lambda w: 1.0 - np.sum(w),
            "jac": lambda w: -np.ones(n),
        })

        # 2. Minimum liquidity constraint:
        # Total liquidity = (1 - sum(w)) + sum(l_i * w_i) >= min_liquidity
        # (1 - min_liquidity) - sum((1 - l_i) * w_i) >= 0
        one_minus_l = 1.0 - model.liquidity_vec
        constraints.append({
            "type": "ineq",
            "fun": lambda w: (1.0 - model.min_liquidity_ratio) - np.dot(one_minus_l, w),
            "jac": lambda w: -one_minus_l,
        })

        # 3. Group caps
        if model.group_caps:
            for grp_name, cap_val in model.group_caps.items():
                grp_mask = np.array([
                    1.0 if (
                        (a.group and a.group.lower() == grp_name.lower())
                        or (a.asset_type and a.asset_type.lower() == grp_name.lower())
                    ) else 0.0
                    for a in model.assets
                ])
                if np.any(grp_mask > 0):
                    constraints.append({
                        "type": "ineq",
                        "fun": lambda w, m=grp_mask, c=cap_val: c - np.dot(m, w),
                        "jac": lambda w, m=grp_mask: -m,
                    })

        # Deterministic initial guess: equal weights capped at budget
        w0 = np.full(n, min(0.5 / max(1, n), max_exp))

        res = minimize(
            fun=objective,
            x0=w0,
            method="SLSQP",
            jac=obj_grad,
            bounds=bounds,
            constraints=constraints,
            options={"maxiter": 500, "ftol": 1e-7, "disp": False},
        )

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        w_opt = np.maximum(0.0, res.x)
        w_opt = np.round(w_opt, 5)

        # Scale weights to equivalent discrete metric evaluation
        capital_used = float(np.sum(w_opt) * model.deployable_capital)
        unspent_cash = max(0.0, model.deployable_capital - capital_used)
        weighted_return = float(np.dot(model.mu, w_opt) + 0.03 * (1.0 - np.sum(w_opt)))
        portfolio_risk = float(np.sqrt(max(0.0, np.dot(w_opt, np.dot(model.Sigma, w_opt)))))
        liquidity = float((1.0 - np.sum(w_opt)) + np.dot(model.liquidity_vec, w_opt))
        obj_val = float(np.dot(model.mu, w_opt) - model.risk_aversion * np.dot(w_opt, np.dot(model.Sigma, w_opt)))

        # Feasibility check
        violations = []
        if np.sum(w_opt) > 1.0 + 1e-4:
            violations.append(f"Budget exceeded: sum of continuous weights = {np.sum(w_opt):.4f} > 1.0")
        if liquidity < model.min_liquidity_ratio - 1e-4:
            violations.append(f"Liquidity deficit: {liquidity*100:.1f}% < {model.min_liquidity_ratio*100:.1f}%")

        allocation = {name: float(w_opt[i]) for i, name in enumerate(model.asset_names)}

        return OptimizationResult(
            method="slsqp",
            status="optimal" if res.success and len(violations) == 0 else "feasible" if len(violations) == 0 else "infeasible",
            allocation=allocation,
            objective_value=obj_val,
            portfolio_return=weighted_return,
            portfolio_risk=portfolio_risk,
            liquidity=liquidity,
            capital_used=capital_used,
            feasible=len(violations) == 0,
            constraint_violations=violations,
            runtime_ms=elapsed_ms,
            metadata={
                "scipy_success": bool(res.success),
                "iterations": int(res.nit),
                "is_continuous": True,
                "solver_message": str(res.message),
            },
        )

