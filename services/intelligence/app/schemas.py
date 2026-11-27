from __future__ import annotations

from decimal import Decimal, InvalidOperation, ROUND_HALF_EVEN
from typing import Any, Literal

from pydantic import BaseModel, ConfigDict, Field, field_validator


CONTRACT_VERSION: Literal["1.0"] = "1.0"
MODEL_SERVICE_VERSION = "0.2.0"


def parse_decimal_str(value: Any, field_name: str) -> Decimal:
    if isinstance(value, bool):
        raise ValueError(f"{field_name} must be a decimal string, not bool")
    if isinstance(value, float):
        raise ValueError(f"{field_name} must be a decimal string, not float")
    try:
        d = Decimal(str(value).strip())
    except (InvalidOperation, ValueError) as exc:
        raise ValueError(f"{field_name} is not a valid decimal: {value!r}") from exc
    if not d.is_finite():
        raise ValueError(f"{field_name} must be finite")
    return d


def format_money(d: Decimal) -> str:
    return str(d.quantize(Decimal("0.0001"), rounding=ROUND_HALF_EVEN))


def format_rate(d: Decimal) -> str:
    return str(d.quantize(Decimal("0.000001"), rounding=ROUND_HALF_EVEN))


class StrictModel(BaseModel):
    model_config = ConfigDict(extra="forbid", str_strip_whitespace=True)


# --- Forecasting Contracts ---


class ForecastRequest(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str = Field(min_length=1)
    treasury_id: str = Field(min_length=1)
    base_currency: str = Field(min_length=1, max_length=10)
    history: list[str | None] = Field(min_length=1)
    horizon: int = Field(ge=1, le=90)
    season_length: int = Field(default=7, ge=1, le=30)
    model: Literal["seasonal_naive", "ets"] = "ets"

    @field_validator("history")
    @classmethod
    def validate_history(cls, v: list[str | None]) -> list[str | None]:
        for idx, item in enumerate(v):
            if item is not None:
                parse_decimal_str(item, f"history[{idx}]")
        return v


class ForecastPoint(StrictModel):
    step: int
    p10: str | None = None
    p50: str
    p90: str | None = None


class BacktestMetrics(StrictModel):
    supported: bool
    folds: int = 0
    mae: str | None = None
    rmse: str | None = None
    coverage_80: str | None = None
    reason: str | None = None


class ForecastResponse(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str
    model_name: str
    model_version: str
    is_authoritative: Literal[False] = False
    input_checks_passed: bool
    missing_values_count: int
    forecasts: list[ForecastPoint]
    backtest: BacktestMetrics
    warnings: list[str] = Field(default_factory=list)


# --- Risk Contracts ---


class StressScenarioInput(StrictModel):
    name: str = Field(min_length=1, max_length=64)
    shock_pct: str

    @field_validator("shock_pct")
    @classmethod
    def validate_shock(cls, v: str) -> str:
        d = parse_decimal_str(v, "shock_pct")
        if d < Decimal("-1.0") or d > Decimal("1.0"):
            raise ValueError("shock_pct must be between -1.0 and 1.0")
        return str(d)


class RiskRequest(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str = Field(min_length=1)
    treasury_id: str = Field(min_length=1)
    base_currency: str = Field(min_length=1, max_length=10)
    portfolio_value: str
    returns_history: list[str] = Field(min_length=2)
    confidence_level: str = "0.95"
    num_simulations: int = Field(default=2000, ge=100, le=50000)
    seed: int = Field(default=42, ge=0, le=2**31 - 1)
    stress_scenarios: list[StressScenarioInput] = Field(default_factory=list)

    @field_validator("portfolio_value")
    @classmethod
    def validate_portfolio_value(cls, v: str) -> str:
        d = parse_decimal_str(v, "portfolio_value")
        if d <= 0:
            raise ValueError("portfolio_value must be positive")
        return format_money(d)

    @field_validator("returns_history")
    @classmethod
    def validate_returns(cls, v: list[str]) -> list[str]:
        for idx, item in enumerate(v):
            parse_decimal_str(item, f"returns_history[{idx}]")
        return v

    @field_validator("confidence_level")
    @classmethod
    def validate_conf(cls, v: str) -> str:
        d = parse_decimal_str(v, "confidence_level")
        if d <= Decimal("0.5") or d >= Decimal("1.0"):
            raise ValueError("confidence_level must be strictly between 0.5 and 1.0")
        return str(d)


class StressScenarioResult(StrictModel):
    name: str
    shock_pct: str
    projected_loss: str
    post_shock_value: str


class AnomalyReport(StrictModel):
    supported: bool
    flagged_indices: list[int] = Field(default_factory=list)
    reason: str | None = None


class RiskResponse(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str
    model_name: str
    model_version: str
    seed: int
    is_authoritative: Literal[False] = False
    input_checks_passed: bool
    confidence_level: str
    historical_var: str
    historical_cvar: str
    parametric_var: str
    parametric_cvar: str
    monte_carlo_var: str
    monte_carlo_cvar: str
    stress_results: list[StressScenarioResult]
    anomalies: AnomalyReport
    assumptions: list[str]
    warnings: list[str] = Field(default_factory=list)


# --- Optimization Contracts ---


class AllocationCandidateInput(StrictModel):
    id: str = Field(min_length=1, max_length=64)
    expected_return: str
    min_allocation: str = "0.0000"
    max_allocation: str
    counterparty_id: str | None = None
    lot_size: str | None = None

    @field_validator("expected_return")
    @classmethod
    def validate_ret(cls, v: str) -> str:
        parse_decimal_str(v, "expected_return")
        return v

    @field_validator("min_allocation", "max_allocation")
    @classmethod
    def validate_bounds(cls, v: str) -> str:
        d = parse_decimal_str(v, "allocation_bound")
        if d < 0:
            raise ValueError("allocation bounds must be non-negative")
        return format_money(d)

    @field_validator("lot_size")
    @classmethod
    def validate_lot(cls, v: str | None) -> str | None:
        if v is not None:
            d = parse_decimal_str(v, "lot_size")
            if d <= 0:
                raise ValueError("lot_size must be positive")
            return format_money(d)
        return None


class CounterpartyLimitInput(StrictModel):
    counterparty_id: str = Field(min_length=1)
    max_exposure_limit: str
    current_exposure: str = "0.0000"

    @field_validator("max_exposure_limit", "current_exposure")
    @classmethod
    def validate_non_neg(cls, v: str) -> str:
        d = parse_decimal_str(v, "counterparty_amount")
        if d < 0:
            raise ValueError("counterparty limit/exposure must be non-negative")
        return format_money(d)


class OptimizeRequest(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str = Field(min_length=1)
    treasury_id: str = Field(min_length=1)
    base_currency: str = Field(min_length=1, max_length=10)
    investable_capital: str
    operating_liquidity_floor: str
    emergency_reserve_floor: str
    total_available_cash: str | None = None
    candidates: list[AllocationCandidateInput] = Field(min_length=1)
    counterparty_limits: list[CounterpartyLimitInput] = Field(default_factory=list)
    solver_mode: Literal["lp", "milp"] = "lp"
    allow_unallocated_cash: bool = True
    timeout_ms: int = Field(default=10000, ge=100, le=60000)

    @field_validator("investable_capital", "operating_liquidity_floor", "emergency_reserve_floor")
    @classmethod
    def validate_money_field(cls, v: str) -> str:
        d = parse_decimal_str(v, "monetary_field")
        if d < 0:
            raise ValueError("monetary amounts must be non-negative")
        return format_money(d)

    @field_validator("total_available_cash")
    @classmethod
    def validate_avail(cls, v: str | None) -> str | None:
        if v is not None:
            d = parse_decimal_str(v, "total_available_cash")
            if d < 0:
                raise ValueError("total_available_cash must be non-negative")
            return format_money(d)
        return None


class OptimizeResponse(StrictModel):
    contract_version: Literal["1.0"] = CONTRACT_VERSION
    correlation_id: str
    status: Literal["COMPLETED", "INFEASIBLE", "SOLVER_FAILED"]
    feasible: bool
    solver_used: str
    solver_version: str
    is_authoritative: Literal[False] = False
    authoritative_verification_required: Literal[True] = True
    allocations: dict[str, str]
    unallocated_cash: str
    objective_value: str | None = None
    constraint_diagnostics: list[str] = Field(default_factory=list)
    warnings: list[str] = Field(default_factory=list)
    runtime_ms: int
    raw_metadata: dict[str, Any] = Field(default_factory=dict)
