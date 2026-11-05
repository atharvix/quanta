<script lang="ts">
  import { onMount } from 'svelte';
  import { user, logout } from '$lib/auth';
  import { canWrite, isAdmin } from '$lib/roles';
  import { api } from '$lib/api';
  import type { TreasuriesListResponse } from '$lib/types';

  let treasuryCount = $state<number | null>(null);
  let isLoadingCount = $state(true);

  onMount(async () => {
    try {
      const res = await api<TreasuriesListResponse>('/api/v1/treasuries');
      treasuryCount = Array.isArray(res.data) ? res.data.length : 0;
    } catch {
      treasuryCount = 0;
    } finally {
      isLoadingCount = false;
    }
  });
</script>

<div class="dashboard-container">
  <div class="dashboard-header">
    <div>
      <h1 class="page-title">Autonomous Treasury Dashboard</h1>
      <p class="page-subtitle">Real-time Balance Sheet Optimization and Execution Oversight</p>
    </div>
    <div class="header-actions">
      <button type="button" class="btn btn-secondary" onclick={logout}>
        Sign Out
      </button>
    </div>
  </div>

  {#if $user}
    <div class="profile-card card">
      <div class="profile-header">
        <div>
          <h2 class="profile-name">{$user.full_name || 'System Operator'}</h2>
          <p class="profile-email">{$user.email}</p>
        </div>
        <div class="role-section">
          <span class="role-label">Assigned Role:</span>
          <span class="badge badge-{$user.role}">{$user.role}</span>
        </div>
      </div>

      <div class="permissions-grid">
        <div class="permission-item">
          <span class="perm-title">Read Access</span>
          <span class="perm-desc">View corporate treasuries, strategies, and orchestration runs.</span>
          <span class="perm-status perm-active">Active</span>
        </div>

        <div class="permission-item">
          <span class="perm-title">Write Operations</span>
          <span class="perm-desc">Create treasuries, trigger strategy generations, and orchestrate capital.</span>
          {#if canWrite($user.role)}
            <span class="perm-status perm-active">Authorized (Analyst / Admin)</span>
          {:else}
            <span class="perm-status perm-inactive">Restricted (Viewer)</span>
          {/if}
        </div>

        <div class="permission-item">
          <span class="perm-title">System Administration</span>
          <span class="perm-desc">Manage system users, reassign roles, and inspect security audit logs.</span>
          {#if isAdmin($user.role)}
            <span class="perm-status perm-active">Authorized (Admin)</span>
          {:else}
            <span class="perm-status perm-inactive">Restricted (Admin Only)</span>
          {/if}
        </div>
      </div>
    </div>

    <div class="overview-grid">
      <div class="card stat-card">
        <h3 class="stat-heading">Corporate Treasuries</h3>
        <p class="stat-value">
          {#if isLoadingCount}
            ...
          {:else}
            {treasuryCount}
          {/if}
        </p>
        <p class="stat-note">
          Balance sheets on record. <a href="/treasuries">Manage treasuries</a>
        </p>
      </div>

      <div class="card stat-card">
        <h3 class="stat-heading">AI Decision Engine</h3>
        <p class="stat-value">Integrated</p>
        <p class="stat-note">Member 1 neuro-symbolic heuristic engine connected on :8000.</p>
      </div>

      <div class="card stat-card">
        <h3 class="stat-heading">Security & Auditing</h3>
        <p class="stat-value">Enforced</p>
        <p class="stat-note">Role-based access control and immutable audit logging active.</p>
      </div>
    </div>
  {/if}
</div>

<style>
  .dashboard-container {
    max-width: 1100px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    width: 100%;
  }

  .dashboard-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 2rem;
    gap: 1rem;
  }

  .page-title {
    font-size: 1.75rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .page-subtitle {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .profile-card {
    margin-bottom: 2rem;
  }

  .profile-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    border-bottom: 1px solid var(--color-border);
    padding-bottom: 1.25rem;
    margin-bottom: 1.25rem;
  }

  .profile-name {
    font-size: 1.25rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .profile-email {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    margin-top: 0.125rem;
  }

  .role-section {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .role-label {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .permissions-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 1rem;
  }

  .permission-item {
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem;
    display: flex;
    flex-direction: column;
  }

  .perm-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .perm-desc {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
    line-height: 1.4;
    flex: 1;
    margin-bottom: 0.75rem;
  }

  .perm-status {
    font-size: 0.75rem;
    font-weight: 600;
    padding: 0.25rem 0.5rem;
    border-radius: var(--radius);
    display: inline-block;
    align-self: flex-start;
  }

  .perm-active {
    background-color: var(--color-success-bg);
    color: var(--color-success);
    border: 1px solid var(--color-success-border);
  }

  .perm-inactive {
    background-color: var(--color-surface);
    color: var(--color-text-muted);
    border: 1px solid var(--color-border);
  }

  .overview-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
    gap: 1.25rem;
  }

  .stat-card {
    padding: 1.25rem;
  }

  .stat-heading {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.5rem;
  }

  .stat-value {
    font-size: 1.5rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.375rem;
  }

  .stat-note {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
  }
</style>
