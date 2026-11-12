"""Mixed-Integer Linear Programming (MILP) solver using scipy.optimize.milp."""

from __future__ import annotations
import time
import numpy as np
from scipy.optimize import milp, LinearConstraint, Bounds

from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import OptimizationResult


class MILPSolver:
    """Solves binary capital allocation instances using HiGHS branch-and-cut via scipy.optimize.milp."""

    def solve(self, model: OptimizationModel) -> OptimizationResult:
        start_time = time.perf_counter()
        n = model.n

        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="milp",
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

        # Objective: minimize c_obj^T x
        # Effective linear objective: - (mu_i - lambda * sigma_i^2)
        variance_diag = np.diag(model.Sigma)
        c_obj = -(model.mu - model.risk_aversion * variance_diag)

        # Constraints matrix A @ x <= b
        row_list = []
        ub_list = []
        lb_list = []

        # 1. Budget: sum(c_i * x_i) <= C_deploy
        row_list.append(model.c)
        ub_list.append(model.deployable_capital)
        lb_list.append(-np.inf)

        # 2. Liquidity: sum((1 - l_i) * c_i * x_i) <= (1 - min_liquidity) * C_deploy
        row_list.append((1.0 - model.liquidity_vec) * model.c)
        ub_list.append((1.0 - model.min_liquidity_ratio) * model.deployable_capital)
        lb_list.append(-np.inf)

        # 3. Individual asset exposure caps
        if model.max_exposure_pct is not None and model.deployable_capital > 0:
            max_asset_spent = model.max_exposure_pct * model.deployable_capital
            for i in range(n):
                r = np.zeros(n)
                r[i] = model.c[i]
                row_list.append(r)
                ub_list.append(max_asset_spent)
                lb_list.append(-np.inf)

        A = np.array(row_list, dtype=np.float64)
        constraints = LinearConstraint(A, lb=lb_list, ub=ub_list)
        integrality = np.ones(n)  # 1 for integer/binary variables
        bounds = Bounds(lb=np.zeros(n), ub=np.ones(n))

        res = milp(c=c_obj, integrality=integrality, bounds=bounds, constraints=constraints)

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        if res.success and res.x is not None:
            x_sol = np.round(res.x).astype(np.float64)
            metrics = model.evaluate_allocation(x_sol)
            violations = model.violation_report(x_sol)
            is_feas = len(violations) == 0
            allocation = {name: float(x_sol[i]) for i, name in enumerate(model.asset_names)}

            return OptimizationResult(
                method="milp",
                status="optimal" if is_feas else "infeasible",
                allocation=allocation,
                objective_value=metrics["objective_value"],
                portfolio_return=metrics["portfolio_return"],
                portfolio_risk=metrics["portfolio_risk"],
                liquidity=metrics["liquidity"],
                capital_used=metrics["capital_used"],
                feasible=is_feas,
                constraint_violations=violations,
                runtime_ms=elapsed_ms,
                metadata={"status_message": res.status, "n_assets": n},
            )
        else:
            x_zero = np.zeros(n, dtype=np.float64)
            violations = model.violation_report(x_zero)
            return OptimizationResult(
                method="milp",
                status="infeasible",
                allocation={name: 0.0 for name in model.asset_names},
                objective_value=0.0,
                portfolio_return=0.0,
                portfolio_risk=0.0,
                liquidity=1.0,
                capital_used=0.0,
                feasible=False,
                constraint_violations=violations or ["MILP solver proved problem infeasible."],
                runtime_ms=elapsed_ms,
                metadata={"solver_status": getattr(res, "status", -1)},
            )

