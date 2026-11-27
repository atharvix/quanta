from decimal import Decimal
import pytest
from fastapi.testclient import TestClient

from app.main import app


client = TestClient(app)


def test_health_and_readiness_with_correlation_id():
    r1 = client.get("/health", headers={"X-Correlation-ID": "corr-py-1"})
    assert r1.status_code == 200
    assert r1.headers["X-Correlation-ID"] == "corr-py-1"
    assert r1.json()["status"] == "ok"

    r2 = client.get("/ready")
    assert r2.status_code == 200
    assert r2.json()["status"] == "ready"


def test_forecasting_seasonal_naive_ets_and_backtesting():
    history = [str(Decimal("100.0000") + Decimal(str(i * 5))) for i in range(14)]
    resp = client.post(
        "/internal/v1/forecast",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-fc-1",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "history": history,
            "horizon": 3,
            "season_length": 7,
            "model": "ets",
        },
    )
    assert resp.status_code == 200, resp.text
    data = resp.json()
    assert data["is_authoritative"] is False
    assert data["backtest"]["supported"] is True
    assert data["backtest"]["folds"] >= 1
    assert Decimal(data["backtest"]["mae"]) >= Decimal("0")
    assert len(data["forecasts"]) == 3
    assert data["forecasts"][0]["p10"] is not None
    assert data["forecasts"][0]["p90"] is not None

    # Short history must not fabricate backtest metrics or p10/p90 intervals
    short_resp = client.post(
        "/internal/v1/forecast",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-fc-2",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "history": ["100.0000", None, "110.0000"],
            "horizon": 2,
            "season_length": 7,
            "model": "seasonal_naive",
        },
    )
    assert short_resp.status_code == 200
    short_data = short_resp.json()
    assert short_data["missing_values_count"] == 1
    assert short_data["backtest"]["supported"] is False
    assert short_data["backtest"]["mae"] is None
    assert short_data["forecasts"][0]["p10"] is None


def test_risk_calculations_mathematically_verifiable_and_seeded():
    # Known returns: 20 observations where worst loss is 10% (-0.10) on 1000.0000 -> loss = 100.0000
    returns = ["0.01"] * 18 + ["-0.05", "-0.10"]
    payload = {
        "contract_version": "1.0",
        "correlation_id": "corr-risk-1",
        "treasury_id": "t-1",
        "base_currency": "USD",
        "portfolio_value": "1000.0000",
        "returns_history": returns,
        "confidence_level": "0.95",
        "num_simulations": 2000,
        "seed": 12345,
        "stress_scenarios": [{"name": "liquidity_shock", "shock_pct": "-0.15"}],
    }
    r1 = client.post("/internal/v1/risk/evaluate", json=payload)
    r2 = client.post("/internal/v1/risk/evaluate", json=payload)
    assert r1.status_code == 200, r1.text
    d1, d2 = r1.json(), r2.json()

    assert d1["is_authoritative"] is False
    assert d1["monte_carlo_var"] == d2["monte_carlo_var"]
    assert d1["monte_carlo_cvar"] == d2["monte_carlo_cvar"]
    # Historical CVaR >= Historical VaR
    assert Decimal(d1["historical_cvar"]) >= Decimal(d1["historical_var"])
    # Stress shock -15% on 1000 -> projected_loss = 150.0000, post_shock_value = 850.0000
    assert d1["stress_results"][0]["projected_loss"] == "150.0000"
    assert d1["stress_results"][0]["post_shock_value"] == "850.0000"
    assert d1["anomalies"]["supported"] is True


def test_optimization_lp_milp_feasibility_conservation_and_infeasibility():
    # LP: 1000 investable, CP1 capped at 400 (already 100 used -> headroom 300), CP2 capped at 1000
    lp_resp = client.post(
        "/internal/v1/optimize",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-opt-lp",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "investable_capital": "1000.0000",
            "operating_liquidity_floor": "200.0000",
            "emergency_reserve_floor": "100.0000",
            "total_available_cash": "1500.0000",
            "candidates": [
                {"id": "mmf_a", "expected_return": "0.055", "min_allocation": "0", "max_allocation": "800", "counterparty_id": "cp_1"},
                {"id": "tbill_b", "expected_return": "0.045", "min_allocation": "0", "max_allocation": "800", "counterparty_id": "cp_2"},
            ],
            "counterparty_limits": [
                {"counterparty_id": "cp_1", "max_exposure_limit": "400.0000", "current_exposure": "100.0000"},
                {"counterparty_id": "cp_2", "max_exposure_limit": "1000.0000", "current_exposure": "0.0000"},
            ],
            "solver_mode": "lp",
        },
    )
    assert lp_resp.status_code == 200, lp_resp.text
    lp_data = lp_resp.json()
    assert lp_data["status"] == "COMPLETED"
    assert lp_data["feasible"] is True
    assert lp_data["allocations"]["mmf_a"] == "300.0000"
    assert lp_data["allocations"]["tbill_b"] == "700.0000"
    assert lp_data["unallocated_cash"] == "0.0000"
    total = Decimal(lp_data["allocations"]["mmf_a"]) + Decimal(lp_data["allocations"]["tbill_b"]) + Decimal(lp_data["unallocated_cash"])
    assert total == Decimal("1000.0000")

    # MILP with lot_size=250.0000
    milp_resp = client.post(
        "/internal/v1/optimize",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-opt-milp",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "investable_capital": "600.0000",
            "operating_liquidity_floor": "100.0000",
            "emergency_reserve_floor": "100.0000",
            "candidates": [
                {"id": "lot_asset", "expected_return": "0.05", "min_allocation": "0", "max_allocation": "600", "lot_size": "250.0000"},
            ],
            "solver_mode": "milp",
            "allow_unallocated_cash": True,
        },
    )
    assert milp_resp.status_code == 200
    milp_data = milp_resp.json()
    assert milp_data["status"] == "COMPLETED"
    assert milp_data["allocations"]["lot_asset"] == "500.0000"
    assert milp_data["unallocated_cash"] == "100.0000"

    # Infeasible when allow_unallocated_cash=False and max_allocation < investable_capital
    inf_resp = client.post(
        "/internal/v1/optimize",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-opt-inf",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "investable_capital": "1000.0000",
            "operating_liquidity_floor": "100.0000",
            "emergency_reserve_floor": "100.0000",
            "candidates": [
                {"id": "small_cap", "expected_return": "0.04", "min_allocation": "0", "max_allocation": "200.0000"},
            ],
            "solver_mode": "lp",
            "allow_unallocated_cash": False,
        },
    )
    assert inf_resp.status_code == 200
    assert inf_resp.json()["status"] == "INFEASIBLE"
    assert inf_resp.json()["feasible"] is False


def test_invalid_inputs_return_422_structured_error():
    bad = client.post(
        "/internal/v1/optimize",
        json={
            "contract_version": "1.0",
            "correlation_id": "corr-bad",
            "treasury_id": "t-1",
            "base_currency": "USD",
            "investable_capital": "-50.0000",
            "operating_liquidity_floor": "100.0000",
            "emergency_reserve_floor": "100.0000",
            "candidates": [{"id": "a", "expected_return": "0.05", "max_allocation": "100"}],
        },
    )
    assert bad.status_code == 422
    assert bad.json()["error"]["code"] == "INVALID_CONTRACT_PAYLOAD"
