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
      const preferred = treasuries.find((t) => t.name.toLowerCase().includes('meridian')) || treasuries[0];
      if (preferred) {
        selectedTreasuryId = preferred.id;
        await loadAssets(preferred.id);
      } else {
        assets = [];
      }
    } catch (err: unknown) {
      treasuries = [];
      assets = [];
      if (err instanceof ApiError && err.status === 403) {
        errorMessage = 'Access denied: insufficient permissions to list treasuries.';
      } else {
        errorMessage = err instanceof Error ? err.message : 'Failed to load corporate treasuries.';
      }
    } finally {
      isLoading = false;
    }
  }

  async function loadAssets(id: string) {
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
        errorMessage = 'Access denied: insufficient permissions to view assets for this treasury.';
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
    if (isNaN(val) || val === null || val === undefined) return '—';
    const isNeg = val < 0;
    const abs = Math.abs(val);
    let str = '';
    if (abs >= 1_000_000_000) str = `$${(abs / 1_000_000_000).toFixed(2)}B`;
    else if (abs >= 1_000_000) str = `$${(abs / 1_000_000).toFixed(1)}M`;
    else str = `$${abs.toLocaleString(undefined, { minimumFractionDigits: 0, maximumFractionDigits: 2 })}`;
    return isNeg ? `-${str}` : str;
  }
</script>

<svelte:head>
  <title>Capital Allocation — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Capital Allocation</h1>
      <p>Asset weights, ticket costs, expected yields, and liquidity reserves per portfolio.</p>
    </div>

    <div class="page-head-actions">
      {#if selectedTreasuryId}
        <a href="/treasuries/{selectedTreasuryId}" class="btn btn-primary btn-sm">
          Open portfolio optimizer &rarr;
        </a>
      {/if}
    </div>
  </header>

  <div class="toolbar">
    <label for="portfolio-pick" class="toolbar-label">Portfolio</label>
    <select
      id="portfolio-pick"
      class="form-select toolbar-select"
      bind:value={selectedTreasuryId}
      onchange={() => loadAssets(selectedTreasuryId)}
    >
      {#each treasuries as t}
        <option value={t.id}>{t.name} ({formatMillions(Number(t.total_capital))})</option>
      {/each}
    </select>
  </div>

  <section class="summary-strip" aria-label="Portfolio policy metrics">
    <div class="summary-cell">
      <span class="summary-label">Deployable Capital</span>
      <span class="summary-value">{formatMillions(capital)}</span>
      <span class="summary-meta">Currency {selectedTreasury?.base_currency || 'USD'}</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Liquidity Floor</span>
      <span class="summary-value">{formatPercent(Number(selectedTreasury?.min_liquidity_ratio) || 0)}</span>
      <span class="summary-meta">{formatMillions(capital * (Number(selectedTreasury?.min_liquidity_ratio) || 0))} reserved</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Risk Tolerance</span>
      <span class="summary-value">{Number(selectedTreasury?.risk_tolerance || 0).toFixed(2)}</span>
      <span class="summary-meta">Upper volatility ceiling</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Registered Assets</span>
      <span class="summary-value">{assets.length}</span>
      <span class="summary-meta">{allocation.label}</span>
    </div>
  </section>

  {#if errorMessage}
    <div class="alert alert-danger" role="alert">{errorMessage}</div>
  {:else if treasuries.length === 0 && !isLoading}
    <div class="empty-msg">
      No corporate treasuries found. <a href="/treasuries">Create a treasury &rarr;</a>
    </div>
  {:else if assets.length === 0}
    <div class="empty-msg">
      No financial assets configured for this portfolio.
      {#if selectedTreasuryId}
        <a href="/treasuries/{selectedTreasuryId}">Add assets in portfolio detail &rarr;</a>
      {/if}
    </div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Asset</th>
            <th>Class</th>
            <th class="text-right">Tx Cost</th>
            <th class="text-right">Expected Return</th>
            <th class="text-right">Risk Score</th>
            <th class="text-right">Liquidity</th>
            <th class="text-right">Weight</th>
            <th class="text-right">Allocated</th>
          </tr>
        </thead>
        <tbody>
          {#each assets as a, idx}
            {@const weight = allocation.weights[idx] ?? 0}
            {@const allocated = allocation.allocatedAmounts[idx] ?? 0}
            <tr>
              <td class="font-bold">{a.name}</td>
              <td class="class-cell">{a.asset_type.replaceAll('_', ' ')}</td>
              <td class="text-right numeric-cell">{(Number(a.cost) * 100).toFixed(2)}%</td>
              <td class="text-right numeric-cell pos-val">+{formatPercent(Number(a.expected_return))}</td>
              <td class="text-right numeric-cell">{Number(a.risk).toFixed(2)}</td>
              <td class="text-right numeric-cell">{(Number(a.liquidity) * 100).toFixed(0)}%</td>
              <td class="text-right numeric-cell">{(weight * 100).toFixed(2)}%</td>
              <td class="text-right numeric-cell">{formatMillions(allocated)}</td>
            </tr>
          {/each}
        </tbody>
        <tfoot>
          <tr class="total-row">
            <td colspan="6">Total allocation ({allocation.label})</td>
            <td class="text-right numeric-cell">{(allocation.totalWeight * 100).toFixed(2)}%</td>
            <td class="text-right numeric-cell">{formatMillions(allocation.totalAllocated)}</td>
          </tr>
        </tfoot>
      </table>
    </div>

    {#if allocation.mode === 'equal-weight'}
      <p class="footnote">
        Currently using equal-weight (1/N) distribution. Open the portfolio workbench or Optimization tab to compute risk-adjusted weights.
      </p>
    {/if}
  {/if}
</div>

<style>
  .toolbar {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .toolbar-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text-secondary);
  }

  .toolbar-select {
    max-width: 420px;
  }

  .class-cell {
    color: var(--color-text-secondary);
    text-transform: capitalize;
  }

  .pos-val {
    color: var(--color-success);
  }

  .total-row td {
    background-color: var(--color-bg-secondary);
    border-top: 1px solid var(--color-border);
    font-weight: 600;
  }

  .footnote {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .empty-msg {
    padding: 2.5rem 1rem;
    text-align: center;
    font-size: 0.875rem;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }
</style>
