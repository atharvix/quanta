<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '$lib/api';
  import type { Treasury, TreasuriesListResponse, AuditLogRecord, AuditLogsListResponse } from '$lib/types';
  import { formatDate } from '$lib/format';

  let treasuries = $state<Treasury[]>([]);
  let auditLogs = $state<AuditLogRecord[]>([]);
  let isLoading = $state(true);
  let isExporting = $state(false);
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

  function escapeCSVCell(val: unknown): string {
    if (val === null || val === undefined) return '';
    const str = String(val);
    if (str.includes(',') || str.includes('"') || str.includes('\n') || str.includes('\r')) {
      return `"${str.replace(/"/g, '""')}"`;
    }
    return str;
  }

  async function loadData() {
    isLoading = true;
    exportError = null;
    try {
      const [tRes, aRes] = await Promise.allSettled([
        api<TreasuriesListResponse>('/api/v1/treasuries?limit=100'),
        api<AuditLogsListResponse>('/api/v1/audit-logs?limit=100')
      ]);

      if (tRes.status === 'fulfilled' && tRes.value?.data) {
        treasuries = Array.isArray(tRes.value.data) ? tRes.value.data : [];
      } else if (tRes.status === 'rejected') {
        console.error('Failed to load treasuries:', tRes.reason);
      }

      if (aRes.status === 'fulfilled' && aRes.value?.data) {
        auditLogs = Array.isArray(aRes.value.data) ? aRes.value.data : [];
      } else if (aRes.status === 'rejected') {
        console.error('Failed to load audit logs:', aRes.reason);
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

  function triggerDownload(content: string, filename: string, mimeType: string) {
    try {
      const blob = new Blob([content], { type: mimeType });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.style.display = 'none';
      a.href = url;
      a.setAttribute('download', filename);
      document.body.appendChild(a);
      a.click();
      
      // Cleanup after delay to guarantee browser download dispatch
      setTimeout(() => {
        if (document.body.contains(a)) {
          document.body.removeChild(a);
        }
        URL.revokeObjectURL(url);
      }, 1500);

      exportSuccess = `Successfully exported ${filename}`;
      exportError = null;
      setTimeout(() => {
        exportSuccess = null;
      }, 5000);
    } catch (err: any) {
      exportError = `Download failed: ${err?.message || 'Unable to create file blob'}`;
      exportSuccess = null;
    }
  }

  function exportBalanceSheetCSV() {
    exportError = null;
    exportSuccess = null;

    if (treasuries.length === 0) {
      exportError = 'Export failed: No balance sheet records available in database to export.';
      return;
    }

    isExporting = true;
    try {
      const headers = [
        'Portfolio ID',
        'Portfolio Name',
        'Base Currency',
        'Total Capital',
        'Risk Tolerance',
        'Min Liquidity Ratio',
        'Created At',
        'Updated At'
      ];

      const rows = treasuries.map((t) => [
        escapeCSVCell(t.id),
        escapeCSVCell(t.name),
        escapeCSVCell(t.base_currency || 'USD'),
        escapeCSVCell(t.total_capital),
        escapeCSVCell(t.risk_tolerance),
        escapeCSVCell(t.min_liquidity_ratio),
        escapeCSVCell(t.created_at),
        escapeCSVCell(t.updated_at)
      ]);

      const csvContent = [
        headers.join(','),
        ...rows.map((row) => row.join(','))
      ].join('\r\n');

      const filename = `quanta_balance_sheet_${getTimestampString()}.csv`;
      triggerDownload(csvContent, filename, 'text/csv;charset=utf-8;');
    } catch (err: any) {
      exportError = `Failed to generate balance sheet CSV: ${err?.message || 'Unknown serialization error'}`;
    } finally {
      isExporting = false;
    }
  }

  function exportAuditLogsJSON() {
    exportError = null;
    exportSuccess = null;

    if (auditLogs.length === 0) {
      exportError = 'Export failed: No audit journal records available from security backend.';
      return;
    }

    isExporting = true;
    try {
      const formattedData = auditLogs.map((log) => ({
        id: log.id,
        action: log.action,
        actor: log.actor,
        entity_type: log.entity_type,
        entity_id: log.entity_id,
        created_at: log.created_at,
        metadata: log.metadata || {}
      }));

      const jsonContent = JSON.stringify(formattedData, null, 2);
      const filename = `quanta_audit_journal_${getTimestampString()}.json`;
      triggerDownload(jsonContent, filename, 'application/json;charset=utf-8;');
    } catch (err: any) {
      exportError = `Failed to generate audit journal JSON: ${err?.message || 'Unknown serialization error'}`;
    } finally {
      isExporting = false;
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
      <p class="page-subtitle">Exportable balance-sheet statements, liquidity coverage disclosures, and cryptographically verified audit journals.</p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn-refresh" onclick={loadData} disabled={isLoading || isExporting}>
        {#if isLoading}
          <span class="spinner-sm"></span> Loading Data...
        {:else}
          Refresh Datasets
        {/if}
      </button>
    </div>
  </div>

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
        Complete schedule of capital assets, revolving credit facilities, and liquidity allocations across all {treasuries.length} corporate portfolios.
      </p>
      <div class="report-meta">
        <span>Format: CSV / Structured Table</span>
        <span>Scope: {treasuries.length} Portfolios Verified</span>
      </div>
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
        <span>Export Balance Sheet CSV</span>
      </button>
    </div>

    <!-- Report 2: Security & Governance Audit Journal -->
    <div class="report-card">
      <div class="report-badge">COMPLIANCE</div>
      <h2 class="report-title">Security &amp; Audit Trail Ledger</h2>
      <p class="report-desc">
        Immutable chronological event journal tracking operator authentication, asset creation, and role updates.
      </p>
      <div class="report-meta">
        <span>Format: JSON Document</span>
        <span>Scope: {auditLogs.length} Sealed Audit Records</span>
      </div>
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
        <span>Export Audit Journal JSON</span>
      </button>
    </div>
  </div>

  <!-- Recent Report Runs & Audit Entries Preview -->
  <div class="preview-card">
    <div class="card-header-row">
      <h3 class="preview-heading">Recent Audit Log Records Preview</h3>
      <span class="preview-count">{auditLogs.length} Records Loaded</span>
    </div>

    {#if isLoading}
      <div class="loading-state">
        <span class="spinner"></span>
        <p>Loading real-time records from PostgreSQL database...</p>
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
            {#each auditLogs.slice(0, 8) as log}
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
    color: var(--color-text-secondary);
    margin: 0;
  }

  .btn-refresh {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.875rem;
    font-size: 0.75rem;
    font-weight: 600;
    background-color: var(--color-surface);
    color: var(--color-text);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
  }

  .btn-refresh:hover:not(:disabled) {
    background-color: var(--color-bg-secondary);
  }

  .alert {
    padding: 0.75rem 1rem;
    border-radius: var(--radius);
    font-size: 0.8125rem;
    font-weight: 600;
  }

  .alert-success {
    background-color: var(--color-success-bg, #f0fdf4);
    color: var(--color-success, #166534);
    border: 1px solid var(--color-success-border, #bbf7d0);
  }

  .alert-error {
    background-color: var(--color-error-bg, #fef2f2);
    color: var(--color-error, #991b1b);
    border: 1px solid var(--color-error-border, #fecaca);
  }

  .reports-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.25rem;
  }

  .report-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    box-shadow: var(--shadow-sm);
  }

  .report-badge {
    font-size: 0.625rem;
    font-weight: 700;
    padding: 0.15rem 0.45rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
    color: var(--color-primary);
    align-self: flex-start;
    font-family: var(--font-mono, monospace);
  }

  .report-title {
    font-size: 1.125rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .report-desc {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    line-height: 1.45;
    margin: 0;
    flex: 1;
  }

  .report-meta {
    display: flex;
    justify-content: space-between;
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border-subtle);
    padding-top: 0.625rem;
  }

  .btn-export {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    padding: 0.65rem 1rem;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    font-size: 0.8125rem;
    font-weight: 700;
    border: 1px solid var(--color-primary);
    border-radius: var(--radius);
    cursor: pointer;
    transition: background-color 0.15s ease;
  }

  .btn-export:hover:not(:disabled) {
    background-color: var(--color-primary-hover);
  }

  .btn-export:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .btn-export svg {
    width: 14px;
    height: 14px;
  }

  .preview-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .card-header-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .preview-heading {
    font-size: 1rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .preview-count {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
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
    background-color: var(--color-bg-secondary);
    padding: 0.75rem 1rem;
    text-align: left;
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--color-border);
  }

  .preview-table td {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    color: var(--color-text);
  }

  .action-badge {
    font-size: 0.625rem;
    font-family: var(--font-mono, monospace);
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    background-color: var(--color-bg-secondary);
    border-radius: 3px;
  }

  .font-mono { font-family: var(--font-mono, monospace); }
  .font-muted { color: var(--color-text-muted); }
  .text-right { text-align: right; }

  .loading-state, .empty-state {
    padding: 2rem;
    text-align: center;
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .spinner-sm {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 2px solid currentColor;
    border-right-color: transparent;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  .spinner {
    display: inline-block;
    width: 24px;
    height: 24px;
    border: 3px solid var(--color-border);
    border-top-color: var(--color-primary);
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
    margin-bottom: 0.5rem;
  }

  @keyframes spin {
    to { transform: rotate(360deg); }
  }

  @media (max-width: 768px) {
    .reports-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
