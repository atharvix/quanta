<script lang="ts">
  import { goto } from '$app/navigation';
  import { register } from '$lib/auth';
  import { ApiError } from '$lib/api';

  let fullName = $state('');
  let email = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let isLoading = $state(false);
  let formError = $state<string | null>(null);
  let fieldErrors = $state<{
    fullName?: string;
    email?: string;
    password?: string;
    confirmPassword?: string;
  }>({});

  function validate(): boolean {
    const errors: typeof fieldErrors = {};
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
      await goto('/login?registered=1');
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        if (err.status === 409) {
          formError = 'An account with this email address already exists.';
        } else if (err.details && err.details.length > 0) {
          formError = err.details.join(', ');
        } else {
          formError = err.message || 'Registration failed.';
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
    <h1 class="auth-title">Create Account</h1>
    <p class="auth-subtitle">Join the QUANTA Autonomous Treasury Platform</p>
  </div>

  {#if formError}
    <div class="alert alert-danger">
      {formError}
    </div>
  {/if}

  <form onsubmit={handleSubmit} novalidate>
    <div class="form-group">
      <label for="fullName" class="form-label">Full Name</label>
      <input
        id="fullName"
        type="text"
        class="form-input"
        placeholder="Jane Doe"
        bind:value={fullName}
        disabled={isLoading}
        autocomplete="name"
      />
    </div>

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
        placeholder="Minimum 8 characters"
        bind:value={password}
        disabled={isLoading}
        autocomplete="new-password"
      />
      {#if fieldErrors.password}
        <p class="field-error">{fieldErrors.password}</p>
      {/if}
    </div>

    <div class="form-group">
      <label for="confirmPassword" class="form-label">Confirm Password</label>
      <input
        id="confirmPassword"
        type="password"
        class="form-input"
        class:has-error={!!fieldErrors.confirmPassword}
        placeholder="Re-enter your password"
        bind:value={confirmPassword}
        disabled={isLoading}
        autocomplete="new-password"
      />
      {#if fieldErrors.confirmPassword}
        <p class="field-error">{fieldErrors.confirmPassword}</p>
      {/if}
    </div>

    <div class="form-actions">
      <button type="submit" class="btn btn-primary btn-block" disabled={isLoading}>
        {isLoading ? 'Creating Account...' : 'Register'}
      </button>
    </div>
  </form>

  <div class="auth-footer">
    <span>Already have an account?</span>
    <a href="/login">Sign in</a>
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
