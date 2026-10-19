"""Unit and integration tests for FastAPI Optimization Engine endpoints."""

import pytest
from fastapi.testclient import TestClient
from app.main import app

client = TestClient(app)


def test_health_check():
    response = client.get("/health")
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "healthy"
    assert data["service"] == "quanta-optimization-engine"
    assert data["port"] == 8001


def test_classical_optimization_endpoints(sample_problem):
    payload = {
        "problem": sample_problem.model_dump(),
        "method": "exact",
        "options": {},
    }
    response = client.post("/api/v1/optimization/classical", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["method"] == "exact"
    assert data["status"] == "optimal"
    assert data["feasible"] is True
    assert "Gov Bond 1" in data["allocation"]


def test_greedy_and_sa_endpoints(sample_problem):
    for method in ["greedy", "sa", "slsqp", "milp"]:
        payload = {
            "problem": sample_problem.model_dump(),
            "method": method,
            "options": {"seed": 42},
        }
        res = client.post("/api/v1/optimization/classical", json=payload)
        assert res.status_code == 200
        data = res.json()
        assert data["method"] in (method, "simulated_annealing")
        assert data["feasible"] is True


def test_quantum_optimization_endpoint(sample_problem):
    payload = {
        "problem": sample_problem.model_dump(),
        "method": "qaoa",
        "options": {"p": 1, "seed": 42, "shots": 512, "use_slack": False},
    }
    response = client.post("/api/v1/optimization/quantum", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["method"] == "qaoa"
    assert data["feasible"] is True
    assert data["metadata"]["simulation"] is True


def test_compare_endpoint(sample_problem):
    payload = {
        "problem": sample_problem.model_dump(),
        "options": {"p": 1, "seed": 42},
    }
    response = client.post("/api/v1/optimization/compare", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert "exact" in data["results"]
    assert "greedy" in data["results"]
    assert "qaoa" in data["results"]
    assert len(data["summary_table"]) >= 5


def test_benchmark_endpoint():
    payload = {
        "n_assets": 4,
        "difficulty": "balanced",
        "seed": 42,
        "qaoa_p": 1,
    }
    response = client.post("/api/v1/optimization/benchmark", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["instance_info"]["n_assets"] == 4
    assert "exact" in data["results"]
    assert "scientific_summary" in data


def test_too_many_qubits_error():
    # Construct problem with 22 assets to breach 20-qubit cap
    assets = [
        {"name": f"Asset_{i}", "cost": 10.0, "expected_return": 0.08, "risk": 0.1, "liquidity": 0.8}
        for i in range(22)
    ]
    payload = {
        "problem": {
            "capital": 100.0,
            "assets": assets,
            "liabilities": [],
            "constraints": [],
            "risk_tolerance": 0.5,
            "minimum_liquidity": 0.2,
        },
        "options": {"use_slack": False},
    }
    response = client.post("/api/v1/optimization/quantum", json=payload)
    assert response.status_code == 400
    assert "exceeds the simulator hard cap" in response.json()["detail"]


def test_validation_error_422():
    # Missing required field 'capital'
    payload = {
        "problem": {
            "assets": [],
            "risk_tolerance": 0.5,
        }
    }
    response = client.post("/api/v1/optimization/classical", json=payload)
    assert response.status_code == 422


def test_infeasible_problem():
    # Debt reserve (120 Cr) > Capital (100 Cr) -> Deployable capital <= 0
    payload = {
        "problem": {
            "capital": 100.0,
            "assets": [
                {"name": "Bond", "cost": 10.0, "expected_return": 0.08, "risk": 0.05, "liquidity": 0.9}
            ],
            "liabilities": [
                {"name": "Massive Debt", "amount": 120.0, "maturity": 6, "interest_rate": 0.08, "is_mandatory_reserve": True}
            ],
            "constraints": [],
            "risk_tolerance": 0.5,
            "minimum_liquidity": 0.2,
        }
    }
    response = client.post("/api/v1/optimization/classical", json=payload)
    assert response.status_code == 200
    data = response.json()
    assert data["status"] == "infeasible"
    assert data["feasible"] is False
    assert len(data["constraint_violations"]) > 0
