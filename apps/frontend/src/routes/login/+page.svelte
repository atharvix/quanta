<script lang="ts">
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { login } from '$lib/auth';
  import { ApiError } from '$lib/api';

  let email = $state('');
  let password = $state('');
  let isLoading = $state(false);
  let formError = $state<string | null>(null);
  let fieldErrors = $state<{ email?: string; password?: string }>({});

  const isJustRegistered = $derived(page.url.searchParams.get('registered') === '1');

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

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    formError = null;

    if (!validate()) {
      return;
    }

    isLoading = true;

    try {
      await login(email.trim(), password);
      await goto('/');
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 401) {
          formError = 'Invalid credentials. Please check your email and password.';
        } else if (err.details && err.details.length > 0) {
          formError = err.details.join(', ');
        } else {
          formError = err.message || 'Authentication failed.';
        }
      } else if (err instanceof Error) {
        formError = err.message;
      } else {
        formError = 'An unexpected error occurred.';
      }
    } finally {
      isLoading = false;
    }
  }
</script>

<div class="auth-card card">
  <div class="auth-header">
    <h1 class="auth-title">Sign In to QUANTA</h1>
    <p class="auth-subtitle">Autonomous Treasury & Capital Allocation Platform</p>
  </div>

  {#if isJustRegistered}
    <div class="alert alert-success">
      Registration successful. Please sign in with your credentials.
    </div>
  {/if}

  {#if formError}
    <div class="alert alert-danger">
      {formError}
    </div>
  {/if}

  <form onsubmit={handleSubmit} novalidate>
    <div class="form-group">
      <label for="email" class="form-label">Email Address</label>
      <input
        id="email"
        type="email"
        class="form-input"
        class:has-error={!!fieldErrors.email}
        placeholder="name@organization.com"
        bind:value={email}
        disabled={isLoading}
        autocomplete="email"
      />
      {#if fieldErrors.email}
        <p class="field-error">{fieldErrors.email}</p>
      {/if}
    </div>

    <div class="form-group">
      <label for="password" class="form-label">Password</label>
      <input
        id="password"
        type="password"
        class="form-input"
        class:has-error={!!fieldErrors.password}
        placeholder="Enter at least 8 characters"
        bind:value={password}
        disabled={isLoading}
        autocomplete="current-password"
      />
      {#if fieldErrors.password}
        <p class="field-error">{fieldErrors.password}</p>
      {/if}
    </div>

    <div class="form-actions">
      <button type="submit" class="btn btn-primary btn-block" disabled={isLoading}>
        {isLoading ? 'Signing In...' : 'Sign In'}
      </button>
    </div>
  </form>

  <div class="auth-footer">
    <span>Do not have an account?</span>
    <a href="/register">Register here</a>
  </div>
</div>

<style>
  .auth-card {
    width: 100%;
    max-width: 420px;
    margin: 0 auto;
  }

  .auth-header {
    text-align: center;
    margin-bottom: 1.5rem;
  }

  .auth-title {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .auth-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .form-actions {
    margin-top: 1.25rem;
    margin-bottom: 1rem;
  }

  .auth-footer {
    text-align: center;
    font-size: 0.875rem;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    padding-top: 1rem;
    margin-top: 0.5rem;
    display: flex;
    justify-content: center;
    gap: 0.375rem;
  }
</style>
