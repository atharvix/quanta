"""Publication-quality, black-and-white friendly visualization generator for benchmark results."""

from __future__ import annotations
import os
from typing import Any, Dict, List
import numpy as np
import matplotlib
matplotlib.use("Agg")  # Non-interactive headless backend
import matplotlib.pyplot as plt


def generate_benchmark_plots(records: List[Dict[str, Any]], output_dir: str) -> List[str]:
    """Generate publication-ready, black-and-white friendly charts for benchmark comparison."""
    os.makedirs(output_dir, exist_ok=True)
    generated_files = []

    # Filter balanced difficulty for clean trend comparison
    balanced_records = [r for r in records if r.get("difficulty") == "balanced"]
    if not balanced_records:
        balanced_records = records

    # -------------------------------------------------------------------------
    # Chart 1: Runtime vs Problem Size n (Log Scale)
    # -------------------------------------------------------------------------
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    methods_config = [
        ("Exact (Brute-Force)", "o", "-", "black", "Exact"),
        ("Greedy Heuristic", "s", "--", "#333333", "Greedy"),
        ("Simulated Annealing", "^", "-.", "#555555", "Sim. Annealing"),
        ("Continuous SLSQP", "v", ":", "#777777", "Continuous SLSQP"),
        ("QAOA (p=1)", "D", "-", "#222222", "QAOA (p=1, Sim)"),
        ("QAOA (p=2)", "*", "--", "#000000", "QAOA (p=2, Sim)"),
    ]

    for m_name, marker, linestyle, color, label in methods_config:
        subset = [r for r in balanced_records if r.get("method") == m_name]
        if subset:
            subset_sorted = sorted(subset, key=lambda x: x["n_assets"])
            ns = [r["n_assets"] for r in subset_sorted]
            runtimes = [max(0.01, r["runtime_ms"]) for r in subset_sorted]
            ax.plot(
                ns,
                runtimes,
                marker=marker,
                linestyle=linestyle,
                color=color,
                label=label,
                linewidth=1.8,
                markersize=7,
            )

    ax.set_yscale("log")
    ax.set_xlabel("Number of Assets (n)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Execution Time (ms, log scale)", fontsize=11, fontweight="bold")
    ax.set_title("Runtime Scaling: Classical Solvers vs QAOA Classical Simulation", fontsize=12, fontweight="bold")
    ax.grid(True, linestyle=":", alpha=0.6)
    ax.legend(loc="upper left", frameon=True, fontsize=9)
    plt.tight_layout()

    chart1_path = os.path.join(output_dir, "runtime_vs_n_scaling.png")
    fig.savefig(chart1_path)
    plt.close(fig)
    generated_files.append(chart1_path)

    # -------------------------------------------------------------------------
    # Chart 2: QAOA Approximation Ratio vs Circuit Depth p
    # -------------------------------------------------------------------------
    fig, ax = plt.subplots(figsize=(7, 4.5), dpi=300)
    p_values = [1, 2, 3]
    markers = ["o", "s", "^", "D"]
    line_styles = ["-", "--", "-.", ":"]

    sample_ns = [4, 6, 8, 10]
    for idx, n in enumerate(sample_ns):
        ratios = []
        valid_ps = []
        for p in p_values:
            match = [
                r for r in records
                if r.get("n_assets") == n and r.get("method") == f"QAOA (p={p})" and r.get("difficulty") == "balanced"
            ]
            if match and match[0].get("approx_ratio") is not None:
                ratios.append(match[0]["approx_ratio"])
                valid_ps.append(p)

        if valid_ps and ratios:
            ax.plot(
                valid_ps,
                ratios,
                marker=markers[idx % len(markers)],
                linestyle=line_styles[idx % len(line_styles)],
                color="black",
                label=f"Instance n={n}",
                linewidth=1.8,
                markersize=8,
            )

    ax.set_xlabel("QAOA Layers (p)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Approximation Ratio (QAOA / Exact Optimum)", fontsize=11, fontweight="bold")
    ax.set_title("QAOA Solution Quality Improvement vs Depth p", fontsize=12, fontweight="bold")
    ax.set_ylim(0.70, 1.05)
    ax.set_xticks(p_values)
    ax.grid(True, linestyle=":", alpha=0.6)
    ax.legend(loc="lower right", frameon=True, fontsize=9)
    plt.tight_layout()

    chart2_path = os.path.join(output_dir, "qaoa_approx_ratio_vs_p.png")
    fig.savefig(chart2_path)
    plt.close(fig)
    generated_files.append(chart2_path)

    # -------------------------------------------------------------------------
    # Chart 3: Feasibility Rate by Method across Instances
    # -------------------------------------------------------------------------
    fig, ax = plt.subplots(figsize=(8, 4.5), dpi=300)
    unique_methods = [
        "Exact (Brute-Force)",
        "Greedy Heuristic",
        "Simulated Annealing",
        "Continuous SLSQP",
        "QAOA (p=1)",
        "QAOA (p=2)",
    ]

    method_labels = ["Exact", "Greedy", "SA", "SLSQP", "QAOA (p=1)", "QAOA (p=2)"]
    feas_rates = []
    hatches = ["//", "\\\\", "xx", "..", "--", "++"]

    for m in unique_methods:
        sub = [r for r in records if r.get("method") == m]
        if sub:
            rate = sum(1 for r in sub if r.get("feasible")) / len(sub) * 100.0
            feas_rates.append(rate)
        else:
            feas_rates.append(0.0)

    bars = ax.bar(
        range(len(unique_methods)),
        feas_rates,
        color="lightgray",
        edgecolor="black",
        linewidth=1.2,
        width=0.55,
    )

    for bar, hatch in zip(bars, hatches):
        bar.set_hatch(hatch)

    ax.set_xticks(range(len(unique_methods)))
    ax.set_xticklabels(method_labels, fontsize=10, fontweight="bold")
    ax.set_ylabel("Strict Feasibility Rate (%)", fontsize=11, fontweight="bold")
    ax.set_title("Constraint Feasibility Satisfaction Across Test Suite", fontsize=12, fontweight="bold")
    ax.set_ylim(0, 115)
    ax.grid(axis="y", linestyle=":", alpha=0.6)

    for bar in bars:
        h = bar.get_height()
        ax.text(
            bar.get_x() + bar.get_width() / 2.0,
            h + 2.0,
            f"{h:.1f}%",
            ha="center",
            va="bottom",
            fontsize=9,
            fontweight="bold",
        )

    plt.tight_layout()
    chart3_path = os.path.join(output_dir, "feasibility_rate_by_method.png")
    fig.savefig(chart3_path)
    plt.close(fig)
    generated_files.append(chart3_path)

    # -------------------------------------------------------------------------
    # Chart 4: Objective Value Gap vs Exact Optimum
    # -------------------------------------------------------------------------
    fig, ax = plt.subplots(figsize=(8, 5), dpi=300)
    compare_methods = [
        ("Greedy Heuristic", "s", "--", "Greedy"),
        ("Simulated Annealing", "^", "-.", "SA"),
        ("QAOA (p=2)", "*", "-", "QAOA (p=2)"),
    ]

    for m_name, marker, linestyle, label in compare_methods:
        sub = [r for r in balanced_records if r.get("method") == m_name]
        sub_sorted = sorted(sub, key=lambda x: x["n_assets"])
        ns = [r["n_assets"] for r in sub_sorted]
        # Gap = (1 - approx_ratio) * 100%
        gaps = [
            max(0.0, (1.0 - r["approx_ratio"]) * 100.0) if r.get("approx_ratio") is not None else 0.0
            for r in sub_sorted
        ]
        ax.plot(
            ns,
            gaps,
            marker=marker,
            linestyle=linestyle,
            color="black",
            label=label,
            linewidth=1.8,
            markersize=7,
        )

    ax.set_xlabel("Number of Assets (n)", fontsize=11, fontweight="bold")
    ax.set_ylabel("Optimality Gap (% below Exact)", fontsize=11, fontweight="bold")
    ax.set_title("Optimality Gap vs Exact Classical Optimum", fontsize=12, fontweight="bold")
    ax.grid(True, linestyle=":", alpha=0.6)
    ax.legend(loc="upper left", frameon=True, fontsize=9)
    plt.tight_layout()

    chart4_path = os.path.join(output_dir, "objective_gap_vs_exact.png")
    fig.savefig(chart4_path)
    plt.close(fig)
    generated_files.append(chart4_path)

    return generated_files

