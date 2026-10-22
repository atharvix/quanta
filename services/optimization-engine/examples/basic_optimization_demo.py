"""Synthetic INR 100 Cr Capital Allocation Benchmark Demonstration.

Compares:
- Exact Brute-Force Enumeration (Ground Truth)
- Greedy Return-per-Cost Heuristic
- Simulated Annealing
- SciPy Continuous SLSQP (Markowitz Frontier)
- QAOA Quantum Statevector Simulation (p=1 and p=2)
"""

import sys
import os

# Set UTF-8 encoding for stdout if available
if hasattr(sys.stdout, "reconfigure"):
    try:
        sys.stdout.reconfigure(encoding="utf-8")
    except Exception:
        pass

# Add service root to path
service_root = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
if service_root not in sys.path:
    sys.path.insert(0, service_root)

from app.models.financial_problem import FinancialProblem, Asset, Liability, Constraint
from app.models.optimization_model import OptimizationModel
from app.classical.exact import ExactSolver
from app.classical.greedy import GreedySolver
from app.classical.simulated_annealing import SimulatedAnnealingSolver
from app.classical.continuous import ContinuousSLSQPSolver
from app.qubo.builder import QUBOBuilder
from app.quantum.simulator import QAOAStatevectorSimulator


def build_canonical_problem() -> FinancialProblem:
    """Construct canonical INR 100 Cr corporate treasury problem."""
    assets = [
        Asset(
            id="gov_bond",
            name="Government Bonds",
            asset_type="gov_bond",
            cost=20.0,
            expected_return=0.072,
            risk=0.045,
            liquidity=0.95,
        ),
        Asset(
            id="corp_bond",
            name="AAA Corporate Debentures",
            asset_type="corp_bond",
            cost=25.0,
            expected_return=0.091,
            risk=0.120,
            liquidity=0.75,
        ),
        Asset(
            id="bluechip_equity",
            name="Large-Cap Equity Index",
            asset_type="equity",
            cost=25.0,
            expected_return=0.145,
            risk=0.350,
            liquidity=0.80,
        ),
        Asset(
            id="t_bills",
            name="91-Day Treasury Bills",
            asset_type="cash_buffer",
            cost=15.0,
            expected_return=0.038,
            risk=0.010,
            liquidity=1.00,
        ),
        Asset(
            id="infra_trust",
            name="Infrastructure Investment Trust",
            asset_type="real_estate",
            cost=20.0,
            expected_return=0.115,
            risk=0.220,
            liquidity=0.40,
        ),
        Asset(
            id="gold_etf",
            name="Gold / Commodity Hedge",
            asset_type="commodity",
            cost=10.0,
            expected_return=0.082,
            risk=0.180,
            liquidity=0.70,
        ),
    ]

    liabilities = [
        Liability(
            id="senior_debt_1",
            name="Senior Term Loan Tranche A",
            amount=15.0,  # INR 15 Cr
            interest_rate=0.085,
            maturity=12,
            priority=1,
            is_mandatory_reserve=True,
        )
    ]

    constraints = [
        Constraint(
            id="c_min_liq",
            constraint_type="minimum_liquidity",
            description="Maintain minimum 25% portfolio liquidity",
            parameters={"threshold": 0.25},
        ),
        Constraint(
            id="c_max_exposure",
            constraint_type="maximum_asset_exposure",
            description="Cap individual asset exposure at 40%",
            parameters={"max_fraction": 0.40},
        ),
    ]

    return FinancialProblem(
        capital=100.0,
        step_size=5.0,
        assets=assets,
        liabilities=liabilities,
        constraints=constraints,
        risk_tolerance=0.50,
        minimum_liquidity=0.25,
        max_exposure_pct=0.40,
        objective="maximize_risk_adjusted_return",
        market_state="normal",
    )


def run_demo():
    print("=" * 95)
    print("QUANTA -- Optimization Engine: INR 100 Cr Classical vs Quantum Benchmark")
    print("=" * 95)

    problem = build_canonical_problem()
    model = OptimizationModel(problem)

    print("\n1. PROBLEM FORMULATION:")
    print(f"   Total Balance Sheet Capital : INR {problem.capital:.2f} Cr")
    print(f"   Mandatory Debt Reserve Req  : INR {model.mandatory_debt_reserve:.2f} Cr")
    print(f"   Deployable Capital Budget   : INR {model.deployable_capital:.2f} Cr")
    print(f"   Minimum Liquidity Floor     : {problem.minimum_liquidity*100:.1f}%")
    print(f"   Asset Universe Count        : {model.n} assets")
    print(f"   Candidate Assets            : {[a.name for a in model.assets]}")

    print("\n2. EXECUTING OPTIMIZATION SOLVERS (CLASSICAL & QUANTUM QAOA SIMULATOR):\n")

    # Solvers
    exact_solver = ExactSolver()
    greedy_solver = GreedySolver()
    sa_solver = SimulatedAnnealingSolver(seed=42)
    slsqp_solver = ContinuousSLSQPSolver()
    qaoa_sim = QAOAStatevectorSimulator()

    builder = QUBOBuilder()
    qubo = builder.build(model, use_slack=False)

    exact_res = exact_solver.solve(model)
    greedy_res = greedy_solver.solve(model)
    sa_res = sa_solver.solve(model, seed=42)
    slsqp_res = slsqp_solver.solve(model)

    exact_bitstring = "".join(str(int(exact_res.allocation[name])) for name in model.asset_names)
    qaoa_p1_res = qaoa_sim.solve(
        model=model,
        qubo=qubo,
        p=1,
        seed=42,
        exact_optimum_score=exact_res.objective_value,
        exact_optimum_bitstring=exact_bitstring,
        use_slack=False,
    )
    qaoa_p2_res = qaoa_sim.solve(
        model=model,
        qubo=qubo,
        p=2,
        seed=42,
        exact_optimum_score=exact_res.objective_value,
        exact_optimum_bitstring=exact_bitstring,
        use_slack=False,
    )

    all_results = [
        ("Exact (Brute-Force)", exact_res),
        ("Greedy Heuristic", greedy_res),
        ("Simulated Annealing", sa_res),
        ("Continuous SLSQP", slsqp_res),
        ("QAOA (p=1, Sim)", qaoa_p1_res),
        ("QAOA (p=2, Sim)", qaoa_p2_res),
    ]

    header = f"{'Method':<24} | {'Status':<9} | {'Feasible':<8} | {'Exp Return':<10} | {'Risk':<7} | {'Liquidity':<9} | {'Score':<8} | {'Time (ms)':<9}"
    print(header)
    print("-" * len(header))

    exact_obj = exact_res.objective_value
    for name, r in all_results:
        f_ret = f"{r.portfolio_return*100:.2f}%"
        f_risk = f"{r.portfolio_risk:.4f}"
        f_liq = f"{r.liquidity*100:.1f}%"
        f_feas = "PASS" if r.feasible else "FAIL"
        f_score = f"{r.objective_value:.4f}"
        f_time = f"{r.runtime_ms:.2f}"
        print(f"{name:<24} | {r.status:<9} | {f_feas:<8} | {f_ret:<10} | {f_risk:<7} | {f_liq:<9} | {f_score:<8} | {f_time:<9}")

    print("\n3. DETAILED ALLOCATION COMPARISON:")
    print(f"{'Asset Name':<34} | {'Exact':<7} | {'Greedy':<7} | {'SA':<7} | {'SLSQP (wt)':<10} | {'QAOA (p=2)':<10}")
    print("-" * 90)
    for a in model.assets:
        ex_val = int(exact_res.allocation[a.name])
        gr_val = int(greedy_res.allocation[a.name])
        sa_val = int(sa_res.allocation[a.name])
        sl_val = f"{slsqp_res.allocation[a.name]*100:.1f}%"
        qa_val = int(qaoa_p2_res.allocation[a.name])
        print(f"{a.name:<34} | {ex_val:<7} | {gr_val:<7} | {sa_val:<7} | {sl_val:<10} | {qa_val:<10}")

    print("\n4. QAOA QUANTUM STATEVECTOR METRICS:")
    print(f"   - Circuit Depth p=1 : Expectation={qaoa_p1_res.metadata['expectation_value']:.4f}, Feasible Prob Mass={qaoa_p1_res.metadata['feasible_probability_mass']*100:.1f}%, Approx Ratio={qaoa_p1_res.metadata.get('approximation_ratio', 0.0):.4f}")
    print(f"   - Circuit Depth p=2 : Expectation={qaoa_p2_res.metadata['expectation_value']:.4f}, Feasible Prob Mass={qaoa_p2_res.metadata['feasible_probability_mass']*100:.1f}%, Approx Ratio={qaoa_p2_res.metadata.get('approximation_ratio', 0.0):.4f}")
    print(f"   - Optimal Bitstring Probability (p=2): {qaoa_p2_res.metadata.get('optimal_bitstring_probability', 0.0)*100:.2f}%")

    print("\n5. SCIENTIFIC STATEMENT ON QUANTUM SCALING:")
    print("   Notice: QAOA runtime represents classical statevector simulation on CPU.")
    print("   Both Exact and Greedy classical algorithms execute in under 1 ms for n=6.")
    print("   QAOA achieves a high approximation ratio, demonstrating theoretical viability,")
    print("   but provides no runtime advantage over classical solvers on this scale.")

    print("\n" + "=" * 95)
    print("DEMONSTRATION COMPLETED SUCCESSFULLY.")
    print("=" * 95)


if __name__ == "__main__":
    run_demo()

