<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Treasury, TreasuriesListResponse, SystemHealthResponse } from '$lib/types';

  let treasuries = $state<Treasury[]>([]);
  let health = $state<SystemHealthResponse | null>(null);
  let isLoading = $state(true);
  let healthError = $state<string | null>(null);

  const isAiUp = $derived(health?.dependencies?.ai_engine === 'up');
  const isOptUp = $derived(health?.dependencies?.optimization_engine === 'up');
  
  const optEngineStatus = $derived.by<'online' | 'offline' | 'not_configured' | 'error'>(() => {
    if (healthError && !health) return 'error';
    if (!health) return 'offline';
    if (health.dependencies?.optimization_engine === 'up') return 'online';
    if (health.dependencies?.optimization_engine === 'down') return 'offline';
    if (!health.dependencies?.optimization_engine) return 'not_configured';
    return 'offline';
  });

  const aiEngineStatus = $derived.by<'online' | 'offline' | 'error'>(() => {
    if (healthError && !health) return 'error';
    if (!health) return 'offline';
    if (isAiUp) return 'online';
    return 'offline';
  });

  async function loadData() {
    isLoading = true;
    healthError = null;
    try {
      const [tRes, hRes] = await Promise.allSettled([
        api<TreasuriesListResponse>('/api/v1/treasuries?limit=50'),
        api<SystemHealthResponse>('/api/v1/health')
      ]);

      if (tRes.status === 'fulfilled' && tRes.value?.data) {
        treasuries = Array.isArray(tRes.value.data) ? tRes.value.data : [];
      } else {
        treasuries = [];
      }

      if (hRes.status === 'fulfilled' && hRes.value) {
        health = hRes.value;
      } else if (hRes.status === 'rejected') {
        healthError = 'Unable to probe microservice health endpoints';
      }
    } catch (err: any) {
      healthError = err?.message || 'Cluster diagnostic failure';
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<svelte:head>
  <title>Optimization Engine - QUANTA</title>
</svelte:head>

<div class="page-root">
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">HEURISTIC &amp; MATHEMATICAL SOLVER CLUSTER</span>
      </div>
      <h1 class="page-title">Optimization Engine</h1>
      <p class="page-subtitle">Discrete state-space heuristic search, branch-and-bound pruning, and multi-objective portfolio allocation solvers.</p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-refresh" onclick={loadData} disabled={isLoading}>
        {#if isLoading}Refreshing...{:else}Probe Solvers{/if}
      </button>
    </div>
  </div>

  <!-- Live Cluster Telemetry Banner -->
  <div class="cluster-status-banner">
    <div class="status-col">
      <span class="status-col-title">HEURISTIC SEARCH ENGINE</span>
      <div class="status-col-val">
        <span class="status-dot {aiEngineStatus === 'online' ? 'dot-online' : 'dot-offline'}"></span>
        <span class="status-text font-bold">
          {aiEngineStatus === 'online' ? 'AI Engine Online (Port 8000)' : 'AI Engine Offline'}
        </span>
      </div>
      <span class="status-sub">FastAPI runtime &middot; A*, BFS, Best-First solvers active &amp; ready for allocation requests</span>
    </div>

    <div class="status-divider"></div>

    <div class="status-col">
      <span class="status-col-title">NUMERICAL / QUANTUM OPTIMIZATION</span>
      <div class="status-col-val">
        <span class="status-dot {optEngineStatus === 'online' ? 'dot-online' : 'dot-offline'}"></span>
        <span class="status-text font-bold">
          {#if optEngineStatus === 'online'}
            Optimization Engine Online (Port 8001)
          {:else if optEngineStatus === 'offline'}
            Optimization Service Offline (Port 8001 Container Not Running)
          {:else if optEngineStatus === 'not_configured'}
            Service Not Configured (Optional Profile)
          {:else}
            Health Check Failed
          {/if}
        </span>
      </div>
      <span class="status-sub">
        {#if optEngineStatus === 'online'}
          SciPy SLSQP, Quadratic Programming &amp; QUBO solvers available
        {:else}
          Member 2 optimization-engine service is not deployed on Port 8001. Continuous QP/SLSQP and QUBO solvers are offline and disabled.
        {/if}
      </span>
    </div>
  </div>

  <!-- Solver Algorithms Grid -->
  <div class="solvers-grid">
    <div class="solver-card">
      <div class="solver-top">
        <span class="solver-type">HEURISTIC SEARCH</span>
        <span class="solver-badge {aiEngineStatus === 'online' ? 'badge-active' : 'badge-offline'}">
          {aiEngineStatus === 'online' ? 'Active' : 'Offline'}
        </span>
      </div>
      <h3 class="solver-title">A* Search (Admissible Heuristic)</h3>
      <p class="solver-desc">
        Guarantees optimal step-size asset allocations by estimating remaining distance to risk-return efficiency frontier.
      </p>
      <div class="solver-meta-row">
        <span class="meta-item">Host: Port 8000</span>
        <span class="meta-item">Optimality: Guaranteed</span>
      </div>
    </div>

    <div class="solver-card">
      <div class="solver-top">
        <span class="solver-type">EXHAUSTIVE GRAPH</span>
        <span class="solver-badge {aiEngineStatus === 'online' ? 'badge-active' : 'badge-offline'}">
          {aiEngineStatus === 'online' ? 'Active' : 'Offline'}
        </span>
      </div>
      <h3 class="solver-title">Breadth-First Search (BFS)</h3>
      <p class="solver-desc">
        Complete level-by-level state space exploration across discrete capital allocation step intervals.
      </p>
      <div class="solver-meta-row">
        <span class="meta-item">Host: Port 8000</span>
        <span class="meta-item">Completeness: 100%</span>
      </div>
    </div>

    <div class="solver-card">
      <div class="solver-top">
        <span class="solver-type">GREEDY APPROXIMATION</span>
        <span class="solver-badge {aiEngineStatus === 'online' ? 'badge-active' : 'badge-offline'}">
          {aiEngineStatus === 'online' ? 'Active' : 'Offline'}
        </span>
      </div>
      <h3 class="solver-title">Best-First Search</h3>
      <p class="solver-desc">
        Prioritizes states with maximum expected marginal return and minimal incremental variance.
      </p>
      <div class="solver-meta-row">
        <span class="meta-item">Host: Port 8000</span>
        <span class="meta-item">Speed: Sub-second</span>
      </div>
    </div>

    <div class="solver-card solver-card-disabled">
      <div class="solver-top">
        <span class="solver-type">NUMERICAL / QP SOLVER</span>
        <span class="solver-badge badge-offline">
          OFFLINE / UNAVAILABLE
        </span>
      </div>
      <h3 class="solver-title">SciPy SLSQP / Quadratic Programming</h3>
      <p class="solver-desc">
        Member 2 solver service on Port 8001 is currently offline and not implemented. Continuous numerical quadratic programming, SLSQP, and quantum simulation solvers cannot execute.
      </p>
      <div class="solver-actions-row">
        <button
          type="button"
          class="btn-solver-disabled"
          disabled
          title="Optimization Engine (Port 8001) is offline. Continuous numerical solvers cannot execute."
        >
          Solver Unavailable (Port 8001 Offline)
        </button>
      </div>
      <div class="solver-meta-row">
        <span class="meta-item">Port: 8001</span>
        <span class="meta-item">Execution: Disabled</span>
      </div>
    </div>
  </div>

  <!-- System Delineation & Disclosure Banner -->
  <div class="disclosure-card">
    <div class="disclosure-header">
      <span class="disclosure-title">SOLVER DELINEATION &amp; STATUS DISCLOSURE</span>
    </div>
    <div class="disclosure-grid">
      <div class="disclosure-item">
        <span class="item-title">Classical Heuristic Solvers (Active &mdash; Port 8000)</span>
        <p class="item-desc">
          Discrete state-space exploration algorithms (A* Admissible Search, BFS, Best-First) execute in production on the AI Engine cluster (Port 8000). Allocations use verified polynomial runtime bounds and deterministic risk constraints.
        </p>
      </div>
      <div class="disclosure-item">
        <span class="item-title">Numerical QP &amp; Quantum Simulation (Unavailable &mdash; Port 8001)</span>
        <p class="item-desc">
          Member 2's Optimization Engine service (Port 8001) is currently not deployed in the repository. Continuous Markowitz SLSQP and QUBO/QAOA formulations are offline and disabled. No quantum hardware execution or quantum advantage is claimed; all quantum formulations are designed as classical state-vector simulations.
        </p>
      </div>
    </div>
  </div>

  <!-- Launch Optimization on Active Portfolios -->
  <div class="portfolios-card">
    <div class="card-header-bar">
      <div>
        <h2 class="card-title">Corporate Portfolio Solver Targets</h2>
        <p class="card-sub">Select an institutional portfolio to trigger neuro-symbolic allocation optimization using active Port 8000 heuristic solvers.</p>
      </div>
      <span class="pill-kicker">{treasuries.length} Available Portfolios</span>
    </div>

    <div class="table-wrap">
      <table class="portfolios-table">
        <thead>
          <tr>
            <th>Portfolio Name</th>
            <th>Currency</th>
            <th class="text-right">Total Capital</th>
            <th class="text-center">Risk Tolerance</th>
            <th class="text-center">Min Liquidity</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          {#each treasuries as t}
            <tr>
              <td class="font-bold">{t.name}</td>
              <td><span class="currency-tag">{t.base_currency || 'USD'}</span></td>
              <td class="text-right font-mono">${(Number(t.total_capital) / 1_000_000).toFixed(1)}M</td>
              <td class="text-center font-mono">{Number(t.risk_tolerance).toFixed(2)}</td>
              <td class="text-center font-mono">{(Number(t.min_liquidity_ratio) * 100).toFixed(1)}%</td>
              <td class="text-right">
                <a href="/treasuries/{t.id}" class="btn-table-action" title="Trigger active AI heuristic solvers on Port 8000">
                  Launch Heuristic Solver (Port 8000) &rarr;
                </a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
</div>

<style>
  .page-root {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    max-width: 1440px;
    margin: 0 auto;
    width: 100%;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--color-border);
  }

  .eyebrow-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-primary);
    letter-spacing: 0.08em;
  }

  .eyebrow-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-primary);
  }

  .page-title {
    font-size: 1.75rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--color-text);
    margin: 0.25rem 0 0.15rem 0;
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .btn-refresh {
    padding: 0.5rem 0.875rem;
    font-size: 0.75rem;
    font-weight: 600;
    background-color: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
  }

  .btn-refresh:hover:not(:disabled) {
    background-color: var(--color-bg-secondary);
  }

  .cluster-status-banner {
    display: flex;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
    gap: 1.5rem;
    box-shadow: var(--shadow-sm);
  }

  .status-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .status-col-title {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
    font-family: var(--font-mono, monospace);
  }

  .status-col-val {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-online {
    background-color: var(--color-success, #16a34a);
  }

  .dot-offline {
    background-color: var(--color-error, #dc2626);
  }

  .status-text {
    font-size: 0.875rem;
    color: var(--color-text);
  }

  .status-sub {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    line-height: 1.4;
  }

  .status-divider {
    width: 1px;
    background-color: var(--color-border);
  }

  .solvers-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .solver-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    box-shadow: var(--shadow-sm);
  }

  .solver-card-disabled {
    border-color: rgba(220, 38, 38, 0.3);
    background-color: rgba(254, 242, 242, 0.02);
  }

  .solver-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .solver-type {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
    font-family: var(--font-mono, monospace);
  }

  .solver-badge {
    font-size: 0.625rem;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    font-family: var(--font-mono, monospace);
  }

  .badge-active {
    background-color: rgba(22, 163, 74, 0.12);
    color: #16a34a;
    border: 1px solid rgba(22, 163, 74, 0.25);
  }

  .badge-offline {
    background-color: rgba(220, 38, 38, 0.12);
    color: #dc2626;
    border: 1px solid rgba(220, 38, 38, 0.25);
  }

  .solver-title {
    font-size: 0.9375rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .solver-desc {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    line-height: 1.4;
    margin: 0;
    flex: 1;
  }

  .solver-actions-row {
    margin-top: 0.5rem;
  }

  .btn-solver-disabled {
    width: 100%;
    padding: 0.45rem 0.65rem;
    font-size: 0.6875rem;
    font-weight: 700;
    color: #dc2626;
    background-color: rgba(220, 38, 38, 0.08);
    border: 1px solid rgba(220, 38, 38, 0.25);
    border-radius: 4px;
    cursor: not-allowed;
    letter-spacing: 0.02em;
  }

  .solver-meta-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border-subtle);
    padding-top: 0.5rem;
    margin-top: auto;
  }

  /* Disclosure Card */
  .disclosure-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-warning);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }

  .disclosure-header {
    display: flex;
    align-items: center;
  }

  .disclosure-title {
    font-size: 0.6875rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    color: var(--color-text);
    font-family: var(--font-mono, monospace);
  }

  .disclosure-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
  }

  .disclosure-item {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .item-title {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .item-desc {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    line-height: 1.45;
    margin: 0;
  }

  .portfolios-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .card-header-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .card-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .card-sub {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    margin: 0.2rem 0 0 0;
  }

  .pill-kicker {
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-muted);
    padding: 0.25rem 0.6rem;
    background-color: var(--color-bg-secondary);
    border-radius: var(--radius);
  }

  .table-wrap {
    overflow-x: auto;
  }

  .portfolios-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .portfolios-table th {
    background-color: var(--color-bg-secondary);
    padding: 0.75rem 1rem;
    text-align: left;
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--color-border);
  }

  .portfolios-table td {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .currency-tag {
    font-size: 0.6875rem;
    font-family: var(--font-mono, monospace);
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
  }

  .btn-table-action {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-primary);
    text-decoration: none;
    padding: 0.3rem 0.6rem;
    border-radius: var(--radius);
    background-color: var(--color-bg-secondary);
    transition: background-color 0.15s ease;
  }

  .btn-table-action:hover {
    background-color: var(--color-border);
  }

  .font-bold { font-weight: 700; }
  .font-mono { font-family: var(--font-mono, monospace); }
  .text-right { text-align: right; }
  .text-center { text-align: center; }

  @media (max-width: 1024px) {
    .solvers-grid {
      grid-template-columns: repeat(2, 1fr);
    }
    .cluster-status-banner {
      flex-direction: column;
      gap: 1rem;
    }
    .status-divider {
      display: none;
    }
    .disclosure-grid {
      grid-template-columns: 1fr;
      gap: 1rem;
    }
  }

  @media (max-width: 640px) {
    .solvers-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
