"""Pydantic v2 schemas for API requests, results, and lab benchmarks."""

from __future__ import annotations
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field
from app.models.financial_problem import FinancialProblem


class OptimizationResult(BaseModel):
    """Canonical result schema returned by all classical and quantum optimization solvers."""
    method: str = Field(..., description="Optimization algorithm used (e.g. exact, greedy, sa, slsqp, qaoa)")
    status: str = Field(..., description="Solver termination status: optimal | feasible | infeasible | error")
    allocation: Dict[str, float] = Field(..., description="Map of asset name -> selected (0/1) or allocation weight")
    objective_value: float = Field(..., description="Achieved objective function score")
    portfolio_return: float = Field(..., description="Expected annualized portfolio return")
    portfolio_risk: float = Field(..., description="Portfolio risk / variance metric")
    liquidity: float = Field(..., description="Achieved portfolio liquidity fraction")
    capital_used: float = Field(..., description="Total capital deployed in Rs Cr")
    feasible: bool = Field(..., description="True if all hard constraints are satisfied")
    constraint_violations: List[str] = Field(default_factory=list, description="List of violated constraints")
    runtime_ms: float = Field(..., description="Execution wall-clock time in ms (or classical simulation time for QAOA)")
    metadata: Dict[str, Any] = Field(default_factory=dict, description="Algorithm-specific metadata (qubits, p, evals, etc.)")


class OptimizationRequest(BaseModel):
    """API request payload for single-method optimization."""
    problem: FinancialProblem = Field(..., description="Treasury capital allocation problem")
    method: Optional[str] = Field(default="exact", description="Solver method: exact | greedy | sa | slsqp | milp | qaoa")
    options: Dict[str, Any] = Field(default_factory=dict, description="Solver hyperparameters (e.g. p, seed, shots, max_iter)")
    candidate_subset: Optional[List[str]] = Field(default=None, description="Optional subset of asset names to consider")


class OptimizationCompareRequest(BaseModel):
    """API request payload for running all solvers side-by-side."""
    problem: FinancialProblem = Field(..., description="Treasury capital allocation problem")
    options: Dict[str, Any] = Field(default_factory=dict, description="Shared options (e.g. seed, p, shots)")
    candidate_subset: Optional[List[str]] = Field(default=None, description="Optional subset of asset names to consider")


class OptimizationCompareResponse(BaseModel):
    """Side-by-side comparison response across all classical and quantum solvers."""
    problem_summary: Dict[str, Any] = Field(..., description="Summary of problem dimensions and constraints")
    results: Dict[str, OptimizationResult] = Field(..., description="Map of solver name -> OptimizationResult")
    summary_table: List[Dict[str, Any]] = Field(..., description="Tabular comparative metrics")
    best_classical_method: str = Field(..., description="Name of top-performing classical solver")
    qaoa_approximation_ratio: Optional[float] = Field(default=None, description="QAOA objective / Exact classical optimum")
    simulation_note: str = Field(
        default="Scientific note: QAOA runtime represents classical statevector SIMULATION cost on CPU, not real quantum processing unit (QPU) hardware runtime.",
        description="Disclaimer on quantum simulation cost"
    )


class BenchmarkLabRequest(BaseModel):
    """Request to generate synthetic instance of size n and run full benchmark suite."""
    n_assets: int = Field(default=6, ge=2, le=18, description="Number of assets in synthetic universe")
    difficulty: str = Field(default="balanced", description="Problem profile: balanced | loose_budget | tight_liquidity | high_risk_aversion")
    seed: int = Field(default=42, description="Random seed for deterministic problem generation")
    qaoa_p: int = Field(default=2, ge=1, le=5, description="Number of QAOA alternating operator layers")


class BenchmarkLabResponse(BaseModel):
    """Response containing benchmark metrics for the Optimization Lab UI."""
    instance_info: Dict[str, Any] = Field(..., description="Generated synthetic problem attributes")
    results: Dict[str, OptimizationResult] = Field(..., description="Solver results")
    summary_table: List[Dict[str, Any]] = Field(..., description="Comparison metrics table")
    scientific_summary: Dict[str, Any] = Field(..., description="Objective gap, feasibility rate, simulation cost")

