"""FastAPI router for QUANTA Optimization Engine."""

from __future__ import annotations
import logging
from typing import Any, Dict, List
from fastapi import APIRouter, HTTPException, status
from fastapi.responses import JSONResponse

from app.config import settings
from app.models.financial_problem import FinancialProblem
from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import (
    OptimizationRequest,
    OptimizationResult,
    OptimizationCompareRequest,
    OptimizationCompareResponse,
    BenchmarkLabRequest,
    BenchmarkLabResponse,
)
from app.classical.exact import ExactSolver
from app.classical.greedy import GreedySolver
from app.classical.simulated_annealing import SimulatedAnnealingSolver
from app.classical.continuous import ContinuousSLSQPSolver
from app.classical.milp import MILPSolver
from app.qubo.builder import QUBOBuilder
from app.quantum.simulator import QAOAStatevectorSimulator
from app.benchmarks.generator import generate_synthetic_problem

logger = logging.getLogger("optimization_engine")
router = APIRouter(tags=["Optimization Engine"])


def _build_model(problem: FinancialProblem, candidate_subset: Any) -> OptimizationModel:
    """Helper to instantiate OptimizationModel with candidate subset."""
    return OptimizationModel(problem=problem, candidate_subset=candidate_subset)


# ---------------------------------------------------------------------------
# 1. Classical Optimization Endpoint
# ---------------------------------------------------------------------------
@router.post(
    "/api/v1/optimization/classical",
    response_model=OptimizationResult,
    summary="Execute Classical Optimization Solver",
)
@router.post(
    "/api/v1/classical/optimize",
    response_model=OptimizationResult,
    include_in_schema=False,
)
async def solve_classical(req: OptimizationRequest):
    """Solve capital allocation using exact, greedy, simulated annealing, continuous SLSQP, or MILP."""
    method = (req.method or "exact").lower()
    model = _build_model(req.problem, req.candidate_subset)

    if method == "exact":
        solver = ExactSolver(max_assets=settings.MAX_QUBITS_CAP)
        return solver.solve(model)
    elif method == "greedy":
        solver = GreedySolver()
        return solver.solve(model)
    elif method in ("sa", "simulated_annealing"):
        seed = int(req.options.get("seed", 42))
        max_iter = int(req.options.get("max_iterations", 1500))
        solver = SimulatedAnnealingSolver(seed=seed, max_iterations=max_iter)
        return solver.solve(model, seed=seed)
    elif method in ("slsqp", "continuous"):
        solver = ContinuousSLSQPSolver()
        return solver.solve(model)
    elif method == "milp":
        solver = MILPSolver()
        return solver.solve(model)
    else:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=f"Unknown classical optimization method '{method}'. Valid options: exact, greedy, sa, slsqp, milp.",
        )


# ---------------------------------------------------------------------------
# 2. Quantum Optimization Endpoint (QAOA)
# ---------------------------------------------------------------------------
@router.post(
    "/api/v1/optimization/quantum",
    response_model=OptimizationResult,
    summary="Execute QAOA Quantum Simulation",
)
@router.post(
    "/api/v1/quantum/qaoa",
    response_model=OptimizationResult,
    include_in_schema=False,
)
async def solve_quantum(req: OptimizationRequest):
    """Execute QAOA quantum circuit simulation on classical statevector backend."""
    model = _build_model(req.problem, req.candidate_subset)

    p = int(req.options.get("p", 2))
    seed = int(req.options.get("seed", 42))
    shots = int(req.options.get("shots", settings.DEFAULT_QAOA_SHOTS))
    use_slack = bool(req.options.get("use_slack", False))

    # Check total estimated qubits
    builder = QUBOBuilder(discretization_unit=settings.DEFAULT_DISCRETIZATION_UNIT)
    est = builder.estimate_qubits(
        n_assets=model.n,
        deployable_capital=model.deployable_capital,
        min_liquidity=model.min_liquidity_ratio,
        use_slack=use_slack,
    )
    total_q = est["total_qubits"]

    if total_q > settings.MAX_QUBITS_CAP:
        raise HTTPException(
            status_code=status.HTTP_400_BAD_REQUEST,
            detail=(
                f"Requested QAOA simulation requires {total_q} qubits "
                f"(assets: {est['asset_qubits']}, slack: {est.get('slack_qubits', 0)}), "
                f"which strictly exceeds the simulator hard cap of {settings.MAX_QUBITS_CAP} qubits. "
                "Set use_slack=false or reduce asset pool size."
            ),
        )

    qubo = builder.build(model, use_slack=use_slack)

    # Compute exact ground truth for approximation ratio
    exact_score = None
    exact_bitstring = None
    if model.n <= 16:
        exact_solver = ExactSolver()
        exact_res = exact_solver.solve(model)
        if exact_res.feasible:
            exact_score = exact_res.objective_value
            exact_bitstring = "".join(str(int(exact_res.allocation[name])) for name in model.asset_names)

    simulator = QAOAStatevectorSimulator(max_qubits=settings.MAX_QUBITS_CAP)
    try:
        result = simulator.solve(
            model=model,
            qubo=qubo,
            p=p,
            seed=seed,
            shots=shots,
            exact_optimum_score=exact_score,
            exact_optimum_bitstring=exact_bitstring,
            use_slack=use_slack,
        )
        return result
    except ValueError as ve:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(ve))


# ---------------------------------------------------------------------------
# 3. QUBO Formulation Endpoint (Integration contract compatibility)
# ---------------------------------------------------------------------------
@router.post(
    "/api/v1/quantum/qubo",
    summary="Formulate Problem as QUBO Matrix",
)
async def generate_qubo(req: OptimizationRequest):
    """Construct QUBO matrix Q, constant offset, and slack variable mapping."""
    model = _build_model(req.problem, req.candidate_subset)
    use_slack = bool(req.options.get("use_slack", False))
    builder = QUBOBuilder(discretization_unit=settings.DEFAULT_DISCRETIZATION_UNIT)
    qubo = builder.build(model, use_slack=use_slack)

    return {
        "n_assets": model.n,
        "total_qubits": qubo.total_qubits,
        "slack_bits": qubo.n_slack,
        "offset": qubo.offset,
        "penalties": qubo.penalties,
        "matrix_shape": list(qubo.Q.shape),
        "qubo_matrix": qubo.Q.tolist(),
        "slack_details": qubo.slack_details,
    }


# ---------------------------------------------------------------------------
# 4. Solvers Side-by-Side Comparison
# ---------------------------------------------------------------------------
@router.post(
    "/api/v1/optimization/compare",
    response_model=OptimizationCompareResponse,
    summary="Compare Classical and Quantum Solvers on Identical Problem",
)
async def compare_all_solvers(req: OptimizationCompareRequest):
    """Run Exact, Greedy, SA, Continuous SLSQP, and QAOA on the exact same problem."""
    model = _build_model(req.problem, req.candidate_subset)
    opts = req.options or {}
    seed = int(opts.get("seed", 42))
    p = int(opts.get("p", 2))
    shots = int(opts.get("shots", settings.DEFAULT_QAOA_SHOTS))

    results: Dict[str, OptimizationResult] = {}

    # 1. Exact
    exact_solver = ExactSolver(max_assets=settings.MAX_QUBITS_CAP)
    exact_res = exact_solver.solve(model)
    results["exact"] = exact_res

    # 2. Greedy
    greedy_solver = GreedySolver()
    greedy_res = greedy_solver.solve(model)
    results["greedy"] = greedy_res

    # 3. Simulated Annealing
    sa_solver = SimulatedAnnealingSolver(seed=seed)
    sa_res = sa_solver.solve(model, seed=seed)
    results["simulated_annealing"] = sa_res

    # 4. Continuous SLSQP
    slsqp_solver = ContinuousSLSQPSolver()
    slsqp_res = slsqp_solver.solve(model)
    results["slsqp"] = slsqp_res

    # 5. QAOA Simulator
    qaoa_sim = QAOAStatevectorSimulator(max_qubits=settings.MAX_QUBITS_CAP)
    builder = QUBOBuilder(discretization_unit=settings.DEFAULT_DISCRETIZATION_UNIT)
    qubo = builder.build(model, use_slack=False)

    exact_opt_score = exact_res.objective_value if exact_res.feasible else None
    exact_bitstring = None
    if exact_res.feasible:
        exact_bitstring = "".join(str(int(exact_res.allocation[name])) for name in model.asset_names)

    qaoa_res = qaoa_sim.solve(
        model=model,
        qubo=qubo,
        p=p,
        seed=seed,
        shots=shots,
        exact_optimum_score=exact_opt_score,
        exact_optimum_bitstring=exact_bitstring,
        use_slack=False,
    )
    results["qaoa"] = qaoa_res

    # Assemble summary table
    summary_table = []
    best_classical_name = "exact"
    best_classical_obj = -float("inf")

    for m_name, res in results.items():
        if m_name != "qaoa" and res.feasible and res.objective_value > best_classical_obj:
            best_classical_obj = res.objective_value
            best_classical_name = m_name

        summary_table.append({
            "method": m_name,
            "status": res.status,
            "objective_value": round(res.objective_value, 4),
            "portfolio_return": round(res.portfolio_return, 4),
            "portfolio_risk": round(res.portfolio_risk, 4),
            "liquidity": round(res.liquidity, 4),
            "capital_used": round(res.capital_used, 2),
            "feasible": res.feasible,
            "violations_count": len(res.constraint_violations),
            "runtime_ms": round(res.runtime_ms, 2),
        })

    # QAOA approximation ratio
    approx_ratio = None
    if exact_res.feasible and exact_res.objective_value > 1e-6 and qaoa_res.feasible:
        approx_ratio = round(qaoa_res.objective_value / exact_res.objective_value, 4)

    return OptimizationCompareResponse(
        problem_summary={
            "capital": model.total_capital,
            "deployable_capital": model.deployable_capital,
            "debt_reserve": model.mandatory_debt_reserve,
            "n_assets": model.n,
            "min_liquidity": model.min_liquidity_ratio,
            "risk_tolerance": model.risk_tolerance,
        },
        results=results,
        summary_table=summary_table,
        best_classical_method=best_classical_name,
        qaoa_approximation_ratio=approx_ratio,
    )


# ---------------------------------------------------------------------------
# 5. Optimization Lab Benchmarking Endpoint
# ---------------------------------------------------------------------------
@router.post(
    "/api/v1/optimization/benchmark",
    response_model=BenchmarkLabResponse,
    summary="Optimization Lab Synthetic Benchmarking Run",
)
async def run_lab_benchmark(req: BenchmarkLabRequest):
    """Generate a reproducible synthetic problem of size n and compare classical vs QAOA solvers."""
    problem = generate_synthetic_problem(
        n_assets=req.n_assets,
        difficulty=req.difficulty,
        seed=req.seed,
    )
    compare_req = OptimizationCompareRequest(
        problem=problem,
        options={"seed": req.seed, "p": req.qaoa_p},
    )
    comp_resp = await compare_all_solvers(compare_req)

    scientific_summary = {
        "n_assets": req.n_assets,
        "difficulty": req.difficulty,
        "seed": req.seed,
        "qaoa_layers_p": req.qaoa_p,
        "exact_runtime_ms": comp_resp.results["exact"].runtime_ms,
        "qaoa_simulation_runtime_ms": comp_resp.results["qaoa"].runtime_ms,
        "qaoa_approx_ratio": comp_resp.qaoa_approximation_ratio,
        "qaoa_feasible": comp_resp.results["qaoa"].feasible,
        "scientific_conclusion": (
            "QAOA finds near-optimal solutions in simulation; however, classical exact and greedy solvers "
            "exhibit significantly lower runtime on this asset size. No quantum advantage is demonstrated."
        ),
    }

    return BenchmarkLabResponse(
        instance_info={
            "n_assets": req.n_assets,
            "difficulty": req.difficulty,
            "seed": req.seed,
            "capital": problem.capital,
            "asset_names": [a.name for a in problem.assets],
        },
        results=comp_resp.results,
        summary_table=comp_resp.summary_table,
        scientific_summary=scientific_summary,
    )

