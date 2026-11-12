"""Deterministic Simulated Annealing solver for discrete capital allocation."""

from __future__ import annotations
import time
import math
import numpy as np

from app.models.optimization_model import OptimizationModel
from app.schemas.optimization import OptimizationResult


class SimulatedAnnealingSolver:
    """Simulated Annealing heuristic with deterministic seeded execution."""

    def __init__(
        self,
        seed: int = 42,
        initial_temp: float = 10.0,
        cooling_rate: float = 0.95,
        max_iterations: int = 1500,
        penalty_scale: float = 50.0,
    ):
        self.seed = seed
        self.initial_temp = initial_temp
        self.cooling_rate = cooling_rate
        self.max_iterations = max_iterations
        self.penalty_scale = penalty_scale

    def _energy(self, model: OptimizationModel, x: np.ndarray) -> float:
        """Compute penalized energy to minimize (lower is better).
        
        Energy = - score(x) + penalty * sum(violation_magnitudes).
        """
        score = model.score(x)
        violations = model.violation_report(x)
        penalty = len(violations) * self.penalty_scale
        return -score + penalty

    def solve(self, model: OptimizationModel, seed: int = 42) -> OptimizationResult:
        start_time = time.perf_counter()
        n = model.n

        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="simulated_annealing",
                status="infeasible",
                allocation={name: 0.0 for name in model.asset_names},
                objective_value=0.0,
                portfolio_return=0.0,
                portfolio_risk=0.0,
                liquidity=1.0,
                capital_used=0.0,
                feasible=False,
                constraint_violations=reasons,
                runtime_ms=elapsed_ms,
                metadata={"seed": seed, "iterations": 0},
            )

        rng = np.random.RandomState(seed or self.seed)

        # Start with all zeros or random valid state
        current_x = np.zeros(n, dtype=np.float64)
        current_energy = self._energy(model, current_x)

        best_x = current_x.copy()
        best_energy = current_energy
        best_feasible_x: np.ndarray | None = current_x.copy() if model.is_feasible(current_x) else None
        best_feasible_score = model.score(best_feasible_x) if best_feasible_x is not None else -float("inf")

        temp = self.initial_temp

        for iteration in range(self.max_iterations):
            # Neighborhood proposal: single bit flip
            flip_idx = rng.randint(0, n)
            candidate_x = current_x.copy()
            candidate_x[flip_idx] = 1.0 - candidate_x[flip_idx]

            candidate_energy = self._energy(model, candidate_x)
            delta_e = candidate_energy - current_energy

            # Metropolis acceptance criterion (minimizing energy)
            if delta_e < 0.0 or rng.rand() < math.exp(-delta_e / max(temp, 1e-6)):
                current_x = candidate_x
                current_energy = candidate_energy

                if current_energy < best_energy:
                    best_energy = current_energy
                    best_x = current_x.copy()

                # Track best strictly feasible state
                if model.is_feasible(current_x):
                    sc = model.score(current_x)
                    if sc > best_feasible_score:
                        best_feasible_score = sc
                        best_feasible_x = current_x.copy()

            temp *= self.cooling_rate

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        final_x = best_feasible_x if best_feasible_x is not None else best_x
        metrics = model.evaluate_allocation(final_x)
        violations = model.violation_report(final_x)
        is_feas = len(violations) == 0

        allocation = {name: float(final_x[i]) for i, name in enumerate(model.asset_names)}
        return OptimizationResult(
            method="simulated_annealing",
            status="feasible" if is_feas else "infeasible",
            allocation=allocation,
            objective_value=metrics["objective_value"],
            portfolio_return=metrics["portfolio_return"],
            portfolio_risk=metrics["portfolio_risk"],
            liquidity=metrics["liquidity"],
            capital_used=metrics["capital_used"],
            feasible=is_feas,
            constraint_violations=violations,
            runtime_ms=elapsed_ms,
            metadata={
                "seed": seed or self.seed,
                "iterations": self.max_iterations,
                "cooling_rate": self.cooling_rate,
                "final_temp": temp,
                "feasible_solution_found": best_feasible_x is not None,
            },
        )

