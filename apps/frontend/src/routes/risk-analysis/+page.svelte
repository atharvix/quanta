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
        const preferred = treasuries.find((t) => t.name.toLowerCase().includes('meridian')) || treasuries[0];
        selectedTreasuryId = preferred.id;
        await triggerAnalysis();
      }
    } catch (err) {
      treasuries = [];
      if (err instanceof ApiError && err.status === 403) {
        errorKind = 'auth';
        errorMessage = 'Access denied: insufficient permissions to view corporate treasuries.';
      } else {
        errorKind = 'network';
        errorMessage = err instanceof Error ? err.message : 'Failed to load treasuries.';
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
      const assetsRes = await api<{ data: Asset[] }>(`/api/v1/treasuries/${selectedTreasuryId}/assets`);
      const assets = Array.isArray(assetsRes.data) ? assetsRes.data : [];

      if (assets.length === 0) {
        hasNoAssets = true;
        riskData = null;
        isAnalyzing = false;
        return;
      }

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
      if (err instanceof ApiError) {
        if (err.status === 403) {
          errorKind = 'auth';
          errorMessage = 'Access denied: you do not have permission to analyze this treasury.';
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
    selectedTreasuryId = (event.target as HTMLSelectElement).value;
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
  <title>Risk Analysis — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Risk Analysis &amp; Stress Testing</h1>
      <p>Bayesian conditional probabilities, Monte Carlo VaR, and macro shock compliance.</p>
    </div>

    <div class="page-head-actions">
      <select
        id="treasury-select"
        aria-label="Target corporate treasury"
        value={selectedTreasuryId}
        onchange={handleTreasuryChange}
        class="form-select header-select"
        disabled={isLoading || isAnalyzing}
      >
        {#if treasuries.length === 0}
          <option value="">No treasuries available</option>
        {:else}
          {#each treasuries as t}
            <option value={t.id}>{t.name} ({t.base_currency})</option>
          {/each}
        {/if}
      </select>

      <button
        type="button"
        class="btn btn-primary btn-sm"
        onclick={triggerAnalysis}
        disabled={isAnalyzing || !selectedTreasuryId}
      >
        {isAnalyzing ? 'Calculating...' : 'Recalculate'}
      </button>
    </div>
  </header>

  {#if errorMessage}
    <div class="alert alert-danger" role="alert">
      <strong>
        {errorKind === 'auth'
          ? 'Permission denied: '
          : errorKind === 'validation'
            ? 'Invalid input: '
            : errorKind === 'engine'
              ? 'AI engine offline: '
              : 'Request failed: '}
      </strong>
      {errorMessage}
    </div>
  {:else if !isLoading && treasuries.length === 0}
    <div class="alert alert-warning">
      No corporate treasuries found. <a href="/treasuries">Create a treasury &rarr;</a>
    </div>
  {:else if hasNoAssets}
    <div class="alert alert-warning">
      Selected treasury has no persisted assets.
      <a href="/treasuries/{selectedTreasuryId}">Add or save starter assets &rarr;</a>
    </div>
  {:else if riskData?.bayesian_inference?.explanation}
    <div class="notice-banner">
      <strong>Bayesian rationale:</strong> {riskData.bayesian_inference.explanation}
    </div>
  {/if}

  <!-- Conditioning Evidence Controls Bar -->
  <section class="evidence-strip" aria-label="Macroeconomic conditioning evidence">
    <span class="strip-label">Conditioning evidence</span>
    <div class="evidence-checks">
      <label class="ev-check" class:checked={inflationHigh}>
        <input type="checkbox" bind:checked={inflationHigh} onchange={triggerAnalysis} disabled={isAnalyzing} />
        <span>High inflation regime</span>
      </label>
      <label class="ev-check" class:checked={rateHike}>
        <input type="checkbox" bind:checked={rateHike} onchange={triggerAnalysis} disabled={isAnalyzing} />
        <span>Central bank rate hike</span>
      </label>
      <label class="ev-check" class:checked={recessionSignal}>
        <input type="checkbox" bind:checked={recessionSignal} onchange={triggerAnalysis} disabled={isAnalyzing} />
        <span>Recession signal</span>
      </label>
    </div>
  </section>

  <!-- Posterior Probabilities Hairline Strip -->
  <section class="summary-strip" aria-label="Bayesian posterior probabilities">
    <div class="summary-cell">
      <span class="summary-label">P(Recession)</span>
      <span class="summary-value {riskData && (riskData.bayesian_inference?.p_recession ?? 0) > 0.5 ? 'neg-val' : ''}">
        {pRecessionDisplay}
      </span>
      <div class="meter-track">
        <div
          class="meter-fill {riskData && (riskData.bayesian_inference?.p_recession ?? 0) > 0.5 ? 'fill-danger' : 'fill-primary'}"
          style="width: {riskData?.bayesian_inference?.p_recession !== undefined ? riskData.bayesian_inference.p_recession * 100 : 0}%;"
        ></div>
      </div>
      <span class="summary-meta">Systemic contraction</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">P(Liquidity Crisis)</span>
      <span class="summary-value {riskData && (riskData.bayesian_inference?.p_liquidity_crisis ?? 0) > 0.5 ? 'neg-val' : ''}">
        {pLiquidityCrisisDisplay}
      </span>
      <div class="meter-track">
        <div
          class="meter-fill {riskData && (riskData.bayesian_inference?.p_liquidity_crisis ?? 0) > 0.5 ? 'fill-danger' : 'fill-primary'}"
          style="width: {riskData?.bayesian_inference?.p_liquidity_crisis !== undefined ? riskData.bayesian_inference.p_liquidity_crisis * 100 : 0}%;"
        ></div>
      </div>
      <span class="summary-meta">Rollover freeze probability</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">P(Equity Downturn)</span>
      <span class="summary-value {riskData && (riskData.bayesian_inference?.p_equity_downturn ?? 0) > 0.5 ? 'neg-val' : ''}">
        {pEquityDownturnDisplay}
      </span>
      <div class="meter-track">
        <div
          class="meter-fill {riskData && (riskData.bayesian_inference?.p_equity_downturn ?? 0) > 0.5 ? 'fill-danger' : 'fill-primary'}"
          style="width: {riskData?.bayesian_inference?.p_equity_downturn !== undefined ? riskData.bayesian_inference.p_equity_downturn * 100 : 0}%;"
        ></div>
      </div>
      <span class="summary-meta">Benchmark drawdown</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">Stress Multiplier</span>
      <span class="summary-value">{stressMultiplierDisplay}</span>
      <div class="meter-track">
        <div
          class="meter-fill fill-warn"
          style="width: {riskData?.bayesian_inference?.stress_multiplier !== undefined ? Math.min(100, (riskData.bayesian_inference.stress_multiplier / 2.5) * 100) : 0}%;"
        ></div>
      </div>
      <span class="summary-meta">Applied to liquidity floor</span>
    </div>
  </section>

  <!-- Multi-Factor & VaR Hairline Strip -->
  <section class="summary-strip cols-5" aria-label="Risk decomposition and VaR metrics">
    <div class="summary-cell">
      <span class="summary-label">Composite Risk</span>
      <span class="summary-value">
        {riskData?.risk_breakdown?.composite_risk_score !== undefined
          ? formatRiskScore(riskData.risk_breakdown.composite_risk_score)
          : '—'}
      </span>
      <span class="summary-meta">Analytical composite score</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">Risk Tier</span>
      <span class="summary-value" style="font-family: var(--font-sans);">
        {riskData?.risk_breakdown?.risk_tier || '—'}
      </span>
      <span class="summary-meta">Governance classification</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">Concentration (HHI)</span>
      <span class="summary-value">
        {riskData?.risk_breakdown?.concentration_index_hhi !== undefined
          ? riskData.risk_breakdown.concentration_index_hhi.toFixed(3)
          : '—'}
      </span>
      <span class="summary-meta">Herfindahl exposure</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">95% Value at Risk</span>
      <span class="summary-value">
        {riskData?.monte_carlo_report?.var_95 !== undefined
          ? formatPercent(riskData.monte_carlo_report.var_95)
          : '—'}
      </span>
      <span class="summary-meta">1,000 Monte Carlo paths</span>
    </div>

    <div class="summary-cell">
      <span class="summary-label">95% Conditional VaR</span>
      <span class="summary-value">
        {riskData?.monte_carlo_report?.cvar_95 !== undefined
          ? formatPercent(riskData.monte_carlo_report.cvar_95)
          : '—'}
      </span>
      <span class="summary-meta">Tail loss expectation</span>
    </div>
  </section>

  <!-- 6 Macro Regime Stress Scenarios -->
  {#if riskData?.scenario_summary && riskData.scenario_summary.length > 0}
    <section class="section-block" aria-label="Macroeconomic regime stress evaluations">
      <div class="block-head">
        <h2>Macroeconomic Regime Stress Scenarios</h2>
        <span class="head-meta font-mono">{riskData.scenario_summary.length} regimes evaluated</span>
      </div>

      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>Regime</th>
              <th class="text-right">Proj. Return</th>
              <th class="text-right">Stressed Risk</th>
              <th class="text-right">Stressed Liquidity</th>
              <th>Solvency</th>
              <th>Liquidity</th>
              <th>Assessment</th>
            </tr>
          </thead>
          <tbody>
            {#each riskData.scenario_summary as sc}
              <tr class:breach-row={!sc.is_solvent || !sc.is_liquid}>
                <td class="font-bold regime-name">{sc.regime.replaceAll('_', ' ')}</td>
                <td class="text-right numeric-cell">{formatPercent(sc.projected_return)}</td>
                <td class="text-right numeric-cell">{formatRiskScore(sc.stressed_risk)}</td>
                <td class="text-right numeric-cell">{formatPercent(sc.stressed_liquidity)}</td>
                <td class={sc.is_solvent ? 'pos-val' : 'neg-val'}>{sc.is_solvent ? 'Solvent' : 'Insolvent'}</td>
                <td class={sc.is_liquid ? 'pos-val' : 'warn-val'}>{sc.is_liquid ? 'Liquid' : 'Illiquid'}</td>
                <td class="muted-cell">{sc.explanation}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}

  <!-- Portfolio Risk Tolerances Table -->
  <section class="section-block" aria-label="Active portfolio risk bounds">
    <div class="block-head">
      <h2>Portfolio Risk Tolerances</h2>
      <span class="head-meta font-mono">{treasuries.length} portfolios</span>
    </div>

    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Portfolio</th>
            <th>Currency</th>
            <th class="text-right">Risk Tolerance</th>
            <th class="text-right">Base Liquidity Floor</th>
            <th class="text-right">Stressed Floor</th>
            <th class="text-right">Action</th>
          </tr>
        </thead>
        <tbody>
          {#each treasuries.slice(0, 10) as t}
            {@const isCurrent = t.id === selectedTreasuryId}
            {@const baseLiq = Number(t.min_liquidity_ratio) || 0.20}
            <tr class:active-row={isCurrent}>
              <td class="font-bold">{t.name}</td>
              <td class="font-mono">{t.base_currency}</td>
              <td class="text-right numeric-cell">{Number(t.risk_tolerance).toFixed(2)}</td>
              <td class="text-right numeric-cell">{formatPercent(baseLiq)}</td>
              <td class="text-right numeric-cell">
                {#if isCurrent}
                  {#if isAnalyzing}
                    <span class="muted-cell">Calculating...</span>
                  {:else if riskData?.bayesian_inference?.stress_multiplier !== undefined}
                    {@const stressedLiq = calculateStressedFloor(baseLiq, riskData.bayesian_inference.stress_multiplier)}
                    <span class={stressedLiq > 0.4 ? 'warn-val' : ''}>{formatPercent(stressedLiq)}</span>
                  {:else}
                    —
                  {/if}
                {:else}
                  <span class="muted-cell">Not active</span>
                {/if}
              </td>
              <td class="text-right actions-cell">
                {#if !isCurrent}
                  <button type="button" class="btn btn-secondary btn-xs" onclick={() => selectAndAnalyze(t.id)}>
                    Analyze
                  </button>
                {/if}
                <a href="/treasuries/{t.id}">Inspect &rarr;</a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </section>
</div>

<style>
  .header-select {
    width: auto;
    min-width: 240px;
    min-height: 34px;
    padding: 0.25rem 0.65rem;
    font-size: 0.875rem;
  }

  .evidence-strip {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 1rem;
    padding: 0.85rem 1.125rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    flex-wrap: wrap;
  }

  .strip-label {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .evidence-checks {
    display: flex;
    align-items: center;
    gap: 1.5rem;
    flex-wrap: wrap;
  }

  .ev-check {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    cursor: pointer;
  }

  .ev-check.checked {
    color: var(--color-text);
    font-weight: 500;
  }

  .ev-check input {
    accent-color: var(--color-primary);
    width: 15px;
    height: 15px;
  }

  .meter-track {
    height: 3px;
    width: 100%;
    background-color: var(--color-bg-secondary);
    border-radius: 2px;
    overflow: hidden;
    margin: 0.2rem 0;
  }

  .meter-fill {
    height: 100%;
    transition: width 240ms var(--ease);
  }

  .fill-primary { background-color: var(--color-primary); }
  .fill-danger { background-color: var(--color-danger); }
  .fill-warn { background-color: var(--color-warning); }

  .section-block {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .block-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }

  .block-head h2 {
    font-size: 1.0625rem;
    font-weight: 500;
  }

  .head-meta {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .regime-name {
    text-transform: capitalize;
  }

  .pos-val { color: var(--color-success); font-weight: 500; }
  .warn-val { color: var(--color-warning); font-weight: 500; }
  .neg-val { color: var(--color-danger); font-weight: 500; }
  .muted-cell { color: var(--color-text-secondary); font-size: 0.8125rem; }

  .breach-row td:first-child {
    border-left: 2px solid var(--color-danger);
  }

  .active-row td:first-child {
    border-left: 2px solid var(--color-primary);
  }

  .actions-cell {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 0.65rem;
  }
</style>
