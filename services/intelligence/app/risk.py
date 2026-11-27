from __future__ import annotations

from decimal import Decimal
import numpy as np
from scipy.stats import norm
from sklearn.ensemble import IsolationForest

from app.schemas import (
    AnomalyReport,
    MODEL_SERVICE_VERSION,
    RiskRequest,
    RiskResponse,
    StressScenarioResult,
    format_money,
    parse_decimal_str,
)


def _var_cvar_from_returns(returns: np.ndarray, portfolio_value: float, confidence: float) -> tuple[Decimal, Decimal]:
    """Compute positive loss VaR and CVaR at `confidence` (left-tail loss convention)."""
    losses = -returns * portfolio_value
    var_val = float(np.quantile(losses, confidence))
    tail_losses = losses[losses >= var_val]
    cvar_val = float(np.mean(tail_losses)) if len(tail_losses) > 0 else var_val
    return Decimal(str(max(0.0, var_val))), Decimal(str(max(0.0, cvar_val)))


def run_risk_analysis(req: RiskRequest) -> RiskResponse:
    portfolio_dec = parse_decimal_str(req.portfolio_value, "portfolio_value")
    portfolio_val = float(portfolio_dec)
    conf = float(parse_decimal_str(req.confidence_level, "confidence_level"))

    returns = np.array([float(parse_decimal_str(r, f"returns_history[{i}]")) for i, r in enumerate(req.returns_history)], dtype=float)
    warnings: list[str] = []

    # 1. Historical VaR / CVaR
    hist_var, hist_cvar = _var_cvar_from_returns(returns, portfolio_val, conf)

    # 2. Parametric VaR / CVaR (Normal assumption)
    mu = float(np.mean(returns))
    sigma = float(np.std(returns, ddof=1))
    alpha = 1.0 - conf
    z_conf = float(norm.ppf(conf))
    param_var_rate = max(0.0, -mu + sigma * z_conf)
    param_cvar_rate = max(0.0, -mu + sigma * (float(norm.pdf(z_conf)) / alpha))
    param_var = Decimal(str(param_var_rate * portfolio_val))
    param_cvar = Decimal(str(param_cvar_rate * portfolio_val))

    # 3. Deterministic Monte Carlo Simulation
    rng = np.random.default_rng(req.seed)
    sim_returns = rng.normal(loc=mu, scale=max(sigma, 1e-12), size=req.num_simulations)
    mc_var, mc_cvar = _var_cvar_from_returns(sim_returns, portfolio_val, conf)

    # 4. Explicit Stress Scenarios
    stress_results: list[StressScenarioResult] = []
    for sc in req.stress_scenarios:
        shock = parse_decimal_str(sc.shock_pct, "shock_pct")
        post_val = portfolio_dec * (Decimal("1") + shock)
        loss = max(Decimal("0"), portfolio_dec - post_val)
        stress_results.append(
            StressScenarioResult(
                name=sc.name,
                shock_pct=str(shock),
                projected_loss=format_money(loss),
                post_shock_value=format_money(max(Decimal("0"), post_val)),
            )
        )

    # 5. Isolation Forest Anomaly Detection
    if len(returns) >= 12:
        iso = IsolationForest(random_state=req.seed, contamination="auto")
        preds = iso.fit_predict(returns.reshape(-1, 1))
        flagged = [int(i) for i, label in enumerate(preds) if label == -1]
        anomalies = AnomalyReport(supported=True, flagged_indices=flagged)
    else:
        anomalies = AnomalyReport(
            supported=False,
            reason=f"insufficient history ({len(returns)} points) for IsolationForest (need >= 12)",
        )
        warnings.append("anomaly detection skipped due to insufficient return history (< 12 points)")

    return RiskResponse(
        correlation_id=req.correlation_id,
        model_name="historical_parametric_mc_risk",
        model_version=MODEL_SERVICE_VERSION,
        seed=req.seed,
        input_checks_passed=True,
        confidence_level=req.confidence_level,
        historical_var=format_money(hist_var),
        historical_cvar=format_money(hist_cvar),
        parametric_var=format_money(param_var),
        parametric_cvar=format_money(param_cvar),
        monte_carlo_var=format_money(mc_var),
        monte_carlo_cvar=format_money(mc_cvar),
        stress_results=stress_results,
        anomalies=anomalies,
        assumptions=[
            "Positive VaR/CVaR values represent projected loss magnitude in base_currency.",
            "Parametric and Monte Carlo VaR/CVaR assume i.i.d. normal returns parameterized by sample mean and sample std (ddof=1).",
            f"Monte Carlo simulation uses numpy PCG64 default_rng(seed={req.seed}) with {req.num_simulations} draws.",
        ],
        warnings=warnings,
    )
