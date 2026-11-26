"""FastAPI route handlers for QUANTA AI Engine."""

from enum import Enum
from typing import Any, Dict, List, Optional
from fastapi import APIRouter, HTTPException, Query, status
from pydantic import BaseModel, Field

from app.domain.financial_problem import FinancialProblem, FinancialAllocationProblem
from app.domain.financial_state import FinancialState
from app.domain.constraint import ConstraintEvaluationResult
from app.domain.strategy import Strategy
from app.search.base import SearchResult
from app.search.bfs import BreadthFirstSearch
from app.search.dfs import DepthFirstSearch
from app.search.generate_and_test import GenerateAndTest
from app.search.hill_climbing import HillClimbingSearch
from app.search.best_first import BestFirstSearch
from app.search.astar import AStarSearch
from app.risk.scoring import RiskEngine, RiskFactorBreakdown
from app.risk.bayesian import BayesianRiskNetwork, BayesianEvidence, BayesianRiskInferenceResult
from app.risk.sampling import MonteCarloRiskSimulator, MonteCarloSimulationReport, ScenarioGenerator, ScenarioResult
from app.game.alpha_beta import AlphaBetaPruningSolver, AdversarialStressTestResult
from app.mdp.environment import TreasuryMDPEnvironment, PolicyEvaluationReport
from app.agents.decision_agent import DecisionAgent, DeliberationSummary


router = APIRouter(prefix="/api/v1")


class SearchAlgorithmEnum(str, Enum):
    BFS = "bfs"
    DFS = "dfs"
    GENERATE_AND_TEST = "generate_and_test"
    HILL_CLIMBING = "hill_climbing"
    BEST_FIRST = "best_first"
    ASTAR = "astar"


class StrategyGenerateRequest(BaseModel):
    problem: FinancialProblem
    algorithm: SearchAlgorithmEnum = SearchAlgorithmEnum.ASTAR


class StrategyGenerateResponse(BaseModel):
    strategy: Strategy
    search_result: SearchResult
    risk_breakdown: RiskFactorBreakdown
    constraint_audit: List[ConstraintEvaluationResult]


class RiskAnalyzeRequest(BaseModel):
    state: FinancialState
    problem: FinancialProblem
    evidence: Optional[BayesianEvidence] = None
    run_monte_carlo: bool = True
    num_scenarios: int = 1000


class RiskAnalyzeResponse(BaseModel):
    risk_breakdown: RiskFactorBreakdown
    bayesian_inference: BayesianRiskInferenceResult
    monte_carlo_report: Optional[MonteCarloSimulationReport] = None
    scenario_summary: Optional[List[ScenarioResult]] = None


class ConstraintsValidateRequest(BaseModel):
    state: FinancialState
    problem: FinancialProblem


class ConstraintsValidateResponse(BaseModel):
    is_compliant: bool
    total_violations: int
    evaluations: List[ConstraintEvaluationResult]


class SearchCompareResponse(BaseModel):
    problem_summary: Dict[str, Any]
    results: Dict[str, SearchResult]


@router.post("/strategy/generate", response_model=StrategyGenerateResponse)
async def generate_strategy(request: StrategyGenerateRequest):
    """Synthesize optimal capital allocation strategy using deterministic AI search."""
    try:
        search_problem = FinancialAllocationProblem(request.problem)
        
        # Select search algorithm
        if request.algorithm == SearchAlgorithmEnum.BFS:
            solver = BreadthFirstSearch()
        elif request.algorithm == SearchAlgorithmEnum.DFS:
            solver = DepthFirstSearch()
        elif request.algorithm == SearchAlgorithmEnum.GENERATE_AND_TEST:
            solver = GenerateAndTest()
        elif request.algorithm == SearchAlgorithmEnum.HILL_CLIMBING:
            solver = HillClimbingSearch()
        elif request.algorithm == SearchAlgorithmEnum.BEST_FIRST:
            solver = BestFirstSearch()
        else:
            solver = AStarSearch()

        search_result = solver.search(search_problem)
        final_state = search_result.final_state or search_problem.initial_state

        # Risk and constraint audits
        risk_engine = RiskEngine()
        risk_breakdown = risk_engine.analyze(final_state, search_problem.assets_map)
        evaluations = search_problem.evaluate_constraints(final_state)
        is_compliant = search_problem.is_state_compliant(final_state)

        # Build explainable strategy
        strategy = Strategy(
            actions=search_result.actions,
            total_allocated=sum(final_state.allocations.values()),
            expected_return=final_state.expected_return,
            risk_score=final_state.risk_score,
            liquidity_ratio=final_state.liquidity_ratio,
            objective_value=search_result.objective_value,
            is_compliant=is_compliant,
            explanation=f"Allocated via {search_result.algorithm} with expected return {final_state.expected_return*100:.2f}% and risk {final_state.risk_score:.4f}.",
            metadata={"algorithm": search_result.algorithm, "states_explored": search_result.states_explored}
        )

        return StrategyGenerateResponse(
            strategy=strategy,
            search_result=search_result,
            risk_breakdown=risk_breakdown,
            constraint_audit=evaluations
        )
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/risk/analyze", response_model=RiskAnalyzeResponse)
async def analyze_risk(request: RiskAnalyzeRequest):
    """Perform multi-factor risk decomposition, Bayesian shock reasoning, and Monte Carlo VaR."""
    try:
        search_problem = FinancialAllocationProblem(request.problem)
        
        # 1. Deterministic Multi-Factor Scoring
        risk_engine = RiskEngine()
        risk_breakdown = risk_engine.analyze(request.state, search_problem.assets_map)

        # 2. Bayesian Inference
        bayes_net = BayesianRiskNetwork()
        bayes_result = bayes_net.infer(request.evidence)

        # 3. Monte Carlo Simulation
        mc_report = None
        if request.run_monte_carlo:
            simulator = MonteCarloRiskSimulator()
            mc_report = simulator.simulate(
                request.state, search_problem.assets_map, num_scenarios=request.num_scenarios
            )

        # 4. Scenario Generator — all 6 regimes
        gen = ScenarioGenerator()
        scenario_summary = gen.generate(
            request.state,
            search_problem.assets_map,
            minimum_liquidity=request.problem.minimum_liquidity,
        )

        return RiskAnalyzeResponse(
            risk_breakdown=risk_breakdown,
            bayesian_inference=bayes_result,
            monte_carlo_report=mc_report,
            scenario_summary=scenario_summary,
        )
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/constraints/validate", response_model=ConstraintsValidateResponse)
async def validate_constraints(request: ConstraintsValidateRequest):
    """Audit a financial state against corporate constraints and return explainable messages."""
    try:
        search_problem = FinancialAllocationProblem(request.problem)
        evaluations = search_problem.evaluate_constraints(request.state)
        is_compliant = search_problem.is_state_compliant(request.state)
        violations = sum(1 for e in evaluations if not e.satisfied)

        return ConstraintsValidateResponse(
            is_compliant=is_compliant,
            total_violations=violations,
            evaluations=evaluations
        )
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/search/run", response_model=SearchResult)
async def run_search(request: StrategyGenerateRequest):
    """Execute a specific AI search algorithm directly on the financial problem."""
    try:
        search_problem = FinancialAllocationProblem(request.problem)
        if request.algorithm == SearchAlgorithmEnum.BFS:
            solver = BreadthFirstSearch()
        elif request.algorithm == SearchAlgorithmEnum.DFS:
            solver = DepthFirstSearch()
        elif request.algorithm == SearchAlgorithmEnum.GENERATE_AND_TEST:
            solver = GenerateAndTest()
        elif request.algorithm == SearchAlgorithmEnum.HILL_CLIMBING:
            solver = HillClimbingSearch()
        elif request.algorithm == SearchAlgorithmEnum.BEST_FIRST:
            solver = BestFirstSearch()
        else:
            solver = AStarSearch()

        return solver.search(search_problem)
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/search/compare", response_model=SearchCompareResponse)
async def compare_all_searches(problem: FinancialProblem):
    """Execute all search algorithms side-by-side to benchmark performance and outcomes."""
    try:
        search_problem = FinancialAllocationProblem(problem)
        solvers = {
            "bfs": BreadthFirstSearch(),
            "dfs": DepthFirstSearch(),
            "generate_and_test": GenerateAndTest(),
            "hill_climbing": HillClimbingSearch(),
            "best_first": BestFirstSearch(),
            "astar": AStarSearch(),
        }

        results = {}
        for key, solver in solvers.items():
            results[key] = solver.search(search_problem)

        return SearchCompareResponse(
            problem_summary={
                "capital": problem.capital,
                "num_assets": len(problem.assets),
                "risk_tolerance": problem.risk_tolerance,
                "minimum_liquidity": problem.minimum_liquidity
            },
            results=results
        )
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/agents/evaluate", response_model=DeliberationSummary)
async def evaluate_agents(problem: FinancialProblem):
    """Run full multi-agent council deliberation (Treasury, Risk, Compliance, Decision)."""
    try:
        search_problem = FinancialAllocationProblem(problem)
        decision_agent = DecisionAgent()
        return decision_agent.synthesize_strategy(search_problem)
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/stress-test/adversarial", response_model=AdversarialStressTestResult)
@router.post("/stress-test", response_model=AdversarialStressTestResult)
async def adversarial_stress_test(problem: FinancialProblem):
    """Execute Alpha-Beta pruning game search against worst-case macroeconomic shock adversary.

    Also accessible at ``/stress-test`` as a convenience alias.
    """
    try:
        search_problem = FinancialAllocationProblem(problem)
        solver = AlphaBetaPruningSolver()
        return solver.stress_test(search_problem)
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))


@router.post("/mdp/solve", response_model=PolicyEvaluationReport)
async def solve_mdp(gamma: float = Query(0.95, ge=0.5, le=0.99)):
    """Compute optimal multi-period dynamic rebalancing policy via Bellman Value Iteration."""
    try:
        env = TreasuryMDPEnvironment(gamma=gamma)
        return env.value_iteration()
    except Exception as e:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(e))
