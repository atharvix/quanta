# QUANTA Optimization Engine: QUBO & QAOA Methodology

> **Summary:** Mathematical and architectural guide to Quadratic Unconstrained Binary Optimization (QUBO) and Quantum Approximate Optimization Algorithm (QAOA).
> Explains slack-variable inequality encoding, programmatic penalty tuning, Ising conversion, and quantum circuit simulation.
> Designed for reproducible execution on QUANTA's custom NumPy statevector simulator.

---

## 1. QUBO Formulation Overview

Quantum annealing and gate-based quantum algorithms like QAOA solve unconstrained binary optimization problems.
QUBO expresses combinatorial problems in the standard matrix form:

$$\text{Minimize } E(y) = y^T Q y + c_0$$

where:
- $y \in \{0, 1\}^N$ is a binary decision vector of length $N$.
- $Q$ is an upper-triangular or symmetric real matrix of size $N \times N$.
- $c_0$ is a scalar energy offset constant.

Since $y_i \in \{0, 1\}$, we have $y_i^2 = y_i$. Linear coefficients sit on the diagonal of $Q$, and quadratic pairwise interactions sit on the off-diagonal entries.

---

## 2. Converting Financial Objectives to QUBO

The platform maximizes return minus risk: $\max [\mu^T x - \lambda x^T \Sigma x]$.
To minimize energy, we negate the objective function:

$$\text{Minimize } \mathcal{H}_{\text{obj}}(x) = - \sum_{i=1}^n \mu_i x_i + \lambda \sum_{i=1}^n \sum_{j=1}^n \Sigma_{ij} x_i x_j$$

For asset bits $i \in \{1, \dots, n\}$:
- Diagonal: $Q_{ii} \mathrel{+}= - \mu_i + \lambda \Sigma_{ii}$
- Off-diagonal ($i < j$): $Q_{ij} \mathrel{+}= 2 \lambda \Sigma_{ij}$

---

## 3. Inequality Constraints and Binary Slack Variables

Standard financial constraints are inequalities (e.g., budget $\le C_{\text{deploy}}$, liquidity deficit $\le B_{\text{liq}}$).
QUBO requires unconstrained objectives. We convert inequality constraints into equality constraints using binary slack variables, and then penalize squared violations.

### 3.1 Binary Slack Expansion
For an inequality $\sum_{i=1}^n a_i x_i \le B$, we introduce integer slack $S \in [0, B]$ such that:
$$\sum_{i=1}^n a_i x_i + S - B = 0$$

Using a discretization unit $\delta$ (default Rs 1 Cr), slack $S$ is expanded in binary powers of two:
$$S \approx \sum_{k=0}^{K-1} 2^k \delta s_k$$
where $s_k \in \{0, 1\}$ are auxiliary slack qubits, and $K = \lceil \log_2(\lfloor B/\delta \rfloor + 1) \rceil$.

### 3.2 Quadratic Penalty Transformation
The equality constraint is penalized by adding a quadratic penalty term:
$$P \cdot \left( \sum_{i=1}^n a_i x_i + \sum_{k=0}^{K-1} 2^k \delta s_k - B \right)^2$$

Expanding $(A^T y - B)^2 = y^T (A A^T) y - 2 B A^T y + B^2$:
- Diagonal: $Q_{ii} \mathrel{+}= P (a_i^2 - 2 B a_i)$
- Off-diagonal: $Q_{ij} \mathrel{+}= 2 P a_i a_j$
- Energy offset: $c_0 \mathrel{+}= P B^2$

---

## 4. Programmatic Penalty Tuning Rule

Choosing penalty weight $P$ too small causes the solver to violate constraints. Choosing $P$ too large washes out the financial return gradient.
QUANTA sets penalty weights using a deterministic, documented programmatic rule:

$$P_{\text{budget}} = \alpha \cdot \max\left(1.0, \frac{\sum_{i=1}^n \max(0, \mu_i)}{\min_{i} c_i}\right)$$

$$P_{\text{liquidity}} = \alpha \cdot \max\left(1.0, \frac{\sum_{i=1}^n \max(0, \mu_i)}{\max(0.1, 1 - \bar{l})}\right)$$

where $\alpha$ is a scaling multiplier (default $5.0$ to $10.0$).
This rule mathematically ensures that the penalty for violating a constraint by 1 unit strictly exceeds the maximum possible return gain from investing in any asset.

---

## 5. Qubit-Count Estimation

The total number of required qubits is:
$$N_{\text{total}} = N_{\text{assets}} + K_{\text{budget\_slack}} + K_{\text{liquidity\_slack}}$$

- $N_{\text{assets}}$: Number of candidate assets ($n$).
- $K_{\text{budget}}$: $\lceil \log_2(C_{\text{deploy}}/\delta + 1) \rceil$.
- $K_{\text{liquidity}}$: $\lceil \log_2((1 - L_{\text{min}}) C_{\text{deploy}}/\delta + 1) \rceil$.

For instance, with $6$ assets, Rs 85 Cr deployable capital, and $\delta = 1$ Cr, budget slack requires $7$ bits and liquidity requires $6$ bits, totaling $19$ qubits.
To benchmark larger asset universes ($n=12$ to $16$) within the $20$-qubit simulator ceiling, the engine also provides a compact slack-free penalized QUBO mode.

---

## 6. QUBO $\leftrightarrow$ Ising Model Conversion

Physical quantum annealers operate on the Ising spin glass Hamiltonian:
$$H_{\text{Ising}}(z) = \sum_{i=1}^N h_i z_i + \sum_{i < j} J_{ij} z_i z_j + E_0$$
where spin variables $z_i \in \{+1, -1\}$.

The exact transformation between binary variables and spin variables is:
$$x_i = \frac{1 - z_i}{2}$$

Substituting into the QUBO energy yields:
- Coupling strengths: $J_{ij} = \frac{Q_{ij}}{4} \quad (i < j)$
- Local magnetic fields: $h_i = - \frac{Q_{ii}}{2} - \sum_{j > i} \frac{Q_{ij}}{4} - \sum_{k < i} \frac{Q_{ki}}{4}$
- Energy offset: $E_0 = c_0 + \sum_{i=1}^N \frac{Q_{ii}}{2} + \sum_{i < j} \frac{Q_{ij}}{4}$

QUANTA provides verified bidirectional conversion between both representations.

---

## 7. QAOA Circuit Simulation Architecture

The Quantum Approximate Optimization Algorithm (Farhi et al., 2014) is implemented using a custom NumPy statevector simulator.

### 7.1 State Initialization
The system begins in an equal superposition of all $2^N$ basis states:
$$|+\rangle^{\otimes N} = \frac{1}{\sqrt{2^N}} \sum_{k=0}^{2^N-1} |k\rangle$$

### 7.2 Alternating Operator Layers ($p$)
The circuit applies $p$ layers of cost and mixer Hamiltonians:
$$|\psi(\vec{\gamma}, \vec{\beta})\rangle = \prod_{l=1}^p \left( e^{-i \beta_l \sum_{j=1}^N X_j} \cdot e^{-i \gamma_l C} \right) |+\rangle^{\otimes N}$$

1. **Phase Separator $e^{-i \gamma_l C}$**: Since $C$ is diagonal in the computational basis, it applies as a direct element-wise phase rotation $\psi_k \leftarrow \psi_k e^{-i \gamma_l C_k}$.
2. **Mixer Hamiltonian $e^{-i \beta_l X_j}$**: Applies single-qubit rotations $R_x(2 \beta_l) = \cos(\beta_l) I - i \sin(\beta_l) X$ along each qubit dimension.

### 7.3 Classical Parameter Optimization
The energy expectation value is evaluated:
$$\langle C \rangle = \langle \psi(\vec{\gamma}, \vec{\beta}) | C | \psi(\vec{\gamma}, \vec{\beta}) \rangle = \sum_{k=0}^{2^N-1} |\psi_k|^2 C_k$$

Classical optimizers (SciPy `COBYLA`) optimize the $2p$ continuous angles $(\vec{\gamma}, \vec{\beta})$ with multiple seeded restarts.

### 7.4 Measurement and Sampling
From the optimized statevector $|\psi^*\rangle$, the probability distribution $p_k = |\psi_k^*|^2$ is sampled for $M$ shots (default 2048) to extract feasible candidate allocations.
Metrics returned include the approximation ratio $\alpha = \text{Score}_{\text{QAOA}} / \text{Score}_{\text{Exact}}$, feasible probability mass, and wall-clock simulation time.

