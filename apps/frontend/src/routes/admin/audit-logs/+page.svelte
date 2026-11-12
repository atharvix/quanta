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

  // Backend-supported filters
  let selectedAction = $state('');
  let selectedEntityType = $state('');

  const actionFilterOptions = [
    { value: '', label: 'All Actions' },
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
    { value: '', label: 'All Entity Types' },
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
      if (selectedAction) {
        query += `&action=${encodeURIComponent(selectedAction)}`;
      }
      if (selectedEntityType) {
        query += `&entity_type=${encodeURIComponent(selectedEntityType)}`;
      }

      const res = await api<AuditLogsListResponse>(query);
      logs = res.data || [];
      offset = targetOffset;
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 403) {
          errorMessage = 'Forbidden: Administrative privileges required to inspect audit logs.';
        } else {
          errorMessage = err.message || 'Failed to retrieve system audit records.';
        }
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

<div class="page-container">
  <div class="page-header">
    <div>
      <h1 class="page-title">Security & Audit Logs</h1>
      <p class="page-subtitle">Immutable append-only ledger tracking all authentication, governance, and capital allocation events</p>
    </div>
    <div>
      <button type="button" class="btn btn-secondary btn-sm" onclick={() => fetchAuditLogs(offset)} disabled={isLoading}>
        Refresh
      </button>
    </div>
  </div>

  {#if errorMessage}
    <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center;">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-sm" onclick={() => fetchAuditLogs(offset)}>
        Retry
      </button>
    </div>
  {/if}

  <!-- Filter Controls Card -->
  <div class="card filter-card">
    <form onsubmit={handleFilterSubmit} class="filter-form">
      <div class="filter-grid">
        <div class="form-group" style="margin-bottom: 0;">
          <label for="actionFilter" class="form-label">Action Filter</label>
          <select
            id="actionFilter"
            class="form-input"
            bind:value={selectedAction}
            disabled={isLoading}
          >
            {#each actionFilterOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
        </div>

        <div class="form-group" style="margin-bottom: 0;">
          <label for="entityTypeFilter" class="form-label">Entity Type</label>
          <select
            id="entityTypeFilter"
            class="form-input"
            bind:value={selectedEntityType}
            disabled={isLoading}
          >
            {#each entityTypeFilterOptions as opt}
              <option value={opt.value}>{opt.label}</option>
            {/each}
          </select>
        </div>

        <div class="filter-actions">
          <button type="submit" class="btn btn-primary" disabled={isLoading}>
            Apply Filters
          </button>
          {#if selectedAction || selectedEntityType}
            <button
              type="button"
              class="btn btn-secondary"
              disabled={isLoading}
              onclick={resetFilters}
            >
              Clear Filters
            </button>
          {/if}
        </div>
      </div>
    </form>
  </div>

  <!-- Audit Table -->
  {#if isLoading}
    <div class="card empty-state">
      <p class="state-message">Querying immutable audit logs...</p>
    </div>
  {:else if logs.length === 0}
    <div class="card empty-state">
      <h3 class="state-title">No Audit Records Found</h3>
      <p class="state-message">
        {#if selectedAction || selectedEntityType}
          No audit records match the applied filter criteria. Try clearing filters.
        {:else if offset > 0}
          No further audit records on this page.
        {:else}
          The audit ledger is currently empty.
        {/if}
      </p>
      {#if selectedAction || selectedEntityType}
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          style="margin-top: 1rem;"
          onclick={resetFilters}
        >
          Reset Filters
        </button>
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
            <th>Entity Type</th>
            <th>Entity ID</th>
            <th>Metadata Details</th>
          </tr>
        </thead>
        <tbody>
          {#each logs as log (log.id)}
            <tr>
              <td class="timestamp-cell">
                {formatDate(log.created_at)}
              </td>
              <td>
                <span class="action-badge">{log.action}</span>
              </td>
              <td>
                <span class="actor-cell" title={log.actor}>
                  <code>{log.actor}</code>
                </span>
              </td>
              <td>
                <span class="entity-type-badge">{log.entity_type}</span>
              </td>
              <td>
                {#if log.entity_id}
                  <code class="entity-id-text" title={log.entity_id}>
                    {log.entity_id.length > 12 ? `${log.entity_id.slice(0, 8)}...` : log.entity_id}
                  </code>
                {:else}
                  <span style="color: var(--color-text-muted); font-size: 0.75rem;">-</span>
                {/if}
              </td>
              <td>
                <div class="metadata-wrap">
                  {#each formatMetadata(log.metadata) as meta}
                    <span class="tag-pill">
                      <strong>{meta.key}:</strong> {meta.value}
                    </span>
                  {:else}
                    <span style="color: var(--color-text-muted); font-size: 0.75rem;">-</span>
                  {/each}
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="pagination">
      <span class="pagination-info">
        Showing records {offset + 1} to {offset + logs.length}
      </span>
      <div class="pagination-actions">
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          disabled={offset === 0 || isLoading}
          onclick={() => fetchAuditLogs(Math.max(0, offset - limit))}
        >
          Previous
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          disabled={logs.length < limit || isLoading}
          onclick={() => fetchAuditLogs(offset + limit)}
        >
          Next
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-container {
    max-width: 1200px;
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

  .filter-card {
    margin-bottom: 1.5rem;
    padding: 1.25rem;
  }

  .filter-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)) auto;
    gap: 1rem;
    align-items: flex-end;
  }

  .filter-actions {
    display: flex;
    gap: 0.5rem;
  }

  .timestamp-cell {
    font-size: 0.8125rem;
    white-space: nowrap;
    color: var(--color-text-muted);
  }

  .action-badge {
    display: inline-block;
    padding: 0.2rem 0.5rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.75rem;
    font-family: monospace;
    font-weight: 600;
    color: var(--color-text);
  }

  .actor-cell code {
    font-size: 0.8125rem;
    color: var(--color-text);
  }

  .entity-type-badge {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    background-color: var(--color-badge-viewer-bg);
    color: var(--color-badge-viewer);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    font-weight: 600;
  }

  .entity-id-text {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .metadata-wrap {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
    max-width: 380px;
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

