<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import { user } from '$lib/auth';
  import { isAdmin } from '$lib/roles';
  import type { Treasury, TreasuriesListResponse, AuditLogRecord, AuditLogsListResponse } from '$lib/types';
  import { formatDate } from '$lib/format';
  import {
    generateBalanceSheetCSV,
    generateAuditTrailCSV,
    generateJSONReport
  } from '$lib/reports';

  let treasuries = $state<Treasury[]>([]);
  let auditLogs = $state<AuditLogRecord[]>([]);
  let isLoading = $state(true);
  let isExporting = $state(false);
  let exportProgress = $state<string | null>(null);
  let exportSuccess = $state<string | null>(null);
  let exportError = $state<string | null>(null);

  function getTimestampString(): string {
    const d = new Date();
    const pad = (n: number) => String(n).padStart(2, '0');
    const yyyy = d.getFullYear();
    const mm = pad(d.getMonth() + 1);
    const dd = pad(d.getDate());
    const hh = pad(d.getHours());
    const min = pad(d.getMinutes());
    const ss = pad(d.getSeconds());
    return `${yyyy}${mm}${dd}_${hh}${min}${ss}`;
  }

  /**
   * Fetches the complete paginated collection of corporate treasuries across all offsets.
   * Overcomes the 100-record per-request backend pagination constraint.
   */
  async function fetchAllTreasuries(): Promise<Treasury[]> {
    const pageSize = 100;
    let offset = 0;
    const allRecords: Treasury[] = [];

    while (true) {
      const res = await api<TreasuriesListResponse>(`/api/v1/treasuries?limit=${pageSize}&offset=${offset}`);
      const batch = Array.isArray(res.data) ? res.data : [];
      allRecords.push(...batch);

      if (batch.length < pageSize) {
        break;
      }
      offset += pageSize;
      if (offset > 50000) break; // Defensive boundary
    }

    return allRecords;
  }

  /**
   * Fetches the complete paginated collection of immutable audit logs across all offsets.
   * Overcomes the 100-record per-request backend pagination constraint.
   */
  async function fetchAllAuditLogs(): Promise<AuditLogRecord[]> {
    const pageSize = 100;
    let offset = 0;
    const allRecords: AuditLogRecord[] = [];

    while (true) {
      const res = await api<AuditLogsListResponse>(`/api/v1/audit-logs?limit=${pageSize}&offset=${offset}`);
      const batch = Array.isArray(res.data) ? res.data : [];
      allRecords.push(...batch);

      if (batch.length < pageSize) {
        break;
      }
      offset += pageSize;
      if (offset > 50000) break; // Defensive boundary
    }

    return allRecords;
  }

  async function loadData() {
    isLoading = true;
    exportError = null;
    try {
      treasuries = await fetchAllTreasuries();
      if (isAdmin($user?.role)) {
        auditLogs = await fetchAllAuditLogs();
      } else {
        auditLogs = [];
      }
    } catch (err: any) {
      exportError = `Failed to load reporting data: ${err?.message || 'Network error'}`;
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  function triggerDownload(content: string, filename: string, mimeType: string, recordCount: number) {
    try {
      const blob = new Blob([content], { type: mimeType });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.style.display = 'none';
      a.href = url;
      a.setAttribute('download', filename);
      document.body.appendChild(a);
      a.click();
      
      setTimeout(() => {
        if (document.body.contains(a)) {
          document.body.removeChild(a);
        }
        URL.revokeObjectURL(url);
      }, 1500);

      exportSuccess = `Successfully exported ${recordCount} records to ${filename}`;
      exportError = null;
      setTimeout(() => {
        exportSuccess = null;
      }, 5000);
    } catch (err: any) {
      exportError = `Download failed: ${err?.message || 'Unable to create file blob'}`;
      exportSuccess = null;
    }
  }

  async function exportBalanceSheetCSV() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = 'Fetching complete balance sheet schedule...';

    try {
      const allTreasuries = await fetchAllTreasuries();
      if (allTreasuries.length === 0) {
        exportError = 'Export failed: No balance sheet records available in database to export.';
        return;
      }

      treasuries = allTreasuries;
      const csvContent = generateBalanceSheetCSV(allTreasuries);
      const filename = `quanta_balance_sheet_${getTimestampString()}.csv`;
      triggerDownload(csvContent, filename, 'text/csv;charset=utf-8;', allTreasuries.length);
    } catch (err: any) {
      exportError = `Failed to generate balance sheet CSV: ${err?.message || 'Serialization error'}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportBalanceSheetJSON() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = 'Fetching complete balance sheet schedule...';

    try {
      const allTreasuries = await fetchAllTreasuries();
      if (allTreasuries.length === 0) {
        exportError = 'Export failed: No balance sheet records available in database to export.';
        return;
      }

      treasuries = allTreasuries;
      const jsonContent = generateJSONReport(allTreasuries);
      const filename = `quanta_balance_sheet_${getTimestampString()}.json`;
      triggerDownload(jsonContent, filename, 'application/json;charset=utf-8;', allTreasuries.length);
    } catch (err: any) {
      exportError = `Failed to generate balance sheet JSON: ${err?.message || 'Serialization error'}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportAuditLogsJSON() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = 'Fetching complete audit journal across all pages...';

    try {
      const allLogs = await fetchAllAuditLogs();
      if (allLogs.length === 0) {
        exportError = 'Export failed: No audit journal records available from security backend.';
        return;
      }

      auditLogs = allLogs;
      const jsonContent = generateJSONReport(allLogs);
      const filename = `quanta_audit_journal_${getTimestampString()}.json`;
      triggerDownload(jsonContent, filename, 'application/json;charset=utf-8;', allLogs.length);
    } catch (err: any) {
      exportError = `Failed to generate audit journal JSON: ${err?.message || 'Serialization error'}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportAuditLogsCSV() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = 'Fetching complete audit journal across all pages...';

    try {
      const allLogs = await fetchAllAuditLogs();
      if (allLogs.length === 0) {
        exportError = 'Export failed: No audit journal records available from security backend.';
        return;
      }

      auditLogs = allLogs;
      const csvContent = generateAuditTrailCSV(allLogs);
      const filename = `quanta_audit_journal_${getTimestampString()}.csv`;
      triggerDownload(csvContent, filename, 'text/csv;charset=utf-8;', allLogs.length);
    } catch (err: any) {
      exportError = `Failed to generate audit journal CSV: ${err?.message || 'Serialization error'}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }
</script>

<svelte:head>
  <title>Financial Reports - QUANTA</title>
</svelte:head>

<div class="page-root">
  <div class="page-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">REGULATORY &amp; COMPLIANCE REPORTING</span>
      </div>
      <h1 class="page-title">Financial Reports &amp; Statements</h1>
      <p class="page-subtitle">Complete un-truncated balance-sheet statements, liquidity coverage disclosures, and cryptographically verified audit journals.</p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-refresh" onclick={loadData} disabled={isLoading || isExporting}>
        {#if isLoading}
          <span class="spinner-sm"></span> Loading Complete Datasets...
        {:else}
          Refresh Datasets
        {/if}
      </button>
    </div>
  </div>

  {#if exportProgress}
    <div class="alert alert-info" role="status">
      <span class="spinner-sm"></span> {exportProgress}
    </div>
  {/if}

  {#if exportSuccess}
    <div class="alert alert-success" role="status">
      {exportSuccess}
    </div>
  {/if}

  {#if exportError}
    <div class="alert alert-error" role="alert">
      {exportError}
    </div>
  {/if}

  <!-- Available Reports Grid -->
  <div class="reports-grid">
    <!-- Report 1: Consolidated Balance Sheet -->
    <div class="report-card">
      <div class="report-badge">STATEMENT</div>
      <h2 class="report-title">Consolidated Balance Sheet Statement</h2>
      <p class="report-desc">
        Complete schedule of capital assets, revolving credit facilities, and liquidity allocations across all {treasuries.length} corporate portfolios without pagination limits.
      </p>
      <div class="report-meta">
        <span>Format: CSV or Structured JSON</span>
        <span>Scope: {treasuries.length} Portfolios Verified</span>
      </div>
      <div class="export-btn-group">
        <button
          type="button"
          class="btn-export"
          onclick={exportBalanceSheetCSV}
          disabled={isLoading || isExporting}
        >
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
            <path d="M2 10v3a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-3" />
            <path d="M8 2v8m-3-3l3 3 3-3" />
          </svg>
          <span>Export CSV</span>
        </button>
        <button
          type="button"
          class="btn-export btn-export-secondary"
          onclick={exportBalanceSheetJSON}
          disabled={isLoading || isExporting}
        >
          <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
            <path d="M2 10v3a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-3" />
            <path d="M8 2v8m-3-3l3 3 3-3" />
          </svg>
          <span>Export JSON</span>
        </button>
      </div>
    </div>

    <!-- Report 2: Security & Governance Audit Journal -->
    <div class="report-card">
      <div class="report-badge">COMPLIANCE</div>
      <h2 class="report-title">Security &amp; Audit Trail Ledger</h2>
      <p class="report-desc">
        Immutable chronological event journal tracking operator authentication, asset creation, and role updates. Full multi-page pagination guarantees complete retrieval.
      </p>
      <div class="report-meta">
        <span>Format: JSON Document or CSV</span>
        <span>
          {#if isAdmin($user?.role)}
            Scope: {auditLogs.length} Sealed Audit Records (100% Exported)
          {:else}
            Scope: Admin privileges required
          {/if}
        </span>
      </div>
      {#if !isAdmin($user?.role)}
        <p class="empty-state" style="padding: 0.75rem 0; text-align: left;">
          Audit trail exports are restricted to Admin users. Balance Sheet CSV &amp; JSON exports above remain fully available.
        </p>
      {:else}
        <div class="export-btn-group">
          <button
            type="button"
            class="btn-export"
            onclick={exportAuditLogsJSON}
            disabled={isLoading || isExporting}
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
              <path d="M2 10v3a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-3" />
              <path d="M8 2v8m-3-3l3 3 3-3" />
            </svg>
            <span>Export JSON</span>
          </button>
          <button
            type="button"
            class="btn-export btn-export-secondary"
            onclick={exportAuditLogsCSV}
            disabled={isLoading || isExporting}
          >
            <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
              <path d="M2 10v3a1 1 0 0 0 1 1h10a1 1 0 0 0 1-1v-3" />
              <path d="M8 2v8m-3-3l3 3 3-3" />
            </svg>
            <span>Export CSV</span>
          </button>
        </div>
      {/if}
    </div>
  </div>

  <!-- Recent Report Runs & Audit Entries Preview -->
  {#if isAdmin($user?.role)}
    <div class="preview-card">
      <div class="card-header-row">
        <h3 class="preview-heading">Recent Audit Log Records Preview</h3>
        <span class="preview-count">{auditLogs.length} Total Records Loaded</span>
      </div>

    {#if isLoading}
      <div class="loading-state">
        <span class="spinner"></span>
        <p>Loading complete records from PostgreSQL database...</p>
      </div>
    {:else if auditLogs.length === 0}
      <div class="empty-state">
        <p>No audit log records currently available.</p>
      </div>
    {:else}
      <div class="table-wrap">
        <table class="preview-table">
          <thead>
            <tr>
              <th>Action</th>
              <th>Entity Type</th>
              <th>Actor</th>
              <th class="text-right">Timestamp</th>
            </tr>
          </thead>
          <tbody>
            {#each auditLogs.slice(0, 10) as log}
              <tr>
                <td><span class="action-badge">{log.action}</span></td>
                <td>{log.entity_type}</td>
                <td class="font-mono">{log.actor}</td>
                <td class="text-right font-muted">{formatDate(log.created_at)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
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
    padding-bottom: 0.5rem;
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
    margin: 0.25rem 0 0.15rem 0;
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    margin: 0;
  }

  .btn-refresh {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.875rem;
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text);
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm, 4px);
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .btn-refresh:hover:not(:disabled) {
    background-color: var(--color-surface-hover, #202632);
    border-color: var(--color-border-hover, #333d4d);
  }

  .btn-refresh:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .alert {
    padding: 0.75rem 1rem;
    border-radius: var(--radius-sm, 4px);
    font-size: 0.875rem;
    font-weight: 500;
  }

  .alert-info {
    background-color: rgba(2, 132, 199, 0.12);
    color: #38bdf8;
    border: 1px solid rgba(2, 132, 199, 0.3);
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .alert-success {
    background-color: rgba(16, 185, 129, 0.12);
    color: #34d399;
    border: 1px solid rgba(16, 185, 129, 0.3);
  }

  .alert-error {
    background-color: rgba(239, 68, 68, 0.12);
    color: #f87171;
    border: 1px solid rgba(239, 68, 68, 0.3);
  }

  .reports-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(420px, 1fr));
    gap: 1.25rem;
  }

  .report-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md, 6px);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    position: relative;
  }

  .report-badge {
    align-self: flex-start;
    padding: 0.2rem 0.5rem;
    background-color: rgba(212, 175, 55, 0.1);
    color: var(--color-primary);
    border: 1px solid rgba(212, 175, 55, 0.25);
    border-radius: var(--radius-sm, 4px);
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.06em;
  }

  .report-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .report-desc {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    line-height: 1.45;
    margin: 0;
    flex: 1;
  }

  .report-meta {
    display: flex;
    justify-content: space-between;
    font-size: 0.75rem;
    color: var(--color-text-muted);
    padding: 0.5rem 0;
    border-top: 1px dashed var(--color-border);
    border-bottom: 1px dashed var(--color-border);
  }

  .export-btn-group {
    display: flex;
    gap: 0.75rem;
    margin-top: 0.25rem;
  }

  .btn-export {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 0.625rem 1rem;
    background-color: var(--color-primary);
    color: #0b0f17;
    font-size: 0.8125rem;
    font-weight: 700;
    border: none;
    border-radius: var(--radius-sm, 4px);
    cursor: pointer;
    transition: filter 0.15s ease;
  }

  .btn-export:hover:not(:disabled) {
    filter: brightness(1.1);
  }

  .btn-export:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .btn-export svg {
    width: 14px;
    height: 14px;
  }

  .btn-export-secondary {
    background-color: transparent;
    color: var(--color-text);
    border: 1px solid var(--color-border);
  }

  .btn-export-secondary:hover:not(:disabled) {
    background-color: var(--color-surface-hover, #202632);
    border-color: var(--color-border-hover, #333d4d);
    filter: none;
  }

  .preview-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md, 6px);
    padding: 1.25rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .card-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .preview-heading {
    font-size: 0.9375rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .preview-count {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-primary);
    background-color: rgba(212, 175, 55, 0.08);
    border: 1px solid rgba(212, 175, 55, 0.2);
    padding: 0.15rem 0.5rem;
    border-radius: var(--radius-sm, 4px);
  }

  .table-wrap {
    overflow-x: auto;
  }

  .preview-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8125rem;
  }

  .preview-table th {
    text-align: left;
    padding: 0.5rem 0.75rem;
    font-size: 0.6875rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--color-text-muted);
    border-bottom: 1px solid var(--color-border);
    text-transform: uppercase;
  }

  .preview-table td {
    padding: 0.625rem 0.75rem;
    border-bottom: 1px solid var(--color-border);
    color: var(--color-text);
  }

  .action-badge {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    font-family: var(--font-mono, monospace);
    font-size: 0.6875rem;
    background-color: var(--color-surface-hover, #202632);
    border: 1px solid var(--color-border);
    border-radius: 3px;
    color: var(--color-text);
  }

  .font-mono {
    font-family: var(--font-mono, monospace);
  }

  .font-muted {
    color: var(--color-text-muted);
  }

  .text-right {
    text-align: right;
  }

  .spinner-sm {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: spin 0.7s linear infinite;
  }

  .spinner {
    display: inline-block;
    width: 24px;
    height: 24px;
    border: 2px solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .loading-state, .empty-state {
    padding: 2.5rem;
    text-align: center;
    color: var(--color-text-muted);
    font-size: 0.875rem;
  }

  @media (max-width: 768px) {
    .reports-grid {
      grid-template-columns: 1fr;
    }
    .export-btn-group {
      flex-direction: column;
    }
  }
</style>
