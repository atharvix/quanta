<script lang="ts">
  import { onMount } from 'svelte';
  import { browser } from '$app/env';
  import { user } from '$lib/auth';
  import { theme } from '$lib/theme';

  let selectedCurrency = $state('USD');
  let pollingInterval = $state('30');
  let liquidityAlert = $state(true);
  let healthAlert = $state(true);
  let auditAlert = $state(true);
  let saveSuccess = $state(false);

  onMount(() => {
    if (browser) {
      selectedCurrency = localStorage.getItem('quanta_currency') || 'USD';
      pollingInterval = localStorage.getItem('quanta_polling_interval') || '30';
      liquidityAlert = localStorage.getItem('quanta_alert_liquidity') !== 'false';
      healthAlert = localStorage.getItem('quanta_alert_health') !== 'false';
      auditAlert = localStorage.getItem('quanta_alert_audit') !== 'false';
    }
  });

  function handleSave(e: SubmitEvent) {
    e.preventDefault();
    if (browser) {
      localStorage.setItem('quanta_currency', selectedCurrency);
      localStorage.setItem('quanta_polling_interval', pollingInterval);
      localStorage.setItem('quanta_alert_liquidity', liquidityAlert ? 'true' : 'false');
      localStorage.setItem('quanta_alert_health', healthAlert ? 'true' : 'false');
      localStorage.setItem('quanta_alert_audit', auditAlert ? 'true' : 'false');
    }
    saveSuccess = true;
    setTimeout(() => {
      saveSuccess = false;
    }, 3000);
  }
</script>

<svelte:head>
  <title>Settings — QUANTA</title>
</svelte:head>

<div class="page-shell narrow">
  <header class="page-head" data-enter>
    <div>
      <h1>Settings</h1>
      <p>Account identity, appearance theme, and workspace telemetry defaults.</p>
    </div>
  </header>

  {#if saveSuccess}
    <div class="alert alert-success" role="status">
      Settings saved to local browser preferences.
    </div>
  {/if}

  <form onsubmit={handleSave} class="settings-sections">
    <section class="settings-band">
      <div class="band-info">
        <h2>Operator Profile</h2>
        <p>Active authenticated session identity and role scope.</p>
      </div>
      {#if $user}
        <div class="band-fields">
          <div class="profile-grid">
            <div class="kv">
              <span class="detail-label">Name</span>
              <span class="kv-val">{$user.full_name || 'Operator'}</span>
            </div>
            <div class="kv">
              <span class="detail-label">Email</span>
              <span class="kv-val font-mono">{$user.email}</span>
            </div>
            <div class="kv">
              <span class="detail-label">Role</span>
              <div><span class="badge badge-{$user.role}">{$user.role}</span></div>
            </div>
          </div>
        </div>
      {/if}
    </section>

    <section class="settings-band">
      <div class="band-info">
        <h2>Display &amp; Telemetry</h2>
        <p>Default reporting currency, theme mode, and health probe interval.</p>
      </div>
      <div class="band-fields fields-two">
        <div class="form-group">
          <label for="currency-select" class="form-label">Reporting currency</label>
          <select id="currency-select" class="form-select" bind:value={selectedCurrency}>
            <option value="USD">USD ($) — US Dollar</option>
            <option value="EUR">EUR (€) — Euro</option>
            <option value="GBP">GBP (£) — British Pound</option>
            <option value="INR">INR (₹) — Indian Rupee</option>
          </select>
        </div>

        <div class="form-group">
          <label for="polling-select" class="form-label">Health poll interval</label>
          <select id="polling-select" class="form-select" bind:value={pollingInterval}>
            <option value="15">15 seconds</option>
            <option value="30">30 seconds (default)</option>
            <option value="60">60 seconds</option>
          </select>
        </div>

        <div class="form-group">
          <label for="theme-select" class="form-label">Color theme</label>
          <select
            id="theme-select"
            class="form-select"
            value={$theme}
            onchange={(e) => theme.setTheme((e.target as HTMLSelectElement).value as 'light' | 'dark' | 'system')}
          >
            <option value="light">Paper &amp; Ink (Light)</option>
            <option value="dark">Carbon Terminal (Dark)</option>
            <option value="system">System Match</option>
          </select>
        </div>
      </div>
    </section>

    <section class="settings-band">
      <div class="band-info">
        <h2>Alerts</h2>
        <p>Operational warnings triggered across local sessions.</p>
      </div>
      <div class="band-fields check-list">
        <label class="check-row">
          <input type="checkbox" bind:checked={liquidityAlert} />
          <div>
            <span class="check-title">Liquidity floor breaches</span>
            <span class="check-desc">Highlight portfolios falling below minimum cash reserve ratio.</span>
          </div>
        </label>

        <label class="check-row">
          <input type="checkbox" bind:checked={healthAlert} />
          <div>
            <span class="check-title">Service offline warnings</span>
            <span class="check-desc">Alert when Postgres, AI Engine (:8000), or Optimizer (:8001) disconnects.</span>
          </div>
        </label>

        <label class="check-row">
          <input type="checkbox" bind:checked={auditAlert} />
          <div>
            <span class="check-title">Administrative role changes</span>
            <span class="check-desc">Surface role reassignment events for Admin operators.</span>
          </div>
        </label>
      </div>
    </section>

    <div class="save-bar">
      <button type="submit" class="btn btn-primary btn-sm">Save preferences</button>
    </div>
  </form>
</div>

<style>
  .page-shell.narrow {
    max-width: 960px;
  }

  .settings-sections {
    display: flex;
    flex-direction: column;
  }

  .settings-band {
    display: grid;
    grid-template-columns: 260px 1fr;
    gap: 2rem;
    padding: 1.5rem 0;
    border-bottom: 1px solid var(--color-border);
    align-items: start;
  }

  .band-info h2 {
    font-size: 1rem;
    font-weight: 500;
  }

  .band-info p {
    margin-top: 0.25rem;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    line-height: 1.45;
  }

  .profile-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(170px, 1fr));
    gap: 1rem;
  }

  .kv {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .kv-val {
    font-size: 0.9375rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .fields-two {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }

  .check-list {
    display: flex;
    flex-direction: column;
    gap: 0.875rem;
  }

  .check-row {
    display: flex;
    align-items: flex-start;
    gap: 0.65rem;
    cursor: pointer;
  }

  .check-row input {
    margin-top: 0.2rem;
    width: 16px;
    height: 16px;
    accent-color: var(--color-primary);
  }

  .check-title {
    display: block;
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .check-desc {
    display: block;
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .save-bar {
    display: flex;
    justify-content: flex-end;
    padding-top: 1.25rem;
  }

  @media (max-width: 800px) {
    .settings-band {
      grid-template-columns: 1fr;
      gap: 1rem;
    }
    .fields-two {
      grid-template-columns: 1fr;
    }
  }
</style>
