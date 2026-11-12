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

  // Role modification confirmation state
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
        if (err.status === 403) {
          errorMessage = 'Forbidden: Administrative privileges required to view users.';
        } else {
          errorMessage = err.message || 'Failed to retrieve user registry.';
        }
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

      successMessage = `Access role for "${updated.email}" updated to "${updated.role}".`;

      // If the authenticated admin modified their own role, refresh local state
      if ($user && selectedUser.id === $user.id) {
        await me();
      }

      showConfirmModal = false;
      selectedUser = null;
      await fetchUsers();
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        errorMessage = err.message || 'Failed to update user role.';
      } else if (err instanceof Error) {
        errorMessage = err.message;
      } else {
        errorMessage = 'An unexpected error occurred during role reassignment.';
      }
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

<div class="page-container">
  <div class="page-header">
    <div>
      <h1 class="page-title">User Management</h1>
      <p class="page-subtitle">Inspect registered operator identities and configure role permissions</p>
    </div>
    <div>
      <button type="button" class="btn btn-secondary btn-sm" onclick={fetchUsers} disabled={isLoading}>
        Refresh
      </button>
    </div>
  </div>

  {#if successMessage}
    <div class="alert alert-success">
      {successMessage}
    </div>
  {/if}

  {#if errorMessage}
    <div class="alert alert-danger" style="display: flex; justify-content: space-between; align-items: center;">
      <span>{errorMessage}</span>
      <button type="button" class="btn btn-secondary btn-sm" onclick={fetchUsers}>
        Retry
      </button>
    </div>
  {/if}

  {#if isLoading}
    <div class="card empty-state">
      <p class="state-message">Loading platform user registry...</p>
    </div>
  {:else if users.length === 0}
    <div class="card empty-state">
      <h3 class="state-title">No Users Found</h3>
      <p class="state-message">No user accounts are registered in the database.</p>
    </div>
  {:else}
    <div class="table-container">
      <table class="table">
        <thead>
          <tr>
            <th>Full Name</th>
            <th>Email Address</th>
            <th>Current Role</th>
            <th>Assign New Role</th>
          </tr>
        </thead>
        <tbody>
          {#each users as u (u.id)}
            <tr>
              <td>
                <span class="user-fullname">{u.full_name || 'System Operator'}</span>
                {#if $user && u.id === $user.id}
                  <span class="badge badge-viewer" style="margin-left: 0.5rem; font-size: 0.6875rem;">You</span>
                {/if}
              </td>
              <td class="user-email-cell">
                <code>{u.email}</code>
              </td>
              <td>
                <span class="badge badge-{u.role}">
                  {u.role}
                </span>
              </td>
              <td>
                <div class="role-control-wrap">
                  <select
                    class="form-input role-select"
                    value={u.role}
                    disabled={isUpdating}
                    onchange={(e) => {
                      const select = e.currentTarget;
                      const val = select.value as 'admin' | 'analyst' | 'viewer';
                      promptRoleChange(u, val);
                      select.value = u.role; // Keep visually in sync until confirmed
                    }}
                  >
                    {#each availableRoles as r}
                      <option value={r} selected={r === u.role}>{r}</option>
                    {/each}
                  </select>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

    <div class="role-legend card" style="margin-top: 1.5rem;">
      <h3 class="legend-title">Role Permission Reference</h3>
      <div class="legend-grid">
        <div class="legend-item">
          <span class="badge badge-admin">admin</span>
          <p class="legend-desc">Full system access: Manage operator accounts, reassign roles, and inspect security audit logs.</p>
        </div>
        <div class="legend-item">
          <span class="badge badge-analyst">analyst</span>
          <p class="legend-desc">Operational write access: Onboard treasuries, execute AI strategies, and orchestrate capital allocations.</p>
        </div>
        <div class="legend-item">
          <span class="badge badge-viewer">viewer</span>
          <p class="legend-desc">Read-only oversight: Inspect existing corporate treasuries, strategies, and historical runs.</p>
        </div>
      </div>
    </div>
  {/if}

  <!-- Confirmation Modal -->
  {#if showConfirmModal && selectedUser}
    <div class="modal-backdrop">
      <div class="modal-card card">
        <h2 class="modal-title">Confirm Role Reassignment</h2>
        <p class="modal-message">
          Are you sure you want to reassign the access level for <strong>{selectedUser.email}</strong>?
        </p>

        <div class="reassign-details">
          <div>
            <span class="detail-label">Current Role</span>
            <span class="badge badge-{selectedUser.role}">{selectedUser.role}</span>
          </div>
          <div class="arrow-indicator">➔</div>
          <div>
            <span class="detail-label">Target Role</span>
            <span class="badge badge-{targetRole}">{targetRole}</span>
          </div>
        </div>

        {#if $user && selectedUser.id === $user.id && targetRole !== 'admin'}
          <div class="alert alert-danger" style="margin-top: 1rem; margin-bottom: 0;">
            Warning: You are modifying your own account role. If downgraded, you will forfeit administrative access immediately.
          </div>
        {/if}

        <div class="modal-actions">
          <button
            type="button"
            class="btn btn-secondary"
            disabled={isUpdating}
            onclick={() => {
              showConfirmModal = false;
              selectedUser = null;
            }}
          >
            Cancel
          </button>
          <button
            type="button"
            class="btn btn-primary"
            disabled={isUpdating}
            onclick={confirmRoleChange}
          >
            {isUpdating ? 'Updating...' : 'Confirm Reassignment'}
          </button>
        </div>
      </div>
    </div>
  {/if}
</div>

<style>
  .page-container {
    max-width: 1100px;
    margin: 0 auto;
    padding: 2rem 1.5rem;
    width: 100%;
  }

  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 1.5rem;
    gap: 1rem;
    flex-wrap: wrap;
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

  .user-fullname {
    font-weight: 600;
    color: var(--color-text);
  }

  .user-email-cell code {
    font-size: 0.8125rem;
  }

  .role-control-wrap {
    max-width: 160px;
  }

  .role-select {
    padding: 0.25rem 0.5rem;
    font-size: 0.8125rem;
    height: 32px;
  }

  .empty-state {
    text-align: center;
    padding: 3rem 1.5rem;
  }

  .state-title {
    font-size: 1.125rem;
    font-weight: 600;
    color: var(--color-text);
    margin-bottom: 0.25rem;
  }

  .state-message {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .role-legend {
    padding: 1.25rem;
  }

  .legend-title {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--color-text);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin-bottom: 0.75rem;
  }

  .legend-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
    gap: 1rem;
  }

  .legend-item {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .legend-desc {
    font-size: 0.8125rem;
    color: var(--color-text-muted);
    line-height: 1.4;
  }

  /* Modal Styles */
  .modal-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    width: 100vw;
    height: 100vh;
    background-color: rgba(15, 23, 42, 0.6);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 100;
    padding: 1rem;
  }

  .modal-card {
    width: 100%;
    max-width: 460px;
    background-color: var(--color-surface);
    box-shadow: var(--shadow-md);
  }

  .modal-title {
    font-size: 1.25rem;
    font-weight: 700;
    color: var(--color-text);
    margin-bottom: 0.5rem;
  }

  .modal-message {
    font-size: 0.875rem;
    color: var(--color-text-muted);
    line-height: 1.5;
  }

  .reassign-details {
    display: flex;
    align-items: center;
    justify-content: space-around;
    background-color: var(--color-bg);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 1rem;
    margin-top: 1rem;
  }

  .arrow-indicator {
    font-size: 1.125rem;
    color: var(--color-text-muted);
  }

  .modal-actions {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    margin-top: 1.5rem;
  }
</style>

