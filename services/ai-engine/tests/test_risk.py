"""Unit tests for Risk Engine, Bayesian Reasoning, and Monte Carlo VaR."""

import pytest
import numpy as np
from app.domain.asset import Asset, AssetType
from app.domain.financial_state import FinancialState
from app.domain.liability import Liability
from app.risk.scoring import RiskEngine
from app.risk.bayesian import BayesianRiskNetwork, BayesianEvidence
from app.risk.sampling import MonteCarloRiskSimulator


@pytest.fixture
def asset_catalog():
    return {
        "gov_bond": Asset(id="gov_bond", name="Gov", asset_type=AssetType.GOV_BOND, expected_return=0.07, risk=0.05, liquidity=0.95),
        "equity": Asset(id="equity", name="Equity", asset_type=AssetType.EQUITY, expected_return=0.14, risk=0.40, liquidity=0.80),
        "high_risk": Asset(id="high_risk", name="Growth", asset_type=AssetType.EQUITY, expected_return=0.20, risk=0.55, liquidity=0.60),
    }


def test_deterministic_risk_scoring(asset_catalog):
    engine = RiskEngine()
    
    # Safe diversified state
    safe_state = FinancialState(
        total_capital=100.0,
        cash=20.0,
        allocations={"gov_bond": 60.0, "equity": 20.0},
        reserved_for_debt=0.0
    )
    safe_report = engine.analyze(safe_state, asset_catalog)
    assert safe_report.composite_risk_score >= 0.0
    assert safe_report.composite_risk_score <= 1.0
    assert safe_report.concentration_index_hhi > 0.0

    # High concentration state
    conc_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"equity": 100.0},
        reserved_for_debt=0.0
    )
    conc_report = engine.analyze(conc_state, asset_catalog)
    assert conc_report.composite_risk_score > safe_report.composite_risk_score


def test_risk_engine_tier_classification(asset_catalog):
    engine = RiskEngine()

    # 1. Ultra-Conservative (< 0.10)
    cash_state = FinancialState(
        total_capital=100.0,
        cash=100.0,
        allocations={},
        reserved_for_debt=0.0
    )
    rep_ultra = engine.analyze(cash_state, asset_catalog)
    assert rep_ultra.composite_risk_score < 0.10
    assert rep_ultra.risk_tier == "Ultra-Conservative"

    # 2. Moderate (0.10 <= score < 0.25)
    mod_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"gov_bond": 60.0, "equity": 40.0},
        reserved_for_debt=0.0
    )
    rep_mod = engine.analyze(mod_state, asset_catalog)
    assert 0.10 <= rep_mod.composite_risk_score < 0.25
    assert rep_mod.risk_tier == "Moderate"

    # 3. High Growth / Moderate Risk (0.25 <= score < 0.40)
    high_growth_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"gov_bond": 20.0, "high_risk": 80.0},
        reserved_for_debt=0.0
    )
    rep_high = engine.analyze(high_growth_state, asset_catalog)
    assert 0.25 <= rep_high.composite_risk_score < 0.40
    assert rep_high.risk_tier == "High Growth / Moderate Risk"

    # 4. Aggressive / Elevated Risk (>= 0.40)
    aggressive_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"high_risk": 100.0},
        reserved_for_debt=0.0
    )
    rep_aggr = engine.analyze(aggressive_state, asset_catalog)
    assert rep_aggr.composite_risk_score >= 0.40
    assert rep_aggr.risk_tier == "Aggressive / Elevated Risk"


def test_risk_engine_hhi_diversification(asset_catalog):
    engine = RiskEngine()

    # 100% in a single asset -> HHI = 1.0
    conc_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"equity": 100.0},
        reserved_for_debt=0.0
    )
    rep_conc = engine.analyze(conc_state, asset_catalog)
    assert abs(rep_conc.concentration_index_hhi - 1.0) < 1e-4

    # Split equally between cash and gov_bond -> HHI = 0.5^2 + 0.5^2 = 0.50
    div_state = FinancialState(
        total_capital=100.0,
        cash=50.0,
        allocations={"gov_bond": 50.0},
        reserved_for_debt=0.0
    )
    rep_div = engine.analyze(div_state, asset_catalog)
    assert abs(rep_div.concentration_index_hhi - 0.50) < 1e-4
    assert rep_div.concentration_penalty < rep_conc.concentration_penalty


def test_risk_engine_zero_capital_edge_case(asset_catalog):
    from pydantic import ValidationError

    # Pydantic validates total_capital > 0
    with pytest.raises(ValidationError):
        FinancialState(total_capital=0.0, cash=0.0, allocations={}, reserved_for_debt=0.0)

    # Engine internal defensive guard for total_capital <= 0
    engine = RiskEngine()
    zero_state = FinancialState.model_construct(
        total_capital=0.0,
        cash=0.0,
        allocations={},
        reserved_for_debt=0.0
    )
    report = engine.analyze(zero_state, asset_catalog)
    assert report.composite_risk_score == 0.0
    assert report.risk_tier == "Conservative"
    assert "Zero capital balance." in report.explanations


def test_risk_engine_liquidity_deficit_penalty(asset_catalog):
    # Asset with low liquidity
    illiquid_catalog = {
        "locked_equity": Asset(id="locked_equity", name="Locked", asset_type=AssetType.EQUITY, expected_return=0.10, risk=0.10, liquidity=0.05)
    }
    engine = RiskEngine(min_liquidity_target=0.20)

    # 100% in locked equity -> liquidity = 0.05 < 0.20
    illiquid_state = FinancialState(
        total_capital=100.0,
        cash=0.0,
        allocations={"locked_equity": 100.0},
        reserved_for_debt=0.0
    )
    report = engine.analyze(illiquid_state, illiquid_catalog)
    assert report.liquidity_deficit_penalty > 0.0
    assert any("shortfall" in exp.lower() for exp in report.explanations)


def test_risk_engine_uncovered_leverage_penalty(asset_catalog):
    engine = RiskEngine()
    debt = Liability(id="loan_1", name="Senior Debt", principal=40.0, interest_rate=0.08)

    # Uncovered debt: reserved_for_debt = 0.0
    uncovered_state = FinancialState(
        total_capital=100.0,
        cash=100.0,
        allocations={},
        reserved_for_debt=0.0,
        liabilities=[debt]
    )
    rep_uncovered = engine.analyze(uncovered_state, asset_catalog)
    assert rep_uncovered.leverage_penalty > 0.0

    # Covered debt: reserved_for_debt = 40.0
    covered_state = FinancialState(
        total_capital=100.0,
        cash=60.0,
        allocations={},
        reserved_for_debt=40.0,
        liabilities=[debt]
    )
    rep_covered = engine.analyze(covered_state, asset_catalog)
    assert rep_covered.leverage_penalty == 0.0


def test_risk_engine_unknown_asset_handling(asset_catalog):
    engine = RiskEngine()
    state = FinancialState(
        total_capital=100.0,
        cash=50.0,
        allocations={"gov_bond": 30.0, "unlisted_token": 20.0},
        reserved_for_debt=0.0
    )
    # Should not crash on unknown asset; unlisted_token is omitted from base risk
    report = engine.analyze(state, asset_catalog)
    assert report.composite_risk_score >= 0.0


def test_bayesian_risk_network_inference():
    bayes = BayesianRiskNetwork()
    
    # Unconditional prior inference
    prior_res = bayes.infer()
    assert 0.0 <= prior_res.p_recession <= 1.0
    assert 0.0 <= prior_res.p_liquidity_crisis <= 1.0
    assert prior_res.stress_multiplier >= 1.0

    # Stressed condition: Inflation + Rate Hike
    stressed_res = bayes.infer(BayesianEvidence(inflation_high=True, rate_hike=True))
    assert stressed_res.p_recession > prior_res.p_recession
    assert stressed_res.p_liquidity_crisis > prior_res.p_liquidity_crisis


def test_bayesian_network_all_signals_false():
    bayes = BayesianRiskNetwork()
    prior_res = bayes.infer()

    # When all adverse signals are false (benign economy)
    benign_evidence = BayesianEvidence(inflation_high=False, rate_hike=False, recession_signal=False)
    benign_res = bayes.infer(benign_evidence)

    assert benign_res.p_recession < prior_res.p_recession
    assert benign_res.p_liquidity_crisis < prior_res.p_liquidity_crisis
    assert benign_res.stress_multiplier < prior_res.stress_multiplier


def test_bayesian_network_all_signals_true():
    bayes = BayesianRiskNetwork()
    prior_res = bayes.infer()

    # When all adverse signals are active
    stress_evidence = BayesianEvidence(inflation_high=True, rate_hike=True, recession_signal=True)
    stress_res = bayes.infer(stress_evidence)

    assert stress_res.p_recession > prior_res.p_recession
    assert stress_res.p_liquidity_crisis > prior_res.p_liquidity_crisis
    assert stress_res.stress_multiplier > prior_res.stress_multiplier


def test_bayesian_network_mixed_signals():
    bayes = BayesianRiskNetwork()
    # High inflation with no rate hike
    ev = BayesianEvidence(inflation_high=True, rate_hike=False, recession_signal=None)
    res = bayes.infer(ev)
    assert 0.0 <= res.p_recession <= 1.0
    assert 0.0 <= res.p_liquidity_crisis <= 1.0
    assert res.stress_multiplier >= 1.0


def test_bayesian_network_probability_bounds():
    bayes = BayesianRiskNetwork()
    
    test_cases = [
        None,
        BayesianEvidence(),
        BayesianEvidence(inflation_high=True),
        BayesianEvidence(inflation_high=False),
        BayesianEvidence(rate_hike=True),
        BayesianEvidence(rate_hike=False),
        BayesianEvidence(recession_signal=True),
        BayesianEvidence(recession_signal=False),
        BayesianEvidence(inflation_high=True, rate_hike=True, recession_signal=True),
        BayesianEvidence(inflation_high=False, rate_hike=False, recession_signal=False),
        BayesianEvidence(inflation_high=True, rate_hike=False, recession_signal=True),
    ]

    for ev in test_cases:
        res = bayes.infer(ev)
        assert 0.0 <= res.p_recession <= 1.0
        assert 0.0 <= res.p_liquidity_crisis <= 1.0
        assert 0.0 <= res.p_equity_downturn <= 1.0
        assert 0.0 <= res.p_credit_spread_widening <= 1.0
        assert res.stress_multiplier >= 1.0


def test_monte_carlo_var_simulation(asset_catalog):
    simulator = MonteCarloRiskSimulator(default_scenarios=500, seed=42)
    state = FinancialState(
        total_capital=100.0,
        cash=10.0,
        allocations={"gov_bond": 50.0, "equity": 40.0},
        reserved_for_debt=0.0
    )
    report = simulator.simulate(state, asset_catalog, num_scenarios=500)
    assert report.num_scenarios == 500
    assert report.var_95 >= 0.0
    assert report.cvar_95 >= report.var_95


def test_monte_carlo_deterministic_seed(asset_catalog):
    sim1 = MonteCarloRiskSimulator(default_scenarios=1000, seed=12345)
    sim2 = MonteCarloRiskSimulator(default_scenarios=1000, seed=12345)
    sim3 = MonteCarloRiskSimulator(default_scenarios=1000, seed=99999)

    state = FinancialState(
        total_capital=100.0,
        cash=20.0,
        allocations={"gov_bond": 50.0, "equity": 30.0},
        reserved_for_debt=0.0
    )

    r1 = sim1.simulate(state, asset_catalog, num_scenarios=1000)
    r2 = sim2.simulate(state, asset_catalog, num_scenarios=1000)
    r3 = sim3.simulate(state, asset_catalog, num_scenarios=1000)

    # Identical seeds must match exactly
    assert r1.mean_simulated_return == r2.mean_simulated_return
    assert r1.var_95 == r2.var_95
    assert r1.cvar_95 == r2.cvar_95
    assert r1.var_99 == r2.var_99
    assert r1.cvar_99 == r2.cvar_99

    # Different seeds should exhibit statistical variance
    assert r1.mean_simulated_return != r3.mean_simulated_return


def test_monte_carlo_scenario_counts(asset_catalog):
    sim = MonteCarloRiskSimulator(seed=42)
    state = FinancialState(
        total_capital=100.0,
        cash=30.0,
        allocations={"gov_bond": 70.0},
        reserved_for_debt=0.0
    )

    for n in [500, 1000, 5000]:
        report = sim.simulate(state, asset_catalog, num_scenarios=n)
        assert report.num_scenarios == n


def test_monte_carlo_all_cash_edge_case(asset_catalog):
    sim = MonteCarloRiskSimulator(seed=42)
    cash_state = FinancialState(
        total_capital=100.0,
        cash=100.0,
        allocations={},
        reserved_for_debt=0.0
    )
    report = sim.simulate(cash_state, asset_catalog, num_scenarios=500)
    assert report.mean_simulated_return == 0.03
    assert report.var_95 == 0.0
    assert report.cvar_95 == 0.0
    assert report.worst_case_loss_pct == 0.0
    assert report.probability_of_loss == 0.0


def test_monte_carlo_var_cvar_ordering(asset_catalog):
    sim = MonteCarloRiskSimulator(default_scenarios=2000, seed=42)
    state = FinancialState(
        total_capital=100.0,
        cash=10.0,
        allocations={"equity": 90.0},
        reserved_for_debt=0.0
    )
    report = sim.simulate(state, asset_catalog, num_scenarios=2000)

    # Statistical consistency invariants:
    # 1. 99% VaR must be >= 95% VaR
    assert report.var_99 >= report.var_95
    # 2. CVaR (Expected Shortfall) must be >= VaR at the same confidence level
    assert report.cvar_95 >= report.var_95
    assert report.cvar_99 >= report.var_99
    # 3. Worst-case loss must be >= 99% CVaR
    assert report.worst_case_loss_pct >= report.cvar_99
