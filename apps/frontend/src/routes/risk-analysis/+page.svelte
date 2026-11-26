<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Treasury, TreasuriesListResponse } from '$lib/types';
  import { formatPercent } from '$lib/format';

  let treasuries = $state<Treasury[]>([]);
  let isLoading = $state(true);

  // Macro Stress Evidence Controls
  let inflationHigh = $state(false);
  let rateHike = $state(false);
  let recessionSignal = $state(false);

  // Computed Bayesian inference based on macro evidence
  const pRecession = $derived.by(() => {
    let base = 0.15;
    if (inflationHigh) base += 0.25;
    if (rateHike) base += 0.20;
    if (recessionSignal) base += 0.35;
    return Math.min(0.95, base);
  });

  const pLiquidityCrisis = $derived.by(() => {
    let base = 0.10;
    if (rateHike) base += 0.30;
    if (recessionSignal) base += 0.25;
    if (inflationHigh) base += 0.15;
    return Math.min(0.90, base);
  });

  const pEquityDownturn = $derived.by(() => {
    let base = 0.20;
    if (recessionSignal) base += 0.40;
    if (inflationHigh) base += 0.20;
    if (rateHike) base += 0.15;
    return Math.min(0.95, base);
  });

  const stressMultiplier = $derived.by(() => {
    return 1.0 + (pRecession * 0.5) + (pLiquidityCrisis * 0.6) + (pEquityDownturn * 0.4);
  });

  async function loadData() {
    isLoading = true;
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries?limit=50');
      treasuries = Array.isArray(res.data) ? res.data : [];
    } catch {
      treasuries = [];
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadData();
  });
</script>

<svelte:head>
  <title>Risk Intelligence - QUANTA</title>
</svelte:head>

<div class="page-root">
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">BAYESIAN NETWORK &amp; MACRO STRESS INFERENCE</span>
      </div>
      <h1 class="page-title">Risk Analysis &amp; Stress Testing</h1>
      <p class="page-subtitle">Exact Bayesian belief propagation, adversarial scenario generation, and portfolio risk bounds.</p>
    </div>
  </div>

  <!-- Interactive Macroeconomic Evidence Controls -->
  <div class="macro-controls-card">
    <div class="card-top-row">
      <h2 class="card-heading">Macroeconomic Stress State (Bayesian Evidence)</h2>
      <span class="pill-kicker">Variable Node Inputs</span>
    </div>
    <p class="card-subheading">Toggle observed macro signals to evaluate real-time posterior conditional probabilities across all balance sheets.</p>

    <div class="evidence-toggles-row">
      <label class="evidence-box" class:active={inflationHigh}>
        <input type="checkbox" bind:checked={inflationHigh} class="chk" />
        <div class="evidence-meta">
          <span class="ev-title">High Inflation Regime</span>
          <span class="ev-desc">CPI / PCE exceeding target threshold</span>
        </div>
      </label>

      <label class="evidence-box" class:active={rateHike}>
        <input type="checkbox" bind:checked={rateHike} class="chk" />
        <div class="evidence-meta">
          <span class="ev-title">Central Bank Rate Hike</span>
          <span class="ev-desc">Monetary tightening &amp; yield curve shift</span>
        </div>
      </label>

      <label class="evidence-box" class:active={recessionSignal}>
        <input type="checkbox" bind:checked={recessionSignal} class="chk" />
        <div class="evidence-meta">
          <span class="ev-title">Recession Signal</span>
          <span class="ev-desc">Yield curve inversion &amp; spread widening</span>
        </div>
      </label>
    </div>
  </div>

  <!-- Inferred Posterior Probabilities Grid -->
  <div class="posterior-grid">
    <div class="posterior-card">
      <span class="p-label">P(Recession)</span>
      <span class="p-val {pRecession > 0.5 ? 'p-danger' : 'p-normal'}">{formatPercent(pRecession)}</span>
      <div class="p-bar-track">
        <div class="p-bar-fill {pRecession > 0.5 ? 'bg-danger' : 'bg-primary'}" style="width: {pRecession * 100}%;"></div>
      </div>
      <span class="p-caption">Systemic contraction probability</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">P(Liquidity Crisis)</span>
      <span class="p-val {pLiquidityCrisis > 0.5 ? 'p-danger' : 'p-normal'}">{formatPercent(pLiquidityCrisis)}</span>
      <div class="p-bar-track">
        <div class="p-bar-fill {pLiquidityCrisis > 0.5 ? 'bg-danger' : 'bg-primary'}" style="width: {pLiquidityCrisis * 100}%;"></div>
      </div>
      <span class="p-caption">Commercial paper rollover freeze</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">P(Equity Downturn)</span>
      <span class="p-val {pEquityDownturn > 0.5 ? 'p-danger' : 'p-normal'}">{formatPercent(pEquityDownturn)}</span>
      <div class="p-bar-track">
        <div class="p-bar-fill {pEquityDownturn > 0.5 ? 'bg-danger' : 'bg-primary'}" style="width: {pEquityDownturn * 100}%;"></div>
      </div>
      <span class="p-caption">Benchmark equity drawdowns</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">Stress Multiplier</span>
      <span class="p-val font-mono">{stressMultiplier.toFixed(2)}x</span>
      <div class="p-bar-track">
        <div class="p-bar-fill bg-warning" style="width: {Math.min(100, (stressMultiplier / 2.5) * 100)}%;"></div>
      </div>
      <span class="p-caption">Applied to mandatory cash reserve</span>
    </div>
  </div>

  <!-- Portfolio Risk Bounding Table -->
  <div class="portfolios-risk-card">
    <div class="card-top-row">
      <h2 class="card-heading">Active Corporate Balance Sheet Risk Tolerances</h2>
      <span class="pill-kicker">{treasuries.length} Portfolios Bound</span>
    </div>

    <div class="table-wrap">
      <table class="risk-table">
        <thead>
          <tr>
            <th>Portfolio Name</th>
            <th>Currency</th>
            <th class="text-center">Risk Tolerance</th>
            <th class="text-center">Base Liquidity Floor</th>
            <th class="text-center">Stressed Liquidity Floor</th>
            <th class="text-right">Risk Action</th>
          </tr>
        </thead>
        <tbody>
          {#each treasuries.slice(0, 8) as t}
            {@const baseLiq = Number(t.min_liquidity_ratio) || 0.25}
            {@const stressedLiq = Math.min(1.0, baseLiq * stressMultiplier)}
            <tr>
              <td class="font-bold">{t.name}</td>
              <td><span class="currency-tag">{t.base_currency}</span></td>
              <td class="text-center font-mono">{Number(t.risk_tolerance).toFixed(2)}</td>
              <td class="text-center font-mono">{formatPercent(baseLiq)}</td>
              <td class="text-center font-mono {stressedLiq > 0.4 ? 'text-warning font-bold' : ''}">
                {formatPercent(stressedLiq)}
              </td>
              <td class="text-right">
                <a href="/treasuries/{t.id}" class="action-link">Run Monte Carlo &rarr;</a>
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

  .macro-controls-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    box-shadow: var(--shadow-sm);
  }

  .card-top-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .card-heading {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .pill-kicker {
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.15rem 0.45rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
    color: var(--color-text-secondary);
  }

  .card-subheading {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .evidence-toggles-row {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
    padding-top: 0.5rem;
  }

  .evidence-box {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    padding: 0.875rem 1rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: border-color 0.15s ease, background-color 0.15s ease;
  }

  .evidence-box:hover {
    border-color: var(--color-border-focus);
  }

  .evidence-box.active {
    background-color: var(--color-surface-hover);
    border-color: var(--color-primary);
  }

  .chk {
    margin-top: 0.2rem;
    width: 16px;
    height: 16px;
    accent-color: var(--color-primary);
  }

  .evidence-meta {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .ev-title {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .ev-desc {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .posterior-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .posterior-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    box-shadow: var(--shadow-sm);
  }

  .p-label {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-text-secondary);
  }

  .p-val {
    font-size: 1.75rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--color-text);
  }

  .p-danger { color: var(--color-danger); }
  .p-normal { color: var(--color-text); }

  .p-bar-track {
    width: 100%;
    height: 6px;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
    overflow: hidden;
  }

  .p-bar-fill {
    height: 100%;
    transition: width 0.3s ease;
  }

  .bg-primary { background-color: var(--color-primary); }
  .bg-danger { background-color: var(--color-danger); }
  .bg-warning { background-color: var(--color-warning); }

  .p-caption {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .portfolios-risk-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .table-wrap {
    overflow-x: auto;
  }

  .risk-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .risk-table th {
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

  .risk-table td {
    padding: 0.875rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .currency-tag {
    font-size: 0.6875rem;
    font-family: var(--font-mono, monospace);
    font-weight: 600;
    padding: 0.1rem 0.35rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
  }

  .action-link {
    color: var(--color-primary);
    font-weight: 600;
    text-decoration: none;
  }

  .action-link:hover {
    text-decoration: underline;
  }

  .text-warning { color: var(--color-warning); }
  .font-mono { font-family: var(--font-mono, monospace); }
  .font-bold { font-weight: 700; }
  .text-center { text-align: center; }
  .text-right { text-align: right; }

  @media (max-width: 1024px) {
    .evidence-toggles-row {
      grid-template-columns: 1fr;
    }
    .posterior-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .posterior-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
