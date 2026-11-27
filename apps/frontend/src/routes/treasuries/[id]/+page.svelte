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
    BayesianEvidence,
    TreasuryAssetsResponse,
    TreasuryLiabilitiesResponse,
    UpdateTreasuryInput,
    CouncilDeliberationSummary,
    EvaluateCouncilInput
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
  let isLoadingAssets = $state(false);
  let isLoadingLiabilities = $state(false);
  let assetsError = $state<string | null>(null);
  let liabilitiesError = $state<string | null>(null);
  let actionSuccessMessage = $state<string | null>(null);
  let actionErrorMessage = $state<string | null>(null);

  // Canonical asset classifications matching Go backend and Python AI Engine
  const CANONICAL_ASSET_TYPES = [
    { value: 'gov_bond', label: 'Government Bonds (gov_bond)' },
    { value: 'corp_bond', label: 'Corporate Bonds (corp_bond)' },
    { value: 'equity', label: 'Public Equity (equity)' },
    { value: 'cash', label: 'Cash & Cash Equivalents (cash)' },
    { value: 'real_estate', label: 'Real Estate / Infrastructure (real_estate)' },
    { value: 'commodity', label: 'Commodities / Hard Assets (commodity)' },
    { value: 'other', label: 'Alternative / Other (other)' }
  ];

  // Modals State
  let showAssetModal = $state(false);
  let modalAssetMode = $state<'create' | 'edit'>('create');
  let assetForm = $state({
    id: '',
    name: '',
    asset_type: 'gov_bond',
    cost: '0.001',
    expected_return: '0.07',
    risk: '0.05',
    liquidity: '0.85',
    volatility: '0.04'
  });
  let assetModalErrors = $state<string[]>([]);
  let isSavingAsset = $state(false);

  let showLiabilityModal = $state(false);
  let modalLiabilityMode = $state<'create' | 'edit'>('create');
  let liabilityForm = $state({
    id: '',
    name: '',
    principal: '10.0',
    interest_rate: '0.06',
    maturity_months: '12',
    priority: '1',
    is_mandatory_reserve: false
  });
  let liabilityModalErrors = $state<string[]>([]);
  let isSavingLiability = $state(false);

  let showDeleteModal = $state(false);
  let deleteTargetType = $state<'asset' | 'liability'>('asset');
  let deleteTargetId = $state('');
  let deleteTargetName = $state('');
  let isDeleting = $state(false);

  // Edit Treasury State
  let showEditTreasuryModal = $state(false);
  let editTreasuryForm = $state({
    name: '',
    base_currency: 'INR',
    total_capital: '',
    risk_tolerance: '',
    min_liquidity_ratio: ''
  });
  let initialTreasuryForm = $state({
    name: '',
    base_currency: 'INR',
    total_capital: '',
    risk_tolerance: '',
    min_liquidity_ratio: ''
  });
  let editTreasuryErrors = $state<string[]>([]);
  let isSavingTreasury = $state(false);
  let showUnsavedPrompt = $state(false);

  const isTreasuryFormDirty = $derived(
    editTreasuryForm.name !== initialTreasuryForm.name ||
    editTreasuryForm.base_currency !== initialTreasuryForm.base_currency ||
    editTreasuryForm.total_capital !== initialTreasuryForm.total_capital ||
    editTreasuryForm.risk_tolerance !== initialTreasuryForm.risk_tolerance ||
    editTreasuryForm.min_liquidity_ratio !== initialTreasuryForm.min_liquidity_ratio
  );

  let isGenerating = $state(false);
  let generationError = $state<string | null>(null);
  let generationDetails = $state<string[]>([]);
  let fieldValidationErrors = $state<string[]>([]);
  let latestResult = $state<OrchestrateAllocationResponse | null>(null);

  // Tab Navigation State
  let activeTab = $state<'balance-sheet' | 'strategy' | 'risk' | 'stress' | 'council'>('balance-sheet');

  // Executive Balance Sheet Portfolio Calculations
  const totalCapital = $derived(treasury?.total_capital ?? 0);
  const totalLiabilitiesPrincipal = $derived.by(() =>
    liabilities.reduce((acc, l) => acc + (Number(l.principal) || 0), 0)
  );
  const netCapitalPosition = $derived(totalCapital - totalLiabilitiesPrincipal);
  const mandatoryDebtReserves = $derived.by(() =>
    liabilities.filter((l) => l.is_mandatory_reserve).reduce((acc, l) => acc + (Number(l.principal) || 0), 0)
  );
  const activeAssetCount = $derived(assets.length);
  const activeLiabilityCount = $derived(liabilities.length);

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

  // Multi-Agent Council State (Task T-3.1)
  let isEvaluatingCouncil = $state(false);
  let councilResult = $state<CouncilDeliberationSummary | null>(null);
  let councilError = $state<string | null>(null);
  let councilDetails = $state<string[]>([]);
  let councilObjective = $state('maximize_risk_adjusted_return');
  let councilMarketState = $state('normal');
  let councilStepSize = $state('');

  const algorithmOptions = [
    { value: 'astar', label: 'A* Heuristic Search (astar)' },
    { value: 'bfs', label: 'Breadth-First Search (bfs)' },
    { value: 'dfs', label: 'Depth-First Search (dfs)' },
    { value: 'generate_and_test', label: 'Generate & Test (generate_and_test)' },
    { value: 'hill_climbing', label: 'Hill Climbing Search (hill_climbing)' },
    { value: 'best_first', label: 'Best-First Greedy Search (best_first)' }
  ];

  const STARTER_ASSETS: Asset[] = [
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
  const STARTER_LIABILITIES: Liability[] = [
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

  let isSavingStarter = $state(false);

  async function fetchAssets() {
    isLoadingAssets = true;
    assetsError = null;
    try {
      const res = await api<TreasuryAssetsResponse>(`/api/v1/treasuries/${treasuryId}/assets`);
      assets = Array.isArray(res.data) ? res.data : [];
    } catch (err: unknown) {
      assets = [];
      if (err instanceof ApiError && err.status === 403) {
        assetsError = 'Forbidden: Access to view or modify assets for this treasury is restricted.';
      } else if (err instanceof ApiError) {
        assetsError = err.message || 'Failed to retrieve assets.';
      } else {
        assetsError = 'Failed to load assets.';
      }
    } finally {
      isLoadingAssets = false;
    }
  }

  async function fetchLiabilities() {
    isLoadingLiabilities = true;
    liabilitiesError = null;
    try {
      const res = await api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${treasuryId}/liabilities`);
      liabilities = Array.isArray(res.data) ? res.data : [];
    } catch (err: unknown) {
      liabilities = [];
      if (err instanceof ApiError && err.status === 403) {
        liabilitiesError = 'Forbidden: Access to view or modify liabilities for this treasury is restricted.';
      } else if (err instanceof ApiError) {
        liabilitiesError = err.message || 'Failed to retrieve liabilities.';
      } else {
        liabilitiesError = 'Failed to load liabilities.';
      }
    } finally {
      isLoadingLiabilities = false;
    }
  }

  async function saveStarterPortfolioToDatabase() {
    if (isSavingStarter) return;
    isSavingStarter = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;

    try {
      for (const item of STARTER_ASSETS) {
        try {
          await api<Asset>(`/api/v1/treasuries/${treasuryId}/assets`, {
            method: 'POST',
            body: item
          });
        } catch (err: unknown) {
          if (!(err instanceof ApiError && err.status === 409)) {
            throw err;
          }
        }
      }

      for (const item of STARTER_LIABILITIES) {
        try {
          await api<Liability>(`/api/v1/treasuries/${treasuryId}/liabilities`, {
            method: 'POST',
            body: item
          });
        } catch (err: unknown) {
          if (!(err instanceof ApiError && err.status === 409)) {
            throw err;
          }
        }
      }

      await Promise.all([fetchAssets(), fetchLiabilities()]);
      actionSuccessMessage = 'Starter institutional portfolio saved to database (3 assets, 1 debt facility).';
    } catch (err: unknown) {
      if (err instanceof ApiError && err.status === 403) {
        actionErrorMessage = 'Forbidden: Analyst or Admin permissions for this treasury are required to save starter data.';
      } else if (err instanceof ApiError) {
        actionErrorMessage = err.message || 'Failed to save starter portfolio to database.';
      } else if (err instanceof Error) {
        actionErrorMessage = err.message;
      } else {
        actionErrorMessage = 'Failed to save starter portfolio to database.';
      }
    } finally {
      isSavingStarter = false;
    }
  }

  function openAddAssetModal() {
    modalAssetMode = 'create';
    assetForm = {
      id: `asset_${assets.length + 1}`,
      name: `Asset ${assets.length + 1}`,
      asset_type: 'gov_bond',
      cost: '0.001',
      expected_return: '0.06',
      risk: '0.05',
      liquidity: '0.80',
      volatility: '0.04'
    };
    assetModalErrors = [];
    showAssetModal = true;
  }

  function openEditAssetModal(a: Asset) {
    modalAssetMode = 'edit';
    assetForm = {
      id: a.id,
      name: a.name,
      asset_type: a.asset_type,
      cost: String(a.cost),
      expected_return: String(a.expected_return),
      risk: String(a.risk),
      liquidity: String(a.liquidity),
      volatility: a.volatility !== undefined ? String(a.volatility) : '0.04'
    };
    assetModalErrors = [];
    showAssetModal = true;
  }

  async function handleSaveAsset(e: SubmitEvent) {
    e.preventDefault();
    assetModalErrors = [];
    const errs: string[] = [];

    if (!assetForm.id.trim()) errs.push('Asset identifier (id) is required.');
    if (!assetForm.name.trim()) errs.push('Asset name is required.');
    const trimmedType = assetForm.asset_type.trim().toLowerCase();
    if (!trimmedType) {
      errs.push('Asset type is required.');
    } else if (!CANONICAL_ASSET_TYPES.some((t) => t.value === trimmedType)) {
      errs.push(`Asset type '${assetForm.asset_type}' is invalid. Allowed: ${CANONICAL_ASSET_TYPES.map((t) => t.value).join(', ')}`);
    }

    const expRet = parseFloat(assetForm.expected_return);
    if (isNaN(expRet) || expRet < -1.0 || expRet > 5.0) errs.push('Expected return must be between -1.00 and 5.00.');

    const rk = parseFloat(assetForm.risk);
    if (isNaN(rk) || rk < 0 || rk > 1.0) errs.push('Risk score must be between 0.00 and 1.00.');

    const liq = parseFloat(assetForm.liquidity);
    if (isNaN(liq) || liq < 0 || liq > 1.0) errs.push('Liquidity score must be between 0.00 and 1.00.');

    const cst = parseFloat(assetForm.cost);
    if (isNaN(cst) || cst < 0) errs.push('Cost must be non-negative.');

    const vol = parseFloat(assetForm.volatility);
    if (isNaN(vol) || vol < 0) errs.push('Volatility must be non-negative.');

    if (modalAssetMode === 'create' && assets.some((a) => a.id.toLowerCase() === assetForm.id.trim().toLowerCase())) {
      errs.push(`An asset with identifier '${assetForm.id.trim()}' already exists in this treasury.`);
    }

    if (errs.length > 0) {
      assetModalErrors = errs;
      return;
    }

    isSavingAsset = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;

    try {
      const payload: Asset = {
        id: assetForm.id.trim(),
        name: assetForm.name.trim(),
        asset_type: assetForm.asset_type.trim(),
        cost: cst,
        expected_return: expRet,
        risk: rk,
        liquidity: liq,
        volatility: vol
      };

      if (modalAssetMode === 'create') {
        const created = await api<Asset>(`/api/v1/treasuries/${treasuryId}/assets`, {
          method: 'POST',
          body: payload
        });
        await fetchAssets();
        actionSuccessMessage = `Asset "${created.name}" created and saved to portfolio universe.`;
      } else {
        const updated = await api<Asset>(`/api/v1/treasuries/${treasuryId}/assets/${encodeURIComponent(payload.id)}`, {
          method: 'PUT',
          body: payload
        });
        await fetchAssets();
        actionSuccessMessage = `Asset "${updated.name}" updated successfully in database.`;
      }

      showAssetModal = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        const baseMsg = err.status === 403
          ? 'Forbidden: Insufficient permissions to modify assets for this treasury.'
          : err.message || 'Failed to persist asset to database.';
        assetModalErrors = err.details && err.details.length > 0 ? [baseMsg, ...err.details] : [baseMsg];
      } else if (err instanceof Error) {
        assetModalErrors = [err.message];
      } else {
        assetModalErrors = ['An unexpected error occurred while saving asset.'];
      }
    } finally {
      isSavingAsset = false;
    }
  }

  function openAddLiabilityModal() {
    modalLiabilityMode = 'create';
    liabilityForm = {
      id: `debt_${liabilities.length + 1}`,
      name: `Debt Facility ${liabilities.length + 1}`,
      principal: '10.0',
      interest_rate: '0.06',
      maturity_months: '12',
      priority: '1',
      is_mandatory_reserve: false
    };
    liabilityModalErrors = [];
    showLiabilityModal = true;
  }

  function openEditLiabilityModal(l: Liability) {
    modalLiabilityMode = 'edit';
    liabilityForm = {
      id: l.id,
      name: l.name,
      principal: String(l.principal),
      interest_rate: String(l.interest_rate),
      maturity_months: l.maturity_months !== undefined ? String(l.maturity_months) : '12',
      priority: String(l.priority),
      is_mandatory_reserve: Boolean(l.is_mandatory_reserve)
    };
    liabilityModalErrors = [];
    showLiabilityModal = true;
  }

  async function handleSaveLiability(e: SubmitEvent) {
    e.preventDefault();
    liabilityModalErrors = [];
    const errs: string[] = [];

    if (!liabilityForm.id.trim()) errs.push('Liability identifier (id) is required.');
    if (!liabilityForm.name.trim()) errs.push('Liability name is required.');

    const pr = parseFloat(liabilityForm.principal);
    if (isNaN(pr) || pr < 0) errs.push('Principal amount must be non-negative.');

    const rate = parseFloat(liabilityForm.interest_rate);
    if (isNaN(rate) || rate < 0) errs.push('Interest rate must be non-negative.');

    const mat = parseInt(liabilityForm.maturity_months, 10);
    if (isNaN(mat) || mat < 1) errs.push('Maturity months must be at least 1.');

    const pri = parseInt(liabilityForm.priority, 10);
    if (isNaN(pri) || pri < 0) errs.push('Priority must be a non-negative integer.');

    if (modalLiabilityMode === 'create' && liabilities.some((l) => l.id.toLowerCase() === liabilityForm.id.trim().toLowerCase())) {
      errs.push(`A liability with identifier '${liabilityForm.id.trim()}' already exists in this treasury.`);
    }

    if (errs.length > 0) {
      liabilityModalErrors = errs;
      return;
    }

    isSavingLiability = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;

    try {
      const payload: Liability = {
        id: liabilityForm.id.trim(),
        name: liabilityForm.name.trim(),
        principal: pr,
        interest_rate: rate,
        maturity_months: mat,
        priority: pri,
        is_mandatory_reserve: liabilityForm.is_mandatory_reserve
      };

      if (modalLiabilityMode === 'create') {
        const created = await api<Liability>(`/api/v1/treasuries/${treasuryId}/liabilities`, {
          method: 'POST',
          body: payload
        });
        await fetchLiabilities();
        actionSuccessMessage = `Liability "${created.name}" created and saved to portfolio.`;
      } else {
        const updated = await api<Liability>(`/api/v1/treasuries/${treasuryId}/liabilities/${encodeURIComponent(payload.id)}`, {
          method: 'PUT',
          body: payload
        });
        await fetchLiabilities();
        actionSuccessMessage = `Liability "${updated.name}" updated successfully in database.`;
      }

      showLiabilityModal = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        const baseMsg = err.status === 403
          ? 'Forbidden: Insufficient permissions to modify liabilities for this treasury.'
          : err.message || 'Failed to persist liability to database.';
        liabilityModalErrors = err.details && err.details.length > 0 ? [baseMsg, ...err.details] : [baseMsg];
      } else if (err instanceof Error) {
        liabilityModalErrors = [err.message];
      } else {
        liabilityModalErrors = ['An unexpected error occurred while saving liability.'];
      }
    } finally {
      isSavingLiability = false;
    }
  }

  function openDeleteConfirm(type: 'asset' | 'liability', id: string, name: string) {
    deleteTargetType = type;
    deleteTargetId = id;
    deleteTargetName = name;
    showDeleteModal = true;
  }

  async function executeDelete() {
    isDeleting = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;

    try {
      if (deleteTargetType === 'asset') {
        await api(`/api/v1/treasuries/${treasuryId}/assets/${encodeURIComponent(deleteTargetId)}`, {
          method: 'DELETE'
        });
        assets = assets.filter((a) => a.id !== deleteTargetId);
        actionSuccessMessage = `Asset "${deleteTargetName}" successfully removed from treasury database.`;
      } else {
        await api(`/api/v1/treasuries/${treasuryId}/liabilities/${encodeURIComponent(deleteTargetId)}`, {
          method: 'DELETE'
        });
        liabilities = liabilities.filter((l) => l.id !== deleteTargetId);
        actionSuccessMessage = `Liability "${deleteTargetName}" successfully removed from treasury database.`;
      }
      showDeleteModal = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        actionErrorMessage = err.message || `Failed to delete ${deleteTargetType}.`;
      } else {
        actionErrorMessage = `Failed to delete ${deleteTargetType}.`;
      }
    } finally {
      isDeleting = false;
    }
  }

  function openEditTreasuryModal() {
    if (!treasury) return;
    editTreasuryForm = {
      name: treasury.name,
      base_currency: treasury.base_currency || 'INR',
      total_capital: String(treasury.total_capital),
      risk_tolerance: String(treasury.risk_tolerance),
      min_liquidity_ratio: String(treasury.min_liquidity_ratio)
    };
    initialTreasuryForm = {
      name: treasury.name,
      base_currency: treasury.base_currency || 'INR',
      total_capital: String(treasury.total_capital),
      risk_tolerance: String(treasury.risk_tolerance),
      min_liquidity_ratio: String(treasury.min_liquidity_ratio)
    };
    editTreasuryErrors = [];
    showUnsavedPrompt = false;
    showEditTreasuryModal = true;
  }

  function handleCloseEditTreasury(force = false) {
    if (!force && isTreasuryFormDirty) {
      showUnsavedPrompt = true;
      return;
    }
    showUnsavedPrompt = false;
    showEditTreasuryModal = false;
    editTreasuryErrors = [];
  }

  async function handleSaveTreasury(e: SubmitEvent) {
    e.preventDefault();
    if (!treasury) return;
    editTreasuryErrors = [];

    const name = editTreasuryForm.name.trim();
    const currency = editTreasuryForm.base_currency.trim();
    const capital = parseFloat(editTreasuryForm.total_capital);
    const risk = parseFloat(editTreasuryForm.risk_tolerance);
    const liquidity = parseFloat(editTreasuryForm.min_liquidity_ratio);

    const errors: string[] = [];
    if (!name) {
      errors.push('Treasury name is required.');
    } else if (name.length > 255) {
      errors.push('Treasury name cannot exceed 255 characters.');
    }

    if (!currency) {
      errors.push('Base currency is required.');
    }

    if (isNaN(capital) || capital <= 0) {
      errors.push('Total capital base must be greater than zero.');
    }

    if (isNaN(risk) || risk < 0 || risk > 1) {
      errors.push('Risk tolerance must be a value between 0.0 and 1.0.');
    }

    if (isNaN(liquidity) || liquidity < 0 || liquidity > 1) {
      errors.push('Minimum liquidity ratio must be a value between 0.0 and 1.0.');
    }

    if (errors.length > 0) {
      editTreasuryErrors = errors;
      return;
    }

    // Build partial patch payload
    const patchPayload: UpdateTreasuryInput = {};
    if (name !== treasury.name) patchPayload.name = name;
    if (currency !== treasury.base_currency) patchPayload.base_currency = currency;
    if (capital !== treasury.total_capital) patchPayload.total_capital = capital;
    if (risk !== treasury.risk_tolerance) patchPayload.risk_tolerance = risk;
    if (liquidity !== treasury.min_liquidity_ratio) patchPayload.min_liquidity_ratio = liquidity;

    if (Object.keys(patchPayload).length === 0) {
      showEditTreasuryModal = false;
      return;
    }

    isSavingTreasury = true;
    try {
      const updated = await api<Treasury>(`/api/v1/treasuries/${treasury.id}`, {
        method: 'PATCH',
        body: patchPayload
      });
      treasury = updated;
      actionSuccessMessage = `Treasury "${updated.name}" updated successfully in database.`;
      actionErrorMessage = null;
      showEditTreasuryModal = false;
      showUnsavedPrompt = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        editTreasuryErrors = err.details && err.details.length > 0 ? err.details : [err.message];
      } else if (err instanceof Error) {
        editTreasuryErrors = [err.message];
      } else {
        editTreasuryErrors = ['Failed to update corporate treasury profile. Please try again.'];
      }
    } finally {
      isSavingTreasury = false;
    }
  }

  async function handleLoadAndPersistSampleAssets() {
    isLoadingAssets = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;
    try {
      const res = await api<TreasuryAssetsResponse>(`/api/v1/treasuries/${treasuryId}/assets`, {
        method: 'PUT',
        body: STARTER_ASSETS
      });
      assets = Array.isArray(res.data) ? res.data : [];
      actionSuccessMessage = 'Starter institutional asset universe committed to database.';
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        actionErrorMessage = err.message || 'Failed to save starter assets.';
      } else {
        actionErrorMessage = 'Failed to load starter assets.';
      }
    } finally {
      isLoadingAssets = false;
    }
  }

  async function handleLoadAndPersistSampleLiabilities() {
    isLoadingLiabilities = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;
    try {
      const res = await api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${treasuryId}/liabilities`, {
        method: 'PUT',
        body: STARTER_LIABILITIES
      });
      liabilities = Array.isArray(res.data) ? res.data : [];
      actionSuccessMessage = 'Starter liability facility committed to database.';
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        actionErrorMessage = err.message || 'Failed to save starter liabilities.';
      } else {
        actionErrorMessage = 'Failed to load starter liabilities.';
      }
    } finally {
      isLoadingLiabilities = false;
    }
  }

  function addAsset() {
    assets = [
      ...assets,
      {
        id: `asset_${assets.length + 1}`,
        name: `Asset ${assets.length + 1}`,
        asset_type: 'gov_bond',
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

  function validateOptionalStepSize(raw: string): string | null {
    if (!raw.trim()) return null;
    const step = parseFloat(raw);
    if (isNaN(step) || step <= 0) {
      return 'Optional step size must be a positive number if provided.';
    }
    if (treasury && treasury.total_capital / step > 200) {
      const minStep = (treasury.total_capital / 200).toFixed(2);
      return `Step size ${step} creates too many steps (total_capital / step_size > 200). Use at least ${minStep} or leave blank for auto (capital / 20).`;
    }
    return null;
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

    const stepErr = validateOptionalStepSize(stepSize);
    if (stepErr) {
      errs.push(stepErr);
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
          generationError = 'AI Decision Engine is currently offline or unreachable.';
        } else if (err.status === 403) {
          generationError = 'Forbidden: Access to orchestrate this treasury is restricted to authorized analysts and administrators.';
        } else {
          generationError = err.message || 'Strategy orchestration failed.';
        }
        if (err.details && err.details.length > 0) {
          const minStepHint = treasury ? ` Minimum recommended step_size: ${(treasury.total_capital / 200).toFixed(2)}.` : '';
          generationDetails = err.details.map((d) => (d.includes('step_size') && minStepHint ? `${d}.${minStepHint}` : d));
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

  async function handleEvaluateCouncil() {
    if (!treasury || isEvaluatingCouncil) return;
    councilError = null;
    councilDetails = [];

    if (assets.length === 0) {
      councilError = 'At least one investment asset must be configured in the asset universe.';
      return;
    }

    const councilStepErr = validateOptionalStepSize(councilStepSize);
    if (councilStepErr) {
      councilError = councilStepErr;
      return;
    }

    isEvaluatingCouncil = true;

    try {
      const thresholdVal = calculateLiquidityFloor(minLiquidity, treasury.min_liquidity_ratio);

      const payload: EvaluateCouncilInput = {
        objective: councilObjective,
        market_state: councilMarketState,
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
        ]
      };

      if (councilStepSize.trim()) {
        const parsed = parseFloat(councilStepSize);
        if (!isNaN(parsed) && parsed > 0) {
          payload.step_size = parsed;
        }
      }

      const res = await api<CouncilDeliberationSummary>(
        `/api/v1/treasuries/${treasuryId}/council/evaluate`,
        {
          method: 'POST',
          body: payload
        }
      );

      councilResult = res;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 502) {
          councilError = 'AI Engine offline: Multi-Agent Council deliberation service is currently unavailable.';
        } else if (err.status === 403) {
          councilError = 'Forbidden: Access to convene the council for this treasury is restricted to its owner and administrators.';
        } else {
          councilError = err.message || 'Multi-agent council evaluation failed.';
        }
        if (err.details && err.details.length > 0) {
          councilDetails = err.details;
        }
      } else if (err instanceof Error) {
        councilError = err.message;
      } else {
        councilError = 'An unexpected error occurred during council deliberation.';
      }
    } finally {
      isEvaluatingCouncil = false;
    }
  }

  onMount(() => {
    if (treasuryId) {
      fetchTreasuryDetails();
      fetchStrategies(0);
      fetchAssets();
      fetchLiabilities();
    }
  });
</script>
<svelte:head>
  <title>{treasury ? `${treasury.name} — QUANTA` : 'Portfolio — QUANTA'}</title>
</svelte:head>

<div class="page-shell">
  <div class="nav-row">
    <a href="/treasuries" class="back-link">&larr; Back to treasuries</a>
  </div>

  {#if isLoadingTreasury}
    <div class="empty-box">Loading treasury portfolio...</div>
  {:else if isNotFound}
    <div class="empty-box">
      <h2>Treasury Not Found</h2>
      <p>The portfolio "{treasuryId}" could not be found.</p>
      <a href="/treasuries" class="btn btn-primary btn-sm" style="margin-top: 1rem;">Return to treasuries</a>
    </div>
  {:else if treasuryError}
    <div class="alert alert-danger row-between">
      <span>{treasuryError}</span>
      <button type="button" class="btn btn-secondary btn-xs" onclick={fetchTreasuryDetails}>Retry</button>
    </div>
  {:else if treasury}
    <header class="page-head" data-enter>
      <div>
        <h1>{treasury.name}</h1>
        <p>
          <span class="font-mono">{treasury.base_currency}</span> &middot;
          ID <code class="font-mono">{treasury.id}</code> &middot;
          Updated {formatDate(treasury.updated_at)}
        </p>
      </div>

      <div class="page-head-actions">
        {#if canWrite($user?.role)}
          <button type="button" class="btn btn-secondary btn-sm" onclick={openEditTreasuryModal}>
            Edit treasury
          </button>
          <button
            type="button"
            class="btn btn-primary btn-sm"
            onclick={() => {
              activeTab = 'strategy';
              showGenerator = !showGenerator;
            }}
          >
            {showGenerator && activeTab === 'strategy' ? 'Close generator' : 'Generate strategy'}
          </button>
        {/if}
      </div>
    </header>

    <!-- Unboxed 5-column Balance Sheet & Policy Summary Strip -->
    <section class="summary-strip cols-5" aria-label="Portfolio balance sheet summary">
      <div class="summary-cell">
        <span class="summary-label">Capital Base</span>
        <span class="summary-value">{formatCurrency(totalCapital, treasury.base_currency)}</span>
        <span class="summary-meta">{treasury.base_currency} &middot; {formatPercent(treasury.risk_tolerance)} risk limit</span>
      </div>

      <div class="summary-cell">
        <span class="summary-label">Debt Obligations</span>
        <span class="summary-value">{formatCurrency(totalLiabilitiesPrincipal, treasury.base_currency)}</span>
        <span class="summary-meta">{activeLiabilityCount} facilities</span>
      </div>

      <div class="summary-cell">
        <span class="summary-label">Net Position</span>
        <span class="summary-value {netCapitalPosition >= 0 ? 'pos-val' : 'neg-val'}">
          {formatCurrency(netCapitalPosition, treasury.base_currency)}
        </span>
        <span class="summary-meta">{netCapitalPosition >= 0 ? 'Net surplus' : 'Capital deficit'}</span>
      </div>

      <div class="summary-cell">
        <span class="summary-label">Mandatory Reserves</span>
        <span class="summary-value">{formatCurrency(mandatoryDebtReserves, treasury.base_currency)}</span>
        <span class="summary-meta">Floor {formatPercent(treasury.min_liquidity_ratio)}</span>
      </div>

      <div class="summary-cell">
        <span class="summary-label">Asset Universe</span>
        <span class="summary-value">{activeAssetCount}</span>
        <span class="summary-meta">Persisted candidate assets</span>
      </div>
    </section>

    {#if actionSuccessMessage}
      <div class="alert alert-success row-between">
        <span>{actionSuccessMessage}</span>
        <button type="button" class="btn btn-secondary btn-xs" onclick={() => (actionSuccessMessage = null)}>Dismiss</button>
      </div>
    {/if}

    {#if actionErrorMessage}
      <div class="alert alert-danger row-between">
        <span>{actionErrorMessage}</span>
        <button type="button" class="btn btn-secondary btn-xs" onclick={() => (actionErrorMessage = null)}>Dismiss</button>
      </div>
    {/if}

    {#if !canWrite($user?.role)}
      <div class="notice-banner">
        Read-only access: your role ({$user?.role || 'viewer'}) permits inspection only. Analyst or Admin access is required to edit holdings or run allocations.
      </div>
    {/if}

    {#if !isLoadingAssets && !assetsError && assets.length === 0}
      <div class="alert alert-warning row-between" style="flex-wrap: wrap;">
        <div>
          <strong>This treasury has no persisted assets yet.</strong>
          <div style="font-size: 0.8125rem; margin-top: 0.2rem;">
            Add investment assets or commit a starter institutional portfolio to PostgreSQL so allocations and risk models use real data.
          </div>
        </div>
        {#if canWrite($user?.role)}
          <div class="btn-row">
            <button
              type="button"
              class="btn btn-primary btn-sm"
              onclick={saveStarterPortfolioToDatabase}
              disabled={isSavingStarter || isLoadingAssets || isLoadingLiabilities}
            >
              {isSavingStarter ? 'Saving...' : 'Save starter portfolio to DB'}
            </button>
            <button
              type="button"
              class="btn btn-secondary btn-sm"
              onclick={openAddAssetModal}
              disabled={isSavingStarter}
            >
              Add asset
            </button>
          </div>
        {/if}
      </div>
    {/if}

    <!-- Clean Underline Tab Navigation -->
    <nav class="tab-bar" aria-label="Portfolio sections">
      <button type="button" class="tab-link-btn" class:active={activeTab === 'balance-sheet'} onclick={() => (activeTab = 'balance-sheet')}>
        Balance Sheet
      </button>
      <button type="button" class="tab-link-btn" class:active={activeTab === 'strategy'} onclick={() => (activeTab = 'strategy')}>
        Capital Allocation
      </button>
      <button type="button" class="tab-link-btn" class:active={activeTab === 'risk'} onclick={() => (activeTab = 'risk')}>
        Risk Analysis &amp; VaR
      </button>
      <button type="button" class="tab-link-btn" class:active={activeTab === 'stress'} onclick={() => (activeTab = 'stress')}>
        Stress Testing
      </button>
      <button type="button" class="tab-link-btn" class:active={activeTab === 'council'} onclick={() => (activeTab = 'council')}>
        Executive Council
      </button>
    </nav>

    <!-- TAB 0: BALANCE SHEET -->
    {#if activeTab === 'balance-sheet'}
      <section class="section-block" aria-label="Investment asset universe">
        <div class="block-head">
          <div>
            <h2>Investment Assets ({activeAssetCount})</h2>
            <p class="block-sub">Candidate financial instruments evaluated in allocation runs.</p>
          </div>
          {#if canWrite($user?.role)}
            <div class="btn-row">
              <button type="button" class="btn btn-secondary btn-sm" onclick={handleLoadAndPersistSampleAssets} disabled={isLoadingAssets}>
                Reset sample assets
              </button>
              <button type="button" class="btn btn-primary btn-sm" onclick={openAddAssetModal} disabled={isLoadingAssets}>
                Add asset
              </button>
            </div>
          {/if}
        </div>

        {#if assetsError}
          <div class="alert alert-danger row-between">
            <span>{assetsError}</span>
            <button type="button" class="btn btn-secondary btn-xs" onclick={fetchAssets}>Retry</button>
          </div>
        {/if}

        {#if isLoadingAssets}
          <div class="empty-box">Loading assets...</div>
        {:else if assets.length === 0}
          <div class="empty-box">
            <p>No investment assets recorded in PostgreSQL.</p>
            {#if canWrite($user?.role)}
              <div class="btn-row" style="justify-content: center; margin-top: 0.875rem;">
                <button type="button" class="btn btn-primary btn-sm" onclick={openAddAssetModal}>Add asset</button>
                <button type="button" class="btn btn-secondary btn-sm" onclick={saveStarterPortfolioToDatabase} disabled={isSavingStarter}>
                  {isSavingStarter ? 'Saving...' : 'Save starter portfolio'}
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Name</th>
                  <th>Class</th>
                  <th class="text-right">Exp. Return</th>
                  <th class="text-right">Risk</th>
                  <th class="text-right">Liquidity</th>
                  <th class="text-right">Cost</th>
                  <th class="text-right">Volatility</th>
                  {#if canWrite($user?.role)}
                    <th class="text-right">Actions</th>
                  {/if}
                </tr>
              </thead>
              <tbody>
                {#each assets as asset (asset.id)}
                  <tr>
                    <td class="font-mono muted-cell">{asset.id}</td>
                    <td class="font-bold">{asset.name}</td>
                    <td class="class-cell">{asset.asset_type.replaceAll('_', ' ')}</td>
                    <td class="text-right numeric-cell">{formatPercent(asset.expected_return)}</td>
                    <td class="text-right numeric-cell">{formatNumber(asset.risk)}</td>
                    <td class="text-right numeric-cell">{formatNumber(asset.liquidity)}</td>
                    <td class="text-right numeric-cell">{formatNumber(asset.cost)}</td>
                    <td class="text-right numeric-cell">{asset.volatility !== undefined ? formatPercent(asset.volatility) : '—'}</td>
                    {#if canWrite($user?.role)}
                      <td class="text-right">
                        <div class="btn-row" style="justify-content: flex-end;">
                          <button type="button" class="btn btn-secondary btn-xs" onclick={() => openEditAssetModal(asset)}>Edit</button>
                          <button type="button" class="btn btn-danger btn-xs" onclick={() => openDeleteConfirm('asset', asset.id, asset.name)}>Delete</button>
                        </div>
                      </td>
                    {/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>

      <section class="section-block" aria-label="Debt obligations">
        <div class="block-head">
          <div>
            <h2>Debt Obligations ({activeLiabilityCount})</h2>
            <p class="block-sub">Contractual liabilities and mandatory liquidity reserve commitments.</p>
          </div>
          {#if canWrite($user?.role)}
            <div class="btn-row">
              <button type="button" class="btn btn-secondary btn-sm" onclick={handleLoadAndPersistSampleLiabilities} disabled={isLoadingLiabilities}>
                Reset sample debt
              </button>
              <button type="button" class="btn btn-primary btn-sm" onclick={openAddLiabilityModal} disabled={isLoadingLiabilities}>
                Add liability
              </button>
            </div>
          {/if}
        </div>

        {#if liabilitiesError}
          <div class="alert alert-danger row-between">
            <span>{liabilitiesError}</span>
            <button type="button" class="btn btn-secondary btn-xs" onclick={fetchLiabilities}>Retry</button>
          </div>
        {/if}

        {#if isLoadingLiabilities}
          <div class="empty-box">Loading liabilities...</div>
        {:else if liabilities.length === 0}
          <div class="empty-box">
            <p>No debt obligations recorded. Full capital base is unencumbered.</p>
            {#if canWrite($user?.role)}
              <div class="btn-row" style="justify-content: center; margin-top: 0.875rem;">
                <button type="button" class="btn btn-primary btn-sm" onclick={openAddLiabilityModal}>Add debt obligation</button>
                <button type="button" class="btn btn-secondary btn-sm" onclick={handleLoadAndPersistSampleLiabilities}>Load sample debt</button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Facility</th>
                  <th class="text-right">Principal</th>
                  <th class="text-right">Interest</th>
                  <th class="text-right">Maturity</th>
                  <th class="text-right">Priority</th>
                  <th>Reserve Status</th>
                  {#if canWrite($user?.role)}
                    <th class="text-right">Actions</th>
                  {/if}
                </tr>
              </thead>
              <tbody>
                {#each liabilities as liab (liab.id)}
                  <tr>
                    <td class="font-mono muted-cell">{liab.id}</td>
                    <td class="font-bold">{liab.name}</td>
                    <td class="text-right numeric-cell">{formatCurrency(liab.principal, treasury.base_currency)}</td>
                    <td class="text-right numeric-cell">{formatPercent(liab.interest_rate)}</td>
                    <td class="text-right numeric-cell">{liab.maturity_months ?? 12} mo</td>
                    <td class="text-right numeric-cell">P{liab.priority}</td>
                    <td class={liab.is_mandatory_reserve ? 'pos-val' : 'muted-cell'}>
                      {liab.is_mandatory_reserve ? 'Mandatory reserve' : 'Discretionary'}
                    </td>
                    {#if canWrite($user?.role)}
                      <td class="text-right">
                        <div class="btn-row" style="justify-content: flex-end;">
                          <button type="button" class="btn btn-secondary btn-xs" onclick={() => openEditLiabilityModal(liab)}>Edit</button>
                          <button type="button" class="btn btn-danger btn-xs" onclick={() => openDeleteConfirm('liability', liab.id, liab.name)}>Delete</button>
                        </div>
                      </td>
                    {/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </section>
    {/if}

    <!-- TAB 1: CAPITAL ALLOCATION STRATEGY -->
    {#if activeTab === 'strategy'}
      {#if showGenerator && canWrite($user?.role)}
        <div class="surface-panel stack-gap">
          <div>
            <h2>Run Strategy Search</h2>
            <p class="block-sub">Configure search algorithm, step size, and liquidity threshold.</p>
          </div>

          {#if generationError}
            <div class="alert alert-danger">
              <strong>{generationError}</strong>
              {#if generationDetails.length > 0}
                <ul class="err-list">
                  {#each generationDetails as d}<li>{d}</li>{/each}
                </ul>
              {/if}
            </div>
          {/if}

          {#if fieldValidationErrors.length > 0}
            <div class="alert alert-danger">
              <strong>Fix input errors:</strong>
              <ul class="err-list">
                {#each fieldValidationErrors as err}<li>{err}</li>{/each}
              </ul>
            </div>
          {/if}

          <form onsubmit={handleGenerateStrategy} novalidate class="stack-gap">
            <div class="settings-grid">
              <div class="form-group" style="margin-bottom: 0;">
                <label for="algorithmSelect" class="form-label">Search algorithm</label>
                <select id="algorithmSelect" class="form-input" bind:value={selectedAlgorithm} disabled={isGenerating}>
                  {#each algorithmOptions as opt}
                    <option value={opt.value}>{opt.label}</option>
                  {/each}
                </select>
              </div>

              <div class="form-group" style="margin-bottom: 0;">
                <label for="stepSizeInput" class="form-label">Step size (optional)</label>
                <input
                  id="stepSizeInput"
                  type="number"
                  step="any"
                  min="0.01"
                  class="form-input font-mono"
                  placeholder={treasury ? `Auto (${(treasury.total_capital / 20).toFixed(2)})` : 'Auto'}
                  bind:value={stepSize}
                  disabled={isGenerating}
                />
                {#if treasury}
                  <span class="field-hint">Min {(treasury.total_capital / 200).toFixed(2)} to keep steps &le; 200.</span>
                {/if}
              </div>

              <div class="form-group" style="margin-bottom: 0;">
                <label for="minLiqInput" class="form-label">Liquidity floor (0.00–1.00)</label>
                <input id="minLiqInput" type="number" step="0.05" min="0" max="1" class="form-input font-mono" bind:value={minLiquidity} disabled={isGenerating} />
              </div>
            </div>

            <label class="checkbox-label">
              <input type="checkbox" bind:checked={includeComparison} disabled={isGenerating} />
              <span>Benchmark all 6 search algorithms side by side</span>
            </label>

            <div class="editor-block">
              <div class="block-head" style="margin-bottom: 0.5rem;">
                <h3>Assets ({assets.length})</h3>
                <div class="btn-row">
                  <button type="button" class="btn btn-secondary btn-xs" onclick={saveStarterPortfolioToDatabase} disabled={isGenerating || isSavingStarter}>
                    {isSavingStarter ? 'Saving...' : 'Save starter portfolio'}
                  </button>
                  <button type="button" class="btn btn-secondary btn-xs" onclick={addAsset} disabled={isGenerating}>Add row</button>
                </div>
              </div>
              <div class="table-container">
                <table class="table editor-table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Name</th>
                      <th>Type</th>
                      <th>Exp. Return</th>
                      <th>Risk</th>
                      <th>Liquidity</th>
                      <th>Cost</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each assets as asset, idx (idx)}
                      <tr>
                        <td><input type="text" class="form-input editor-input font-mono" bind:value={asset.id} disabled={isGenerating} /></td>
                        <td><input type="text" class="form-input editor-input" bind:value={asset.name} disabled={isGenerating} /></td>
                        <td>
                          <select class="form-input editor-input" bind:value={asset.asset_type} disabled={isGenerating}>
                            {#each CANONICAL_ASSET_TYPES as opt}<option value={opt.value}>{opt.value}</option>{/each}
                          </select>
                        </td>
                        <td><input type="number" step="0.01" min="0" class="form-input editor-input font-mono" bind:value={asset.expected_return} disabled={isGenerating} /></td>
                        <td><input type="number" step="0.01" min="0" max="1" class="form-input editor-input font-mono" bind:value={asset.risk} disabled={isGenerating} /></td>
                        <td><input type="number" step="0.01" min="0" max="1" class="form-input editor-input font-mono" bind:value={asset.liquidity} disabled={isGenerating} /></td>
                        <td><input type="number" step="0.001" min="0" class="form-input editor-input font-mono" bind:value={asset.cost} disabled={isGenerating} /></td>
                        <td>
                          <button type="button" class="btn btn-danger btn-xs" onclick={() => removeAsset(idx)} disabled={isGenerating || assets.length <= 1}>Remove</button>
                        </td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>

            <div class="editor-block">
              <div class="block-head" style="margin-bottom: 0.5rem;">
                <h3>Liabilities ({liabilities.length})</h3>
                <div class="btn-row">
                  <button type="button" class="btn btn-secondary btn-xs" onclick={handleLoadAndPersistSampleLiabilities} disabled={isGenerating || isLoadingLiabilities}>Reset sample debt</button>
                  <button type="button" class="btn btn-secondary btn-xs" onclick={addLiability} disabled={isGenerating}>Add row</button>
                </div>
              </div>
              <div class="table-container">
                <table class="table editor-table">
                  <thead>
                    <tr>
                      <th>ID</th>
                      <th>Name</th>
                      <th>Principal</th>
                      <th>Rate</th>
                      <th>Priority</th>
                      <th>Mandatory</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {#each liabilities as liab, idx (idx)}
                      <tr>
                        <td><input type="text" class="form-input editor-input font-mono" bind:value={liab.id} disabled={isGenerating} /></td>
                        <td><input type="text" class="form-input editor-input" bind:value={liab.name} disabled={isGenerating} /></td>
                        <td><input type="number" step="any" min="0" class="form-input editor-input font-mono" bind:value={liab.principal} disabled={isGenerating} /></td>
                        <td><input type="number" step="0.005" min="0" class="form-input editor-input font-mono" bind:value={liab.interest_rate} disabled={isGenerating} /></td>
                        <td><input type="number" min="1" class="form-input editor-input font-mono" bind:value={liab.priority} disabled={isGenerating} /></td>
                        <td class="text-center"><input type="checkbox" bind:checked={liab.is_mandatory_reserve} disabled={isGenerating} /></td>
                        <td><button type="button" class="btn btn-danger btn-xs" onclick={() => removeLiability(idx)} disabled={isGenerating}>Remove</button></td>
                      </tr>
                    {/each}
                  </tbody>
                </table>
              </div>
            </div>

            <div class="btn-row">
              <button type="submit" class="btn btn-primary btn-sm" disabled={isGenerating}>
                {isGenerating ? 'Executing orchestration...' : 'Execute allocation'}
              </button>
              <button type="button" class="btn btn-secondary btn-sm" disabled={isGenerating} onclick={() => (showGenerator = false)}>Cancel</button>
            </div>
          </form>
        </div>
      {/if}

      {#if latestResult}
        <section class="section-block" aria-label="Latest synthesized strategy">
          <div class="block-head">
            <div>
              <h2>Latest Strategy ({latestResult.ai.algorithm_used})</h2>
              <p class="block-sub font-mono">Run {latestResult.run_id}</p>
            </div>
            <span class="pos-val">Completed</span>
          </div>

          <div class="summary-strip cols-5">
            <div class="summary-cell"><span class="summary-label">Objective</span><span class="summary-value">{formatNumber(latestResult.ai.objective_value)}</span></div>
            <div class="summary-cell"><span class="summary-label">Expected Return</span><span class="summary-value pos-val">{formatPercent(latestResult.ai.expected_return)}</span></div>
            <div class="summary-cell"><span class="summary-label">Risk Score</span><span class="summary-value">{formatNumber(latestResult.ai.risk_score)}</span></div>
            <div class="summary-cell"><span class="summary-label">Liquidity</span><span class="summary-value">{formatPercent(latestResult.ai.liquidity_ratio)}</span></div>
            <div class="summary-cell"><span class="summary-label">Runtime</span><span class="summary-value">{latestResult.ai.execution_time_ms.toFixed(1)} ms</span></div>
          </div>

          {#if latestResult.ai.explanation}
            <div class="notice-banner">{latestResult.ai.explanation}</div>
          {/if}

          <div class="chart-container">
            <h3 style="margin-bottom: 0.75rem;">Capital Allocation Breakdown</h3>
            {#each parseAllocations(latestResult.ai.allocations_json) as item}
              {@const pct = totalAllocationAmount > 0 ? (item.amount / totalAllocationAmount) * 100 : 0}
              <div class="bar-row">
                <span class="bar-label" title={item.asset}>{item.asset}</span>
                <div class="bar-track"><div class="bar-fill" style="width: {Math.min(100, Math.max(2, pct))}%;"></div></div>
                <span class="bar-value font-mono">{item.formattedAmount} ({pct.toFixed(1)}%)</span>
              </div>
            {:else}
              <p class="muted-cell">No discrete allocations returned.</p>
            {/each}
          </div>

          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Constraint</th>
                  <th class="text-right">Required</th>
                  <th class="text-right">Achieved</th>
                  <th>Status</th>
                  <th>Note</th>
                </tr>
              </thead>
              <tbody>
                {#each parseConstraintsAudit(latestResult.ai.constraints_audit_json) as audit}
                  <tr>
                    <td class="font-mono">{audit.constraint_type}</td>
                    <td class="text-right numeric-cell">{formatNumber(audit.required)}</td>
                    <td class="text-right numeric-cell">{formatNumber(audit.actual)}</td>
                    <td class={audit.satisfied ? 'pos-val' : 'neg-val'}>{audit.satisfied ? 'Pass' : 'Fail'}</td>
                    <td class="muted-cell">{audit.message || '—'}</td>
                  </tr>
                {:else}
                  <tr><td colspan="5" class="text-center muted-cell">No constraints logged.</td></tr>
                {/each}
              </tbody>
            </table>
          </div>

          {#if latestResult.comparison?.results}
            <div class="table-container">
              <table class="table">
                <thead>
                  <tr>
                    <th>Algorithm</th>
                    <th class="text-right">Objective</th>
                    <th class="text-right">Exp. Return</th>
                    <th class="text-right">Risk</th>
                    <th class="text-right">Liquidity</th>
                    <th class="text-right">States</th>
                    <th class="text-right">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {#each comparisonResultsList as [algoKey, compResult]}
                    {@const isBest = algoKey === bestComparisonAlgo}
                    <tr class:highlight-row={isBest}>
                      <td class="font-mono font-bold">{algoKey}{isBest ? ' (best)' : ''}</td>
                      <td class="text-right numeric-cell">{formatNumber(compResult.objective_value)}</td>
                      <td class="text-right numeric-cell">{formatPercent(compResult.expected_return)}</td>
                      <td class="text-right numeric-cell">{formatNumber(compResult.risk_score)}</td>
                      <td class="text-right numeric-cell">{formatPercent(compResult.liquidity_ratio)}</td>
                      <td class="text-right numeric-cell">{compResult.states_explored}</td>
                      <td class="text-right numeric-cell">{compResult.execution_time_ms.toFixed(1)} ms</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>

            <div class="chart-container">
              <h3 style="margin-bottom: 0.75rem;">Objective Value Comparison</h3>
              {#each comparisonResultsList as [algoKey, compResult]}
                {@const isBest = algoKey === bestComparisonAlgo}
                {@const barWidth = bestComparisonObjective > 0 ? (compResult.objective_value / bestComparisonObjective) * 100 : 0}
                <div class="bar-row">
                  <span class="bar-label font-mono">{algoKey}</span>
                  <div class="bar-track"><div class="bar-fill" class:bar-fill-highlight={isBest} style="width: {Math.min(100, Math.max(2, barWidth))}%;"></div></div>
                  <span class="bar-value font-mono">{formatNumber(compResult.objective_value)}</span>
                </div>
              {/each}
            </div>
          {/if}

          <div class="notice-banner">
            <strong>Optimizer (:8001):</strong>
            {#if latestResult.optimization.status === 'unavailable' || latestResult.optimization.status === 'not_available'}
              Offline &mdash; {latestResult.optimization.reason}
            {:else}
              {latestResult.optimization.status} &mdash; {latestResult.optimization.reason}
              {#if latestResult.optimization.best_classical_method}
                &middot; Best classical: {latestResult.optimization.best_classical_method}
              {/if}
              {#if latestResult.optimization.qaoa_approximation_ratio != null}
                &middot; QAOA ratio: {(latestResult.optimization.qaoa_approximation_ratio * 100).toFixed(1)}%
              {/if}
            {/if}
          </div>
        </section>
      {/if}

      <section class="section-block" aria-label="Saved execution strategies">
        <div class="block-head">
          <div>
            <h2>Saved Strategies</h2>
            <p class="block-sub">Historical algorithm outputs persisted for this treasury.</p>
          </div>
          {#if canWrite($user?.role)}
            <button type="button" class="btn btn-primary btn-sm" onclick={() => (showGenerator = !showGenerator)}>
              {showGenerator ? 'Close generator' : 'New strategy run'}
            </button>
          {/if}
        </div>

        {#if strategiesError}
          <div class="alert alert-danger row-between">
            <span>{strategiesError}</span>
            <button type="button" class="btn btn-secondary btn-xs" onclick={() => fetchStrategies(strategyOffset)}>Retry</button>
          </div>
        {/if}

        {#if isLoadingStrategies}
          <div class="empty-box">Loading saved strategies...</div>
        {:else if strategies.length === 0}
          <div class="empty-box">
            {strategyOffset > 0 ? 'No further strategy records on this page.' : 'No optimization runs recorded for this treasury yet.'}
          </div>
        {:else}
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Algorithm</th>
                  <th class="text-right">Objective</th>
                  <th class="text-right">Risk</th>
                  <th class="text-right">Exp. Return</th>
                  <th class="text-right">Liquidity</th>
                  <th class="text-right">Time</th>
                  <th>Allocations</th>
                  <th class="text-right">Created</th>
                  <th class="text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                {#each strategies as s (s.id)}
                  <tr>
                    <td class="font-mono font-bold">{s.algorithm_used}</td>
                    <td class="text-right numeric-cell">{formatNumber(s.objective_value)}</td>
                    <td class="text-right numeric-cell">{formatNumber(s.risk_score)}</td>
                    <td class="text-right numeric-cell">{formatPercent(s.expected_return)}</td>
                    <td class="text-right numeric-cell">{formatPercent(s.liquidity_ratio)}</td>
                    <td class="text-right numeric-cell">{s.execution_time_ms ? `${s.execution_time_ms.toFixed(1)} ms` : '—'}</td>
                    <td class="allocations-cell">
                      {#each parseAllocations(s.allocations_json) as item}
                        <span class="alloc-line font-mono">{item.asset}: {item.formattedAmount}</span>
                      {:else}
                        <span class="muted-cell">—</span>
                      {/each}
                    </td>
                    <td class="text-right muted-cell">{formatDate(s.created_at)}</td>
                    <td class="text-right">
                      <button
                        type="button"
                        class="btn btn-secondary btn-xs"
                        onclick={() => {
                          selectStrategyForAnalysis(s);
                          activeTab = 'risk';
                        }}
                      >
                        Analyze risk
                      </button>
                    </td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>

          <div class="pagination">
            <span class="pagination-info">Showing {strategyOffset + 1} to {strategyOffset + strategies.length}</span>
            <div class="pagination-actions">
              <button type="button" class="btn btn-secondary btn-sm" disabled={strategyOffset === 0 || isLoadingStrategies} onclick={() => fetchStrategies(Math.max(0, strategyOffset - strategyLimit))}>Previous</button>
              <button type="button" class="btn btn-secondary btn-sm" disabled={strategies.length < strategyLimit || isLoadingStrategies} onclick={() => fetchStrategies(strategyOffset + strategyLimit)}>Next</button>
            </div>
          </div>
        {/if}
      </section>
    {/if}

    <!-- TAB 2: AI RISK ANALYSIS & MONTE CARLO VaR -->
    {#if activeTab === 'risk'}
      <div class="notice-banner row-between" style="flex-wrap: wrap;">
        <div>
          <strong>Target scope:</strong>
          {#if hasActiveAllocationStrategy}
            Strategy <code class="font-mono">{activeStrategyAlgorithm}</code> ({activeStrategyRunId}) &mdash; {allocatedAssetCount} assets, {formatCurrency(totalAllocationAmount, treasury.base_currency)}.
          {:else}
            Baseline cash position ({formatCurrency(treasury.total_capital, treasury.base_currency)}) against registered assets and mandatory debt reserves.
          {/if}
        </div>
        {#if hasActiveAllocationStrategy && selectedSavedStrategy}
          <button type="button" class="btn btn-secondary btn-xs" onclick={clearSelectedStrategy}>Use baseline</button>
        {/if}
      </div>

      <div class="surface-panel stack-gap">
        <div class="block-head">
          <div>
            <h2>Bayesian Network &amp; Monte Carlo VaR</h2>
            <p class="block-sub">Condition macro signals to compute posterior probabilities and tail loss bounds.</p>
          </div>
          <button type="button" class="btn btn-secondary btn-xs" onclick={resetBayesianEvidence} disabled={isAnalyzingRisk}>Reset prior</button>
        </div>

        {#if riskError}
          <div class="alert alert-danger">
            <strong>{riskError}</strong>
            {#if riskDetails.length > 0}
              <ul class="err-list">{#each riskDetails as d}<li>{d}</li>{/each}</ul>
            {/if}
          </div>
        {/if}

        <div class="settings-grid">
          <div class="form-group" style="margin-bottom: 0;">
            <label for="inflationSignalSelect" class="form-label">High inflation</label>
            <select id="inflationSignalSelect" class="form-input" bind:value={signalInflation} disabled={isAnalyzingRisk}>
              <option value="unobserved">Unobserved (prior)</option>
              <option value="true">Observed true</option>
              <option value="false">Observed false</option>
            </select>
          </div>
          <div class="form-group" style="margin-bottom: 0;">
            <label for="rateHikeSignalSelect" class="form-label">Central bank rate hike</label>
            <select id="rateHikeSignalSelect" class="form-input" bind:value={signalRateHike} disabled={isAnalyzingRisk}>
              <option value="unobserved">Unobserved (prior)</option>
              <option value="true">Observed true</option>
              <option value="false">Observed false</option>
            </select>
          </div>
          <div class="form-group" style="margin-bottom: 0;">
            <label for="recessionSignalSelect" class="form-label">Recession inversion</label>
            <select id="recessionSignalSelect" class="form-input" bind:value={signalRecession} disabled={isAnalyzingRisk}>
              <option value="unobserved">Unobserved (prior)</option>
              <option value="true">Observed true</option>
              <option value="false">Observed false</option>
            </select>
          </div>
        </div>

        <div class="btn-row" style="justify-content: space-between; flex-wrap: wrap;">
          <div class="btn-row" style="gap: 1rem;">
            <label class="checkbox-label">
              <input type="checkbox" bind:checked={runMonteCarlo} disabled={isAnalyzingRisk} />
              <span>Run Monte Carlo simulation</span>
            </label>
            {#if runMonteCarlo}
              <select aria-label="Scenarios" class="form-select compact-select font-mono" bind:value={mcScenarios} disabled={isAnalyzingRisk}>
                <option value="500">500 scenarios</option>
                <option value="1000">1,000 scenarios</option>
                <option value="2500">2,500 scenarios</option>
                <option value="5000">5,000 scenarios</option>
              </select>
            {/if}
          </div>

          <button type="button" class="btn btn-primary btn-sm" disabled={isAnalyzingRisk || !canWrite($user?.role)} onclick={handleAnalyzeRisk}>
            {isAnalyzingRisk ? 'Running analysis...' : 'Run risk analysis & VaR'}
          </button>
        </div>
      </div>

      {#if riskResult}
        <section class="section-block" aria-label="Risk analysis output">
          <div class="block-head">
            <h2>Portfolio Risk Breakdown ({riskResult.risk_breakdown.risk_tier})</h2>
            <span class="font-mono font-bold">Composite {formatNumber(riskResult.risk_breakdown.composite_risk_score)}</span>
          </div>

          <div class="summary-strip cols-5">
            <div class="summary-cell"><span class="summary-label">Asset Risk</span><span class="summary-value">{formatNumber(riskResult.risk_breakdown.weighted_asset_risk)}</span></div>
            <div class="summary-cell"><span class="summary-label">HHI Index</span><span class="summary-value">{formatNumber(riskResult.risk_breakdown.concentration_index_hhi)}</span></div>
            <div class="summary-cell"><span class="summary-label">Concentration Pen.</span><span class="summary-value">{formatNumber(riskResult.risk_breakdown.concentration_penalty)}</span></div>
            <div class="summary-cell"><span class="summary-label">Liquidity Deficit</span><span class="summary-value">{formatNumber(riskResult.risk_breakdown.liquidity_deficit_penalty)}</span></div>
            <div class="summary-cell"><span class="summary-label">Leverage Penalty</span><span class="summary-value">{formatNumber(riskResult.risk_breakdown.leverage_penalty)}</span></div>
          </div>

          {#if riskResult.risk_breakdown.explanations?.length > 0}
            <ul class="bullet-list">
              {#each riskResult.risk_breakdown.explanations as exp}<li>{exp}</li>{/each}
            </ul>
          {/if}

          <div class="block-head" style="margin-top: 0.5rem;">
            <h2>Bayesian Posterior Probabilities</h2>
            <span class="font-mono muted-cell">Stress multiplier {formatNumber(riskResult.bayesian_inference.stress_multiplier)}x</span>
          </div>

          <div class="summary-strip">
            <div class="summary-cell"><span class="summary-label">P(Recession)</span><span class="summary-value">{formatPercent(riskResult.bayesian_inference.p_recession)}</span></div>
            <div class="summary-cell"><span class="summary-label">P(Liquidity Crisis)</span><span class="summary-value">{formatPercent(riskResult.bayesian_inference.p_liquidity_crisis)}</span></div>
            <div class="summary-cell"><span class="summary-label">P(Equity Downturn)</span><span class="summary-value">{formatPercent(riskResult.bayesian_inference.p_equity_downturn)}</span></div>
            <div class="summary-cell"><span class="summary-label">P(Spread Widening)</span><span class="summary-value">{formatPercent(riskResult.bayesian_inference.p_credit_spread_widening)}</span></div>
          </div>

          {#if riskResult.bayesian_inference.explanation}
            <div class="notice-banner">{riskResult.bayesian_inference.explanation}</div>
          {/if}

          {#if riskResult.monte_carlo_report}
            <div class="block-head" style="margin-top: 0.5rem;">
              <h2>Monte Carlo Return Distribution</h2>
              <span class="font-mono muted-cell">{formatNumber(riskResult.monte_carlo_report.num_scenarios)} scenarios</span>
            </div>
            <div class="summary-strip">
              <div class="summary-cell"><span class="summary-label">Mean Return</span><span class="summary-value">{formatPercent(riskResult.monte_carlo_report.mean_simulated_return)}</span></div>
              <div class="summary-cell"><span class="summary-label">Std Dev</span><span class="summary-value">{formatPercent(riskResult.monte_carlo_report.std_simulated_return)}</span></div>
              <div class="summary-cell"><span class="summary-label">95% VaR / CVaR</span><span class="summary-value">{formatPercent(riskResult.monte_carlo_report.var_95)} / {formatPercent(riskResult.monte_carlo_report.cvar_95)}</span></div>
              <div class="summary-cell"><span class="summary-label">Worst-Case Loss</span><span class="summary-value neg-val">{formatPercent(riskResult.monte_carlo_report.worst_case_loss_pct)}</span></div>
            </div>
          {/if}
        </section>
      {/if}
    {/if}

    <!-- TAB 3: ADVERSARIAL STRESS TESTING -->
    {#if activeTab === 'stress'}
      <div class="surface-panel stack-gap">
        <div class="block-head">
          <div>
            <h2>Minimax Adversarial Game Tree Search</h2>
            <p class="block-sub">Zero-sum Alpha-Beta search pitting treasury policy against adverse macro shocks.</p>
          </div>
          <button type="button" class="btn btn-primary btn-sm" disabled={isStressTesting || !canWrite($user?.role)} onclick={handleStressTest}>
            {isStressTesting ? 'Running minimax...' : 'Run stress test'}
          </button>
        </div>

        {#if stressError}
          <div class="alert alert-danger">
            <strong>{stressError}</strong>
            {#if stressDetails.length > 0}
              <ul class="err-list">{#each stressDetails as d}<li>{d}</li>{/each}</ul>
            {/if}
          </div>
        {/if}
      </div>

      {#if stressResult}
        <section class="section-block" aria-label="Stress test results">
          <div class="block-head">
            <h2>Adversarial Outcome</h2>
            <span class="font-mono font-bold">Worst payoff {formatNumber(stressResult.adversarial_result.worst_case_payoff)}</span>
          </div>

          <div class="summary-strip cols-5">
            <div class="summary-cell"><span class="summary-label">Worst Shock</span><span class="summary-value" style="font-size: 1rem;">{stressResult.adversarial_result.worst_market_shock}</span></div>
            <div class="summary-cell"><span class="summary-label">Best Defense</span><span class="summary-value" style="font-size: 1rem;">{formatTreasuryAction(stressResult.adversarial_result.best_treasury_action)}</span></div>
            <div class="summary-cell"><span class="summary-label">Nodes Evaluated</span><span class="summary-value">{formatNumber(stressResult.adversarial_result.nodes_evaluated)}</span></div>
            <div class="summary-cell"><span class="summary-label">Pruned Branches</span><span class="summary-value">{formatNumber(stressResult.adversarial_result.pruned_branches)}</span></div>
            <div class="summary-cell"><span class="summary-label">Execution Time</span><span class="summary-value">{stressResult.adversarial_result.execution_time_ms.toFixed(1)} ms</span></div>
          </div>

          {#if stressResult.adversarial_result.explanation}
            <div class="notice-banner">{stressResult.adversarial_result.explanation}</div>
          {/if}

          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Crisis Scenario</th>
                  <th class="text-right">Equity Shock</th>
                  <th class="text-right">Spread Spike</th>
                  <th class="text-right">Liquidity Freeze</th>
                  <th class="text-right">Stressed Return</th>
                  <th class="text-right">Stressed Liquidity</th>
                  <th>Compliance</th>
                </tr>
              </thead>
              <tbody>
                {#each stressResult.scenarios as item}
                  <tr>
                    <td>
                      <div class="font-bold">{item.scenario.shock_name}</div>
                      <div class="muted-cell" style="font-size: 0.75rem;">{item.scenario.description}</div>
                    </td>
                    <td class="text-right numeric-cell">{formatPercent(item.scenario.equity_drawdown)}</td>
                    <td class="text-right numeric-cell">{formatPercent(item.scenario.credit_spread_spike)}</td>
                    <td class="text-right numeric-cell">{formatPercent(item.scenario.liquidity_freeze)}</td>
                    <td class="text-right numeric-cell {item.stressed_return < 0 ? 'neg-val' : ''}">{formatPercent(item.stressed_return)}</td>
                    <td class="text-right numeric-cell">{formatPercent(item.stressed_liquidity)}</td>
                    <td class={item.is_compliant ? 'pos-val' : 'neg-val'}>{item.is_compliant ? 'Compliant' : 'Breach'}</td>
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        </section>
      {/if}
    {/if}

    <!-- TAB 4: MULTI-AGENT EXECUTIVE COUNCIL -->
    {#if activeTab === 'council'}
      <div class="surface-panel stack-gap">
        <div class="block-head">
          <div>
            <h2>Executive Deliberation Council</h2>
            <p class="block-sub">Evaluate allocation consensus across Treasury, Risk Officer, and Compliance roles.</p>
          </div>
          {#if councilResult}
            <span class={councilResult.consensus_reached ? 'pos-val' : 'neg-val'}>
              {councilResult.consensus_reached ? 'Consensus reached' : 'Objections raised'}
            </span>
          {/if}
        </div>

        {#if councilError}
          <div class="alert alert-danger">
            <strong>{councilError}</strong>
            {#if councilDetails.length > 0}
              <ul class="err-list">{#each councilDetails as detail}<li>{detail}</li>{/each}</ul>
            {/if}
          </div>
        {/if}

        <form onsubmit={(e) => { e.preventDefault(); handleEvaluateCouncil(); }} class="stack-gap">
          <div class="settings-grid">
            <div class="form-group" style="margin-bottom: 0;">
              <label for="council-objective" class="form-label">Objective</label>
              <select id="council-objective" class="form-select" bind:value={councilObjective}>
                <option value="maximize_risk_adjusted_return">Maximize risk-adjusted return</option>
                <option value="minimize_risk">Minimize risk</option>
                <option value="maximize_liquidity">Maximize liquidity buffer</option>
              </select>
            </div>
            <div class="form-group" style="margin-bottom: 0;">
              <label for="council-market-state" class="form-label">Market regime</label>
              <select id="council-market-state" class="form-select" bind:value={councilMarketState}>
                <option value="normal">Normal</option>
                <option value="bull">Bull expansion</option>
                <option value="bear">Bear contraction</option>
                <option value="high_volatility">High volatility</option>
                <option value="liquidity_crunch">Liquidity crunch</option>
              </select>
            </div>
            <div class="form-group" style="margin-bottom: 0;">
              <label for="council-step-size" class="form-label">Step size (optional)</label>
              <input id="council-step-size" type="number" step="any" min="0.01" class="form-input font-mono" placeholder="Auto (capital / 20)" bind:value={councilStepSize} />
            </div>
          </div>

          <div class="btn-row" style="justify-content: flex-end;">
            <button type="submit" class="btn btn-primary btn-sm" disabled={isEvaluatingCouncil || !canWrite($user?.role)}>
              {isEvaluatingCouncil ? 'Convening council...' : 'Convene executive council'}
            </button>
          </div>
        </form>
      </div>

      {#if councilResult}
        <section class="section-block" aria-label="Council verdicts and strategy">
          <div class="notice-banner row-between">
            <div>
              <strong>{councilResult.consensus_reached ? 'Consensus reached:' : 'Consensus not reached:'}</strong>
              {councilResult.consensus_reached
                ? ' All three specialist evaluators verified liquidity, risk, and statutory mandates.'
                : ' Specialist objections flagged violations requiring capital adjustment.'}
            </div>
            <span class="font-mono">{formatNumber(councilResult.search_metrics.execution_time_ms, 1)} ms</span>
          </div>

          <div class="council-grid">
            {#each [
              { label: 'Treasury', opinion: councilResult.treasury_opinion, metricLabel: 'Utility score', okLabel: 'Acceptable', noLabel: 'Unacceptable' },
              { label: 'Risk Officer', opinion: councilResult.risk_opinion, metricLabel: 'Risk score', okLabel: 'Acceptable', noLabel: 'Unacceptable' },
              { label: 'Compliance', opinion: councilResult.compliance_opinion, metricLabel: 'Compliance ratio', okLabel: 'Compliant', noLabel: 'Non-compliant' }
            ] as agent}
              <div class="surface-panel stack-gap">
                <div class="block-head">
                  <div>
                    <span class="muted-cell" style="font-size: 0.75rem;">{agent.label}</span>
                    <h3>{agent.opinion.agent_name}</h3>
                  </div>
                  <span class={agent.opinion.is_acceptable ? 'pos-val' : 'neg-val'}>
                    {agent.opinion.is_acceptable ? agent.okLabel : agent.noLabel}
                  </span>
                </div>

                <div class="row-between score-line">
                  <span class="muted-cell">{agent.metricLabel}</span>
                  <span class="font-mono font-bold">{formatNumber(agent.opinion.score, 3)}</span>
                </div>

                {#if agent.opinion.findings?.length > 0}
                  <ul class="bullet-list">
                    {#each agent.opinion.findings as finding}<li>{finding}</li>{/each}
                  </ul>
                {:else}
                  <p class="muted-cell" style="font-size: 0.8125rem;">No findings logged.</p>
                {/if}
              </div>
            {/each}
          </div>

          <div class="summary-strip cols-5">
            <div class="summary-cell"><span class="summary-label">Allocated</span><span class="summary-value">{formatCurrency(councilResult.recommended_strategy.total_allocated, treasury?.base_currency)}</span></div>
            <div class="summary-cell"><span class="summary-label">Exp. Return</span><span class="summary-value pos-val">{formatPercent(councilResult.recommended_strategy.expected_return)}</span></div>
            <div class="summary-cell"><span class="summary-label">Risk Score</span><span class="summary-value">{formatNumber(councilResult.recommended_strategy.risk_score, 3)}</span></div>
            <div class="summary-cell"><span class="summary-label">Liquidity</span><span class="summary-value">{formatPercent(councilResult.recommended_strategy.liquidity_ratio)}</span></div>
            <div class="summary-cell"><span class="summary-label">Objective</span><span class="summary-value">{formatNumber(councilResult.recommended_strategy.objective_value, 4)}</span></div>
          </div>

          <div class="notice-banner">{councilResult.recommended_strategy.explanation}</div>

          {#if councilResult.recommended_strategy.actions?.length > 0}
            <div class="table-container">
              <table class="table">
                <thead>
                  <tr>
                    <th>Action</th>
                    <th>Target</th>
                    <th class="text-right">Amount</th>
                    <th class="text-right">Cost</th>
                    <th>Rationale</th>
                  </tr>
                </thead>
                <tbody>
                  {#each councilResult.recommended_strategy.actions as act}
                    <tr>
                      <td class="font-mono font-bold">{act.action_type}</td>
                      <td class="font-mono muted-cell">{act.asset_id || act.liability_id || 'Portfolio Cash'}</td>
                      <td class="text-right numeric-cell">{formatCurrency(act.amount, treasury?.base_currency)}</td>
                      <td class="text-right numeric-cell">{formatCurrency(act.cost, treasury?.base_currency)}</td>
                      <td class="muted-cell">{act.reason}</td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          {/if}
        </section>
      {/if}
    {/if}
  {/if}

  <!-- Asset Modal -->
  {#if showAssetModal}
    <div
      class="modal-backdrop"
      onclick={(e) => { if (e.target === e.currentTarget) showAssetModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showAssetModal = false; }}
      role="dialog"
      tabindex="-1"
      aria-modal="true"
    >
      <div class="modal-dialog">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">{modalAssetMode === 'create' ? 'Add investment asset' : `Edit ${assetForm.name}`}</h3>
            <p class="modal-subtitle">Define expected return, risk score, and liquidity rating.</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showAssetModal = false)} aria-label="Close">&times;</button>
        </div>

        <form onsubmit={handleSaveAsset} novalidate>
          <div class="modal-body">
            {#if assetModalErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul class="err-list">{#each assetModalErrors as err}<li>{err}</li>{/each}</ul>
              </div>
            {/if}

            <div class="settings-grid">
              <div class="form-group"><label for="assetIdInput" class="form-label">Identifier</label><input id="assetIdInput" type="text" class="form-input font-mono" bind:value={assetForm.id} disabled={modalAssetMode === 'edit' || isSavingAsset} /></div>
              <div class="form-group"><label for="assetNameInput" class="form-label">Name</label><input id="assetNameInput" type="text" class="form-input" bind:value={assetForm.name} disabled={isSavingAsset} /></div>
              <div class="form-group"><label for="assetTypeInput" class="form-label">Class</label><select id="assetTypeInput" class="form-input" bind:value={assetForm.asset_type} disabled={isSavingAsset}>{#each CANONICAL_ASSET_TYPES as opt}<option value={opt.value}>{opt.label}</option>{/each}</select></div>
              <div class="form-group"><label for="assetReturnInput" class="form-label">Expected return</label><input id="assetReturnInput" type="number" step="0.005" class="form-input font-mono" bind:value={assetForm.expected_return} disabled={isSavingAsset} /></div>
              <div class="form-group"><label for="assetRiskInput" class="form-label">Risk (0–1)</label><input id="assetRiskInput" type="number" step="0.01" min="0" max="1" class="form-input font-mono" bind:value={assetForm.risk} disabled={isSavingAsset} /></div>
              <div class="form-group"><label for="assetLiqInput" class="form-label">Liquidity (0–1)</label><input id="assetLiqInput" type="number" step="0.01" min="0" max="1" class="form-input font-mono" bind:value={assetForm.liquidity} disabled={isSavingAsset} /></div>
              <div class="form-group"><label for="assetCostInput" class="form-label">Transaction cost</label><input id="assetCostInput" type="number" step="0.001" min="0" class="form-input font-mono" bind:value={assetForm.cost} disabled={isSavingAsset} /></div>
              <div class="form-group"><label for="assetVolInput" class="form-label">Volatility</label><input id="assetVolInput" type="number" step="0.01" min="0" class="form-input font-mono" bind:value={assetForm.volatility} disabled={isSavingAsset} /></div>
            </div>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary btn-sm" onclick={() => (showAssetModal = false)} disabled={isSavingAsset}>Cancel</button>
            <button type="submit" class="btn btn-primary btn-sm" disabled={isSavingAsset}>{isSavingAsset ? 'Saving...' : 'Save asset'}</button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- Liability Modal -->
  {#if showLiabilityModal && treasury}
    <div
      class="modal-backdrop"
      onclick={(e) => { if (e.target === e.currentTarget) showLiabilityModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showLiabilityModal = false; }}
      role="dialog"
      tabindex="-1"
      aria-modal="true"
    >
      <div class="modal-dialog">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">{modalLiabilityMode === 'create' ? 'Add debt obligation' : `Edit ${liabilityForm.name}`}</h3>
            <p class="modal-subtitle">Set facility principal, interest rate, maturity, and mandatory reserve flag.</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showLiabilityModal = false)} aria-label="Close">&times;</button>
        </div>

        <form onsubmit={handleSaveLiability} novalidate>
          <div class="modal-body">
            {#if liabilityModalErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul class="err-list">{#each liabilityModalErrors as err}<li>{err}</li>{/each}</ul>
              </div>
            {/if}

            <div class="settings-grid">
              <div class="form-group"><label for="liabIdInput" class="form-label">Identifier</label><input id="liabIdInput" type="text" class="form-input font-mono" bind:value={liabilityForm.id} disabled={modalLiabilityMode === 'edit' || isSavingLiability} /></div>
              <div class="form-group"><label for="liabNameInput" class="form-label">Facility name</label><input id="liabNameInput" type="text" class="form-input" bind:value={liabilityForm.name} disabled={isSavingLiability} /></div>
              <div class="form-group"><label for="liabPrincipalInput" class="form-label">Principal ({treasury.base_currency})</label><input id="liabPrincipalInput" type="number" step="any" min="0" class="form-input font-mono" bind:value={liabilityForm.principal} disabled={isSavingLiability} /></div>
              <div class="form-group"><label for="liabRateInput" class="form-label">Interest rate</label><input id="liabRateInput" type="number" step="0.005" min="0" class="form-input font-mono" bind:value={liabilityForm.interest_rate} disabled={isSavingLiability} /></div>
              <div class="form-group"><label for="liabMaturityInput" class="form-label">Maturity (months)</label><input id="liabMaturityInput" type="number" min="1" class="form-input font-mono" bind:value={liabilityForm.maturity_months} disabled={isSavingLiability} /></div>
              <div class="form-group"><label for="liabPriorityInput" class="form-label">Priority</label><input id="liabPriorityInput" type="number" min="1" class="form-input font-mono" bind:value={liabilityForm.priority} disabled={isSavingLiability} /></div>
            </div>

            <label class="checkbox-label" style="margin-top: 0.75rem;">
              <input type="checkbox" bind:checked={liabilityForm.is_mandatory_reserve} disabled={isSavingLiability} />
              <span>Mandatory debt service reserve</span>
            </label>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary btn-sm" onclick={() => (showLiabilityModal = false)} disabled={isSavingLiability}>Cancel</button>
            <button type="submit" class="btn btn-primary btn-sm" disabled={isSavingLiability}>{isSavingLiability ? 'Saving...' : 'Save liability'}</button>
          </div>
        </form>
      </div>
    </div>
  {/if}

  <!-- Delete Confirmation Modal -->
  {#if showDeleteModal}
    <div
      class="modal-backdrop"
      onclick={(e) => { if (e.target === e.currentTarget) showDeleteModal = false; }}
      onkeydown={(e) => { if (e.key === 'Escape') showDeleteModal = false; }}
      role="dialog"
      tabindex="-1"
      aria-modal="true"
    >
      <div class="modal-dialog" style="max-width: 460px;">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">Remove {deleteTargetType}</h3>
            <p class="modal-subtitle">Permanent deletion from PostgreSQL ledger</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showDeleteModal = false)} aria-label="Close">&times;</button>
        </div>

        <div class="modal-body">
          <p>Remove <strong>{deleteTargetName}</strong> (<code class="font-mono">{deleteTargetId}</code>) from this treasury?</p>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary btn-sm" onclick={() => (showDeleteModal = false)} disabled={isDeleting}>Cancel</button>
          <button type="button" class="btn btn-danger btn-sm" onclick={executeDelete} disabled={isDeleting}>{isDeleting ? 'Deleting...' : 'Delete'}</button>
        </div>
      </div>
    </div>
  {/if}

  <!-- Edit Treasury Modal -->
  {#if showEditTreasuryModal && treasury}
    <div
      class="modal-backdrop"
      onclick={(e) => { if (e.target === e.currentTarget) handleCloseEditTreasury(); }}
      onkeydown={(e) => { if (e.key === 'Escape') handleCloseEditTreasury(); }}
      role="dialog"
      tabindex="-1"
      aria-modal="true"
    >
      <div class="modal-dialog">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">Edit Corporate Treasury</h3>
            <p class="modal-subtitle">Update portfolio capital base and policy parameters.</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => handleCloseEditTreasury()} aria-label="Close">&times;</button>
        </div>

        {#if showUnsavedPrompt}
          <div class="alert alert-warning row-between" style="margin: 1rem 1.375rem 0;">
            <span>Discard unsaved changes?</span>
            <div class="btn-row">
              <button type="button" class="btn btn-secondary btn-xs" onclick={() => (showUnsavedPrompt = false)}>Keep editing</button>
              <button type="button" class="btn btn-danger btn-xs" onclick={() => handleCloseEditTreasury(true)}>Discard</button>
            </div>
          </div>
        {/if}

        <form onsubmit={handleSaveTreasury} novalidate>
          <div class="modal-body">
            {#if editTreasuryErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul class="err-list">{#each editTreasuryErrors as err}<li>{err}</li>{/each}</ul>
              </div>
            {/if}

            <div class="form-group">
              <label for="edit-treasury-name" class="form-label">Treasury name</label>
              <input id="edit-treasury-name" type="text" class="form-input" bind:value={editTreasuryForm.name} maxlength="255" required />
            </div>

            <div class="settings-grid">
              <div class="form-group">
                <label for="edit-treasury-currency" class="form-label">Currency</label>
                <select id="edit-treasury-currency" class="form-select" bind:value={editTreasuryForm.base_currency}>
                  <option value="INR">INR</option>
                  <option value="USD">USD</option>
                  <option value="EUR">EUR</option>
                  <option value="GBP">GBP</option>
                  <option value="SGD">SGD</option>
                  <option value="JPY">JPY</option>
                </select>
              </div>
              <div class="form-group">
                <label for="edit-treasury-capital" class="form-label">Total capital</label>
                <input id="edit-treasury-capital" type="number" step="any" min="1" class="form-input font-mono" bind:value={editTreasuryForm.total_capital} required />
              </div>
              <div class="form-group" style="margin-bottom: 0;">
                <label for="edit-treasury-risk" class="form-label">Risk tolerance (0–1)</label>
                <input id="edit-treasury-risk" type="number" step="0.01" min="0" max="1" class="form-input font-mono" bind:value={editTreasuryForm.risk_tolerance} required />
              </div>
              <div class="form-group" style="margin-bottom: 0;">
                <label for="edit-treasury-liquidity" class="form-label">Min liquidity (0–1)</label>
                <input id="edit-treasury-liquidity" type="number" step="0.01" min="0" max="1" class="form-input font-mono" bind:value={editTreasuryForm.min_liquidity_ratio} required />
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary btn-sm" onclick={() => handleCloseEditTreasury()} disabled={isSavingTreasury}>Cancel</button>
            <button type="submit" class="btn btn-primary btn-sm" disabled={isSavingTreasury || !isTreasuryFormDirty}>{isSavingTreasury ? 'Saving...' : 'Save changes'}</button>
          </div>
        </form>
      </div>
    </div>
  {/if}
</div>

<style>
  .nav-row {
    margin-bottom: -0.5rem;
  }

  .back-link {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text-secondary);
  }

  .back-link:hover {
    color: var(--color-text);
  }

  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
  }

  .btn-row {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
  }

  .stack-gap {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .section-block {
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
  }

  .block-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .block-head h2,
  .block-head h3 {
    font-size: 1.0625rem;
    font-weight: 500;
  }

  .block-sub {
    margin-top: 0.2rem;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .settings-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
    gap: 0.875rem;
  }

  .field-hint {
    display: block;
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin-top: 0.25rem;
  }

  .checkbox-label {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.875rem;
    cursor: pointer;
  }

  .editor-block {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .editor-table td {
    padding: 0.35rem 0.4rem;
  }

  .editor-input {
    min-height: 32px;
    padding: 0.2rem 0.45rem;
    font-size: 0.8125rem;
  }

  .compact-select {
    width: auto;
    min-height: 32px;
    padding: 0.2rem 0.5rem;
    font-size: 0.8125rem;
  }

  .chart-container {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 0.55rem;
  }

  .bar-row {
    display: grid;
    grid-template-columns: minmax(120px, 180px) 1fr auto;
    align-items: center;
    gap: 0.875rem;
    font-size: 0.875rem;
  }

  .bar-label {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .bar-track {
    height: 8px;
    background-color: var(--color-bg-secondary);
    border-radius: 2px;
    overflow: hidden;
  }

  .bar-fill {
    height: 100%;
    background-color: var(--color-border-strong);
  }

  .bar-fill-highlight {
    background-color: var(--color-primary);
  }

  .bar-value {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    white-space: nowrap;
  }

  .highlight-row td {
    background-color: var(--color-bg-secondary);
  }

  .allocations-cell {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .alloc-line {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .council-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 1rem;
  }

  .score-line {
    padding: 0.5rem 0;
    border-top: 1px solid var(--color-border-subtle);
    border-bottom: 1px solid var(--color-border-subtle);
  }

  .bullet-list,
  .err-list {
    margin: 0.25rem 0 0;
    padding-left: 1.25rem;
    font-size: 0.8125rem;
    line-height: 1.5;
  }

  .class-cell {
    color: var(--color-text-secondary);
    text-transform: capitalize;
  }

  .pos-val { color: var(--color-success); font-weight: 500; }
  .neg-val { color: var(--color-danger); font-weight: 500; }
  .muted-cell { color: var(--color-text-muted); }

  .empty-box {
    padding: 2.5rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }

  @media (max-width: 640px) {
    .bar-row {
      grid-template-columns: 1fr;
      gap: 0.25rem;
    }
  }
</style>
