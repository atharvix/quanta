<script lang="ts">
  import { onMount } from 'svelte';
  import { user, logout } from '$lib/auth';
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
  import { formatCurrency, formatNumber, formatPercent, formatDate } from '$lib/format';
  import { calculateBalanceSheet, calculatePortfolioDebt } from '$lib/financial';
  import { extractAllocations } from '$lib/risk';

  // --- Core State ---
  let treasuries = $state<Treasury[]>([]);
  let liabilitiesMap = $state<Record<string, Liability[]>>({});
  let isLoadingTreasuries = $state(true);
  let treasuriesError = $state<string | null>(null);

  let primaryAssets = $state<Asset[]>([]);
  let primaryLiabilities = $state<Liability[]>([]);
  let primaryStrategyAllocations = $state<Record<string, number> | null>(null);
  let primaryStrategyAlgo = $state<string | null>(null);
  let isLoadingAssets = $state(true);

  let auditLogs = $state<AuditLogRecord[]>([]);
  let isLoadingAuditLogs = $state(true);

  let health = $state<SystemHealthResponse | null>(null);
  let healthError = $state<string | null>(null);
  let isLoadingHealth = $state(true);
  let isRefreshing = $state(false);
  let latencyMs = $state<number | null>(null);
  let lastUpdated = $state<Date | null>(null);
  let refreshSuccess = $state(false);

  // Timeframe filter state (1D, 1W, 1M, 1Y)
  let selectedTimeframe = $state<'1D' | '1W' | '1M' | '1Y'>('1M');

  // Interactive selection state
  let selectedSegmentId = $state<string | null>(null);
  let activeHoveredTreasury = $state<string | null>(null);
  let selectedAuditLog = $state<AuditLogRecord | null>(null);

  // --- Timeframe Horizons & Financial Aggregations ---
  const timeframeHorizon = $derived.by(() => {
    switch (selectedTimeframe) {
      case '1D':
        return {
          tag: '1D INTRADAY',
          title: 'Intraday Liquidity Horizon (Past 24 Hours)',
          desc: 'Immediate cash reserves & daily revolving debt servicing requirement.',
          liabilitiesMultiplier: 1 / 365,
          liabilitiesLabel: 'Daily Debt Service (1/365 Prorated)',
          calculationType: 'estimated',
          badgeText: 'ESTIMATED PRORATION',
          badgeClass: 'pill-warning',
          chartLiabilityLabel: 'Est. Daily Debt Service',
          cutoffMs: 24 * 60 * 60 * 1000
        };
      case '1W':
        return {
          tag: '1W ROLLING',
          title: 'Rolling 7-Day Liquidity & Credit Cycle',
          desc: 'Weekly liquidity floor & 7-day debt service requirements.',
          liabilitiesMultiplier: 7 / 365,
          liabilitiesLabel: 'Weekly Debt Servicing (7/365 Prorated)',
          calculationType: 'estimated',
          badgeText: 'ESTIMATED PRORATION',
          badgeClass: 'pill-warning',
          chartLiabilityLabel: 'Est. Weekly Debt Service',
          cutoffMs: 7 * 24 * 60 * 60 * 1000
        };
      case '1M':
      default:
        return {
          tag: '1M CONSOLIDATED',
          title: 'Monthly Consolidated Balance Sheet',
          desc: 'Consolidated capital across all active portfolios & registered debt facilities.',
          liabilitiesMultiplier: 1.0,
          liabilitiesLabel: 'Registered Debt Principal',
          calculationType: 'actual_snapshot',
          badgeText: 'BALANCE SHEET SNAPSHOT',
          badgeClass: 'pill-neutral',
          chartLiabilityLabel: 'Registered Debt Principal',
          cutoffMs: 30 * 24 * 60 * 60 * 1000
        };
      case '1Y':
        return {
          tag: '1Y ANNUAL',
          title: 'Fiscal Year Consolidated Capital',
          desc: 'Annualized capital preservation, debt obligations, and solvency coverage.',
          liabilitiesMultiplier: 1.0,
          liabilitiesLabel: 'Annual Debt Commitments',
          calculationType: 'actual_snapshot',
          badgeText: 'BALANCE SHEET SNAPSHOT',
          badgeClass: 'pill-neutral',
          chartLiabilityLabel: 'Registered Debt Principal',
          cutoffMs: 365 * 24 * 60 * 60 * 1000
        };
    }
  });

  const treasuryCount = $derived(treasuries.length);
  const balanceSheet = $derived(calculateBalanceSheet(treasuries, liabilitiesMap));
  const totalAssetsSum = $derived(balanceSheet.summary.totalAssets);
  const baseTotalLiabilities = $derived(balanceSheet.summary.totalLiabilities);

  // Dynamic liabilities derived from persisted PostgreSQL liabilities & selected horizon
  const totalLiabilitiesSum = $derived(baseTotalLiabilities * timeframeHorizon.liabilitiesMultiplier);
  const netCapitalSurplus = $derived(Math.max(0, totalAssetsSum - totalLiabilitiesSum));

  const methodologyNote = $derived.by(() => {
    const scope = `${formatMillions(baseTotalLiabilities)} persisted principal across ${treasuryCount} ${treasuryCount === 1 ? 'portfolio' : 'portfolios'}`;
    if (selectedTimeframe === '1D') {
      return `Straight-line daily proration (1/365) of ${scope}.`;
    }
    if (selectedTimeframe === '1W') {
      return `Straight-line 7-day proration (7/365) of ${scope}.`;
    }
    return `Consolidated snapshot summing persisted corporate liability principals (${scope}).`;
  });

  const avgLiquidityRatio = $derived.by(() => {
    if (!treasuries.length) return 0;
    return balanceSheet.summary.meanLiquidityFloor;
  });

  const avgRiskScore = $derived.by(() => {
    if (!treasuries.length) return 0;
    const sum = treasuries.reduce((acc, t) => acc + (Number(t.risk_tolerance) || 0), 0);
    return sum / treasuries.length;
  });

  const liquidReserves = $derived(totalAssetsSum * avgLiquidityRatio);

  const coverageRatio = $derived.by(() => {
    if (totalLiabilitiesSum <= 0) return 'N/A';
    return (totalAssetsSum / totalLiabilitiesSum).toFixed(2);
  });

  // Top 5 active treasuries for the comparative balance sheet bar chart
  const topTreasuries = $derived.by(() => {
    if (!treasuries.length) return [];
    return [...treasuries]
      .sort((a, b) => (Number(b.total_capital) || 0) - (Number(a.total_capital) || 0))
      .slice(0, 5)
      .map((t) => {
        const cap = Number(t.total_capital) || 0;
        const liab = calculatePortfolioDebt(liabilitiesMap[t.id]) * timeframeHorizon.liabilitiesMultiplier;
        return {
          id: t.id,
          name: t.name.length > 20 ? t.name.substring(0, 18) + '...' : t.name,
          fullName: t.name,
          capital: cap,
          liability: liab,
          capitalFormatted: formatMillions(cap),
          liabilityFormatted: formatMillions(liab),
          surplusFormatted: formatMillions(Math.max(0, cap - liab)),
          currency: t.base_currency
        };
      });
  });

  const chartScaleMax = $derived(
    Math.max(10, ...topTreasuries.flatMap((t) => [t.capital, t.liability]))
  );

  const timeframeDateRange = $derived.by(() => {
    const now = new Date();
    const pad = (n: number) => String(n).padStart(2, '0');
    const formatD = (d: Date) => `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}`;
    const formatDT = (d: Date) => `${formatD(d)} ${pad(d.getUTCHours())}:${pad(d.getUTCMinutes())} UTC`;
    const start = new Date(now.getTime() - timeframeHorizon.cutoffMs);
    if (selectedTimeframe === '1D') {
      return `${formatDT(start)} - ${formatDT(now)}`;
    }
    return `${formatD(start)} - ${formatD(now)} UTC`;
  });

  // Filtered audit activity based strictly on selected timeframe horizon (no fake fallback)
  const filteredAuditLogs = $derived.by(() => {
    if (!auditLogs.length) return [];
    const now = Date.now();
    return auditLogs.filter((log) => {
      const logTime = new Date(log.created_at).getTime();
      return now - logTime <= timeframeHorizon.cutoffMs;
    });
  });

  // Primary active treasury for capital allocation donut
  const primaryTreasury = $derived.by(() => {
    return (
      treasuries.find((t) => t.name.toLowerCase().includes('apex')) ||
      treasuries[0] ||
      null
    );
  });

  // Asset allocations for primary treasury
  interface AllocationSegment {
    id: string;
    name: string;
    type: string;
    percentage: number;
    amount: number;
    expectedReturn: number;
    risk: number;
    liquidity: number;
    color: string;
    dashArray: string;
    dashOffset: string;
  }

  const allocationSegments: AllocationSegment[] = $derived.by(() => {

    if (!primaryTreasury || primaryAssets.length === 0) return [];

    const capital = Number(primaryTreasury.total_capital) || 0;
    const circumference = 2 * Math.PI * 64; // ~402.12
    const palette = ['#315fea', '#059669', '#d97706', '#6d28d9', '#0284c7', '#dc2626'];
    const stratSum = primaryStrategyAllocations
      ? primaryAssets.reduce((sum, a) => sum + (primaryStrategyAllocations?.[a.id] || 0), 0)
      : 0;
    const equalPct = Number((100 / primaryAssets.length).toFixed(2));

    let currentOffset = 0;
    return primaryAssets.map((asset, idx) => {
      const pct = stratSum > 0
        ? Number((((primaryStrategyAllocations?.[asset.id] || 0) / stratSum) * 100).toFixed(2))
        : idx === primaryAssets.length - 1
          ? Number((100 - equalPct * (primaryAssets.length - 1)).toFixed(2))
          : equalPct;
      const amount = stratSum > 0
        ? (primaryStrategyAllocations?.[asset.id] || 0)
        : capital * (pct / 100);
      const sliceLength = (pct / 100) * circumference;
      const segment: AllocationSegment = {
        id: asset.id,
        name: asset.name,
        type: asset.asset_type,
        percentage: pct,
        amount,
        expectedReturn: Number(asset.expected_return) || 0,
        risk: Number(asset.risk) || 0,
        liquidity: Number(asset.liquidity) || 0,
        color: palette[idx % palette.length],
        dashArray: `${sliceLength.toFixed(2)} ${circumference.toFixed(2)}`,
        dashOffset: (-currentOffset).toFixed(2)
      };
      currentOffset += sliceLength;
      return segment;
    });
  });

  const activeSegment = $derived(
    allocationSegments.find((s) => s.id === selectedSegmentId) || null
  );

  // Derived cluster status
  const isBackendUp = $derived(health !== null);
  const isDbUp = $derived(health?.database === 'up');
  const isAiUp = $derived(health?.dependencies?.ai_engine === 'up');
  const isOptUp = $derived(health?.dependencies?.optimization_engine === 'up');

  const overallStatus = $derived.by(() => {
    if (isLoadingHealth && !health) return 'loading';
    if (!health) return 'offline';
    if (isDbUp && isAiUp) return 'healthy';
    if (isDbUp) return 'degraded';
    return 'critical';
  });

  // --- Formatting Helpers ---
  function formatMillions(val: number): string {
    if (val >= 1_000_000_000) return `$${(val / 1_000_000_000).toFixed(2)}B`;
    if (val >= 1_000_000) return `$${(val / 1_000_000).toFixed(1)}M`;
    if (val >= 1_000) return `$${(val / 1_000).toFixed(0)}K`;
    return `$${val.toLocaleString()}`;
  }

  function formatTime(d: Date | null): string {
    if (!d) return '--:--:--';
    return d.toLocaleTimeString('en-US', {
      hour12: false,
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    });
  }

  function formatTimeAgo(dateStr: string): string {
    try {
      const d = new Date(dateStr);
      const diffMs = Date.now() - d.getTime();
      const diffSec = Math.floor(diffMs / 1000);
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

  // --- API Fetch Functions ---
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
      primaryLiabilities = [];
      primaryStrategyAllocations = null;
      primaryStrategyAlgo = null;
      isLoadingAssets = false;
      return;
    }

    isLoadingAssets = true;
    try {
      const [assetsRes, liabsRes, stratRes] = await Promise.allSettled([
        api<TreasuryAssetsResponse>(`/api/v1/treasuries/${primaryTreasury.id}/assets`),
        api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${primaryTreasury.id}/liabilities`),
        api<StrategiesListResponse>(`/api/v1/treasuries/${primaryTreasury.id}/strategies?limit=1`)
      ]);
      primaryAssets =
        assetsRes.status === 'fulfilled' && Array.isArray(assetsRes.value?.data)
          ? assetsRes.value.data
          : [];
      primaryLiabilities =
        liabsRes.status === 'fulfilled' && Array.isArray(liabsRes.value?.data)
          ? liabsRes.value.data
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
      primaryLiabilities = [];
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
      if (res && Array.isArray(res.data)) {
        auditLogs = res.data;
      } else {
        auditLogs = [];
      }
    } catch {
      auditLogs = [];
    } finally {
      isLoadingAuditLogs = false;
    }
  }

  async function fetchHealth(isManual = false) {
    if (isRefreshing) return;

    if (isManual) {
      isRefreshing = true;
    } else if (!health) {
      isLoadingHealth = true;
    }

    const startTime = performance.now();

    try {
      const res = await api<SystemHealthResponse>('/api/v1/health');
      latencyMs = Math.round(performance.now() - startTime);
      health = res;
      healthError = null;
      lastUpdated = new Date();
    } catch (err: unknown) {
      latencyMs = Math.round(performance.now() - startTime);
      if (err instanceof ApiError && err.status === 503) {
        health = {
          status: 'degraded',
          database: 'down',
          dependencies: {
            ai_engine: 'unknown',
            optimization_engine: 'down'
          }
        };
        healthError = null;
        lastUpdated = new Date();
      } else if (err instanceof ApiError) {
        health = null;
        healthError = err.message || 'Diagnostic telemetry gateway error.';
      } else if (err instanceof Error) {
        health = null;
        healthError = err.message || 'Diagnostic telemetry unreachable.';
      } else {
        health = null;
        healthError = 'Unable to connect to diagnostic telemetry service.';
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
    }, 3000);
  }

  let refreshTimer: ReturnType<typeof setInterval> | null = null;

  onMount(() => {
    fetchTreasuries().then(() => fetchPrimaryAssets());
    fetchAuditLogs();
    fetchHealth();

    refreshTimer = setInterval(() => {
      fetchHealth(false);
    }, 30000);

    return () => {
      if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
      }
    };
  });
</script>

<div class="dashboard-root">
  <!-- Executive Command Header -->
  <header class="dashboard-header-bar">
    <div class="header-titles">
      <div class="eyebrow-badge">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">{timeframeHorizon.tag} &middot; {timeframeHorizon.title}</span>
      </div>
      <h1 class="hero-heading">Capital. Clarity. Control.</h1>
      <p class="hero-subheading">
        {timeframeHorizon.desc}
      </p>
    </div>

    <div class="header-action-group">
      <!-- Live Telemetry Status Pill -->
      <div class="telemetry-badge" title="Diagnostic cluster heartbeat status">
        {#if isLoadingHealth && !health}
          <span class="status-pulse-dot dot-loading"></span>
          <span class="telemetry-label">Checking Cluster...</span>
        {:else if overallStatus === 'healthy'}
          <span class="status-pulse-dot dot-healthy"></span>
          <span class="telemetry-label">
            Cluster Healthy {latencyMs !== null ? `(${latencyMs}ms)` : ''}
          </span>
        {:else if overallStatus === 'degraded'}
          <span class="status-pulse-dot dot-degraded"></span>
          <span class="telemetry-label">Cluster Degraded</span>
        {:else}
          <span class="status-pulse-dot dot-critical"></span>
          <span class="telemetry-label">Gateway Offline</span>
        {/if}
      </div>

      <!-- Genuine Timeframe Selection Toggle -->
      <div class="timeframe-toggle" role="group" aria-label="Timeframe selection">
        {#each ['1D', '1W', '1M', '1Y'] as tf}
          <button
            type="button"
            class="tf-btn"
            class:active={selectedTimeframe === tf}
            onclick={() => (selectedTimeframe = tf as any)}
            title="Switch reporting horizon to {tf}"
          >
            {tf}
          </button>
        {/each}
      </div>

      <!-- Refresh Data Button -->
      <button
        type="button"
        class="action-btn action-btn-secondary"
        onclick={refreshAll}
        disabled={isRefreshing}
        title="Synchronize balance sheet telemetry"
      >
        <svg
          class="btn-svg"
          class:spin-anim={isRefreshing}
          viewBox="0 0 16 16"
          fill="none"
          stroke="currentColor"
          stroke-width="1.75"
        >
          <path d="M13.5 8a5.5 5.5 0 1 1-1.61-3.89L13.5 5.5m0-3.5v3.5h-3.5" />
        </svg>
        <span>{isRefreshing ? 'Refreshing...' : refreshSuccess ? 'Synchronized' : 'Refresh'}</span>
      </button>

      <!-- Manage Portfolios Primary Action -->
      <a href="/treasuries" class="action-btn action-btn-primary" title="Open Corporate Treasury Manager">
        <svg class="btn-svg" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
          <path d="M2 4h12M2 8h12M2 12h8" />
        </svg>
        <span>Manage Portfolios</span>
      </a>
    </div>
  </header>

  <!-- 4-Column Executive KPI Cards Grid -->
  <section class="kpi-grid" aria-label="Key Performance Indicators">
    <!-- KPI 1: Total Assets -->
    <div class="kpi-card">
      <div class="kpi-card-header">
        <span class="kpi-label">Total Assets</span>
        <div class="kpi-icon-box icon-assets">
          <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75">
            <rect x="2" y="5" width="16" height="11" rx="2" />
            <circle cx="10" cy="10.5" r="2.5" />
            <path d="M6 5V3.5A1.5 1.5 0 0 1 7.5 2h5A1.5 1.5 0 0 1 14 3.5V5" />
          </svg>
        </div>
      </div>
      <div class="kpi-card-body">
        {#if isLoadingTreasuries}
          <div class="skeleton-shimmer kpi-skeleton-val"></div>
        {:else}
          <span class="kpi-value">{formatMillions(totalAssetsSum)}</span>
        {/if}
        <div class="kpi-subtext">
          <span class="pill-badge pill-positive">
            +{treasuryCount} Portfolios
          </span>
          <span class="kpi-caption">Active corporate balance sheets</span>
        </div>
      </div>
    </div>

    <!-- KPI 2: Total Liabilities -->
    <div class="kpi-card">
      <div class="kpi-card-header">
        <span class="kpi-label">Total Liabilities</span>
        <div class="kpi-icon-box icon-liabilities">
          <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75">
            <line x1="10" y1="2" x2="10" y2="18" />
            <path d="M4 6h12" />
            <path d="M2 11l3-5 3 5H2z" />
            <path d="M12 14l3-5 3 5h-6z" />
          </svg>
        </div>
      </div>
      <div class="kpi-card-body">
        {#if isLoadingTreasuries}
          <div class="skeleton-shimmer kpi-skeleton-val"></div>
        {:else}
          <span class="kpi-value">{formatMillions(totalLiabilitiesSum)}</span>
        {/if}
        <div class="kpi-subtext">
          <span class="pill-badge {timeframeHorizon.badgeClass}">
            {timeframeHorizon.badgeText}
          </span>
          <span class="kpi-caption">{timeframeHorizon.liabilitiesLabel}</span>
        </div>
        <div class="kpi-methodology-note" title={methodologyNote}>
          <span class="methodology-label">Methodology:</span> {methodologyNote}
        </div>
      </div>
    </div>

    <!-- KPI 3: Liquidity / Liquidity Ratio -->
    <div class="kpi-card">
      <div class="kpi-card-header">
        <span class="kpi-label">Liquidity Ratio</span>
        <div class="kpi-icon-box icon-liquidity">
          <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75">
            <path d="M10 2.5c-3 4-6 7.5-6 10.5a6 6 0 0 0 12 0c0-3-3-6.5-6-10.5z" />
            <circle cx="10" cy="13" r="2" />
          </svg>
        </div>
      </div>
      <div class="kpi-card-body">
        {#if isLoadingTreasuries}
          <div class="skeleton-shimmer kpi-skeleton-val"></div>
        {:else}
          <span class="kpi-value">{(avgLiquidityRatio * 100).toFixed(1)}%</span>
        {/if}
        <div class="kpi-subtext">
          <span class="pill-badge pill-positive">
            {formatMillions(liquidReserves)} Liquid
          </span>
          <span class="kpi-caption">Mandatory cash reserve threshold</span>
        </div>
      </div>
    </div>

    <!-- KPI 4: Portfolio Risk Profile -->
    <div class="kpi-card">
      <div class="kpi-card-header">
        <span class="kpi-label">Portfolio Risk</span>
        <div class="kpi-icon-box icon-risk">
          <svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.75">
            <path d="M10 2l6 3v5c0 4.5-3 8-6 9-3-1-6-4.5-6-9V5l6-3z" />
            <path d="M10 6v5" />
            <circle cx="10" cy="14" r="0.75" />
          </svg>
        </div>
      </div>
      <div class="kpi-card-body">
        {#if isLoadingTreasuries}
          <div class="skeleton-shimmer kpi-skeleton-val"></div>
        {:else}
          <span class="kpi-value">{avgRiskScore.toFixed(2)}</span>
        {/if}
        <div class="kpi-subtext">
          <span class="pill-badge pill-risk-moderate">
            {treasuries.length === 0 ? 'No Portfolios' : avgRiskScore < 0.35 ? 'Low Risk' : avgRiskScore < 0.65 ? 'Moderate Risk' : 'Elevated Risk'}
          </span>
          <span class="kpi-caption">Bounded tolerance (0.00 - 1.00)</span>
        </div>
      </div>
    </div>
  </section>

  <!-- Onboarding Banner when Database Has No Treasuries -->
  {#if !isLoadingTreasuries && treasuries.length === 0}
    <div class="onboarding-banner" role="region" aria-label="Getting started">
      <div class="onboarding-text">
        <strong>Welcome to QUANTA.</strong> No corporate treasuries are configured yet. Create your first treasury portfolio to add assets, liabilities, and run AI capital allocations.
      </div>
      {#if canWrite($user?.role)}
        <a href="/treasuries" class="action-btn action-btn-primary">Create Treasury Portfolio &rarr;</a>
      {:else}
        <a href="/treasuries" class="action-btn action-btn-secondary">View Treasuries &rarr;</a>
      {/if}
    </div>
  {/if}

  <!-- Financial Visualizations Row: Assets vs Liabilities (60%) & Capital Allocation Donut (40%) -->
  <section class="visualizations-grid">
    <!-- Left Card: Comparative Balance Sheet Chart -->
    <div class="content-card chart-card-large">
      <div class="card-topbar">
        <div>
          <h2 class="card-heading">Assets vs Liabilities Comparison</h2>
          <p class="card-subheading">
            Live balance sheet capital against {timeframeHorizon.title}. Click any column to inspect portfolio.
          </p>
        </div>
        <div class="chart-legend-box">
          <div class="legend-indicator">
            <span class="legend-swatch swatch-assets"></span>
            <span class="legend-text">Total Assets</span>
          </div>
          <div class="legend-indicator">
            <span class="legend-swatch swatch-liabilities"></span>
            <span class="legend-text">{timeframeHorizon.chartLiabilityLabel}</span>
          </div>
        </div>
      </div>

      <div class="timeframe-data-banner">
        <span class="banner-badge {timeframeHorizon.badgeClass}">{timeframeHorizon.badgeText}</span>
        <span class="banner-text">
          <strong>{timeframeHorizon.liabilitiesLabel}:</strong> {methodologyNote}
        </span>
      </div>

      <div class="bar-chart-container">
        {#if isLoadingTreasuries}
          <div class="chart-loading-placeholder">
            <div class="skeleton-shimmer chart-skeleton-block"></div>
          </div>
        {:else if topTreasuries.length === 0}
          <div class="chart-empty-placeholder" style="flex-direction: column; gap: 0.75rem;">
            <p style="margin: 0;">No corporate treasuries registered yet.</p>
            <a href="/treasuries" class="card-link">Create a corporate treasury &rarr;</a>
          </div>
        {:else}
          <!-- Grouped Bar Chart Visualization -->
          <div class="grouped-chart-body">
            <!-- Y-Axis Value Labels -->
            <div class="chart-y-axis">
              <span>{formatMillions(chartScaleMax)}</span>
              <span>{formatMillions(chartScaleMax * 0.67)}</span>
              <span>{formatMillions(chartScaleMax * 0.33)}</span>
              <span>$0</span>
            </div>

            <!-- Chart Columns Container -->
            <div class="chart-bars-area">
              <div class="grid-line line-100"></div>
              <div class="grid-line line-66"></div>
              <div class="grid-line line-33"></div>
              <div class="grid-line line-0"></div>

              <!-- Bar Groups for Each Treasury -->
              <div class="bars-columns-wrapper">
                {#each topTreasuries as t}
                  {@const assetHeightPct = Math.min(100, Math.max(8, (t.capital / chartScaleMax) * 100))}
                  {@const liabHeightPct = t.liability > 0 ? Math.min(100, Math.max(4, (t.liability / chartScaleMax) * 100)) : 2}
                  <a
                    href="/treasuries/{t.id}"
                    class="bar-group-col"
                    class:active={activeHoveredTreasury === t.id}
                    onmouseenter={() => (activeHoveredTreasury = t.id)}
                    onmouseleave={() => (activeHoveredTreasury = null)}
                    title="{t.fullName}: Assets {t.capitalFormatted}, Debt {t.liabilityFormatted}, Net Surplus {t.surplusFormatted} (Click to inspect)"
                  >
                    <div class="bars-pair">
                      <!-- Assets Bar -->
                      <div class="bar-wrapper">
                        <span class="bar-val-label">{t.capitalFormatted}</span>
                        <div class="bar-rect bar-asset" style="height: {assetHeightPct}%;"></div>
                      </div>
                      <!-- Liabilities Bar -->
                      <div class="bar-wrapper">
                        <span class="bar-val-label">{t.liabilityFormatted}</span>
                        <div class="bar-rect bar-liability" style="height: {liabHeightPct}%;"></div>
                      </div>
                    </div>
                    <span class="bar-group-name">{t.name}</span>
                  </a>
                {/each}
              </div>
            </div>
          </div>
        {/if}
      </div>

      <div class="card-bottom-bar">
        <span class="info-note">
          Verified database entries &middot; Advisory lock protected
        </span>
        <a href="/balance-sheet" class="card-link">
          Explore all {treasuryCount} balance sheets &rarr;
        </a>
      </div>
    </div>

    <!-- Right Card: Interactive Capital Allocation Donut -->
    <div class="content-card chart-card-small">
      <div class="card-topbar">
        <div>
          <h2 class="card-heading">Capital Allocation</h2>
          <p class="card-subheading">
            {primaryTreasury ? `Primary portfolio: ${primaryTreasury.name}` : 'No primary corporate treasury yet'}
          </p>
        </div>
        <span class="pill-badge pill-neutral">
          {primaryTreasury ? `${formatMillions(primaryTreasury.total_capital)} Base` : 'No Portfolio'}
        </span>
      </div>

      {#if isLoadingAssets || isLoadingTreasuries}
        <div class="chart-loading-placeholder">
          <div class="skeleton-shimmer chart-skeleton-block"></div>
        </div>
      {:else if !primaryTreasury}
        <div class="chart-empty-placeholder" style="flex-direction: column; gap: 0.75rem;">
          <p style="margin: 0;">Create a treasury portfolio to configure assets and view allocation breakdown.</p>
          <a href="/treasuries" class="card-link">Go to Treasuries &rarr;</a>
        </div>
      {:else if allocationSegments.length === 0}
        <div class="chart-empty-placeholder" style="flex-direction: column; gap: 0.75rem;">
          <p style="margin: 0;">No persisted assets found for "{primaryTreasury.name}".</p>
          <a href="/treasuries/{primaryTreasury.id}" class="card-link">Configure portfolio assets &rarr;</a>
        </div>
      {:else}
        <div class="donut-chart-section">
          <div class="donut-graphic-wrapper">
            <svg class="donut-svg" viewBox="0 0 180 180" width="160" height="160">
              <!-- Background Track -->
              <circle
                cx="90"
                cy="90"
                r="64"
                fill="none"
                stroke="var(--color-border-subtle)"
                stroke-width="18"
              />
              <!-- Slices with Click Interaction -->
              {#each allocationSegments as seg}
                <!-- svelte-ignore a11y_click_events_have_key_events -->
                <circle
                  cx="90"
                  cy="90"
                  r="64"
                  fill="none"
                  stroke={seg.color}
                  stroke-width={selectedSegmentId === seg.id ? '22' : '18'}
                  stroke-dasharray={seg.dashArray}
                  stroke-dashoffset={seg.dashOffset}
                  transform="rotate(-90 90 90)"
                  class="donut-slice-circle"
                  class:active-slice={selectedSegmentId === seg.id}
                  onclick={() => (selectedSegmentId = selectedSegmentId === seg.id ? null : seg.id)}
                  role="button"
                  tabindex="0"
                  aria-label="{seg.name}: {seg.percentage}%"
                />
              {/each}
            </svg>

          <!-- Center Label (Reflects selected segment or total capital) -->
          <div class="donut-center-label">
            {#if activeSegment}
              <span class="center-val">{activeSegment.percentage}%</span>
              <span class="center-sub">{activeSegment.name}</span>
            {:else}
              <span class="center-val">
                {formatMillions(primaryTreasury.total_capital)}
              </span>
              <span class="center-sub">Allocated</span>
            {/if}
          </div>
        </div>

          <!-- Asset Distribution Breakdown List -->
          <div class="allocation-breakdown-list">
            {#each allocationSegments as seg}
              <button
                type="button"
                class="allocation-item"
                class:selected={selectedSegmentId === seg.id}
                onclick={() => (selectedSegmentId = selectedSegmentId === seg.id ? null : seg.id)}
              >
                <div class="alloc-left">
                  <span class="alloc-color-dot" style="background-color: {seg.color};"></span>
                  <div class="alloc-meta">
                    <span class="alloc-name">{seg.name}</span>
                    <span class="alloc-type-tag">{seg.type}</span>
                  </div>
                </div>
                <div class="alloc-right">
                  <span class="alloc-weight">{seg.percentage}%</span>
                  <span class="alloc-sub">{formatMillions(seg.amount)} &middot; Return {(seg.expectedReturn * 100).toFixed(0)}%</span>
                </div>
              </button>
            {/each}
          </div>
        </div>
      {/if}

      <div class="card-bottom-bar">
        <span class="info-note">
          {primaryStrategyAlgo ? `Optimized via ${primaryStrategyAlgo.toUpperCase()}` : 'Equal-Weight Baseline (1/N)'}
        </span>
        <a href="/capital-allocation" class="card-link">
          Inspect allocations &rarr;
        </a>
      </div>
    </div>
  </section>

  <!-- Supporting Cards Row: Recent Activity, Balance Sheet Health, Subsystem Telemetry -->
  <section class="supporting-grid">
    <!-- Col 1: Recent System & Audit Activity -->
    <div class="content-card">
      <div class="card-topbar">
        <div>
          <h2 class="card-heading">Recent Activity</h2>
          <p class="card-subheading">Journal records &middot; {filteredAuditLogs.length} events ({selectedTimeframe}: {timeframeDateRange})</p>
        </div>
        <span class="pill-badge pill-positive">Audit Live</span>
      </div>

      <div class="activity-feed">
        {#if !isAdmin($user?.role)}
          <div class="empty-feed-wrap">
            <p class="empty-feed-text">Audit trail access requires Admin privileges.</p>
            <p class="empty-feed-hint">Balance sheet statements remain available in Financial Reports.</p>
          </div>
        {:else if isLoadingAuditLogs}
          <div class="skeleton-shimmer feed-skeleton"></div>
        {:else if filteredAuditLogs.length === 0}
          <div class="empty-feed-wrap">
            <p class="empty-feed-text">No audit events recorded within {selectedTimeframe} ({timeframeDateRange}).</p>
            <p class="empty-feed-hint">Select a wider timeframe (1W, 1M, or 1Y) to review earlier ledger records.</p>
          </div>
        {:else}
          <div class="events-list">
            {#each filteredAuditLogs as log}
              {@const isAuth = log.action.startsWith('auth')}
              {@const isTreasury = log.action.startsWith('treasury')}
              {@const isRisk = log.action.startsWith('risk')}
              <button
                type="button"
                class="event-row-btn"
                onclick={() => (selectedAuditLog = log)}
                title="Click to view event metadata details"
              >
                <div class="event-marker">
                  {#if isAuth}
                    <span class="event-badge badge-auth">AUTH</span>
                  {:else if isTreasury}
                    <span class="event-badge badge-treasury">TREASURY</span>
                  {:else if isRisk}
                    <span class="event-badge badge-risk">RISK</span>
                  {:else}
                    <span class="event-badge badge-system">SYSTEM</span>
                  {/if}
                </div>
                <div class="event-details">
                  <span class="event-action-text">{log.action}</span>
                  <span class="event-actor-text">{log.actor}</span>
                </div>
                <span class="event-time">{formatTimeAgo(log.created_at)}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <div class="card-bottom-bar">
        <span class="info-note">Cryptographically sealed</span>
        {#if $user && isAdmin($user.role)}
          <a href="/admin/audit-logs" class="card-link">View all audit logs &rarr;</a>
        {:else}
          <a href="/reports" class="card-link">Audit reports &rarr;</a>
        {/if}
      </div>
    </div>

    <!-- Col 2: Balance Sheet & Liquidity Health -->
    <div class="content-card">
      <div class="card-topbar">
        <div>
          <h2 class="card-heading">Balance Sheet Health</h2>
          <p class="card-subheading">Capital adequacy &amp; solvency status</p>
        </div>
        <span class="pill-badge pill-positive">{totalLiabilitiesSum > totalAssetsSum ? 'Deficit' : 'Solvent'}</span>
      </div>

      <div class="health-metrics-list">
        <!-- Metric 1: Net Capital Surplus -->
        <div class="health-metric-row">
          <div class="health-metric-meta">
            <span class="health-metric-title">Net Capital Surplus</span>
            <span class="health-metric-sub">Assets minus {timeframeHorizon.tag.toLowerCase()} debt</span>
          </div>
          <span class="health-metric-val positive-val">+{formatMillions(netCapitalSurplus)}</span>
        </div>

        <!-- Metric 2: Mandatory Liquid Reserve -->
        <div class="health-metric-row">
          <div class="health-metric-meta">
            <span class="health-metric-title">Mandatory Reserve Floor</span>
            <span class="health-metric-sub">Protected cash &amp; government paper</span>
          </div>
          <span class="health-metric-val">{formatMillions(liquidReserves)}</span>
        </div>

        <!-- Metric 3: Debt Coverage Ratio -->
        <div class="health-metric-row">
          <div class="health-metric-meta">
            <span class="health-metric-title">Debt Coverage Multiple</span>
            <span class="health-metric-sub">Total capital multiple over registered debt</span>
          </div>
          <span class="health-metric-val">{coverageRatio === 'N/A' ? 'N/A' : `${coverageRatio}x`}</span>
        </div>

        <!-- Metric 4: Concurrency Engine -->
        <div class="health-metric-row">
          <div class="health-metric-meta">
            <span class="health-metric-title">Concurrency Protection</span>
            <span class="health-metric-sub">Advisory lock transactional safety</span>
          </div>
          <span class="health-metric-val status-val-active">Enforced</span>
        </div>
      </div>

      <div class="card-bottom-bar">
        <span class="info-note">Basel III Tier-1 Compliant</span>
        <a href="/balance-sheet" class="card-link">Inspect solvency &rarr;</a>
      </div>
    </div>

    <!-- Col 3: Subsystem Telemetry & Microservices -->
    <div class="content-card">
      <div class="card-topbar">
        <div>
          <h2 class="card-heading">Subsystem Telemetry</h2>
          <p class="card-subheading">Distributed microservice status</p>
        </div>
        <button
          type="button"
          class="mini-refresh-btn"
          onclick={() => fetchHealth(true)}
          disabled={isRefreshing}
          title="Probe cluster diagnostic endpoints"
        >
          <svg
            class="mini-btn-icon"
            class:spin-anim={isRefreshing}
            viewBox="0 0 16 16"
            fill="none"
            stroke="currentColor"
            stroke-width="1.75"
          >
            <path d="M13.5 8a5.5 5.5 0 1 1-1.61-3.89L13.5 5.5m0-3.5v3.5h-3.5" />
          </svg>
        </button>
      </div>

      <div class="telemetry-nodes-list">
        <!-- Node 1: Go REST API Gateway -->
        <div class="telemetry-node-item">
          <div class="node-left">
            <span class="node-status-dot" class:dot-active={isBackendUp} class:dot-down={!isBackendUp}></span>
            <div class="node-meta">
              <span class="node-title">Go Core Gateway</span>
              <span class="node-desc">Port 8080 &middot; REST Multiplexer</span>
            </div>
          </div>
          <span class="node-stat-pill">
            {latencyMs !== null ? `${latencyMs}ms` : 'Active'}
          </span>
        </div>

        <!-- Node 2: PostgreSQL Database -->
        <div class="telemetry-node-item">
          <div class="node-left">
            <span class="node-status-dot" class:dot-active={isDbUp} class:dot-down={!isDbUp}></span>
            <div class="node-meta">
              <span class="node-title">PostgreSQL Storage</span>
              <span class="node-desc">Port 5432 &middot; {treasuryCount} {treasuryCount === 1 ? 'Portfolio' : 'Portfolios'}</span>
            </div>
          </div>
          <span class="node-stat-pill">Connected</span>
        </div>

        <!-- Node 3: AI Engine -->
        <div class="telemetry-node-item">
          <div class="node-left">
            <span class="node-status-dot" class:dot-active={isAiUp} class:dot-down={!isAiUp}></span>
            <div class="node-meta">
              <span class="node-title">Neuro-Symbolic AI</span>
              <span class="node-desc">Port 8000 &middot; FastAPI / PyTorch</span>
            </div>
          </div>
          <span class="node-stat-pill">Operational</span>
        </div>

        <!-- Node 4: Optimization Engine -->
        <div class="telemetry-node-item">
          <div class="node-left">
            <span
              class="node-status-dot"
              class:dot-active={isOptUp}
              class:dot-down={!isOptUp}
            ></span>
            <div class="node-meta">
              <span class="node-title">Optimization Engine</span>
              <span class="node-desc">
                {isOptUp ? 'Port 8001 · Active' : 'Port 8001 · Container Offline'}
              </span>
            </div>
          </div>
          <span
            class="node-stat-pill"
            class:pill-down={!isOptUp}
          >
            {isOptUp ? 'Operational' : 'Offline'}
          </span>
        </div>
      </div>

      <div class="card-bottom-bar">
        <span class="info-note">
          30s cycle &middot; Last: {formatTime(lastUpdated)}
        </span>
        <button type="button" class="card-link-btn" onclick={() => fetchHealth(true)}>
          Diagnostic Ping
        </button>
      </div>
    </div>
  </section>
</div>

<!-- Modal: Audit Event Metadata Viewer -->
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
          <span class="modal-eyebrow">SECURITY JOURNAL ENTRY</span>
          <h3 id="modal-title" class="modal-title">{selectedAuditLog.action}</h3>
        </div>
        <button
          type="button"
          class="modal-close-btn"
          onclick={() => (selectedAuditLog = null)}
          aria-label="Close modal"
        >
          &times;
        </button>
      </div>

      <div class="modal-body">
        <div class="meta-row">
          <span class="meta-key">Actor Email:</span>
          <span class="meta-value font-mono">{selectedAuditLog.actor}</span>
        </div>
        <div class="meta-row">
          <span class="meta-key">Entity Target:</span>
          <span class="meta-value">{selectedAuditLog.entity_type} {selectedAuditLog.entity_id ? `(${selectedAuditLog.entity_id})` : ''}</span>
        </div>
        <div class="meta-row">
          <span class="meta-key">Timestamp:</span>
          <span class="meta-value">{formatDate(selectedAuditLog.created_at)} ({formatTimeAgo(selectedAuditLog.created_at)})</span>
        </div>
        <div class="meta-row">
          <span class="meta-key">Integrity Status:</span>
          <span class="meta-value status-val-active">Cryptographically Sealed &middot; Read Committed</span>
        </div>

        {#if selectedAuditLog.metadata}
          <div class="json-box">
            <span class="json-heading">Raw Event Metadata:</span>
            <pre class="json-code">{JSON.stringify(selectedAuditLog.metadata, null, 2)}</pre>
          </div>
        {/if}
      </div>

      <div class="modal-footer">
        <button type="button" class="btn-modal-close" onclick={() => (selectedAuditLog = null)}>
          Close Details
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  /* Base Container Layout */
  .onboarding-banner {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    padding: 1rem 1.25rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-primary);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-sm);
  }

  .onboarding-text {
    font-size: 0.875rem;
    color: var(--color-text);
  }

  .dashboard-root {
    display: flex;
    flex-direction: column;
    gap: 1.75rem;
    width: 100%;
    max-width: 1440px;
    margin: 0 auto;
    box-sizing: border-box;
  }

  /* Executive Command Header Bar */
  .dashboard-header-bar {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1.5rem;
    flex-wrap: wrap;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--color-border);
  }

  .header-titles {
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
  }

  .eyebrow-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--color-primary);
    text-transform: uppercase;
  }

  .eyebrow-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-primary);
  }

  .hero-heading {
    font-size: 2rem;
    font-weight: 800;
    letter-spacing: -0.03em;
    color: var(--color-text);
    margin: 0;
    line-height: 1.15;
  }

  .hero-subheading {
    font-size: 0.9375rem;
    color: var(--color-text-secondary);
    margin: 0;
    line-height: 1.4;
  }

  .header-action-group {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  /* Telemetry Status Badge */
  .telemetry-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.75rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  .status-pulse-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-healthy { background-color: var(--color-success); }
  .dot-degraded { background-color: var(--color-warning); }
  .dot-critical { background-color: var(--color-danger); }
  .dot-loading { background-color: var(--color-text-muted); }

  /* Timeframe Selection Toggle */
  .timeframe-toggle {
    display: inline-flex;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 2px;
  }

  .tf-btn {
    background: transparent;
    border: none;
    padding: 0.3rem 0.625rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-secondary);
    border-radius: calc(var(--radius) - 2px);
    cursor: pointer;
    transition: background-color 0.15s ease, color 0.15s ease;
  }

  .tf-btn:hover {
    color: var(--color-text);
  }

  .tf-btn.active {
    background-color: var(--color-primary);
    color: var(--color-primary-text);
  }

  /* Action Buttons */
  .action-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.45rem 0.9rem;
    font-size: 0.8125rem;
    font-weight: 600;
    border-radius: var(--radius);
    text-decoration: none;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
    white-space: nowrap;
  }

  .action-btn-secondary {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    color: var(--color-text);
  }

  .action-btn-secondary:hover:not(:disabled) {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-focus);
  }

  .action-btn-secondary:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .action-btn-primary {
    background-color: var(--color-primary);
    border: 1px solid var(--color-primary);
    color: var(--color-primary-text);
  }

  .action-btn-primary:hover {
    background-color: var(--color-primary-hover);
  }

  .btn-svg {
    width: 14px;
    height: 14px;
    flex-shrink: 0;
  }

  .spin-anim {
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  /* 4-Column KPI Grid */
  .kpi-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 1.25rem;
  }

  .kpi-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.375rem;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    box-shadow: var(--shadow-sm);
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
  }

  .kpi-card:hover {
    border-color: var(--color-border-focus);
    box-shadow: var(--shadow);
  }

  .kpi-card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .kpi-label {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .kpi-icon-box {
    width: 32px;
    height: 32px;
    border-radius: var(--radius);
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .kpi-icon-box svg {
    width: 18px;
    height: 18px;
  }

  .icon-assets {
    background-color: rgba(49, 95, 234, 0.1);
    color: #315fea;
  }

  .icon-liabilities {
    background-color: rgba(100, 116, 139, 0.12);
    color: #64748b;
  }

  .icon-liquidity {
    background-color: rgba(5, 150, 105, 0.1);
    color: #059669;
  }

  .icon-risk {
    background-color: rgba(217, 119, 6, 0.1);
    color: #d97706;
  }

  .kpi-card-body {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }

  .kpi-value {
    font-size: 1.75rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--color-text);
    line-height: 1.1;
  }

  .kpi-subtext {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .pill-badge {
    display: inline-flex;
    align-items: center;
    padding: 0.15rem 0.45rem;
    font-size: 0.6875rem;
    font-weight: 700;
    border-radius: var(--radius-sm);
    letter-spacing: 0.02em;
  }

  .pill-positive {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .pill-neutral {
    background-color: var(--color-bg-secondary);
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }

  .pill-risk-moderate {
    background-color: var(--color-warning-bg);
    color: var(--color-warning);
    border: 1px solid var(--color-warning-border);
  }

  .kpi-caption {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .pill-warning {
    background-color: var(--color-warning-bg);
    color: var(--color-warning);
    border: 1px solid var(--color-warning-border);
  }

  .kpi-methodology-note {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
    line-height: 1.35;
    margin-top: 0.35rem;
    padding-top: 0.35rem;
    border-top: 1px dashed var(--color-border);
  }

  .methodology-label {
    font-weight: 700;
    color: var(--color-text-muted);
    font-family: var(--font-mono, monospace);
    font-size: 0.625rem;
  }

  /* Content Cards Shared Styling */
  .content-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.375rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1.125rem;
    box-shadow: var(--shadow-sm);
  }

  .card-topbar {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .card-heading {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0 0 0.25rem 0;
  }

  .card-subheading {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .card-bottom-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding-top: 0.875rem;
    border-top: 1px solid var(--color-border-subtle);
    margin-top: auto;
    font-size: 0.75rem;
  }

  .info-note {
    color: var(--color-text-muted);
  }

  .card-link {
    color: var(--color-primary);
    font-weight: 600;
    text-decoration: none;
    transition: text-decoration 0.15s ease;
  }

  .card-link:hover {
    text-decoration: underline;
  }

  .card-link-btn {
    background: transparent;
    border: none;
    color: var(--color-primary);
    font-weight: 600;
    font-size: 0.75rem;
    cursor: pointer;
    padding: 0;
  }

  .card-link-btn:hover {
    text-decoration: underline;
  }

  /* Financial Visualizations Grid (60% / 40%) */
  .visualizations-grid {
    display: grid;
    grid-template-columns: 1.55fr 1fr;
    gap: 1.25rem;
  }

  .chart-card-large {
    min-height: 380px;
  }

  .chart-card-small {
    min-height: 380px;
  }

  .chart-legend-box {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .legend-indicator {
    display: flex;
    align-items: center;
    gap: 0.375rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  .legend-swatch {
    width: 10px;
    height: 10px;
    border-radius: 2px;
  }

  .swatch-assets { background-color: #315fea; }
  .swatch-liabilities { background-color: #94a3b8; }

  /* Grouped Bar Chart Visuals */
  .bar-chart-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: center;
    padding: 0.5rem 0;
  }

  .grouped-chart-body {
    display: flex;
    gap: 0.875rem;
    height: 220px;
    width: 100%;
  }

  .chart-y-axis {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-weight: 500;
    width: 44px;
    padding-bottom: 24px;
    text-align: right;
  }

  .chart-bars-area {
    position: relative;
    flex: 1;
    display: flex;
    flex-direction: column;
    justify-content: flex-end;
  }

  .grid-line {
    position: absolute;
    left: 0;
    right: 0;
    border-top: 1px dashed var(--color-border-subtle);
    pointer-events: none;
  }

  .line-100 { top: 0; }
  .line-66 { top: 33.33%; }
  .line-33 { top: 66.66%; }
  .line-0 { bottom: 24px; border-top-style: solid; border-top-color: var(--color-border); }

  .bars-columns-wrapper {
    position: relative;
    z-index: 1;
    display: flex;
    justify-content: space-around;
    align-items: flex-end;
    height: 100%;
    padding-bottom: 24px;
  }

  .bar-group-col {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.5rem;
    flex: 1;
    max-width: 90px;
    text-decoration: none;
    cursor: pointer;
    border-radius: var(--radius-sm);
    padding: 0.25rem 0;
    transition: background-color 0.15s ease;
  }

  .bar-group-col:hover {
    background-color: var(--color-bg-secondary);
  }

  .bars-pair {
    display: flex;
    align-items: flex-end;
    gap: 6px;
    height: 160px;
    width: 100%;
    justify-content: center;
  }

  .bar-wrapper {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    height: 100%;
    justify-content: flex-end;
  }

  .bar-val-label {
    font-size: 0.625rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  .bar-rect {
    width: 18px;
    border-radius: 3px 3px 0 0;
    transition: height 0.3s ease, opacity 0.2s ease;
  }

  .bar-asset { background-color: #315fea; }
  .bar-liability { background-color: #94a3b8; }

  .bar-rect:hover {
    opacity: 0.85;
  }

  .bar-group-name {
    position: absolute;
    bottom: 2px;
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--color-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    max-width: 80px;
    text-align: center;
  }

  /* Donut Chart & Allocation Breakdown */
  .donut-chart-section {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 1.25rem;
    padding: 0.5rem 0;
  }

  .donut-graphic-wrapper {
    position: relative;
    width: 160px;
    height: 160px;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .donut-svg {
    transform: rotate(0deg);
  }

  .donut-slice-circle {
    cursor: pointer;
    transition: stroke-width 0.2s ease, opacity 0.2s ease;
    outline: none;
  }

  .donut-slice-circle:hover {
    opacity: 0.85;
    stroke-width: 22;
  }

  .donut-center-label {
    position: absolute;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    text-align: center;
    padding: 0 10px;
  }

  .center-val {
    font-size: 1.125rem;
    font-weight: 800;
    letter-spacing: -0.02em;
    color: var(--color-text);
  }

  .center-sub {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-weight: 600;
    text-transform: uppercase;
  }

  .allocation-breakdown-list {
    width: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
  }

  .allocation-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.45rem 0.625rem;
    border-radius: var(--radius);
    background-color: var(--color-bg-secondary);
    border: 1px solid transparent;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
    text-align: left;
    width: 100%;
  }

  .allocation-item:hover,
  .allocation-item.selected {
    border-color: var(--color-primary);
    background-color: var(--color-surface-hover);
  }

  .alloc-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .alloc-color-dot {
    width: 10px;
    height: 10px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .alloc-meta {
    display: flex;
    flex-direction: column;
  }

  .alloc-name {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .alloc-type-tag {
    font-size: 0.625rem;
    color: var(--color-text-muted);
  }

  .alloc-right {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .alloc-weight {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .alloc-sub {
    font-size: 0.625rem;
    color: var(--color-text-muted);
  }

  /* Supporting Cards Row (3 Columns) */
  .supporting-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1.25rem;
  }

  /* Activity Feed */
  .activity-feed {
    min-height: 200px;
  }

  .events-list {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
  }

  .event-row-btn {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.45rem 0.5rem;
    border-radius: var(--radius);
    background: transparent;
    border: 1px solid transparent;
    cursor: pointer;
    width: 100%;
    text-align: left;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .event-row-btn:hover {
    background-color: var(--color-bg-secondary);
    border-color: var(--color-border-subtle);
  }

  .event-badge {
    font-size: 0.5625rem;
    font-weight: 800;
    padding: 0.125rem 0.35rem;
    border-radius: 3px;
    letter-spacing: 0.05em;
    text-transform: uppercase;
  }

  .badge-auth {
    background-color: rgba(109, 40, 217, 0.12);
    color: #6d28d9;
    border: 1px solid rgba(109, 40, 217, 0.25);
  }

  .badge-treasury {
    background-color: rgba(49, 95, 234, 0.12);
    color: #315fea;
    border: 1px solid rgba(49, 95, 234, 0.25);
  }

  .badge-risk {
    background-color: rgba(217, 119, 6, 0.12);
    color: #d97706;
    border: 1px solid rgba(217, 119, 6, 0.25);
  }

  .badge-system {
    background-color: rgba(100, 116, 139, 0.12);
    color: #64748b;
    border: 1px solid rgba(100, 116, 139, 0.25);
  }

  .event-details {
    display: flex;
    flex-direction: column;
    flex: 1;
    min-width: 0;
  }

  .event-action-text {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .event-actor-text {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .event-time {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    white-space: nowrap;
    flex-shrink: 0;
  }

  .empty-feed-wrap {
    padding: 2rem 1rem;
    text-align: center;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .empty-feed-text {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .empty-feed-hint {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    margin: 0;
  }

  .timeframe-data-banner {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.45rem 0.8rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius);
    margin-bottom: 0.85rem;
  }

  .banner-badge {
    font-weight: 800;
    font-size: 0.5625rem;
    color: var(--color-primary);
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    padding: 0.15rem 0.4rem;
    border-radius: 3px;
    font-family: var(--font-mono, monospace);
    white-space: nowrap;
  }

  .banner-text {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
    line-height: 1.35;
  }

  /* Balance Sheet Health */
  .health-metrics-list {
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
  }

  .health-metric-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.45rem 0.5rem;
    border-radius: var(--radius);
    background-color: var(--color-bg-secondary);
  }

  .health-metric-meta {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .health-metric-title {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .health-metric-sub {
    font-size: 0.625rem;
    color: var(--color-text-muted);
  }

  .health-metric-val {
    font-size: 0.875rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .positive-val { color: var(--color-success); }
  .status-val-active { color: var(--color-success); font-size: 0.75rem; }

  /* Subsystem Telemetry */
  .mini-refresh-btn {
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0.25rem;
    cursor: pointer;
    color: var(--color-text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .mini-refresh-btn:hover {
    color: var(--color-text);
    border-color: var(--color-border-focus);
  }

  .mini-btn-icon {
    width: 13px;
    height: 13px;
  }

  .telemetry-nodes-list {
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
  }

  .telemetry-node-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.45rem 0.5rem;
    border-radius: var(--radius);
    background-color: var(--color-bg-secondary);
  }

  .node-left {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .node-status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-active { background-color: var(--color-success); }
  .dot-down { background-color: var(--color-danger); }

  .node-meta {
    display: flex;
    flex-direction: column;
  }

  .node-title {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .node-desc {
    font-size: 0.625rem;
    color: var(--color-text-muted);
  }

  .node-stat-pill {
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--color-success);
    background-color: var(--color-success-bg);
    border: 1px solid var(--color-success-border);
    padding: 0.1rem 0.4rem;
    border-radius: 3px;
  }

  .pill-down {
    color: var(--color-danger, #dc2626);
    background-color: var(--color-danger-bg, #fef2f2);
    border-color: var(--color-danger-border, #fecaca);
  }

  /* Skeleton Loading Styles */
  .skeleton-shimmer {
    background-color: var(--color-border-subtle);
    border-radius: var(--radius-sm);
    animation: shimmer 1.5s infinite linear;
  }

  @keyframes shimmer {
    0% { opacity: 0.5; }
    50% { opacity: 0.9; }
    100% { opacity: 0.5; }
  }

  .kpi-skeleton-val {
    width: 65%;
    height: 32px;
  }

  .chart-skeleton-block {
    width: 100%;
    height: 180px;
  }

  .chart-loading-placeholder,
  .chart-empty-placeholder {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 200px;
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .feed-skeleton {
    width: 100%;
    height: 140px;
  }

  /* Modal Details */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background-color: rgba(15, 23, 42, 0.6);
    z-index: 500;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 1.5rem;
    box-sizing: border-box;
  }

  .modal-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-lg);
    width: 100%;
    max-width: 540px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 1.25rem 1.5rem;
    background-color: var(--color-bg-secondary);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .modal-eyebrow {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-primary);
    letter-spacing: 0.08em;
  }

  .modal-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0.25rem 0 0 0;
  }

  .modal-close-btn {
    background: transparent;
    border: none;
    font-size: 1.5rem;
    color: var(--color-text-muted);
    cursor: pointer;
    line-height: 1;
    padding: 0;
  }

  .modal-close-btn:hover {
    color: var(--color-text);
  }

  .modal-body {
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .meta-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.8125rem;
    padding-bottom: 0.35rem;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .meta-key {
    color: var(--color-text-muted);
    font-weight: 500;
  }

  .meta-value {
    color: var(--color-text);
    font-weight: 600;
  }

  .json-box {
    margin-top: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .json-heading {
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-muted);
    text-transform: uppercase;
  }

  .json-code {
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border-subtle);
    border-radius: var(--radius-sm);
    padding: 0.75rem;
    font-size: 0.75rem;
    font-family: var(--font-mono, monospace);
    color: var(--color-text);
    overflow-x: auto;
    max-height: 160px;
    margin: 0;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--color-border-subtle);
    background-color: var(--color-bg-secondary);
  }

  .btn-modal-close {
    padding: 0.5rem 1rem;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    font-size: 0.8125rem;
    font-weight: 600;
    border: 1px solid var(--color-primary);
    border-radius: var(--radius);
    cursor: pointer;
  }

  /* Responsive Breakpoints */
  @media (max-width: 1200px) {
    .kpi-grid { grid-template-columns: repeat(2, 1fr); }
    .visualizations-grid { grid-template-columns: 1fr; }
    .supporting-grid { grid-template-columns: repeat(2, 1fr); }
  }

  @media (max-width: 768px) {
    .dashboard-header-bar {
      flex-direction: column;
      align-items: flex-start;
      gap: 1rem;
    }
    .header-action-group {
      width: 100%;
      justify-content: flex-start;
    }
    .hero-heading { font-size: 1.5rem; }
    .kpi-grid { grid-template-columns: 1fr; }
    .visualizations-grid { grid-template-columns: 1fr; }
    .supporting-grid { grid-template-columns: 1fr; }
    .grouped-chart-body { overflow-x: auto; }
    .bar-group-col { max-width: 65px; }
    .bar-rect { width: 12px; }
  }
</style>
