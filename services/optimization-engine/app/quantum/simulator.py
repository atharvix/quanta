"""NumPy statevector QAOA simulator for quantum portfolio optimization."""

from __future__ import annotations
import time
from typing import Any, Dict, List, Optional, Tuple
import numpy as np
from scipy.optimize import minimize

from app.models.optimization_model import OptimizationModel
from app.qubo.builder import QUBOFormulation, QUBOBuilder
from app.schemas.optimization import OptimizationResult


class QAOAStatevectorSimulator:
    """Fast, deterministic NumPy statevector simulator for QAOA.
    
    Operates on diagonal cost Hamiltonian C and X-mixer Hamiltonian.
    Strictly capped at max_qubits (default 20) to prevent OOM.
    """

    def __init__(self, max_qubits: int = 20, default_shots: int = 2048):
        self.max_qubits = max_qubits
        self.default_shots = default_shots

    def _apply_phase_separator(self, psi: np.ndarray, gamma: float, C_diag: np.ndarray) -> np.ndarray:
        """Apply phase separator e^(-i * gamma * C)."""
        return psi * np.exp(-1j * gamma * C_diag)

    def _apply_mixer(self, psi: np.ndarray, beta: float, n_qubits: int) -> np.ndarray:
        """Apply single-qubit X mixers e^(-i * beta * X_j) for all qubits j."""
        c = np.cos(beta)
        s = -1j * np.sin(beta)

        # Apply to each qubit along its binary dimension
        for j in range(n_qubits):
            step = 1 << j
            double_step = step << 1
            # Reshape into (-1, 2, step) to vectorize qubit j
            psi_reshaped = psi.reshape(-1, 2, step)
            psi_0 = psi_reshaped[:, 0, :].copy()
            psi_1 = psi_reshaped[:, 1, :].copy()
            psi_reshaped[:, 0, :] = c * psi_0 + s * psi_1
            psi_reshaped[:, 1, :] = s * psi_0 + c * psi_1

        return psi

    def simulate_circuit(
        self,
        gammas: np.ndarray,
        betas: np.ndarray,
        C_diag: np.ndarray,
        n_qubits: int,
    ) -> np.ndarray:
        """Evolve the statevector |+>^N through p QAOA layers."""
        dim = 1 << n_qubits
        # Initial uniform superposition |+>^N
        psi = np.full(dim, 1.0 / np.sqrt(dim), dtype=np.complex128)

        p = len(gammas)
        for layer in range(p):
            psi = self._apply_phase_separator(psi, gammas[layer], C_diag)
            psi = self._apply_mixer(psi, betas[layer], n_qubits)

        return psi

    def compute_expectation(
        self,
        params: np.ndarray,
        C_diag: np.ndarray,
        n_qubits: int,
        p: int,
    ) -> float:
        """Compute the energy expectation value <psi | C | psi>."""
        gammas = params[:p]
        betas = params[p:]
        psi = self.simulate_circuit(gammas, betas, C_diag, n_qubits)
        prob = np.abs(psi) ** 2
        return float(np.sum(prob * C_diag))

    def solve(
        self,
        model: OptimizationModel,
        qubo: Optional[QUBOFormulation] = None,
        p: int = 2,
        seed: int = 42,
        shots: Optional[int] = None,
        exact_optimum_score: Optional[float] = None,
        exact_optimum_bitstring: Optional[str] = None,
        num_restarts: int = 3,
        use_slack: bool = False,
    ) -> OptimizationResult:
        """Execute QAOA optimization and return canonical OptimizationResult."""
        start_time = time.perf_counter()
        n = model.n

        # Check inherent problem infeasibility
        is_infeasible, reasons = model.is_problem_infeasible()
        if is_infeasible:
            elapsed_ms = (time.perf_counter() - start_time) * 1000.0
            return OptimizationResult(
                method="qaoa",
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
                metadata={"simulation": True, "qubits": n, "p": p},
            )

        # Build QUBO if not provided
        if qubo is None:
            builder = QUBOBuilder()
            qubo = builder.build(model, use_slack=use_slack)

        n_qubits = qubo.total_qubits

        # Hard cap check
        if n_qubits > self.max_qubits:
            raise ValueError(
                f"Requested QAOA simulation has {n_qubits} qubits, "
                f"which exceeds the hard simulator cap of {self.max_qubits} qubits."
            )

        dim = 1 << n_qubits
        rng = np.random.RandomState(seed)

        # Precompute diagonal cost Hamiltonian C_diag over 2^n basis states
        # Basis state index k -> bit vector y
        indices = np.arange(dim)
        y_matrix = np.zeros((dim, n_qubits), dtype=np.float64)
        for j in range(n_qubits):
            y_matrix[:, j] = (indices >> j) & 1

        # C_diag = diag(y @ Q @ y^T) + offset
        Q_sym = 0.5 * (qubo.Q + qubo.Q.T)
        C_diag = np.sum(y_matrix @ Q_sym * y_matrix, axis=1) + qubo.offset

        # Parameter optimization with restarts
        best_fun = float("inf")
        best_params: Optional[np.ndarray] = None
        eval_counter = 0

        for restart in range(num_restarts):
            # Seeded initial angles
            init_gamma = rng.uniform(0.0, 2.0 * np.pi, p)
            init_beta = rng.uniform(0.0, np.pi, p)
            init_params = np.concatenate([init_gamma, init_beta])

            def cost_fn(params: np.ndarray) -> float:
                nonlocal eval_counter
                eval_counter += 1
                return self.compute_expectation(params, C_diag, n_qubits, p)

            opt_res = minimize(
                cost_fn,
                init_params,
                method="COBYLA",
                options={"maxiter": 120, "tol": 1e-4},
            )

            if opt_res.fun < best_fun:
                best_fun = opt_res.fun
                best_params = opt_res.x

        if best_params is None:
            best_params = np.concatenate([np.full(p, 0.5), np.full(p, 0.5)])

        # Evolve final statevector
        gammas_opt = best_params[:p]
        betas_opt = best_params[p:]
        final_psi = self.simulate_circuit(gammas_opt, betas_opt, C_diag, n_qubits)
        prob_dist = np.abs(final_psi) ** 2
        prob_dist = prob_dist / np.sum(prob_dist)  # Ensure normalization

        # Measure sample shots
        num_shots = shots or self.default_shots
        sampled_indices = rng.choice(dim, size=num_shots, p=prob_dist)
        counts = np.bincount(sampled_indices, minlength=dim)

        # Analyze feasible probability mass and best sampled bitstring
        feasible_prob_mass = 0.0
        best_sampled_score = -float("inf")
        best_sampled_x: Optional[np.ndarray] = None
        best_sampled_metrics: Optional[Dict] = None

        opt_bitstring_prob = 0.0

        for idx in range(dim):
            x_candidate = y_matrix[idx, :model.n]
            is_feas = model.is_feasible(x_candidate)
            if is_feas:
                feasible_prob_mass += prob_dist[idx]
                score = model.score(x_candidate)
                if counts[idx] > 0 and score > best_sampled_score:
                    best_sampled_score = score
                    best_sampled_x = x_candidate
                    best_sampled_metrics = model.evaluate_allocation(x_candidate)

            # Check if this state matches the exact ground truth
            bitstring_str = "".join(str(int(b)) for b in x_candidate)
            if exact_optimum_bitstring and bitstring_str == exact_optimum_bitstring:
                opt_bitstring_prob += prob_dist[idx]

        # If no feasible candidate was sampled, pick the argmax probability state
        if best_sampled_x is None:
            top_idx = int(np.argmax(counts))
            best_sampled_x = y_matrix[top_idx, :model.n]
            best_sampled_metrics = model.evaluate_allocation(best_sampled_x)

        elapsed_ms = (time.perf_counter() - start_time) * 1000.0

        violations = model.violation_report(best_sampled_x)
        is_feas = len(violations) == 0

        # Approximation ratio
        approx_ratio = None
        if exact_optimum_score is not None and abs(exact_optimum_score) > 1e-6:
            approx_ratio = max(0.0, float(best_sampled_metrics["objective_value"] / exact_optimum_score))

        allocation = {name: float(best_sampled_x[i]) for i, name in enumerate(model.asset_names)}

        return OptimizationResult(
            method="qaoa",
            status="optimal" if is_feas and (approx_ratio is not None and approx_ratio >= 0.99) else "feasible" if is_feas else "infeasible",
            allocation=allocation,
            objective_value=best_sampled_metrics["objective_value"],
            portfolio_return=best_sampled_metrics["portfolio_return"],
            portfolio_risk=best_sampled_metrics["portfolio_risk"],
            liquidity=best_sampled_metrics["liquidity"],
            capital_used=best_sampled_metrics["capital_used"],
            feasible=is_feas,
            constraint_violations=violations,
            runtime_ms=elapsed_ms,
            metadata={
                "simulation": True,
                "qubits": n_qubits,
                "p": p,
                "evaluations": eval_counter,
                "seed": seed,
                "shots": num_shots,
                "expectation_value": float(best_fun),
                "feasible_probability_mass": float(feasible_prob_mass),
                "optimal_bitstring_probability": float(opt_bitstring_prob),
                "approximation_ratio": approx_ratio,
                "optimal_angles": {
                    "gamma": [float(g) for g in gammas_opt],
                    "beta": [float(b) for b in betas_opt],
                },
                "scientific_disclaimer": "QAOA runtime reflects classical statevector simulation on CPU, not QPU quantum hardware time.",
            },
        )

