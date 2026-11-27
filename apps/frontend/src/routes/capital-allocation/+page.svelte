<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '$lib/api';
  import type { Treasury, TreasuriesListResponse, Asset, TreasuryAssetsResponse } from '$lib/types';
  import { formatPercent } from '$lib/format';
  import { calculateCapitalAllocation } from '$lib/financial';

  let treasuries = $state<Treasury[]>([]);
  let selectedTreasuryId = $state<string>('');
  let assets = $state<Asset[]>([]);
  let isLoading = $state(true);
  let errorMessage = $state<string | null>(null);

  async function loadTreasuries() {
    isLoading = true;
    errorMessage = null;
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries?limit=50');
      treasuries = Array.isArray(res.data) ? res.data : [];
      const apex = treasuries.find((t) => t.name.toLowerCase().includes('apex')) || treasuries[0];
      if (apex) {
        selectedTreasuryId = apex.id;
        await loadAssets(apex.id);
      } else {
        assets = [];
      }
    } catch (err: unknown) {
      treasuries = [];
      assets = [];
      if (err instanceof ApiError && err.status === 403) {
        errorMessage = 'Access denied: Insufficient permissions to list corporate treasuries.';
      } else {
        errorMessage = err instanceof Error ? err.message : 'Failed to load corporate treasuries.';
      }
    } finally {
      isLoading = false;
    }
  }

  async function loadAssets(id: string) {
    if (!id) return;
    if (!id) {
      assets = [];
      return;
    }
    errorMessage = null;
    try {
      const res = await api<TreasuryAssetsResponse>(`/api/v1/treasuries/${id}/assets`);
      assets = Array.isArray(res.data) ? res.data : [];
    } catch (err: unknown) {
      assets = [];
      if (err instanceof ApiError && err.status === 403) {
        errorMessage = 'Access denied: Insufficient permissions to view assets for this corporate treasury.';
      } else if (err instanceof ApiError && err.status === 404) {
        errorMessage = 'Selected corporate treasury was not found.';
      } else {
        errorMessage = err instanceof Error ? err.message : 'Failed to load financial assets.';
      }
    }
  }

  onMount(() => {
    loadTreasuries();
  });

  const selectedTreasury = $derived(treasuries.find((t) => t.id === selectedTreasuryId));
  const capital = $derived(selectedTreasury ? Number(selectedTreasury.total_capital) || 0 : 0);
  const allocation = $derived(calculateCapitalAllocation(assets, capital));

  function formatMillions(val: number): string {
    if (isNaN(val) || val === null || val === undefined) return 'Unavailable';
    const isNeg = val < 0;
    const abs = Math.abs(val);
    let str = '';
    if (abs >= 1_000_000_000) {
      str = `$${(abs / 1_000_000_000).toFixed(2)}B`;
    } else if (abs >= 1_000_000) {
      str = `$${(abs / 1_000_000).toFixed(1)}M`;
    } else {
      str = `$${abs.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`;
    }
    return isNeg ? `-${str}` : str;
  }
</script>

<svelte:head>
  <title>Capital Allocation - QUANTA</title>
</svelte:head>

<div class="page-root">
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">PORTFOLIO WEIGHTING &amp; ASSET ALLOCATION</span>
      </div>
      <h1 class="page-title">Capital Allocation Architecture</h1>
      <p class="page-subtitle">Multi-asset capital distribution, risk-adjusted expected return optimization, and mandatory liquidity floors.</p>
    </div>
    <div class="header-actions">
      {#if selectedTreasuryId}
        <a href="/treasuries/{selectedTreasuryId}" class="btn-primary-action">
          Open Allocation Optimizer &rarr;
        </a>
      {/if}
    </div>
  </div>

  <!-- Portfolio Selector Bar -->
  <div class="selector-card">
    <label for="portfolio-pick" class="sel-label">Selected Portfolio:</label>
    <select
      id="portfolio-pick"
      class="portfolio-select"
      bind:value={selectedTreasuryId}
      onchange={() => loadAssets(selectedTreasuryId)}
    >
      {#each treasuries as t}
        <option value={t.id}>{t.name} ({formatMillions(Number(t.total_capital))})</option>
      {/each}
    </select>
  </div>

  <!-- Overview Metrics -->
  <div class="metrics-grid">
    <div class="metric-card">
      <span class="m-label">Deployable Capital</span>
      <span class="m-val">{formatMillions(capital)}</span>
      <span class="m-sub">Base Currency: {selectedTreasury?.base_currency || 'USD'}</span>
    </div>
    <div class="metric-card">
      <span class="m-label">Liquidity Floor</span>
      <span class="m-val">{formatPercent(Number(selectedTreasury?.min_liquidity_ratio) || 0)}</span>
      <span class="m-sub">{formatMillions(capital * (Number(selectedTreasury?.min_liquidity_ratio) || 0))} Mandatory Liquid</span>
    </div>
    <div class="metric-card">
      <span class="m-label">Risk Tolerance</span>
      <span class="m-val">{Number(selectedTreasury?.risk_tolerance || 0).toFixed(2)}</span>
      <span class="m-sub">Upper Volatility Boundary</span>
    </div>
    <div class="metric-card">
      <span class="m-label">Active Asset Classes</span>
      <span class="m-val">{assets.length}</span>
      <span class="m-sub">{assets.length} Registered Universe Assets</span>
    </div>
  </div>

  <!-- Asset Holdings Breakdown -->
  <div class="holdings-card">
    <div class="card-header-row">
      <div>
        <h2 class="card-title">Registered Financial Assets</h2>
        <span class="allocation-subtitle">
          {allocation.mode === 'value-weighted'
            ? 'Allocation derived from persisted asset valuation'
            : 'Equal-weight portfolio baseline (1/N unoptimized)'}
        </span>
      </div>
      <span class="tag-status">{allocation.label}</span>
    </div>

    {#if errorMessage}
      <div class="alert-error" role="alert">{errorMessage}</div>
    {:else if treasuries.length === 0 && !isLoading}
      <div class="empty-state">
        No corporate treasuries found. <a href="/treasuries" style="color: var(--color-primary); font-weight: 600;">Create your first treasury &rarr;</a>
      </div>
    {:else if assets.length === 0}
      <div class="empty-state">No financial assets configured for this treasury. Configure assets in treasury details to establish allocation.</div>
    {:else}
      <div class="assets-table-wrap">
        <table class="assets-table">
          <thead>
            <tr>
              <th>Asset Name</th>
              <th>Class</th>
              <th class="text-right">Transaction Cost</th>
              <th class="text-right">Expected Return</th>
              <th class="text-center">Risk Score</th>
              <th class="text-center">Liquidity Rating</th>
              <th class="text-right">Allocation Weight</th>
              <th class="text-right">Allocated Capital</th>
            </tr>
          </thead>
          <tbody>
            {#each assets as a, idx}
              {@const weight = allocation.weights[idx] ?? 0}
              {@const allocated = allocation.allocatedAmounts[idx] ?? 0}
              <tr>
                <td class="col-name">{a.name}</td>
                <td><span class="class-pill">{a.asset_type.toUpperCase()}</span></td>
                <td class="text-right font-mono">{(Number(a.cost) * 100).toFixed(2)}%</td>
                <td class="text-right font-bold positive-text">+{formatPercent(Number(a.expected_return))}</td>
                <td class="text-center font-mono">{Number(a.risk).toFixed(2)}</td>
                <td class="text-center font-mono">{(Number(a.liquidity) * 100).toFixed(0)}%</td>
                <td class="text-right font-bold font-mono">{(weight * 100).toFixed(2)}%</td>
                <td class="text-right font-bold font-mono">{formatMillions(allocated)}</td>
              </tr>
            {/each}
          </tbody>
          <tfoot>
            <tr class="summary-total-row">
              <td colspan="6" class="font-bold">Total Portfolio Allocation ({allocation.label})</td>
              <td class="text-right font-bold font-mono">{(allocation.totalWeight * 100).toFixed(2)}%</td>
              <td class="text-right font-bold font-mono">{formatMillions(allocation.totalAllocated)}</td>
            </tr>
          </tfoot>
        </table>
      </div>
      {#if allocation.mode === 'equal-weight'}
        <p class="allocation-notice">Notice: Assets currently reflect an unoptimized equal-weight distribution baseline (1/N). Run the AI Decision Council or Optimization Engine for risk-adjusted recommendations.</p>
      {/if}
    {/if}
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
    gap: 1rem;
    flex-wrap: wrap;
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

  .btn-primary-action {
    display: inline-flex;
    align-items: center;
    padding: 0.55rem 1.15rem;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    font-size: 0.8125rem;
    font-weight: 700;
    text-decoration: none;
    border-radius: var(--radius);
  }

  .selector-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.875rem 1.25rem;
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .sel-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .portfolio-select {
    flex: 1;
    max-width: 480px;
    padding: 0.5rem 0.75rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text);
    font-size: 0.8125rem;
    outline: none;
  }

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .metric-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    box-shadow: var(--shadow-sm);
  }

  .m-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .m-val {
    font-size: 1.5rem;
    font-weight: 800;
    color: var(--color-text);
    letter-spacing: -0.02em;
  }

  .m-sub {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .holdings-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .card-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .card-title {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .allocation-subtitle {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .summary-total-row {
    background-color: var(--color-bg-secondary);
    border-top: 2px solid var(--color-border);
  }

  .allocation-notice {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin: 0.75rem 0 0;
    line-height: 1.4;
  }

  .tag-status {
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.15rem 0.5rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
    color: var(--color-text-secondary);
  }

  .assets-table-wrap {
    overflow-x: auto;
  }

  .assets-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .assets-table th {
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

  .assets-table td {
    padding: 0.875rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .col-name {
    font-weight: 600;
  }

  .class-pill {
    font-size: 0.625rem;
    font-family: var(--font-mono, monospace);
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 3px;
  }

  .positive-text { color: var(--color-success); }
  .font-mono { font-family: var(--font-mono, monospace); }
  .font-bold { font-weight: 700; }
  .text-right { text-align: right; }
  .text-center { text-align: center; }

  .alert-error {
    padding: 0.875rem 1rem;
    border-radius: var(--radius);
    background-color: rgba(239, 68, 68, 0.1);
    border: 1px solid rgba(239, 68, 68, 0.35);
    color: var(--color-danger, #ef4444);
    font-size: 0.875rem;
    font-weight: 500;
  }

  .empty-state {
    padding: 2.5rem;
    text-align: center;
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  @media (max-width: 1024px) {
    .metrics-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .metrics-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
