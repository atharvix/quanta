<script lang="ts">
  import { onMount } from 'svelte';
  import { user } from '$lib/auth';
  import { canWrite, isAdmin } from '$lib/roles';
  import { api, ApiError } from '$lib/api';
  import type {
    Treasury,
    TreasuriesListResponse,
    Asset,
    Liability,
    TreasuryAssetsResponse,
    TreasuryLiabilitiesResponse,
    AuditLogRecord,
    AuditLogsListResponse,
    SystemHealthResponse,
    StrategiesListResponse
  } from '$lib/types';
  import { formatCurrency, formatPercent, formatDate } from '$lib/format';
  import { calculateBalanceSheet, calculatePortfolioDebt } from '$lib/financial';
  import { extractAllocations } from '$lib/risk';

  let treasuries = $state<Treasury[]>([]);
  let liabilitiesMap = $state<Record<string, Liability[]>>({});
  let isLoadingTreasuries = $state(true);
  let treasuriesError = $state<string | null>(null);

  let primaryAssets = $state<Asset[]>([]);
  let primaryStrategyAllocations = $state<Record<string, number> | null>(null);
  let primaryStrategyAlgo = $state<string | null>(null);
  let isLoadingAssets = $state(true);

  let auditLogs = $state<AuditLogRecord[]>([]);
  let isLoadingAuditLogs = $state(true);

  let health = $state<SystemHealthResponse | null>(null);
  let isLoadingHealth = $state(true);
  let isRefreshing = $state(false);
  let latencyMs = $state<number | null>(null);
  let lastUpdated = $state<Date | null>(null);
  let refreshSuccess = $state(false);

  let selectedTimeframe = $state<'1D' | '1W' | '1M' | '1Y'>('1M');
  let selectedSegmentId = $state<string | null>(null);
  let selectedAuditLog = $state<AuditLogRecord | null>(null);

  const timeframeHorizon = $derived.by(() => {
    switch (selectedTimeframe) {
      case '1D':
        return {
          title: 'Intraday horizon (24h)',
          desc: 'Cash reserves measured against prorated 1-day debt service obligations.',
          liabilitiesMultiplier: 1 / 365,
          liabilitiesLabel: 'Daily debt service (1/365 prorated)',
          chartLiabilityLabel: 'Prorated daily debt',
          cutoffMs: 24 * 60 * 60 * 1000
        };
      case '1W':
        return {
          title: 'Rolling 7-day cycle',
          desc: 'Weekly liquidity coverage against prorated 7-day debt service.',
          liabilitiesMultiplier: 7 / 365,
          liabilitiesLabel: 'Weekly debt service (7/365 prorated)',
          chartLiabilityLabel: 'Prorated weekly debt',
          cutoffMs: 7 * 24 * 60 * 60 * 1000
        };
      case '1M':
      default:
        return {
          title: 'Consolidated snapshot',
          desc: 'Balance sheet assets, registered debt principals, and liquidity floors across visible portfolios.',
          liabilitiesMultiplier: 1.0,
          liabilitiesLabel: 'Registered debt principal',
          chartLiabilityLabel: 'Registered debt',
          cutoffMs: 30 * 24 * 60 * 60 * 1000
        };
      case '1Y':
        return {
          title: 'Annual consolidated view',
          desc: 'Full-year capital base, debt commitments, and risk tolerance bounds.',
          liabilitiesMultiplier: 1.0,
          liabilitiesLabel: 'Annual debt commitments',
          chartLiabilityLabel: 'Registered debt',
          cutoffMs: 365 * 24 * 60 * 60 * 1000
        };
    }
  });

  const treasuryCount = $derived(treasuries.length);
  const balanceSheet = $derived(calculateBalanceSheet(treasuries, liabilitiesMap));
  const totalAssetsSum = $derived(balanceSheet.summary.totalAssets);
  const baseTotalLiabilities = $derived(balanceSheet.summary.totalLiabilities);
  const totalLiabilitiesSum = $derived(baseTotalLiabilities * timeframeHorizon.liabilitiesMultiplier);
  const netCapitalSurplus = $derived(totalAssetsSum - totalLiabilitiesSum);

  const methodologyNote = $derived.by(() => {
    const scope = `${formatMillions(baseTotalLiabilities)} registered principal across ${treasuryCount} ${treasuryCount === 1 ? 'portfolio' : 'portfolios'}`;
    if (selectedTimeframe === '1D') return `Daily proration (1/365) of ${scope}.`;
    if (selectedTimeframe === '1W') return `Weekly proration (7/365) of ${scope}.`;
    return `Direct sum of persisted liability principals (${scope}).`;
  });

  const avgLiquidityRatio = $derived(treasuries.length ? balanceSheet.summary.meanLiquidityFloor : 0);
  const avgRiskScore = $derived.by(() => {
    if (!treasuries.length) return 0;
    return treasuries.reduce((acc, t) => acc + (Number(t.risk_tolerance) || 0), 0) / treasuries.length;
  });

  const liquidReserves = $derived(totalAssetsSum * avgLiquidityRatio);
  const coverageRatio = $derived(totalLiabilitiesSum <= 0 ? '—' : `${(totalAssetsSum / totalLiabilitiesSum).toFixed(2)}x`);

  const topTreasuries = $derived.by(() => {
    if (!treasuries.length) return [];
    return [...treasuries]
      .sort((a, b) => (Number(b.total_capital) || 0) - (Number(a.total_capital) || 0))
      .slice(0, 6)
      .map((t) => {
        const cap = Number(t.total_capital) || 0;
        const liab = calculatePortfolioDebt(liabilitiesMap[t.id]) * timeframeHorizon.liabilitiesMultiplier;
        return {
          id: t.id,
          name: t.name.length > 22 ? `${t.name.slice(0, 20)}...` : t.name,
          fullName: t.name,
          capital: cap,
          liability: liab,
          capitalFormatted: formatMillions(cap),
          liabilityFormatted: formatMillions(liab),
          surplusFormatted: formatMillions(cap - liab),
          currency: t.base_currency
        };
      });
  });

  const chartScaleMax = $derived(Math.max(10, ...topTreasuries.flatMap((t) => [t.capital, t.liability])));

  const filteredAuditLogs = $derived.by(() => {
    if (!auditLogs.length) return [];
    const now = Date.now();
    return auditLogs
      .filter((log) => now - new Date(log.created_at).getTime() <= timeframeHorizon.cutoffMs)
      .slice(0, 8);
  });

  const primaryTreasury = $derived.by(() => {
    return (
      treasuries.find((t) => t.name.toLowerCase().includes('meridian')) ||
      treasuries.find((t) => t.name.toLowerCase().includes('apex')) ||
      treasuries[0] ||
      null
    );
  });

  interface AllocationSegment {
    id: string;
    name: string;
    type: string;
    percentage: number;
    amount: number;
    expectedReturn: number;
    color: string;
  }

  const allocationSegments: AllocationSegment[] = $derived.by(() => {
    if (!primaryTreasury || primaryAssets.length === 0) return [];
    const capital = Number(primaryTreasury.total_capital) || 0;
    const palette = ['#0d5c63', '#0f172a', '#2563eb', '#b45309', '#475569', '#059669'];
    const stratSum = primaryStrategyAllocations
      ? primaryAssets.reduce((sum, a) => sum + (primaryStrategyAllocations?.[a.id] || 0), 0)
      : 0;
    const equalPct = Number((100 / primaryAssets.length).toFixed(2));

    return primaryAssets.map((asset, idx) => {
      const pct =
        stratSum > 0
          ? Number((((primaryStrategyAllocations?.[asset.id] || 0) / stratSum) * 100).toFixed(2))
          : idx === primaryAssets.length - 1
            ? Number((100 - equalPct * (primaryAssets.length - 1)).toFixed(2))
            : equalPct;
      const amount =
        stratSum > 0
          ? primaryStrategyAllocations?.[asset.id] || 0
          : capital * (pct / 100);
      return {
        id: asset.id,
        name: asset.name,
        type: asset.asset_type.replaceAll('_', ' '),
        percentage: pct,
        amount,
        expectedReturn: Number(asset.expected_return) || 0,
        color: palette[idx % palette.length]
      };
    });
  });

  const isBackendUp = $derived(health !== null);
  const isDbUp = $derived(health?.database === 'up');
  const isAiUp = $derived(health?.dependencies?.ai_engine === 'up');
  const isOptUp = $derived(health?.dependencies?.optimization_engine === 'up');

  function formatMillions(val: number): string {
    const neg = val < 0;
    const abs = Math.abs(val);
    let formatted = '';
    if (abs >= 1_000_000_000) formatted = `$${(abs / 1_000_000_000).toFixed(2)}B`;
    else if (abs >= 1_000_000) formatted = `$${(abs / 1_000_000).toFixed(1)}M`;
    else if (abs >= 1_000) formatted = `$${(abs / 1_000).toFixed(0)}K`;
    else formatted = `$${abs.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
    return neg ? `-${formatted}` : formatted;
  }

  function formatTime(d: Date | null): string {
    if (!d) return '--:--:--';
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function formatTimeAgo(dateStr: string): string {
    try {
      const diffSec = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
      if (diffSec < 60) return `${Math.max(1, diffSec)}s ago`;
      const diffMin = Math.floor(diffSec / 60);
      if (diffMin < 60) return `${diffMin}m ago`;
      const diffHr = Math.floor(diffMin / 60);
      if (diffHr < 24) return `${diffHr}h ago`;
      return `${Math.floor(diffHr / 24)}d ago`;
    } catch {
      return dateStr;
    }
  }

  async function fetchTreasuries() {
    isLoadingTreasuries = true;
    treasuriesError = null;
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries?limit=50');
      treasuries = Array.isArray(res.data) ? res.data : [];

      const nextMap: Record<string, Liability[]> = {};
      await Promise.allSettled(
        treasuries.map(async (t) => {
          try {
            const liabRes = await api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${t.id}/liabilities`);
            nextMap[t.id] = Array.isArray(liabRes.data) ? liabRes.data : [];
          } catch {
            nextMap[t.id] = [];
          }
        })
      );
      liabilitiesMap = nextMap;
    } catch (err: unknown) {
      treasuriesError = err instanceof Error ? err.message : 'Failed to load treasuries.';
      treasuries = [];
      liabilitiesMap = {};
    } finally {
      isLoadingTreasuries = false;
    }
  }

  async function fetchPrimaryAssets() {
    if (!primaryTreasury?.id) {
      primaryAssets = [];
      primaryStrategyAllocations = null;
      primaryStrategyAlgo = null;
      isLoadingAssets = false;
      return;
    }

    isLoadingAssets = true;
    try {
      const [assetsRes, stratRes] = await Promise.allSettled([
        api<TreasuryAssetsResponse>(`/api/v1/treasuries/${primaryTreasury.id}/assets`),
        api<StrategiesListResponse>(`/api/v1/treasuries/${primaryTreasury.id}/strategies?limit=1`)
      ]);
      primaryAssets =
        assetsRes.status === 'fulfilled' && Array.isArray(assetsRes.value?.data)
          ? assetsRes.value.data
          : [];
      if (stratRes.status === 'fulfilled' && Array.isArray(stratRes.value?.data) && stratRes.value.data[0]?.allocations_json) {
        const extracted = extractAllocations(stratRes.value.data[0].allocations_json, primaryAssets);
        primaryStrategyAllocations = Object.keys(extracted).length > 0 ? extracted : null;
        primaryStrategyAlgo = primaryStrategyAllocations ? stratRes.value.data[0].algorithm_used : null;
      } else {
        primaryStrategyAllocations = null;
        primaryStrategyAlgo = null;
      }
    } catch {
      primaryAssets = [];
      primaryStrategyAllocations = null;
      primaryStrategyAlgo = null;
    } finally {
      isLoadingAssets = false;
    }
  }

  async function fetchAuditLogs() {
    if (!isAdmin($user?.role)) {
      auditLogs = [];
      isLoadingAuditLogs = false;
      return;
    }
    isLoadingAuditLogs = true;
    try {
      const res = await api<AuditLogsListResponse>('/api/v1/audit-logs?limit=100');
      auditLogs = res && Array.isArray(res.data) ? res.data : [];
    } catch {
      auditLogs = [];
    } finally {
      isLoadingAuditLogs = false;
    }
  }

  async function fetchHealth(isManual = false) {
    if (isRefreshing) return;
    if (isManual) isRefreshing = true;
    else if (!health) isLoadingHealth = true;

    const start = performance.now();
    try {
      health = await api<SystemHealthResponse>('/api/v1/health');
      latencyMs = Math.round(performance.now() - start);
      lastUpdated = new Date();
    } catch (err: unknown) {
      latencyMs = Math.round(performance.now() - start);
      if (err instanceof ApiError && err.status === 503) {
        health = { status: 'degraded', database: 'down', dependencies: { ai_engine: 'unknown', optimization_engine: 'down' } };
        lastUpdated = new Date();
      } else {
        health = null;
      }
    } finally {
      isLoadingHealth = false;
      isRefreshing = false;
    }
  }

  async function refreshAll() {
    isRefreshing = true;
    await Promise.allSettled([
      fetchTreasuries().then(() => fetchPrimaryAssets()),
      fetchAuditLogs(),
      fetchHealth(true)
    ]);
    isRefreshing = false;
    refreshSuccess = true;
    setTimeout(() => {
      refreshSuccess = false;
    }, 2500);
  }

  onMount(() => {
    fetchTreasuries().then(() => fetchPrimaryAssets());
    fetchAuditLogs();
    fetchHealth();

    const timer = setInterval(() => fetchHealth(false), 30000);
    return () => clearInterval(timer);
  });
</script>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Treasury Overview</h1>
      <p>{timeframeHorizon.title} &mdash; {timeframeHorizon.desc}</p>
    </div>

    <div class="page-head-actions">
      <div class="seg-control" role="group" aria-label="Reporting horizon">
        {#each ['1D', '1W', '1M', '1Y'] as tf}
          <button
            type="button"
            class="seg-btn"
            class:active={selectedTimeframe === tf}
            onclick={() => (selectedTimeframe = tf as '1D' | '1W' | '1M' | '1Y')}
          >
            {tf}
          </button>
        {/each}
      </div>

      <button type="button" class="btn btn-secondary btn-sm" onclick={refreshAll} disabled={isRefreshing}>
        {isRefreshing ? 'Refreshing...' : refreshSuccess ? 'Updated' : 'Refresh'}
      </button>

      {#if canWrite($user?.role)}
        <a href="/treasuries" class="btn btn-primary btn-sm">New treasury</a>
      {:else}
        <a href="/treasuries" class="btn btn-secondary btn-sm">View treasuries</a>
      {/if}
    </div>
  </header>

  {#if treasuriesError}
    <div class="alert alert-danger" role="alert">
      {treasuriesError}
    </div>
  {/if}

  <!-- Tier 1: Unboxed 1px Hairline Summary Strip -->
  <section class="summary-strip" aria-label="Consolidated position summary">
    <div class="summary-cell">
      <span class="summary-label">Total Assets</span>
      <span class="summary-value">{isLoadingTreasuries ? '—' : formatMillions(totalAssetsSum)}</span>
      <span class="summary-meta">{treasuryCount} active {treasuryCount === 1 ? 'portfolio' : 'portfolios'}</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">{timeframeHorizon.liabilitiesLabel}</span>
      <span class="summary-value">{isLoadingTreasuries ? '—' : formatMillions(totalLiabilitiesSum)}</span>
      <span class="summary-meta">Debt coverage {coverageRatio}</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">Mandatory Liquidity Floor</span>
      <span class="summary-value">{isLoadingTreasuries ? '—' : `${(avgLiquidityRatio * 100).toFixed(1)}%`}</span>
      <span class="summary-meta">{formatMillions(liquidReserves)} reserved</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">Mean Risk Tolerance</span>
      <span class="summary-value">{isLoadingTreasuries ? '—' : avgRiskScore.toFixed(2)}</span>
      <span class="summary-meta">
        Net capital surplus {netCapitalSurplus >= 0 ? '+' : ''}{formatMillions(netCapitalSurplus)}
      </span>
    </div>
  </section>

  {#if !isLoadingTreasuries && treasuries.length === 0}
    <div class="notice-banner onboarding-row" role="region" aria-label="Onboarding">
      <span>No corporate treasuries exist for this account yet. Create a portfolio to persist assets, debt obligations, and run optimizations.</span>
      {#if canWrite($user?.role)}
        <a href="/treasuries" class="btn btn-primary btn-sm">Create treasury</a>
      {/if}
    </div>
  {/if}

  <!-- Tier 2: 65/35 Assets vs Debt Bar Chart & Primary Portfolio Distribution -->
  <section class="split-row" aria-label="Capital and allocation views">
    <div class="section-pane">
      <div class="pane-head">
        <div>
          <h2>Capital vs. Debt Coverage</h2>
          <p class="pane-sub">{methodologyNote}</p>
        </div>
        <div class="chart-legend">
          <span class="legend-item"><span class="swatch swatch-primary"></span> Assets</span>
          <span class="legend-item"><span class="swatch swatch-muted"></span> {timeframeHorizon.chartLiabilityLabel}</span>
        </div>
      </div>

      {#if isLoadingTreasuries}
        <div class="empty-block">Loading portfolio ledger...</div>
      {:else if topTreasuries.length === 0}
        <div class="empty-block">
          No portfolios on record. <a href="/treasuries">Create a treasury &rarr;</a>
        </div>
      {:else}
        <div class="bar-chart">
          <div class="y-axis font-mono">
            <span>{formatMillions(chartScaleMax)}</span>
            <span>{formatMillions(chartScaleMax * 0.67)}</span>
            <span>{formatMillions(chartScaleMax * 0.33)}</span>
            <span>$0</span>
          </div>
          <div class="bars-field">
            <div class="rule r-100"></div>
            <div class="rule r-66"></div>
            <div class="rule r-33"></div>
            <div class="rule r-0"></div>
            <div class="bars-grid">
              {#each topTreasuries as t}
                {@const assetPct = Math.min(100, Math.max(6, (t.capital / chartScaleMax) * 100))}
                {@const liabPct = t.liability > 0 ? Math.min(100, Math.max(4, (t.liability / chartScaleMax) * 100)) : 2}
                <a
                  href="/treasuries/{t.id}"
                  class="bar-col"
                  title="{t.fullName}: Assets {t.capitalFormatted}, Debt {t.liabilityFormatted}"
                >
                  <div class="bar-pair">
                    <div class="bar-slot">
                      <span class="bar-num font-mono">{t.capitalFormatted}</span>
                      <div class="bar-fill asset" style="height: {assetPct}%;"></div>
                    </div>
                    <div class="bar-slot">
                      <span class="bar-num font-mono">{t.liabilityFormatted}</span>
                      <div class="bar-fill debt" style="height: {liabPct}%;"></div>
                    </div>
                  </div>
                  <span class="bar-name">{t.name}</span>
                </a>
              {/each}
            </div>
          </div>
        </div>
      {/if}

      <div class="pane-foot">
        <span>Click any column to open portfolio workbench</span>
        <a href="/balance-sheet">Inspect balance sheet &rarr;</a>
      </div>
    </div>

    <div class="section-pane">
      <div class="pane-head">
        <div>
          <h2>Primary Portfolio Weights</h2>
          <p class="pane-sub">
            {#if primaryTreasury}
              {primaryTreasury.name} ({primaryTreasury.base_currency})
              &middot; {primaryStrategyAlgo ? `Optimized (${primaryStrategyAlgo})` : 'Equal-weight baseline'}
            {:else}
              No primary portfolio selected
            {/if}
          </p>
        </div>
        <a href="/capital-allocation" class="head-link">Details &rarr;</a>
      </div>

      {#if isLoadingAssets || isLoadingTreasuries}
        <div class="empty-block">Loading allocation weights...</div>
      {:else if !primaryTreasury}
        <div class="empty-block">Create a portfolio to inspect asset weights.</div>
      {:else if allocationSegments.length === 0}
        <div class="empty-block">
          No assets persisted in {primaryTreasury.name}.
          <a href="/treasuries/{primaryTreasury.id}">Add assets &rarr;</a>
        </div>
      {:else}
        <div class="weights-table">
          {#each allocationSegments as seg}
            <button
              type="button"
              class="weight-row"
              class:selected={selectedSegmentId === seg.id}
              onclick={() => (selectedSegmentId = selectedSegmentId === seg.id ? null : seg.id)}
            >
              <div class="weight-top">
                <div class="weight-name-wrap">
                  <span class="weight-swatch" style="background-color: {seg.color};"></span>
                  <span class="weight-name">{seg.name}</span>
                </div>
                <span class="weight-pct font-mono">{seg.percentage}%</span>
              </div>
              <div class="weight-track">
                <div class="weight-progress" style="width: {seg.percentage}%; background-color: {seg.color};"></div>
              </div>
              <div class="weight-sub">
                <span>{seg.type}</span>
                <span class="font-mono">{formatMillions(seg.amount)} &middot; Exp. +{(seg.expectedReturn * 100).toFixed(1)}%</span>
              </div>
            </button>
          {/each}
        </div>
      {/if}
    </div>
  </section>

  <!-- Tier 3: Workhorse Portfolio Table + Operational Ledger -->
  <section class="split-row" aria-label="Portfolios and operational activity">
    <div class="section-pane flush">
      <div class="pane-head padded">
        <div>
          <h2>Active Portfolios</h2>
          <p class="pane-sub">Direct balance sheets accessible under your role ({$user?.role || 'viewer'})</p>
        </div>
        <a href="/treasuries" class="head-link">All treasuries &rarr;</a>
      </div>

      {#if isLoadingTreasuries}
        <div class="empty-block">Loading portfolios...</div>
      {:else if treasuries.length === 0}
        <div class="empty-block">No treasuries on record.</div>
      {:else}
        <div class="table-scroll">
          <table class="table">
            <thead>
              <tr>
                <th>Portfolio</th>
                <th>Currency</th>
                <th class="text-right">Capital</th>
                <th class="text-right">Debt</th>
                <th class="text-right">Min Liquidity</th>
                <th class="text-right">Risk</th>
              </tr>
            </thead>
            <tbody>
              {#each treasuries.slice(0, 8) as t (t.id)}
                {@const liab = calculatePortfolioDebt(liabilitiesMap[t.id]) * timeframeHorizon.liabilitiesMultiplier}
                <tr>
                  <td>
                    <a href="/treasuries/{t.id}" class="row-link">{t.name}</a>
                  </td>
                  <td class="font-mono">{t.base_currency}</td>
                  <td class="text-right font-mono">{formatCurrency(t.total_capital, t.base_currency)}</td>
                  <td class="text-right font-mono">{formatMillions(liab)}</td>
                  <td class="text-right font-mono">{formatPercent(t.min_liquidity_ratio)}</td>
                  <td class="text-right font-mono">{Number(t.risk_tolerance).toFixed(2)}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </div>

    <div class="section-pane stack-pane">
      <div class="sub-block">
        <div class="pane-head">
          <h2>System Services</h2>
          <button type="button" class="text-btn" onclick={() => fetchHealth(true)} disabled={isRefreshing}>
            {isLoadingHealth ? 'Probing...' : 'Ping'}
          </button>
        </div>
        <div class="kv-list">
          <div class="kv-row">
            <span>Go API Gateway (:8080)</span>
            <span class="status-inline">
              <span class="status-dot {isBackendUp ? 'ok' : 'err'}"></span>
              <span class="font-mono">{isBackendUp ? `${latencyMs ?? 0}ms` : 'Offline'}</span>
            </span>
          </div>
          <div class="kv-row">
            <span>PostgreSQL Storage (:5432)</span>
            <span class="status-inline">
              <span class="status-dot {isDbUp ? 'ok' : 'err'}"></span>
              <span class="font-mono">{isDbUp ? 'Up' : 'Down'}</span>
            </span>
          </div>
          <div class="kv-row">
            <span>AI Risk &amp; Search Engine (:8000)</span>
            <span class="status-inline">
              <span class="status-dot {isAiUp ? 'ok' : 'err'}"></span>
              <span class="font-mono">{isAiUp ? 'Active' : 'Offline'}</span>
            </span>
          </div>
          <div class="kv-row">
            <span>QAOA Optimization Engine (:8001)</span>
            <span class="status-inline">
              <span class="status-dot {isOptUp ? 'ok' : 'warn'}"></span>
              <span class="font-mono">{isOptUp ? 'Active' : 'Offline'}</span>
            </span>
          </div>
        </div>
        <p class="stamp-line font-mono">Last checked {formatTime(lastUpdated)}</p>
      </div>

      <div class="sub-block bordered-top">
        <div class="pane-head">
          <h2>Audit Activity</h2>
          {#if $user && isAdmin($user.role)}
            <a href="/admin/audit-logs" class="head-link">Full log &rarr;</a>
          {:else}
            <a href="/reports" class="head-link">Reports &rarr;</a>
          {/if}
        </div>

        {#if !isAdmin($user?.role)}
          <p class="pane-sub">Audit log inspection is restricted to Admin accounts. Portfolio exports remain open in Reports.</p>
        {:else if isLoadingAuditLogs}
          <p class="pane-sub">Loading recent audit events...</p>
        {:else if filteredAuditLogs.length === 0}
          <p class="pane-sub">No events recorded in the {selectedTimeframe} window.</p>
        {:else}
          <div class="audit-rows">
            {#each filteredAuditLogs as log}
              <button type="button" class="audit-row-btn" onclick={() => (selectedAuditLog = log)}>
                <div class="audit-main">
                  <span class="audit-action font-mono">{log.action}</span>
                  <span class="audit-actor">{log.actor}</span>
                </div>
                <span class="audit-time font-mono">{formatTimeAgo(log.created_at)}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    </div>
  </section>
</div>

{#if selectedAuditLog}
  <div
    class="modal-backdrop"
    onclick={() => (selectedAuditLog = null)}
    onkeydown={(e) => { if (e.key === 'Escape') selectedAuditLog = null; }}
    role="presentation"
  >
    <div
      class="modal-card"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
      tabindex="-1"
    >
      <div class="modal-header">
        <div>
          <h3 id="modal-title" class="modal-title font-mono">{selectedAuditLog.action}</h3>
          <p class="modal-subtitle">Audit journal entry</p>
        </div>
        <button type="button" class="modal-close-btn" onclick={() => (selectedAuditLog = null)} aria-label="Close">
          &times;
        </button>
      </div>

      <div class="modal-body">
        <div class="detail-grid">
          <div class="detail-item">
            <span class="detail-label">Actor</span>
            <span class="detail-value" style="font-size: 0.875rem;">{selectedAuditLog.actor}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Entity</span>
            <span class="detail-value" style="font-size: 0.875rem;">{selectedAuditLog.entity_type} {selectedAuditLog.entity_id || ''}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Recorded</span>
            <span class="detail-value" style="font-size: 0.875rem;">{formatDate(selectedAuditLog.created_at)}</span>
          </div>
        </div>

        {#if selectedAuditLog.metadata}
          <pre class="json-code">{JSON.stringify(selectedAuditLog.metadata, null, 2)}</pre>
        {/if}
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-primary btn-sm" onclick={() => (selectedAuditLog = null)}>
          Close
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .onboarding-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .split-row {
    display: grid;
    grid-template-columns: 1.55fr 1fr;
    gap: 1.5rem;
    align-items: start;
  }

  .section-pane {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .section-pane.flush {
    padding: 0;
    gap: 0;
    overflow: hidden;
  }

  .pane-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .pane-head.padded {
    padding: 1.125rem 1.25rem;
    border-bottom: 1px solid var(--color-border);
  }

  .pane-head h2 {
    font-size: 1rem;
    font-weight: 500;
  }

  .pane-sub {
    margin-top: 0.2rem;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .head-link {
    font-size: 0.8125rem;
    font-weight: 500;
    white-space: nowrap;
  }

  .text-btn {
    background: none;
    border: none;
    color: var(--color-primary);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
  }

  .chart-legend {
    display: flex;
    gap: 0.875rem;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .legend-item {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
  }

  .swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }

  .swatch-primary { background-color: var(--color-primary); }
  .swatch-muted { background-color: var(--color-border-strong); }

  .bar-chart {
    display: flex;
    gap: 0.75rem;
    height: 220px;
    padding-top: 0.5rem;
  }

  .y-axis {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
    width: 52px;
    text-align: right;
    padding-bottom: 24px;
  }

  .bars-field {
    position: relative;
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }

  .rule {
    position: absolute;
    left: 0;
    right: 0;
    border-top: 1px dashed var(--color-border-subtle);
  }

  .r-100 { top: 0; }
  .r-66 { top: 33.33%; }
  .r-33 { top: 66.66%; }
  .r-0 { bottom: 24px; border-top: 1px solid var(--color-border); }

  .bars-grid {
    position: relative;
    z-index: 1;
    display: flex;
    justify-content: space-around;
    align-items: flex-end;
    height: 100%;
    padding-bottom: 24px;
  }

  .bar-col {
    display: flex;
    flex-direction: column;
    align-items: center;
    flex: 1;
    max-width: 96px;
    padding: 0.25rem;
    border-radius: var(--radius-sm);
    transition: background-color 140ms var(--ease);
  }

  .bar-col:hover {
    background-color: var(--color-bg-secondary);
  }

  .bar-pair {
    display: flex;
    align-items: flex-end;
    gap: 5px;
    height: 160px;
  }

  .bar-slot {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    height: 100%;
    gap: 3px;
  }

  .bar-num {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
  }

  .bar-fill {
    width: 16px;
    border-radius: 2px 2px 0 0;
  }

  .bar-fill.asset { background-color: var(--color-primary); }
  .bar-fill.debt { background-color: var(--color-border-strong); }

  .bar-name {
    position: absolute;
    bottom: 2px;
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 90px;
  }

  .pane-foot {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 0.75rem;
    border-top: 1px solid var(--color-border-subtle);
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .weights-table {
    display: flex;
    flex-direction: column;
  }

  .weight-row {
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--color-border-subtle);
    padding: 0.7rem 0.25rem;
    cursor: pointer;
    text-align: left;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
    transition: background-color 140ms var(--ease);
  }

  .weight-row:last-child {
    border-bottom: none;
  }

  .weight-row:hover,
  .weight-row.selected {
    background-color: var(--color-surface-hover);
  }

  .weight-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.875rem;
  }

  .weight-name-wrap {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .weight-swatch {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    flex-shrink: 0;
  }

  .weight-name {
    font-weight: 500;
    color: var(--color-text);
  }

  .weight-pct {
    font-weight: 500;
    color: var(--color-text);
  }

  .weight-track {
    height: 4px;
    width: 100%;
    background-color: var(--color-bg-secondary);
    border-radius: 2px;
    overflow: hidden;
  }

  .weight-progress {
    height: 100%;
    border-radius: 2px;
  }

  .weight-sub {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
    text-transform: capitalize;
  }

  .table-scroll {
    overflow-x: auto;
  }

  .row-link {
    font-weight: 500;
    color: var(--color-text);
  }

  .row-link:hover {
    color: var(--color-primary);
    text-decoration: underline;
  }

  .stack-pane {
    gap: 1.25rem;
  }

  .sub-block {
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
  }

  .bordered-top {
    padding-top: 1rem;
    border-top: 1px solid var(--color-border);
  }

  .kv-list {
    display: flex;
    flex-direction: column;
  }

  .kv-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.45rem 0;
    border-bottom: 1px solid var(--color-border-subtle);
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .kv-row:last-child {
    border-bottom: none;
  }

  .stamp-line {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .audit-rows {
    display: flex;
    flex-direction: column;
  }

  .audit-row-btn {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    padding: 0.45rem 0.25rem;
    background: transparent;
    border: none;
    border-bottom: 1px solid var(--color-border-subtle);
    cursor: pointer;
    text-align: left;
  }

  .audit-row-btn:last-child {
    border-bottom: none;
  }

  .audit-row-btn:hover {
    background-color: var(--color-surface-hover);
  }

  .audit-main {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .audit-action {
    font-size: 0.8125rem;
    color: var(--color-text);
  }

  .audit-actor {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .audit-time {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    white-space: nowrap;
  }

  .empty-block {
    padding: 2rem 1rem;
    text-align: center;
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .json-code {
    margin-top: 1rem;
    padding: 0.875rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: 0.8125rem;
    font-family: var(--font-mono);
    overflow-x: auto;
  }

  @media (max-width: 1100px) {
    .split-row {
      grid-template-columns: 1fr;
    }
  }
</style>
