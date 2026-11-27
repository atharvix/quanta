from __future__ import annotations

from decimal import Decimal
import math
import numpy as np
from scipy.optimize import minimize
from scipy.stats import norm

from app.schemas import (
    BacktestMetrics,
    ForecastPoint,
    ForecastRequest,
    ForecastResponse,
    MODEL_SERVICE_VERSION,
    format_money,
    format_rate,
    parse_decimal_str,
)


def _clean_history(raw: list[str | None]) -> tuple[np.ndarray, int, list[str]]:
    """Convert decimal string history with explicit missing-value handling (linear interpolation / carry)."""
    warnings: list[str] = []
    missing_indices = [i for i, v in enumerate(raw) if v is None]
    missing_count = len(missing_indices)
    if missing_count == len(raw):
        raise ValueError("history contains only missing values")

    values = np.array(
        [float(parse_decimal_str(v, f"history[{i}]")) if v is not None else np.nan for i, v in enumerate(raw)],
        dtype=float,
    )
    if missing_count > 0:
        valid_idx = np.where(~np.isnan(values))[0]
        all_idx = np.arange(len(values))
        values = np.interp(all_idx, valid_idx, values[valid_idx])
        warnings.append(f"interpolated {missing_count} missing observations using linear/edge carry")

    return values, missing_count, warnings


def _seasonal_naive_forecast(series: np.ndarray, horizon: int, season_length: int) -> np.ndarray:
    n = len(series)
    if n < season_length or season_length <= 1:
        return np.full(horizon, series[-1], dtype=float)
    last_season = series[-season_length:]
    return np.array([last_season[h % season_length] for h in range(horizon)], dtype=float)


def _fit_ets(series: np.ndarray) -> tuple[float, float, np.ndarray, np.ndarray]:
    """Fit Holt's linear exponential smoothing (additive trend) by minimizing one-step SSE."""
    n = len(series)
    if n < 3:
        fitted = np.full(n, series[0], dtype=float)
        return 0.5, 0.0, fitted, series - fitted

    def simulate(alpha: float, beta: float) -> tuple[np.ndarray, float, float]:
        level = series[0]
        trend = series[1] - series[0]
        fitted = np.empty(n, dtype=float)
        fitted[0] = level
        for t in range(1, n):
            forecast_t = level + trend
            fitted[t] = forecast_t
            prev_level = level
            level = alpha * series[t] + (1.0 - alpha) * (level + trend)
            trend = beta * (level - prev_level) + (1.0 - beta) * trend
        return fitted, level, trend

    def loss(params: np.ndarray) -> float:
        alpha, beta = float(params[0]), float(params[1])
        fitted, _, _ = simulate(alpha, beta)
        err = series[1:] - fitted[1:]
        return float(np.sum(err * err))

    res = minimize(loss, x0=np.array([0.4, 0.1]), bounds=[(0.05, 0.95), (0.01, 0.5)], method="L-BFGS-B")
    alpha_opt, beta_opt = float(res.x[0]), float(res.x[1])
    fitted, last_level, last_trend = simulate(alpha_opt, beta_opt)
    residuals = series[1:] - fitted[1:]
    return last_level, last_trend, residuals, fitted


def _ets_forecast(series: np.ndarray, horizon: int) -> tuple[np.ndarray, np.ndarray]:
    if len(series) < 3:
        mean_val = float(np.mean(series))
        return np.full(horizon, mean_val, dtype=float), np.array([], dtype=float)
    level, trend, residuals, _ = _fit_ets(series)
    steps = np.arange(1, horizon + 1, dtype=float)
    point = level + steps * trend
    return point, residuals


def _rolling_backtest(
    series: np.ndarray,
    model: str,
    season_length: int,
    horizon: int,
) -> BacktestMetrics:
    min_train = max(4, season_length + 1) if model == "seasonal_naive" else 5
    eval_horizon = min(horizon, 3)
    if len(series) < min_train + eval_horizon:
        return BacktestMetrics(
            supported=False,
            reason=f"insufficient history ({len(series)} points) for rolling out-of-time backtest (need >= {min_train + eval_horizon})",
        )

    errors: list[float] = []
    covered = 0
    interval_total = 0
    folds = 0
    z80 = float(norm.ppf(0.90))

    for origin in range(min_train, len(series) - eval_horizon + 1):
        train = series[:origin]
        actual = series[origin : origin + eval_horizon]
        if model == "seasonal_naive":
            pred = _seasonal_naive_forecast(train, eval_horizon, season_length)
            resid = train[1:] - train[:-1] if len(train) > 1 else np.array([])
        else:
            pred, resid = _ets_forecast(train, eval_horizon)

        folds += 1
        for h_idx in range(eval_horizon):
            errors.append(float(actual[h_idx] - pred[h_idx]))
            if len(resid) >= 4:
                sigma = float(np.std(resid, ddof=1))
                half_width = z80 * sigma * math.sqrt(h_idx + 1)
                interval_total += 1
                if (pred[h_idx] - half_width) <= actual[h_idx] <= (pred[h_idx] + half_width):
                    covered += 1

    arr_err = np.array(errors, dtype=float)
    mae = Decimal(str(float(np.mean(np.abs(arr_err)))))
    rmse = Decimal(str(float(np.sqrt(np.mean(arr_err * arr_err)))))
    cov_str = format_rate(Decimal(str(covered / interval_total))) if interval_total > 0 else None

    return BacktestMetrics(
        supported=True,
        folds=folds,
        mae=format_money(mae),
        rmse=format_money(rmse),
        coverage_80=cov_str,
    )


def run_forecast(req: ForecastRequest) -> ForecastResponse:
    series, missing_count, warnings = _clean_history(req.history)
    if req.model == "seasonal_naive":
        point = _seasonal_naive_forecast(series, req.horizon, req.season_length)
        residuals = series[1:] - series[:-1] if len(series) > 1 else np.array([])
        model_name = "seasonal_naive"
    else:
        point, residuals = _ets_forecast(series, req.horizon)
        model_name = "ets_holt_linear"
        if len(series) < 3:
            warnings.append("fewer than 3 observations; fell back to sample mean without trend")

    sigma: float | None = None
    if len(residuals) >= 4:
        sigma = float(np.std(residuals, ddof=1))
    else:
        warnings.append("fewer than 4 residuals; p10/p90 intervals omitted rather than fabricated")

    z80 = float(norm.ppf(0.90))
    points: list[ForecastPoint] = []
    for idx, val in enumerate(point, start=1):
        p50 = format_money(Decimal(str(float(val))))
        if sigma is not None:
            hw = z80 * sigma * math.sqrt(idx)
            p10 = format_money(Decimal(str(float(val - hw))))
            p90 = format_money(Decimal(str(float(val + hw))))
        else:
            p10 = None
            p90 = None
        points.append(ForecastPoint(step=idx, p10=p10, p50=p50, p90=p90))

    backtest = _rolling_backtest(series, req.model, req.season_length, req.horizon)

    return ForecastResponse(
        correlation_id=req.correlation_id,
        model_name=model_name,
        model_version=MODEL_SERVICE_VERSION,
        input_checks_passed=True,
        missing_values_count=missing_count,
        forecasts=points,
        backtest=backtest,
        warnings=warnings,
    )
