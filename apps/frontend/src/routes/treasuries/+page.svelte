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
    if (!name.trim()) errors.name = 'Treasury name is required.';

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
    if (!validateCreate()) return;

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

      showCreateForm = false;
      name = '';
      baseCurrency = 'INR';
      totalCapital = '';
      riskTolerance = '0.50';
      minLiquidityRatio = '0.20';
      fieldErrors = {};
      createSuccessMessage = `Treasury "${created.name}" created.`;

      await fetchTreasuries(0);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        formError = err.status === 403
          ? 'Access denied: insufficient permissions to create treasuries.'
          : err.message || 'Failed to create treasury.';
        if (err.details?.length) formDetails = err.details;
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'An unexpected error occurred while creating the treasury.';
      }
    } finally {
      isCreating = false;
    }
  }

  onMount(() => {
    fetchTreasuries(0);
  });
</script>

<svelte:head>
  <title>Treasuries — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Corporate Treasuries</h1>
      <p>Multi-currency portfolios with role-scoped isolation and constraint policies.</p>
    </div>

    <div class="page-head-actions">
      {#if canWrite($user?.role)}
        <button
          type="button"
          class="btn btn-primary btn-sm"
          onclick={() => (showCreateForm = !showCreateForm)}
        >
          {showCreateForm ? 'Cancel' : 'New treasury'}
        </button>
      {/if}
    </div>
  </header>

  {#if !canWrite($user?.role)}
    <div class="notice-banner">
      Read-only access: your role ({$user?.role || 'viewer'}) permits inspection only.
    </div>
  {/if}

  {#if createSuccessMessage}
    <div class="alert alert-success">{createSuccessMessage}</div>
  {/if}

  {#if showCreateForm && canWrite($user?.role)}
    <div class="surface-panel">
      <div class="panel-title-row">
        <h2 class="section-title">Create corporate treasury</h2>
        <p class="section-subtitle">Define capital base, reporting currency, and policy bounds.</p>
      </div>

      {#if formError}
        <div class="alert alert-danger" style="margin-top: 0.875rem;">
          <p><strong>{formError}</strong></p>
          {#if formDetails.length > 0}
            <ul style="margin-top: 0.4rem; padding-left: 1.25rem;">
              {#each formDetails as d}
                <li>{d}</li>
              {/each}
            </ul>
          {/if}
        </div>
      {/if}

      <form onsubmit={handleCreate} novalidate style="margin-top: 1rem;">
        <div class="form-grid">
          <div class="form-group">
            <label for="treasuryName" class="form-label">Treasury name</label>
            <input
              id="treasuryName"
              type="text"
              class="form-input"
              class:has-error={!!fieldErrors.name}
              placeholder="Meridian Treasury"
              bind:value={name}
              disabled={isCreating}
            />
            {#if fieldErrors.name}
              <p class="field-error">{fieldErrors.name}</p>
            {/if}
          </div>

          <div class="form-group">
            <label for="baseCurrency" class="form-label">Currency</label>
            <input
              id="baseCurrency"
              type="text"
              class="form-input font-mono"
              placeholder="USD"
              bind:value={baseCurrency}
              disabled={isCreating}
            />
          </div>

          <div class="form-group">
            <label for="totalCapital" class="form-label">Total capital</label>
            <input
              id="totalCapital"
              type="number"
              step="any"
              min="0.01"
              class="form-input font-mono"
              class:has-error={!!fieldErrors.totalCapital}
              placeholder="10000000"
              bind:value={totalCapital}
              disabled={isCreating}
            />
            {#if fieldErrors.totalCapital}
              <p class="field-error">{fieldErrors.totalCapital}</p>
            {/if}
          </div>

          <div class="form-group">
            <label for="riskTolerance" class="form-label">Risk tolerance (0.00 to 1.00)</label>
            <input
              id="riskTolerance"
              type="number"
              step="0.05"
              min="0"
              max="1"
              class="form-input font-mono"
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
            <label for="minLiquidityRatio" class="form-label">Min liquidity ratio (0.00 to 1.00)</label>
            <input
              id="minLiquidityRatio"
              type="number"
              step="0.05"
              min="0"
              max="1"
              class="form-input font-mono"
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

        <div class="form-actions">
          <button type="submit" class="btn btn-primary btn-sm" disabled={isCreating}>
            {isCreating ? 'Creating...' : 'Save treasury'}
          </button>
          <button type="button" class="btn btn-secondary btn-sm" disabled={isCreating} onclick={() => (showCreateForm = false)}>
            Cancel
          </button>
        </div>
      </form>
    </div>
  {/if}

  {#if errorMessage}
    <div class="alert alert-danger row-between">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-xs" onclick={() => fetchTreasuries(offset)}>Retry</button>
    </div>
  {/if}

  {#if isLoading}
    <div class="empty-box">Loading corporate treasuries...</div>
  {:else if treasuries.length === 0}
    <div class="empty-box">
      <p>No corporate treasuries on record.</p>
      {#if offset > 0}
        <button type="button" class="btn btn-secondary btn-sm" style="margin-top: 0.75rem;" onclick={() => fetchTreasuries(0)}>
          Return to page 1
        </button>
      {/if}
    </div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Portfolio name</th>
            <th>Currency</th>
            <th class="text-right">Total capital</th>
            <th class="text-right">Risk tolerance</th>
            <th class="text-right">Min liquidity</th>
            <th class="text-right">Created</th>
          </tr>
        </thead>
        <tbody>
          {#each treasuries as t (t.id)}
            <tr>
              <td>
                <a href="/treasuries/{t.id}" class="portfolio-link">{t.name}</a>
              </td>
              <td class="font-mono">{t.base_currency}</td>
              <td class="text-right numeric-cell">{formatCurrency(t.total_capital, t.base_currency)}</td>
              <td class="text-right numeric-cell">{formatPercent(t.risk_tolerance)}</td>
              <td class="text-right numeric-cell">{formatPercent(t.min_liquidity_ratio)}</td>
              <td class="text-right muted-cell">{formatDate(t.created_at)}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <span class="pagination-info">Showing {offset + 1} to {offset + treasuries.length}</span>
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
  .form-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(210px, 1fr));
    gap: 0.875rem;
  }

  .form-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.75rem;
  }

  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .portfolio-link {
    font-weight: 500;
    color: var(--color-text);
  }

  .portfolio-link:hover {
    color: var(--color-primary);
    text-decoration: underline;
  }

  .muted-cell {
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .empty-box {
    padding: 2.5rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }
</style>
