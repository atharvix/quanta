<script lang="ts">
  import { onMount } from 'svelte';
  import {
    api,
    compareOptimizations,
    runClassicalOptimization,
    runQuantumOptimization
  } from '$lib/api';
  import type {
    Asset,
    Liability,
    Treasury,
    TreasuriesListResponse,
    TreasuryAssetsResponse,
    TreasuryLiabilitiesResponse,
    SystemHealthResponse,
    OptimizationAsset,
    OptimizationLiability,
    OptimizationMethod,
    OptimizationProblemPayload,
    OptimizationResult,
    OptimizationCompareResponse
  } from '$lib/types';

  const SAMPLE_SOURCE_ID = '__sample__';

  const SAMPLE_ASSETS: OptimizationAsset[] = [
    { id: 'gov_bond_10y', name: 'Sovereign 10Y G-Sec', asset_type: 'gov_bond', cost: 15.0, expected_return: 0.072, risk: 0.04, liquidity: 0.94, volatility: 0.045 },
    { id: 'corp_bond_aaa', name: 'AAA Corporate Bond', asset_type: 'corp_bond', cost: 20.0, expected_return: 0.092, risk: 0.11, liquidity: 0.78, volatility: 0.12 },
    { id: 'bluechip_equity', name: 'Large-Cap Bluechip Equity', asset_type: 'equity', cost: 25.0, expected_return: 0.148, risk: 0.34, liquidity: 0.82, volatility: 0.36 },
    { id: 'infra_trust', name: 'Commercial Infrastructure Trust', asset_type: 'real_estate', cost: 20.0, expected_return: 0.115, risk: 0.21, liquidity: 0.35, volatility: 0.23 },
    { id: 'gold_hedge', name: 'Gold / Commodity Hedge', asset_type: 'commodity', cost: 15.0, expected_return: 0.088, risk: 0.22, liquidity: 0.65, volatility: 0.24 },
    { id: 'tbill_91d', name: '91-Day Treasury Bills', asset_type: 'cash', cost: 10.0, expected_return: 0.038, risk: 0.01, liquidity: 0.99, volatility: 0.012 }
  ];

  const SAMPLE_LIABILITIES: OptimizationLiability[] = [
    { id: 'senior_term_loan', name: 'Senior Term Loan Tranche A', principal: 15.0, amount: 15.0, interest_rate: 0.085, maturity: 12, maturity_months: 12, priority: 1, is_mandatory_reserve: true }
  ];

  let treasuries = $state<Treasury[]>([]);
  let health = $state<SystemHealthResponse | null>(null);
  let isProbing = $state(true);
  let healthError = $state<string | null>(null);

  let selectedSourceId = $state<string>(SAMPLE_SOURCE_ID);
  let isLoadingSource = $state(false);
  let sourceWarning = $state<string | null>(null);

  let problemCurrency = $state<string>('INR');
  let problemCapital = $state<number>(100.0);
  let minLiquidity = $state<number>(0.25);
  let maxExposurePct = $state<number>(0.45);
  let loadedAssets = $state<OptimizationAsset[]>([...SAMPLE_ASSETS]);
  let loadedLiabilities = $state<OptimizationLiability[]>([...SAMPLE_LIABILITIES]);

  let qaoaDepthP = $state<number>(2);
  let shots = $state<number>(1024);
  let seed = $state<number>(42);
  let riskAversion = $state<number>(0.5);
  let classicalMethod = $state<OptimizationMethod>('exact');

  let isRunningCompare = $state(false);
  let isRunningSingle = $state(false);
  let runError = $state<string | null>(null);
  let compareResult = $state<OptimizationCompareResponse | null>(null);
  let singleResult = $state<OptimizationResult | null>(null);

  const selectedTreasury = $derived(treasuries.find((t) => t.id === selectedSourceId) || null);
  const effectiveRiskTolerance = $derived(Number(Math.max(0, Math.min(1, 1 - riskAversion)).toFixed(4)));

  const mandatoryDebtReserve = $derived.by(() => {
    let total = 0;
    for (const l of loadedLiabilities) {
      const amt = Number(l.amount ?? l.principal ?? 0);
      const maturity = l.maturity ?? l.maturity_months ?? 12;
      if (l.is_mandatory_reserve || maturity <= 12) total += amt;
    }
    return total;
  });

  const deployableCapitalEstimate = $derived(Math.max(0, problemCapital - mandatoryDebtReserve));
  const isAiUp = $derived(health?.dependencies?.ai_engine === 'up');

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
    return isAiUp ? 'online' : 'offline';
  });

  const bestClassicalResult = $derived.by<OptimizationResult | null>(() => {
    if (!compareResult) return null;
    const key = compareResult.best_classical_method;
    return compareResult.results?.[key] || compareResult.results?.exact || null;
  });

  const qaoaResult = $derived<OptimizationResult | null>(compareResult?.results?.qaoa || null);
  const slsqpResult = $derived<OptimizationResult | null>(compareResult?.results?.slsqp || null);

  const estimatedDiscreteCost = $derived.by(() => {
    const n = Math.max(loadedAssets.length, 4);
    const dep = Math.max(1, deployableCapitalEstimate);
    return Number((dep / n).toFixed(2));
  });

  function buildProblemPayload(): OptimizationProblemPayload {
    const stepSize = estimatedDiscreteCost > 0.05 ? estimatedDiscreteCost : 5.0;
    const safeMinLiq = Math.max(0, Math.min(1, Number(minLiquidity) || 0.2));
    const safeMaxExp = Math.max(0.1, Math.min(1, Number(maxExposurePct) || 0.45));

    return {
      capital: Math.max(1, Number(problemCapital) || 100),
      step_size: stepSize,
      assets: loadedAssets.map((a) => ({
        ...a,
        id: a.id || a.name,
        cost: Number(a.cost ?? 0),
        expected_return: Number(a.expected_return ?? 0),
        risk: Number(a.risk ?? 0),
        liquidity: Number(a.liquidity ?? 0)
      })),
      liabilities: loadedLiabilities.map((l) => {
        const amt = Number(l.amount ?? l.principal ?? 0);
        const mat = Number(l.maturity ?? l.maturity_months ?? 12);
        return { ...l, amount: amt, principal: amt, interest_rate: Number(l.interest_rate ?? 0), maturity: mat, maturity_months: mat };
      }),
      constraints: [
        {
          id: 'c_min_liquidity',
          constraint_type: 'minimum_liquidity',
          description: `Maintain minimum ${(safeMinLiq * 100).toFixed(0)}% portfolio liquidity`,
          parameters: { threshold: safeMinLiq }
        }
      ],
      risk_tolerance: effectiveRiskTolerance,
      minimum_liquidity: safeMinLiq,
      max_exposure_pct: safeMaxExp,
      objective: 'maximize_risk_adjusted_return',
      market_state: 'normal'
    };
  }

  function applySampleProblem() {
    selectedSourceId = SAMPLE_SOURCE_ID;
    sourceWarning = null;
    problemCurrency = 'INR';
    problemCapital = 100.0;
    minLiquidity = 0.25;
    maxExposurePct = 0.45;
    riskAversion = 0.5;
    loadedAssets = SAMPLE_ASSETS.map((a) => ({ ...a }));
    loadedLiabilities = SAMPLE_LIABILITIES.map((l) => ({ ...l }));
    runError = null;
  }

  async function selectProblemSource(sourceId: string) {
    selectedSourceId = sourceId;
    runError = null;
    sourceWarning = null;

    if (sourceId === SAMPLE_SOURCE_ID) {
      applySampleProblem();
      return;
    }

    const treasury = treasuries.find((t) => t.id === sourceId);
    if (!treasury) return;

    isLoadingSource = true;
    try {
      const [assetsRes, liabRes] = await Promise.allSettled([
        api<TreasuryAssetsResponse>(`/api/v1/treasuries/${treasury.id}/assets`),
        api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${treasury.id}/liabilities`)
      ]);

      const fetchedAssets: Asset[] =
        assetsRes.status === 'fulfilled' && Array.isArray(assetsRes.value?.data) ? assetsRes.value.data : [];
      const fetchedLiabilities: Liability[] =
        liabRes.status === 'fulfilled' && Array.isArray(liabRes.value?.data) ? liabRes.value.data : [];

      problemCurrency = treasury.base_currency || 'USD';
      problemCapital = Number(treasury.total_capital) || 100.0;
      minLiquidity = Number(treasury.min_liquidity_ratio ?? 0.2);
      const tol = Number(treasury.risk_tolerance ?? 0.5);
      riskAversion = Number(Math.max(0, Math.min(1, 1 - tol)).toFixed(2));
      maxExposurePct = fetchedAssets.length <= 3 ? 0.6 : 0.45;

      loadedLiabilities = fetchedLiabilities.map((l) => ({
        ...l,
        amount: Number(l.principal ?? 0),
        maturity: Number(l.maturity_months ?? 12)
      }));

      if (fetchedAssets.length === 0) {
        loadedAssets = [];
        sourceWarning = `Treasury "${treasury.name}" has 0 configured assets. Configure assets in Treasury Details or switch to the sample problem.`;
      } else {
        loadedAssets = fetchedAssets.map((a) => ({ ...a }));
        if (fetchedAssets.length > 16) {
          sourceWarning = `Portfolio has ${fetchedAssets.length} assets; QAOA simulation is capped at 20 qubits.`;
        }
      }
    } catch (err: any) {
      sourceWarning = err?.message || 'Failed to load treasury assets and liabilities.';
    } finally {
      isLoadingSource = false;
    }
  }

  async function probeCluster() {
    isProbing = true;
    healthError = null;
    try {
      const [tRes, hRes] = await Promise.allSettled([
        api<TreasuriesListResponse>('/api/v1/treasuries?limit=50'),
        api<SystemHealthResponse>('/api/v1/health')
      ]);

      treasuries = tRes.status === 'fulfilled' && Array.isArray(tRes.value?.data) ? tRes.value.data : [];

      if (hRes.status === 'fulfilled' && hRes.value) {
        health = hRes.value;
      } else {
        healthError = 'Unable to probe microservice health endpoints';
      }
    } catch (err: any) {
      healthError = err?.message || 'Cluster diagnostic failure';
    } finally {
      isProbing = false;
    }
  }

  async function handleRunBenchmark() {
    if (loadedAssets.length === 0) {
      runError = 'At least 1 asset is required to run the optimization benchmark.';
      return;
    }

    isRunningCompare = true;
    runError = null;
    singleResult = null;
    try {
      const problem = buildProblemPayload();
      const clampedP = Math.max(1, Math.min(4, Math.round(Number(qaoaDepthP) || 2)));
      const clampedShots = Math.max(64, Math.round(Number(shots) || 1024));
      const cleanSeed = Math.round(Number(seed) || 42);

      compareResult = await compareOptimizations({
        problem,
        options: { p: clampedP, shots: clampedShots, seed: cleanSeed }
      });
    } catch (err: any) {
      runError = err?.message || 'Failed to execute optimization comparison benchmark.';
    } finally {
      isRunningCompare = false;
    }
  }

  async function handleRunSingleSolver(kind: 'classical' | 'quantum') {
    if (loadedAssets.length === 0) {
      runError = 'At least 1 asset is required to execute solver.';
      return;
    }

    isRunningSingle = true;
    runError = null;
    try {
      const problem = buildProblemPayload();
      const clampedP = Math.max(1, Math.min(4, Math.round(Number(qaoaDepthP) || 2)));
      const clampedShots = Math.max(64, Math.round(Number(shots) || 1024));
      const cleanSeed = Math.round(Number(seed) || 42);

      if (kind === 'classical') {
        singleResult = await runClassicalOptimization({
          problem,
          method: classicalMethod,
          options: { seed: cleanSeed }
        });
      } else {
        singleResult = await runQuantumOptimization({
          problem,
          method: 'qaoa',
          options: { p: clampedP, shots: clampedShots, seed: cleanSeed }
        });
      }
    } catch (err: any) {
      runError = err?.message || `Failed to run ${kind} optimization solver.`;
    } finally {
      isRunningSingle = false;
    }
  }

  function formatCapital(val: number, curr = problemCurrency): string {
    if (val === null || val === undefined || isNaN(val)) return '—';
    if (curr === 'INR' && val < 10000) return `₹${val.toFixed(2)} Cr`;
    if (Math.abs(val) >= 1_000_000) return `${curr === 'INR' ? '₹' : '$'}${(val / 1_000_000).toFixed(2)}M`;
    return `${curr === 'INR' ? '₹' : '$'}${val.toLocaleString(undefined, { maximumFractionDigits: 2 })}`;
  }

  function formatAssetCost(a: OptimizationAsset): number {
    if (a.cost > 0.05) return Number(a.cost);
    if (a.min_investment && a.min_investment > 0.05) return Number(a.min_investment);
    return estimatedDiscreteCost > 0.05 ? estimatedDiscreteCost : 5.0;
  }

  function formatSolverName(name: string): string {
    const map: Record<string, string> = {
      exact: 'Exact Enumeration',
      greedy: 'Greedy Heuristic',
      simulated_annealing: 'Simulated Annealing',
      sa: 'Simulated Annealing',
      slsqp: 'SciPy SLSQP (Continuous)',
      milp: 'MILP Branch-and-Bound',
      qaoa: 'QAOA Simulator'
    };
    return map[name] || name;
  }

  onMount(async () => {
    await probeCluster();
    if (treasuries.length > 0) {
      await selectProblemSource(treasuries[0].id);
      if (loadedAssets.length === 0) applySampleProblem();
    }
  });
</script>

<svelte:head>
  <title>Optimization Lab — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Classical vs. Quantum Optimization</h1>
      <p>Side-by-side portfolio benchmarking across Exact, Greedy, Simulated Annealing, SLSQP, and QAOA statevector simulation.</p>
    </div>

    <div class="page-head-actions">
      <div class="engine-bar">
        <span class="status-inline">
          <span class="status-dot {aiEngineStatus === 'online' ? 'ok' : 'err'}"></span>
          AI :8000
        </span>
        <span class="status-inline">
          <span class="status-dot {optEngineStatus === 'online' ? 'ok' : 'warn'}"></span>
          QAOA :8001
        </span>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" onclick={probeCluster} disabled={isProbing}>
        {isProbing ? 'Checking...' : 'Check engines'}
      </button>
    </div>
  </header>

  <!-- Configurator Split -->
  <section class="config-split" aria-label="Optimization problem setup and QAOA parameters">
    <div class="surface-panel stack-gap">
      <div class="panel-top">
        <div>
          <h2>Problem input</h2>
          <p class="panel-sub">Select a persisted PostgreSQL treasury or use the 6-asset sample problem.</p>
        </div>
        <button type="button" class="link-btn" onclick={applySampleProblem}>Reset sample</button>
      </div>

      <div class="form-group">
        <label for="problem-source" class="form-label">Problem source</label>
        <select
          id="problem-source"
          class="form-select"
          value={selectedSourceId}
          onchange={(e) => selectProblemSource((e.target as HTMLSelectElement).value)}
          disabled={isLoadingSource}
        >
          <option value={SAMPLE_SOURCE_ID}>Sample Diversified Problem (₹100 Cr, 6 Assets)</option>
          {#each treasuries as t}
            <option value={t.id}>
              {t.name} ({t.base_currency} &middot; {formatCapital(Number(t.total_capital), t.base_currency || 'USD')})
            </option>
          {/each}
        </select>
      </div>

      {#if sourceWarning}
        <div class="alert alert-warning row-between">
          <span>{sourceWarning}</span>
          {#if loadedAssets.length === 0}
            <button type="button" class="btn btn-secondary btn-xs" onclick={applySampleProblem}>Load sample</button>
          {/if}
        </div>
      {/if}

      <div class="meta-strip">
        <div>
          <span class="detail-label">Capital base</span>
          <span class="detail-value">{formatCapital(problemCapital)}</span>
        </div>
        <div>
          <span class="detail-label">Debt reserve</span>
          <span class="detail-value">{formatCapital(mandatoryDebtReserve)}</span>
        </div>
        <div>
          <span class="detail-label">Deployable</span>
          <span class="detail-value">{formatCapital(deployableCapitalEstimate)}</span>
        </div>
      </div>

      <div class="two-col">
        <div class="form-group" style="margin-bottom: 0;">
          <label for="min-liq-input" class="form-label">Min liquidity floor</label>
          <input id="min-liq-input" type="number" class="form-input font-mono" min="0" max="1" step="0.05" bind:value={minLiquidity} />
        </div>
        <div class="form-group" style="margin-bottom: 0;">
          <label for="max-exp-input" class="form-label">Max single-asset cap</label>
          <input id="max-exp-input" type="number" class="form-input font-mono" min="0.1" max="1" step="0.05" bind:value={maxExposurePct} />
        </div>
      </div>
    </div>

    <div class="surface-panel stack-gap">
      <div class="panel-top">
        <div>
          <h2>Solver &amp; QAOA controls</h2>
          <p class="panel-sub">Set circuit depth, shots, seed, and risk aversion parameter &lambda;.</p>
        </div>
        <span class="head-meta font-mono">{loadedAssets.length} qubits</span>
      </div>

      <div>
        <span class="form-label">QAOA depth (p layers)</span>
        <div class="seg-control" role="group" aria-label="QAOA circuit depth">
          {#each [1, 2, 3, 4] as layer}
            <button type="button" class="seg-btn" class:active={qaoaDepthP === layer} onclick={() => (qaoaDepthP = layer)}>
              p = {layer}
            </button>
          {/each}
        </div>
      </div>

      <div class="two-col">
        <div class="form-group" style="margin-bottom: 0;">
          <label for="qaoa-shots" class="form-label">Measurement shots</label>
          <select id="qaoa-shots" class="form-select font-mono" bind:value={shots}>
            <option value={512}>512</option>
            <option value={1024}>1,024</option>
            <option value={2048}>2,048</option>
            <option value={4096}>4,096</option>
          </select>
        </div>
        <div class="form-group" style="margin-bottom: 0;">
          <label for="solver-seed" class="form-label">PRNG seed</label>
          <input id="solver-seed" type="number" class="form-input font-mono" min="0" max="999999" bind:value={seed} />
        </div>
      </div>

      <div>
        <div class="row-between" style="margin-bottom: 0.35rem;">
          <label for="risk-aversion-slider" class="form-label" style="margin-bottom: 0;">Risk aversion (&lambda;)</label>
          <span class="head-meta font-mono">&lambda; = {riskAversion.toFixed(2)} &middot; Tol {effectiveRiskTolerance.toFixed(2)}</span>
        </div>
        <input id="risk-aversion-slider" type="range" min="0" max="1" step="0.05" class="range-slider" bind:value={riskAversion} />
      </div>

      <div class="action-stack">
        <button
          type="button"
          class="btn btn-primary btn-block"
          onclick={handleRunBenchmark}
          disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
        >
          {isRunningCompare ? 'Running all solvers...' : 'Run Classical vs. QAOA benchmark'}
        </button>

        <div class="solo-row">
          <div class="solo-group">
            <select aria-label="Classical solver method" class="form-select compact-select font-mono" bind:value={classicalMethod}>
              <option value="exact">exact</option>
              <option value="greedy">greedy</option>
              <option value="sa">sa</option>
              <option value="slsqp">slsqp</option>
              <option value="milp">milp</option>
            </select>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={() => handleRunSingleSolver('classical')}
              disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
            >
              Run classical
            </button>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={() => handleRunSingleSolver('quantum')}
            disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
          >
            Run QAOA solo
          </button>
        </div>
      </div>

      {#if runError}
        <div class="alert alert-danger">{runError}</div>
      {/if}
    </div>
  </section>

  <!-- Asset Universe -->
  {#if loadedAssets.length > 0}
    <section class="section-block" aria-label="Asset universe">
      <div class="block-head">
        <h2>Asset Universe ({loadedAssets.length} assets, {loadedLiabilities.length} liabilities)</h2>
        <span class="head-meta font-mono">QUBO dim: {loadedAssets.length} vars</span>
      </div>
      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>Asset</th>
              <th>Class</th>
              <th class="text-right">Ticket Cost</th>
              <th class="text-right">Expected Return</th>
              <th class="text-right">Risk</th>
              <th class="text-right">Liquidity</th>
            </tr>
          </thead>
          <tbody>
            {#each loadedAssets as a}
              <tr>
                <td class="font-bold">{a.name}</td>
                <td class="class-cell">{(a.asset_type || 'other').replaceAll('_', ' ')}</td>
                <td class="text-right numeric-cell">{formatCapital(formatAssetCost(a))}</td>
                <td class="text-right numeric-cell pos-val">+{(Number(a.expected_return) * 100).toFixed(2)}%</td>
                <td class="text-right numeric-cell">{Number(a.risk).toFixed(2)}</td>
                <td class="text-right numeric-cell">{(Number(a.liquidity) * 100).toFixed(0)}%</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}

  <!-- Solo Result -->
  {#if singleResult && !isRunningCompare}
    <section class="section-block" aria-label="Single solver result">
      <div class="block-head">
        <h2>{formatSolverName(singleResult.method)}</h2>
        <span class={singleResult.feasible ? 'pos-val' : 'neg-val'}>
          {singleResult.feasible ? 'Feasible' : 'Infeasible'} ({singleResult.status})
        </span>
      </div>
      <div class="summary-strip cols-5">
        <div class="summary-cell">
          <span class="summary-label">Objective</span>
          <span class="summary-value">{singleResult.objective_value.toFixed(4)}</span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">Expected Return</span>
          <span class="summary-value pos-val">{(singleResult.portfolio_return * 100).toFixed(2)}%</span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">Portfolio Risk</span>
          <span class="summary-value">{singleResult.portfolio_risk.toFixed(4)}</span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">Liquidity</span>
          <span class="summary-value">{(singleResult.liquidity * 100).toFixed(1)}%</span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">Runtime</span>
          <span class="summary-value">{singleResult.runtime_ms.toFixed(2)} ms</span>
        </div>
      </div>
      {#if singleResult.constraint_violations?.length > 0}
        <div class="alert alert-danger">
          <strong>Constraint violations:</strong> {singleResult.constraint_violations.join(' | ')}
        </div>
      {/if}
    </section>
  {/if}

  <!-- Benchmark Comparison Results -->
  {#if compareResult}
    <section class="section-block" aria-label="Classical vs quantum comparison results">
      <div class="notice-banner">
        {compareResult.simulation_note || 'QAOA metrics measure local CPU statevector simulation execution rather than physical QPU hardware.'}
        {#if qaoaResult?.metadata?.qubits !== undefined}
          <span class="font-mono"> ({qaoaResult.metadata.qubits} qubits · p={qaoaResult.metadata.p ?? qaoaDepthP} · {qaoaResult.metadata.shots ?? shots} shots)</span>
        {/if}
      </div>

      <div class="summary-strip cols-3">
        <div class="summary-cell">
          <span class="summary-label">Best Classical Solver</span>
          <span class="summary-value" style="font-family: var(--font-sans);">{formatSolverName(compareResult.best_classical_method)}</span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">QAOA Approximation Ratio</span>
          <span class="summary-value">
            {#if compareResult.qaoa_approximation_ratio != null}
              {compareResult.qaoa_approximation_ratio.toFixed(4)} ({(compareResult.qaoa_approximation_ratio * 100).toFixed(1)}%)
            {:else}
              Unavailable
            {/if}
          </span>
        </div>
        <div class="summary-cell">
          <span class="summary-label">Wall-Clock Speed</span>
          <span class="summary-value" style="font-size: 1.0625rem;">
            {#if bestClassicalResult && qaoaResult}
              {bestClassicalResult.runtime_ms.toFixed(2)} ms vs {qaoaResult.runtime_ms.toFixed(2)} ms
            {:else}
              Completed
            {/if}
          </span>
        </div>
      </div>

      <div class="two-col">
        {#if bestClassicalResult}
          <div class="surface-panel">
            <div class="panel-top">
              <div>
                <h3>{formatSolverName(bestClassicalResult.method)}</h3>
                <p class="panel-sub">Classical baseline</p>
              </div>
              <span class={bestClassicalResult.feasible ? 'pos-val' : 'neg-val'}>
                {bestClassicalResult.feasible ? 'Feasible' : 'Infeasible'}
              </span>
            </div>
            <div class="kv-grid">
              <div><span class="detail-label">Objective</span><span class="detail-value">{bestClassicalResult.objective_value.toFixed(4)}</span></div>
              <div><span class="detail-label">Exp. Return</span><span class="detail-value pos-val">{(bestClassicalResult.portfolio_return * 100).toFixed(2)}%</span></div>
              <div><span class="detail-label">Risk</span><span class="detail-value">{bestClassicalResult.portfolio_risk.toFixed(4)}</span></div>
              <div><span class="detail-label">Liquidity</span><span class="detail-value">{(bestClassicalResult.liquidity * 100).toFixed(1)}%</span></div>
              <div><span class="detail-label">Capital Deployed</span><span class="detail-value">{formatCapital(bestClassicalResult.capital_used)}</span></div>
              <div><span class="detail-label">Runtime</span><span class="detail-value">{bestClassicalResult.runtime_ms.toFixed(2)} ms</span></div>
            </div>
            {#if bestClassicalResult.constraint_violations?.length > 0}
              <p class="viol-line">{bestClassicalResult.constraint_violations.join(' | ')}</p>
            {/if}
          </div>
        {/if}

        {#if qaoaResult}
          <div class="surface-panel quantum-panel">
            <div class="panel-top">
              <div>
                <h3>QAOA Statevector ({qaoaResult.status})</h3>
                <p class="panel-sub">Quantum QUBO simulation</p>
              </div>
              <span class={qaoaResult.feasible ? 'pos-val' : 'neg-val'}>
                {qaoaResult.feasible ? 'Feasible' : 'Infeasible'}
              </span>
            </div>
            <div class="kv-grid">
              <div><span class="detail-label">Objective</span><span class="detail-value">{qaoaResult.objective_value.toFixed(4)}</span></div>
              <div><span class="detail-label">Exp. Return</span><span class="detail-value pos-val">{(qaoaResult.portfolio_return * 100).toFixed(2)}%</span></div>
              <div><span class="detail-label">Risk</span><span class="detail-value">{qaoaResult.portfolio_risk.toFixed(4)}</span></div>
              <div><span class="detail-label">Liquidity</span><span class="detail-value">{(qaoaResult.liquidity * 100).toFixed(1)}%</span></div>
              <div><span class="detail-label">Capital Deployed</span><span class="detail-value">{formatCapital(qaoaResult.capital_used)}</span></div>
              <div><span class="detail-label">Sim Runtime</span><span class="detail-value">{qaoaResult.runtime_ms.toFixed(2)} ms</span></div>
            </div>
            {#if qaoaResult.constraint_violations?.length > 0}
              <p class="viol-line">{qaoaResult.constraint_violations.join(' | ')}</p>
            {/if}
          </div>
        {/if}
      </div>

      <div class="block-head" style="margin-top: 0.75rem;">
        <h2>Solver Matrix</h2>
        <span class="head-meta">All classical methods vs. QAOA</span>
      </div>
      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Status</th>
              <th class="text-right">Objective</th>
              <th class="text-right">Exp. Return</th>
              <th class="text-right">Risk</th>
              <th class="text-right">Liquidity</th>
              <th class="text-right">Capital Used</th>
              <th class="text-center">Feasible</th>
              <th class="text-right">Runtime (ms)</th>
            </tr>
          </thead>
          <tbody>
            {#each compareResult.summary_table as row}
              <tr class:highlight-row={row.method === compareResult.best_classical_method || row.method === 'qaoa'}>
                <td class="font-bold">
                  {formatSolverName(row.method)}
                  {#if row.method === compareResult.best_classical_method}
                    <span class="role-note"> (Best classical)</span>
                  {/if}
                </td>
                <td class="font-mono">{row.status}</td>
                <td class="text-right numeric-cell">{Number(row.objective_value).toFixed(4)}</td>
                <td class="text-right numeric-cell pos-val">{(Number(row.portfolio_return) * 100).toFixed(2)}%</td>
                <td class="text-right numeric-cell">{Number(row.portfolio_risk).toFixed(4)}</td>
                <td class="text-right numeric-cell">{(Number(row.liquidity) * 100).toFixed(1)}%</td>
                <td class="text-right numeric-cell">{formatCapital(Number(row.capital_used))}</td>
                <td class="text-center {row.feasible ? 'pos-val' : 'neg-val'}">{row.feasible ? 'Yes' : 'No'}</td>
                <td class="text-right numeric-cell">{Number(row.runtime_ms).toFixed(2)} ms</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>

      <div class="block-head" style="margin-top: 0.75rem;">
        <h2>Allocated Asset Breakdown</h2>
        <span class="head-meta">Discrete binary decisions vs. continuous SLSQP weights</span>
      </div>
      <div class="table-container">
        <table class="table">
          <thead>
            <tr>
              <th>Asset</th>
              <th>Class</th>
              <th class="text-right">Ticket Cost</th>
              <th class="text-center">Classical ({compareResult.best_classical_method})</th>
              <th class="text-center">QAOA</th>
              {#if slsqpResult}
                <th class="text-right">SLSQP Weight</th>
              {/if}
            </tr>
          </thead>
          <tbody>
            {#each loadedAssets as a}
              {@const cAlloc = bestClassicalResult?.allocation?.[a.name] ?? bestClassicalResult?.allocation?.[a.id] ?? 0}
              {@const qAlloc = qaoaResult?.allocation?.[a.name] ?? qaoaResult?.allocation?.[a.id] ?? 0}
              {@const sAlloc = slsqpResult?.allocation?.[a.name] ?? slsqpResult?.allocation?.[a.id] ?? 0}
              {@const isCContinuous = Boolean(bestClassicalResult?.metadata?.is_continuous)}
              <tr>
                <td class="font-bold">{a.name}</td>
                <td class="class-cell">{(a.asset_type || 'other').replaceAll('_', ' ')}</td>
                <td class="text-right numeric-cell">{formatCapital(formatAssetCost(a))}</td>
                <td class="text-center">
                  {#if isCContinuous}
                    <span class="numeric-cell">{(cAlloc * 100).toFixed(1)}%</span>
                  {:else}
                    <span class={cAlloc >= 0.5 ? 'pos-val' : 'muted-cell'}>{cAlloc >= 0.5 ? 'Selected' : 'Skipped'}</span>
                  {/if}
                </td>
                <td class="text-center">
                  <span class={qAlloc >= 0.5 ? 'pos-val' : 'muted-cell'}>{qAlloc >= 0.5 ? 'Selected' : 'Skipped'}</span>
                </td>
                {#if slsqpResult}
                  <td class="text-right">
                    <div class="weight-cell">
                      <div class="w-track"><div class="w-fill" style="width: {Math.min(100, Math.max(0, sAlloc * 100))}%;"></div></div>
                      <span class="numeric-cell">{(sAlloc * 100).toFixed(1)}%</span>
                    </div>
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {/if}
</div>

<style>
  .engine-bar {
    display: inline-flex;
    align-items: center;
    gap: 1rem;
    padding: 0 0.75rem;
    font-family: var(--font-mono);
  }

  .config-split {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.5rem;
  }

  .stack-gap {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .panel-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .panel-top h2,
  .panel-top h3 {
    font-size: 1rem;
    font-weight: 500;
  }

  .panel-sub {
    margin-top: 0.2rem;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .head-meta {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .link-btn {
    background: none;
    border: none;
    color: var(--color-primary);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
  }

  .link-btn:hover {
    text-decoration: underline;
  }

  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
  }

  .meta-strip {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.75rem;
    padding: 0.75rem 0;
    border-top: 1px solid var(--color-border-subtle);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .meta-strip > div {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .two-col {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }

  .range-slider {
    width: 100%;
    accent-color: var(--color-primary);
  }

  .action-stack {
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
    margin-top: auto;
  }

  .solo-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .solo-group {
    display: flex;
    gap: 0.45rem;
    align-items: center;
  }

  .compact-select {
    width: auto;
    min-height: 34px;
    padding: 0.2rem 0.5rem;
    font-size: 0.8125rem;
  }

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

  .summary-strip.cols-3 {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .quantum-panel {
    border-top: 2px solid var(--color-primary);
  }

  .kv-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem 1rem;
    margin-top: 0.875rem;
  }

  .kv-grid > div {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .viol-line {
    margin-top: 0.75rem;
    font-size: 0.8125rem;
    color: var(--color-danger);
  }

  .highlight-row td {
    background-color: var(--color-bg-secondary);
  }

  .role-note {
    font-weight: 400;
    color: var(--color-primary);
    font-size: 0.8125rem;
  }

  .class-cell {
    color: var(--color-text-secondary);
    text-transform: capitalize;
  }

  .pos-val { color: var(--color-success); font-weight: 500; }
  .neg-val { color: var(--color-danger); font-weight: 500; }
  .muted-cell { color: var(--color-text-muted); }

  .weight-cell {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
  }

  .w-track {
    width: 64px;
    height: 4px;
    background-color: var(--color-bg-secondary);
    border-radius: 2px;
    overflow: hidden;
  }

  .w-fill {
    height: 100%;
    background-color: var(--color-primary);
  }

  @media (max-width: 960px) {
    .config-split,
    .two-col,
    .summary-strip.cols-3 {
      grid-template-columns: 1fr;
    }
  }
</style>
