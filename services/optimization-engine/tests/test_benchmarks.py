"""Smoke tests for benchmark generator, runner, and plotting routines."""

import os
import pytest
from app.benchmarks.generator import generate_synthetic_problem
from app.benchmarks.runner import BenchmarkSuite
from app.benchmarks.plots import generate_benchmark_plots


def test_generator():
    problem = generate_synthetic_problem(n_assets=5, difficulty="balanced", seed=42)
    assert len(problem.assets) == 5
    assert problem.capital == 100.0
    assert problem.risk_tolerance == 0.50


def test_benchmark_runner_smoke(tmp_path):
    suite = BenchmarkSuite(
        asset_counts=[4],
        difficulties=["balanced"],
        output_dir=str(tmp_path),
        seed=42,
    )
    records = suite.run_full_suite()
    assert len(records) > 0
    assert any(r["method"] == "Exact (Brute-Force)" for r in records)
    assert any("QAOA" in r["method"] for r in records)

    # Test plotting smoke
    charts = generate_benchmark_plots(records, str(tmp_path))
    assert len(charts) == 4
    for chart in charts:
        assert os.path.exists(chart)

