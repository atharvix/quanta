"""Benchmark runner executing classical and quantum solvers across synthetic instances."""

from __future__ import annotations
import os
import json
import csv
import time
from typing import Any, Dict, List, Optional
import numpy as np

from app.models.optimization_model import OptimizationModel
from app.classical.exact import ExactSolver
from app.classical.greedy import GreedySolver
from app.classical.simulated_annealing import SimulatedAnnealingSolver
from app.classical.continuous import ContinuousSLSQPSolver
from app.classical.milp import MILPSolver
from app.qubo.builder import QUBOBuilder
from app.quantum.simulator import QAOAStatevectorSimulator
from app.benchmarks.generator import generate_synthetic_problem


class BenchmarkSuite:
    """Orchestrates comprehensive benchmark experiments across varying asset counts and regimes."""

    def __init__(
        self,
        asset_counts: Optional[List[int]] = None,
        difficulties: Optional[List[str]] = None,
        output_dir: Optional[str] = None,
        seed: int = 42,
    ):
        self.asset_counts = asset_counts or [4, 6, 8, 10, 12, 14, 16]
        self.difficulties = difficulties or ["balanced", "tight_liquidity", "high_risk_aversion"]
        self.seed = seed

        if output_dir:
            self.output_dir = output_dir
        else:
            base_dir = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", ".."))
            self.output_dir = os.path.join(base_dir, "benchmarks", "results")
        os.makedirs(self.output_dir, exist_ok=True)

    def run_instance(
        self,
        n_assets: int,
        difficulty: str,
        seed: int,
    ) -> List[Dict[str, Any]]:
        """Run all solvers on a single instance and return metrics rows."""
        problem = generate_synthetic_problem(n_assets=n_assets, difficulty=difficulty, seed=seed)
        model = OptimizationModel(problem)

        rows: List[Dict[str, Any]] = []

        # 1. Exact Solver (ground truth)
        exact_solver = ExactSolver(max_assets=20)
        exact_res = exact_solver.solve(model)
        exact_opt_obj = exact_res.objective_value if exact_res.feasible else None
        exact_bitstring = None
        if exact_res.feasible:
            exact_bitstring = "".join(str(int(exact_res.allocation[name])) for name in model.asset_names)

        rows.append({
            "n_assets": n_assets,
            "difficulty": difficulty,
            "seed": seed,
            "method": "Exact (Brute-Force)",
            "p": None,
            "status": exact_res.status,
            "objective_value": round(exact_res.objective_value, 6),
            "portfolio_return": round(exact_res.portfolio_return, 6),
            "portfolio_risk": round(exact_res.portfolio_risk, 6),
            "liquidity": round(exact_res.liquidity, 4),
            "capital_used": round(exact_res.capital_used, 2),
            "feasible": exact_res.feasible,
            "violations_count": len(exact_res.constraint_violations),
            "runtime_ms": round(exact_res.runtime_ms, 2),
            "approx_ratio": 1.0 if exact_res.feasible else None,
            "simulation": False,
        })

        # 2. Greedy Heuristic
        greedy_solver = GreedySolver()
        greedy_res = greedy_solver.solve(model)
        greedy_ratio = None
        if exact_opt_obj and exact_opt_obj > 1e-6 and greedy_res.feasible:
            greedy_ratio = round(greedy_res.objective_value / exact_opt_obj, 4)

        rows.append({
            "n_assets": n_assets,
            "difficulty": difficulty,
            "seed": seed,
            "method": "Greedy Heuristic",
            "p": None,
            "status": greedy_res.status,
            "objective_value": round(greedy_res.objective_value, 6),
            "portfolio_return": round(greedy_res.portfolio_return, 6),
            "portfolio_risk": round(greedy_res.portfolio_risk, 6),
            "liquidity": round(greedy_res.liquidity, 4),
            "capital_used": round(greedy_res.capital_used, 2),
            "feasible": greedy_res.feasible,
            "violations_count": len(greedy_res.constraint_violations),
            "runtime_ms": round(greedy_res.runtime_ms, 2),
            "approx_ratio": greedy_ratio,
            "simulation": False,
        })

        # 3. Simulated Annealing
        sa_solver = SimulatedAnnealingSolver(seed=seed, max_iterations=1200)
        sa_res = sa_solver.solve(model, seed=seed)
        sa_ratio = None
        if exact_opt_obj and exact_opt_obj > 1e-6 and sa_res.feasible:
            sa_ratio = round(sa_res.objective_value / exact_opt_obj, 4)

        rows.append({
            "n_assets": n_assets,
            "difficulty": difficulty,
            "seed": seed,
            "method": "Simulated Annealing",
            "p": None,
            "status": sa_res.status,
            "objective_value": round(sa_res.objective_value, 6),
            "portfolio_return": round(sa_res.portfolio_return, 6),
            "portfolio_risk": round(sa_res.portfolio_risk, 6),
            "liquidity": round(sa_res.liquidity, 4),
            "capital_used": round(sa_res.capital_used, 2),
            "feasible": sa_res.feasible,
            "violations_count": len(sa_res.constraint_violations),
            "runtime_ms": round(sa_res.runtime_ms, 2),
            "approx_ratio": sa_ratio,
            "simulation": False,
        })

        # 4. Continuous SLSQP
        slsqp_solver = ContinuousSLSQPSolver()
        slsqp_res = slsqp_solver.solve(model)
        slsqp_ratio = None
        if exact_opt_obj and exact_opt_obj > 1e-6 and slsqp_res.feasible:
            slsqp_ratio = round(slsqp_res.objective_value / exact_opt_obj, 4)

        rows.append({
            "n_assets": n_assets,
            "difficulty": difficulty,
            "seed": seed,
            "method": "Continuous SLSQP",
            "p": None,
            "status": slsqp_res.status,
            "objective_value": round(slsqp_res.objective_value, 6),
            "portfolio_return": round(slsqp_res.portfolio_return, 6),
            "portfolio_risk": round(slsqp_res.portfolio_risk, 6),
            "liquidity": round(slsqp_res.liquidity, 4),
            "capital_used": round(slsqp_res.capital_used, 2),
            "feasible": slsqp_res.feasible,
            "violations_count": len(slsqp_res.constraint_violations),
            "runtime_ms": round(slsqp_res.runtime_ms, 2),
            "approx_ratio": slsqp_ratio,
            "simulation": False,
        })

        # 5. QAOA Simulator (p = 1, 2, 3)
        builder = QUBOBuilder()
        qubo = builder.build(model, use_slack=False)
        sim = QAOAStatevectorSimulator(max_qubits=20)

        for p_val in [1, 2, 3]:
            # For n >= 14, run p=1,2 to keep simulation fast
            if n_assets > 12 and p_val == 3:
                continue

            qaoa_res = sim.solve(
                model=model,
                qubo=qubo,
                p=p_val,
                seed=seed,
                shots=2048,
                exact_optimum_score=exact_opt_obj,
                exact_optimum_bitstring=exact_bitstring,
                num_restarts=2,
                use_slack=False,
            )

            q_ratio = None
            if exact_opt_obj and exact_opt_obj > 1e-6 and qaoa_res.feasible:
                q_ratio = round(qaoa_res.objective_value / exact_opt_obj, 4)

            rows.append({
                "n_assets": n_assets,
                "difficulty": difficulty,
                "seed": seed,
                "method": f"QAOA (p={p_val})",
                "p": p_val,
                "status": qaoa_res.status,
                "objective_value": round(qaoa_res.objective_value, 6),
                "portfolio_return": round(qaoa_res.portfolio_return, 6),
                "portfolio_risk": round(qaoa_res.portfolio_risk, 6),
                "liquidity": round(qaoa_res.liquidity, 4),
                "capital_used": round(qaoa_res.capital_used, 2),
                "feasible": qaoa_res.feasible,
                "violations_count": len(qaoa_res.constraint_violations),
                "runtime_ms": round(qaoa_res.runtime_ms, 2),
                "approx_ratio": q_ratio,
                "simulation": True,
            })

        return rows

    def run_full_suite(self) -> List[Dict[str, Any]]:
        """Run full benchmark matrix across all asset counts and difficulty profiles."""
        all_rows: List[Dict[str, Any]] = []

        print(f"Starting QUANTA Benchmark Suite across {len(self.asset_counts)} sizes and {len(self.difficulties)} profiles...")

        for n in self.asset_counts:
            for diff in self.difficulties:
                print(f"  -> Running Instance: n={n}, difficulty={diff}...")
                rows = self.run_instance(n_assets=n, difficulty=diff, seed=self.seed)
                all_rows.extend(rows)

        # Save to CSV
        csv_path = os.path.join(self.output_dir, "benchmark_metrics.csv")
        if all_rows:
            keys = list(all_rows[0].keys())
            with open(csv_path, mode="w", newline="", encoding="utf-8") as f:
                writer = csv.DictWriter(f, fieldnames=keys)
                writer.writeheader()
                writer.writerows(all_rows)
            print(f"Saved benchmark CSV to: {csv_path}")

        # Save to JSON
        json_path = os.path.join(self.output_dir, "benchmark_metrics.json")
        with open(json_path, mode="w", encoding="utf-8") as f:
            json.dump(all_rows, f, indent=2)
        print(f"Saved benchmark JSON to: {json_path}")

        return all_rows

