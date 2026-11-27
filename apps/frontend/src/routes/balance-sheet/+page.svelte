<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Treasury, TreasuriesListResponse, Liability } from '$lib/types';
  import { formatPercent } from '$lib/format';
  import { calculateBalanceSheet } from '$lib/financial';

  let treasuries = $state<Treasury[]>([]);
  let liabilitiesMap = $state<Record<string, Liability[]>>({});
  let liabilitiesLoadingMap = $state<Record<string, boolean>>({});
  let liabilitiesErrorMap = $state<Record<string, string>>({});
  let isLoading = $state(true);
  let error = $state<string | null>(null);
  let searchQuery = $state('');

  async function loadData() {
    isLoading = true;
    error = null;
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries?limit=50');
      treasuries = Array.isArray(res.data) ? res.data : [];

      await Promise.allSettled(
        treasuries.map(async (t) => {
          liabilitiesLoadingMap[t.id] = true;
          try {
            const liabRes = await api<{ data: Liability[] }>(`/api/v1/treasuries/${t.id}/liabilities`);
            liabilitiesMap[t.id] = Array.isArray(liabRes.data) ? liabRes.data : [];
          } catch (err: unknown) {
            liabilitiesErrorMap[t.id] = err instanceof Error ? err.message : 'Failed to load debt';
            liabilitiesMap[t.id] = [];
          } finally {
            liabilitiesLoadingMap[t.id] = false;
          }
        })
      );
    } catch (err: unknown) {
      error = err instanceof Error ? err.message : 'Failed to fetch balance sheets.';
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  const filteredTreasuries = $derived.by(() => {
    if (!searchQuery.trim()) return treasuries;
    const q = searchQuery.toLowerCase();
    return treasuries.filter(
      (t) => t.name.toLowerCase().includes(q) || t.base_currency.toLowerCase().includes(q)
    );
  });

  const balanceSheetCalculation = $derived(calculateBalanceSheet(filteredTreasuries, liabilitiesMap));
  const summary = $derived(balanceSheetCalculation.summary);
  const portfolios = $derived(balanceSheetCalculation.portfolios);

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
  <title>Balance Sheet — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Balance Sheet</h1>
      <p>Consolidated capital assets, senior debt obligations, and reserve floors.</p>
    </div>

    <div class="page-head-actions">
      <button type="button" class="btn btn-secondary btn-sm" onclick={loadData} disabled={isLoading}>
        {isLoading ? 'Refreshing...' : 'Refresh'}
      </button>
    </div>
  </header>

  <section class="summary-strip" aria-label="Consolidated balance sheet metrics">
    <div class="summary-cell">
      <span class="summary-label">Consolidated Assets</span>
      <span class="summary-value">{formatMillions(summary.totalAssets)}</span>
      <span class="summary-meta">{summary.portfolioCount} portfolios</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Senior Debt</span>
      <span class="summary-value">{formatMillions(summary.totalLiabilities)}</span>
      <span class="summary-meta">{summary.totalLiabilitiesCount} active facilities</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Net Surplus</span>
      <span class="summary-value {summary.netSurplus >= 0 ? 'pos-val' : 'neg-val'}">
        {summary.netSurplus >= 0 ? '+' : ''}{formatMillions(summary.netSurplus)}
      </span>
      <span class="summary-meta">{summary.netSurplus >= 0 ? 'Solvent position' : 'Net deficit'}</span>
    </div>
    <div class="summary-cell">
      <span class="summary-label">Mean Liquidity Floor</span>
      <span class="summary-value">{(summary.meanLiquidityFloor * 100).toFixed(1)}%</span>
      <span class="summary-meta">Minimum reserve policy</span>
    </div>
  </section>

  <div class="toolbar">
    <input
      type="text"
      placeholder="Filter portfolios by name or currency..."
      class="form-input search-input"
      bind:value={searchQuery}
    />
  </div>

  <div class="table-container">
    {#if isLoading}
      <div class="empty-msg">Loading balance sheets...</div>
    {:else if error}
      <div class="empty-msg err-text">{error}</div>
    {:else if portfolios.length === 0}
      <div class="empty-msg">No matching portfolios found.</div>
    {:else}
      <table class="table">
        <thead>
          <tr>
            <th>Portfolio</th>
            <th>Currency</th>
            <th class="text-right">Total Assets</th>
            <th class="text-right">Senior Debt</th>
            <th class="text-right">Net Surplus</th>
            <th class="text-right">Min Liquidity</th>
            <th class="text-right">Risk Tolerance</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          {#each portfolios as p}
            {@const isLiabLoading = liabilitiesLoadingMap[p.treasuryId]}
            {@const hasLiabError = Boolean(liabilitiesErrorMap[p.treasuryId])}
            <tr>
              <td>
                <a href="/treasuries/{p.treasuryId}" class="portfolio-link">{p.name}</a>
              </td>
              <td class="font-mono">{p.baseCurrency}</td>
              <td class="text-right numeric-cell">{formatMillions(p.totalAssets)}</td>
              <td class="text-right numeric-cell muted-text">
                {#if isLiabLoading}
                  Loading...
                {:else if hasLiabError}
                  <span title={liabilitiesErrorMap[p.treasuryId]}>Unavailable</span>
                {:else}
                  {formatMillions(p.totalLiabilities)}
                {/if}
              </td>
              <td class="text-right numeric-cell {p.netSurplus >= 0 ? 'pos-val' : 'neg-val'}">
                {#if isLiabLoading || hasLiabError}
                  —
                {:else}
                  {p.netSurplus >= 0 ? '+' : ''}{formatMillions(p.netSurplus)}
                {/if}
              </td>
              <td class="text-right numeric-cell">{formatPercent(p.minLiquidityRatio)}</td>
              <td class="text-right numeric-cell">{p.riskTolerance.toFixed(2)}</td>
              <td class="text-right">
                <a href="/treasuries/{p.treasuryId}">Inspect &rarr;</a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {/if}
  </div>
</div>

<style>
  .toolbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }

  .search-input {
    max-width: 360px;
  }

  .portfolio-link {
    font-weight: 500;
    color: var(--color-text);
  }

  .portfolio-link:hover {
    color: var(--color-primary);
    text-decoration: underline;
  }

  .pos-val { color: var(--color-success); }
  .neg-val { color: var(--color-danger); }
  .muted-text { color: var(--color-text-secondary); }
  .err-text { color: var(--color-danger); }

  .empty-msg {
    padding: 2.25rem 1rem;
    text-align: center;
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }
</style>
