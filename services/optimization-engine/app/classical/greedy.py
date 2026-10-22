"""Greedy heuristic solver based on risk-adjusted return per unit cost."""

from __future__ import annotations
import time
import numpy as np

from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import OptimizationResult


class GreedySolver:
    """Greedy heuristic solver prioritizing highest risk-adjusted efficiency."""

    def solve(self, model: OptimizationModel) -> OptimizationResult:
        start_time = time.perf_counter()
        n = model.n

        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="greedy",
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
                metadata={"n_assets": n},
            )

        # Compute return-to-risk efficiency per unit cost
        # Efficiency = (mu_i - lambda * sigma_i^2) / c_i
        variance_diag = np.diag(model.Sigma)
        efficiency = (model.mu - model.risk_aversion * variance_diag) / np.maximum(model.c, 1e-4)

        # Sort candidate assets in descending order of efficiency
        ranked_indices = np.argsort(-efficiency)

        # Greedily activate assets if resulting vector remains feasible
        x = np.zeros(n, dtype=np.float64)

        for idx in ranked_indices:
            trial_x = x.copy()
            trial_x[idx] = 1.0

            # If setting trial_x is feasible and doesn't worsen violations
            if model.is_feasible(trial_x):
                x = trial_x

        metrics = model.evaluate_allocation(x)
        violations = model.violation_report(x)
        is_feas = len(violations) == 0
        status = "feasible" if is_feas else "infeasible"

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0
        allocation = {name: float(x[i]) for i, name in enumerate(model.asset_names)}

        return OptimizationResult(
            method="greedy",
            status=status,
            allocation=allocation,
            objective_value=metrics["objective_value"],
            portfolio_return=metrics["portfolio_return"],
            portfolio_risk=metrics["portfolio_risk"],
            liquidity=metrics["liquidity"],
            capital_used=metrics["capital_used"],
            feasible=is_feas,
            constraint_violations=violations,
            runtime_ms=elapsed_ms,
            metadata={"n_assets": n, "ranked_order": [model.asset_names[i] for i in ranked_indices]},
        )

