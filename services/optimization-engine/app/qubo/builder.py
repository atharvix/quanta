"""QUBO matrix formulation, slack variable discretization, and penalty tuning."""

from __future__ import annotations
import math
from typing import Any, Dict, List, Optional, Tuple
import numpy as np

from app.models.optimization_model import OptimizationModel


class QUBOFormulation:
    """Represents a Quadratic Unconstrained Binary Optimization formulation.
    
    Energy function: E(y) = y^T Q y + offset
    where y in {0, 1}^total_qubits.
    """

    def __init__(
        self,
        Q: np.ndarray,
        offset: float,
        n_assets: int,
        n_slack: int,
        asset_names: List[str],
        slack_details: List[Dict[str, Any]],
        penalties: Dict[str, float],
    ):
        self.Q = Q  # (total_qubits x total_qubits)
        self.offset = float(offset)
        self.n_assets = n_assets
        self.n_slack = n_slack
        self.total_qubits = n_assets + n_slack
        self.asset_names = asset_names
        self.slack_details = slack_details
        self.penalties = penalties

    def energy(self, y: np.ndarray) -> float:
        """Compute QUBO energy for binary vector y."""
        y = np.asarray(y, dtype=np.float64)
        return float(np.dot(y, np.dot(self.Q, y)) + self.offset)

    def extract_asset_bits(self, y: np.ndarray) -> np.ndarray:
        """Extract asset decision vector x from combined vector y."""
        return np.asarray(y[: self.n_assets], dtype=np.float64)

    def to_dict(self) -> Dict[str, Any]:
        """Convert formulation to serializable summary dictionary."""
        return {
            "total_qubits": self.total_qubits,
            "asset_bits": self.n_assets,
            "slack_bits": self.n_slack,
            "offset": self.offset,
            "penalties": self.penalties,
            "matrix_shape": list(self.Q.shape),
        }


class QUBOBuilder:
    """Builds QUBO matrices from OptimizationModel with programmatic penalty tuning."""

    def __init__(
        self,
        discretization_unit: float = 1.0,
        penalty_multiplier: float = 5.0,
    ):
        self.discretization_unit = max(0.1, discretization_unit)
        self.penalty_multiplier = penalty_multiplier

    def estimate_qubits(
        self,
        n_assets: int,
        deployable_capital: float,
        min_liquidity: float,
        discretization_unit: Optional[float] = None,
        use_slack: bool = True,
    ) -> Dict[str, int]:
        """Expose qubit-count estimation (asset bits + slack bits)."""
        if not use_slack:
            return {"asset_qubits": n_assets, "slack_qubits": 0, "total_qubits": n_assets}

        d_unit = discretization_unit or self.discretization_unit

        # Budget slack bits
        max_budget_slack = deployable_capital
        m_budget = max(1, int(math.floor(max_budget_slack / d_unit)))
        k_budget = int(math.ceil(math.log2(m_budget + 1)))

        # Liquidity slack bits
        max_liq_slack = (1.0 - min_liquidity) * deployable_capital
        m_liq = max(1, int(math.floor(max_liq_slack / d_unit)))
        k_liq = int(math.ceil(math.log2(m_liq + 1)))

        total_slack = k_budget + k_liq
        return {
            "asset_qubits": n_assets,
            "budget_slack_qubits": k_budget,
            "liquidity_slack_qubits": k_liq,
            "slack_qubits": total_slack,
            "total_qubits": n_assets + total_slack,
        }

    def _generate_slack_weights(self, max_val: float, unit: float) -> List[float]:
        """Generate binary power-of-two weights for slack variable expansion."""
        if max_val <= 0:
            return [1.0]
        m = max(1, int(math.floor(max_val / unit)))
        k = int(math.ceil(math.log2(m + 1)))
        weights = []
        accum = 0
        for i in range(k):
            val = float(2 ** i * unit)
            if accum + val > max_val:
                remaining = max_val - accum
                if remaining > 1e-4:
                    weights.append(remaining)
                break
            weights.append(val)
            accum += val
        if not weights:
            weights = [max_val]
        return weights

    def build(
        self,
        model: OptimizationModel,
        use_slack: bool = True,
        penalty_scale: Optional[float] = None,
    ) -> QUBOFormulation:
        """Construct the QUBO matrix and constant offset."""
        n = model.n
        scale = penalty_scale or self.penalty_multiplier

        # Programmatic penalty weight rule:
        # Penalty must exceed the maximum potential objective gain from violating constraint.
        # Max theoretical objective gain = sum(max(0, mu_i))
        max_possible_return = float(np.sum(np.maximum(0.0, model.mu)))
        min_cost = float(np.min(model.c)) if len(model.c) > 0 and np.min(model.c) > 0 else 1.0

        p_budget = scale * max(1.0, max_possible_return / min_cost)
        p_liquidity = scale * max(1.0, max_possible_return / max(0.1, 1.0 - np.mean(model.liquidity_vec)))

        if not use_slack:
            # Compact slack-free penalized QUBO (N = n qubits)
            # Budget quadratic target penalty: P * (sum c_i x_i - C_deploy)^2 / C_deploy
            # Liquidity quadratic penalty on deficit
            total_qubits = n
            Q = np.zeros((n, n), dtype=np.float64)
            offset = 0.0

            # Objective: min - mu^T x + lambda * x^T Sigma x
            for i in range(n):
                Q[i, i] += -model.mu[i] + model.risk_aversion * model.Sigma[i, i]
            for i in range(n):
                for j in range(i + 1, n):
                    term = 2.0 * model.risk_aversion * model.Sigma[i, j]
                    Q[i, j] += term

            # Budget soft quadratic penalty:
            # P_b / C * (sum c_i x_i - C)^2 = P_b / C * (sum c_i c_j x_i x_j - 2 C sum c_i x_i + C^2)
            c_factor = p_budget / max(1.0, model.deployable_capital)
            for i in range(n):
                Q[i, i] += c_factor * (model.c[i] ** 2 - 2.0 * model.deployable_capital * model.c[i])
            for i in range(n):
                for j in range(i + 1, n):
                    Q[i, j] += 2.0 * c_factor * model.c[i] * model.c[j]
            offset += c_factor * (model.deployable_capital ** 2)

            return QUBOFormulation(
                Q=Q,
                offset=offset,
                n_assets=n,
                n_slack=0,
                asset_names=model.asset_names,
                slack_details=[],
                penalties={"budget_penalty": p_budget, "liquidity_penalty": p_liquidity},
            )

        # Full slack-variable QUBO formulation
        # 1. Budget constraint slack: sum(c_i x_i) + sum(w_k s_k) = C_deploy
        budget_weights = self._generate_slack_weights(model.deployable_capital, self.discretization_unit)
        k_budget = len(budget_weights)

        # 2. Liquidity constraint slack:
        # sum((1 - l_i) c_i x_i) + sum(w_m s_m) = (1 - min_liq) * C_deploy
        max_liq_slack = max(0.1, (1.0 - model.min_liquidity_ratio) * model.deployable_capital)
        liq_weights = self._generate_slack_weights(max_liq_slack, self.discretization_unit)
        k_liq = len(liq_weights)

        total_slack = k_budget + k_liq
        total_qubits = n + total_slack

        Q = np.zeros((total_qubits, total_qubits), dtype=np.float64)
        offset = 0.0

        # Objective term on asset bits: - mu^T x + lambda * x^T Sigma x
        for i in range(n):
            Q[i, i] += -model.mu[i] + model.risk_aversion * model.Sigma[i, i]
        for i in range(n):
            for j in range(i + 1, n):
                term = 2.0 * model.risk_aversion * model.Sigma[i, j]
                Q[i, j] += term

        # -------------------------------------------------------------
        # Penalty 1: Budget Equality
        # (sum_{i=1}^n c_i x_i + sum_{k=0}^{K_1-1} w_{1,k} s_{1,k} - C_deploy)^2
        # Let linear coefficient vector for all variables be a_b:
        # a_b[i] = c[i] for i < n
        # a_b[n + k] = budget_weights[k] for 0 <= k < k_budget
        # -------------------------------------------------------------
        a_b = np.zeros(total_qubits, dtype=np.float64)
        a_b[:n] = model.c
        for k in range(k_budget):
            a_b[n + k] = budget_weights[k]

        C_b = model.deployable_capital

        # Expand: P_b * ( sum_i a_b[i] y_i - C_b )^2
        # = P_b * ( sum_i a_b[i]^2 y_i + 2 sum_{i < j} a_b[i] a_b[j] y_i y_j - 2 C_b sum_i a_b[i] y_i + C_b^2 )
        for i in range(total_qubits):
            Q[i, i] += p_budget * (a_b[i] ** 2 - 2.0 * C_b * a_b[i])
        for i in range(total_qubits):
            for j in range(i + 1, total_qubits):
                Q[i, j] += 2.0 * p_budget * a_b[i] * a_b[j]
        offset += p_budget * (C_b ** 2)

        # -------------------------------------------------------------
        # Penalty 2: Liquidity Equality
        # (sum_{i=1}^n (1 - l_i) c_i x_i + sum_{m=0}^{K_2-1} w_{2,m} s_{2,m} - B_liq)^2
        # where B_liq = (1 - min_liq) * C_deploy
        # -------------------------------------------------------------
        a_l = np.zeros(total_qubits, dtype=np.float64)
        a_l[:n] = (1.0 - model.liquidity_vec) * model.c
        slack_start_liq = n + k_budget
        for m in range(k_liq):
            a_l[slack_start_liq + m] = liq_weights[m]

        B_liq = max_liq_slack

        for i in range(total_qubits):
            Q[i, i] += p_liquidity * (a_l[i] ** 2 - 2.0 * B_liq * a_l[i])
        for i in range(total_qubits):
            for j in range(i + 1, total_qubits):
                Q[i, j] += 2.0 * p_liquidity * a_l[i] * a_l[j]
        offset += p_liquidity * (B_liq ** 2)

        slack_details = [
            {"type": "budget_slack", "index_start": n, "count": k_budget, "weights": budget_weights},
            {"type": "liquidity_slack", "index_start": slack_start_liq, "count": k_liq, "weights": liq_weights},
        ]

        return QUBOFormulation(
            Q=Q,
            offset=offset,
            n_assets=n,
            n_slack=total_slack,
            asset_names=model.asset_names,
            slack_details=slack_details,
            penalties={"budget_penalty": p_budget, "liquidity_penalty": p_liquidity},
        )

