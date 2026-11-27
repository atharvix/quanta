<script lang="ts">
  import { onMount } from 'svelte';
  import { api, ApiError } from '$lib/api';
  import type {
    Treasury,
    TreasuriesListResponse,
    AIRiskAnalyzeResponse,
    Asset,
    Liability,
    StrategiesListResponse
  } from '$lib/types';
  import { formatPercent } from '$lib/format';
  import {
    formatRiskScore,
    formatPosteriorProb,
    calculateStressedFloor,
    extractAllocations
  } from '$lib/risk';

  let treasuries = $state<Treasury[]>([]);
  let selectedTreasuryId = $state<string>('');
  let isLoading = $state(true);
  let isAnalyzing = $state(false);
  let errorMessage = $state<string | null>(null);
  let errorKind = $state<'auth' | 'engine' | 'validation' | 'network' | null>(null);
  let hasNoAssets = $state(false);
  let riskData = $state<AIRiskAnalyzeResponse | null>(null);

  // Macro Stress Evidence Controls
  let inflationHigh = $state(false);
  let rateHike = $state(false);
  let recessionSignal = $state(false);

  const selectedTreasury = $derived(
    treasuries.find((t) => t.id === selectedTreasuryId) || null
  );

  const pRecessionDisplay = $derived(
    riskData?.bayesian_inference?.p_recession !== undefined
      ? formatPosteriorProb(riskData.bayesian_inference.p_recession)
      : 'Unavailable'
  );

  const pLiquidityCrisisDisplay = $derived(
    riskData?.bayesian_inference?.p_liquidity_crisis !== undefined
      ? formatPosteriorProb(riskData.bayesian_inference.p_liquidity_crisis)
      : 'Unavailable'
  );

  const pEquityDownturnDisplay = $derived(
    riskData?.bayesian_inference?.p_equity_downturn !== undefined
      ? formatPosteriorProb(riskData.bayesian_inference.p_equity_downturn)
      : 'Unavailable'
  );

  const stressMultiplierDisplay = $derived(
    riskData?.bayesian_inference?.stress_multiplier !== undefined
      ? `${riskData.bayesian_inference.stress_multiplier.toFixed(2)}x`
      : 'Unavailable'
  );

  async function loadData() {
    isLoading = true;
    errorMessage = null;
    errorKind = null;
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries?limit=100');
      treasuries = Array.isArray(res.data) ? res.data : [];
      if (treasuries.length > 0) {
        selectedTreasuryId = treasuries[0].id;
        await triggerAnalysis();
      }
    } catch (err) {
      treasuries = [];
      errorMessage = err instanceof Error ? err.message : 'Failed to load treasuries';
      if (err instanceof ApiError && err.status === 403) {
        errorKind = 'auth';
        errorMessage = 'Access denied: Insufficient permissions to view corporate treasuries.';
      } else {
        errorKind = 'network';
        errorMessage = err instanceof Error ? err.message : 'Failed to load treasuries';
      }
    } finally {
      isLoading = false;
    }
  }

  async function triggerAnalysis() {
    if (!selectedTreasuryId) return;

    isAnalyzing = true;
    errorMessage = null;
    errorKind = null;
    hasNoAssets = false;

    try {
      // 1. Fetch current asset catalog for the selected treasury
      const assetsRes = await api<{ data: Asset[] }>(`/api/v1/treasuries/${selectedTreasuryId}/assets`);
      const assets = Array.isArray(assetsRes.data) ? assetsRes.data : [];

      if (assets.length === 0) {
        hasNoAssets = true;
        riskData = null;
        isAnalyzing = false;
        return;
      }

      // 2. Fetch current liabilities and latest saved strategy allocations in parallel
      const [liabRes, stratRes] = await Promise.allSettled([
        api<{ data: Liability[] }>(`/api/v1/treasuries/${selectedTreasuryId}/liabilities`),
        api<StrategiesListResponse>(`/api/v1/treasuries/${selectedTreasuryId}/strategies?limit=1`)
      ]);

      const liabilities: Liability[] =
        liabRes.status === 'fulfilled' && Array.isArray(liabRes.value?.data)
          ? liabRes.value.data
          : [];

      let allocations: Record<string, number> | undefined;
      if (stratRes.status === 'fulfilled' && Array.isArray(stratRes.value?.data) && stratRes.value.data[0]?.allocations_json) {
        const extracted = extractAllocations(stratRes.value.data[0].allocations_json, assets);
        const mandatoryReserve = liabilities.reduce(
          (sum, l) => (l.is_mandatory_reserve ? sum + (Number(l.principal) || 0) : sum),
          0
        );
        const allocSum = Object.values(extracted).reduce((s, v) => s + v, 0);
        const totalCap = Number(selectedTreasury?.total_capital) || 0;
        if (allocSum > 0 && allocSum + mandatoryReserve <= totalCap + 1e-4) {
          allocations = extracted;
        }
      }

      // 3. Dispatch to Go backend and Python AI Engine Bayesian inference
      const res = await api<AIRiskAnalyzeResponse>(`/api/v1/treasuries/${selectedTreasuryId}/risk/analyze`, {
        method: 'POST',
        body: {
          assets,
          liabilities,
          ...(allocations ? { allocations } : {}),
          evidence: {
            inflation_high: inflationHigh,
            rate_hike: rateHike,
            recession_signal: recessionSignal
          },
          run_monte_carlo: true
        }
      });

      riskData = res;
    } catch (err) {
      riskData = null;
      errorMessage = err instanceof Error ? err.message : 'AI Decision Engine is offline or unreachable';
      if (err instanceof ApiError) {
        if (err.status === 403) {
          errorKind = 'auth';
          errorMessage = 'Access denied: You do not have permission to analyze or view this treasury.';
        } else if (err.status === 502 || err.status === 503) {
          errorKind = 'engine';
          errorMessage = err.message || 'AI Decision Engine is offline or unreachable.';
        } else if (err.status === 400 || err.status === 422) {
          errorKind = 'validation';
          errorMessage = err.details.length > 0 ? err.details.join(', ') : err.message;
        } else {
          errorKind = 'network';
          errorMessage = err.message || 'Risk analysis request failed.';
        }
      } else {
        errorKind = 'network';
        errorMessage = err instanceof Error ? err.message : 'Unable to reach backend gateway.';
      }
    } finally {
      isAnalyzing = false;
    }
  }

  function handleTreasuryChange(event: Event) {
    const target = event.target as HTMLSelectElement;
    selectedTreasuryId = target.value;
    triggerAnalysis();
  }

  function handleToggleChange() {
    triggerAnalysis();
  }

  function selectAndAnalyze(treasuryId: string) {
    selectedTreasuryId = treasuryId;
    triggerAnalysis();
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

  <!-- Portfolio Selector & Provenance Bar -->
  <div class="portfolio-selection-card">
    <div class="selector-row">
      <div class="selector-group">
        <label for="treasury-select" class="selector-label">Target Corporate Treasury:</label>
        <select
          id="treasury-select"
          value={selectedTreasuryId}
          onchange={handleTreasuryChange}
          class="portfolio-select"
          disabled={isLoading || isAnalyzing}
        >
          {#if treasuries.length === 0}
            <option value="">No Treasuries Available</option>
          {:else}
            {#each treasuries as t}
              <option value={t.id}>{t.name} ({t.base_currency})</option>
            {/each}
          {/if}
        </select>
      </div>

      <div class="status-group">
        {#if isAnalyzing}
          <span class="status-pill status-loading">Calculating Bayesian Posterior...</span>
        {:else if errorMessage}
          <span class="status-pill status-danger">
            {errorKind === 'auth'
              ? 'Permission Denied (403)'
              : errorKind === 'validation'
                ? 'Invalid Request Input'
                : errorKind === 'engine'
                  ? 'AI Engine Offline (Inference Unavailable)'
                  : 'Request Error'}
          </span>
        {:else if hasNoAssets}
          <span class="status-pill status-warning">No Assets Configured</span>
        {:else if riskData}
          <span class="status-pill status-success">Bayesian Posterior Verified</span>
        {/if}

        <button
          type="button"
          class="btn-refresh"
          onclick={triggerAnalysis}
          disabled={isAnalyzing || !selectedTreasuryId}
        >
          {isAnalyzing ? 'Calculating...' : 'Recalculate Inference'}
        </button>
      </div>
    </div>
  </div>

  <!-- Diagnostic Alert when Engine is Offline or Assets Missing -->
  <!-- Diagnostic Alert when Engine is Offline, Access Denied, or Assets Missing -->
  {#if errorMessage}
    <div class="alert-box alert-danger">
      <div class="alert-content">
        <span class="alert-title">
          {errorKind === 'auth'
            ? 'Permission Denied on Selected Treasury:'
            : errorKind === 'validation'
              ? 'Risk Analysis Input Validation Error:'
              : errorKind === 'engine'
                ? 'AI Decision Engine Unavailable:'
                : 'Risk Analysis Error:'}
        </span>
        <span class="alert-desc">{errorMessage}</span>
      </div>
    </div>
  {:else if hasNoAssets}
    <div class="alert-box alert-warning">
      <div class="alert-content">
        <span class="alert-title">Balance Sheet Unfunded:</span>
        <span class="alert-desc">The selected treasury has no registered investment assets. Add assets in Balance Sheet management to evaluate multi-factor risk, Bayesian stress conditioning, and Monte Carlo VaR.</span>
      </div>
    </div>
  {:else if riskData?.bayesian_inference?.explanation}
    <div class="alert-box alert-info">
      <div class="alert-content">
        <span class="alert-title">Bayesian Network Inference Rationale:</span>
        <span class="alert-desc">{riskData.bayesian_inference.explanation}</span>
      </div>
    </div>
  {/if}

  <!-- Interactive Macroeconomic Evidence Controls -->
  <div class="macro-controls-card">
    <div class="card-top-row">
      <h2 class="card-heading">Macroeconomic Stress State (Bayesian Conditioning Evidence)</h2>
      <span class="pill-kicker">Variable Node Inputs</span>
    </div>
    <p class="card-subheading">Toggle observed macro signals to evaluate real-time posterior conditional probabilities across the active balance sheet.</p>

    <div class="evidence-toggles-row">
      <label class="evidence-box" class:active={inflationHigh}>
        <input
          type="checkbox"
          bind:checked={inflationHigh}
          onchange={handleToggleChange}
          class="chk"
          disabled={isAnalyzing}
        />
        <div class="evidence-meta">
          <span class="ev-title">High Inflation Regime</span>
          <span class="ev-desc">CPI / PCE exceeding target threshold</span>
        </div>
      </label>

      <label class="evidence-box" class:active={rateHike}>
        <input
          type="checkbox"
          bind:checked={rateHike}
          onchange={handleToggleChange}
          class="chk"
          disabled={isAnalyzing}
        />
        <div class="evidence-meta">
          <span class="ev-title">Central Bank Rate Hike</span>
          <span class="ev-desc">Monetary tightening &amp; yield curve shift</span>
        </div>
      </label>

      <label class="evidence-box" class:active={recessionSignal}>
        <input
          type="checkbox"
          bind:checked={recessionSignal}
          onchange={handleToggleChange}
          class="chk"
          disabled={isAnalyzing}
        />
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
      <span class="p-val {riskData && (riskData.bayesian_inference?.p_recession ?? 0) > 0.5 ? 'p-danger' : 'p-normal'}">
        {pRecessionDisplay}
      </span>
      <div class="p-bar-track">
        <div
          class="p-bar-fill {riskData && (riskData.bayesian_inference?.p_recession ?? 0) > 0.5 ? 'bg-danger' : 'bg-primary'}"
          style="width: {riskData?.bayesian_inference?.p_recession !== undefined ? riskData.bayesian_inference.p_recession * 100 : 0}%;"
        ></div>
      </div>
      <span class="p-caption">Systemic contraction probability</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">P(Liquidity Crisis)</span>
      <span class="p-val {riskData && (riskData.bayesian_inference?.p_liquidity_crisis ?? 0) > 0.5 ? 'p-danger' : 'p-normal'}">
        {pLiquidityCrisisDisplay}
      </span>
      <div class="p-bar-track">
        <div
          class="p-bar-fill {riskData && (riskData.bayesian_inference?.p_liquidity_crisis ?? 0) > 0.5 ? 'bg-danger' : 'bg-primary'}"
          style="width: {riskData?.bayesian_inference?.p_liquidity_crisis !== undefined ? riskData.bayesian_inference.p_liquidity_crisis * 100 : 0}%;"
        ></div>
      </div>
      <span class="p-caption">Commercial paper rollover freeze</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">P(Equity Downturn)</span>
      <span class="p-val {riskData && (riskData.bayesian_inference?.p_equity_downturn ?? 0) > 0.5 ? 'p-danger' : 'p-normal'}">
        {pEquityDownturnDisplay}
      </span>
      <div class="p-bar-track">
        <div
          class="p-bar-fill {riskData && (riskData.bayesian_inference?.p_equity_downturn ?? 0) > 0.5 ? 'bg-danger' : 'bg-primary'}"
          style="width: {riskData?.bayesian_inference?.p_equity_downturn !== undefined ? riskData.bayesian_inference.p_equity_downturn * 100 : 0}%;"
        ></div>
      </div>
      <span class="p-caption">Benchmark equity drawdowns</span>
    </div>

    <div class="posterior-card">
      <span class="p-label">Stress Multiplier</span>
      <span class="p-val font-mono">{stressMultiplierDisplay}</span>
      <div class="p-bar-track">
        <div
          class="p-bar-fill bg-warning"
          style="width: {riskData?.bayesian_inference?.stress_multiplier !== undefined ? Math.min(100, (riskData.bayesian_inference.stress_multiplier / 2.5) * 100) : 0}%;"
        ></div>
      </div>
      <span class="p-caption">Applied to mandatory cash reserve</span>
    </div>
  </div>

  <!-- Multi-Factor Decomposition & Monte Carlo Risk Bounds -->
  <div class="metrics-grid">
    <div class="metric-card">
      <span class="m-label">Composite Risk Score</span>
      <span class="m-val font-mono">
        {riskData?.risk_breakdown?.composite_risk_score !== undefined
          ? formatRiskScore(riskData.risk_breakdown.composite_risk_score)
          : 'Unavailable'}
      </span>
      <span class="m-desc">Multi-factor analytical risk weight</span>
    </div>

    <div class="metric-card">
      <span class="m-label">Assigned Risk Tier</span>
      <span class="m-val font-bold">
        {riskData?.risk_breakdown?.risk_tier || 'Unavailable'}
      </span>
      <span class="m-desc">Categorical governance policy tier</span>
    </div>

    <div class="metric-card">
      <span class="m-label">Concentration Index (HHI)</span>
      <span class="m-val font-mono">
        {riskData?.risk_breakdown?.concentration_index_hhi !== undefined
          ? riskData.risk_breakdown.concentration_index_hhi.toFixed(3)
          : 'Unavailable'}
      </span>
      <span class="m-desc">Herfindahl-Hirschman capital spread</span>
    </div>

    <div class="metric-card">
      <span class="m-label">95% Value at Risk (VaR)</span>
      <span class="m-val font-mono">
        {riskData?.monte_carlo_report?.var_95 !== undefined
          ? formatPercent(riskData.monte_carlo_report.var_95)
          : 'Unavailable'}
      </span>
      <span class="m-desc">1,000 Scenario Monte Carlo simulation</span>
    </div>

    <div class="metric-card">
      <span class="m-label">95% Conditional VaR (CVaR)</span>
      <span class="m-val font-mono">
        {riskData?.monte_carlo_report?.cvar_95 !== undefined
          ? formatPercent(riskData.monte_carlo_report.cvar_95)
          : 'Unavailable'}
      </span>
      <span class="m-desc">Expected tail loss beyond 95th percentile</span>
    </div>
  </div>

  <!-- 6 Macroeconomic Regime Stress Evaluations -->
  {#if riskData?.scenario_summary && riskData.scenario_summary.length > 0}
    <div class="macro-controls-card">
      <div class="card-top-row">
        <h2 class="card-heading">Macroeconomic Regime Stress Scenarios</h2>
        <span class="pill-kicker">{riskData.scenario_summary.length} Market Regimes Evaluated</span>
      </div>
      <p class="card-subheading">Projected portfolio return, risk, liquidity, solvency, and compliance across stressed macroeconomic regimes.</p>

      <div class="scenarios-grid">
        {#each riskData.scenario_summary as sc}
          <div class="scenario-card" class:scenario-breach={!sc.is_solvent || !sc.is_liquid}>
            <div class="scenario-top">
              <span class="scenario-name">{sc.regime.replaceAll('_', ' ').toUpperCase()}</span>
              <div class="scenario-badges">
                <span class="status-pill {sc.is_solvent ? 'status-success' : 'status-danger'}">
                  {sc.is_solvent ? 'Solvent' : 'Insolvent'}
                </span>
                <span class="status-pill {sc.is_liquid ? 'status-success' : 'status-warning'}">
                  {sc.is_liquid ? 'Liquid' : 'Illiquid'}
                </span>
              </div>
            </div>
            <div class="scenario-stats">
              <div>
                <span class="m-label">Proj. Return</span>
                <div class="font-mono font-bold">{formatPercent(sc.projected_return)}</div>
              </div>
              <div>
                <span class="m-label">Stressed Risk</span>
                <div class="font-mono font-bold">{formatRiskScore(sc.stressed_risk)}</div>
              </div>
              <div>
                <span class="m-label">Stressed Liq.</span>
                <div class="font-mono font-bold">{formatPercent(sc.stressed_liquidity)}</div>
              </div>
            </div>
            <p class="alert-desc">{sc.explanation}</p>
          </div>
        {/each}
      </div>
    </div>
  {/if}

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
          {#each treasuries.slice(0, 10) as t}
            {@const isCurrent = t.id === selectedTreasuryId}
            {@const baseLiq = Number(t.min_liquidity_ratio) || 0.20}
            <tr class:row-selected={isCurrent}>
              <td class="font-bold">
                {t.name}
                {#if isCurrent}
                  <span class="badge-active">Selected</span>
                {/if}
              </td>
              <td><span class="currency-tag">{t.base_currency}</span></td>
              <td class="text-center font-mono">{Number(t.risk_tolerance).toFixed(2)}</td>
              <td class="text-center font-mono">{formatPercent(baseLiq)}</td>
              <td class="text-center font-mono">
                {#if isCurrent}
                  {#if isAnalyzing}
                    <span class="text-muted">Calculating...</span>
                  {:else if riskData?.bayesian_inference?.stress_multiplier !== undefined}
                    {@const stressedLiq = calculateStressedFloor(baseLiq, riskData.bayesian_inference.stress_multiplier)}
                    <span class="{stressedLiq > 0.4 ? 'text-warning font-bold' : ''}">
                      {formatPercent(stressedLiq)}
                    </span>
                  {:else}
                    <span class="text-muted">Unavailable</span>
                  {/if}
                {:else}
                  <span class="text-muted">Select to Analyze</span>
                {/if}
              </td>
              <td class="text-right">
                {#if !isCurrent}
                  <button
                    type="button"
                    class="btn-table-action"
                    onclick={() => selectAndAnalyze(t.id)}
                  >
                    Analyze
                  </button>
                {/if}
                <a href="/treasuries/{t.id}" class="action-link">Detail &rarr;</a>
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

  .portfolio-selection-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.25rem 1.5rem;
    box-shadow: var(--shadow-sm);
  }

  .selector-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1.5rem;
    flex-wrap: wrap;
  }

  .selector-group {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .selector-label {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .portfolio-select {
    padding: 0.5rem 0.875rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.875rem;
    font-weight: 600;
    min-width: 280px;
    cursor: pointer;
  }

  .portfolio-select:focus {
    outline: none;
    border-color: var(--color-primary);
  }

  .status-group {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .status-pill {
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.35rem 0.65rem;
    border-radius: 4px;
    letter-spacing: 0.02em;
  }

  .status-loading {
    background-color: var(--color-bg-secondary);
    color: var(--color-text-secondary);
    border: 1px solid var(--color-border);
  }

  .status-success {
    background-color: rgba(16, 185, 129, 0.12);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }

  .status-warning {
    background-color: rgba(245, 158, 11, 0.12);
    color: #f59e0b;
    border: 1px solid rgba(245, 158, 11, 0.3);
  }

  .status-danger {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .btn-refresh {
    padding: 0.5rem 0.875rem;
    background-color: var(--color-primary);
    color: var(--color-bg);
    border: none;
    border-radius: var(--radius);
    font-size: 0.8125rem;
    font-weight: 700;
    cursor: pointer;
    transition: opacity 0.15s ease;
  }

  .btn-refresh:hover:not(:disabled) {
    opacity: 0.9;
  }

  .btn-refresh:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .alert-box {
    border-radius: var(--radius);
    padding: 0.875rem 1.25rem;
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
  }

  .alert-danger {
    background-color: rgba(239, 68, 68, 0.08);
    border: 1px solid rgba(239, 68, 68, 0.3);
    color: var(--color-text);
  }

  .alert-warning {
    background-color: rgba(245, 158, 11, 0.08);
    border: 1px solid rgba(245, 158, 11, 0.3);
    color: var(--color-text);
  }

  .alert-info {
    background-color: rgba(59, 130, 246, 0.08);
    border: 1px solid rgba(59, 130, 246, 0.3);
    color: var(--color-text);
  }

  .alert-content {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .alert-title {
    font-size: 0.8125rem;
    font-weight: 700;
  }

  .alert-desc {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    line-height: 1.4;
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

  .metrics-grid {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 1rem;
  }

  .metric-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .m-label {
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .m-val {
    font-size: 1.25rem;
    font-weight: 800;
    color: var(--color-text);
  }

  .m-desc {
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

  .row-selected {
    background-color: rgba(212, 160, 23, 0.04);
  }

  .badge-active {
    font-size: 0.625rem;
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    background-color: var(--color-primary);
    color: var(--color-bg);
    border-radius: 3px;
    margin-left: 0.5rem;
    vertical-align: middle;
  }

  .currency-tag {
    font-size: 0.6875rem;
    font-family: var(--font-mono, monospace);
    font-weight: 600;
    padding: 0.1rem 0.35rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
  }

  .btn-table-action {
    background-color: transparent;
    border: 1px solid var(--color-border);
    color: var(--color-primary);
    padding: 0.25rem 0.5rem;
    border-radius: 3px;
    font-size: 0.6875rem;
    font-weight: 600;
    cursor: pointer;
    margin-right: 0.5rem;
    transition: background-color 0.15s ease;
  }

  .btn-table-action:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-primary);
  }

  .action-link {
    color: var(--color-primary);
    font-weight: 600;
    text-decoration: none;
    font-size: 0.75rem;
  }

  .action-link:hover {
    text-decoration: underline;
  }

  .text-warning { color: var(--color-warning); }

  .scenarios-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 1rem;
  }

  .scenario-card {
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .scenario-card.scenario-breach {
    border-color: rgba(239, 68, 68, 0.45);
  }

  .scenario-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .scenario-name {
    font-size: 0.8125rem;
    font-weight: 800;
    color: var(--color-text);
  }

  .scenario-badges {
    display: flex;
    gap: 0.35rem;
  }

  .scenario-stats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.5rem;
  }

  @media (max-width: 1024px) {
    .scenarios-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .scenarios-grid {
      grid-template-columns: 1fr;
    }
  }
  .text-muted { color: var(--color-text-muted); }
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
    .metrics-grid {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .posterior-grid {
      grid-template-columns: 1fr;
    }
    .metrics-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
