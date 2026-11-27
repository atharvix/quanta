<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { login } from '$lib/auth';
  import { api, ApiError } from '$lib/api';
  import type { SystemHealthResponse } from '$lib/types';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import QuantaMonogram from '$lib/components/QuantaMonogram.svelte';
  import ProductLedgerPreview from '$lib/components/ProductLedgerPreview.svelte';

  let email = $state('');
  let password = $state('');
  let showPassword = $state(false);
  let isLoading = $state(false);
  let formError = $state<string | null>(null);
  let fieldErrors = $state<{ email?: string; password?: string }>({});

  const demoAccounts = [
    {
      role: 'admin',
      label: 'Admin',
      email: 'admin@quanta.local',
      password: 'password123',
      scope: 'Full fleet, user roles, audit ledger'
    },
    {
      role: 'analyst',
      label: 'Analyst',
      email: 'analyst@quanta.local',
      password: 'password123',
      scope: 'Create portfolios, run AI & QAOA'
    },
    {
      role: 'viewer',
      label: 'Viewer',
      email: 'viewer@quanta.local',
      password: 'password123',
      scope: 'Read-only portfolios & exports'
    }
  ] as const;

  const isJustRegistered = $derived(page.url.searchParams.get('registered') === '1');

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
    const errors: { email?: string; password?: string } = {};
    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
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

    fieldErrors = errors;
    return Object.keys(errors).length === 0;
  }

  async function performLogin(loginEmail: string, loginPass: string) {
    isLoading = true;
    formError = null;
    try {
      await login(loginEmail, loginPass);
      const redirect = page.url.searchParams.get('redirect') || '/';
      await goto(redirect);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          formError = 'Invalid email or password.';
        } else if (err.details && err.details.length > 0) {
          formError = err.details.join(', ');
        } else {
          formError = err.message;
        }
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'Unable to sign in right now.';
      }
    } finally {
      isLoading = false;
    }
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    formError = null;
    if (!validate()) return;
    await performLogin(email, password);
  }

  async function signInDemo(demoEmail: string, demoPassword: string) {
    email = demoEmail;
    password = demoPassword;
    fieldErrors = {};
    await performLogin(demoEmail, demoPassword);
  }
</script>

<svelte:head>
  <title>Sign in — QUANTA</title>
</svelte:head>

<div class="auth-layout">
  <section class="editorial-col" aria-label="QUANTA Product Overview">
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
          <h1>Corporate treasury ledger, Bayesian risk, and solver benchmarks.</h1>
          <p class="lead-copy">
            Model balance sheet assets and debt commitments, evaluate macro shock regimes, and compare classical and QAOA capital allocations in one workspace.
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

  <main class="form-col" aria-label="Sign in">
    <div class="form-topbar">
      <ThemeToggle />
    </div>

    <div class="form-wrap" data-enter>
      <div class="form-heading">
        <h2>Sign in to QUANTA</h2>
        <p>Use your account credentials or choose a seeded test account.</p>
      </div>

      {#if isJustRegistered}
        <div class="alert alert-success" role="status">
          Account created. Sign in to continue.
        </div>
      {/if}

      {#if formError}
        <div class="alert alert-danger" role="alert">
          {formError}
        </div>
      {/if}

      <form onsubmit={handleSubmit} novalidate class="login-form">
        <div class="form-group">
          <label for="email" class="form-label">Email address</label>
          <input
            id="email"
            type="email"
            class="form-input"
            class:has-error={!!fieldErrors.email}
            placeholder="analyst@quanta.local"
            bind:value={email}
            disabled={isLoading}
            autocomplete="email"
            aria-invalid={!!fieldErrors.email}
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
            placeholder="Enter password"
            bind:value={password}
            disabled={isLoading}
            autocomplete="current-password"
            aria-invalid={!!fieldErrors.password}
            required
          />
          {#if fieldErrors.password}
            <p class="field-error">{fieldErrors.password}</p>
          {/if}
        </div>

        <button type="submit" class="btn btn-primary btn-block" disabled={isLoading}>
          {isLoading ? 'Signing in...' : 'Sign in'}
        </button>
      </form>

      <section class="demo-section" aria-label="Test demo accounts by role">
        <div class="demo-head">
          <span class="demo-title">Test demo accounts</span>
          <span class="demo-pw">Password: <code class="font-mono">password123</code></span>
        </div>

        <div class="demo-rows">
          {#each demoAccounts as acc}
            <button
              type="button"
              class="demo-row"
              onclick={() => signInDemo(acc.email, acc.password)}
              disabled={isLoading}
            >
              <span class="badge badge-{acc.role}">{acc.label}</span>
              <span class="demo-info">
                <span class="demo-email font-mono">{acc.email}</span>
                <span class="demo-scope">{acc.scope}</span>
              </span>
              <span class="demo-action">Sign in &rarr;</span>
            </button>
          {/each}
        </div>
      </section>

      <p class="auth-switch">
        New user? <a href="/register">Create operator account &rarr;</a>
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

  .demo-section {
    padding-top: 1rem;
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: 0.625rem;
  }

  .demo-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
    font-size: 0.8125rem;
    gap: 0.5rem;
  }

  .demo-title {
    font-weight: 500;
    color: var(--color-text-secondary);
  }

  .demo-pw {
    color: var(--color-text-muted);
  }

  .demo-pw code {
    color: var(--color-text);
    background-color: var(--color-bg-secondary);
    padding: 0.1rem 0.35rem;
    border: 1px solid var(--color-border);
    border-radius: 3px;
  }

  .demo-rows {
    display: flex;
    flex-direction: column;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    overflow: hidden;
  }

  .demo-row {
    display: grid;
    grid-template-columns: auto 1fr auto;
    align-items: center;
    gap: 0.7rem;
    width: 100%;
    padding: 0.625rem 0.75rem;
    background-color: var(--color-surface);
    border: none;
    border-bottom: 1px solid var(--color-border-subtle);
    cursor: pointer;
    text-align: left;
    transition: background-color 140ms var(--ease);
  }

  .demo-row:last-child {
    border-bottom: none;
  }

  .demo-row:hover:not(:disabled) {
    background-color: var(--color-surface-hover);
  }

  .demo-row:disabled {
    opacity: 0.55;
    cursor: not-allowed;
  }

  .demo-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .demo-email {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text);
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .demo-scope {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .demo-action {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-primary);
    white-space: nowrap;
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
