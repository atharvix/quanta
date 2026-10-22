# QUANTA Optimization Engine: Quantum Limitations & Scientific Statement

> **Summary:** Transparent documentation of computational limitations, scaling bottlenecks, and quantum hardware constraints.
> Contains an explicit, unambiguous statement confirming that quantum advantage is not claimed or achieved on this platform.
> Analyzes classical simulation overhead, NISQ physical device noise, slack-bit expansion, and penalty sensitivity.

---

## 1. Explicit Scientific Statement on Quantum Advantage

> [!WARNING]
> **NO QUANTUM ADVANTAGE CLAIMED OR DEMONSTRATED**  
> QUANTA is an educational and academic research decision intelligence platform.
> We do not claim quantum supremacy or practical quantum advantage over classical optimization methods.
> In all measured benchmark experiments, classical exact and heuristic algorithms executed orders of magnitude faster than quantum simulation while achieving identical or superior solution quality.

---

## 2. Classical Simulation Scaling ($2^N$ Bottleneck)

Gate-based quantum algorithms simulated on classical computers track full complex statevectors:
- An $N$-qubit system requires a statevector of length $2^N$ complex numbers.
- At $64$-bit double precision ($16$ bytes per complex number):
  - $10$ qubits: $1,024$ amplitudes $\approx 16$ KB.
  - $16$ qubits: $65,536$ amplitudes $\approx 1$ MB.
  - $20$ qubits: $1,048,576$ amplitudes $\approx 16$ MB.
  - $30$ qubits: $1,073,741,824$ amplitudes $\approx 16$ GB.
  - $40$ qubits: $1.1 \times 10^{12}$ amplitudes $\approx 16$ TB (intractable on workstations).

Because of this exponential growth, QUANTA enforces a strict hard cap of **$20$ qubits**.
Any request requesting simulation beyond $20$ qubits immediately returns an HTTP 400 Bad Request error.

---

## 3. Simulation Time vs Real Quantum Hardware Time

All QAOA runtimes reported in QUANTA's API, tables, and documentation represent **classical statevector simulation time on a CPU**, NOT physical quantum processor (QPU) execution time:

| Factor | Classical Simulation (CPU) | Physical QPU Hardware |
| :--- | :--- | :--- |
| **State Evolution** | Matrix-vector multiplication over $2^N$ entries | Natural quantum physical unitary evolution |
| **Scaling** | Exponential runtime $O(p \cdot 2^N)$ | Linear circuit execution time $O(p)$ |
| **Noise & Errors** | Exact mathematical precision ($0$ decoherence) | Gate infidelity, thermal noise, state decay |
| **Readout** | Direct access to all probabilities | Statistical shot noise from projective measurement |

Therefore, comparing classical simulation runtime against classical solver runtime only measures simulator efficiency on a CPU, not physical quantum speedup.

---

## 4. Slack-Variable Overhead in Constrained Problems

In physical finance, portfolios must satisfy hard inequality constraints:
- Capital budget: $\sum c_i x_i \le C_{\text{deploy}}$.
- Minimum liquidity: $\sum (1 - l_i) c_i x_i \le (1 - L_{\text{min}}) C_{\text{deploy}}$.

Because QUBO is unconstrained, inequalities must be converted to equalities by adding binary slack variables.
- Representing slack values up to Rs 85 Cr with Rs 1 Cr resolution requires $7$ extra slack qubits for budget and $6$ extra slack qubits for liquidity.
- A small portfolio of $6$ assets requires $6 + 7 + 6 = 19$ qubits!
- This slack-qubit overhead consumes over $65\%$ of the available qubit budget on auxiliary variables rather than financial assets.

---

## 5. Physical NISQ Device Constraints

If deployed onto current Noisy Intermediate-Scale Quantum (NISQ) processors, additional physical constraints emerge:

1. **Qubit Coherence Time**: Superconducting qubits lose quantum information within $100$ to $300$ microseconds. Deep QAOA circuits ($p \ge 3$) often exceed coherence limits.
2. **Two-Qubit Gate Errors**: Physical Controlled-NOT (CNOT) or CZ gates have error rates of $0.1\%$ to $1.0\%$. In a dense covariance graph, circuit infidelity accumulates rapidly.
3. **Limited Hardware Connectivity**: Most physical QPUs have nearest-neighbor planar grid topologies (e.g., Heavy-Hex). Implementing all-to-all financial covariance couplings requires extensive SWAP gate routing, dramatically inflating circuit depth.

---

## 6. Classical Solver Superiority for Commercial Treasury

For the problem sizes encountered in enterprise treasury operations ($n = 10$ to $100$ asset classes):
- **Greedy Heuristics**: Execute in $<1$ millisecond and consistently achieve $>85\%$ approximation ratios.
- **Simulated Annealing**: Finds near-optimal allocations in $<50$ milliseconds.
- **SciPy SLSQP / HiGHS MILP**: Solves continuous and mixed-integer models with mathematical optimality certificates in milliseconds.

For foreseeable commercial applications, classical mathematical programming remains the recommended, most reliable, and cost-effective approach for corporate capital allocation.

