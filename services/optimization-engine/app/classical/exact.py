"""Exact solver using exhaustive enumeration / branch-and-bound for n <= 20."""

from __future__ import annotations
import time
from typing import Dict, Optional
import numpy as np

from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import OptimizationResult


class ExactSolver:
    """Exact ground-truth solver for discrete capital allocation instances."""

    def __init__(self, max_assets: int = 20):
        self.max_assets = max_assets

    def solve(self, model: OptimizationModel) -> OptimizationResult:
        start_time = time.perf_counter()
        n = model.n

        # Check inherent problem infeasibility
        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="exact",
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
                metadata={"total_states_evaluated": 0, "n_assets": n},
            )

        if n > self.max_assets:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="exact",
                status="error",
                allocation={name: 0.0 for name in model.asset_names},
                objective_value=0.0,
                portfolio_return=0.0,
                portfolio_risk=0.0,
                liquidity=0.0,
                capital_used=0.0,
                feasible=False,
                constraint_violations=[f"Problem size n={n} exceeds exact solver limit of {self.max_assets} assets."],
                runtime_ms=elapsed_ms,
                metadata={"n_assets": n},
            )

        best_score = -float("inf")
        best_x: Optional[np.ndarray] = None
        best_metrics: Optional[Dict] = None
        feasible_count = 0
        total_states = 1 << n

        # For n <= 16, vectorized evaluation is extremely fast
        # Generate all 2^n bit combinations
        for idx in range(total_states):
            # Extract bits
            x = np.array([(idx >> j) & 1 for j in range(n)], dtype=np.float64)
            if model.is_feasible(x):
                feasible_count += 1
                metrics = model.evaluate_allocation(x)
                score = metrics["objective_value"]
                if score > best_score:
                    best_score = score
                    best_x = x
                    best_metrics = metrics

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        if best_x is not None and best_metrics is not None:
            allocation = {name: float(best_x[i]) for i, name in enumerate(model.asset_names)}
            return OptimizationResult(
                method="exact",
                status="optimal",
                allocation=allocation,
                objective_value=best_metrics["objective_value"],
                portfolio_return=best_metrics["portfolio_return"],
                portfolio_risk=best_metrics["portfolio_risk"],
                liquidity=best_metrics["liquidity"],
                capital_used=best_metrics["capital_used"],
                feasible=True,
                constraint_violations=[],
                runtime_ms=elapsed_ms,
                metadata={
                    "total_states_evaluated": total_states,
                    "feasible_states_count": feasible_count,
                    "n_assets": n,
                },
            )
        else:
            # No feasible state found
            x_zero = np.zeros(n, dtype=np.float64)
            violations = model.violation_report(x_zero)
            allocation = {name: 0.0 for name in model.asset_names}
            return OptimizationResult(
                method="exact",
                status="infeasible",
                allocation=allocation,
                objective_value=0.0,
                portfolio_return=0.0,
                portfolio_risk=0.0,
                liquidity=1.0,
                capital_used=0.0,
                feasible=False,
                constraint_violations=violations or ["No combination of assets satisfies all constraints simultaneously."],
                runtime_ms=elapsed_ms,
                metadata={
                    "total_states_evaluated": total_states,
                    "feasible_states_count": 0,
                    "n_assets": n,
                },
            )

