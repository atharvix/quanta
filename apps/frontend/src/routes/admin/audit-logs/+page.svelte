<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { user } from '$lib/auth';
  import { isAdmin } from '$lib/roles';
  import { api, ApiError } from '$lib/api';
  import type { AuditLogRecord, AuditLogsListResponse } from '$lib/types';
  import { formatDate } from '$lib/format';

  let logs = $state<AuditLogRecord[]>([]);
  let isLoading = $state(true);
  let errorMessage = $state<string | null>(null);

  const limit = 20;
  let offset = $state(0);
  let selectedAction = $state('');
  let selectedEntityType = $state('');

  const actionFilterOptions = [
    { value: '', label: 'All actions' },
    { value: 'auth.registered', label: 'auth.registered' },
    { value: 'auth.login', label: 'auth.login' },
    { value: 'auth.login_failed', label: 'auth.login_failed' },
    { value: 'user.role_changed', label: 'user.role_changed' },
    { value: 'treasury.created', label: 'treasury.created' },
    { value: 'strategy.generated', label: 'strategy.generated' },
    { value: 'financial_problem.created', label: 'financial_problem.created' },
    { value: 'orchestration.completed', label: 'orchestration.completed' },
    { value: 'orchestration.failed', label: 'orchestration.failed' }
  ];

  const entityTypeFilterOptions = [
    { value: '', label: 'All entities' },
    { value: 'user', label: 'user' },
    { value: 'treasury', label: 'treasury' },
    { value: 'financial_problem', label: 'financial_problem' },
    { value: 'orchestration_run', label: 'orchestration_run' },
    { value: 'strategy_execution_log', label: 'strategy_execution_log' }
  ];

  async function fetchAuditLogs(targetOffset: number) {
    isLoading = true;
    errorMessage = null;
    try {
      let query = `/api/v1/audit-logs?limit=${limit}&offset=${targetOffset}`;
      if (selectedAction) query += `&action=${encodeURIComponent(selectedAction)}`;
      if (selectedEntityType) query += `&entity_type=${encodeURIComponent(selectedEntityType)}`;

      const res = await api<AuditLogsListResponse>(query);
      logs = res.data || [];
      offset = targetOffset;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        errorMessage = err.status === 403
          ? 'Forbidden: Administrative privileges required to inspect audit logs.'
          : err.message || 'Failed to retrieve system audit records.';
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = 'An unexpected error occurred while loading audit trail.';
      }
    } finally {
      isLoading = false;
    }
  }

  function handleFilterSubmit(e: SubmitEvent) {
    e.preventDefault();
    fetchAuditLogs(0);
  }

  function resetFilters() {
    selectedAction = '';
    selectedEntityType = '';
    fetchAuditLogs(0);
  }

  function formatMetadata(raw: any): Array<{ key: string; value: string }> {
    if (!raw) return [];
    let obj = raw;
    if (typeof raw === 'string') {
      try {
        obj = JSON.parse(raw);
      } catch {
        return [{ key: 'raw', value: raw }];
      }
    }
    if (typeof obj === 'object' && obj !== null) {
      return Object.entries(obj).map(([k, v]) => ({
        key: k,
        value: typeof v === 'object' ? JSON.stringify(v) : String(v)
      }));
    }
    return [{ key: 'value', value: String(obj) }];
  }

  onMount(() => {
    if (!isAdmin($user?.role)) {
      goto('/');
      return;
    }
    fetchAuditLogs(0);
  });
</script>

<svelte:head>
  <title>Audit Log — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Audit Log</h1>
      <p>Append-only chronological record of authentication, governance, and orchestration events.</p>
    </div>
    <div class="page-head-actions">
      <button type="button" class="btn btn-secondary btn-sm" onclick={() => fetchAuditLogs(offset)} disabled={isLoading}>
        Refresh
      </button>
    </div>
  </header>

  {#if errorMessage}
    <div class="alert alert-danger row-between">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-xs" onclick={() => fetchAuditLogs(offset)}>Retry</button>
    </div>
  {/if}

  <form onsubmit={handleFilterSubmit} class="filter-strip">
    <div class="filter-field">
      <label for="actionFilter" class="form-label">Action</label>
      <select id="actionFilter" class="form-select" bind:value={selectedAction} disabled={isLoading}>
        {#each actionFilterOptions as opt}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </div>

    <div class="filter-field">
      <label for="entityTypeFilter" class="form-label">Entity type</label>
      <select id="entityTypeFilter" class="form-select" bind:value={selectedEntityType} disabled={isLoading}>
        {#each entityTypeFilterOptions as opt}
          <option value={opt.value}>{opt.label}</option>
        {/each}
      </select>
    </div>

    <div class="filter-btns">
      <button type="submit" class="btn btn-primary btn-sm" disabled={isLoading}>Apply</button>
      {#if selectedAction || selectedEntityType}
        <button type="button" class="btn btn-secondary btn-sm" disabled={isLoading} onclick={resetFilters}>Clear</button>
      {/if}
    </div>
  </form>

  {#if isLoading}
    <div class="empty-msg">Querying audit records...</div>
  {:else if logs.length === 0}
    <div class="empty-msg">
      <p>
        {#if selectedAction || selectedEntityType}
          No records match the selected filters.
        {:else if offset > 0}
          No additional audit records on this page.
        {:else}
          The audit ledger is empty.
        {/if}
      </p>
      {#if selectedAction || selectedEntityType}
        <button type="button" class="btn btn-secondary btn-sm" style="margin-top: 0.75rem;" onclick={resetFilters}>Reset filters</button>
      {/if}
    </div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Action</th>
            <th>Actor</th>
            <th>Entity</th>
            <th>Target ID</th>
            <th>Metadata</th>
          </tr>
        </thead>
        <tbody>
          {#each logs as log (log.id)}
            <tr>
              <td class="muted-cell" style="white-space: nowrap;">{formatDate(log.created_at)}</td>
              <td class="font-mono font-bold">{log.action}</td>
              <td class="font-mono">{log.actor}</td>
              <td class="font-mono muted-cell">{log.entity_type}</td>
              <td class="font-mono muted-cell" title={log.entity_id || ''}>
                {#if log.entity_id}
                  {log.entity_id.length > 12 ? `${log.entity_id.slice(0, 8)}...` : log.entity_id}
                {:else}
                  —
                {/if}
              </td>
              <td class="meta-cell">
                {#each formatMetadata(log.metadata) as meta, i}
                  <span class="meta-pair font-mono">{meta.key}={meta.value}{i < formatMetadata(log.metadata).length - 1 ? ', ' : ''}</span>
                {:else}
                  <span class="muted-cell">—</span>
                {/each}
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <span class="pagination-info">Showing {offset + 1} to {offset + logs.length}</span>
      <div class="pagination-actions">
        <button type="button" class="btn btn-secondary btn-sm" disabled={offset === 0 || isLoading} onclick={() => fetchAuditLogs(Math.max(0, offset - limit))}>Previous</button>
        <button type="button" class="btn btn-secondary btn-sm" disabled={logs.length < limit || isLoading} onclick={() => fetchAuditLogs(offset + limit)}>Next</button>
      </div>
    </div>
  {/if}
</div>

<style>
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .filter-strip {
    display: flex;
    align-items: flex-end;
    gap: 1rem;
    flex-wrap: wrap;
    padding: 0.875rem 1rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
  }

  .filter-field {
    min-width: 210px;
    flex: 1;
    max-width: 280px;
  }

  .filter-field .form-label {
    margin-bottom: 0.3rem;
  }

  .filter-btns {
    display: flex;
    gap: 0.5rem;
  }

  .muted-cell {
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .meta-cell {
    max-width: 360px;
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    word-break: break-word;
  }

  .empty-msg {
    padding: 2.25rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }
</style>
