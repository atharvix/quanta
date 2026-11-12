"""Classical optimization solvers registry."""

from app.classical.exact import ExactSolver
from app.classical.greedy import GreedySolver
from app.classical.simulated_annealing import SimulatedAnnealingSolver
from app.classical.continuous import ContinuousSLSQPSolver
from app.classical.milp import MILPSolver

__all__ = [
    "ExactSolver",
    "GreedySolver",
    "SimulatedAnnealingSolver",
    "ContinuousSLSQPSolver",
    "MILPSolver",
]
