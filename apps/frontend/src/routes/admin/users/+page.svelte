<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { user, me } from '$lib/auth';
  import { isAdmin } from '$lib/roles';
  import { api, ApiError } from '$lib/api';
  import type { UserRecord } from '$lib/types';

  let users = $state<UserRecord[]>([]);
  let isLoading = $state(true);
  let errorMessage = $state<string | null>(null);
  let successMessage = $state<string | null>(null);

  let showConfirmModal = $state(false);
  let selectedUser = $state<UserRecord | null>(null);
  let targetRole = $state<'admin' | 'analyst' | 'viewer'>('viewer');
  let isUpdating = $state(false);

  const availableRoles: Array<'admin' | 'analyst' | 'viewer'> = ['admin', 'analyst', 'viewer'];

  async function fetchUsers() {
    isLoading = true;
    errorMessage = null;
    try {
      const data = await api<UserRecord[]>('/api/v1/users');
      users = data || [];
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        errorMessage = err.status === 403
          ? 'Forbidden: Administrative privileges required to view users.'
          : err.message || 'Failed to retrieve user registry.';
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = 'An unexpected error occurred while loading users.';
      }
    } finally {
      isLoading = false;
    }
  }

  function promptRoleChange(u: UserRecord, newRole: 'admin' | 'analyst' | 'viewer') {
    if (u.role === newRole) return;
    selectedUser = u;
    targetRole = newRole;
    showConfirmModal = true;
    errorMessage = null;
    successMessage = null;
  }

  async function confirmRoleChange() {
    if (!selectedUser) return;
    isUpdating = true;
    errorMessage = null;
    successMessage = null;
    try {
      const updated = await api<UserRecord>(`/api/v1/users/${selectedUser.id}/role`, {
        method: 'PATCH',
        body: { role: targetRole }
      });
      successMessage = `Updated role for ${updated.email} to ${updated.role}.`;
      if ($user && selectedUser.id === $user.id) await me();
      showConfirmModal = false;
      selectedUser = null;
      await fetchUsers();
    } catch (err: unknown) {
      errorMessage = err instanceof ApiError
        ? err.message || 'Failed to update user role.'
        : err instanceof Error ? err.message : 'Role update failed.';
      showConfirmModal = false;
    } finally {
      isUpdating = false;
    }
  }

  onMount(() => {
    if (!isAdmin($user?.role)) {
      goto('/');
      return;
    }
    fetchUsers();
  });
</script>

<svelte:head>
  <title>Users — QUANTA</title>
</svelte:head>

<div class="page-shell">
  <header class="page-head" data-enter>
    <div>
      <h1>User Registry</h1>
      <p>Operator accounts and role-based access scopes.</p>
    </div>
    <div class="page-head-actions">
      <button type="button" class="btn btn-secondary btn-sm" onclick={fetchUsers} disabled={isLoading}>
        Refresh
      </button>
    </div>
  </header>

  {#if successMessage}
    <div class="alert alert-success">{successMessage}</div>
  {/if}
  {#if errorMessage}
    <div class="alert alert-danger row-between">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-xs" onclick={fetchUsers}>Retry</button>
    </div>
  {/if}

  {#if isLoading}
    <div class="empty-msg">Loading users...</div>
  {:else if users.length === 0}
    <div class="empty-msg">No users found in registry.</div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Role</th>
            <th>Reassign role</th>
          </tr>
        </thead>
        <tbody>
          {#each users as u (u.id)}
            <tr>
              <td>
                <span class="font-bold">{u.full_name || 'Operator'}</span>
                {#if $user && u.id === $user.id}
                  <span class="you-note">(you)</span>
                {/if}
              </td>
              <td class="font-mono">{u.email}</td>
              <td><span class="badge badge-{u.role}">{u.role}</span></td>
              <td>
                <select
                  aria-label="Assign role for {u.email}"
                  class="form-select role-select"
                  value={u.role}
                  disabled={isUpdating}
                  onchange={(e) => {
                    const select = e.currentTarget;
                    const val = select.value as 'admin' | 'analyst' | 'viewer';
                    promptRoleChange(u, val);
                    select.value = u.role;
                  }}
                >
                  {#each availableRoles as r}
                    <option value={r} selected={r === u.role}>{r}</option>
                  {/each}
                </select>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="summary-strip cols-3" aria-label="Role permission scopes">
      <div class="summary-cell">
        <span class="summary-label">Admin</span>
        <span class="summary-meta">All portfolios, role assignment, and audit journal inspection.</span>
      </div>
      <div class="summary-cell">
        <span class="summary-label">Analyst</span>
        <span class="summary-meta">Create and modify owned portfolios, run AI risk, stress, and QAOA optimizations.</span>
      </div>
      <div class="summary-cell">
        <span class="summary-label">Viewer</span>
        <span class="summary-meta">Read-only access to assigned treasuries, strategies, and reports.</span>
      </div>
    </div>
  {/if}

  {#if showConfirmModal && selectedUser}
    <div class="modal-backdrop">
      <div class="modal-card">
        <div class="modal-header">
          <div>
            <h2 class="modal-title">Confirm role change</h2>
            <p class="modal-subtitle">{selectedUser.email}</p>
          </div>
        </div>
        <div class="modal-body">
          <div class="reassign-row">
            <span><span class="badge badge-{selectedUser.role}">{selectedUser.role}</span></span>
            <span>&rarr;</span>
            <span><span class="badge badge-{targetRole}">{targetRole}</span></span>
          </div>
          {#if $user && selectedUser.id === $user.id && targetRole !== 'admin'}
            <div class="alert alert-danger" style="margin-top: 1rem;">
              Warning: downgrading your own account removes Admin permissions immediately.
            </div>
          {/if}
        </div>
        <div class="modal-footer">
          <button
            type="button"
            class="btn btn-secondary btn-sm"
            disabled={isUpdating}
            onclick={() => { showConfirmModal = false; selectedUser = null; }}
          >
            Cancel
          </button>
          <button type="button" class="btn btn-primary btn-sm" disabled={isUpdating} onclick={confirmRoleChange}>
            {isUpdating ? 'Updating...' : 'Confirm'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .row-between {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .you-note {
    margin-left: 0.35rem;
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  .role-select {
    width: auto;
    min-width: 130px;
    min-height: 32px;
    padding: 0.2rem 0.5rem;
    font-size: 0.8125rem;
  }

  .summary-strip.cols-3 {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }

  .reassign-row {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .empty-msg {
    padding: 2.25rem 1rem;
    text-align: center;
    color: var(--color-text-muted);
    border-top: 1px solid var(--color-border);
    border-bottom: 1px solid var(--color-border);
  }

  @media (max-width: 768px) {
    .summary-strip.cols-3 {
      grid-template-columns: 1fr;
    }
  }
</style>
