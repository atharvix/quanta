<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { login } from '$lib/auth';
  import { api, ApiError } from '$lib/api';
  import type { SystemHealthResponse } from '$lib/types';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import QuantaMonogram from '$lib/components/QuantaMonogram.svelte';
  import CapitalIntelligenceVisual from '$lib/components/CapitalIntelligenceVisual.svelte';

  let email = $state('');
  let password = $state('');
  let showPassword = $state(false);
  let isLoading = $state(false);
  let formError = $state<string | null>(null);
  let fieldErrors = $state<{ email?: string; password?: string }>({});

  const isJustRegistered = $derived(page.url.searchParams.get('registered') === '1');

  // Real Subsystem Status (Live backend telemetry)
  interface LiveClusterStatus {
    gateway: 'online' | 'offline';
    database: 'up' | 'down' | 'checking';
    aiEngine: 'up' | 'down' | 'checking';
    optimizer: 'up' | 'down' | 'checking';
  }

  let status = $state<LiveClusterStatus>({
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

  function togglePassword() {
    showPassword = !showPassword;
  }

  function validate(): boolean {
    const errors: { email?: string; password?: string } = {};
    const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!email) {
      errors.email = 'Email address is required.';
    } else if (!emailPattern.test(email)) {
      errors.email = 'Please enter a valid email address.';
    }

    if (!password) {
      errors.password = 'Password is required.';
    } else if (password.length < 8) {
      errors.password = 'Password must be at least 8 characters.';
    }

    fieldErrors = errors;
    return Object.keys(errors).length === 0;
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    formError = null;

    if (!validate()) {
      return;
    }

    isLoading = true;

    try {
      await login(email, password);
      const redirect = page.url.searchParams.get('redirect') || '/';
      await goto(redirect);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          formError = 'Invalid credentials. Please verify your email and password.';
        } else if (err.details && err.details.length > 0) {
          formError = err.details.join(', ');
        } else {
          formError = err.message;
        }
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'An unexpected authentication error occurred.';
      }
    } finally {
      isLoading = false;
    }
  }

  function fillDemo(userEmail: string, userPass: string) {
    email = userEmail;
    password = userPass;
    fieldErrors = {};
    formError = null;
  }
</script>

<svelte:head>
  <title>Sign In - QUANTA</title>
</svelte:head>

<div class="auth-page-root">
  <!-- LEFT PANEL: Institutional Architecture & Capital Intelligence (~58%) -->
  <section class="identity-panel" aria-label="QUANTA Architecture & Identity">
    <div class="identity-panel-inner">
      <!-- Top Brand Header -->
      <header class="brand-header">
        <a href="/" class="brand-link">
          <QuantaMonogram size={32} />
          <div class="brand-text">
            <span class="brand-title">QUANTA</span>
            <span class="brand-subtitle">INSTITUTIONAL TREASURY OS</span>
          </div>
        </a>
      </header>

      <!-- Center Hero Statement & Flowing Streams Visual -->
      <div class="identity-content">
        <div class="hero-block">
          <div class="tagline-pill">
            <span class="tagline-dot"></span>
            <span class="tagline-kicker">AUTONOMOUS CAPITAL ORCHESTRATION</span>
          </div>
          <h1 class="hero-tagline">
            <span class="brand-lead">QUANTA</span> &mdash; Capital intelligence. Engineered for control.
          </h1>
          <p class="hero-description">
            Unify treasury operations, understand exposure, and make disciplined capital allocation decisions through one intelligent platform.
          </p>
        </div>

        <!-- Interactive Flowing Capital Streams & Core Visual -->
        <div class="visual-wrap">
          <CapitalIntelligenceVisual {status} />
        </div>
      </div>

      <!-- Real Backend Telemetry Strip -->
      <footer class="telemetry-footer">
        <div class="telemetry-bar">
          <div class="telemetry-label">
            <span class="pulse-indicator"></span>
            <span class="telemetry-title">CLUSTER TELEMETRY</span>
          </div>
          <div class="telemetry-nodes">
            <div class="telemetry-node">
              <span class="node-name">Gateway</span>
              <span class="node-status {status.gateway === 'online' ? 'status-up' : 'status-down'}">
                {status.gateway === 'online' ? 'Online' : 'Offline'}
              </span>
            </div>
            <div class="telemetry-node">
              <span class="node-name">Postgres</span>
              <span class="node-status {status.database === 'up' ? 'status-up' : 'status-down'}">
                {status.database === 'up' ? 'Connected' : 'Offline'}
              </span>
            </div>
            <div class="telemetry-node">
              <span class="node-name">AI Engine</span>
              <span class="node-status {status.aiEngine === 'up' ? 'status-up' : 'status-down'}">
                {status.aiEngine === 'up' ? 'Active' : 'Offline'}
              </span>
            </div>
            <div class="telemetry-node">
              <span class="node-name">Optimizer</span>
              <span class="node-status {status.optimizer === 'up' ? 'status-up' : 'status-standby'}">
                {status.optimizer === 'up' ? 'Active' : 'Standby'}
              </span>
            </div>
          </div>
        </div>
      </footer>
    </div>
  </section>

  <!-- RIGHT PANEL: Authentication Workspace (~42%) -->
  <main class="auth-panel" aria-label="Sign In">
    <div class="auth-top-actions">
      <ThemeToggle />
    </div>

    <div class="auth-card-container">
      <div class="auth-card">
        <!-- Card Header -->
        <div class="card-header">
          <div class="card-brand-logo">
            <QuantaMonogram size={38} />
          </div>
          <h2 class="card-title">Sign In to QUANTA</h2>
          <p class="card-subtitle">Enter your institutional credentials to access the platform.</p>
        </div>

        <!-- Notification Alerts -->
        {#if isJustRegistered}
          <div class="alert alert-success" role="status">
            Registration successful. Sign in with your new account credentials.
          </div>
        {/if}

        {#if formError}
          <div class="alert alert-danger" role="alert">
            {formError}
          </div>
        {/if}

        <!-- Credentials Form -->
        <form onsubmit={handleSubmit} novalidate class="auth-form">
          <div class="form-group">
            <label for="email" class="form-label">Email Address</label>
            <input
              id="email"
              type="email"
              class="form-input"
              class:input-error={!!fieldErrors.email}
              placeholder="name@organization.com"
              bind:value={email}
              disabled={isLoading}
              autocomplete="email"
              required
            />
            {#if fieldErrors.email}
              <span class="field-error-text">{fieldErrors.email}</span>
            {/if}
          </div>

          <div class="form-group">
            <div class="password-label-row">
              <label for="password" class="form-label">Password</label>
              <button
                type="button"
                class="btn-text-action"
                onclick={togglePassword}
                tabindex="-1"
              >
                {showPassword ? 'Hide' : 'Show'}
              </button>
            </div>
            <div class="password-input-wrap">
              <input
                id="password"
                type={showPassword ? 'text' : 'password'}
                class="form-input"
                class:input-error={!!fieldErrors.password}
                placeholder="Enter account password"
                bind:value={password}
                disabled={isLoading}
                autocomplete="current-password"
                required
              />
            </div>
            {#if fieldErrors.password}
              <span class="field-error-text">{fieldErrors.password}</span>
            {/if}
          </div>

          <button
            type="submit"
            class="btn-submit-primary"
            disabled={isLoading}
          >
            {#if isLoading}
              <span class="spinner-icon"></span>
              <span>Authenticating...</span>
            {:else}
              <span>Sign In to Platform</span>
            {/if}
          </button>
        </form>

        <!-- Demo Accounts Shortcut Bar -->
        <div class="demo-accounts-section">
          <span class="demo-heading">DEMO OPERATOR CREDENTIALS</span>
          <div class="demo-pills-row">
            <button
              type="button"
              class="demo-pill"
              onclick={() => fillDemo('admin@quanta.local', 'password123')}
              title="Sign in as Administrator"
            >
              <span class="pill-role role-admin">ADMIN</span>
              <span class="pill-email">admin@quanta.local</span>
            </button>

            <button
              type="button"
              class="demo-pill"
              onclick={() => fillDemo('viewer@quanta.local', 'password123')}
              title="Sign in as Treasury Analyst"
            >
              <span class="pill-role role-analyst">ANALYST</span>
              <span class="pill-email">viewer@quanta.local</span>
            </button>

            <button
              type="button"
              class="demo-pill"
              onclick={() => fillDemo('auditor_viewer@quanta.local', 'password123')}
              title="Sign in as Read-only Auditor"
            >
              <span class="pill-role role-viewer">AUDITOR</span>
              <span class="pill-email">auditor_viewer@quanta.local</span>
            </button>
          </div>
        </div>

        <!-- Registration Link -->
        <div class="auth-card-footer">
          <span class="footer-prompt">New operator?</span>
          <a href="/register" class="footer-link">Register new account &rarr;</a>
        </div>
      </div>
    </div>
  </main>
</div>

<style>
  /* Layout Root (Strict 100vh Fullscreen without vertical scroll on standard 1440x900 desktop) */
  .auth-page-root {
    display: flex;
    min-height: 100vh;
    height: 100vh;
    width: 100%;
    background-color: var(--color-bg);
    box-sizing: border-box;
    overflow: hidden;
  }

  /* LEFT PANEL (~58% width) */
  .identity-panel {
    flex: 1.38;
    background-color: var(--color-bg-secondary);
    border-right: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    box-sizing: border-box;
    position: relative;
    height: 100%;
    overflow: hidden;
  }

  .identity-panel-inner {
    padding: 1.25rem 2rem;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    height: 100%;
    box-sizing: border-box;
    gap: 0.75rem;
  }

  .brand-header {
    display: flex;
    align-items: center;
  }

  .brand-link {
    display: inline-flex;
    align-items: center;
    gap: 0.75rem;
    text-decoration: none;
    color: var(--color-text);
  }

  .brand-text {
    display: flex;
    flex-direction: column;
  }

  .brand-title {
    font-size: 1.15rem;
    font-weight: 800;
    letter-spacing: 0.08em;
    line-height: 1.1;
    color: var(--color-text);
  }

  .brand-subtitle {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #d4af37;
    text-transform: uppercase;
    font-family: var(--font-mono, monospace);
  }

  /* Identity Content */
  .identity-content {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    margin: auto 0;
  }

  .hero-block {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .tagline-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
  }

  .tagline-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background-color: #d4af37;
  }

  .tagline-kicker {
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #d4af37;
    text-transform: uppercase;
  }

  .hero-tagline {
    font-size: 1.625rem;
    font-weight: 700;
    letter-spacing: -0.025em;
    color: var(--color-text);
    margin: 0;
    line-height: 1.2;
  }

  .brand-lead {
    color: #d4af37;
    font-weight: 800;
    letter-spacing: 0.02em;
  }

  .hero-description {
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
    line-height: 1.45;
    margin: 0;
    max-width: 540px;
  }

  .visual-wrap {
    width: 100%;
  }

  /* Telemetry Footer */
  .telemetry-footer {
    padding-top: 0.65rem;
    border-top: 1px solid var(--color-border);
  }

  .telemetry-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.75rem;
    flex-wrap: wrap;
  }

  .telemetry-label {
    display: flex;
    align-items: center;
    gap: 0.45rem;
  }

  .pulse-indicator {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: #10b981;
  }

  .telemetry-title {
    font-size: 0.625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: var(--color-text-muted);
    font-family: var(--font-mono, monospace);
  }

  .telemetry-nodes {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .telemetry-node {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    font-size: 0.6875rem;
  }

  .node-name {
    color: var(--color-text-muted);
  }

  .node-status {
    font-weight: 600;
    font-size: 0.625rem;
    padding: 0.08rem 0.3rem;
    border-radius: 3px;
  }

  .status-up {
    color: #10b981;
    background-color: rgba(16, 185, 129, 0.12);
  }

  .status-down {
    color: #ef4444;
    background-color: rgba(239, 68, 68, 0.12);
  }

  .status-standby {
    color: #f59e0b;
    background-color: rgba(245, 158, 11, 0.12);
  }

  /* RIGHT PANEL (~42% width) */
  .auth-panel {
    flex: 1;
    background-color: var(--color-bg);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    padding: 1.5rem 2rem;
    box-sizing: border-box;
    position: relative;
    overflow-y: auto;
  }

  .auth-top-actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
  }

  .auth-card-container {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 1;
    padding: 0.75rem 0;
  }

  .auth-card {
    width: 100%;
    max-width: 390px;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: 8px;
    padding: 1.5rem 1.75rem;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
    box-sizing: border-box;
  }

  .card-header {
    display: flex;
    flex-direction: column;
    align-items: center;
    text-align: center;
    gap: 0.25rem;
  }

  .card-brand-logo {
    margin-bottom: 0.25rem;
  }

  .card-title {
    font-size: 1.25rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    color: var(--color-text);
    margin: 0;
  }

  .card-subtitle {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
    margin: 0;
    line-height: 1.35;
  }

  /* Alerts */
  .alert {
    padding: 0.55rem 0.75rem;
    border-radius: 4px;
    font-size: 0.6875rem;
    line-height: 1.4;
  }

  .alert-success {
    background-color: rgba(16, 185, 129, 0.12);
    color: #10b981;
    border: 1px solid rgba(16, 185, 129, 0.25);
  }

  .alert-danger {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.25);
  }

  /* Form Elements */
  .auth-form {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .form-label {
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--color-text);
    letter-spacing: 0.02em;
  }

  .password-label-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .btn-text-action {
    background: transparent;
    border: none;
    font-size: 0.625rem;
    font-weight: 600;
    color: #d4af37;
    cursor: pointer;
    padding: 0;
  }

  .btn-text-action:hover {
    text-decoration: underline;
  }

  .form-input {
    width: 100%;
    padding: 0.55rem 0.75rem;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: 4px;
    font-size: 0.8125rem;
    color: var(--color-text);
    box-sizing: border-box;
    transition: border-color 0.15s ease, box-shadow 0.15s ease;
  }

  .form-input:focus {
    outline: none;
    border-color: #d4af37;
    box-shadow: 0 0 0 1px #d4af37;
  }

  .form-input.input-error {
    border-color: #ef4444;
  }

  .field-error-text {
    font-size: 0.625rem;
    color: #ef4444;
    font-weight: 500;
  }

  .btn-submit-primary {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.65rem;
    background-color: #d4af37;
    color: #0b0f19;
    font-size: 0.8125rem;
    font-weight: 700;
    border: 1px solid #d4af37;
    border-radius: 4px;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
    margin-top: 0.2rem;
  }

  .btn-submit-primary:hover:not(:disabled) {
    background-color: #c5a059;
    border-color: #c5a059;
  }

  .btn-submit-primary:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .spinner-icon {
    width: 12px;
    height: 12px;
    border: 2px solid transparent;
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  /* Demo Credentials Section */
  .demo-accounts-section {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    padding-top: 0.75rem;
    border-top: 1px solid var(--color-border);
  }

  .demo-heading {
    font-size: 0.5625rem;
    font-weight: 700;
    color: var(--color-text-muted);
    letter-spacing: 0.08em;
    font-family: var(--font-mono, monospace);
    text-align: center;
  }

  .demo-pills-row {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
  }

  .demo-pill {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0.3rem 0.55rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: 4px;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .demo-pill:hover {
    background-color: var(--color-bg);
    border-color: #d4af37;
  }

  .pill-role {
    font-size: 0.5625rem;
    font-weight: 800;
    padding: 0.08rem 0.3rem;
    border-radius: 2px;
    letter-spacing: 0.05em;
  }

  .role-admin {
    background-color: rgba(212, 175, 55, 0.15);
    color: #d4af37;
  }

  .role-analyst {
    background-color: rgba(56, 189, 248, 0.12);
    color: #38bdf8;
  }

  .role-viewer {
    background-color: rgba(148, 163, 184, 0.15);
    color: #94a3b8;
  }

  .pill-email {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
    font-family: var(--font-mono, monospace);
  }

  /* Card Footer */
  .auth-card-footer {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.6875rem;
    padding-top: 0.35rem;
  }

  .footer-prompt {
    color: var(--color-text-muted);
  }

  .footer-link {
    color: #d4af37;
    font-weight: 600;
    text-decoration: none;
  }

  .footer-link:hover {
    text-decoration: underline;
  }

  /* Responsive Breakpoints */
  @media (max-width: 1024px) {
    .auth-page-root {
      flex-direction: column;
      height: auto;
      overflow-y: auto;
    }

    .identity-panel {
      border-right: none;
      border-bottom: 1px solid var(--color-border);
      height: auto;
      overflow: visible;
    }

    .identity-panel-inner {
      padding: 1.5rem;
    }

    .auth-panel {
      padding: 1.5rem;
    }
  }

  @media (max-width: 640px) {
    .identity-panel {
      display: none;
    }

    .auth-panel {
      padding: 1rem;
    }

    .auth-card {
      padding: 1.25rem;
      border: 1px solid var(--color-border);
      background-color: var(--color-surface);
      box-shadow: none;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .spinner-icon {
      animation-duration: 2s;
    }

    .btn-submit-primary,
    .demo-pill,
    .form-input {
      transition: none;
    }
  }
</style>
