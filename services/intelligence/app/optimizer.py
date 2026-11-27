from __future__ import annotations

from decimal import Decimal
import time
import numpy as np
import scipy
from scipy.optimize import Bounds, LinearConstraint, linprog, milp

from app.schemas import (
    OptimizeRequest,
    OptimizeResponse,
    format_money,
    format_rate,
    parse_decimal_str,
)


def run_optimization(req: OptimizeRequest) -> OptimizeResponse:
    started = time.perf_counter()
    investable = float(parse_decimal_str(req.investable_capital, "investable_capital"))
    op_floor = float(parse_decimal_str(req.operating_liquidity_floor, "operating_liquidity_floor"))
    em_floor = float(parse_decimal_str(req.emergency_reserve_floor, "emergency_reserve_floor"))

    diagnostics: list[str] = []
    warnings: list[str] = []

    if req.total_available_cash is not None:
        total_avail = float(parse_decimal_str(req.total_available_cash, "total_available_cash"))
        if total_avail - investable < (op_floor + em_floor) - 1e-6:
            diagnostics.append("investable_capital breaches operating_liquidity_floor + emergency_reserve_floor")
            return OptimizeResponse(
                correlation_id=req.correlation_id,
                status="INFEASIBLE",
                feasible=False,
                solver_used="precheck_liquidity_floor",
                solver_version=scipy.__version__,
                allocations={c.id: "0.0000" for c in req.candidates},
                unallocated_cash=format_money(Decimal(str(investable))),
                constraint_diagnostics=diagnostics,
                runtime_ms=int((time.perf_counter() - started) * 1000),
            )

    n = len(req.candidates)
    # Variables: x_0 .. x_{n-1} (candidate allocations), plus x_n = unallocated_cash
    num_vars = n + 1
    c_obj = np.zeros(num_vars, dtype=float)
    lb = np.zeros(num_vars, dtype=float)
    ub = np.zeros(num_vars, dtype=float)
    integrality = np.zeros(num_vars, dtype=int)
    lot_scales = np.ones(num_vars, dtype=float)

    for idx, cand in enumerate(req.candidates):
        ret = float(parse_decimal_str(cand.expected_return, f"candidates[{idx}].expected_return"))
        min_a = float(parse_decimal_str(cand.min_allocation, f"candidates[{idx}].min_allocation"))
        max_a = float(parse_decimal_str(cand.max_allocation, f"candidates[{idx}].max_allocation"))
        if min_a > max_a:
            diagnostics.append(f"candidate {cand.id}: min_allocation ({min_a}) > max_allocation ({max_a})")
            return OptimizeResponse(
                correlation_id=req.correlation_id,
                status="INFEASIBLE",
                feasible=False,
                solver_used="precheck_bounds",
                solver_version=scipy.__version__,
                allocations={c.id: "0.0000" for c in req.candidates},
                unallocated_cash=format_money(Decimal(str(investable))),
                constraint_diagnostics=diagnostics,
                runtime_ms=int((time.perf_counter() - started) * 1000),
            )

        if req.solver_mode == "milp" and cand.lot_size is not None:
            lot = float(parse_decimal_str(cand.lot_size, f"candidates[{idx}].lot_size"))
            lot_scales[idx] = lot
            integrality[idx] = 1
            lb[idx] = np.ceil((min_a - 1e-9) / lot)
            ub[idx] = np.floor((max_a + 1e-9) / lot)
            c_obj[idx] = -ret * lot
        else:
            lb[idx] = min_a
            ub[idx] = max_a
            c_obj[idx] = -ret

    # Unallocated cash variable x_n
    lb[n] = 0.0
    ub[n] = investable if req.allow_unallocated_cash else 0.0
    c_obj[n] = 0.0

    # Constraint rows:
    # 1) Capital conservation: sum(lot_scale[i] * x_i) + x_n == investable
    a_rows = [lot_scales.copy()]
    lb_rows = [investable]
    ub_rows = [investable]

    # 2) Counterparty caps: sum_{i in cp} lot_scale[i] * x_i <= max(0, max_limit - current_exposure)
    cp_map = {cp.counterparty_id: cp for cp in req.counterparty_limits}
    cp_members: dict[str, list[int]] = {}
    for idx, cand in enumerate(req.candidates):
        if cand.counterparty_id:
            cp_members.setdefault(cand.counterparty_id, []).append(idx)

    for cp_id, idxs in cp_members.items():
        if cp_id not in cp_map:
            warnings.append(f"candidate references counterparty_id={cp_id} with no explicit limit")
            continue
        limit_val = float(parse_decimal_str(cp_map[cp_id].max_exposure_limit, "max_exposure_limit"))
        cur_val = float(parse_decimal_str(cp_map[cp_id].current_exposure, "current_exposure"))
        headroom = limit_val - cur_val
        if headroom < -1e-6:
            diagnostics.append(f"counterparty {cp_id} already exceeds max_exposure_limit")
            return OptimizeResponse(
                correlation_id=req.correlation_id,
                status="INFEASIBLE",
                feasible=False,
                solver_used="precheck_counterparty",
                solver_version=scipy.__version__,
                allocations={c.id: "0.0000" for c in req.candidates},
                unallocated_cash=format_money(Decimal(str(investable))),
                constraint_diagnostics=diagnostics,
                runtime_ms=int((time.perf_counter() - started) * 1000),
            )
        row = np.zeros(num_vars, dtype=float)
        for i in idxs:
            row[i] = lot_scales[i]
        a_rows.append(row)
        lb_rows.append(-np.inf)
        ub_rows.append(max(0.0, headroom))

    a_mat = np.vstack(a_rows)
    b_lb = np.array(lb_rows, dtype=float)
    b_ub = np.array(ub_rows, dtype=float)

    try:
        if req.solver_mode == "milp":
            solver_used = "scipy_milp_highs"
            res = milp(
                c=c_obj,
                integrality=integrality,
                bounds=Bounds(lb, ub),
                constraints=LinearConstraint(a_mat, b_lb, b_ub),
                options={"time_limit": req.timeout_ms / 1000.0},
            )
        else:
            solver_used = "scipy_linprog_highs"
            a_eq = a_mat[0:1]
            b_eq = np.array([investable], dtype=float)
            a_ub = a_mat[1:] if a_mat.shape[0] > 1 else None
            b_ub_lp = b_ub[1:] if len(b_ub) > 1 else None
            res = linprog(
                c=c_obj,
                A_ub=a_ub,
                b_ub=b_ub_lp,
                A_eq=a_eq,
                b_eq=b_eq,
                bounds=list(zip(lb, ub)),
                method="highs",
                options={"time_limit": req.timeout_ms / 1000.0},
            )
    except Exception as exc:
        return OptimizeResponse(
            correlation_id=req.correlation_id,
            status="SOLVER_FAILED",
            feasible=False,
            solver_used=f"scipy_{req.solver_mode}_highs",
            solver_version=scipy.__version__,
            allocations={c.id: "0.0000" for c in req.candidates},
            unallocated_cash=format_money(Decimal(str(investable))),
            constraint_diagnostics=[f"solver raised exception: {exc}"],
            runtime_ms=int((time.perf_counter() - started) * 1000),
        )

    runtime_ms = int((time.perf_counter() - started) * 1000)

    if res.status == 2:
        diagnostics.append(getattr(res, "message", "problem is infeasible"))
        return OptimizeResponse(
            correlation_id=req.correlation_id,
            status="INFEASIBLE",
            feasible=False,
            solver_used=solver_used,
            solver_version=scipy.__version__,
            allocations={c.id: "0.0000" for c in req.candidates},
            unallocated_cash=format_money(Decimal(str(investable))),
            constraint_diagnostics=diagnostics,
            runtime_ms=runtime_ms,
        )

    if not res.success or res.x is None:
        diagnostics.append(getattr(res, "message", f"solver failed with status={res.status}"))
        return OptimizeResponse(
            correlation_id=req.correlation_id,
            status="SOLVER_FAILED",
            feasible=False,
            solver_used=solver_used,
            solver_version=scipy.__version__,
            allocations={c.id: "0.0000" for c in req.candidates},
            unallocated_cash=format_money(Decimal(str(investable))),
            constraint_diagnostics=diagnostics,
            runtime_ms=runtime_ms,
        )

    # Quantize to 4 decimals and route rounding residual to unallocated_cash to preserve exact conservation.
    investable_dec = parse_decimal_str(req.investable_capital, "investable_capital")
    allocs: dict[str, str] = {}
    allocated_sum = Decimal("0.0000")
    for idx, cand in enumerate(req.candidates):
        raw_val = max(0.0, float(res.x[idx] * lot_scales[idx]))
        q_str = format_money(Decimal(str(raw_val)))
        allocs[cand.id] = q_str
        allocated_sum += Decimal(q_str)

    unallocated_dec = investable_dec - allocated_sum
    if unallocated_dec < Decimal("0.0000"):
        unallocated_dec = Decimal("0.0000")

    obj_val = Decimal(str(float(-res.fun)))
    return OptimizeResponse(
        correlation_id=req.correlation_id,
        status="COMPLETED",
        feasible=True,
        solver_used=solver_used,
        solver_version=scipy.__version__,
        allocations=allocs,
        unallocated_cash=format_money(unallocated_dec),
        objective_value=format_rate(obj_val),
        constraint_diagnostics=["all LP/MILP solver constraints satisfied; Go verification required"],
        warnings=warnings,
        runtime_ms=runtime_ms,
        raw_metadata={"highs_status": int(res.status), "nit": int(getattr(res, "nit", 0))},
    )
