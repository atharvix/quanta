<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { register } from '$lib/auth';
  import { api, ApiError } from '$lib/api';
  import type { SystemHealthResponse } from '$lib/types';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import QuantaMonogram from '$lib/components/QuantaMonogram.svelte';
  import ProductLedgerPreview from '$lib/components/ProductLedgerPreview.svelte';

  let fullName = $state('');
  let email = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let showPassword = $state(false);
  let showConfirmPassword = $state(false);
  let isLoading = $state(false);
  let formError = $state<string | null>(null);
  let fieldErrors = $state<{
    fullName?: string;
    email?: string;
    password?: string;
    confirmPassword?: string;
  }>({});

  let status = $state<{
    gateway: 'online' | 'offline';
    database: 'up' | 'down' | 'checking';
    aiEngine: 'up' | 'down' | 'checking';
    optimizer: 'up' | 'down' | 'checking';
  }>({
    gateway: 'online',
    database: 'checking',
    aiEngine: 'checking',
    optimizer: 'checking'
  });

  async function checkLiveHealth() {
    try {
      const res = await api<SystemHealthResponse>('/api/v1/health');
      status = {
        gateway: 'online',
        database: res.database === 'up' ? 'up' : 'down',
        aiEngine: res.dependencies?.ai_engine === 'up' ? 'up' : 'down',
        optimizer: res.dependencies?.optimization_engine === 'up' ? 'up' : 'down'
      };
    } catch {
      status = {
        gateway: 'offline',
        database: 'down',
        aiEngine: 'down',
        optimizer: 'down'
      };
    }
  }

  onMount(() => {
    checkLiveHealth();
  });

  function validate(): boolean {
    const errors: typeof fieldErrors = {};
    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

    if (!fullName.trim()) {
      errors.fullName = 'Full name is required.';
    }
    if (!email.trim()) {
      errors.email = 'Email address is required.';
    } else if (!emailPattern.test(email.trim())) {
      errors.email = 'Enter a valid email address.';
    }
    if (!password) {
      errors.password = 'Password is required.';
    } else if (password.length < 8) {
      errors.password = 'Password must be at least 8 characters.';
    }
    if (password !== confirmPassword) {
      errors.confirmPassword = 'Passwords do not match.';
    }

    fieldErrors = errors;
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    formError = null;
    if (!validate()) return;

    isLoading = true;
    try {
      await register(email.trim(), password, fullName.trim());
      await goto('/');
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          formError = 'An account with this email address already exists.';
        } else if (err.details && err.details.length > 0) {
          formError = err.details.join(', ');
        } else {
          formError = err.message;
        }
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'Registration failed.';
      }
    } finally {
      isLoading = false;
    }
  }
</script>

<svelte:head>
  <title>Create account — QUANTA</title>
</svelte:head>

<div class="auth-layout">
  <section class="editorial-col" aria-label="QUANTA Workspace Preview">
    <div class="editorial-inner">
      <header class="brand-bar">
        <a href="/" class="brand-mark-link">
          <QuantaMonogram size={28} />
          <span class="brand-name">QUANTA</span>
        </a>
        <span class="brand-domain">Corporate Treasury</span>
      </header>

      <div class="editorial-main" data-enter>
        <div class="headline-group">
          <h1>Create an analyst account with immediate portfolio access.</h1>
          <p class="lead-copy">
            New registrations receive Analyst permissions to create treasuries, persist asset and liability schedules, and run risk and QAOA benchmarks.
          </p>
        </div>

        <ProductLedgerPreview />
      </div>

      <footer class="health-strip" aria-label="Cluster status">
        <span class="health-label">Local stack</span>
        <div class="health-items">
          <span class="status-inline">
            <span class="status-dot {status.gateway === 'online' ? 'ok' : 'err'}"></span>
            Gateway :8080
          </span>
          <span class="status-inline">
            <span class="status-dot {status.database === 'up' ? 'ok' : 'err'}"></span>
            Postgres :5432
          </span>
          <span class="status-inline">
            <span class="status-dot {status.aiEngine === 'up' ? 'ok' : 'err'}"></span>
            AI :8000
          </span>
          <span class="status-inline">
            <span class="status-dot {status.optimizer === 'up' ? 'ok' : 'warn'}"></span>
            Optimizer :8001
          </span>
        </div>
      </footer>
    </div>
  </section>

  <main class="form-col" aria-label="Create account">
    <div class="form-topbar">
      <ThemeToggle />
    </div>

    <div class="form-wrap" data-enter>
      <div class="form-heading">
        <h2>Create operator account</h2>
        <p>Signs in automatically upon registration.</p>
      </div>

      {#if formError}
        <div class="alert alert-danger" role="alert">
          {formError}
        </div>
      {/if}

      <form onsubmit={handleSubmit} novalidate>
        <div class="form-group">
          <label for="fullName" class="form-label">Full name</label>
          <input
            id="fullName"
            type="text"
            class="form-input"
            class:has-error={!!fieldErrors.fullName}
            placeholder="Aarav Mehta"
            bind:value={fullName}
            disabled={isLoading}
            autocomplete="name"
            required
          />
          {#if fieldErrors.fullName}
            <p class="field-error">{fieldErrors.fullName}</p>
          {/if}
        </div>

        <div class="form-group">
          <label for="email" class="form-label">Work email address</label>
          <input
            id="email"
            type="email"
            class="form-input"
            class:has-error={!!fieldErrors.email}
            placeholder="operator@quanta.local"
            bind:value={email}
            disabled={isLoading}
            autocomplete="email"
            required
          />
          {#if fieldErrors.email}
            <p class="field-error">{fieldErrors.email}</p>
          {/if}
        </div>

        <div class="form-group">
          <div class="label-row">
            <label for="password" class="form-label">Password</label>
            <button
              type="button"
              class="text-toggle"
              onclick={() => (showPassword = !showPassword)}
              tabindex="-1"
            >
              {showPassword ? 'Hide' : 'Show'}
            </button>
          </div>
          <input
            id="password"
            type={showPassword ? 'text' : 'password'}
            class="form-input"
            class:has-error={!!fieldErrors.password}
            placeholder="Minimum 8 characters"
            bind:value={password}
            disabled={isLoading}
            autocomplete="new-password"
            required
          />
          {#if fieldErrors.password}
            <p class="field-error">{fieldErrors.password}</p>
          {/if}
        </div>

        <div class="form-group">
          <div class="label-row">
            <label for="confirmPassword" class="form-label">Confirm password</label>
            <button
              type="button"
              class="text-toggle"
              onclick={() => (showConfirmPassword = !showConfirmPassword)}
              tabindex="-1"
            >
              {showConfirmPassword ? 'Hide' : 'Show'}
            </button>
          </div>
          <input
            id="confirmPassword"
            type={showConfirmPassword ? 'text' : 'password'}
            class="form-input"
            class:has-error={!!fieldErrors.confirmPassword}
            placeholder="Re-enter password"
            bind:value={confirmPassword}
            disabled={isLoading}
            autocomplete="new-password"
            required
          />
          {#if fieldErrors.confirmPassword}
            <p class="field-error">{fieldErrors.confirmPassword}</p>
          {/if}
        </div>

        <button type="submit" class="btn btn-primary btn-block" disabled={isLoading}>
          {isLoading ? 'Creating account...' : 'Create account'}
        </button>
      </form>

      <p class="auth-switch">
        Already registered? <a href="/login">Sign in &rarr;</a>
      </p>
    </div>
  </main>
</div>

<style>
  .auth-layout {
    display: grid;
    grid-template-columns: 1.18fr 1fr;
    min-height: 100vh;
    width: 100%;
    background-color: var(--color-surface);
  }

  .editorial-col {
    background-color: var(--color-bg);
    border-right: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
  }

  .editorial-inner {
    padding: 2rem 2.75rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    height: 100%;
    gap: 2rem;
    max-width: 760px;
    margin: 0 auto;
    width: 100%;
  }

  .brand-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
  }

  .brand-mark-link {
    display: inline-flex;
    align-items: center;
    gap: 0.625rem;
    color: var(--color-text);
  }

  .brand-name {
    font-size: 1.0625rem;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  .brand-domain {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .editorial-main {
    display: flex;
    flex-direction: column;
    gap: 1.75rem;
    margin: auto 0;
  }

  .headline-group h1 {
    font-size: clamp(1.75rem, 2.3vw, 2.25rem);
    font-weight: 500;
    letter-spacing: -0.035em;
    line-height: 1.12;
    max-width: 560px;
  }

  .lead-copy {
    margin-top: 0.75rem;
    font-size: 0.9375rem;
    color: var(--color-text-secondary);
    line-height: 1.55;
    max-width: 540px;
  }

  .health-strip {
    padding-top: 1rem;
    border-top: 1px solid var(--color-border);
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
  }

  .health-label {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }

  .health-items {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-wrap: wrap;
    font-family: var(--font-mono);
  }

  .form-col {
    display: flex;
    flex-direction: column;
    padding: 1.75rem 2.5rem;
    background-color: var(--color-surface);
  }

  .form-topbar {
    display: flex;
    justify-content: flex-end;
  }

  .form-wrap {
    width: 100%;
    max-width: 400px;
    margin: auto;
    display: flex;
    flex-direction: column;
    gap: 1.125rem;
  }

  .form-heading h2 {
    font-size: 1.375rem;
    font-weight: 500;
    letter-spacing: -0.025em;
  }

  .form-heading p {
    margin-top: 0.3rem;
    font-size: 0.875rem;
    color: var(--color-text-secondary);
  }

  .label-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.4rem;
  }

  .label-row .form-label {
    margin-bottom: 0;
  }

  .text-toggle {
    background: none;
    border: none;
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-primary);
    cursor: pointer;
  }

  .text-toggle:hover {
    text-decoration: underline;
  }

  .auth-switch {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    text-align: center;
  }

  .auth-switch a {
    font-weight: 500;
  }

  @media (max-width: 1024px) {
    .auth-layout {
      grid-template-columns: 1fr;
    }
    .editorial-col {
      border-right: none;
      border-bottom: 1px solid var(--color-border);
    }
    .editorial-inner {
      padding: 1.5rem;
    }
  }

  @media (max-width: 640px) {
    .editorial-col {
      display: none;
    }
    .form-col {
      padding: 1.25rem;
    }
  }
</style>
