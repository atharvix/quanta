<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Treasury, TreasuriesListResponse, Liability } from '$lib/types';
  import { formatCurrency, formatPercent } from '$lib/format';
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

      // Fetch actual persisted liabilities concurrently per treasury
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

  // Data-driven calculation using actual persisted liabilities
  const balanceSheetCalculation = $derived(calculateBalanceSheet(filteredTreasuries, liabilitiesMap));
  const summary = $derived(balanceSheetCalculation.summary);
  const portfolios = $derived(balanceSheetCalculation.portfolios);

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
  <title>Balance Sheet Engine - QUANTA</title>
</svelte:head>

<div class="page-root">
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">CONSOLIDATED ENTERPRISE FINANCIALS</span>
      </div>
      <h1 class="page-title">Balance Sheet Engine</h1>
      <p class="page-subtitle">Real-time consolidated balance sheet capital, debt obligations, and coverage multiples across corporate portfolios.</p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-refresh" onclick={loadData} disabled={isLoading}>
        <span>{isLoading ? 'Synchronizing...' : 'Sync Balance Sheets'}</span>
      </button>
    </div>
  </div>

  <!-- Summary KPI Cards -->
  <div class="summary-cards-grid">
    <div class="summary-card">
      <span class="sum-label">Consolidated Assets</span>
      <span class="sum-val">{formatMillions(summary.totalAssets)}</span>
      <span class="sum-sub">{summary.portfolioCount} Registered Portfolios</span>
    </div>
    <div class="summary-card">
      <span class="sum-label">Senior Obligations</span>
      <span class="sum-val">{formatMillions(summary.totalLiabilities)}</span>
      <span class="sum-sub">{summary.totalLiabilitiesCount} Active Facilities across Portfolios</span>
    </div>
    <div class="summary-card">
      <span class="sum-label">Net Capital Surplus</span>
      <span class="sum-val {summary.netSurplus >= 0 ? 'positive-text' : 'negative-text'}">
        {summary.netSurplus >= 0 ? '+' : ''}{formatMillions(summary.netSurplus)}
      </span>
      <span class="sum-sub">{summary.netSurplus >= 0 ? 'Unencumbered Capital' : 'Capital Deficit / Net Debt'}</span>
    </div>
    <div class="summary-card">
      <span class="sum-label">Mean Liquidity Floor</span>
      <span class="sum-val">{(summary.meanLiquidityFloor * 100).toFixed(1)}%</span>
      <span class="sum-sub">Mandatory Liquidity Reserve</span>
    </div>
  </div>

  <!-- Search & Filter Bar -->
  <div class="filter-bar">
    <input
      type="text"
      placeholder="Filter by portfolio name or currency..."
      class="search-input"
      bind:value={searchQuery}
    />
  </div>

  <!-- Balance Sheet Portfolio Ledger -->
  <div class="table-container">
    {#if isLoading}
      <div class="state-placeholder">Synchronizing corporate balance sheets...</div>
    {:else if error}
      <div class="state-error">Error: {error}</div>
    {:else if portfolios.length === 0}
      <div class="state-placeholder">No matching portfolios found.</div>
    {:else}
      <table class="ledger-table">
        <thead>
          <tr>
            <th>Portfolio Name</th>
            <th>Currency</th>
            <th class="text-right">Total Assets</th>
            <th class="text-right">Senior Debt</th>
            <th class="text-right">Net Surplus</th>
            <th class="text-center">Min Liquidity</th>
            <th class="text-center">Risk Tolerance</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          {#each portfolios as p}
            {@const isLiabLoading = liabilitiesLoadingMap[p.treasuryId]}
            {@const hasLiabError = Boolean(liabilitiesErrorMap[p.treasuryId])}
            <tr>
              <td class="col-name">
                <a href="/treasuries/{p.treasuryId}" class="portfolio-link">{p.name}</a>
              </td>
              <td><span class="currency-tag">{p.baseCurrency}</span></td>
              <td class="text-right font-bold">{formatMillions(p.totalAssets)}</td>
              <td class="text-right font-muted">
                {#if isLiabLoading}
                  <span class="status-loading">Loading...</span>
                {:else if hasLiabError}
                  <span class="status-unavailable" title={liabilitiesErrorMap[p.treasuryId]}>Unavailable</span>
                {:else}
                  {formatMillions(p.totalLiabilities)}
                {/if}
              </td>
              <td class="text-right font-bold {p.netSurplus >= 0 ? 'positive-text' : 'negative-text'}">
                {#if isLiabLoading}
                  <span class="status-loading">—</span>
                {:else if hasLiabError}
                  <span class="status-unavailable">—</span>
                {:else}
                  {p.netSurplus >= 0 ? '+' : ''}{formatMillions(p.netSurplus)}
                {/if}
              </td>
              <td class="text-center">{formatPercent(p.minLiquidityRatio)}</td>
              <td class="text-center">{p.riskTolerance.toFixed(2)}</td>
              <td class="text-right">
                <a href="/treasuries/{p.treasuryId}" class="action-link">Inspect &rarr;</a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
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

  .btn-refresh {
    padding: 0.5rem 1rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.8125rem;
    font-weight: 600;
    cursor: pointer;
  }

  .summary-cards-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .summary-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    box-shadow: var(--shadow-sm);
  }

  .sum-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .sum-val {
    font-size: 1.5rem;
    font-weight: 800;
    color: var(--color-text);
    letter-spacing: -0.02em;
  }

  .sum-sub {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .positive-text {
    color: var(--color-success);
  }

  .negative-text {
    color: var(--color-danger, #ef4444);
  }

  .status-loading {
    color: var(--color-text-muted);
    font-style: italic;
    font-size: 0.8125rem;
  }

  .status-unavailable {
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .filter-bar {
    display: flex;
    gap: 1rem;
  }

  .search-input {
    flex: 1;
    max-width: 400px;
    padding: 0.6rem 0.85rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.8125rem;
    outline: none;
  }

  .search-input:focus {
    border-color: var(--color-primary);
  }

  .table-container {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    overflow-x: auto;
    box-shadow: var(--shadow-sm);
  }

  .ledger-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .ledger-table th {
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

  .ledger-table td {
    padding: 0.875rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .col-name {
    font-weight: 600;
  }

  .portfolio-link {
    color: var(--color-text);
    text-decoration: none;
  }

  .portfolio-link:hover {
    color: var(--color-primary);
    text-decoration: underline;
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

  .text-right { text-align: right; }
  .text-center { text-align: center; }
  .font-bold { font-weight: 700; }
  .font-muted { color: var(--color-text-muted); }

  .state-placeholder, .state-error {
    padding: 3rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
  }

  .state-error {
    color: var(--color-danger);
  }

  @media (max-width: 1024px) {
    .summary-cards-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .summary-cards-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
