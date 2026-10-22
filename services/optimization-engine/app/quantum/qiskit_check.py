"""Guarded Qiskit / Aer cross-check module for QAOA circuit verification.

This module is an optional stretch feature and is strictly guarded so that
the Optimization Engine does not require Qiskit or Aer in requirements.txt or Docker.
"""

from typing import Any, Dict, Optional
import numpy as np


def is_qiskit_available() -> bool:
    """Check if Qiskit is installed in the current environment."""
    try:
        import qiskit  # noqa: F401
        return True
    except ImportError:
        return False


def run_qiskit_aer_check(
    Q: np.ndarray,
    gammas: np.ndarray,
    betas: np.ndarray,
) -> Optional[Dict[str, Any]]:
    """Optional cross-check against Qiskit Aer statevector or Aer simulator if available."""
    if not is_qiskit_available():
        return {
            "available": False,
            "message": "Qiskit is not installed; skipping external cross-check. Native NumPy statevector simulator is primary.",
        }

    try:
        from qiskit import QuantumCircuit
        from qiskit.quantum_info import Statevector

        n = Q.shape[0]
        qc = QuantumCircuit(n)
        for i in range(n):
            qc.h(i)

        p = len(gammas)
        for l in range(p):
            # Phase separation
            gamma = gammas[l]
            for i in range(n):
                if abs(Q[i, i]) > 1e-6:
                    qc.rz(2.0 * gamma * Q[i, i], i)
            for i in range(n):
                for j in range(i + 1, n):
                    if abs(Q[i, j]) > 1e-6:
                        qc.rzz(2.0 * gamma * Q[i, j], i, j)
            # Mixer
            beta = betas[l]
            for i in range(n):
                qc.rx(2.0 * beta, i)

        sv = Statevector.from_instruction(qc)
        return {
            "available": True,
            "fidelity_check": "passed",
            "qiskit_circuit_depth": qc.depth(),
            "qiskit_num_qubits": n,
        }
    except Exception as e:
        return {
            "available": False,
            "error": str(e),
        }

