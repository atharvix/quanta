<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { user } from '$lib/auth';
  import { canWrite } from '$lib/roles';
  import { api, ApiError } from '$lib/api';
  import type {
    Treasury,
    StrategyExecutionLog,
    StrategiesListResponse,
    Asset,
    Liability,
    ConstraintAuditItem,
    OrchestrateAllocationResponse,
    AIRiskAnalyzeResponse,
    TreasuryStressTestResponse,
    AnalyzeTreasuryRiskInput,
    AdversarialStressTestInput,
    BayesianEvidence
  } from '$lib/types';
  import { formatCurrency, formatNumber, formatPercent, formatDate } from '$lib/format';
  import {
    parseAllocations,
    extractAllocations,
    calculateLiquidityFloor,
    formatBayesianEvidence,
    getRiskTierClass,
    formatTreasuryAction
  } from '$lib/risk';

  let treasury = $state<Treasury | null>(null);
  let strategies = $state<StrategyExecutionLog[]>([]);
  let isLoadingTreasury = $state(true);
  let isLoadingStrategies = $state(true);
  let isNotFound = $state(false);
  let treasuryError = $state<string | null>(null);
  let strategiesError = $state<string | null>(null);

  const strategyLimit = 10;
  let strategyOffset = $state(0);

  const treasuryId = $derived(page.params.id);

  // Strategy Generator Form State
  let showGenerator = $state(false);
  let selectedAlgorithm = $state('astar');
  let includeComparison = $state(false);
  let stepSize = $state('');
  let minLiquidity = $state('0.20');

  let assets = $state<Asset[]>([]);
  let liabilities = $state<Liability[]>([]);

  let isGenerating = $state(false);
  let generationError = $state<string | null>(null);
  let generationDetails = $state<string[]>([]);
  let fieldValidationErrors = $state<string[]>([]);
  let latestResult = $state<OrchestrateAllocationResponse | null>(null);

  // Tab Navigation State
  let activeTab = $state<'strategy' | 'risk' | 'stress'>('strategy');

  // Bayesian Risk Analysis State
  let signalInflation = $state<'unobserved' | 'true' | 'false'>('unobserved');
  let signalRateHike = $state<'unobserved' | 'true' | 'false'>('unobserved');
  let signalRecession = $state<'unobserved' | 'true' | 'false'>('unobserved');
  let runMonteCarlo = $state(true);
  let mcScenarios = $state('1000');

  let isAnalyzingRisk = $state(false);
  let riskResult = $state<AIRiskAnalyzeResponse | null>(null);
  let riskError = $state<string | null>(null);
  let riskDetails = $state<string[]>([]);

  // Adversarial Stress Testing State
  let isStressTesting = $state(false);
  let stressResult = $state<TreasuryStressTestResponse | null>(null);
  let stressError = $state<string | null>(null);
  let stressDetails = $state<string[]>([]);

  // Selected Saved Strategy for Risk / Stress Testing
  let selectedSavedStrategy = $state<StrategyExecutionLog | null>(null);

  const algorithmOptions = [
    { value: 'astar', label: 'A* Heuristic Search (astar)' },
    { value: 'bfs', label: 'Breadth-First Search (bfs)' },
    { value: 'dfs', label: 'Depth-First Search (dfs)' },
    { value: 'generate_and_test', label: 'Generate & Test (generate_and_test)' },
    { value: 'hill_climbing', label: 'Hill Climbing Search (hill_climbing)' },
    { value: 'best_first', label: 'Best-First Greedy Search (best_first)' }
  ];

  function loadSampleAssets() {
    assets = [
      {
        id: 'gov_bond',
        name: 'Government Bonds',
        asset_type: 'gov_bond',
        cost: 0.001,
        expected_return: 0.07,
        risk: 0.05,
        liquidity: 0.95,
        volatility: 0.03
      },
      {
        id: 'bluechip_equity',
        name: 'Bluechip Equity Index',
        asset_type: 'equity',
        cost: 0.002,
        expected_return: 0.12,
        risk: 0.18,
        liquidity: 0.85,
        volatility: 0.15
      },
      {
        id: 'commercial_paper',
        name: 'Corporate Commercial Paper',
        asset_type: 'corp_bond',
        cost: 0.001,
        expected_return: 0.08,
        risk: 0.08,
        liquidity: 0.75,
        volatility: 0.06
      }
    ];
  }

  function loadSampleLiabilities() {
    liabilities = [
      {
        id: 'senior_loan',
        name: 'Senior Revolving Facility',
        principal: 15.0,
        interest_rate: 0.065,
        maturity_months: 12,
        priority: 1,
        is_mandatory_reserve: true
      }
    ];
  }

  function addAsset() {
    assets = [
      ...assets,
      {
        id: `asset_${assets.length + 1}`,
        name: `Asset ${assets.length + 1}`,
        asset_type: 'bond',
        cost: 0.001,
        expected_return: 0.06,
        risk: 0.06,
        liquidity: 0.80,
        volatility: 0.04
      }
    ];
  }

  function removeAsset(index: number) {
    assets = assets.filter((_, i) => i !== index);
  }

  function addLiability() {
    liabilities = [
      ...liabilities,
      {
        id: `debt_${liabilities.length + 1}`,
        name: `Debt Facility ${liabilities.length + 1}`,
        principal: 10.0,
        interest_rate: 0.06,
        priority: 1,
        is_mandatory_reserve: false
      }
    ];
  }

  function removeLiability(index: number) {
    liabilities = liabilities.filter((_, i) => i !== index);
  }

  function validateGenerator(): boolean {
    const errs: string[] = [];

    if (assets.length === 0) {
      errs.push('At least one investment asset must be configured.');
    }

    assets.forEach((a, idx) => {
      if (!a.id.trim()) errs.push(`Asset #${idx + 1}: Identifier (id) is required.`);
      if (!a.name.trim()) errs.push(`Asset #${idx + 1}: Name is required.`);
      if (isNaN(a.risk) || a.risk < 0 || a.risk > 1) {
        errs.push(`Asset #${idx + 1} (${a.name || a.id}): Risk must be between 0.00 and 1.00.`);
      }
      if (isNaN(a.liquidity) || a.liquidity < 0 || a.liquidity > 1) {
        errs.push(`Asset #${idx + 1} (${a.name || a.id}): Liquidity must be between 0.00 and 1.00.`);
      }
      if (isNaN(a.expected_return) || a.expected_return < 0) {
        errs.push(`Asset #${idx + 1} (${a.name || a.id}): Expected return must be non-negative.`);
      }
      if (isNaN(a.cost) || a.cost < 0) {
        errs.push(`Asset #${idx + 1} (${a.name || a.id}): Cost must be non-negative.`);
      }
    });

    liabilities.forEach((l, idx) => {
      if (!l.id.trim()) errs.push(`Liability #${idx + 1}: Identifier (id) is required.`);
      if (isNaN(l.principal) || l.principal < 0) {
        errs.push(`Liability #${idx + 1} (${l.name || l.id}): Principal must be non-negative.`);
      }
      if (isNaN(l.interest_rate) || l.interest_rate < 0) {
        errs.push(`Liability #${idx + 1} (${l.name || l.id}): Interest rate must be non-negative.`);
      }
    });

    const liqFloor = parseFloat(minLiquidity);
    if (isNaN(liqFloor) || liqFloor < 0 || liqFloor > 1) {
      errs.push('Minimum liquidity constraint floor must be between 0.00 and 1.00.');
    }

    if (stepSize.trim()) {
      const step = parseFloat(stepSize);
      if (isNaN(step) || step <= 0) {
        errs.push('Optional step size must be a positive number if provided.');
      }
    }

    fieldValidationErrors = errs;
    return errs.length === 0;
  }

  async function handleGenerateStrategy(e: SubmitEvent) {
    e.preventDefault();
    generationError = null;
    generationDetails = [];
    fieldValidationErrors = [];

    if (!validateGenerator()) {
      return;
    }

    isGenerating = true;

    try {
      const payload: any = {
        treasury_id: treasuryId,
        algorithm: selectedAlgorithm,
        include_comparison: includeComparison,
        assets: assets.map((a) => ({
          ...a,
          cost: Number(a.cost),
          expected_return: Number(a.expected_return),
          risk: Number(a.risk),
          liquidity: Number(a.liquidity),
          volatility: a.volatility !== undefined ? Number(a.volatility) : 0.05
        })),
        liabilities: liabilities.map((l) => ({
          ...l,
          principal: Number(l.principal),
          interest_rate: Number(l.interest_rate),
          priority: Number(l.priority) || 1,
          is_mandatory_reserve: Boolean(l.is_mandatory_reserve)
        })),
        constraints: [
          {
            id: 'c_min_liq',
            constraint_type: 'minimum_liquidity',
            description: 'Ensure portfolio liquidity meets designated floor',
            parameters: {
              threshold: parseFloat(minLiquidity)
            }
          }
        ]
      };

      if (stepSize.trim()) {
        payload.step_size = parseFloat(stepSize);
      }

      const res = await api<OrchestrateAllocationResponse>('/api/v1/orchestrate/allocation', {
        method: 'POST',
        body: payload
      });

      latestResult = res;

      // Auto-refresh saved strategies list to reflect new run
      await fetchStrategies(0);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 502) {
          generationError = 'AI engine abhi available nahi (AI Decision Engine is currently offline or unreachable).';
        } else {
          generationError = err.message || 'Strategy orchestration failed.';
        }
        if (err.details && err.details.length > 0) {
          generationDetails = err.details;
        }
      } else if (err instanceof Error) {
        generationError = err.message;
      } else {
        generationError = 'An unexpected error occurred during strategy orchestration.';
      }
    } finally {
      isGenerating = false;
    }
  }

  async function fetchTreasuryDetails() {
    isLoadingTreasury = true;
    treasuryError = null;
    isNotFound = false;

    try {
      const data = await api<Treasury>(`/api/v1/treasuries/${treasuryId}`);
      treasury = data;
      if (data.min_liquidity_ratio) {
        minLiquidity = String(data.min_liquidity_ratio);
      }
    } catch (err: unknown) {
      if (err instanceof ApiError && (err.status === 404 || err.status === 400)) {
        isNotFound = true;
      } else if (err instanceof ApiError) {
        treasuryError = err.message || 'Failed to load treasury details.';
      } else if (err instanceof Error) {
        treasuryError = err.message;
      } else {
        treasuryError = 'An unexpected error occurred.';
      }
    } finally {
      isLoadingTreasury = false;
    }
  }

  async function fetchStrategies(targetOffset: number) {
    isLoadingStrategies = true;
    strategiesError = null;

    try {
      const res = await api<StrategiesListResponse>(
        `/api/v1/treasuries/${treasuryId}/strategies?limit=${strategyLimit}&offset=${targetOffset}`
      );
      strategies = res.data || [];
      strategyOffset = targetOffset;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        strategiesError = err.message || 'Failed to load strategy execution logs.';
      } else if (err instanceof Error) {
        strategiesError = err.message;
      } else {
        strategiesError = 'Failed to load strategies.';
      }
    } finally {
      isLoadingStrategies = false;
    }
  }


  function parseConstraintsAudit(raw: any): ConstraintAuditItem[] {
    if (!raw) return [];
    let obj = raw;
    if (typeof raw === 'string') {
      try {
        obj = JSON.parse(raw);
      } catch {
        return [];
      }
    }
    if (Array.isArray(obj)) {
      return obj as ConstraintAuditItem[];
    }
    return [];
  }

  // Derive comparison statistics
  const comparisonResultsList = $derived(
    latestResult?.comparison?.results ? Object.entries(latestResult.comparison.results) : []
  );

  const bestComparisonObjective = $derived.by(() => {
    if (comparisonResultsList.length === 0) return 0;
    return Math.max(...comparisonResultsList.map(([_, r]) => r.objective_value));
  });

  const bestComparisonAlgo = $derived.by(() => {
    if (comparisonResultsList.length === 0) return '';
    let bestKey = '';
    let maxVal = -Infinity;
    for (const [key, r] of comparisonResultsList) {
      if (r.objective_value > maxVal) {
        maxVal = r.objective_value;
        bestKey = key;
      }
    }
    return bestKey;
  });

  const totalAllocationAmount = $derived.by(() => {
    if (!latestResult) return 0;
    const items = parseAllocations(latestResult.ai.allocations_json);
    return items.reduce((acc, it) => acc + it.amount, 0);
  });

  function getCurrentAllocations(): Record<string, number> {
    const rawAllocations = latestResult?.ai?.allocations_json ?? selectedSavedStrategy?.allocations_json;
    return extractAllocations(rawAllocations, assets);
  }

  const hasActiveAllocationStrategy = $derived(
    (latestResult !== null && latestResult.ai !== undefined) || selectedSavedStrategy !== null
  );
  const activeStrategyAlgorithm = $derived(
    latestResult?.ai?.algorithm_used ?? selectedSavedStrategy?.algorithm_used ?? ''
  );
  const activeStrategyRunId = $derived(
    latestResult?.run_id ?? selectedSavedStrategy?.id ?? ''
  );
  const currentAllocationsMap = $derived(getCurrentAllocations());
  const allocatedAssetCount = $derived(Object.keys(currentAllocationsMap).length);

  function selectStrategyForAnalysis(s: StrategyExecutionLog) {
    selectedSavedStrategy = s;
  }

  function clearSelectedStrategy() {
    selectedSavedStrategy = null;
  }

  function resetBayesianEvidence() {
    signalInflation = 'unobserved';
    signalRateHike = 'unobserved';
    signalRecession = 'unobserved';
  }

  async function handleAnalyzeRisk() {
    if (!treasury || isAnalyzingRisk) return;
    riskError = null;
    riskDetails = [];

    if (assets.length === 0) {
      riskError = 'At least one investment asset must be configured in the asset universe.';
      return;
    }

    isAnalyzingRisk = true;

    try {
      const evidence = formatBayesianEvidence(signalInflation, signalRateHike, signalRecession);
      const numScenariosVal = parseInt(mcScenarios, 10);
      const thresholdVal = calculateLiquidityFloor(minLiquidity, treasury.min_liquidity_ratio);

      const payload: AnalyzeTreasuryRiskInput = {
        assets: assets.map((a) => ({
          ...a,
          cost: Number(a.cost),
          expected_return: Number(a.expected_return),
          risk: Number(a.risk),
          liquidity: Number(a.liquidity),
          volatility: a.volatility !== undefined ? Number(a.volatility) : 0.05
        })),
        liabilities: liabilities.map((l) => ({
          ...l,
          principal: Number(l.principal),
          interest_rate: Number(l.interest_rate),
          priority: Number(l.priority) || 1,
          is_mandatory_reserve: Boolean(l.is_mandatory_reserve)
        })),
        constraints: [
          {
            id: 'c_min_liq',
            constraint_type: 'minimum_liquidity',
            description: 'Ensure portfolio liquidity meets designated floor',
            parameters: {
              threshold: thresholdVal
            }
          }
        ],
        allocations: getCurrentAllocations(),
        evidence,
        run_monte_carlo: runMonteCarlo,
        num_scenarios: isNaN(numScenariosVal) || numScenariosVal <= 0 ? 1000 : numScenariosVal
      };

      if (stepSize.trim()) {
        payload.step_size = parseFloat(stepSize);
      }

      const res = await api<AIRiskAnalyzeResponse>(`/api/v1/treasuries/${treasuryId}/risk/analyze`, {
        method: 'POST',
        body: payload
      });

      riskResult = res;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 502) {
          riskError = 'AI Engine offline: Bayesian risk reasoning service is currently unavailable.';
        } else if (err.status === 403) {
          riskError = 'Forbidden: Access to analyze this treasury is restricted to its owner and administrators.';
        } else {
          riskError = err.message || 'Risk analysis failed.';
        }
        if (err.details && err.details.length > 0) {
          riskDetails = err.details;
        }
      } else if (err instanceof Error) {
        riskError = err.message;
      } else {
        riskError = 'An unexpected error occurred during risk analysis.';
      }
    } finally {
      isAnalyzingRisk = false;
    }
  }

  async function handleStressTest() {
    if (!treasury || isStressTesting) return;
    stressError = null;
    stressDetails = [];

    if (assets.length === 0) {
      stressError = 'At least one investment asset must be configured in the asset universe.';
      return;
    }

    isStressTesting = true;

    try {
      const thresholdVal = calculateLiquidityFloor(minLiquidity, treasury.min_liquidity_ratio);

      const payload: AdversarialStressTestInput = {
        assets: assets.map((a) => ({
          ...a,
          cost: Number(a.cost),
          expected_return: Number(a.expected_return),
          risk: Number(a.risk),
          liquidity: Number(a.liquidity),
          volatility: a.volatility !== undefined ? Number(a.volatility) : 0.05
        })),
        liabilities: liabilities.map((l) => ({
          ...l,
          principal: Number(l.principal),
          interest_rate: Number(l.interest_rate),
          priority: Number(l.priority) || 1,
          is_mandatory_reserve: Boolean(l.is_mandatory_reserve)
        })),
        constraints: [
          {
            id: 'c_min_liq',
            constraint_type: 'minimum_liquidity',
            description: 'Ensure portfolio liquidity meets designated floor',
            parameters: {
              threshold: thresholdVal
            }
          }
        ],
        allocations: getCurrentAllocations()
      };

      if (stepSize.trim()) {
        payload.step_size = parseFloat(stepSize);
      }

      const res = await api<TreasuryStressTestResponse>(`/api/v1/treasuries/${treasuryId}/stress-test`, {
        method: 'POST',
        body: payload
      });

      stressResult = res;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 502) {
          stressError = 'AI Engine offline: Adversarial stress testing service is currently unavailable.';
        } else if (err.status === 403) {
          stressError = 'Forbidden: Access to stress test this treasury is restricted to its owner and administrators.';
        } else {
          stressError = err.message || 'Adversarial stress testing failed.';
        }
        if (err.details && err.details.length > 0) {
          stressDetails = err.details;
        }
      } else if (err instanceof Error) {
        stressError = err.message;
      } else {
        stressError = 'An unexpected error occurred during stress testing.';
      }
    } finally {
      isStressTesting = false;
    }
  }

  onMount(() => {
    if (treasuryId) {
      fetchTreasuryDetails();
      fetchStrategies(0);
      loadSampleAssets();
      loadSampleLiabilities();
    }
  });
</script>

<div class="page-container">
  <div class="navigation-bar">
    <a href="/treasuries" class="btn btn-secondary btn-sm">
      Back to Treasuries
    </a>
  </div>

  {#if isLoadingTreasury}
    <div class="card empty-card">
      <p class="state-message">Loading treasury profile...</p>
    </div>
  {:else if isNotFound}
    <div class="card not-found-card">
      <h2 class="state-title">Treasury Not Found</h2>
      <p class="state-message">The corporate treasury with ID "{treasuryId}" could not be located or does not exist.</p>
      <div style="margin-top: 1.5rem;">
        <a href="/treasuries" class="btn btn-primary">
          Return to Treasuries List
        </a>
      </div>
    </div>
  {:else if treasuryError}
    <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center;">
      <span>{treasuryError}</span>
      <button type="button" class="btn btn-secondary btn-sm" onclick={fetchTreasuryDetails}>
        Retry
      </button>
    </div>
  {:else if treasury}
    <!-- Treasury Overview Card -->
    <div class="card treasury-card">
      <div class="treasury-header">
        <div>
          <h1 class="treasury-title">{treasury.name}</h1>
          <p class="treasury-id">ID: {treasury.id}</p>
        </div>
        <div style="display: flex; align-items: center; gap: 0.75rem;">
          <span class="currency-badge">{treasury.base_currency}</span>
          {#if canWrite($user?.role)}
            <button
              type="button"
              class="btn btn-primary btn-sm"
              onclick={() => {
                activeTab = 'strategy';
                showGenerator = !showGenerator;
              }}
            >
              {showGenerator && activeTab === 'strategy' ? 'Close Generator' : 'Generate Strategy'}
            </button>
          {/if}
        </div>
      </div>

      <div class="detail-grid" style="margin-top: 1.5rem;">
        <div class="detail-item">
          <span class="detail-label">Base Currency</span>
          <span class="detail-value">{treasury.base_currency}</span>
        </div>

        <div class="detail-item">
          <span class="detail-label">Total Capital Base</span>
          <span class="detail-value">{formatCurrency(treasury.total_capital, treasury.base_currency)}</span>
        </div>

        <div class="detail-item">
          <span class="detail-label">Risk Tolerance Policy</span>
          <span class="detail-value">{formatPercent(treasury.risk_tolerance)}</span>
        </div>

        <div class="detail-item">
          <span class="detail-label">Minimum Liquidity Floor</span>
          <span class="detail-value">{formatPercent(treasury.min_liquidity_ratio)}</span>
        </div>

        <div class="detail-item">
          <span class="detail-label">Onboarding Timestamp</span>
          <span class="detail-value" style="font-size: 0.9375rem;">{formatDate(treasury.created_at)}</span>
        </div>

        <div class="detail-item">
          <span class="detail-label">Last Policy Modification</span>
          <span class="detail-value" style="font-size: 0.9375rem;">{formatDate(treasury.updated_at)}</span>
        </div>
      </div>
    </div>

    <!-- Read-only note for viewers -->
    {#if !canWrite($user?.role)}
      <div class="notice-banner" style="margin-bottom: 2rem;">
        Read-only access: Your current role ({$user?.role || 'viewer'}) permits inspection only. Analyst or Admin credentials are required to synthesize new strategies or execute capital orchestration.
      </div>
    {/if}

    <!-- Workspace Navigation Tabs -->
    <div class="tabs-nav">
      <button
        type="button"
        class="tab-btn"
        class:tab-active={activeTab === 'strategy'}
        onclick={() => (activeTab = 'strategy')}
      >
        Capital Allocation
      </button>
      <button
        type="button"
        class="tab-btn"
        class:tab-active={activeTab === 'risk'}
        onclick={() => (activeTab = 'risk')}
      >
        AI Risk Analysis & VaR
      </button>
      <button
        type="button"
        class="tab-btn"
        class:tab-active={activeTab === 'stress'}
        onclick={() => (activeTab = 'stress')}
      >
        Adversarial Stress Testing
      </button>
    </div>

    <!-- TAB 1: CAPITAL ALLOCATION STRATEGY -->
    {#if activeTab === 'strategy'}
      <!-- Strategy Synthesis Form Card (Analyst/Admin only) -->
    {#if showGenerator && canWrite($user?.role)}
      <div class="card generator-card">
        <div class="section-header">
          <h2 class="section-title">Synthesize Capital Allocation Strategy</h2>
          <p class="section-subtitle">
            Configure asset universe, obligations, and algorithmic search parameters to execute orchestration.
          </p>
        </div>

        {#if generationError}
          <div class="alert alert-danger" style="margin-top: 1rem;">
            <p><strong>{generationError}</strong></p>
            {#if generationDetails.length > 0}
              <ul style="margin-top: 0.5rem; padding-left: 1.25rem;">
                {#each generationDetails as d}
                  <li>{d}</li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}

        {#if fieldValidationErrors.length > 0}
          <div class="alert alert-danger" style="margin-top: 1rem;">
            <p><strong>Please correct the following inputs:</strong></p>
            <ul style="margin-top: 0.5rem; padding-left: 1.25rem;">
              {#each fieldValidationErrors as err}
                <li>{err}</li>
              {/each}
            </ul>
          </div>
        {/if}

        <form onsubmit={handleGenerateStrategy} novalidate style="margin-top: 1.25rem;">
          <div class="settings-grid">
            <div class="form-group">
              <label for="algorithmSelect" class="form-label">Search Algorithm</label>
              <select
                id="algorithmSelect"
                class="form-input"
                bind:value={selectedAlgorithm}
                disabled={isGenerating}
              >
                {#each algorithmOptions as opt}
                  <option value={opt.value}>{opt.label}</option>
                {/each}
              </select>
            </div>

            <div class="form-group">
              <label for="stepSizeInput" class="form-label">Discretization Step Size (Optional)</label>
              <input
                id="stepSizeInput"
                type="number"
                step="any"
                min="0.1"
                class="form-input"
                placeholder="Leave blank for backend default (capital / 20)"
                bind:value={stepSize}
                disabled={isGenerating}
              />
            </div>

            <div class="form-group">
              <label for="minLiqInput" class="form-label">Liquidity Constraint Floor (0.00 to 1.00)</label>
              <input
                id="minLiqInput"
                type="number"
                step="0.05"
                min="0"
                max="1"
                class="form-input"
                bind:value={minLiquidity}
                disabled={isGenerating}
              />
            </div>
          </div>

          <div class="checkbox-group" style="margin-bottom: 1.5rem;">
            <label class="checkbox-label">
              <input
                type="checkbox"
                bind:checked={includeComparison}
                disabled={isGenerating}
              />
              <span>Include full comparative benchmarking of all six search algorithms</span>
            </label>
          </div>

          <!-- Assets Table Editor -->
          <div class="editor-section">
            <div class="editor-header">
              <div>
                <h3 class="editor-title">Investment Asset Universe</h3>
                <p class="editor-subtitle">Eligible asset classes for portfolio capital allocation</p>
              </div>
              <div style="display: flex; gap: 0.5rem;">
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  onclick={loadSampleAssets}
                  disabled={isGenerating}
                >
                  Load Sample Assets
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  onclick={addAsset}
                  disabled={isGenerating}
                >
                  + Add Asset
                </button>
              </div>
            </div>

            <div class="table-container" style="margin-top: 0.75rem;">
              <table class="table editor-table">
                <thead>
                  <tr>
                    <th>Identifier</th>
                    <th>Name</th>
                    <th>Type</th>
                    <th>Exp. Return</th>
                    <th>Risk (0-1)</th>
                    <th>Liquidity (0-1)</th>
                    <th>Cost</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {#each assets as asset, idx (idx)}
                    <tr>
                      <td>
                        <input
                          type="text"
                          class="form-input editor-input"
                          bind:value={asset.id}
                          disabled={isGenerating}
                          placeholder="id"
                        />
                      </td>
                      <td>
                        <input
                          type="text"
                          class="form-input editor-input"
                          bind:value={asset.name}
                          disabled={isGenerating}
                          placeholder="name"
                        />
                      </td>
                      <td>
                        <input
                          type="text"
                          class="form-input editor-input"
                          bind:value={asset.asset_type}
                          disabled={isGenerating}
                          placeholder="type"
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          class="form-input editor-input"
                          bind:value={asset.expected_return}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          max="1"
                          class="form-input editor-input"
                          bind:value={asset.risk}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.01"
                          min="0"
                          max="1"
                          class="form-input editor-input"
                          bind:value={asset.liquidity}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.001"
                          min="0"
                          class="form-input editor-input"
                          bind:value={asset.cost}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <button
                          type="button"
                          class="btn btn-danger btn-xs"
                          onclick={() => removeAsset(idx)}
                          disabled={isGenerating || assets.length <= 1}
                        >
                          Remove
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>

          <!-- Liabilities Table Editor -->
          <div class="editor-section" style="margin-top: 1.5rem;">
            <div class="editor-header">
              <div>
                <h3 class="editor-title">Liabilities & Debt Obligations</h3>
                <p class="editor-subtitle">Outstanding maturities requiring liquidity provision</p>
              </div>
              <div style="display: flex; gap: 0.5rem;">
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  onclick={loadSampleLiabilities}
                  disabled={isGenerating}
                >
                  Load Sample Liabilities
                </button>
                <button
                  type="button"
                  class="btn btn-secondary btn-xs"
                  onclick={addLiability}
                  disabled={isGenerating}
                >
                  + Add Liability
                </button>
              </div>
            </div>

            <div class="table-container" style="margin-top: 0.75rem;">
              <table class="table editor-table">
                <thead>
                  <tr>
                    <th>Identifier</th>
                    <th>Name</th>
                    <th>Principal</th>
                    <th>Interest Rate</th>
                    <th>Priority</th>
                    <th>Mandatory Reserve</th>
                    <th>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {#each liabilities as liab, idx (idx)}
                    <tr>
                      <td>
                        <input
                          type="text"
                          class="form-input editor-input"
                          bind:value={liab.id}
                          disabled={isGenerating}
                          placeholder="id"
                        />
                      </td>
                      <td>
                        <input
                          type="text"
                          class="form-input editor-input"
                          bind:value={liab.name}
                          disabled={isGenerating}
                          placeholder="name"
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="any"
                          min="0"
                          class="form-input editor-input"
                          bind:value={liab.principal}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          step="0.005"
                          min="0"
                          class="form-input editor-input"
                          bind:value={liab.interest_rate}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <input
                          type="number"
                          min="1"
                          class="form-input editor-input"
                          bind:value={liab.priority}
                          disabled={isGenerating}
                        />
                      </td>
                      <td style="text-align: center;">
                        <input
                          type="checkbox"
                          bind:checked={liab.is_mandatory_reserve}
                          disabled={isGenerating}
                        />
                      </td>
                      <td>
                        <button
                          type="button"
                          class="btn btn-danger btn-xs"
                          onclick={() => removeLiability(idx)}
                          disabled={isGenerating}
                        >
                          Remove
                        </button>
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>

          <div style="margin-top: 1.5rem; display: flex; gap: 0.75rem; align-items: center;">
            <button type="submit" class="btn btn-primary" disabled={isGenerating}>
              {isGenerating ? 'Synthesizing Strategy...' : 'Execute Allocation Orchestration'}
            </button>
            <button
              type="button"
              class="btn btn-secondary"
              disabled={isGenerating}
              onclick={() => (showGenerator = false)}
            >
              Cancel
            </button>
          </div>

          {#if isGenerating}
            <p class="state-message" style="margin-top: 0.75rem;">
              Executing AI search and synthesizing allocation strategy... This may take several seconds.
            </p>
          {/if}
        </form>
      </div>
    {/if}

    <!-- Result View of Latest Generated Strategy -->
    {#if latestResult}
      <div class="card strategy-result-card">
        <div class="section-header">
          <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem;">
            <div>
              <span class="badge badge-pass" style="margin-bottom: 0.35rem;">Orchestration Completed</span>
              <h2 class="section-title">Synthesized Allocation Strategy</h2>
              <p class="section-subtitle">Run ID: {latestResult.run_id}</p>
            </div>
            <div>
              <span class="algo-badge" style="font-size: 0.8125rem;">
                Algorithm: {latestResult.ai.algorithm_used}
              </span>
            </div>
          </div>
        </div>

        <!-- Strategy Metrics Summary -->
        <div class="detail-grid" style="margin-top: 1.25rem;">
          <div class="detail-item">
            <span class="detail-label">Objective Value</span>
            <span class="detail-value">{formatNumber(latestResult.ai.objective_value)}</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">Expected Return</span>
            <span class="detail-value">{formatPercent(latestResult.ai.expected_return)}</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">Portfolio Risk Score</span>
            <span class="detail-value">{formatNumber(latestResult.ai.risk_score)}</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">Liquidity Ratio</span>
            <span class="detail-value">{formatPercent(latestResult.ai.liquidity_ratio)}</span>
          </div>

          <div class="detail-item">
            <span class="detail-label">Execution Time</span>
            <span class="detail-value">{latestResult.ai.execution_time_ms.toFixed(1)} ms</span>
          </div>
        </div>

        <!-- AI Natural Language Explanation -->
        {#if latestResult.ai.explanation}
          <div class="explanation-box" style="margin-top: 1.25rem;">
            <span class="detail-label">AI Decision Explanation</span>
            <p class="explanation-text">{latestResult.ai.explanation}</p>
          </div>
        {/if}

        <!-- Allocations Horizontal Bar Chart -->
        <div style="margin-top: 1.75rem;">
          <h3 class="section-title" style="font-size: 1.0625rem;">Capital Allocation Breakdown</h3>
          <p class="section-subtitle">Distribution of capital base across evaluated asset classes</p>

          <div class="chart-container">
            {#each parseAllocations(latestResult.ai.allocations_json) as item}
              {@const pct = totalAllocationAmount > 0 ? (item.amount / totalAllocationAmount) * 100 : 0}
              <div class="bar-row">
                <span class="bar-label" title={item.asset}>{item.asset}</span>
                <div class="bar-track">
                  <div class="bar-fill" style="width: {Math.min(100, Math.max(2, pct))}%;"></div>
                </div>
                <span class="bar-value">{item.formattedAmount} ({pct.toFixed(1)}%)</span>
              </div>
            {:else}
              <p class="state-message">No discrete asset allocations returned.</p>
            {/each}
          </div>
        </div>

        <!-- Constraints Audit Table -->
        <div style="margin-top: 1.75rem;">
          <h3 class="section-title" style="font-size: 1.0625rem;">Constraints Compliance Audit</h3>
          <p class="section-subtitle">Regulatory and policy constraint verification</p>

          <div class="table-container" style="margin-top: 0.75rem;">
            <table class="table">
              <thead>
                <tr>
                  <th>Constraint Type</th>
                  <th>Required Floor</th>
                  <th>Actual Achieved</th>
                  <th>Compliance Status</th>
                  <th>Audit Note</th>
                </tr>
              </thead>
              <tbody>
                {#each parseConstraintsAudit(latestResult.ai.constraints_audit_json) as audit}
                  <tr>
                    <td><code>{audit.constraint_type}</code></td>
                    <td class="numeric-cell">{formatNumber(audit.required)}</td>
                    <td class="numeric-cell">{formatNumber(audit.actual)}</td>
                    <td>
                      {#if audit.satisfied}
                        <span class="badge badge-pass">Pass</span>
                      {:else}
                        <span class="badge badge-fail">Fail</span>
                      {/if}
                    </td>
                    <td>{audit.message || '-'}</td>
                  </tr>
                {:else}
                  <tr>
                    <td colspan="5" style="text-align: center; color: var(--color-text-muted);">
                      No constraint audit items logged.
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </div>

        <!-- Algorithm Comparison Benchmark (when included) -->
        {#if latestResult.comparison?.results}
          <div style="margin-top: 2rem;">
            <h3 class="section-title" style="font-size: 1.0625rem;">Algorithmic Search Benchmark</h3>
            <p class="section-subtitle">Side-by-side performance comparison across deterministic heuristics</p>

            <div class="table-container" style="margin-top: 0.75rem;">
              <table class="table">
                <thead>
                  <tr>
                    <th>Algorithm</th>
                    <th>Objective Value</th>
                    <th>Expected Return</th>
                    <th>Risk Score</th>
                    <th>Liquidity Ratio</th>
                    <th>States Explored</th>
                    <th>Execution Time</th>
                  </tr>
                </thead>
                <tbody>
                  {#each comparisonResultsList as [algoKey, compResult]}
                    {@const isBest = algoKey === bestComparisonAlgo}
                    <tr class:row-highlight={isBest}>
                      <td>
                        <span class="algo-badge">{algoKey}</span>
                        {#if isBest}
                          <span class="badge badge-pass" style="margin-left: 0.5rem;">Best Objective</span>
                        {/if}
                      </td>
                      <td class="numeric-cell">{formatNumber(compResult.objective_value)}</td>
                      <td class="numeric-cell">{formatPercent(compResult.expected_return)}</td>
                      <td class="numeric-cell">{formatNumber(compResult.risk_score)}</td>
                      <td class="numeric-cell">{formatPercent(compResult.liquidity_ratio)}</td>
                      <td class="numeric-cell">{compResult.states_explored}</td>
                      <td class="numeric-cell">{compResult.execution_time_ms.toFixed(1)} ms</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <!-- Objective Value Comparative Bar Chart -->
            <div style="margin-top: 1.25rem;">
              <h4 class="editor-title" style="font-size: 0.875rem;">Objective Value Comparison Chart</h4>
              <div class="chart-container">
                {#each comparisonResultsList as [algoKey, compResult]}
                  {@const isBest = algoKey === bestComparisonAlgo}
                  {@const barWidth = bestComparisonObjective > 0 ? (compResult.objective_value / bestComparisonObjective) * 100 : 0}
                  <div class="bar-row">
                    <span class="bar-label" title={algoKey}>
                      {algoKey}
                      {#if isBest}
                        <span class="badge badge-pass" style="font-size: 0.625rem; padding: 0.1rem 0.3rem;">Best</span>
                      {/if}
                    </span>
                    <div class="bar-track">
                      <div
                        class="bar-fill"
                        class:bar-fill-highlight={isBest}
                        style="width: {Math.min(100, Math.max(2, barWidth))}%;"
                      ></div>
                    </div>
                    <span class="bar-value">{formatNumber(compResult.objective_value)}</span>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        {/if}

        <!-- Optimization Engine Status -->
        <div class="optimization-status-box" style="margin-top: 1.75rem;">
          <h4 class="editor-title" style="font-size: 0.875rem;">Optimization Engine Status</h4>
          {#if latestResult.optimization.status === 'not_available'}
            <p class="state-message" style="margin-top: 0.25rem;">
              Optimization engine abhi integrate nahi hua (Member 2 solver awaiting integration).
            </p>
          {:else}
            <p style="margin-top: 0.25rem; font-size: 0.875rem;">
              Status: <strong>{latestResult.optimization.status}</strong> - {latestResult.optimization.reason}
            </p>
          {/if}
        </div>
      </div>
    {/if}

    <!-- Historical Saved Strategies Section -->
    <div class="strategies-section">
      <div class="section-header">
        <div>
          <h2 class="section-title">Saved Execution Strategies</h2>
          <p class="section-subtitle">Historical audit trail of quantitative algorithm outputs and capital allocations</p>
        </div>
      </div>

      {#if strategiesError}
        <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center;">
          <span>{strategiesError}</span>
          <button type="button" class="btn btn-secondary btn-sm" onclick={() => fetchStrategies(strategyOffset)}>
            Retry
          </button>
        </div>
      {/if}

      {#if isLoadingStrategies}
        <div class="card empty-card">
          <p class="state-message">Loading strategy execution logs...</p>
        </div>
      {:else if strategies.length === 0}
        <div class="card empty-card">
          <h3 class="state-title">No Strategies Recorded</h3>
          <p class="state-message">
            {#if strategyOffset > 0}
              No additional strategy logs on this page.
            {:else}
              No optimization runs have been executed for this corporate balance sheet yet.
            {/if}
          </p>
        </div>
      {:else}
        <div class="table-container">
          <table class="table">
            <thead>
              <tr>
                <th>Algorithm</th>
                <th>Objective Value</th>
                <th>Risk Score</th>
                <th>Expected Return</th>
                <th>Liquidity Ratio</th>
                <th>Execution Time</th>
                <th>Allocations Breakdown</th>
                <th>Created Date</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {#each strategies as s (s.id)}
                <tr>
                  <td>
                    <span class="algo-badge">{s.algorithm_used}</span>
                  </td>
                  <td class="numeric-cell">
                    {formatNumber(s.objective_value)}
                  </td>
                  <td class="numeric-cell">
                    {formatNumber(s.risk_score)}
                  </td>
                  <td class="numeric-cell">
                    {formatPercent(s.expected_return)}
                  </td>
                  <td class="numeric-cell">
                    {formatPercent(s.liquidity_ratio)}
                  </td>
                  <td class="numeric-cell">
                    {s.execution_time_ms ? `${s.execution_time_ms.toFixed(1)} ms` : '-'}
                  </td>
                  <td>
                    <div class="allocations-wrap">
                      {#each parseAllocations(s.allocations_json) as item}
                        <span class="tag-pill">
                          <strong>{item.asset}:</strong> {item.formattedAmount}
                        </span>
                      {:else}
                        <span style="color: var(--color-text-muted); font-size: 0.75rem;">None</span>
                      {/each}
                    </div>
                  </td>
                  <td>
                    {formatDate(s.created_at)}
                  </td>
                  <td>
                    <button
                      type="button"
                      class="btn btn-secondary btn-xs"
                      title="Load this historical allocation into AI Risk & Stress Testing"
                      onclick={() => {
                        selectStrategyForAnalysis(s);
                        activeTab = 'risk';
                      }}
                    >
                      Select for Risk/Stress
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>

        <div class="pagination">
          <span class="pagination-info">
            Showing records {strategyOffset + 1} to {strategyOffset + strategies.length}
          </span>
          <div class="pagination-actions">
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              disabled={strategyOffset === 0 || isLoadingStrategies}
              onclick={() => fetchStrategies(Math.max(0, strategyOffset - strategyLimit))}
            >
              Previous
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              disabled={strategies.length < strategyLimit || isLoadingStrategies}
              onclick={() => fetchStrategies(strategyOffset + strategyLimit)}
            >
              Next
            </button>
          </div>
        </div>
      {/if}
    </div>
    {/if}

    <!-- TAB 2: AI RISK ANALYSIS & MONTE CARLO VaR -->
    {#if activeTab === 'risk'}
      <!-- Portfolio State Context Banner -->
      <div class="portfolio-banner">
        <div class="portfolio-banner-header">
          <span class="detail-label">Evaluated Portfolio Target</span>
          {#if hasActiveAllocationStrategy}
            <div style="display: flex; gap: 0.5rem; align-items: center;">
              <span class="badge badge-pass">Strategy Allocations Active</span>
              {#if selectedSavedStrategy}
                <button type="button" class="btn btn-secondary btn-xs" onclick={clearSelectedStrategy}>
                  Reset to Baseline
                </button>
              {/if}
            </div>
          {:else}
            <span class="badge badge-viewer">Baseline Balance Sheet</span>
          {/if}
        </div>
        {#if hasActiveAllocationStrategy}
          <p class="portfolio-banner-text">
            Analyzing portfolio strategy <strong>({activeStrategyAlgorithm})</strong> [Ref: <code>{activeStrategyRunId}</code>].
            Evaluates {allocatedAssetCount} discrete allocation(s) totaling {formatCurrency(totalAllocationAmount, treasury.base_currency)}.
          </p>
        {:else}
          <p class="portfolio-banner-text">
            No synthesized allocation strategy is active. Risk analysis will evaluate the baseline balance-sheet cash position ({formatCurrency(treasury.total_capital, treasury.base_currency)}) against configured asset risk parameters and mandatory debt reserves.
            To analyze optimized allocations, synthesize an allocation strategy in the Capital Allocation tab or select a saved strategy below.
          </p>
        {/if}
      </div>

      <!-- Risk Analysis Controls Card -->
      <div class="card" style="margin-bottom: 2rem;">
        <div class="section-header">
          <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem;">
            <div>
              <h2 class="section-title">Bayesian Network Conditioning & Simulation Settings</h2>
              <p class="section-subtitle">
                Configure macroeconomic observation signals to update Bayesian risk posteriors, and define Monte Carlo simulation parameters.
              </p>
            </div>
            <div>
              <button
                type="button"
                class="btn btn-secondary btn-xs"
                onclick={resetBayesianEvidence}
                disabled={isAnalyzingRisk}
              >
                Reset Signals to Prior
              </button>
            </div>
          </div>
        </div>

        {#if riskError}
          <div class="alert alert-danger" style="margin-top: 1rem;">
            <p><strong>{riskError}</strong></p>
            {#if riskDetails.length > 0}
              <ul style="margin-top: 0.5rem; padding-left: 1.25rem;">
                {#each riskDetails as d}
                  <li>{d}</li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}

        <div class="settings-grid" style="margin-top: 1.25rem;">
          <div class="form-group">
            <label for="inflationSignalSelect" class="form-label">Signal: High Inflation Environment</label>
            <select
              id="inflationSignalSelect"
              class="form-input"
              bind:value={signalInflation}
              disabled={isAnalyzingRisk}
            >
              <option value="unobserved">Unobserved (Prior Default)</option>
              <option value="true">Observed Active (True)</option>
              <option value="false">Observed Inactive (False)</option>
            </select>
          </div>

          <div class="form-group">
            <label for="rateHikeSignalSelect" class="form-label">Signal: Central Bank Rate Hike</label>
            <select
              id="rateHikeSignalSelect"
              class="form-input"
              bind:value={signalRateHike}
              disabled={isAnalyzingRisk}
            >
              <option value="unobserved">Unobserved (Prior Default)</option>
              <option value="true">Observed Active (True)</option>
              <option value="false">Observed Inactive (False)</option>
            </select>
          </div>

          <div class="form-group">
            <label for="recessionSignalSelect" class="form-label">Signal: Yield Curve Recession Inversion</label>
            <select
              id="recessionSignalSelect"
              class="form-input"
              bind:value={signalRecession}
              disabled={isAnalyzingRisk}
            >
              <option value="unobserved">Unobserved (Prior Default)</option>
              <option value="true">Observed Active (True)</option>
              <option value="false">Observed Inactive (False)</option>
            </select>
          </div>
        </div>

        <div style="margin-top: 0.5rem; margin-bottom: 1.5rem; display: flex; flex-wrap: wrap; gap: 1.5rem; align-items: center;">
          <label class="checkbox-label">
            <input
              type="checkbox"
              bind:checked={runMonteCarlo}
              disabled={isAnalyzingRisk}
            />
            <span>Execute Monte Carlo return distribution simulation</span>
          </label>

          {#if runMonteCarlo}
            <div style="display: flex; align-items: center; gap: 0.5rem;">
              <label for="mcScenariosInput" class="form-label" style="margin-bottom: 0;">Scenarios:</label>
              <select
                id="mcScenariosInput"
                class="form-input"
                style="width: 120px;"
                bind:value={mcScenarios}
                disabled={isAnalyzingRisk}
              >
                <option value="500">500</option>
                <option value="1000">1,000</option>
                <option value="2500">2,500</option>
                <option value="5000">5,000</option>
              </select>
            </div>
          {/if}
        </div>

        <div style="display: flex; gap: 0.75rem; align-items: center;">
          <button
            type="button"
            class="btn btn-primary"
            disabled={isAnalyzingRisk || !canWrite($user?.role)}
            onclick={handleAnalyzeRisk}
          >
            {isAnalyzingRisk ? 'Executing Bayesian Risk Analysis...' : 'Run Bayesian Risk Analysis & VaR'}
          </button>
        </div>

        {#if isAnalyzingRisk}
          <p class="state-message" style="margin-top: 0.75rem;">
            Querying Bayesian network inference engine and executing Monte Carlo return simulations...
          </p>
        {/if}
      </div>

      <!-- Risk Analysis Results View -->
      {#if riskResult}
        <div class="card result-card" style="margin-bottom: 2rem;">
          <div class="section-header">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem;">
              <div>
                <span class="badge {getRiskTierClass(riskResult.risk_breakdown.risk_tier)}" style="margin-bottom: 0.35rem;">
                  {riskResult.risk_breakdown.risk_tier} RISK TIER
                </span>
                <h2 class="section-title">Portfolio Risk Decomposition</h2>
                <p class="section-subtitle">Granular explainability of risk scores and portfolio concentration</p>
              </div>
              <div class="composite-score-box">
                <span class="detail-label">Composite Risk Score</span>
                <span class="composite-score-value">{formatNumber(riskResult.risk_breakdown.composite_risk_score)}</span>
              </div>
            </div>
          </div>

          <div class="detail-grid" style="margin-top: 1.25rem;">
            <div class="detail-item">
              <span class="detail-label">Weighted Asset Risk</span>
              <span class="detail-value">{formatNumber(riskResult.risk_breakdown.weighted_asset_risk)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Concentration Index (HHI)</span>
              <span class="detail-value">{formatNumber(riskResult.risk_breakdown.concentration_index_hhi)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Concentration Penalty</span>
              <span class="detail-value">{formatNumber(riskResult.risk_breakdown.concentration_penalty)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Liquidity Deficit Penalty</span>
              <span class="detail-value">{formatNumber(riskResult.risk_breakdown.liquidity_deficit_penalty)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Leverage Penalty</span>
              <span class="detail-value">{formatNumber(riskResult.risk_breakdown.leverage_penalty)}</span>
            </div>
          </div>

          {#if riskResult.risk_breakdown.explanations && riskResult.risk_breakdown.explanations.length > 0}
            <div style="margin-top: 1.5rem;">
              <h3 class="editor-title" style="margin-bottom: 0.5rem;">Risk Factor Explanations</h3>
              <ul class="explanations-list">
                {#each riskResult.risk_breakdown.explanations as exp}
                  <li>{exp}</li>
                {/each}
              </ul>
            </div>
          {/if}
        </div>

        <!-- Bayesian Posterior Probabilities Card -->
        <div class="card result-card" style="margin-bottom: 2rem;">
          <div class="section-header">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem;">
              <div>
                <h2 class="section-title">Bayesian Macroeconomic Posterior Probabilities</h2>
                <p class="section-subtitle">Belief state inferred by conditional probability network</p>
              </div>
              <div>
                <span class="algo-badge" style="font-size: 0.8125rem;">
                  Stress Multiplier: {formatNumber(riskResult.bayesian_inference.stress_multiplier)}x
                </span>
              </div>
            </div>
          </div>

          <div class="detail-grid" style="margin-top: 1.25rem;">
            <div class="detail-item">
              <span class="detail-label">P(Recession)</span>
              <span class="detail-value">{formatPercent(riskResult.bayesian_inference.p_recession)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">P(Liquidity Crisis)</span>
              <span class="detail-value">{formatPercent(riskResult.bayesian_inference.p_liquidity_crisis)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">P(Equity Downturn)</span>
              <span class="detail-value">{formatPercent(riskResult.bayesian_inference.p_equity_downturn)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">P(Credit Spread Widening)</span>
              <span class="detail-value">{formatPercent(riskResult.bayesian_inference.p_credit_spread_widening)}</span>
            </div>
          </div>

          {#if riskResult.bayesian_inference.explanation}
            <div class="explanation-box" style="margin-top: 1.25rem;">
              <span class="detail-label">Bayesian Network Rationale</span>
              <p class="explanation-text">{riskResult.bayesian_inference.explanation}</p>
            </div>
          {/if}
        </div>

        <!-- Monte Carlo VaR Simulation Card -->
        {#if riskResult.monte_carlo_report}
          <div class="card result-card" style="margin-bottom: 2rem;">
            <div class="section-header">
              <h2 class="section-title">Monte Carlo Return Simulation & Value at Risk (VaR)</h2>
              <p class="section-subtitle">
                Stochastic parametric simulation ({formatNumber(riskResult.monte_carlo_report.num_scenarios)} scenarios)
              </p>
            </div>

            <div class="detail-grid" style="margin-top: 1.25rem;">
              <div class="detail-item">
                <span class="detail-label">Mean Simulated Return</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.mean_simulated_return)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Return Volatility (Std Dev)</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.std_simulated_return)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Value at Risk (95% VaR)</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.var_95)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Conditional VaR (95% CVaR)</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.cvar_95)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Value at Risk (99% VaR)</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.var_99)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Conditional VaR (99% CVaR)</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.cvar_99)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Worst-Case Simulated Loss</span>
                <span class="detail-value" style="color: var(--color-danger);">{formatPercent(riskResult.monte_carlo_report.worst_case_loss_pct)}</span>
              </div>

              <div class="detail-item">
                <span class="detail-label">Probability of Loss</span>
                <span class="detail-value">{formatPercent(riskResult.monte_carlo_report.probability_of_loss)}</span>
              </div>
            </div>
          </div>
        {/if}
      {/if}
    {/if}

    <!-- TAB 3: ADVERSARIAL STRESS TESTING -->
    {#if activeTab === 'stress'}
      <!-- Portfolio State Context Banner -->
      <div class="portfolio-banner">
        <div class="portfolio-banner-header">
          <span class="detail-label">Evaluated Portfolio Target</span>
          {#if hasActiveAllocationStrategy}
            <div style="display: flex; gap: 0.5rem; align-items: center;">
              <span class="badge badge-pass">Strategy Allocations Active</span>
              {#if selectedSavedStrategy}
                <button type="button" class="btn btn-secondary btn-xs" onclick={clearSelectedStrategy}>
                  Reset to Baseline
                </button>
              {/if}
            </div>
          {:else}
            <span class="badge badge-viewer">Baseline Balance Sheet</span>
          {/if}
        </div>
        {#if hasActiveAllocationStrategy}
          <p class="portfolio-banner-text">
            Stress testing portfolio strategy <strong>({activeStrategyAlgorithm})</strong> [Ref: <code>{activeStrategyRunId}</code>].
            Evaluates {allocatedAssetCount} discrete allocation(s) totaling {formatCurrency(totalAllocationAmount, treasury.base_currency)}.
          </p>
        {:else}
          <p class="portfolio-banner-text">
            No synthesized allocation strategy is active. Stress testing will evaluate the baseline balance-sheet cash position ({formatCurrency(treasury.total_capital, treasury.base_currency)}) against macroeconomic shocks and mandatory debt obligations.
            To stress test optimized allocations, synthesize an allocation strategy in the Capital Allocation tab or select a saved strategy below.
          </p>
        {/if}
      </div>

      <!-- Stress Test Controls Card -->
      <div class="card" style="margin-bottom: 2rem;">
        <div class="section-header">
          <h2 class="section-title">Minimax Adversarial Game Tree Search</h2>
          <p class="section-subtitle">
            Simulate two-player zero-sum adversarial game with Alpha-Beta pruning: Treasury portfolio manager vs. hostile market adversary.
          </p>
        </div>

        {#if stressError}
          <div class="alert alert-danger" style="margin-top: 1rem;">
            <p><strong>{stressError}</strong></p>
            {#if stressDetails.length > 0}
              <ul style="margin-top: 0.5rem; padding-left: 1.25rem;">
                {#each stressDetails as d}
                  <li>{d}</li>
                {/each}
              </ul>
            {/if}
          </div>
        {/if}

        <div style="margin-top: 1.25rem; display: flex; gap: 0.75rem; align-items: center;">
          <button
            type="button"
            class="btn btn-primary"
            disabled={isStressTesting || !canWrite($user?.role)}
            onclick={handleStressTest}
          >
            {isStressTesting ? 'Executing Minimax Adversarial Search...' : 'Run Adversarial Stress Test'}
          </button>
        </div>

        {#if isStressTesting}
          <p class="state-message" style="margin-top: 0.75rem;">
            Searching adversarial game tree with Alpha-Beta branch pruning and calculating discrete shock evaluations...
          </p>
        {/if}
      </div>

      <!-- Stress Test Results View -->
      {#if stressResult}
        <!-- Adversarial Search Result Card -->
        <div class="card result-card" style="margin-bottom: 2rem;">
          <div class="section-header">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 0.5rem;">
              <div>
                <span class="badge badge-analyst" style="margin-bottom: 0.35rem;">Alpha-Beta Minimax Completed</span>
                <h2 class="section-title">Adversarial Search Outcome</h2>
                <p class="section-subtitle">Worst-case macroeconomic response and optimal portfolio defensive posture</p>
              </div>
              <div class="composite-score-box">
                <span class="detail-label">Worst-Case Payoff</span>
                <span class="composite-score-value">{formatNumber(stressResult.adversarial_result.worst_case_payoff)}</span>
              </div>
            </div>
          </div>

          <div class="detail-grid" style="margin-top: 1.25rem;">
            <div class="detail-item">
              <span class="detail-label">Worst Market Shock</span>
              <span class="detail-value">{stressResult.adversarial_result.worst_market_shock}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Recommended Defensive Action</span>
              <span class="detail-value">{formatTreasuryAction(stressResult.adversarial_result.best_treasury_action)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Game Tree Nodes Evaluated</span>
              <span class="detail-value">{formatNumber(stressResult.adversarial_result.nodes_evaluated)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Pruned Branches (Alpha-Beta)</span>
              <span class="detail-value">{formatNumber(stressResult.adversarial_result.pruned_branches)}</span>
            </div>

            <div class="detail-item">
              <span class="detail-label">Search Execution Time</span>
              <span class="detail-value">{stressResult.adversarial_result.execution_time_ms.toFixed(1)} ms</span>
            </div>
          </div>

          {#if stressResult.adversarial_result.explanation}
            <div class="explanation-box" style="margin-top: 1.25rem;">
              <span class="detail-label">Adversarial Decision Rationale</span>
              <p class="explanation-text">{stressResult.adversarial_result.explanation}</p>
            </div>
          {/if}
        </div>

        <!-- 4 Macroeconomic Shock Scenarios Grid -->
        <div style="margin-bottom: 2.5rem;">
          <div class="section-header">
            <h2 class="section-title">Macroeconomic Stress Scenarios & Statutory Compliance</h2>
            <p class="section-subtitle">
              Deterministic portfolio stress evaluation across the four standard macroeconomic crisis scenarios. Compliance indicates whether minimum liquidity constraints remain satisfied under shock.
            </p>
          </div>

          <div class="scenarios-grid">
            {#each stressResult.scenarios as item}
              <div class="scenario-card" class:scenario-card-breach={!item.is_compliant}>
                <div class="scenario-card-header">
                  <div>
                    <h3 class="scenario-title">{item.scenario.shock_name}</h3>
                    <p class="scenario-desc">{item.scenario.description}</p>
                  </div>
                  <div>
                    {#if item.is_compliant}
                      <span class="badge badge-pass">COMPLIANT</span>
                    {:else}
                      <span class="badge badge-fail">BREACH</span>
                    {/if}
                  </div>
                </div>

                <div class="scenario-shocks-box">
                  <span class="detail-label" style="font-size: 0.6875rem;">Macro Shock Parameters</span>
                  <div class="scenario-shock-params">
                    <span>Equity: <strong>{formatPercent(item.scenario.equity_drawdown)}</strong></span>
                    <span>Spread: <strong>{formatPercent(item.scenario.credit_spread_spike)}</strong></span>
                    <span>Liquidity: <strong>{formatPercent(item.scenario.liquidity_freeze)}</strong></span>
                  </div>
                </div>

                <div class="scenario-metrics-grid">
                  <div class="scenario-metric">
                    <span class="detail-label" style="font-size: 0.6875rem;">Stressed Return</span>
                    <span class="scenario-metric-val" class:val-negative={item.stressed_return < 0}>
                      {formatPercent(item.stressed_return)}
                    </span>
                  </div>

                  <div class="scenario-metric">
                    <span class="detail-label" style="font-size: 0.6875rem;">Stressed Liquidity</span>
                    <span class="scenario-metric-val">
                      {formatPercent(item.stressed_liquidity)}
                    </span>
                  </div>

                  <div class="scenario-metric">
                    <span class="detail-label" style="font-size: 0.6875rem;">Stressed Risk</span>
                    <span class="scenario-metric-val">
                      {formatNumber(item.stressed_risk)}
                    </span>
                  </div>

                  <div class="scenario-metric">
                    <span class="detail-label" style="font-size: 0.6875rem;">Estimated Payoff</span>
                    <span class="scenario-metric-val">
                      {formatNumber(item.estimated_payoff)}
                    </span>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        </div>
      {/if}
    {/if}
  {/if}
</div>

<style>
  .page-container {
    max-width: 1100px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    width: 100%;
  }

  .navigation-bar {
    margin-bottom: 1.5rem;
  }

  .treasury-card {
    margin-bottom: 2rem;
  }

  .generator-card {
    margin-bottom: 2rem;
  }

  .strategy-result-card {
    margin-bottom: 2.5rem;
    border-left: 4px solid var(--color-primary);
  }

  .treasury-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    border-bottom: 1px solid var(--color-border);
    padding-bottom: 1.25rem;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .treasury-title {
    font-size: 1.75rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .treasury-id {
    font-size: 0.8125rem;
    font-family: monospace;
    color: var(--color-text-muted);
    margin-top: 0.25rem;
  }

  .currency-badge {
    display: inline-block;
    padding: 0.25rem 0.6rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
  }

  .settings-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 1rem;
    margin-bottom: 1rem;
  }

  .checkbox-group {
    display: flex;
    align-items: center;
  }

  .checkbox-label {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text);
    cursor: pointer;
  }

  .editor-section {
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem;
  }

  .editor-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .editor-title {
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .editor-subtitle {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .editor-table th {
    padding: 0.5rem 0.625rem;
    font-size: 0.6875rem;
  }

  .editor-table td {
    padding: 0.375rem 0.5rem;
  }

  .editor-input {
    padding: 0.25rem 0.5rem;
    font-size: 0.8125rem;
  }

  .explanation-box {
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.875rem 1rem;
  }

  .explanation-text {
    font-size: 0.875rem;
    color: var(--color-text);
    line-height: 1.5;
    margin-top: 0.25rem;
  }

  .optimization-status-box {
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.75rem 1rem;
  }

  .strategies-section {
    margin-top: 2rem;
  }

  .section-header {
    margin-bottom: 1.25rem;
  }

  .section-title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .section-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .algo-badge {
    display: inline-block;
    padding: 0.2rem 0.45rem;
    background-color: var(--color-badge-analyst-bg);
    color: var(--color-badge-analyst);
    border: 1px solid var(--color-badge-analyst-bg);
    border-radius: var(--radius);
    font-size: 0.75rem;
    font-weight: 600;
  }

  .allocations-wrap {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    max-width: 320px;
  }

  .numeric-cell {
    font-variant-numeric: tabular-nums;
  }

  .empty-card {
    text-align: center;
    padding: 3rem 1.5rem;
  }

  .not-found-card {
    text-align: center;
    padding: 3.5rem 1.5rem;
  }

  .state-title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.5rem;
  }

  .state-message {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  /* F5 Tab Navigation */
  .tabs-nav {
    display: flex;
    gap: 0.5rem;
    border-bottom: 2px solid var(--color-border);
    margin-bottom: 1.5rem;
  }

  .tab-btn {
    background: none;
    border: none;
    padding: 0.75rem 1.25rem;
    font-size: 0.9375rem;
    font-weight: 600;
    color: var(--color-text-muted);
    cursor: pointer;
    border-bottom: 2px solid transparent;
    margin-bottom: -2px;
    transition: color 0.15s ease, border-color 0.15s ease;
  }

  .tab-btn:hover {
    color: var(--color-text);
  }

  .tab-btn.tab-active {
    color: var(--color-primary);
    border-bottom-color: var(--color-primary);
  }

  /* F5 Portfolio State Context Banner */
  .portfolio-banner {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-left: 4px solid var(--color-primary);
    border-radius: var(--radius);
    padding: 1rem 1.25rem;
    margin-bottom: 1.5rem;
  }

  .portfolio-banner-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.35rem;
    gap: 0.5rem;
  }

  .portfolio-banner-text {
    font-size: 0.875rem;
    color: var(--color-text);
    line-height: 1.5;
  }

  .result-card {
    border-left: 4px solid var(--color-primary);
  }

  .composite-score-box {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
  }

  .composite-score-value {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-primary);
    font-variant-numeric: tabular-nums;
  }

  .explanations-list {
    margin: 0;
    padding-left: 1.25rem;
    font-size: 0.875rem;
    color: var(--color-text);
    line-height: 1.6;
  }

  /* F5 Risk Tier Badges */
  .badge-tier-low {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .badge-tier-moderate {
    background-color: var(--color-badge-analyst-bg);
    color: var(--color-badge-analyst);
    border: 1px solid var(--color-badge-analyst-bg);
  }

  .badge-tier-high,
  .badge-tier-critical {
    background-color: var(--color-danger-bg);
    color: var(--color-danger);
    border: 1px solid var(--color-danger-border);
  }

  /* F5 Stress Scenarios Grid */
  .scenarios-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
    gap: 1.25rem;
    margin-top: 1rem;
  }

  .scenario-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    box-shadow: var(--shadow-sm);
  }

  .scenario-card-breach {
    border-color: var(--color-danger-border);
    border-left: 4px solid var(--color-danger);
  }

  .scenario-card-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 0.5rem;
  }

  .scenario-title {
    font-size: 0.9375rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .scenario-desc {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin-top: 0.2rem;
  }

  .scenario-shocks-box {
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.5rem 0.75rem;
  }

  .scenario-shock-params {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    margin-top: 0.25rem;
    color: var(--color-text);
    flex-wrap: wrap;
    gap: 0.35rem;
  }

  .scenario-metrics-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 0.75rem;
  }

  .scenario-metric {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .scenario-metric-val {
    font-size: 0.9375rem;
    font-weight: 600;
    font-variant-numeric: tabular-nums;
  }

  .val-negative {
    color: var(--color-danger);
  }
</style>
