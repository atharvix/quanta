"""Bidirectional conversion between QUBO and Ising formulations."""

from __future__ import annotations
from typing import Dict, Tuple
import numpy as np


class IsingModel:
    """Ising spin glass formulation.
    
    Energy function: H(z) = sum_i h_i z_i + sum_{i < j} J_ij z_i z_j + offset
    where z_i in {+1, -1}.
    """

    def __init__(self, h: np.ndarray, J: np.ndarray, offset: float):
        self.h = h  # (N,)
        self.J = J  # (N, N) upper triangular
        self.offset = float(offset)
        self.num_spins = len(h)

    def energy(self, z: np.ndarray) -> float:
        """Evaluate Ising energy for spin configuration z in {+1, -1}^N."""
        z = np.asarray(z, dtype=np.float64)
        linear = float(np.dot(self.h, z))
        quadratic = float(np.sum(np.triu(self.J, k=1) * np.outer(z, z)))
        return linear + quadratic + self.offset


def qubo_to_ising(Q: np.ndarray, offset: float = 0.0) -> IsingModel:
    """Convert QUBO matrix Q (upper-triangular or general) to Ising Hamiltonian.
    
    Mapping: x_i = (1 - z_i) / 2 where x_i in {0, 1} and z_i in {+1, -1}.
    """
    Q = np.asarray(Q, dtype=np.float64)
    N = Q.shape[0]

    # Convert to upper-triangular format if full matrix
    Q_tri = np.triu(Q, k=1) + np.tril(Q, k=-1).T
    np.fill_diagonal(Q_tri, np.diag(Q))

    h = np.zeros(N, dtype=np.float64)
    J = np.zeros((N, N), dtype=np.float64)
    ising_offset = float(offset)

    # 1. Two-spin interactions J_ij = Q_ij / 4 for i < j
    for i in range(N):
        for j in range(i + 1, N):
            J[i, j] = Q_tri[i, j] / 4.0

    # 2. Local fields h_i
    for i in range(N):
        diag_term = -Q_tri[i, i] / 2.0
        off_term = -0.25 * (np.sum(Q_tri[i, i + 1:]) + np.sum(Q_tri[:i, i]))
        h[i] = diag_term + off_term

    # 3. Constant energy shift
    ising_offset += 0.5 * float(np.sum(np.diag(Q_tri)))
    ising_offset += 0.25 * float(np.sum(np.triu(Q_tri, k=1)))

    return IsingModel(h=h, J=J, offset=ising_offset)


def ising_to_qubo(ising: IsingModel) -> Tuple[np.ndarray, float]:
    """Convert Ising model back to QUBO matrix and constant offset.
    
    Mapping: z_i = 1 - 2 x_i.
    """
    N = ising.num_spins
    Q = np.zeros((N, N), dtype=np.float64)
    qubo_offset = ising.offset

    # Off-diagonal terms Q_ij = 4 J_ij for i < j
    for i in range(N):
        for j in range(i + 1, N):
            Q[i, j] = 4.0 * ising.J[i, j]

    # Diagonal terms
    for i in range(N):
        sum_J = float(np.sum(ising.J[i, i + 1:]) + np.sum(ising.J[:i, i]))
        Q[i, i] = -2.0 * ising.h[i] - 2.0 * sum_J

    # Offset
    qubo_offset += float(np.sum(ising.h))
    qubo_offset += float(np.sum(np.triu(ising.J, k=1)))

    return Q, qubo_offset

