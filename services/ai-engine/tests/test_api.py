"""Unit and integration tests for FastAPI AI Engine endpoints."""

import pytest
from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)


def test_health_check_endpoint():
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "healthy"
    assert data["service"] == "quanta-ai-engine"


def test_generate_strategy_endpoint():
    payload = {
        "problem": {
            "capital": 100.0,
            "step_size": 10.0,
            "assets": [
                {
                    "id": "gov_bond",
                    "name": "Gov Bonds",
                    "asset_type": "gov_bond",
                    "expected_return": 0.07,
                    "risk": 0.05,
                    "liquidity": 0.95
                },
                {
                    "id": "equity",
                    "name": "Equity",
                    "asset_type": "equity",
                    "expected_return": 0.14,
                    "risk": 0.40,
                    "liquidity": 0.80
                }
            ],
            "liabilities": [
                {
                    "id": "debt_1",
                    "name": "Loan",
                    "principal": 10.0,
                    "interest_rate": 0.08,
                    "priority": 1,
                    "is_mandatory_reserve": True
                }
            ],
            "constraints": [
                {
                    "id": "c_min_liq",
                    "constraint_type": "minimum_liquidity",
                    "parameters": {"threshold": 0.20}
                }
            ],
            "risk_tolerance": 0.50,
            "minimum_liquidity": 0.20
        },
        "algorithm": "astar"
    }
    response = client.post("/api/v1/strategy/generate", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["strategy"]["is_compliant"]
    assert data["search_result"]["success"]
    assert "risk_breakdown" in data
    assert "constraint_audit" in data


def test_constraints_validate_endpoint():
    payload = {
        "state": {
            "total_capital": 100.0,
            "cash": 85.0,
            "allocations": {"gov_bond": 0.0},
            "reserved_for_debt": 15.0,
            "liquidity_ratio": 1.0,
            "risk_score": 0.01,
            "expected_return": 0.03
        },
        "problem": {
            "capital": 100.0,
            "step_size": 10.0,
            "assets": [],
            "liabilities": [],
            "constraints": [
                {
                    "id": "c_min_liq",
                    "constraint_type": "minimum_liquidity",
                    "parameters": {"threshold": 0.20}
                }
            ]
        }
    }
    response = client.post("/api/v1/constraints/validate", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["is_compliant"]
    assert data["total_violations"] == 0


def test_risk_analyze_endpoint_success():
    payload = {
        "state": {
            "total_capital": 100.0,
            "cash": 20.0,
            "allocations": {"gov_bond": 50.0, "equity": 30.0},
            "reserved_for_debt": 0.0,
            "liquidity_ratio": 0.90,
            "risk_score": 0.08,
            "expected_return": 0.09
        },
        "problem": {
            "capital": 100.0,
            "step_size": 10.0,
            "assets": [
                {
                    "id": "gov_bond",
                    "name": "Gov Bonds",
                    "asset_type": "gov_bond",
                    "expected_return": 0.07,
                    "risk": 0.05,
                    "liquidity": 0.95
                },
                {
                    "id": "equity",
                    "name": "Equity",
                    "asset_type": "equity",
                    "expected_return": 0.14,
                    "risk": 0.40,
                    "liquidity": 0.80
                }
            ],
            "liabilities": [],
            "constraints": []
        },
        "evidence": {
            "inflation_high": True,
            "rate_hike": True,
            "recession_signal": False
        },
        "run_monte_carlo": True,
        "num_scenarios": 500
    }
    response = client.post("/api/v1/risk/analyze", json=payload)
    assert response.status_code == 200
    data = response.json()

    assert "risk_breakdown" in data
    assert "composite_risk_score" in data["risk_breakdown"]
    assert "risk_tier" in data["risk_breakdown"]

    assert "bayesian_inference" in data
    assert "stress_multiplier" in data["bayesian_inference"]
    assert "p_recession" in data["bayesian_inference"]

    assert "monte_carlo_report" in data
    assert data["monte_carlo_report"] is not None
    assert data["monte_carlo_report"]["num_scenarios"] == 500
    assert "var_95" in data["monte_carlo_report"]
    assert "cvar_95" in data["monte_carlo_report"]


def test_risk_analyze_endpoint_disabled_monte_carlo():
    payload = {
        "state": {
            "total_capital": 100.0,
            "cash": 40.0,
            "allocations": {"gov_bond": 60.0},
            "reserved_for_debt": 0.0
        },
        "problem": {
            "capital": 100.0,
            "step_size": 10.0,
            "assets": [
                {
                    "id": "gov_bond",
                    "name": "Gov Bonds",
                    "asset_type": "gov_bond",
                    "expected_return": 0.07,
                    "risk": 0.05,
                    "liquidity": 0.95
                }
            ]
        },
        "run_monte_carlo": False
    }
    response = client.post("/api/v1/risk/analyze", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert "risk_breakdown" in data
    assert "bayesian_inference" in data
    assert data["monte_carlo_report"] is None


def test_risk_analyze_endpoint_missing_fields_validation_error():
    # Missing required 'state' and 'problem'
    response = client.post("/api/v1/risk/analyze", json={"run_monte_carlo": True})
    assert response.status_code == 422


def test_adversarial_stress_test_endpoint_success():
    payload = {
        "capital": 30.0,
        "step_size": 10.0,
        "assets": [
            {
                "id": "gov_bond",
                "name": "Gov Bonds",
                "asset_type": "gov_bond",
                "expected_return": 0.07,
                "risk": 0.05,
                "liquidity": 0.95
            },
            {
                "id": "equity",
                "name": "Equity",
                "asset_type": "equity",
                "expected_return": 0.14,
                "risk": 0.40,
                "liquidity": 0.80
            }
        ],
        "risk_tolerance": 0.40,
        "minimum_liquidity": 0.20
    }
    response = client.post("/api/v1/stress-test/adversarial", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert "worst_case_payoff" in data
    assert "best_treasury_action" in data
    assert "worst_market_shock" in data
    assert data["nodes_evaluated"] > 0
    assert data["pruned_branches"] >= 0
    assert data["execution_time_ms"] >= 0.0
    assert "explanation" in data


def test_adversarial_stress_test_endpoint_missing_fields_validation_error():
    # Missing required 'capital' and 'step_size'
    response = client.post("/api/v1/stress-test/adversarial", json={"assets": []})
    assert response.status_code == 422
