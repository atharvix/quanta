<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { register } from '$lib/auth';
  import { api, ApiError } from '$lib/api';
  import type { SystemHealthResponse } from '$lib/types';
  import ThemeToggle from '$lib/components/ThemeToggle.svelte';
  import QuantaMonogram from '$lib/components/QuantaMonogram.svelte';
  import CapitalIntelligenceVisual from '$lib/components/CapitalIntelligenceVisual.svelte';

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

  function toggleConfirmPassword() {
    showConfirmPassword = !showConfirmPassword;
  }

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

    if (!validate()) {
      return;
    }

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
        formError = 'Registration failed. Please check your details and try again.';
      }
    } finally {
      isLoading = false;
    }
  }
</script>

<svelte:head>
  <title>Register - QUANTA</title>
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

  <!-- RIGHT PANEL: Registration Workspace (~42%) -->
  <main class="auth-panel" aria-label="Register Account">
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
          <h2 class="card-title">Create QUANTA Account</h2>
          <p class="card-subtitle">Onboard as a new treasury system operator.</p>
        </div>

        {#if formError}
          <div class="alert alert-danger" role="alert">
            {formError}
          </div>
        {/if}

        <!-- Registration Form -->
        <form onsubmit={handleSubmit} novalidate class="auth-form">
          <div class="form-group">
            <label for="fullName" class="form-label">Full Legal Name</label>
            <input
              id="fullName"
              type="text"
              class="form-input"
              class:input-error={!!fieldErrors.fullName}
              placeholder="e.g. Alexander Vance"
              bind:value={fullName}
              disabled={isLoading}
              autocomplete="name"
              required
            />
            {#if fieldErrors.fullName}
              <span class="field-error-text">{fieldErrors.fullName}</span>
            {/if}
          </div>

          <div class="form-group">
            <label for="email" class="form-label">Work Email Address</label>
            <input
              id="email"
              type="email"
              class="form-input"
              class:input-error={!!fieldErrors.email}
              placeholder="operator@organization.com"
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
                placeholder="Minimum 8 characters"
                bind:value={password}
                disabled={isLoading}
                autocomplete="new-password"
                required
              />
            </div>
            {#if fieldErrors.password}
              <span class="field-error-text">{fieldErrors.password}</span>
            {/if}
          </div>

          <div class="form-group">
            <div class="password-label-row">
              <label for="confirmPassword" class="form-label">Confirm Password</label>
              <button
                type="button"
                class="btn-text-action"
                onclick={toggleConfirmPassword}
                tabindex="-1"
              >
                {showConfirmPassword ? 'Hide' : 'Show'}
              </button>
            </div>
            <div class="password-input-wrap">
              <input
                id="confirmPassword"
                type={showConfirmPassword ? 'text' : 'password'}
                class="form-input"
                class:input-error={!!fieldErrors.confirmPassword}
                placeholder="Re-enter password"
                bind:value={confirmPassword}
                disabled={isLoading}
                autocomplete="new-password"
                required
              />
            </div>
            {#if fieldErrors.confirmPassword}
              <span class="field-error-text">{fieldErrors.confirmPassword}</span>
            {/if}
          </div>

          <button
            type="submit"
            class="btn-submit-primary"
            disabled={isLoading}
          >
            {#if isLoading}
              <span class="spinner-icon"></span>
              <span>Registering...</span>
            {:else}
              <span>Create Account</span>
            {/if}
          </button>
        </form>

        <!-- Login Redirect Link -->
        <div class="auth-card-footer">
          <span class="footer-prompt">Already an operator?</span>
          <a href="/login" class="footer-link">Sign in to platform &rarr;</a>
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

  .alert-danger {
    background-color: rgba(239, 68, 68, 0.12);
    color: #ef4444;
    border: 1px solid rgba(239, 68, 68, 0.25);
  }

  /* Form Elements */
  .auth-form {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
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
    padding: 0.5rem 0.75rem;
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

  /* Card Footer */
  .auth-card-footer {
    display: flex;
    justify-content: center;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.6875rem;
    padding-top: 0.25rem;
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
    .form-input {
      transition: none;
    }
  }
</style>
