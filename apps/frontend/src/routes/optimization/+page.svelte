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
    {
      id: 'gov_bond_10y',
      name: 'Sovereign 10Y G-Sec',
      asset_type: 'gov_bond',
      cost: 15.0,
      expected_return: 0.072,
      risk: 0.04,
      liquidity: 0.94,
      volatility: 0.045
    },
    {
      id: 'corp_bond_aaa',
      name: 'AAA Corporate Bond',
      asset_type: 'corp_bond',
      cost: 20.0,
      expected_return: 0.092,
      risk: 0.11,
      liquidity: 0.78,
      volatility: 0.12
    },
    {
      id: 'bluechip_equity',
      name: 'Large-Cap Bluechip Equity',
      asset_type: 'equity',
      cost: 25.0,
      expected_return: 0.148,
      risk: 0.34,
      liquidity: 0.82,
      volatility: 0.36
    },
    {
      id: 'infra_trust',
      name: 'Commercial Infrastructure Trust',
      asset_type: 'real_estate',
      cost: 20.0,
      expected_return: 0.115,
      risk: 0.21,
      liquidity: 0.35,
      volatility: 0.23
    },
    {
      id: 'gold_hedge',
      name: 'Gold / Commodity Hedge',
      asset_type: 'commodity',
      cost: 15.0,
      expected_return: 0.088,
      risk: 0.22,
      liquidity: 0.65,
      volatility: 0.24
    },
    {
      id: 'tbill_91d',
      name: '91-Day Treasury Bills',
      asset_type: 'cash',
      cost: 10.0,
      expected_return: 0.038,
      risk: 0.01,
      liquidity: 0.99,
      volatility: 0.012
    }
  ];

  const SAMPLE_LIABILITIES: OptimizationLiability[] = [
    {
      id: 'senior_term_loan',
      name: 'Senior Term Loan Tranche A',
      principal: 15.0,
      amount: 15.0,
      interest_rate: 0.085,
      maturity: 12,
      maturity_months: 12,
      priority: 1,
      is_mandatory_reserve: true
    }
  ];

  // Cluster & problem state
  let treasuries = $state<Treasury[]>([]);
  let health = $state<SystemHealthResponse | null>(null);
  let isProbing = $state(true);
  let healthError = $state<string | null>(null);

  let selectedSourceId = $state<string>(SAMPLE_SOURCE_ID);
  let isLoadingSource = $state(false);
  let sourceWarning = $state<string | null>(null);

  // Editable/loaded problem fields
  let problemCurrency = $state<string>('INR');
  let problemCapital = $state<number>(100.0);
  let minLiquidity = $state<number>(0.25);
  let maxExposurePct = $state<number>(0.45);
  let loadedAssets = $state<OptimizationAsset[]>([...SAMPLE_ASSETS]);
  let loadedLiabilities = $state<OptimizationLiability[]>([...SAMPLE_LIABILITIES]);

  // Solver Hyperparameters
  let qaoaDepthP = $state<number>(2);
  let shots = $state<number>(1024);
  let seed = $state<number>(42);
  let riskAversion = $state<number>(0.5);
  let classicalMethod = $state<OptimizationMethod>('exact');

  // Execution state & results
  let isRunningCompare = $state(false);
  let isRunningSingle = $state(false);
  let runError = $state<string | null>(null);
  let compareResult = $state<OptimizationCompareResponse | null>(null);
  let singleResult = $state<OptimizationResult | null>(null);

  const selectedTreasury = $derived(
    treasuries.find((t) => t.id === selectedSourceId) || null
  );

  const effectiveRiskTolerance = $derived(
    Number(Math.max(0, Math.min(1, 1 - riskAversion)).toFixed(4))
  );

  const mandatoryDebtReserve = $derived.by(() => {
    let total = 0;
    for (const l of loadedLiabilities) {
      const amt = Number(l.amount ?? l.principal ?? 0);
      const maturity = l.maturity ?? l.maturity_months ?? 12;
      if (l.is_mandatory_reserve || maturity <= 12) {
        total += amt;
      }
    }
    return total;
  });

  const deployableCapitalEstimate = $derived(
    Math.max(0, problemCapital - mandatoryDebtReserve)
  );

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

  const qaoaResult = $derived<OptimizationResult | null>(
    compareResult?.results?.qaoa || null
  );

  const slsqpResult = $derived<OptimizationResult | null>(
    compareResult?.results?.slsqp || null
  );

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
        return {
          ...l,
          amount: amt,
          principal: amt,
          interest_rate: Number(l.interest_rate ?? 0),
          maturity: mat,
          maturity_months: mat
        };
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
        assetsRes.status === 'fulfilled' && Array.isArray(assetsRes.value?.data)
          ? assetsRes.value.data
          : [];
      const fetchedLiabilities: Liability[] =
        liabRes.status === 'fulfilled' && Array.isArray(liabRes.value?.data)
          ? liabRes.value.data
          : [];

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
        sourceWarning = `Treasury "${treasury.name}" has 0 configured assets. Configure assets in Treasury Details or switch to the Prefilled Sample Treasury Problem.`;
      } else {
        loadedAssets = fetchedAssets.map((a) => ({ ...a }));
        if (fetchedAssets.length > 16) {
          sourceWarning = `Portfolio has ${fetchedAssets.length} assets; QAOA simulation is capped at 20 qubits. Consider evaluating a smaller asset pool.`;
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

      if (tRes.status === 'fulfilled' && tRes.value?.data) {
        treasuries = Array.isArray(tRes.value.data) ? tRes.value.data : [];
      } else {
        treasuries = [];
      }

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
        options: {
          p: clampedP,
          shots: clampedShots,
          seed: cleanSeed
        }
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
    if (val === null || val === undefined || isNaN(val)) return 'N/A';
    if (curr === 'INR' && val < 10000) {
      return `₹${val.toFixed(2)} Cr`;
    }
    if (Math.abs(val) >= 1_000_000) {
      return `${curr === 'INR' ? '₹' : '$'}${(val / 1_000_000).toFixed(2)}M`;
    }
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
    return map[name] || name.toUpperCase();
  }

  onMount(async () => {
    await probeCluster();
    if (treasuries.length > 0) {
      // Check if the first treasury has assets; otherwise keep prefilled sample ready
      await selectProblemSource(treasuries[0].id);
      if (loadedAssets.length === 0) {
        applySampleProblem();
      }
    }
  });
</script>

<svelte:head>
  <title>Classical vs. Quantum Optimization Lab - QUANTA</title>
</svelte:head>

<div class="page-root">
  <!-- Header -->
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">MATHEMATICAL PROGRAMMING &amp; QAOA BENCHMARK SUITE</span>
      </div>
      <h1 class="page-title">Classical vs. Quantum Optimization Lab</h1>
      <p class="page-subtitle">
        Side-by-side portfolio benchmarking across Exact, Greedy, Simulated Annealing, Continuous SLSQP, and QUBO-encoded QAOA statevector simulation.
      </p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-secondary" onclick={probeCluster} disabled={isProbing}>
        {#if isProbing}Probing...{:else}Probe Cluster Health{/if}
      </button>
    </div>
  </div>

  <!-- Live Cluster Telemetry Banner -->
  <div class="cluster-status-banner">
    <div class="status-col">
      <span class="status-col-title">AI DECISION ENGINE (PORT 8000)</span>
      <div class="status-col-val">
        <span class="status-dot {aiEngineStatus === 'online' ? 'dot-online' : 'dot-offline'}"></span>
        <span class="status-text font-bold">
          {aiEngineStatus === 'online' ? 'Online — Search & Risk Active' : 'Offline'}
        </span>
      </div>
      <span class="status-sub">A*, BFS, Best-First heuristic search &amp; Bayesian risk modeling</span>
    </div>

    <div class="status-divider"></div>

    <div class="status-col">
      <span class="status-col-title">OPTIMIZATION &amp; QUANTUM ENGINE (PORT 8001)</span>
      <div class="status-col-val">
        <span class="status-dot {optEngineStatus === 'online' ? 'dot-online' : 'dot-offline'}"></span>
        <span class="status-text font-bold">
          {#if optEngineStatus === 'online'}
            Online — Classical &amp; QAOA Solvers Ready
          {:else if optEngineStatus === 'offline'}
            Offline (Port 8001 Unreachable)
          {:else if optEngineStatus === 'not_configured'}
            Not Configured
          {:else}
            Health Check Failed
          {/if}
        </span>
      </div>
      <span class="status-sub">
        Exact, Greedy, SA, SciPy SLSQP, MILP &amp; QUBO/QAOA statevector simulator
      </span>
    </div>
  </div>

  <!-- Lab Controls & Treasury Problem Configurator -->
  <div class="lab-config-grid">
    <!-- Left: Problem Source & Treasury Profile -->
    <div class="panel-card">
      <div class="panel-header">
        <div>
          <h2 class="panel-title">Treasury Problem Input</h2>
          <p class="panel-sub">Load an institutional portfolio from PostgreSQL or run the prefilled 6-asset sample problem.</p>
        </div>
        <button type="button" class="btn-link" onclick={applySampleProblem}>
          Reset to Sample
        </button>
      </div>

      <div class="field-group">
        <label for="problem-source" class="field-label">Problem Source</label>
        <select
          id="problem-source"
          class="field-select"
          value={selectedSourceId}
          onchange={(e) => selectProblemSource((e.target as HTMLSelectElement).value)}
          disabled={isLoadingSource}
        >
          <option value={SAMPLE_SOURCE_ID}>
            Prefilled Sample Treasury Problem (₹100 Cr Diversified, 6 Assets)
          </option>
          {#each treasuries as t}
            <option value={t.id}>
              Treasury: {t.name} ({t.base_currency} &middot; {formatCapital(Number(t.total_capital), t.base_currency || 'USD')})
            </option>
          {/each}
        </select>
      </div>

      {#if sourceWarning}
        <div class="inline-alert alert-warn">
          <span>{sourceWarning}</span>
          {#if loadedAssets.length === 0}
            <button type="button" class="btn-inline" onclick={applySampleProblem}>Use Sample Problem</button>
          {/if}
        </div>
      {/if}

      <div class="snapshot-grid">
        <div class="snap-item">
          <span class="snap-label">Total Capital</span>
          <span class="snap-val font-mono">{formatCapital(problemCapital)}</span>
        </div>
        <div class="snap-item">
          <span class="snap-label">Mandatory Debt Reserve</span>
          <span class="snap-val font-mono">{formatCapital(mandatoryDebtReserve)}</span>
        </div>
        <div class="snap-item">
          <span class="snap-label">Deployable Capital</span>
          <span class="snap-val font-mono">{formatCapital(deployableCapitalEstimate)}</span>
        </div>
        <div class="snap-item">
          <span class="snap-label">Min Liquidity Floor</span>
          <span class="snap-val font-mono">{(minLiquidity * 100).toFixed(1)}%</span>
        </div>
      </div>

      <div class="two-col-inputs">
        <div class="field-group">
          <label for="min-liq-input" class="field-label">Min Liquidity Floor</label>
          <input
            id="min-liq-input"
            type="number"
            class="field-input font-mono"
            min="0"
            max="1"
            step="0.05"
            bind:value={minLiquidity}
          />
        </div>
        <div class="field-group">
          <label for="max-exp-input" class="field-label">Max Single-Asset Cap</label>
          <input
            id="max-exp-input"
            type="number"
            class="field-input font-mono"
            min="0.1"
            max="1"
            step="0.05"
            bind:value={maxExposurePct}
          />
        </div>
      </div>
    </div>

    <!-- Right: QAOA & Risk Aversion Controls -->
    <div class="panel-card">
      <div class="panel-header">
        <div>
          <h2 class="panel-title">QAOA Circuit &amp; Solver Controls</h2>
          <p class="panel-sub">Tune quantum ansatz depth, measurement shots, PRNG seed, and risk aversion penalty &lambda;.</p>
        </div>
        <span class="pill-kicker font-mono">{loadedAssets.length} Qubits Needed</span>
      </div>

      <div class="field-group">
        <span class="field-label">QAOA Circuit Depth (p layers)</span>
        <div class="depth-pills" role="group" aria-label="QAOA depth p">
          {#each [1, 2, 3, 4] as layer}
            <button
              type="button"
              class="depth-pill {qaoaDepthP === layer ? 'active' : ''}"
              onclick={() => (qaoaDepthP = layer)}
            >
              p = {layer}
            </button>
          {/each}
        </div>
      </div>

      <div class="two-col-inputs">
        <div class="field-group">
          <label for="qaoa-shots" class="field-label">Measurement Shots</label>
          <select id="qaoa-shots" class="field-select font-mono" bind:value={shots}>
            <option value={512}>512 shots</option>
            <option value={1024}>1024 shots</option>
            <option value={2048}>2048 shots</option>
            <option value={4096}>4096 shots</option>
          </select>
        </div>

        <div class="field-group">
          <label for="solver-seed" class="field-label">Deterministic Seed</label>
          <input
            id="solver-seed"
            type="number"
            class="field-input font-mono"
            min="0"
            max="999999"
            bind:value={seed}
          />
        </div>
      </div>

      <div class="field-group">
        <div class="slider-header">
          <label for="risk-aversion-slider" class="field-label">Risk Aversion (&lambda; = 1 - Risk Tolerance)</label>
          <span class="slider-readout font-mono">&lambda; = {riskAversion.toFixed(2)} &middot; Tol = {effectiveRiskTolerance.toFixed(2)}</span>
        </div>
        <input
          id="risk-aversion-slider"
          type="range"
          min="0"
          max="1"
          step="0.05"
          class="range-slider"
          bind:value={riskAversion}
        />
        <div class="slider-ticks">
          <span>0.00 (Return-Seeking)</span>
          <span>0.50 (Balanced)</span>
          <span>1.00 (Defensive)</span>
        </div>
      </div>

      <!-- Primary & Secondary Execution Actions -->
      <div class="action-bar">
        <button
          type="button"
          class="btn-primary-benchmark"
          onclick={handleRunBenchmark}
          disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
        >
          {#if isRunningCompare}
            Running Classical &amp; QAOA Solvers...
          {:else}
            Run Classical vs. Quantum Benchmark
          {/if}
        </button>

        <div class="secondary-actions">
          <div class="solo-classical-group">
            <select
              aria-label="Classical solver method"
              class="field-select-compact font-mono"
              bind:value={classicalMethod}
            >
              <option value="exact">exact</option>
              <option value="greedy">greedy</option>
              <option value="sa">sa</option>
              <option value="slsqp">slsqp</option>
              <option value="milp">milp</option>
            </select>
            <button
              type="button"
              class="btn-secondary-compact"
              onclick={() => handleRunSingleSolver('classical')}
              disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
            >
              Run Classical
            </button>
          </div>
          <button
            type="button"
            class="btn-secondary-compact"
            onclick={() => handleRunSingleSolver('quantum')}
            disabled={isRunningCompare || isRunningSingle || loadedAssets.length === 0}
          >
            Run QAOA Solo
          </button>
        </div>
      </div>

      {#if runError}
        <div class="inline-alert alert-error">
          <strong>Execution Failed:</strong> {runError}
        </div>
      {/if}
    </div>
  </div>

  <!-- Candidate Asset Universe Preview -->
  {#if loadedAssets.length > 0}
    <div class="section-card">
      <div class="card-header-bar">
        <div>
          <h2 class="card-title">Active Asset Universe ({loadedAssets.length} Assets &middot; {loadedLiabilities.length} Liabilities)</h2>
          <p class="card-sub">
            {#if selectedSourceId === SAMPLE_SOURCE_ID}
              Prefilled diversified corporate treasury universe with realistic ticket sizes.
            {:else if selectedTreasury}
              Loaded live from treasury <strong>{selectedTreasury.name}</strong> (default ticket size auto-scaled to {formatCapital(estimatedDiscreteCost)} when cost &le; 0.05).
            {/if}
          </p>
        </div>
        <span class="pill-kicker font-mono">QUBO Dimension: {loadedAssets.length} vars</span>
      </div>
      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>Asset</th>
              <th>Class</th>
              <th class="text-right">Ticket Cost</th>
              <th class="text-right">Expected Return</th>
              <th class="text-center">Risk</th>
              <th class="text-center">Liquidity</th>
            </tr>
          </thead>
          <tbody>
            {#each loadedAssets as a}
              <tr>
                <td class="font-bold">{a.name}</td>
                <td><span class="tag-pill">{(a.asset_type || 'other').toUpperCase()}</span></td>
                <td class="text-right font-mono">{formatCapital(formatAssetCost(a))}</td>
                <td class="text-right font-mono text-success">+{(Number(a.expected_return) * 100).toFixed(2)}%</td>
                <td class="text-center font-mono">{Number(a.risk).toFixed(2)}</td>
                <td class="text-center font-mono">{(Number(a.liquidity) * 100).toFixed(0)}%</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}

  <!-- Standalone Classical / QAOA Single-Solver Output -->
  {#if singleResult && !isRunningCompare}
    <div class="section-card single-result-card">
      <div class="card-header-bar">
        <div>
          <span class="solver-eyebrow">SINGLE SOLVER EXECUTION</span>
          <h2 class="card-title">{formatSolverName(singleResult.method)}</h2>
        </div>
        <span class="feas-badge {singleResult.feasible ? 'feas-ok' : 'feas-bad'}">
          {singleResult.feasible ? 'FEASIBLE' : 'INFEASIBLE'} ({singleResult.status.toUpperCase()})
        </span>
      </div>
      <div class="metric-row-5">
        <div class="mini-kpi">
          <span class="mk-label">Objective Value</span>
          <span class="mk-val font-mono">{singleResult.objective_value.toFixed(4)}</span>
        </div>
        <div class="mini-kpi">
          <span class="mk-label">Expected Return</span>
          <span class="mk-val font-mono text-success">{(singleResult.portfolio_return * 100).toFixed(2)}%</span>
        </div>
        <div class="mini-kpi">
          <span class="mk-label">Portfolio Risk</span>
          <span class="mk-val font-mono">{singleResult.portfolio_risk.toFixed(4)}</span>
        </div>
        <div class="mini-kpi">
          <span class="mk-label">Liquidity Ratio</span>
          <span class="mk-val font-mono">{(singleResult.liquidity * 100).toFixed(1)}%</span>
        </div>
        <div class="mini-kpi">
          <span class="mk-label">Runtime</span>
          <span class="mk-val font-mono">{singleResult.runtime_ms.toFixed(2)} ms</span>
        </div>
      </div>
      {#if singleResult.constraint_violations?.length > 0}
        <div class="violations-box">
          <strong>Constraint Violations:</strong> {singleResult.constraint_violations.join(' | ')}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Classical vs. Quantum Comparison Results -->
  {#if compareResult}
    <!-- Explicit Scientific Simulation Disclaimer -->
    <div class="simulation-disclaimer" role="note">
      <div class="disclaimer-head">
        <span class="disclaimer-badge">SIMULATION PROVENANCE DISCLOSURE</span>
        {#if qaoaResult?.metadata?.qubits !== undefined}
          <span class="disclaimer-meta font-mono">
            {qaoaResult.metadata.qubits} Qubits &middot; p = {qaoaResult.metadata.p ?? qaoaDepthP} Layers &middot; {qaoaResult.metadata.shots ?? shots} Shots
          </span>
        {/if}
      </div>
      <p class="disclaimer-body">
        {compareResult.simulation_note || 'Scientific note: QAOA runtime represents classical statevector SIMULATION cost on CPU, not physical quantum processing unit (QPU) hardware execution.'}
      </p>
    </div>

    <!-- Approximation Ratio & Winner Summary Bar -->
    <div class="approx-banner">
      <div class="approx-kpi">
        <span class="approx-label">BEST CLASSICAL BASELINE</span>
        <span class="approx-val">{formatSolverName(compareResult.best_classical_method)}</span>
      </div>
      <div class="approx-divider"></div>
      <div class="approx-kpi">
        <span class="approx-label">QAOA APPROXIMATION RATIO (VS. EXACT)</span>
        <span class="approx-val font-mono {compareResult.qaoa_approximation_ratio !== null && compareResult.qaoa_approximation_ratio !== undefined && compareResult.qaoa_approximation_ratio >= 0.9 ? 'text-success' : ''}">
          {#if compareResult.qaoa_approximation_ratio !== null && compareResult.qaoa_approximation_ratio !== undefined}
            {compareResult.qaoa_approximation_ratio.toFixed(4)} ({(compareResult.qaoa_approximation_ratio * 100).toFixed(1)}%)
          {:else}
            N/A (Infeasible Exact or QAOA Baseline)
          {/if}
        </span>
      </div>
      <div class="approx-divider"></div>
      <div class="approx-kpi">
        <span class="approx-label">SPEED COMPARISON</span>
        <span class="approx-val font-mono">
          {#if bestClassicalResult && qaoaResult}
            Classical {bestClassicalResult.runtime_ms.toFixed(2)} ms vs QAOA Sim {qaoaResult.runtime_ms.toFixed(2)} ms
          {:else}
            Completed
          {/if}
        </span>
      </div>
    </div>

    <!-- Side-by-Side Hero Comparison: Best Classical vs. QAOA Quantum -->
    <div class="comparison-pair">
      {#if bestClassicalResult}
        <div class="compare-card">
          <div class="compare-top">
            <div>
              <span class="solver-eyebrow">CLASSICAL BASELINE</span>
              <h3 class="compare-title">{formatSolverName(bestClassicalResult.method)}</h3>
            </div>
            <span class="feas-badge {bestClassicalResult.feasible ? 'feas-ok' : 'feas-bad'}">
              {bestClassicalResult.feasible ? 'Feasible' : 'Infeasible'}
            </span>
          </div>

          <div class="compare-metrics-grid">
            <div class="cm-item">
              <span class="cm-label">Objective Value</span>
              <span class="cm-val font-mono">{bestClassicalResult.objective_value.toFixed(4)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Expected Return</span>
              <span class="cm-val font-mono text-success">{(bestClassicalResult.portfolio_return * 100).toFixed(2)}%</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Portfolio Risk</span>
              <span class="cm-val font-mono">{bestClassicalResult.portfolio_risk.toFixed(4)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Liquidity Ratio</span>
              <span class="cm-val font-mono">{(bestClassicalResult.liquidity * 100).toFixed(1)}%</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Capital Deployed</span>
              <span class="cm-val font-mono">{formatCapital(bestClassicalResult.capital_used)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Wall-Clock Runtime</span>
              <span class="cm-val font-mono">{bestClassicalResult.runtime_ms.toFixed(2)} ms</span>
            </div>
          </div>

          {#if bestClassicalResult.constraint_violations?.length > 0}
            <div class="violations-box">
              <strong>Violations:</strong> {bestClassicalResult.constraint_violations.join(' | ')}
            </div>
          {/if}
        </div>
      {/if}

      {#if qaoaResult}
        <div class="compare-card compare-card-quantum">
          <div class="compare-top">
            <div>
              <span class="solver-eyebrow">QUBO STATEVECTOR SIMULATOR</span>
              <h3 class="compare-title">QAOA Quantum ({qaoaResult.status.toUpperCase()})</h3>
            </div>
            <span class="feas-badge {qaoaResult.feasible ? 'feas-ok' : 'feas-bad'}">
              {qaoaResult.feasible ? 'Feasible' : 'Infeasible'}
            </span>
          </div>

          <div class="compare-metrics-grid">
            <div class="cm-item">
              <span class="cm-label">Objective Value</span>
              <span class="cm-val font-mono">{qaoaResult.objective_value.toFixed(4)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Expected Return</span>
              <span class="cm-val font-mono text-success">{(qaoaResult.portfolio_return * 100).toFixed(2)}%</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Portfolio Risk</span>
              <span class="cm-val font-mono">{qaoaResult.portfolio_risk.toFixed(4)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Liquidity Ratio</span>
              <span class="cm-val font-mono">{(qaoaResult.liquidity * 100).toFixed(1)}%</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Capital Deployed</span>
              <span class="cm-val font-mono">{formatCapital(qaoaResult.capital_used)}</span>
            </div>
            <div class="cm-item">
              <span class="cm-label">Statevector Sim Time</span>
              <span class="cm-val font-mono">{qaoaResult.runtime_ms.toFixed(2)} ms</span>
            </div>
          </div>

          {#if qaoaResult.constraint_violations?.length > 0}
            <div class="violations-box">
              <strong>Violations:</strong> {qaoaResult.constraint_violations.join(' | ')}
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- All Solvers Summary Table -->
    <div class="section-card">
      <div class="card-header-bar">
        <div>
          <h2 class="card-title">Full Solver Benchmark Matrix</h2>
          <p class="card-sub">All classical baselines compared directly against QAOA statevector simulation on identical inputs.</p>
        </div>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>Method</th>
              <th>Status</th>
              <th class="text-right">Objective</th>
              <th class="text-right">Expected Return</th>
              <th class="text-right">Risk</th>
              <th class="text-right">Liquidity</th>
              <th class="text-right">Capital Used</th>
              <th class="text-center">Feasible</th>
              <th class="text-right">Runtime (ms)</th>
            </tr>
          </thead>
          <tbody>
            {#each compareResult.summary_table as row}
              <tr class={row.method === compareResult.best_classical_method || row.method === 'qaoa' ? 'highlight-row' : ''}>
                <td class="font-bold">
                  {formatSolverName(row.method)}
                  {#if row.method === compareResult.best_classical_method}
                    <span class="mini-tag tag-best">BEST CLASSICAL</span>
                  {:else if row.method === 'qaoa'}
                    <span class="mini-tag tag-quantum">QAOA SIM</span>
                  {/if}
                </td>
                <td><span class="tag-pill">{String(row.status).toUpperCase()}</span></td>
                <td class="text-right font-mono font-bold">{Number(row.objective_value).toFixed(4)}</td>
                <td class="text-right font-mono text-success">{(Number(row.portfolio_return) * 100).toFixed(2)}%</td>
                <td class="text-right font-mono">{Number(row.portfolio_risk).toFixed(4)}</td>
                <td class="text-right font-mono">{(Number(row.liquidity) * 100).toFixed(1)}%</td>
                <td class="text-right font-mono">{formatCapital(Number(row.capital_used))}</td>
                <td class="text-center">
                  <span class="feas-badge {row.feasible ? 'feas-ok' : 'feas-bad'}">
                    {row.feasible ? 'Yes' : 'No'}
                  </span>
                </td>
                <td class="text-right font-mono">{Number(row.runtime_ms).toFixed(2)} ms</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>

    <!-- Allocated Asset Breakdown -->
    <div class="section-card">
      <div class="card-header-bar">
        <div>
          <h2 class="card-title">Allocated Asset Breakdown</h2>
          <p class="card-sub">Binary selection (0/1 discrete QUBO / Exact) alongside continuous SLSQP portfolio weights (%).</p>
        </div>
      </div>

      <div class="table-wrap">
        <table class="data-table">
          <thead>
            <tr>
              <th>Asset</th>
              <th>Class</th>
              <th class="text-right">Ticket Cost</th>
              <th class="text-center">Best Classical ({compareResult.best_classical_method})</th>
              <th class="text-center">QAOA Quantum</th>
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
                <td><span class="tag-pill">{(a.asset_type || 'other').toUpperCase()}</span></td>
                <td class="text-right font-mono">{formatCapital(formatAssetCost(a))}</td>
                <td class="text-center">
                  {#if isCContinuous}
                    <span class="font-mono font-bold">{(cAlloc * 100).toFixed(1)}%</span>
                  {:else}
                    <span class="alloc-pill {cAlloc >= 0.5 ? 'alloc-selected' : 'alloc-skipped'}">
                      {cAlloc >= 0.5 ? 'Selected (1)' : 'Skipped (0)'}
                    </span>
                  {/if}
                </td>
                <td class="text-center">
                  <span class="alloc-pill {qAlloc >= 0.5 ? 'alloc-quantum' : 'alloc-skipped'}">
                    {qAlloc >= 0.5 ? 'Selected (1)' : 'Skipped (0)'}
                  </span>
                </td>
                {#if slsqpResult}
                  <td class="text-right">
                    <div class="weight-bar-cell">
                      <div class="weight-track">
                        <div class="weight-fill" style="width: {Math.min(100, Math.max(0, sAlloc * 100))}%;"></div>
                      </div>
                      <span class="font-mono">{(sAlloc * 100).toFixed(1)}%</span>
                    </div>
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {/if}
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
    gap: 1rem;
    padding-bottom: 0.75rem;
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
    margin: 0.25rem 0 0.2rem 0;
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .btn-secondary {
    padding: 0.5rem 0.9rem;
    font-size: 0.75rem;
    font-weight: 600;
    background-color: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-secondary:hover:not(:disabled) {
    background-color: var(--color-bg-secondary);
  }

  /* Telemetry Banner */
  .cluster-status-banner {
    display: flex;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.15rem 1.5rem;
    gap: 1.5rem;
    box-shadow: var(--shadow-sm);
  }

  .status-col {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .status-col-title {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
    font-family: var(--font-mono, monospace);
  }

  .status-col-val {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-online { background-color: var(--color-success, #10b981); }
  .dot-offline { background-color: var(--color-danger, #ef4444); }

  .status-text { font-size: 0.875rem; color: var(--color-text); }
  .status-sub { font-size: 0.75rem; color: var(--color-text-secondary); }
  .status-divider { width: 1px; background-color: var(--color-border); }

  /* Configurator Grid */
  .lab-config-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.25rem;
  }

  .panel-card,
  .section-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.4rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .panel-header,
  .card-header-bar {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
  }

  .panel-title,
  .card-title {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .panel-sub,
  .card-sub {
    font-size: 0.78rem;
    color: var(--color-text-secondary);
    margin: 0.2rem 0 0 0;
  }

  .btn-link {
    background: none;
    border: none;
    color: var(--color-primary);
    font-size: 0.75rem;
    font-weight: 700;
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-link:hover { text-decoration: underline; }

  .field-group {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .field-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text-secondary);
  }

  .field-select,
  .field-input {
    width: 100%;
    padding: 0.55rem 0.75rem;
    font-size: 0.8125rem;
    background-color: var(--color-bg-secondary);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
  }

  .snapshot-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 0.75rem;
    background-color: var(--color-bg-secondary);
    padding: 0.9rem;
    border-radius: var(--radius);
    border: 1px solid var(--color-border-subtle);
  }

  .snap-item {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .snap-label {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }

  .snap-val {
    font-size: 0.9375rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .two-col-inputs {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.85rem;
  }

  .depth-pills {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.5rem;
  }

  .depth-pill {
    padding: 0.45rem 0.6rem;
    font-size: 0.8125rem;
    font-weight: 700;
    font-family: var(--font-mono, monospace);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    background-color: var(--color-bg-secondary);
    color: var(--color-text-secondary);
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .depth-pill.active {
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    border-color: var(--color-primary);
  }

  .slider-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .slider-readout {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-primary);
  }

  .range-slider {
    width: 100%;
    accent-color: var(--color-primary);
    cursor: pointer;
  }

  .slider-ticks {
    display: flex;
    justify-content: space-between;
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .action-bar {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    margin-top: 0.25rem;
  }

  .btn-primary-benchmark {
    width: 100%;
    padding: 0.75rem 1rem;
    font-size: 0.875rem;
    font-weight: 700;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    border: none;
    border-radius: var(--radius);
    cursor: pointer;
    transition: background-color 0.15s ease;
  }

  .btn-primary-benchmark:hover:not(:disabled) {
    background-color: var(--color-primary-hover);
  }

  .btn-primary-benchmark:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .secondary-actions {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.65rem;
  }

  .solo-classical-group {
    display: flex;
    gap: 0.4rem;
    flex: 1;
  }

  .field-select-compact {
    padding: 0.4rem 0.55rem;
    font-size: 0.75rem;
    background-color: var(--color-bg-secondary);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
  }

  .btn-secondary-compact {
    padding: 0.42rem 0.75rem;
    font-size: 0.75rem;
    font-weight: 600;
    background-color: var(--color-bg-secondary);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    white-space: nowrap;
  }

  .btn-secondary-compact:hover:not(:disabled) {
    background-color: var(--color-surface-hover);
  }

  .inline-alert {
    padding: 0.65rem 0.85rem;
    border-radius: var(--radius);
    font-size: 0.78rem;
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
  }

  .alert-warn {
    background-color: var(--color-warning-bg);
    border: 1px solid var(--color-warning-border);
    color: var(--color-warning);
  }

  .alert-error {
    background-color: var(--color-danger-bg);
    border: 1px solid var(--color-danger-border);
    color: var(--color-danger);
  }

  .btn-inline {
    padding: 0.25rem 0.55rem;
    font-size: 0.72rem;
    font-weight: 700;
    background-color: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    white-space: nowrap;
  }

  /* Results Presentation */
  .simulation-disclaimer {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-left: 3px solid var(--color-warning);
    border-radius: var(--radius-lg);
    padding: 1rem 1.35rem;
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
  }

  .disclaimer-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    flex-wrap: wrap;
    gap: 0.5rem;
  }

  .disclaimer-badge {
    font-size: 0.6875rem;
    font-weight: 800;
    letter-spacing: 0.06em;
    color: var(--color-warning);
    font-family: var(--font-mono, monospace);
  }

  .disclaimer-meta {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .disclaimer-body {
    font-size: 0.8125rem;
    color: var(--color-text);
    margin: 0;
  }

  .approx-banner {
    display: flex;
    align-items: center;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1rem 1.5rem;
    gap: 1.5rem;
  }

  .approx-kpi {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .approx-label {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
    font-family: var(--font-mono, monospace);
  }

  .approx-val {
    font-size: 1rem;
    font-weight: 800;
    color: var(--color-text);
  }

  .approx-divider {
    width: 1px;
    height: 36px;
    background-color: var(--color-border);
  }

  .comparison-pair {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1.25rem;
  }

  .compare-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.35rem 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .compare-card-quantum {
    border-color: var(--color-primary);
  }

  .compare-top {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 0.75rem;
  }

  .solver-eyebrow {
    font-size: 0.625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.06em;
    font-family: var(--font-mono, monospace);
  }

  .compare-title {
    font-size: 1.0625rem;
    font-weight: 800;
    color: var(--color-text);
    margin: 0.15rem 0 0 0;
  }

  .compare-metrics-grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 0.85rem;
  }

  .metric-row-5 {
    display: grid;
    grid-template-columns: repeat(5, 1fr);
    gap: 0.85rem;
  }

  .cm-item,
  .mini-kpi {
    background-color: var(--color-bg-secondary);
    padding: 0.65rem 0.75rem;
    border-radius: var(--radius);
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .cm-label,
  .mk-label {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .cm-val,
  .mk-val {
    font-size: 0.9375rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .feas-badge {
    display: inline-block;
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.2rem 0.5rem;
    border-radius: var(--radius-sm);
    font-family: var(--font-mono, monospace);
  }

  .feas-ok {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .feas-bad {
    background-color: var(--color-danger-bg);
    color: var(--color-danger);
    border: 1px solid var(--color-danger-border);
  }

  .violations-box {
    font-size: 0.75rem;
    color: var(--color-danger);
    background-color: var(--color-danger-bg);
    border: 1px solid var(--color-danger-border);
    padding: 0.5rem 0.75rem;
    border-radius: var(--radius);
  }

  /* Tables */
  .pill-kicker {
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-muted);
    padding: 0.25rem 0.6rem;
    background-color: var(--color-bg-secondary);
    border-radius: var(--radius);
    white-space: nowrap;
  }

  .table-wrap { overflow-x: auto; }

  .data-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .data-table th {
    background-color: var(--color-bg-secondary);
    padding: 0.7rem 0.9rem;
    text-align: left;
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--color-border);
  }

  .data-table td {
    padding: 0.7rem 0.9rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .highlight-row {
    background-color: rgba(214, 173, 85, 0.06);
  }

  .tag-pill {
    font-size: 0.6875rem;
    font-family: var(--font-mono, monospace);
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
  }

  .mini-tag {
    font-size: 0.625rem;
    font-weight: 800;
    padding: 0.12rem 0.4rem;
    border-radius: 3px;
    margin-left: 0.45rem;
    font-family: var(--font-mono, monospace);
  }

  .tag-best {
    background-color: var(--color-success-bg);
    color: var(--color-success);
  }

  .tag-quantum {
    background-color: var(--color-primary-focus);
    color: var(--color-primary);
  }

  .alloc-pill {
    display: inline-block;
    font-size: 0.6875rem;
    font-weight: 700;
    font-family: var(--font-mono, monospace);
    padding: 0.2rem 0.55rem;
    border-radius: var(--radius-sm);
  }

  .alloc-selected {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .alloc-quantum {
    background-color: var(--color-primary-focus);
    color: var(--color-primary);
    border: 1px solid var(--color-primary);
  }

  .alloc-skipped {
    background-color: var(--color-bg-secondary);
    color: var(--color-text-muted);
    border: 1px solid var(--color-border-subtle);
  }

  .weight-bar-cell {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 0.6rem;
  }

  .weight-track {
    width: 80px;
    height: 6px;
    background-color: var(--color-bg-secondary);
    border-radius: 999px;
    overflow: hidden;
  }

  .weight-fill {
    height: 100%;
    background-color: var(--color-primary);
    border-radius: 999px;
  }

  .text-success { color: var(--color-success); }
  .font-bold { font-weight: 700; }
  .font-mono { font-family: var(--font-mono, monospace); }
  .text-right { text-align: right; }
  .text-center { text-align: center; }

  @media (max-width: 1024px) {
    .lab-config-grid,
    .comparison-pair {
      grid-template-columns: 1fr;
    }
    .cluster-status-banner,
    .approx-banner {
      flex-direction: column;
      align-items: flex-start;
      gap: 0.9rem;
    }
    .status-divider,
    .approx-divider {
      display: none;
    }
    .metric-row-5 {
      grid-template-columns: repeat(2, 1fr);
    }
  }

  @media (max-width: 640px) {
    .compare-metrics-grid,
    .snapshot-grid,
    .two-col-inputs,
    .depth-pills {
      grid-template-columns: 1fr 1fr;
    }
  }
</style>
