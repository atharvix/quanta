"""Unit tests for Game Theory (Minimax, Alpha-Beta) and MDP Policy Solver."""

import pytest
from app.domain.asset import Asset, AssetType
from app.domain.financial_problem import FinancialProblem, FinancialAllocationProblem
from app.domain.financial_state import FinancialState
from app.game.game_state import TreasuryGameState, MarketShockAction
from app.game.minimax import MinimaxSolver, MARKET_SHOCK_ACTIONS
from app.game.alpha_beta import AlphaBetaPruningSolver, AdversarialStressTestResult
from app.mdp.environment import TreasuryMDPEnvironment


@pytest.fixture
def mini_problem():
    assets = [
        Asset(id="gov_bond", name="Gov", asset_type=AssetType.GOV_BOND, expected_return=0.07, risk=0.05, liquidity=0.95),
        Asset(id="equity", name="Equity", asset_type=AssetType.EQUITY, expected_return=0.14, risk=0.40, liquidity=0.80),
    ]
    prob = FinancialProblem(
        capital=30.0,
        step_size=10.0,
        assets=assets,
        liabilities=[],
        constraints=[]
    )
    return FinancialAllocationProblem(prob)


def test_alpha_beta_adversarial_stress_test(mini_problem):
    solver = AlphaBetaPruningSolver(max_depth=3)
    stress_report = solver.stress_test(mini_problem)
    assert stress_report.nodes_evaluated > 0
    assert stress_report.worst_case_payoff != 0.0
    assert stress_report.best_treasury_action is not None


def test_minimax_and_alpha_beta_consistency(mini_problem):
    """Verify that Alpha-Beta pruning returns the exact same minimax payoff as exhaustive Minimax."""
    root_node = TreasuryGameState(
        financial_state=mini_problem.initial_state,
        assets_map=mini_problem.assets_map,
        is_max_turn=True,
        depth=0
    )

    mm_solver = MinimaxSolver(max_depth=3)
    mm_payoff = mm_solver.minimax(root_node, mini_problem, depth=0, is_maximizing=True)

    ab_solver = AlphaBetaPruningSolver(max_depth=3)
    ab_payoff = ab_solver.alpha_beta(
        root_node, mini_problem, depth=0,
        alpha=-float("inf"), beta=float("inf"),
        is_maximizing=True
    )

    # Core theorem: Alpha-Beta pruning produces identical game value to Minimax
    assert abs(mm_payoff - ab_payoff) < 1e-6
    # Alpha-Beta must evaluate less than or equal nodes to exhaustive Minimax
    assert ab_solver.nodes_evaluated <= mm_solver.nodes_evaluated


def test_alpha_beta_depth_scaling(mini_problem):
    """Verify node count scaling as search depth increases."""
    s1 = AlphaBetaPruningSolver(max_depth=1)
    r1 = s1.stress_test(mini_problem)

    s2 = AlphaBetaPruningSolver(max_depth=2)
    r2 = s2.stress_test(mini_problem)

    s3 = AlphaBetaPruningSolver(max_depth=3)
    r3 = s3.stress_test(mini_problem)

    assert r1.nodes_evaluated <= r2.nodes_evaluated <= r3.nodes_evaluated
    assert r1.best_treasury_action is not None
    assert r2.best_treasury_action is not None
    assert r3.best_treasury_action is not None


def test_alpha_beta_empty_successors_edge_case():
    """Verify graceful handling when no actions can be taken from initial state."""
    assets = [
        Asset(id="gov_bond", name="Gov", asset_type=AssetType.GOV_BOND, expected_return=0.07, risk=0.05, liquidity=0.95),
    ]
    # step_size (20.0) > capital (10.0), so 0 successors available
    prob = FinancialProblem(
        capital=10.0,
        step_size=20.0,
        assets=assets,
        liabilities=[],
        constraints=[]
    )
    alloc_prob = FinancialAllocationProblem(prob)
    solver = AlphaBetaPruningSolver(max_depth=3)
    result = solver.stress_test(alloc_prob)

    assert result.worst_case_payoff == 0.0
    assert result.best_treasury_action is None
    assert result.nodes_evaluated == 1
    assert "No initial actions available" in result.explanation


def test_adversarial_stress_test_response_contract(mini_problem):
    """Verify that AdversarialStressTestResult strictly adheres to expected schema contracts."""
    solver = AlphaBetaPruningSolver(max_depth=3)
    result = solver.stress_test(mini_problem)

    assert isinstance(result, AdversarialStressTestResult)
    assert isinstance(result.worst_case_payoff, float)
    assert isinstance(result.best_treasury_action, str)
    assert isinstance(result.worst_market_shock, str)
    assert isinstance(result.nodes_evaluated, int) and result.nodes_evaluated > 0
    assert isinstance(result.pruned_branches, int) and result.pruned_branches >= 0
    assert isinstance(result.execution_time_ms, float) and result.execution_time_ms >= 0.0
    assert isinstance(result.explanation, str) and len(result.explanation) > 0


def test_mdp_value_iteration():
    env = TreasuryMDPEnvironment(gamma=0.90)
    report = env.value_iteration(max_iterations=100)
    assert report.iterations_to_converge > 0
    assert len(report.optimal_policy) == len(env.all_states)
    assert len(report.state_values) == len(env.all_states)


def test_evaluate_payoff_uses_asset_type_not_literal_key():
    """evaluate_payoff must aggregate shock losses by asset_type, not by literal allocation key.

    Assets are keyed by custom IDs ('bluechip_equity', 'corp_bond_aaa') that do NOT match the
    old hard-coded lookup keys ('equity', 'corp_bond'). Both equity_drawdown and credit_spread_spike
    must apply accurately, and missing asset IDs must fall back gracefully without crashing.
    """
    assets = [
        Asset(id="bluechip_equity", name="Bluechip Equity", asset_type=AssetType.EQUITY,
              expected_return=0.12, risk=0.35, liquidity=0.80),
        Asset(id="corp_bond_aaa", name="AAA Corp Bond", asset_type=AssetType.CORP_BOND,
              expected_return=0.06, risk=0.08, liquidity=0.90),
    ]
    assets_map = {a.id: a for a in assets}

    stressed_state = FinancialState(
        total_capital=100.0,
        cash=10.0,
        allocations={"bluechip_equity": 50.0, "corp_bond_aaa": 35.0, "unlisted_asset": 5.0},
        reserved_for_debt=0.0,
        expected_return=0.084,
        liquidity_ratio=0.815,
        risk_score=0.20,
    )

    equity_only_shock = MarketShockAction(
        shock_name="Equity Only",
        equity_drawdown=-0.30,
        credit_spread_spike=0.0,
        liquidity_freeze=0.0,
    )
    bond_only_shock = MarketShockAction(
        shock_name="Credit Only",
        equity_drawdown=0.0,
        credit_spread_spike=-0.10,
        liquidity_freeze=0.0,
    )
    combined_shock = MarketShockAction(
        shock_name="Equity Crash + Credit Squeeze",
        equity_drawdown=-0.30,
        credit_spread_spike=-0.10,
        liquidity_freeze=0.15,
    )
    zero_shock = MarketShockAction(shock_name="None")

    base_payoff = TreasuryGameState(stressed_state, assets_map, is_max_turn=False, active_shock=zero_shock).evaluate_payoff(0.50)
    eq_payoff = TreasuryGameState(stressed_state, assets_map, is_max_turn=False, active_shock=equity_only_shock).evaluate_payoff(0.50)
    bond_payoff = TreasuryGameState(stressed_state, assets_map, is_max_turn=False, active_shock=bond_only_shock).evaluate_payoff(0.50)
    comb_payoff = TreasuryGameState(stressed_state, assets_map, is_max_turn=False, active_shock=combined_shock).evaluate_payoff(0.50)

    # Exact shock deductions: 50*0.30/100 = 0.15 for equity, 35*0.10/100 = 0.035 for corp bond
    assert eq_payoff == pytest.approx(base_payoff - 0.15, abs=1e-6)
    assert bond_payoff == pytest.approx(base_payoff - 0.035, abs=1e-6)
    assert comb_payoff == pytest.approx(base_payoff - 0.185 - 0.20 * 0.15, abs=1e-6)
    assert comb_payoff < 0.0

    # Negative liquidity_freeze (as defined in MARKET_SHOCK_ACTIONS) must also reduce payoff, not add liquidity
    neg_liq_shock = MarketShockAction(
        shock_name="Negative Liquidity Squeeze",
        equity_drawdown=0.0,
        credit_spread_spike=0.0,
        liquidity_freeze=-0.25,
    )
    neg_liq_payoff = TreasuryGameState(stressed_state, assets_map, is_max_turn=False, active_shock=neg_liq_shock).evaluate_payoff(0.50)
    assert neg_liq_payoff == pytest.approx(base_payoff - 0.20 * 0.25, abs=1e-6)


def test_alpha_beta_computes_actual_worst_shock():
    """Verify worst_market_shock picks the genuine damaging shock, never Status Quo."""
    assets = [
        Asset(id="gov_bond", name="Gov", asset_type=AssetType.GOV_BOND, expected_return=0.07, risk=0.05, liquidity=0.95),
        Asset(id="equity", name="Equity", asset_type=AssetType.EQUITY, expected_return=0.14, risk=0.40, liquidity=0.80),
    ]
    prob = FinancialProblem(capital=20.0, step_size=10.0, assets=assets, liabilities=[], constraints=[])
    solver = AlphaBetaPruningSolver()
    res = solver.stress_test(FinancialAllocationProblem(prob))
    assert res.worst_market_shock in {s.shock_name for s in MARKET_SHOCK_ACTIONS}
    assert res.worst_market_shock != "Status Quo"
