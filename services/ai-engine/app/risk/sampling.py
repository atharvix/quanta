"""Monte Carlo scenario sampling, Value-at-Risk (VaR) estimation, and scenario generation."""

from dataclasses import dataclass
from typing import Dict, List, Optional
import numpy as np
from pydantic import BaseModel, Field
from app.domain.asset import Asset
from app.domain.financial_state import FinancialState


class MonteCarloSimulationReport(BaseModel):
    """Statistical simulation report for portfolio loss distribution."""
    num_scenarios: int
    mean_simulated_return: float
    std_simulated_return: float
    var_95: float = Field(..., description="Value at Risk at 95% confidence (annual fraction)")
    cvar_95: float = Field(..., description="Conditional Value at Risk / Expected Shortfall at 95%")
    var_99: float = Field(..., description="Value at Risk at 99% confidence")
    cvar_99: float = Field(..., description="Conditional Value at Risk at 99%")
    worst_case_loss_pct: float
    probability_of_loss: float


class MonteCarloRiskSimulator:
    """Simulates multivariate stochastic returns for a given financial state."""

    def __init__(self, default_scenarios: int = 2000, seed: Optional[int] = 42):
        self.default_scenarios = default_scenarios
        self.seed = seed

    def simulate(
        self,
        state: FinancialState,
        assets_map: Dict[str, Asset],
        num_scenarios: Optional[int] = None
    ) -> MonteCarloSimulationReport:
        """Generate correlated return scenarios and compute parametric / historical VaR."""
        n_sims = num_scenarios or self.default_scenarios
        rng = np.random.default_rng(self.seed)

        active_allocations = [
            (a_id, amt) for a_id, amt in state.allocations.items()
            if a_id in assets_map and amt > 0
        ]
        
        total_cap = state.total_capital
        if not active_allocations or total_cap <= 0:
            # All cash state
            return MonteCarloSimulationReport(
                num_scenarios=n_sims,
                mean_simulated_return=0.03,
                std_simulated_return=0.001,
                var_95=0.0,
                cvar_95=0.0,
                var_99=0.0,
                cvar_99=0.0,
                worst_case_loss_pct=0.0,
                probability_of_loss=0.0
            )

        weights = []
        expected_returns = []
        volatilities = []

        # Cash weight
        cash_weight = (state.cash + state.reserved_for_debt) / total_cap
        weights.append(cash_weight)
        expected_returns.append(0.03)
        volatilities.append(0.002)

        for a_id, amt in active_allocations:
            asset = assets_map[a_id]
            weights.append(amt / total_cap)
            expected_returns.append(asset.expected_return)
            volatilities.append(asset.volatility or (asset.risk * 0.35))

        w = np.array(weights)
        mu = np.array(expected_returns)
        sigma = np.array(volatilities)
        n_assets = len(weights)

        # Build correlation matrix (simplified Equicorrelation with positive cross-asset correlation)
        corr_matrix = np.full((n_assets, n_assets), 0.25)
        np.fill_diagonal(corr_matrix, 1.0)
        # Cash has 0 correlation with risky assets
        corr_matrix[0, :] = 0.0
        corr_matrix[:, 0] = 0.0
        corr_matrix[0, 0] = 1.0

        cov_matrix = np.outer(sigma, sigma) * corr_matrix

        # Generate Gaussian draws
        simulated_returns = rng.multivariate_normal(mu, cov_matrix, size=n_sims)
        portfolio_returns = simulated_returns @ w

        # Loss distribution = -portfolio_returns
        losses = -portfolio_returns

        var_95 = float(np.percentile(losses, 95))
        cvar_95 = float(np.mean(losses[losses >= var_95]))

        var_99 = float(np.percentile(losses, 99))
        cvar_99 = float(np.mean(losses[losses >= var_99]))

        prob_loss = float(np.mean(portfolio_returns < 0.0))
        worst_loss = float(np.max(losses))

        return MonteCarloSimulationReport(
            num_scenarios=n_sims,
            mean_simulated_return=float(np.mean(portfolio_returns)),
            std_simulated_return=float(np.std(portfolio_returns)),
            var_95=max(0.0, var_95),
            cvar_95=max(0.0, cvar_95),
            var_99=max(0.0, var_99),
            cvar_99=max(0.0, cvar_99),
            worst_case_loss_pct=max(0.0, worst_loss),
            probability_of_loss=prob_loss
        )


# ---------------------------------------------------------------------------
# Scenario Generator — 6 Market Regimes
# ---------------------------------------------------------------------------

@dataclass
class _RegimeConfig:
    """Internal configuration for a market regime scenario."""
    return_multiplier: float     # Multiplicative scaling on baseline expected return
    return_shift: float          # Additive macro shock to expected portfolio return
    volatility_multiplier: float # Multiplier applied to portfolio volatility / risk
    liquidity_haircut: float     # Deducted from portfolio liquidity ratio (0-1)
    description: str             # Human-readable regime description


# Regime definitions — calibrated to realistic macroeconomic conditions
_REGIME_CONFIGS: Dict[str, _RegimeConfig] = {
    "normal": _RegimeConfig(
        return_multiplier=1.00,
        return_shift=0.00,
        volatility_multiplier=1.00,
        liquidity_haircut=0.00,
        description="Normal market conditions with baseline return and volatility expectations.",
    ),
    "bullish": _RegimeConfig(
        return_multiplier=1.25,
        return_shift=0.025,
        volatility_multiplier=0.75,
        liquidity_haircut=0.00,
        description="Risk-on bull market: elevated returns, compressed volatility, ample liquidity.",
    ),
    "bearish": _RegimeConfig(
        return_multiplier=0.70,
        return_shift=-0.04,
        volatility_multiplier=1.40,
        liquidity_haircut=0.05,
        description="Risk-off bear market: depressed returns, heightened volatility, modest liquidity strain.",
    ),
    "recession": _RegimeConfig(
        return_multiplier=0.50,
        return_shift=-0.08,
        volatility_multiplier=1.80,
        liquidity_haircut=0.12,
        description="Economic contraction: sharp drawdowns across asset classes, funding markets tighten.",
    ),
    "market_crash": _RegimeConfig(
        return_multiplier=0.20,
        return_shift=-0.22,
        volatility_multiplier=3.00,
        liquidity_haircut=0.25,
        description="Acute crash event: extreme drawdowns, correlations spike to 1, severe liquidity freeze.",
    ),
    "liquidity_crisis": _RegimeConfig(
        return_multiplier=0.60,
        return_shift=-0.06,
        volatility_multiplier=2.20,
        liquidity_haircut=0.40,
        description="Systemic liquidity crisis: credit markets seize, asset fire-sales, funding drought.",
    ),
}

SUPPORTED_REGIMES: List[str] = list(_REGIME_CONFIGS.keys())


class ScenarioResult(BaseModel):
    """Result of evaluating a financial state under a specific market regime."""
    regime: str = Field(..., description="Market regime identifier")
    projected_return: float = Field(..., description="Stress-adjusted projected portfolio return")
    stressed_risk: float = Field(..., description="Stressed composite risk score (0-1 clamped)")
    stressed_liquidity: float = Field(..., description="Post-haircut portfolio liquidity ratio (0-1 clamped)")
    is_solvent: bool = Field(..., description="True if stressed capital remains positive and covers total liabilities")
    is_liquid: bool = Field(..., description="True if stressed_liquidity >= minimum_liquidity constraint")
    explanation: str = Field(..., description="Narrative explanation of regime impact")


class ScenarioGenerator:
    """Evaluates a FinancialState under each of the 6 required market regimes.

    Usage::

        gen = ScenarioGenerator()
        results = gen.generate(state, assets_map)                     # all 6 regimes
        result  = gen.generate(state, assets_map, regime="recession") # single regime
    """

    def generate(
        self,
        state: FinancialState,
        assets_map: Dict[str, Asset],
        regime: Optional[str] = None,
        minimum_liquidity: float = 0.20,
    ) -> List[ScenarioResult]:
        """Evaluate *state* under one or all regimes.

        Args:
            state: The current financial state to stress-test.
            assets_map: Mapping of asset id -> Asset domain object.
            regime: If given, only the named regime is evaluated; otherwise all 6.
            minimum_liquidity: Liquidity floor for ``is_liquid`` flag (default 0.20).

        Returns:
            List of :class:`ScenarioResult` — one entry per evaluated regime.

        Raises:
            ValueError: If *regime* is specified but not in ``SUPPORTED_REGIMES``.
        """
        if regime is not None:
            if regime not in _REGIME_CONFIGS:
                raise ValueError(
                    f"Unknown regime '{regime}'. Supported: {SUPPORTED_REGIMES}"
                )
            regimes_to_run = {regime: _REGIME_CONFIGS[regime]}
        else:
            regimes_to_run = _REGIME_CONFIGS

        has_known_allocs = any(
            a_id in assets_map and amt > 0 for a_id, amt in state.allocations.items()
        )
        has_any_allocs = any(amt > 0 for amt in state.allocations.values())

        if has_known_allocs or not has_any_allocs:
            base_return = state.calculate_expected_return(assets_map)
            base_risk = state.calculate_risk(assets_map)
            base_liquidity = state.calculate_liquidity(assets_map)
        else:
            base_return = state.expected_return
            base_risk = state.risk_score
            base_liquidity = state.liquidity_ratio

        total_debt = sum(l.principal for l in state.liabilities)

        results: List[ScenarioResult] = []
        for regime_name, cfg in regimes_to_run.items():
            if base_return >= 0:
                projected_return = base_return * cfg.return_multiplier + cfg.return_shift
            else:
                projected_return = base_return + cfg.return_shift

            stressed_risk = min(1.0, max(0.0, base_risk * cfg.volatility_multiplier))
            stressed_liquidity = min(1.0, max(0.0, base_liquidity - cfg.liquidity_haircut))

            stressed_capital = state.total_capital * (1.0 + projected_return)
            is_solvent = state.total_capital > 0 and stressed_capital > 0 and stressed_capital >= total_debt
            is_liquid = stressed_liquidity >= minimum_liquidity

            explanation = (
                f"[{regime_name.upper()}] {cfg.description} "
                f"Return multiplier: {cfg.return_multiplier:.2f}x, "
                f"Return shift: {cfg.return_shift:+.1%}, "
                f"Volatility multiplier: {cfg.volatility_multiplier:.2f}x, "
                f"Liquidity haircut: {cfg.liquidity_haircut:.1%}. "
                f"Projected return: {projected_return:.2%}, "
                f"Stressed risk: {stressed_risk:.4f}, "
                f"Stressed liquidity: {stressed_liquidity:.2%} "
                f"({'solvent' if is_solvent else 'INSOLVENT'}, "
                f"{'liquid' if is_liquid else 'BELOW LIQUIDITY FLOOR'})."
            )

            results.append(
                ScenarioResult(
                    regime=regime_name,
                    projected_return=round(projected_return, 6),
                    stressed_risk=round(stressed_risk, 6),
                    stressed_liquidity=round(stressed_liquidity, 6),
                    is_solvent=is_solvent,
                    is_liquid=is_liquid,
                    explanation=explanation,
                )
            )

        return results
