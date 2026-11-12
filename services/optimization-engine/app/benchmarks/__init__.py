"""Benchmarks package for QUANTA Optimization Engine."""

from app.benchmarks.generator import generate_synthetic_problem
from app.benchmarks.runner import BenchmarkSuite
from app.benchmarks.plots import generate_benchmark_plots
from app.benchmarks.reporter import generate_benchmark_markdown_doc, generate_benchmark_and_reports

__all__ = [
    "generate_synthetic_problem",
    "BenchmarkSuite",
    "generate_benchmark_plots",
    "generate_benchmark_markdown_doc",
    "generate_benchmark_and_reports",
]
