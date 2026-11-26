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

  async function fetchAssets() {
    isLoadingAssets = true;
    assetsError = null;
    try {
      const res = await api<TreasuryAssetsResponse>(`/api/v1/treasuries/${treasuryId}/assets`);
      if (res.data && res.data.length > 0) {
        assets = res.data;
      } else {
        loadSampleAssets();
      }
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        assetsError = err.message || 'Failed to retrieve assets.';
      } else {
        assetsError = 'Failed to load assets.';
      }
      loadSampleAssets();
    } finally {
      isLoadingAssets = false;
    }
  }

  async function fetchLiabilities() {
    isLoadingLiabilities = true;
    liabilitiesError = null;
    try {
      const res = await api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${treasuryId}/liabilities`);
      if (res.data && res.data.length > 0) {
        liabilities = res.data;
      } else {
        loadSampleLiabilities();
      }
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        liabilitiesError = err.message || 'Failed to retrieve liabilities.';
      } else {
        liabilitiesError = 'Failed to load liabilities.';
      }
      loadSampleLiabilities();
    } finally {
      isLoadingLiabilities = false;
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
        assets = [...assets, created];
        actionSuccessMessage = `Asset "${created.name}" created and saved to portfolio universe.`;
      } else {
        const updated = await api<Asset>(`/api/v1/treasuries/${treasuryId}/assets/${encodeURIComponent(payload.id)}`, {
          method: 'PUT',
          body: payload
        });
        assets = assets.map((a) => (a.id === updated.id ? updated : a));
        actionSuccessMessage = `Asset "${updated.name}" updated successfully in database.`;
      }

      showAssetModal = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        assetModalErrors = [err.message || 'Failed to persist asset to database.'];
        if (err.details && err.details.length > 0) {
          assetModalErrors = [...assetModalErrors, ...err.details];
        }
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
        liabilities = [...liabilities, created];
        actionSuccessMessage = `Liability "${created.name}" created and saved to portfolio.`;
      } else {
        const updated = await api<Liability>(`/api/v1/treasuries/${treasuryId}/liabilities/${encodeURIComponent(payload.id)}`, {
          method: 'PUT',
          body: payload
        });
        liabilities = liabilities.map((l) => (l.id === updated.id ? updated : l));
        actionSuccessMessage = `Liability "${updated.name}" updated successfully in database.`;
      }

      showLiabilityModal = false;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        liabilityModalErrors = [err.message || 'Failed to persist liability to database.'];
        if (err.details && err.details.length > 0) {
          liabilityModalErrors = [...liabilityModalErrors, ...err.details];
        }
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
    const sampleAssets: Asset[] = [
      { id: 'gov_bond', name: 'Government Bonds', asset_type: 'gov_bond', cost: 0.001, expected_return: 0.07, risk: 0.05, liquidity: 0.95, volatility: 0.03 },
      { id: 'bluechip_equity', name: 'Bluechip Equity Index', asset_type: 'equity', cost: 0.002, expected_return: 0.12, risk: 0.18, liquidity: 0.85, volatility: 0.15 },
      { id: 'commercial_paper', name: 'Corporate Commercial Paper', asset_type: 'corp_bond', cost: 0.001, expected_return: 0.08, risk: 0.08, liquidity: 0.75, volatility: 0.06 }
    ];
    try {
      const res = await api<TreasuryAssetsResponse>(`/api/v1/treasuries/${treasuryId}/assets`, {
        method: 'PUT',
        body: sampleAssets
      });
      assets = res.data;
      actionSuccessMessage = 'Sample institutional asset universe loaded and committed to database.';
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        actionErrorMessage = err.message || 'Failed to save sample assets.';
      } else {
        actionErrorMessage = 'Failed to load sample assets.';
      }
    } finally {
      isLoadingAssets = false;
    }
  }

  async function handleLoadAndPersistSampleLiabilities() {
    isLoadingLiabilities = true;
    actionErrorMessage = null;
    actionSuccessMessage = null;
    const sampleLiabilities: Liability[] = [
      { id: 'senior_loan', name: 'Senior Revolving Facility', principal: 15.0, interest_rate: 0.065, maturity_months: 12, priority: 1, is_mandatory_reserve: true }
    ];
    try {
      const res = await api<TreasuryLiabilitiesResponse>(`/api/v1/treasuries/${treasuryId}/liabilities`, {
        method: 'PUT',
        body: sampleLiabilities
      });
      liabilities = res.data;
      actionSuccessMessage = 'Sample liability facility loaded and committed to database.';
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        actionErrorMessage = err.message || 'Failed to save sample liabilities.';
      } else {
        actionErrorMessage = 'Failed to load sample liabilities.';
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

  async function handleEvaluateCouncil() {
    if (!treasury || isEvaluatingCouncil) return;
    councilError = null;
    councilDetails = [];

    if (assets.length === 0) {
      councilError = 'At least one investment asset must be configured in the asset universe.';
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
              class="btn btn-secondary btn-sm"
              onclick={openEditTreasuryModal}
            >
              Edit Treasury
            </button>
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

    <!-- Executive Balance Sheet Summary Row -->
    <div class="balance-sheet-kpis">
      <div class="kpi-card">
        <div class="kpi-header">
          <span class="kpi-category">Capital Base</span>
          <span class="currency-tag">{treasury.base_currency}</span>
        </div>
        <div class="kpi-value">{formatCurrency(totalCapital, treasury.base_currency)}</div>
        <p class="kpi-subtext">Total balance sheet assets on record</p>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <span class="kpi-category">Debt Obligations</span>
          <span class="badge badge-viewer" style="font-size: 0.6875rem;">{activeLiabilityCount} Facilities</span>
        </div>
        <div class="kpi-value">{formatCurrency(totalLiabilitiesPrincipal, treasury.base_currency)}</div>
        <p class="kpi-subtext">Total outstanding liability principal</p>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <span class="kpi-category">Net Capital Position</span>
          <span class="status-indicator-dot" class:dot-positive={netCapitalPosition >= 0} class:dot-negative={netCapitalPosition < 0}></span>
        </div>
        <div class="kpi-value" class:val-positive={netCapitalPosition >= 0} class:val-negative={netCapitalPosition < 0}>
          {formatCurrency(netCapitalPosition, treasury.base_currency)}
        </div>
        <p class="kpi-subtext">{netCapitalPosition >= 0 ? 'Unencumbered net capital surplus' : 'Balance sheet capital deficit'}</p>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <span class="kpi-category">Mandatory Debt Reserves</span>
          <span class="badge badge-analyst" style="font-size: 0.6875rem;">Statutory</span>
        </div>
        <div class="kpi-value">{formatCurrency(mandatoryDebtReserves, treasury.base_currency)}</div>
        <p class="kpi-subtext">Liquidity ring-fenced for debt service</p>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <span class="kpi-category">Asset Universe</span>
          <span class="badge badge-pass" style="font-size: 0.6875rem;">Live</span>
        </div>
        <div class="kpi-value">{activeAssetCount} <span style="font-size: 0.875rem; font-weight: 500; color: var(--color-text-muted);">classes</span></div>
        <p class="kpi-subtext">Eligible portfolio allocation candidates</p>
      </div>
    </div>

    <!-- Real-time Action Feedback Banners -->
    {#if actionSuccessMessage}
      <div class="alert alert-success" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
        <span>{actionSuccessMessage}</span>
        <button type="button" class="btn btn-secondary btn-xs" onclick={() => (actionSuccessMessage = null)}>
          Dismiss
        </button>
      </div>
    {/if}

    {#if actionErrorMessage}
      <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem;">
        <span>{actionErrorMessage}</span>
        <button type="button" class="btn btn-secondary btn-xs" onclick={() => (actionErrorMessage = null)}>
          Dismiss
        </button>
      </div>
    {/if}

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
        class:tab-active={activeTab === 'balance-sheet'}
        onclick={() => (activeTab = 'balance-sheet')}
      >
        Balance Sheet & Universe
      </button>
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
      <button
        type="button"
        class="tab-btn"
        class:tab-active={activeTab === 'council'}
        onclick={() => (activeTab = 'council')}
      >
        Multi-Agent Council
      </button>
    </div>

    <!-- TAB 0: BALANCE SHEET & ASSET UNIVERSE -->
    {#if activeTab === 'balance-sheet'}
      <!-- Investment Asset Universe Card -->
      <div class="card" style="margin-bottom: 2rem;">
        <div class="section-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
              <h2 class="section-title">Investment Asset Universe</h2>
              <span class="badge badge-pass">{activeAssetCount} Persisted</span>
            </div>
            <p class="section-subtitle">
              Configured financial assets and investment vehicles evaluated during AI capital optimization runs.
            </p>
          </div>
          {#if canWrite($user?.role)}
            <div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={handleLoadAndPersistSampleAssets}
                disabled={isLoadingAssets}
                title="Populate baseline corporate assets (Bonds, Equities, Commercial Paper)"
              >
                Reset to Sample Universe
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                onclick={openAddAssetModal}
                disabled={isLoadingAssets}
              >
                + Add Asset
              </button>
            </div>
          {/if}
        </div>

        {#if assetsError}
          <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center; margin-top: 1rem;">
            <span>{assetsError}</span>
            <button type="button" class="btn btn-secondary btn-sm" onclick={fetchAssets}>Retry</button>
          </div>
        {/if}

        {#if isLoadingAssets}
          <div class="empty-card">
            <p class="state-message">Loading asset universe from secure PostgreSQL ledger...</p>
          </div>
        {:else if assets.length === 0}
          <div class="empty-card">
            <h3 class="state-title">No Investment Assets Configured</h3>
            <p class="state-message">
              This corporate treasury currently has no configured investment assets in its allocation universe.
            </p>
            {#if canWrite($user?.role)}
              <div style="margin-top: 1.25rem; display: flex; gap: 0.75rem; justify-content: center;">
                <button type="button" class="btn btn-primary" onclick={openAddAssetModal}>
                  + Add First Asset
                </button>
                <button type="button" class="btn btn-secondary" onclick={handleLoadAndPersistSampleAssets}>
                  Load Baseline Institutional Universe
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="table-container" style="margin-top: 1rem;">
            <table class="table">
              <thead>
                <tr>
                  <th>Identifier</th>
                  <th>Name</th>
                  <th>Class / Type</th>
                  <th style="text-align: right;">Exp. Return</th>
                  <th style="text-align: right;">Risk Score</th>
                  <th style="text-align: right;">Liquidity</th>
                  <th style="text-align: right;">Cost</th>
                  <th style="text-align: right;">Volatility</th>
                  {#if canWrite($user?.role)}
                    <th style="text-align: right;">Actions</th>
                  {/if}
                </tr>
              </thead>
              <tbody>
                {#each assets as asset (asset.id)}
                  <tr>
                    <td><code>{asset.id}</code></td>
                    <td><strong>{asset.name}</strong></td>
                    <td><span class="tag-pill">{asset.asset_type}</span></td>
                    <td class="numeric-cell" style="text-align: right;">{formatPercent(asset.expected_return)}</td>
                    <td class="numeric-cell" style="text-align: right;">{formatNumber(asset.risk)}</td>
                    <td class="numeric-cell" style="text-align: right;">{formatNumber(asset.liquidity)}</td>
                    <td class="numeric-cell" style="text-align: right;">{formatNumber(asset.cost)}</td>
                    <td class="numeric-cell" style="text-align: right;">{asset.volatility !== undefined ? formatPercent(asset.volatility) : '-'}</td>
                    {#if canWrite($user?.role)}
                      <td style="text-align: right;">
                        <div style="display: flex; gap: 0.35rem; justify-content: flex-end;">
                          <button
                            type="button"
                            class="btn btn-secondary btn-xs"
                            onclick={() => openEditAssetModal(asset)}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            class="btn btn-danger btn-xs"
                            onclick={() => openDeleteConfirm('asset', asset.id, asset.name)}
                          >
                            Delete
                          </button>
                        </div>
                      </td>
                    {/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>

      <!-- Liabilities & Debt Obligations Card -->
      <div class="card" style="margin-bottom: 2rem;">
        <div class="section-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
              <h2 class="section-title">Liabilities & Debt Obligations</h2>
              <span class="badge badge-viewer">{activeLiabilityCount} Active</span>
            </div>
            <p class="section-subtitle">
              Contractual debt facilities, notes, and commitments requiring statutory liquidity provisioning.
            </p>
          </div>
          {#if canWrite($user?.role)}
            <div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                onclick={handleLoadAndPersistSampleLiabilities}
                disabled={isLoadingLiabilities}
                title="Populate baseline corporate senior revolving credit facility"
              >
                Reset to Sample Debt
              </button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                onclick={openAddLiabilityModal}
                disabled={isLoadingLiabilities}
              >
                + Add Liability
              </button>
            </div>
          {/if}
        </div>

        {#if liabilitiesError}
          <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center; margin-top: 1rem;">
            <span>{liabilitiesError}</span>
            <button type="button" class="btn btn-secondary btn-sm" onclick={fetchLiabilities}>Retry</button>
          </div>
        {/if}

        {#if isLoadingLiabilities}
          <div class="empty-card">
            <p class="state-message">Loading liabilities from secure PostgreSQL ledger...</p>
          </div>
        {:else if liabilities.length === 0}
          <div class="empty-card">
            <h3 class="state-title">No Debt Obligations Recorded</h3>
            <p class="state-message">
              This balance sheet has no outstanding debts or loan obligations. The full capital base is unencumbered.
            </p>
            {#if canWrite($user?.role)}
              <div style="margin-top: 1.25rem; display: flex; gap: 0.75rem; justify-content: center;">
                <button type="button" class="btn btn-primary" onclick={openAddLiabilityModal}>
                  + Add Debt Obligation
                </button>
                <button type="button" class="btn btn-secondary" onclick={handleLoadAndPersistSampleLiabilities}>
                  Load Sample Debt Facility
                </button>
              </div>
            {/if}
          </div>
        {:else}
          <div class="table-container" style="margin-top: 1rem;">
            <table class="table">
              <thead>
                <tr>
                  <th>Identifier</th>
                  <th>Name</th>
                  <th style="text-align: right;">Principal</th>
                  <th style="text-align: right;">Interest Rate</th>
                  <th style="text-align: right;">Maturity</th>
                  <th style="text-align: center;">Priority</th>
                  <th style="text-align: center;">Statutory Reserve</th>
                  {#if canWrite($user?.role)}
                    <th style="text-align: right;">Actions</th>
                  {/if}
                </tr>
              </thead>
              <tbody>
                {#each liabilities as liab (liab.id)}
                  <tr>
                    <td><code>{liab.id}</code></td>
                    <td><strong>{liab.name}</strong></td>
                    <td class="numeric-cell" style="text-align: right;">{formatCurrency(liab.principal, treasury.base_currency)}</td>
                    <td class="numeric-cell" style="text-align: right;">{formatPercent(liab.interest_rate)}</td>
                    <td class="numeric-cell" style="text-align: right;">{liab.maturity_months ?? 12} mos</td>
                    <td style="text-align: center;"><span class="tag-pill">P{liab.priority}</span></td>
                    <td style="text-align: center;">
                      {#if liab.is_mandatory_reserve}
                        <span class="badge badge-analyst" style="font-size: 0.6875rem;">Mandatory</span>
                      {:else}
                        <span class="badge badge-viewer" style="font-size: 0.6875rem;">Discretionary</span>
                      {/if}
                    </td>
                    {#if canWrite($user?.role)}
                      <td style="text-align: right;">
                        <div style="display: flex; gap: 0.35rem; justify-content: flex-end;">
                          <button
                            type="button"
                            class="btn btn-secondary btn-xs"
                            onclick={() => openEditLiabilityModal(liab)}
                          >
                            Edit
                          </button>
                          <button
                            type="button"
                            class="btn btn-danger btn-xs"
                            onclick={() => openDeleteConfirm('liability', liab.id, liab.name)}
                          >
                            Delete
                          </button>
                        </div>
                      </td>
                    {/if}
                  </tr>
                {/each}
              </tbody>
            </table>
          </div>
        {/if}
      </div>
    {/if}

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
                        <select
                          class="form-input editor-input"
                          bind:value={asset.asset_type}
                          disabled={isGenerating}
                        >
                          {#each CANONICAL_ASSET_TYPES as opt}
                            <option value={opt.value}>{opt.value}</option>
                          {/each}
                        </select>
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
              Optimization Engine (Port 8001) is currently unavailable (Member 2 solver awaiting deployment).
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

  <!-- TAB 4: MULTI-AGENT COUNCIL -->
  {#if activeTab === 'council'}
    <!-- Council Deliberation Configuration Card -->
    <div class="card" style="margin-bottom: 2rem;">
      <div class="section-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
        <div>
          <h2 class="section-title">Multi-Agent Executive Council</h2>
          <p class="section-subtitle">
            Tri-agent deliberative governance: Autonomous evaluation by Chief Treasury Agent, Chief Risk Officer Agent, and Regulatory Compliance Agent.
          </p>
        </div>
        {#if councilResult}
          <span class="badge {councilResult.consensus_reached ? 'badge-pass' : 'badge-breach'}">
            {councilResult.consensus_reached ? 'Consensus Approved' : 'Consensus Disputed'}
          </span>
        {/if}
      </div>

      <!-- Portfolio Alignment Notice -->
      <div class="portfolio-banner" style="margin-bottom: 1.5rem;">
        <div class="portfolio-banner-header">
          <span style="font-size: 0.75rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-primary);">
            Institutional Deliberation Protocol
          </span>
          <span style="font-size: 0.75rem; color: var(--color-text-muted); font-family: monospace;">
            Capital: {formatCurrency(totalCapital, treasury?.base_currency)} | Assets: {activeAssetCount} | Liabilities: {activeLiabilityCount}
          </span>
        </div>
        <p class="portfolio-banner-text">
          The Multi-Agent Council evaluates portfolio candidates against competing fiduciary objectives: Treasury seeks optimal risk-adjusted yield, CRO enforces tail-risk constraints and VaR limits, and Compliance enforces mandatory statutory reserve ratios.
        </p>
      </div>

      {#if councilError}
        <div class="alert alert-danger" style="margin-bottom: 1.5rem;">
          <div style="font-weight: 600; margin-bottom: 0.25rem;">Council Deliberation Error</div>
          <div>{councilError}</div>
          {#if councilDetails.length > 0}
            <ul style="margin: 0.5rem 0 0; padding-left: 1.25rem;">
              {#each councilDetails as detail}
                <li>{detail}</li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}

      <form onsubmit={(e) => { e.preventDefault(); handleEvaluateCouncil(); }}>
        <div class="settings-grid" style="grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); margin-bottom: 1.25rem;">
          <div class="form-group">
            <label for="council-objective" class="form-label">Strategic Objective</label>
            <select id="council-objective" class="form-select" bind:value={councilObjective}>
              <option value="maximize_risk_adjusted_return">Maximize Risk-Adjusted Return</option>
              <option value="minimize_risk">Minimize Risk (Preserve Capital)</option>
              <option value="maximize_liquidity">Maximize Liquidity Buffer</option>
            </select>
          </div>

          <div class="form-group">
            <label for="council-market-state" class="form-label">Simulated Market State</label>
            <select id="council-market-state" class="form-select" bind:value={councilMarketState}>
              <option value="normal">Normal Operating Environment</option>
              <option value="bull">Bull Market Expansion</option>
              <option value="bear">Bear Market Contraction</option>
              <option value="high_volatility">High Volatility Regime</option>
              <option value="liquidity_crunch">Liquidity Crunch / Credit Freeze</option>
            </select>
          </div>

          <div class="form-group">
            <label for="council-step-size" class="form-label">
              Discretization Step Size
              <span style="font-weight: 400; color: var(--color-text-muted); font-size: 0.75rem;">(optional)</span>
            </label>
            <input
              id="council-step-size"
              type="number"
              step="any"
              min="0.01"
              class="form-input"
              placeholder="Auto (Capital / 20)"
              bind:value={councilStepSize}
            />
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; align-items: center; gap: 1rem; flex-wrap: wrap;">
          {#if !canWrite($user?.role)}
            <span style="font-size: 0.8125rem; color: var(--color-text-muted);">
              Analyst or Admin permissions required to convene council.
            </span>
          {/if}
          <button
            type="submit"
            class="btn btn-primary"
            disabled={isEvaluatingCouncil || !canWrite($user?.role)}
            style="min-width: 180px;"
          >
            {isEvaluatingCouncil ? 'Convening Council...' : 'Convene AI Council'}
          </button>
        </div>
      </form>
    </div>

    <!-- Deliberation Results Section -->
    {#if councilResult}
      <!-- Consensus Status Banner -->
      <div
        class="card"
        style="margin-bottom: 2rem; border-left: 4px solid {councilResult.consensus_reached ? 'var(--color-success)' : 'var(--color-danger)'};"
      >
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
              <span
                class="badge {councilResult.consensus_reached ? 'badge-pass' : 'badge-breach'}"
                style="font-size: 0.8125rem; padding: 0.3rem 0.65rem;"
              >
                {councilResult.consensus_reached ? 'STATUS: APPROVED' : 'STATUS: ACTION REQUIRED'}
              </span>
              <h3 style="font-size: 1.125rem; font-weight: 700; color: var(--color-text); margin: 0;">
                {councilResult.consensus_reached
                  ? 'Council Consensus Reached: Strategy Approved'
                  : 'Council Consensus Not Reached: Specialist Objections Raised'}
              </h3>
            </div>
            <p style="font-size: 0.875rem; color: var(--color-text-muted); margin: 0;">
              {councilResult.consensus_reached
                ? 'All 3 specialist agents have verified that the candidate allocation satisfies liquidity, risk, and statutory mandates.'
                : 'Specialist agents identified fiduciary or compliance violations requiring strategy re-tuning or capital rebalancing.'}
            </p>
          </div>
          <div style="text-align: right;">
            <span style="font-size: 0.75rem; text-transform: uppercase; color: var(--color-text-muted); font-weight: 600;">Execution Time</span>
            <div style="font-size: 1.125rem; font-weight: 700; color: var(--color-text); font-variant-numeric: tabular-nums;">
              {formatNumber(councilResult.search_metrics.execution_time_ms, 1)} ms
            </div>
          </div>
        </div>
      </div>

      <!-- Specialist Agent Assessment Cards (3 Columns) -->
      <div style="margin-bottom: 2rem;">
        <h3 class="section-title" style="margin-bottom: 0.5rem;">Specialist Agent Deliberation</h3>
        <p class="section-subtitle" style="margin-bottom: 1.25rem;">
          Individual verdicts, diagnostic health scores, and key findings recorded by the council members.
        </p>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 1.25rem;">
          <!-- Agent 1: Chief Treasury Agent -->
          <div class="card" style="display: flex; flex-direction: column; justify-content: space-between; border-top: 3px solid var(--color-primary);">
            <div>
              <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                <div>
                  <span style="font-size: 0.6875rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-text-muted);">
                    ROLE: TREASURY
                  </span>
                  <h4 style="font-size: 1rem; font-weight: 700; color: var(--color-text); margin: 0.15rem 0 0;">
                    {councilResult.treasury_opinion.agent_name}
                  </h4>
                </div>
                <span class="badge {councilResult.treasury_opinion.is_acceptable ? 'badge-pass' : 'badge-breach'}">
                  {councilResult.treasury_opinion.is_acceptable ? 'Acceptable' : 'Unacceptable'}
                </span>
              </div>

              <div style="display: flex; justify-content: space-between; align-items: baseline; padding: 0.75rem; background-color: var(--color-bg); border-radius: var(--radius); margin-bottom: 1rem;">
                <span style="font-size: 0.8125rem; color: var(--color-text-muted); font-weight: 600;">Utility Score</span>
                <span style="font-size: 1.25rem; font-weight: 700; color: var(--color-text); font-variant-numeric: tabular-nums;">
                  {formatNumber(councilResult.treasury_opinion.score, 3)}
                </span>
              </div>

              <div>
                <span style="font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--color-text-muted);">
                  Key Findings
                </span>
                {#if councilResult.treasury_opinion.findings && councilResult.treasury_opinion.findings.length > 0}
                  <ul style="margin: 0.5rem 0 0; padding-left: 1.25rem; font-size: 0.8125rem; color: var(--color-text); line-height: 1.5;">
                    {#each councilResult.treasury_opinion.findings as finding}
                      <li>{finding}</li>
                    {/each}
                  </ul>
                {:else}
                  <p style="font-size: 0.8125rem; color: var(--color-text-muted); margin-top: 0.35rem;">No explicit findings logged.</p>
                {/if}
              </div>
            </div>

            {#if councilResult.treasury_opinion.suggested_actions && councilResult.treasury_opinion.suggested_actions.length > 0}
              <div style="margin-top: 1rem; padding-top: 0.75rem; border-top: 1px solid var(--color-border);">
                <span style="font-size: 0.6875rem; font-weight: 600; text-transform: uppercase; color: var(--color-text-muted);">
                  Suggested Action Proposals ({councilResult.treasury_opinion.suggested_actions.length})
                </span>
                <div style="display: flex; flex-direction: column; gap: 0.35rem; margin-top: 0.35rem;">
                  {#each councilResult.treasury_opinion.suggested_actions as proposal}
                    <div style="font-size: 0.75rem; background: var(--color-bg); padding: 0.35rem 0.5rem; border-radius: var(--radius);">
                      <span style="font-weight: 600; text-transform: uppercase;">{proposal.action.action_type}</span>: {proposal.rationale}
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>

          <!-- Agent 2: Chief Risk Officer Agent -->
          <div class="card" style="display: flex; flex-direction: column; justify-content: space-between; border-top: 3px solid var(--color-primary);">
            <div>
              <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                <div>
                  <span style="font-size: 0.6875rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-text-muted);">
                    ROLE: RISK
                  </span>
                  <h4 style="font-size: 1rem; font-weight: 700; color: var(--color-text); margin: 0.15rem 0 0;">
                    {councilResult.risk_opinion.agent_name}
                  </h4>
                </div>
                <span class="badge {councilResult.risk_opinion.is_acceptable ? 'badge-pass' : 'badge-breach'}">
                  {councilResult.risk_opinion.is_acceptable ? 'Acceptable' : 'Unacceptable'}
                </span>
              </div>

              <div style="display: flex; justify-content: space-between; align-items: baseline; padding: 0.75rem; background-color: var(--color-bg); border-radius: var(--radius); margin-bottom: 1rem;">
                <span style="font-size: 0.8125rem; color: var(--color-text-muted); font-weight: 600;">Risk Score</span>
                <span style="font-size: 1.25rem; font-weight: 700; color: var(--color-text); font-variant-numeric: tabular-nums;">
                  {formatNumber(councilResult.risk_opinion.score, 3)}
                </span>
              </div>

              <div>
                <span style="font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--color-text-muted);">
                  Key Findings
                </span>
                {#if councilResult.risk_opinion.findings && councilResult.risk_opinion.findings.length > 0}
                  <ul style="margin: 0.5rem 0 0; padding-left: 1.25rem; font-size: 0.8125rem; color: var(--color-text); line-height: 1.5;">
                    {#each councilResult.risk_opinion.findings as finding}
                      <li>{finding}</li>
                    {/each}
                  </ul>
                {:else}
                  <p style="font-size: 0.8125rem; color: var(--color-text-muted); margin-top: 0.35rem;">No explicit findings logged.</p>
                {/if}
              </div>
            </div>

            {#if councilResult.risk_opinion.suggested_actions && councilResult.risk_opinion.suggested_actions.length > 0}
              <div style="margin-top: 1rem; padding-top: 0.75rem; border-top: 1px solid var(--color-border);">
                <span style="font-size: 0.6875rem; font-weight: 600; text-transform: uppercase; color: var(--color-text-muted);">
                  Suggested Action Proposals ({councilResult.risk_opinion.suggested_actions.length})
                </span>
                <div style="display: flex; flex-direction: column; gap: 0.35rem; margin-top: 0.35rem;">
                  {#each councilResult.risk_opinion.suggested_actions as proposal}
                    <div style="font-size: 0.75rem; background: var(--color-bg); padding: 0.35rem 0.5rem; border-radius: var(--radius);">
                      <span style="font-weight: 600; text-transform: uppercase;">{proposal.action.action_type}</span>: {proposal.rationale}
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>

          <!-- Agent 3: Regulatory Compliance Agent -->
          <div class="card" style="display: flex; flex-direction: column; justify-content: space-between; border-top: 3px solid var(--color-primary);">
            <div>
              <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 1rem;">
                <div>
                  <span style="font-size: 0.6875rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; color: var(--color-text-muted);">
                    ROLE: COMPLIANCE
                  </span>
                  <h4 style="font-size: 1rem; font-weight: 700; color: var(--color-text); margin: 0.15rem 0 0;">
                    {councilResult.compliance_opinion.agent_name}
                  </h4>
                </div>
                <span class="badge {councilResult.compliance_opinion.is_acceptable ? 'badge-pass' : 'badge-breach'}">
                  {councilResult.compliance_opinion.is_acceptable ? 'Compliant' : 'Non-Compliant'}
                </span>
              </div>

              <div style="display: flex; justify-content: space-between; align-items: baseline; padding: 0.75rem; background-color: var(--color-bg); border-radius: var(--radius); margin-bottom: 1rem;">
                <span style="font-size: 0.8125rem; color: var(--color-text-muted); font-weight: 600;">Compliance Ratio</span>
                <span style="font-size: 1.25rem; font-weight: 700; color: var(--color-text); font-variant-numeric: tabular-nums;">
                  {formatNumber(councilResult.compliance_opinion.score, 3)}
                </span>
              </div>

              <div>
                <span style="font-size: 0.75rem; font-weight: 600; text-transform: uppercase; letter-spacing: 0.04em; color: var(--color-text-muted);">
                  Key Findings
                </span>
                {#if councilResult.compliance_opinion.findings && councilResult.compliance_opinion.findings.length > 0}
                  <ul style="margin: 0.5rem 0 0; padding-left: 1.25rem; font-size: 0.8125rem; color: var(--color-text); line-height: 1.5;">
                    {#each councilResult.compliance_opinion.findings as finding}
                      <li>{finding}</li>
                    {/each}
                  </ul>
                {:else}
                  <p style="font-size: 0.8125rem; color: var(--color-text-muted); margin-top: 0.35rem;">No explicit findings logged.</p>
                {/if}
              </div>
            </div>

            {#if councilResult.compliance_opinion.suggested_actions && councilResult.compliance_opinion.suggested_actions.length > 0}
              <div style="margin-top: 1rem; padding-top: 0.75rem; border-top: 1px solid var(--color-border);">
                <span style="font-size: 0.6875rem; font-weight: 600; text-transform: uppercase; color: var(--color-text-muted);">
                  Suggested Action Proposals ({councilResult.compliance_opinion.suggested_actions.length})
                </span>
                <div style="display: flex; flex-direction: column; gap: 0.35rem; margin-top: 0.35rem;">
                  {#each councilResult.compliance_opinion.suggested_actions as proposal}
                    <div style="font-size: 0.75rem; background: var(--color-bg); padding: 0.35rem 0.5rem; border-radius: var(--radius);">
                      <span style="font-weight: 600; text-transform: uppercase;">{proposal.action.action_type}</span>: {proposal.rationale}
                    </div>
                  {/each}
                </div>
              </div>
            {/if}
          </div>
        </div>
      </div>

      <!-- Synthesized Strategy Summary Card -->
      <div class="card result-card" style="margin-bottom: 2rem;">
        <div class="section-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
              <h3 class="section-title">Synthesized Strategy Synthesis</h3>
              <span class="badge {councilResult.recommended_strategy.is_compliant ? 'badge-pass' : 'badge-breach'}">
                {councilResult.recommended_strategy.is_compliant ? 'Mandate Compliant' : 'Non-Compliant'}
              </span>
            </div>
            <p class="section-subtitle">
              Synthesized by Decision Agent balancing Treasury, Risk, and Compliance mandates.
            </p>
          </div>
        </div>

        <!-- Strategy Key Performance Indicators -->
        <div class="metrics-grid" style="grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); margin-bottom: 1.5rem;">
          <div class="metric-card">
            <span class="metric-label">Total Allocated</span>
            <span class="metric-value">
              {formatCurrency(councilResult.recommended_strategy.total_allocated, treasury?.base_currency)}
            </span>
          </div>

          <div class="metric-card">
            <span class="metric-label">Expected Return</span>
            <span class="metric-value" style="color: var(--color-success);">
              {formatPercent(councilResult.recommended_strategy.expected_return)}
            </span>
          </div>

          <div class="metric-card">
            <span class="metric-label">Portfolio Risk Score</span>
            <span class="metric-value">
              {formatNumber(councilResult.recommended_strategy.risk_score, 3)}
            </span>
          </div>

          <div class="metric-card">
            <span class="metric-label">Liquidity Ratio</span>
            <span class="metric-value">
              {formatPercent(councilResult.recommended_strategy.liquidity_ratio)}
            </span>
          </div>

          <div class="metric-card">
            <span class="metric-label">Objective Value</span>
            <span class="metric-value">
              {formatNumber(councilResult.recommended_strategy.objective_value, 4)}
            </span>
          </div>
        </div>

        <!-- Executive Strategy Explanation -->
        <div class="explanation-box" style="margin-bottom: 1.5rem;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.25rem;">
            <span class="detail-label" style="font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em;">
              Council Deliberation Rationale
            </span>
          </div>
          <p class="explanation-text">
            {councilResult.recommended_strategy.explanation}
          </p>
        </div>

        <!-- Operational Actions Table (if actions exist) -->
        {#if councilResult.recommended_strategy.actions && councilResult.recommended_strategy.actions.length > 0}
          <div style="margin-bottom: 1.5rem;">
            <h4 style="font-size: 0.9375rem; font-weight: 700; color: var(--color-text); margin-bottom: 0.75rem;">
              Recommended Financial Operations
            </h4>
            <div class="table-container">
              <table class="table">
                <thead>
                  <tr>
                    <th>Action</th>
                    <th>Target ID</th>
                    <th style="text-align: right;">Amount</th>
                    <th style="text-align: right;">Cost</th>
                    <th>Fiduciary Rationale</th>
                  </tr>
                </thead>
                <tbody>
                  {#each councilResult.recommended_strategy.actions as act}
                    <tr>
                      <td>
                        <span class="badge badge-pass" style="text-transform: uppercase; font-size: 0.6875rem;">
                          {act.action_type}
                        </span>
                      </td>
                      <td style="font-family: monospace; font-size: 0.8125rem;">
                        {act.asset_id || act.liability_id || 'Portfolio Cash'}
                      </td>
                      <td style="text-align: right;" class="numeric-cell">
                        {formatCurrency(act.amount, treasury?.base_currency)}
                      </td>
                      <td style="text-align: right;" class="numeric-cell">
                        {formatCurrency(act.cost, treasury?.base_currency)}
                      </td>
                      <td style="font-size: 0.8125rem; color: var(--color-text-muted);">
                        {act.reason}
                      </td>
                    </tr>
                  {/each}
                </tbody>
              </table>
            </div>
          </div>
        {/if}

        <!-- Search Telemetry & AI Safety -->
        <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border); font-size: 0.75rem; color: var(--color-text-muted);">
          <div>
            <span>Search Algorithm: <strong style="color: var(--color-text);">{councilResult.search_metrics.algorithm}</strong></span>
            <span style="margin: 0 0.5rem;">|</span>
            <span>States Explored: <strong style="color: var(--color-text);">{councilResult.search_metrics.states_explored}</strong></span>
            <span style="margin: 0 0.5rem;">|</span>
            <span>Search Depth: <strong style="color: var(--color-text);">{councilResult.search_metrics.depth}</strong></span>
          </div>
          <div style="font-style: italic;">
            Autonomous Governance Gate: Council outputs represent AI decision-support. Manual authorization is enforced.
          </div>
        </div>
      </div>
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
            <h3 class="modal-title">{modalAssetMode === 'create' ? 'Add Investment Asset' : `Edit Asset: ${assetForm.name}`}</h3>
            <p class="modal-subtitle">Define risk, liquidity, and expected financial returns</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showAssetModal = false)} aria-label="Close modal">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="4" y1="4" x2="12" y2="12" stroke-linecap="round" />
              <line x1="12" y1="4" x2="4" y2="12" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        <form onsubmit={handleSaveAsset} novalidate>
          <div class="modal-body">
            {#if assetModalErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul style="padding-left: 1.25rem; margin: 0;">
                  {#each assetModalErrors as err}
                    <li>{err}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            <div class="settings-grid">
              <div class="form-group">
                <label for="assetIdInput" class="form-label">Asset Identifier</label>
                <input
                  id="assetIdInput"
                  type="text"
                  class="form-input"
                  placeholder="e.g. gov_bond_10y"
                  bind:value={assetForm.id}
                  disabled={modalAssetMode === 'edit' || isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetNameInput" class="form-label">Asset Name</label>
                <input
                  id="assetNameInput"
                  type="text"
                  class="form-input"
                  placeholder="e.g. Sovereign Treasury Bond"
                  bind:value={assetForm.name}
                  disabled={isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetTypeInput" class="form-label">Asset Class / Type</label>
                <select
                  id="assetTypeInput"
                  class="form-input"
                  bind:value={assetForm.asset_type}
                  disabled={isSavingAsset}
                >
                  {#each CANONICAL_ASSET_TYPES as opt}
                    <option value={opt.value}>{opt.label}</option>
                  {/each}
                </select>
              </div>

              <div class="form-group">
                <label for="assetReturnInput" class="form-label">Expected Annual Return</label>
                <input
                  id="assetReturnInput"
                  type="number"
                  step="0.005"
                  class="form-input"
                  placeholder="e.g. 0.08"
                  bind:value={assetForm.expected_return}
                  disabled={isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetRiskInput" class="form-label">Risk Score (0.00 to 1.00)</label>
                <input
                  id="assetRiskInput"
                  type="number"
                  step="0.01"
                  min="0"
                  max="1"
                  class="form-input"
                  placeholder="e.g. 0.05"
                  bind:value={assetForm.risk}
                  disabled={isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetLiqInput" class="form-label">Liquidity Score (0.00 to 1.00)</label>
                <input
                  id="assetLiqInput"
                  type="number"
                  step="0.01"
                  min="0"
                  max="1"
                  class="form-input"
                  placeholder="e.g. 0.85"
                  bind:value={assetForm.liquidity}
                  disabled={isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetCostInput" class="form-label">Transaction / Execution Cost</label>
                <input
                  id="assetCostInput"
                  type="number"
                  step="0.001"
                  min="0"
                  class="form-input"
                  placeholder="e.g. 0.001"
                  bind:value={assetForm.cost}
                  disabled={isSavingAsset}
                />
              </div>

              <div class="form-group">
                <label for="assetVolInput" class="form-label">Annualized Volatility</label>
                <input
                  id="assetVolInput"
                  type="number"
                  step="0.01"
                  min="0"
                  class="form-input"
                  placeholder="e.g. 0.04"
                  bind:value={assetForm.volatility}
                  disabled={isSavingAsset}
                />
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" onclick={() => (showAssetModal = false)} disabled={isSavingAsset}>
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" disabled={isSavingAsset}>
              {isSavingAsset ? 'Saving to Ledger...' : (modalAssetMode === 'create' ? 'Create Asset' : 'Save Changes')}
            </button>
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
            <h3 class="modal-title">{modalLiabilityMode === 'create' ? 'Add Debt Obligation' : `Edit Liability: ${liabilityForm.name}`}</h3>
            <p class="modal-subtitle">Configure debt facility, principal, interest rate, and mandatory reserves</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showLiabilityModal = false)} aria-label="Close modal">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="4" y1="4" x2="12" y2="12" stroke-linecap="round" />
              <line x1="12" y1="4" x2="4" y2="12" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        <form onsubmit={handleSaveLiability} novalidate>
          <div class="modal-body">
            {#if liabilityModalErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul style="padding-left: 1.25rem; margin: 0;">
                  {#each liabilityModalErrors as err}
                    <li>{err}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            <div class="settings-grid">
              <div class="form-group">
                <label for="liabIdInput" class="form-label">Liability Identifier</label>
                <input
                  id="liabIdInput"
                  type="text"
                  class="form-input"
                  placeholder="e.g. credit_facility_1"
                  bind:value={liabilityForm.id}
                  disabled={modalLiabilityMode === 'edit' || isSavingLiability}
                />
              </div>

              <div class="form-group">
                <label for="liabNameInput" class="form-label">Facility / Liability Name</label>
                <input
                  id="liabNameInput"
                  type="text"
                  class="form-input"
                  placeholder="e.g. Senior Revolving Facility"
                  bind:value={liabilityForm.name}
                  disabled={isSavingLiability}
                />
              </div>

              <div class="form-group">
                <label for="liabPrincipalInput" class="form-label">Principal Amount ({treasury.base_currency})</label>
                <input
                  id="liabPrincipalInput"
                  type="number"
                  step="any"
                  min="0"
                  class="form-input"
                  bind:value={liabilityForm.principal}
                  disabled={isSavingLiability}
                />
              </div>

              <div class="form-group">
                <label for="liabRateInput" class="form-label">Interest Rate (e.g. 0.065 = 6.5%)</label>
                <input
                  id="liabRateInput"
                  type="number"
                  step="0.005"
                  min="0"
                  class="form-input"
                  bind:value={liabilityForm.interest_rate}
                  disabled={isSavingLiability}
                />
              </div>

              <div class="form-group">
                <label for="liabMaturityInput" class="form-label">Maturity Term (Months)</label>
                <input
                  id="liabMaturityInput"
                  type="number"
                  min="1"
                  class="form-input"
                  bind:value={liabilityForm.maturity_months}
                  disabled={isSavingLiability}
                />
              </div>

              <div class="form-group">
                <label for="liabPriorityInput" class="form-label">Settlement Priority (1 = Highest)</label>
                <input
                  id="liabPriorityInput"
                  type="number"
                  min="1"
                  class="form-input"
                  bind:value={liabilityForm.priority}
                  disabled={isSavingLiability}
                />
              </div>
            </div>

            <div class="checkbox-group" style="margin-top: 1rem;">
              <label class="checkbox-label">
                <input
                  type="checkbox"
                  bind:checked={liabilityForm.is_mandatory_reserve}
                  disabled={isSavingLiability}
                />
                <span>Mandatory debt service reserve (requires cash liquidity provisioning)</span>
              </label>
            </div>
          </div>

          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" onclick={() => (showLiabilityModal = false)} disabled={isSavingLiability}>
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" disabled={isSavingLiability}>
              {isSavingLiability ? 'Saving to Ledger...' : (modalLiabilityMode === 'create' ? 'Create Liability' : 'Save Changes')}
            </button>
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
      <div class="modal-dialog" style="max-width: 480px;">
        <div class="modal-header">
          <div>
            <h3 class="modal-title">Confirm Permanent Removal</h3>
            <p class="modal-subtitle">Removing {deleteTargetType} from corporate treasury database</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => (showDeleteModal = false)} aria-label="Close modal">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="4" y1="4" x2="12" y2="12" stroke-linecap="round" />
              <line x1="12" y1="4" x2="4" y2="12" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        <div class="modal-body">
          <p style="font-size: 0.9375rem; color: var(--color-text); line-height: 1.5;">
            Are you sure you want to remove <strong>{deleteTargetName}</strong> (<code>{deleteTargetId}</code>) from this portfolio?
          </p>
          <p style="font-size: 0.8125rem; color: var(--color-text-muted); margin-top: 0.75rem;">
            This will permanently delete the record from PostgreSQL and update all future capital allocation runs.
          </p>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn btn-secondary" onclick={() => (showDeleteModal = false)} disabled={isDeleting}>
            Cancel
          </button>
          <button type="button" class="btn btn-danger" onclick={executeDelete} disabled={isDeleting}>
            {isDeleting ? 'Deleting...' : 'Confirm Delete'}
          </button>
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
            <p class="modal-subtitle">Modify portfolio metadata and governance parameters</p>
          </div>
          <button type="button" class="modal-close-btn" onclick={() => handleCloseEditTreasury()} aria-label="Close modal">
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="2" width="14" height="14">
              <line x1="4" y1="4" x2="12" y2="12" stroke-linecap="round" />
              <line x1="12" y1="4" x2="4" y2="12" stroke-linecap="round" />
            </svg>
          </button>
        </div>

        {#if showUnsavedPrompt}
          <div class="alert alert-warning" style="margin: 1rem 1.5rem 0 1.5rem; display: flex; justify-content: space-between; align-items: center;">
            <span>You have unsaved changes. Discard them?</span>
            <div style="display: flex; gap: 0.5rem;">
              <button type="button" class="btn btn-secondary btn-sm" onclick={() => (showUnsavedPrompt = false)}>
                Keep Editing
              </button>
              <button type="button" class="btn btn-danger btn-sm" onclick={() => handleCloseEditTreasury(true)}>
                Discard
              </button>
            </div>
          </div>
        {/if}

        <form onsubmit={handleSaveTreasury} novalidate>
          <div class="modal-body">
            {#if editTreasuryErrors.length > 0}
              <div class="alert alert-danger" style="margin-bottom: 1rem;">
                <ul style="padding-left: 1.25rem; margin: 0;">
                  {#each editTreasuryErrors as err}
                    <li>{err}</li>
                  {/each}
                </ul>
              </div>
            {/if}

            <div class="form-group" style="margin-bottom: 1rem;">
              <div style="display: flex; justify-content: space-between; align-items: baseline;">
                <label for="edit-treasury-name" class="form-label">Treasury Name *</label>
                <span style="font-size: 0.75rem; color: var(--color-text-muted);">{editTreasuryForm.name.length}/255</span>
              </div>
              <input
                id="edit-treasury-name"
                type="text"
                class="form-input"
                bind:value={editTreasuryForm.name}
                maxlength="255"
                placeholder="e.g. Apex Global Liquid Treasury"
                required
              />
            </div>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1rem;">
              <div class="form-group">
                <label for="edit-treasury-currency" class="form-label">Base Currency *</label>
                <select
                  id="edit-treasury-currency"
                  class="form-select"
                  bind:value={editTreasuryForm.base_currency}
                >
                  <option value="INR">INR - Indian Rupee</option>
                  <option value="USD">USD - US Dollar</option>
                  <option value="EUR">EUR - Euro</option>
                  <option value="GBP">GBP - British Pound</option>
                  <option value="SGD">SGD - Singapore Dollar</option>
                  <option value="JPY">JPY - Japanese Yen</option>
                </select>
              </div>

              <div class="form-group">
                <label for="edit-treasury-capital" class="form-label">Total Capital Base *</label>
                <input
                  id="edit-treasury-capital"
                  type="number"
                  step="any"
                  min="1"
                  class="form-input"
                  bind:value={editTreasuryForm.total_capital}
                  placeholder="e.g. 100000000"
                  required
                />
              </div>
            </div>

            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
              <div class="form-group">
                <label for="edit-treasury-risk" class="form-label">Risk Tolerance (0.00 - 1.00) *</label>
                <input
                  id="edit-treasury-risk"
                  type="number"
                  step="0.01"
                  min="0.00"
                  max="1.00"
                  class="form-input"
                  bind:value={editTreasuryForm.risk_tolerance}
                  placeholder="0.50"
                  required
                />
              </div>

              <div class="form-group">
                <label for="edit-treasury-liquidity" class="form-label">Min Liquidity Floor (0.00 - 1.00) *</label>
                <input
                  id="edit-treasury-liquidity"
                  type="number"
                  step="0.01"
                  min="0.00"
                  max="1.00"
                  class="form-input"
                  bind:value={editTreasuryForm.min_liquidity_ratio}
                  placeholder="0.20"
                  required
                />
              </div>
            </div>
          </div>

          <div class="modal-footer">
            <button
              type="button"
              class="btn btn-secondary"
              onclick={() => handleCloseEditTreasury()}
              disabled={isSavingTreasury}
            >
              Cancel
            </button>
            <button
              type="submit"
              class="btn btn-primary"
              disabled={isSavingTreasury || !isTreasuryFormDirty}
            >
              {isSavingTreasury ? 'Saving Changes...' : 'Save Changes'}
            </button>
          </div>
        </form>
      </div>
    </div>
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

  /* Executive Balance Sheet Summary Row */
  .balance-sheet-kpis {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
    gap: 1rem;
    margin-bottom: 2rem;
  }

  .kpi-card {
    background-color: var(--color-surface, #ffffff);
    border: 1px solid var(--color-border, #e2e8f0);
    border-radius: var(--radius, 8px);
    padding: 1.125rem 1.25rem;
  }

  .kpi-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.35rem;
  }

  .kpi-category {
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--color-text-muted, #64748b);
    text-transform: uppercase;
  }

  .currency-tag {
    font-size: 0.6875rem;
    font-weight: 700;
    padding: 0.15rem 0.4rem;
    background-color: var(--color-bg, #f8fafc);
    border: 1px solid var(--color-border, #e2e8f0);
    border-radius: 4px;
    color: var(--color-text-muted, #64748b);
  }

  .kpi-value {
    font-size: 1.375rem;
    font-weight: 700;
    color: var(--color-text, #0f172a);
    line-height: 1.2;
    font-variant-numeric: tabular-nums;
  }

  .kpi-subtext {
    font-size: 0.75rem;
    color: var(--color-text-muted, #64748b);
    margin-top: 0.35rem;
  }

  .val-positive {
    color: #059669;
  }

  .status-indicator-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }

  .dot-positive {
    background-color: #059669;
  }

  .dot-negative {
    background-color: #e11d48;
  }

  /* Institutional Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(15, 23, 42, 0.6);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
    padding: 1.5rem;
  }

  .modal-dialog {
    background-color: var(--color-surface, #ffffff);
    border: 1px solid var(--color-border, #e2e8f0);
    border-radius: var(--radius, 8px);
    width: 100%;
    max-width: 580px;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 10px 10px -5px rgba(0, 0, 0, 0.04);
  }

  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    padding: 1.25rem 1.5rem;
    border-bottom: 1px solid var(--color-border, #e2e8f0);
  }

  .modal-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-text, #0f172a);
  }

  .modal-subtitle {
    font-size: 0.8125rem;
    color: var(--color-text-muted, #64748b);
    margin-top: 0.15rem;
  }

  .modal-close-btn {
    background: none;
    border: none;
    color: var(--color-text-muted, #64748b);
    cursor: pointer;
    font-size: 1.25rem;
    padding: 0.25rem;
    line-height: 1;
    border-radius: 4px;
  }

  .modal-close-btn:hover {
    color: var(--color-text, #0f172a);
    background-color: var(--color-bg, #f8fafc);
  }

  .modal-body {
    padding: 1.25rem 1.5rem;
  }

  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 1rem 1.5rem;
    border-top: 1px solid var(--color-border, #e2e8f0);
    background-color: var(--color-bg, #f8fafc);
  }
</style>
