<script lang="ts">
  import { onMount } from "svelte";
  import { api } from "$lib/api";
  import { user } from "$lib/auth";
  import { isAdmin } from "$lib/roles";
  import type {
    Treasury,
    TreasuriesListResponse,
    AuditLogRecord,
    AuditLogsListResponse,
  } from "$lib/types";
  import { formatDate } from "$lib/format";
  import {
    generateBalanceSheetCSV,
    generateAuditTrailCSV,
    generateJSONReport,
  } from "$lib/reports";

  let treasuries = $state<Treasury[]>([]);
  let auditLogs = $state<AuditLogRecord[]>([]);
  let isLoading = $state(true);
  let isExporting = $state(false);
  let exportProgress = $state<string | null>(null);
  let exportSuccess = $state<string | null>(null);
  let exportError = $state<string | null>(null);

  function getTimestampString(): string {
    const d = new Date();
    const pad = (n: number) => String(n).padStart(2, "0");
    return `${d.getFullYear()}${pad(d.getMonth() + 1)}${pad(d.getDate())}_${pad(d.getHours())}${pad(d.getMinutes())}${pad(d.getSeconds())}`;
  }

  async function fetchAllTreasuries(): Promise<Treasury[]> {
    const pageSize = 100;
    let offset = 0;
    const allRecords: Treasury[] = [];
    while (true) {
      const res = await api<TreasuriesListResponse>(
        `/api/v1/treasuries?limit=${pageSize}&offset=${offset}`,
      );
      const batch = Array.isArray(res.data) ? res.data : [];
      allRecords.push(...batch);
      if (batch.length < pageSize) break;
      offset += pageSize;
      if (offset > 50000) break;
    }
    return allRecords;
  }

  async function fetchAllAuditLogs(): Promise<AuditLogRecord[]> {
    const pageSize = 100;
    let offset = 0;
    const allRecords: AuditLogRecord[] = [];
    while (true) {
      const res = await api<AuditLogsListResponse>(
        `/api/v1/audit-logs?limit=${pageSize}&offset=${offset}`,
      );
      const batch = Array.isArray(res.data) ? res.data : [];
      allRecords.push(...batch);
      if (batch.length < pageSize) break;
      offset += pageSize;
      if (offset > 50000) break;
    }
    return allRecords;
  }

  async function loadData() {
    isLoading = true;
    exportError = null;
    try {
      treasuries = await fetchAllTreasuries();
      auditLogs = isAdmin($user?.role) ? await fetchAllAuditLogs() : [];
    } catch (err: any) {
      exportError = `Failed to load reporting data: ${err?.message || "Network error"}`;
    } finally {
      isLoading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  function triggerDownload(
    content: string,
    filename: string,
    mimeType: string,
    recordCount: number,
  ) {
    try {
      const blob = new Blob([content], { type: mimeType });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.style.display = "none";
      a.href = url;
      a.setAttribute("download", filename);
      document.body.appendChild(a);
      a.click();

      setTimeout(() => {
        if (document.body.contains(a)) document.body.removeChild(a);
        URL.revokeObjectURL(url);
      }, 1500);

      exportSuccess = `Exported ${recordCount} records to ${filename}`;
      exportError = null;
      setTimeout(() => {
        exportSuccess = null;
      }, 5000);
    } catch (err: any) {
      exportError = `Download failed: ${err?.message || "Unable to create file blob"}`;
      exportSuccess = null;
    }
  }

  async function exportBalanceSheetCSV() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = "Fetching balance sheet records...";
    try {
      const allTreasuries = await fetchAllTreasuries();
      if (allTreasuries.length === 0) {
        exportError = "No balance sheet records available to export.";
        return;
      }
      treasuries = allTreasuries;
      triggerDownload(
        generateBalanceSheetCSV(allTreasuries),
        `quanta_balance_sheet_${getTimestampString()}.csv`,
        "text/csv;charset=utf-8;",
        allTreasuries.length,
      );
    } catch (err: any) {
      exportError = `Failed to export CSV: ${err?.message || "Serialization error"}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportBalanceSheetJSON() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = "Fetching balance sheet records...";
    try {
      const allTreasuries = await fetchAllTreasuries();
      if (allTreasuries.length === 0) {
        exportError = "No balance sheet records available to export.";
        return;
      }
      treasuries = allTreasuries;
      triggerDownload(
        generateJSONReport(allTreasuries),
        `quanta_balance_sheet_${getTimestampString()}.json`,
        "application/json;charset=utf-8;",
        allTreasuries.length,
      );
    } catch (err: any) {
      exportError = `Failed to export JSON: ${err?.message || "Serialization error"}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportAuditLogsJSON() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = "Fetching audit journal...";
    try {
      const allLogs = await fetchAllAuditLogs();
      if (allLogs.length === 0) {
        exportError = "No audit records available to export.";
        return;
      }
      auditLogs = allLogs;
      triggerDownload(
        generateJSONReport(allLogs),
        `quanta_audit_journal_${getTimestampString()}.json`,
        "application/json;charset=utf-8;",
        allLogs.length,
      );
    } catch (err: any) {
      exportError = `Failed to export audit JSON: ${err?.message || "Serialization error"}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }

  async function exportAuditLogsCSV() {
    exportError = null;
    exportSuccess = null;
    isExporting = true;
    exportProgress = "Fetching audit journal...";
    try {
      const allLogs = await fetchAllAuditLogs();
      if (allLogs.length === 0) {
        exportError = "No audit records available to export.";
        return;
      }
      auditLogs = allLogs;
      triggerDownload(
        generateAuditTrailCSV(allLogs),
        `quanta_audit_journal_${getTimestampString()}.csv`,
        "text/csv;charset=utf-8;",
        allLogs.length,
      );
    } catch (err: any) {
      exportError = `Failed to export audit CSV: ${err?.message || "Serialization error"}`;
    } finally {
      isExporting = false;
      exportProgress = null;
    }
  }
</script>

<svelte:head>
  <title>Financial Reports — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>Financial Reports</h1>
      <p>
        Full-dataset balance sheet statements and role-scoped audit journal
        exports.
      </p>
    </div>
    <div class="page-head-actions">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        onclick={loadData}
        disabled={isLoading || isExporting}
      >
        {isLoading ? "Loading..." : "Refresh"}
      </button>
    </div>
  </header>

  {#if exportProgress}
    <div class="notice-banner" role="status">{exportProgress}</div>
  {/if}
  {#if exportSuccess}
    <div class="alert alert-success" role="status">{exportSuccess}</div>
  {/if}
  {#if exportError}
    <div class="alert alert-danger" role="alert">{exportError}</div>
  {/if}

  <div class="reports-grid">
    <section
      class="surface-panel report-panel"
      aria-label="Balance sheet exports"
    >
      <div>
        <h2>Balance Sheet Statement</h2>
        <p class="report-desc">
          Un-truncated schedule of capital bases, risk tolerances, and liquidity
          policy floors across all {treasuries.length} portfolios.
        </p>
      </div>
      <div class="report-meta font-mono">
        <span>Scope: {treasuries.length} portfolios</span>
        <span>Formats: CSV, JSON</span>
      </div>
      <div class="btn-row">
        <button
          type="button"
          class="btn btn-primary btn-sm"
          onclick={exportBalanceSheetCSV}
          disabled={isLoading || isExporting}
        >
          Export CSV
        </button>
        <button
          type="button"
          class="btn btn-secondary btn-sm"
          onclick={exportBalanceSheetJSON}
          disabled={isLoading || isExporting}
        >
          Export JSON
        </button>
      </div>
    </section>

    <section
      class="surface-panel report-panel"
      aria-label="Audit trail exports"
    >
      <div>
        <h2>Audit Journal</h2>
        <p class="report-desc">
          Chronological system event ledger covering authentication events,
          treasury writes, and role modifications.
        </p>
      </div>
      <div class="report-meta font-mono">
        {#if isAdmin($user?.role)}
          <span>Scope: {auditLogs.length} events</span>
          <span>Formats: JSON, CSV</span>
        {:else}
          <span>Scope: Admin access required</span>
        {/if}
      </div>
      {#if isAdmin($user?.role)}
        <div class="btn-row">
          <button
            type="button"
            class="btn btn-primary btn-sm"
            onclick={exportAuditLogsJSON}
            disabled={isLoading || isExporting}
          >
            Export JSON
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            onclick={exportAuditLogsCSV}
            disabled={isLoading || isExporting}
          >
            Export CSV
          </button>
        </div>
      {:else}
        <p class="muted-note">
          Audit exports are restricted to Admin operators. Balance sheet exports
          on the left remain available.
        </p>
      {/if}
    </section>
  </div>

  {#if isAdmin($user?.role)}
    <section class="section-block" aria-label="Recent audit preview">
      <div class="block-head">
        <h2>Recent Audit Records</h2>
        <span class="head-meta font-mono">{auditLogs.length} total loaded</span>
      </div>

      {#if isLoading}
        <div class="empty-msg">Loading audit records...</div>
      {:else if auditLogs.length === 0}
        <div class="empty-msg">No audit records recorded.</div>
      {:else}
        <div class="table-container">
          <table class="table">
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
                  <td class="font-mono font-bold">{log.action}</td>
                  <td>{log.entity_type}</td>
                  <td class="font-mono">{log.actor}</td>
                  <td class="text-right muted-cell"
                    >{formatDate(log.created_at)}</td
                  >
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .reports-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.5rem;
  }

  .report-panel {
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    gap: 1.25rem;
  }

  .report-panel h2 {
    font-size: 1.0625rem;
    font-weight: 500;
  }

  .report-desc {
    margin-top: 0.35rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
    line-height: 1.5;
  }

  .report-meta {
    display: flex;
    gap: 1.25rem;
    padding: 0.65rem 0;
    border-top: 1px solid var(--color-border-subtle);
    border-bottom: 1px solid var(--color-border-subtle);
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .btn-row {
    display: flex;
    gap: 0.625rem;
    flex-wrap: wrap;
  }

  .muted-note {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .section-block {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .block-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
  }

  .block-head h2 {
    font-size: 1.0625rem;
    font-weight: 500;
  }

  .head-meta {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .muted-cell {
    color: var(--color-text-muted);
    font-size: 0.8125rem;
  }

  .empty-msg {
    padding: 2rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }

  @media (max-width: 768px) {
    .reports-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
