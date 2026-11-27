"""Alpha-Beta Pruning adversarial search for stress-testing treasury decisions."""

import time
from typing import Dict, List, Optional, Tuple
from pydantic import BaseModel, Field
from app.game.game_state import TreasuryGameState, MarketShockAction
from app.game.minimax import MARKET_SHOCK_ACTIONS
from app.domain.financial_problem import FinancialAllocationProblem
from app.domain.strategy import FinancialAction


class AdversarialStressTestResult(BaseModel):
    """Outcome of adversarial stress game evaluation."""
    worst_case_payoff: float
    best_treasury_action: Optional[str]
    worst_market_shock: str
    nodes_evaluated: int
    pruned_branches: int
    execution_time_ms: float
    explanation: str


class AlphaBetaPruningSolver:
    """Alpha-Beta Pruning search over Treasury actions vs Market adversary shocks."""

    def __init__(self, max_depth: int = 4):
        self.max_depth = max_depth
        self.nodes_evaluated = 0
        self.pruned_branches = 0

    def alpha_beta(
        self,
        node: TreasuryGameState,
        problem: FinancialAllocationProblem,
        depth: int,
        alpha: float,
        beta: float,
        is_maximizing: bool
    ) -> float:
        """Recursive alpha-beta pruning."""
        self.nodes_evaluated += 1

        if depth >= self.max_depth or node.is_terminal(self.max_depth):
            return node.evaluate_payoff(problem.risk_tolerance)

        if is_maximizing:
            max_eval = -float("inf")
            succs = problem.successors(node.financial_state)
            if not succs:
                return node.evaluate_payoff(problem.risk_tolerance)

            for act, next_st in succs:
                child = TreasuryGameState(
                    financial_state=next_st,
                    assets_map=node.assets_map,
                    is_max_turn=False,
                    active_shock=node.active_shock,
                    depth=depth + 1
                )
                eval_val = self.alpha_beta(child, problem, depth + 1, alpha, beta, is_maximizing=False)
                max_eval = max(max_eval, eval_val)
                alpha = max(alpha, eval_val)
                if beta <= alpha:
                    self.pruned_branches += 1
                    break  # Beta cut-off
            return max_eval
        else:
            min_eval = float("inf")
            for shock in MARKET_SHOCK_ACTIONS:
                child = TreasuryGameState(
                    financial_state=node.financial_state,
                    assets_map=node.assets_map,
                    is_max_turn=True,
                    active_shock=shock,
                    depth=depth + 1
                )
                eval_val = self.alpha_beta(child, problem, depth + 1, alpha, beta, is_maximizing=True)
                min_eval = min(min_eval, eval_val)
                beta = min(beta, eval_val)
                if beta <= alpha:
                    self.pruned_branches += 1
                    break  # Alpha cut-off
            return min_eval

    def stress_test(self, problem: FinancialAllocationProblem) -> AdversarialStressTestResult:
        """Find the minimax robust first action for Treasury against worst-case adversary shock."""
        start_time = time.perf_counter()
        self.nodes_evaluated = 0
        self.pruned_branches = 0

        root_game = TreasuryGameState(
            financial_state=problem.initial_state,
            assets_map=problem.assets_map,
            is_max_turn=True,
            depth=0
        )

        succs = problem.successors(problem.initial_state)
        if not succs:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return AdversarialStressTestResult(
                worst_case_payoff=0.0,
                best_treasury_action=None,
                worst_market_shock="None",
                nodes_evaluated=1,
                pruned_branches=0,
                execution_time_ms=elapsed_ms,
                explanation="No initial actions available."
            )

        best_action: Optional[FinancialAction] = None
        best_next_st = None
        best_val = -float("inf")
        alpha = -float("inf")
        beta = float("inf")

        for act, next_st in succs:
            child = TreasuryGameState(
                financial_state=next_st,
                assets_map=problem.assets_map,
                is_max_turn=False,
                depth=1
            )
            val = self.alpha_beta(child, problem, depth=1, alpha=alpha, beta=beta, is_maximizing=False)
            if val > best_val:
                best_val = val
                best_action = act
                best_next_st = next_st
            alpha = max(alpha, val)

        # Identify actual worst-case shock directly against the chosen defensive state
        worst_shock_name = MARKET_SHOCK_ACTIONS[0].shock_name
        if best_next_st is not None:
            worst_shock_val = float("inf")
            for shock in MARKET_SHOCK_ACTIONS:
                shock_node = TreasuryGameState(
                    financial_state=best_next_st,
                    assets_map=problem.assets_map,
                    is_max_turn=True,
                    active_shock=shock,
                    depth=2,
                )
                shock_val = shock_node.evaluate_payoff(problem.risk_tolerance)
                if shock_val < worst_shock_val:
                    worst_shock_val = shock_val
                    worst_shock_name = shock.shock_name

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        explanation = (
            f"Adversarial search evaluated {self.nodes_evaluated} nodes with {self.pruned_branches} pruned branches. "
            f"Worst-case payoff under adversary shock is {best_val:.4f}, selecting action '{repr(best_action)}'."
        )

        return AdversarialStressTestResult(
            worst_case_payoff=best_val,
            best_treasury_action=repr(best_action),
            worst_market_shock=worst_shock_name,
            nodes_evaluated=self.nodes_evaluated,
            pruned_branches=self.pruned_branches,
            execution_time_ms=elapsed_ms,
            explanation=explanation
        )
