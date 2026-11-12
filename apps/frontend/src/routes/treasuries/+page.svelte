<script lang="ts">
  import { onMount } from 'svelte';
  import { user } from '$lib/auth';
  import { canWrite } from '$lib/roles';
  import { api, ApiError } from '$lib/api';
  import type { Treasury, TreasuriesListResponse } from '$lib/types';
  import { formatCurrency, formatPercent, formatDate } from '$lib/format';

  let treasuries = $state<Treasury[]>([]);
  let isLoading = $state(true);
  let errorMessage = $state<string | null>(null);

  const limit = 10;
  let offset = $state(0);

  let showCreateForm = $state(false);
  let createSuccessMessage = $state<string | null>(null);

  let name = $state('');
  let baseCurrency = $state('INR');
  let totalCapital = $state('');
  let riskTolerance = $state('0.50');
  let minLiquidityRatio = $state('0.20');
  let isCreating = $state(false);
  let formError = $state<string | null>(null);
  let formDetails = $state<string[]>([]);
  let fieldErrors = $state<{
    name?: string;
    totalCapital?: string;
    riskTolerance?: string;
    minLiquidityRatio?: string;
  }>({});

  async function fetchTreasuries(targetOffset: number) {
    isLoading = true;
    errorMessage = null;

    try {
      const res = await api<TreasuriesListResponse>(
        `/api/v1/treasuries?limit=${limit}&offset=${targetOffset}`
      );
      treasuries = res.data || [];
      offset = targetOffset;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        errorMessage = err.message || 'Failed to retrieve treasuries.';
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = 'An unexpected error occurred while fetching treasuries.';
      }
    } finally {
      isLoading = false;
    }
  }

  function validateCreate(): boolean {
    const errors: typeof fieldErrors = {};

    if (!name.trim()) {
      errors.name = 'Treasury name is required.';
    }

    const cap = parseFloat(totalCapital);
    if (!totalCapital.trim() || isNaN(cap) || cap <= 0) {
      errors.totalCapital = 'Total capital must be greater than 0.';
    }

    const risk = parseFloat(riskTolerance);
    if (!riskTolerance.trim() || isNaN(risk) || risk < 0 || risk > 1) {
      errors.riskTolerance = 'Risk tolerance must be between 0.0 and 1.0.';
    }

    const liq = parseFloat(minLiquidityRatio);
    if (!minLiquidityRatio.trim() || isNaN(liq) || liq < 0 || liq > 1) {
      errors.minLiquidityRatio = 'Minimum liquidity ratio must be between 0.0 and 1.0.';
    }

    fieldErrors = errors;
    return Object.keys(errors).length === 0;
  }

  async function handleCreate(e: SubmitEvent) {
    e.preventDefault();
    formError = null;
    formDetails = [];
    createSuccessMessage = null;

    if (!validateCreate()) {
      return;
    }

    isCreating = true;

    try {
      const created = await api<Treasury>('/api/v1/treasuries', {
        method: 'POST',
        body: {
          name: name.trim(),
          base_currency: (baseCurrency.trim() || 'INR').toUpperCase(),
          total_capital: parseFloat(totalCapital),
          risk_tolerance: parseFloat(riskTolerance),
          min_liquidity_ratio: parseFloat(minLiquidityRatio)
        }
      });

      createSuccessMessage = `Treasury "${created.name}" onboarded successfully.`;
      name = '';
      totalCapital = '';
      riskTolerance = '0.50';
      minLiquidityRatio = '0.20';
      showCreateForm = false;
      await fetchTreasuries(0);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        formError = err.message || 'Validation failed.';
        if (err.details && err.details.length > 0) {
          formDetails = err.details;
        }
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'Failed to create corporate treasury.';
      }
    } finally {
      isCreating = false;
    }
  }

  onMount(() => {
    fetchTreasuries(0);
  });
</script>

<div class="page-container">
  <div class="page-header">
    <div>
      <h1 class="page-title">Corporate Treasuries</h1>
      <p class="page-subtitle">Multi-currency balance sheets under autonomous capital allocation</p>
    </div>

    <div>
      {#if canWrite($user?.role)}
        <button
          type="button"
          class="btn btn-primary"
          onclick={() => (showCreateForm = !showCreateForm)}
        >
          {showCreateForm ? 'Cancel' : 'New Treasury'}
        </button>
      {/if}
    </div>
  </div>

  {#if !canWrite($user?.role)}
    <div class="notice-banner" style="margin-bottom: 1.5rem;">
      Read-only access: Your current role ({$user?.role || 'viewer'}) permits inspection only. Analyst or Admin credentials are required to onboard new balance sheets.
    </div>
  {/if}

  {#if createSuccessMessage}
    <div class="alert alert-success">
      {createSuccessMessage}
    </div>
  {/if}

  {#if showCreateForm && canWrite($user?.role)}
    <div class="card create-card">
      <h2 class="card-title">Onboard New Corporate Treasury</h2>
      <p class="card-subtitle">Define capital base, reference currency, and policy constraints.</p>

      {#if formError}
        <div class="alert alert-danger" style="margin-top: 1rem;">
          <p><strong>{formError}</strong></p>
          {#if formDetails.length > 0}
            <ul style="margin-top: 0.5rem; padding-left: 1.25rem;">
              {#each formDetails as d}
                <li>{d}</li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}

      <form onsubmit={handleCreate} novalidate style="margin-top: 1.25rem;">
        <div class="form-grid">
          <div class="form-group">
            <label for="treasuryName" class="form-label">Treasury Name</label>
            <input
              id="treasuryName"
              type="text"
              class="form-input"
              class:has-error={!!fieldErrors.name}
              placeholder="e.g. Acme Corp Treasury"
              bind:value={name}
              disabled={isCreating}
            />
            {#if fieldErrors.name}
              <p class="field-error">{fieldErrors.name}</p>
            {/if}
          </div>

          <div class="form-group">
            <label for="baseCurrency" class="form-label">Base Currency</label>
            <input
              id="baseCurrency"
              type="text"
              class="form-input"
              placeholder="INR"
              bind:value={baseCurrency}
              disabled={isCreating}
            />
          </div>

          <div class="form-group">
            <label for="totalCapital" class="form-label">Total Capital</label>
            <input
              id="totalCapital"
              type="number"
              step="any"
              min="0.01"
              class="form-input"
              class:has-error={!!fieldErrors.totalCapital}
              placeholder="e.g. 10000000"
              bind:value={totalCapital}
              disabled={isCreating}
            />
            {#if fieldErrors.totalCapital}
              <p class="field-error">{fieldErrors.totalCapital}</p>
            {/if}
          </div>

          <div class="form-group">
            <label for="riskTolerance" class="form-label">Risk Tolerance (0.00 to 1.00)</label>
            <input
              id="riskTolerance"
              type="number"
              step="0.05"
              min="0"
              max="1"
              class="form-input"
              class:has-error={!!fieldErrors.riskTolerance}
              placeholder="0.50"
              bind:value={riskTolerance}
              disabled={isCreating}
            />
            {#if fieldErrors.riskTolerance}
              <p class="field-error">{fieldErrors.riskTolerance}</p>
            {/if}
          </div>

          <div class="form-group">
            <label for="minLiquidityRatio" class="form-label">Min Liquidity Ratio (0.00 to 1.00)</label>
            <input
              id="minLiquidityRatio"
              type="number"
              step="0.05"
              min="0"
              max="1"
              class="form-input"
              class:has-error={!!fieldErrors.minLiquidityRatio}
              placeholder="0.20"
              bind:value={minLiquidityRatio}
              disabled={isCreating}
            />
            {#if fieldErrors.minLiquidityRatio}
              <p class="field-error">{fieldErrors.minLiquidityRatio}</p>
            {/if}
          </div>
        </div>

        <div style="display: flex; gap: 0.75rem; margin-top: 1rem;">
          <button type="submit" class="btn btn-primary" disabled={isCreating}>
            {isCreating ? 'Onboarding...' : 'Save Treasury'}
          </button>
          <button
            type="button"
            class="btn btn-secondary"
            disabled={isCreating}
            onclick={() => (showCreateForm = false)}
          >
            Cancel
          </button>
        </div>
      </form>
    </div>
  {/if}

  {#if errorMessage}
    <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center;">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-sm" onclick={() => fetchTreasuries(offset)}>
        Retry
      </button>
    </div>
  {/if}

  {#if isLoading}
    <div class="card empty-state">
      <p class="state-message">Loading corporate treasuries...</p>
    </div>
  {:else if treasuries.length === 0}
    <div class="card empty-state">
      <h3 class="state-title">No Treasuries Found</h3>
      <p class="state-message">
        {#if offset > 0}
          No additional treasuries exist on this page.
        {:else}
          No corporate treasuries have been onboarded yet.
        {/if}
      </p>
      {#if offset > 0}
        <button type="button" class="btn btn-secondary btn-sm" style="margin-top: 1rem;" onclick={() => fetchTreasuries(0)}>
          Return to First Page
        </button>
      {/if}
    </div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Base Currency</th>
            <th>Total Capital</th>
            <th>Risk Tolerance</th>
            <th>Min Liquidity</th>
            <th>Created Date</th>
          </tr>
        </thead>
        <tbody>
          {#each treasuries as t (t.id)}
            <tr>
              <td>
                <a href="/treasuries/{t.id}" class="treasury-name-link">
                  {t.name}
                </a>
              </td>
              <td>
                <span class="currency-tag">{t.base_currency}</span>
              </td>
              <td class="numeric-cell">
                {formatCurrency(t.total_capital, t.base_currency)}
              </td>
              <td>
                {formatPercent(t.risk_tolerance)}
              </td>
              <td>
                {formatPercent(t.min_liquidity_ratio)}
              </td>
              <td>
                {formatDate(t.created_at)}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <span class="pagination-info">
        Showing records {offset + 1} to {offset + treasuries.length}
      </span>
      <div class="pagination-actions">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          disabled={offset === 0 || isLoading}
          onclick={() => fetchTreasuries(Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          disabled={treasuries.length < limit || isLoading}
          onclick={() => fetchTreasuries(offset + limit)}
        >
          Next
        </button>
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

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1.5rem;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .page-title {
    font-size: 1.75rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .create-card {
    margin-bottom: 2rem;
  }

  .card-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .card-subtitle {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
    margin-top: 0.125rem;
  }

  .form-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
    gap: 1rem;
  }

  .treasury-name-link {
    font-weight: 600;
    color: var(--color-primary);
  }

  .treasury-name-link:hover {
    text-decoration: underline;
  }

  .currency-tag {
    display: inline-block;
    padding: 0.15rem 0.4rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.75rem;
    font-weight: 600;
  }

  .numeric-cell {
    font-variant-numeric: tabular-nums;
    font-weight: 500;
  }

  .empty-state {
    text-align: center;
    padding: 3rem 1.5rem;
  }

  .state-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .state-message {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }
</style>

