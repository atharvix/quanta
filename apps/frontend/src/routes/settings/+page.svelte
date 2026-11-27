<script lang="ts">
  import { onMount } from "svelte";
  import { browser } from "$app/env";
  import { user } from "$lib/auth";
  import { theme } from "$lib/theme";

  let selectedCurrency = $state("USD");
  let pollingInterval = $state("30");
  let liquidityAlert = $state(true);
  let healthAlert = $state(true);
  let auditAlert = $state(true);
  let saveSuccess = $state(false);

  onMount(() => {
    if (browser) {
      selectedCurrency = localStorage.getItem("quanta_currency") || "USD";
      pollingInterval = localStorage.getItem("quanta_polling_interval") || "30";
      liquidityAlert =
        localStorage.getItem("quanta_alert_liquidity") !== "false";
      healthAlert = localStorage.getItem("quanta_alert_health") !== "false";
      auditAlert = localStorage.getItem("quanta_alert_audit") !== "false";
    }
  });

  function handleSave(e: SubmitEvent) {
    e.preventDefault();
    if (browser) {
      localStorage.setItem("quanta_currency", selectedCurrency);
      localStorage.setItem("quanta_polling_interval", pollingInterval);
      localStorage.setItem(
        "quanta_alert_liquidity",
        liquidityAlert ? "true" : "false",
      );
      localStorage.setItem(
        "quanta_alert_health",
        healthAlert ? "true" : "false",
      );
      localStorage.setItem("quanta_alert_audit", auditAlert ? "true" : "false");
    }
    saveSuccess = true;
    setTimeout(() => {
      saveSuccess = false;
    }, 3000);
  }
</script>

<svelte:head>
  <title>Platform Settings - QUANTA</title>
</svelte:head>

<div class="settings-container">
  <div class="settings-header">
    <div class="header-titles">
      <div class="eyebrow-pill">
        <span class="eyebrow-dot"></span>
        <span class="eyebrow-text">SYSTEM PREFERENCES &amp; CONFIGURATION</span>
      </div>
      <h1 class="page-title">Platform Settings</h1>
      <p class="page-subtitle">
        Manage operator workspace configurations, telemetry intervals, and
        security preferences.
      </p>
    </div>
  </div>

  {#if saveSuccess}
    <div class="alert alert-success" role="status">
      Configuration preferences saved successfully.
    </div>
  {/if}

  <form onsubmit={handleSave} class="settings-form">
    <!-- Section 1: Operator Identity & RBAC Profile -->
    <div class="settings-card">
      <h2 class="card-heading">Operator Profile</h2>
      <p class="card-subheading">
        Active institutional session credentials and assigned access control
        role.
      </p>

      {#if $user}
        <div class="profile-details-grid">
          <div class="detail-item">
            <span class="detail-label">Full Name</span>
            <span class="detail-val"
              >{$user.full_name || "System Operator"}</span
            >
          </div>
          <div class="detail-item">
            <span class="detail-label">Email Address</span>
            <span class="detail-val mono-text">{$user.email}</span>
          </div>
          <div class="detail-item">
            <span class="detail-label">Role Classification</span>
            <div>
              <span class="badge badge-{$user.role}"
                >{$user.role.toUpperCase()}</span
              >
            </div>
          </div>
          <div class="detail-item">
            <span class="detail-label">Access Model</span>
            <span class="detail-val">Advisory Lock Protected RBAC</span>
          </div>
        </div>
      {/if}
    </div>

    <!-- Section 2: Financial & Telemetry Preferences -->
    <div class="settings-card">
      <h2 class="card-heading">Financial &amp; Telemetry Defaults</h2>
      <p class="card-subheading">
        Customize default reporting currency and live microservice polling
        frequency.
      </p>

      <div class="fields-grid">
        <div class="form-group">
          <label for="currency-select" class="form-label"
            >Base Reporting Currency</label
          >
          <select
            id="currency-select"
            class="form-select"
            bind:value={selectedCurrency}
          >
            <option value="USD">USD ($) - United States Dollar</option>
            <option value="EUR">EUR (€) - Eurozone</option>
            <option value="GBP">GBP (£) - British Pound</option>
            <option value="INR">INR (₹) - Indian Rupee</option>
          </select>
          <span class="form-hint"
            >Applied across consolidated balance sheet summaries.</span
          >
        </div>

        <div class="form-group">
          <label for="polling-select" class="form-label"
            >Diagnostic Telemetry Cycle</label
          >
          <select
            id="polling-select"
            class="form-select"
            bind:value={pollingInterval}
          >
            <option value="15">15 Seconds (High Frequency)</option>
            <option value="30">30 Seconds (Default Balanced)</option>
            <option value="60">60 Seconds (Low Network Footprint)</option>
          </select>
          <span class="form-hint"
            >Determines automated cluster health heartbeat polling.</span
          >
        </div>

        <div class="form-group">
          <label for="theme-select" class="form-label"
            >Workspace Appearance</label
          >
          <select
            id="theme-select"
            class="form-select"
            value={$theme}
            onchange={(e) =>
              theme.setTheme((e.target as HTMLSelectElement).value as any)}
          >
            <option value="light">Executive Light (Crisp Enterprise)</option>
            <option value="dark"
              >Command Center Dark (Obsidian &amp; Gold)</option
            >
            <option value="system">System Match (Auto OS Sync)</option>
          </select>
          <span class="form-hint"
            >Dual-theme high-contrast financial palette.</span
          >
        </div>
      </div>
    </div>

    <!-- Section 3: Notification Alerts -->
    <div class="settings-card">
      <h2 class="card-heading">System &amp; Risk Alert Channels</h2>
      <p class="card-subheading">
        Configure in-app notification indicators for critical treasury
        thresholds.
      </p>

      <div class="toggle-list">
        <label class="toggle-item">
          <input
            type="checkbox"
            bind:checked={liquidityAlert}
            class="toggle-checkbox"
          />
          <div class="toggle-meta">
            <span class="toggle-title">Liquidity Floor Warning</span>
            <span class="toggle-desc"
              >Trigger notification when any portfolio drops below its mandatory
              liquidity ratio.</span
            >
          </div>
        </label>

        <label class="toggle-item">
          <input
            type="checkbox"
            bind:checked={healthAlert}
            class="toggle-checkbox"
          />
          <div class="toggle-meta">
            <span class="toggle-title">Microservice Degradation Warning</span>
            <span class="toggle-desc"
              >Alert when Go Gateway, Postgres, or AI Engine reports
              non-operational state.</span
            >
          </div>
        </label>

        <label class="toggle-item">
          <input
            type="checkbox"
            bind:checked={auditAlert}
            class="toggle-checkbox"
          />
          <div class="toggle-meta">
            <span class="toggle-title"
              >Administrative Audit Trail Notifications</span
            >
            <span class="toggle-desc"
              >Log notifications when sensitive role modifications or asset
              allocations occur.</span
            >
          </div>
        </label>
      </div>
    </div>

    <div class="form-actions-bar">
      <button type="submit" class="btn-primary-save">
        <span>Save Platform Configuration</span>
      </button>
    </div>
  </form>
</div>

<style>
  .settings-container {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
    max-width: 960px;
    margin: 0 auto;
    width: 100%;
  }

  .settings-header {
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

  .alert {
    padding: 0.75rem 1rem;
    border-radius: var(--radius);
    font-size: 0.8125rem;
    font-weight: 600;
  }

  .alert-success {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  .settings-card {
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    padding: 1.5rem 1.75rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    box-shadow: var(--shadow-sm);
  }

  .card-heading {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
  }

  .card-subheading {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    margin: 0;
  }

  .profile-details-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;
    padding-top: 0.5rem;
  }

  .detail-item {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .detail-label {
    font-size: 0.6875rem;
    font-weight: 600;
    text-transform: uppercase;
    color: var(--color-text-muted);
    letter-spacing: 0.04em;
  }

  .detail-val {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .mono-text {
    font-family: var(--font-mono, monospace);
  }

  .fields-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1.25rem;
    padding-top: 0.5rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .form-label {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .form-select {
    padding: 0.6rem 0.75rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.8125rem;
    outline: none;
    transition: border-color 0.15s ease;
  }

  .form-select:focus {
    border-color: var(--color-primary);
  }

  .form-hint {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .toggle-list {
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
    padding-top: 0.5rem;
  }

  .toggle-item {
    display: flex;
    align-items: flex-start;
    gap: 0.75rem;
    cursor: pointer;
    padding: 0.6rem 0.75rem;
    background-color: var(--color-bg-secondary);
    border-radius: var(--radius);
    border: 1px solid var(--color-border-subtle);
  }

  .toggle-checkbox {
    margin-top: 0.2rem;
    width: 16px;
    height: 16px;
    accent-color: var(--color-primary);
  }

  .toggle-meta {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .toggle-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .toggle-desc {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    line-height: 1.35;
  }

  .form-actions-bar {
    display: flex;
    justify-content: flex-end;
    padding-top: 0.5rem;
  }

  .btn-primary-save {
    padding: 0.65rem 1.25rem;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    font-size: 0.8125rem;
    font-weight: 700;
    border: 1px solid var(--color-primary);
    border-radius: var(--radius);
    cursor: pointer;
    transition: background-color 0.15s ease;
  }

  .btn-primary-save:hover {
    background-color: var(--color-primary-hover);
  }

  @media (max-width: 768px) {
    .profile-details-grid,
    .fields-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
