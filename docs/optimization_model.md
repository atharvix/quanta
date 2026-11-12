# QUANTA Optimization Engine: Mathematical Model Specification

> **Summary:** Mathematical formulation of corporate treasury capital allocation under hard financial constraints.
> Models discrete binary investments and continuous portfolio weights with risk-adjusted return maximization.
> Strictly enforces capital budget limits, mandatory debt servicing reserves, minimum liquidity ratios, and asset exposure caps.

---

## 1. Overview and Problem Context

Corporate treasury management requires allocating capital across competing investment vehicles while preserving safety and liquidity.
In QUANTA, all financial values use Indian Rupees in Crores (Rs Cr).
The corporate treasury operates with an immutable capital base $C$ (e.g., Rs 100 Cr).

The Optimization Engine solves this allocation problem through both discrete binary decision variables and continuous portfolio weights.

---

## 2. Decision Variables

### 2.1 Discrete Binary Formulation (Primary)
Each candidate investment asset $i \in \{1, \dots, n\}$ is assigned a binary decision variable:
$$x_i \in \{0, 1\}$$
- $x_i = 1$: Allocate capital to asset $i$ at ticket cost $c_i$.
- $x_i = 0$: Do not invest in asset $i$.

### 2.2 Continuous Weight Formulation (Classical Benchmark)
For continuous solvers (e.g., SciPy SLSQP), decision variables represent capital fractions:
$$w_i \in [0, 1] \quad \text{for } i = 1, \dots, n$$
where $w_i$ is the fraction of deployable capital invested in asset $i$.

---

## 3. Objective Function: Risk-Adjusted Mean-Variance Utility

The platform maximizes portfolio expected return minus a quadratic penalty for volatility:

$$\text{Maximize } \mathcal{U}(x) = \sum_{i=1}^n \mu_i x_i - \lambda x^T \Sigma x$$

Equivalently, written as a minimization problem:
$$\text{Minimize } \mathcal{J}(x) = - \sum_{i=1}^n \mu_i x_i + \lambda x^T \Sigma x$$

### Parameters:
- $\mu_i$: Annualized expected return fraction of asset $i$ (e.g., $0.08$ for $8\%$).
- $\lambda$: Risk aversion parameter derived from corporate risk tolerance:
  $$\lambda = 1.0 - \text{risk\_tolerance}$$
  When `risk_tolerance = 1.0` (aggressive), $\lambda = 0.0$ and the engine seeks maximum return. When `risk_tolerance = 0.0` (conservative), $\lambda = 1.0$ and volatility is heavily penalized.
- $\Sigma$: Covariance matrix ($n \times n$). By default, assets are modeled as uncorrelated with diagonal variance:
  $$\Sigma_{ii} = \sigma_i^2 = (\text{risk}_i)^2, \quad \Sigma_{ij} = 0 \quad (i \neq j)$$
  If an empirical covariance matrix is provided, it is used directly.

---

## 4. Hard Constraints

### 4.1 Mandatory Debt Reserve and Deployable Capital
Before deploying funds, the treasury must reserve capital for liabilities maturing within the planning horizon ($12$ months) or marked as mandatory:
$$R_{\text{debt}} = \sum_{l \in \text{Liabilities}} \mathbb{I}(\text{mandatory}_l \lor \text{maturity}_l \le 12) \cdot \text{amount}_l$$

Deployable capital $C_{\text{deploy}}$ is the remaining capital available for investment:
$$C_{\text{deploy}} = \max(0, C - R_{\text{debt}})$$

### 4.2 Physical Capital Budget Constraint
Total capital spent across selected assets cannot exceed deployable capital:
$$\sum_{i=1}^n c_i x_i \le C_{\text{deploy}}$$

Unallocated capital remains as liquid cash buffer:
$$\text{Cash}_{\text{unspent}} = C_{\text{deploy}} - \sum_{i=1}^n c_i x_i \ge 0$$

### 4.3 Minimum Liquidity Ratio Floor
Unspent cash is fully liquid ($100\%$ liquid, factor $= 1.0$).
Each invested asset contributes its normalized liquidity factor $l_i \in [0, 1]$.
Total liquid capital is:
$$L_{\text{total}} = \text{Cash}_{\text{unspent}} + \sum_{i=1}^n l_i c_i x_i = C_{\text{deploy}} - \sum_{i=1}^n (1 - l_i) c_i x_i$$

Policy requires portfolio liquidity ratio to meet or exceed minimum liquidity floor $L_{\text{min}}$:
$$\frac{L_{\text{total}}}{C_{\text{deploy}}} \ge L_{\text{min}} \iff \sum_{i=1}^n (1 - l_i) c_i x_i \le (1 - L_{\text{min}}) C_{\text{deploy}}$$
This constraint is strictly linear in decision vector $x$.

### 4.4 Maximum Asset Exposure
To prevent over-concentration, no single asset may exceed a policy fraction of deployable capital:
$$c_i x_i \le \text{max\_exposure\_pct} \cdot C_{\text{deploy}} \quad \forall i$$

### 4.5 Group / Sector Caps (Optional)
If assets belong to specific asset classes (e.g., Equity, Real Estate), category totals are capped:
$$\sum_{i \in \text{Group}_g} c_i x_i \le \text{cap}_g \cdot C_{\text{deploy}}$$

---

## 5. Scoring and Feasibility Engine

The engine provides three evaluation functions:
1. `score(allocation)`: Computes the net objective value $\mu^T x - \lambda x^T \Sigma x$.
2. `is_feasible(allocation)`: Boolean indicator checking whether all constraints are satisfied within numerical tolerance ($10^{-5}$).
3. `violation_report(allocation)`: Returns an itemized list of failed constraints detailing actual values, policy thresholds, and deficit amounts.

If an incoming problem is mathematically impossible (e.g., mandatory debt reserve exceeds total capital), the engine returns `status: "infeasible"` with explanatory violation notes without raising an unhandled exception.

