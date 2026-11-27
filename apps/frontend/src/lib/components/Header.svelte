<script lang="ts">
  import { onMount } from 'svelte';
  import { page } from '$app/state';
  import { user, logout } from '$lib/auth';
  import { api } from '$lib/api';
  import type { SystemHealthResponse } from '$lib/types';
  import ThemeToggle from './ThemeToggle.svelte';

  let {
    isMobileOpen = $bindable(false)
  }: {
    isMobileOpen?: boolean;
  } = $props();

  let isStatusOpen = $state(false);
  let isProfileOpen = $state(false);
  let health = $state<SystemHealthResponse | null>(null);
  let healthLatency = $state<number | null>(null);

  async function checkServices() {
    const start = performance.now();
    try {
      health = await api<SystemHealthResponse>('/api/v1/health');
      healthLatency = Math.round(performance.now() - start);
    } catch {
      health = null;
      healthLatency = null;
    }
  }

  onMount(() => {
    checkServices();
  });

  const activeServicesCount = $derived.by(() => {
    if (!health) return 0;
    let count = 1; // Gateway up
    if (health.database === 'up') count += 1;
    if (health.dependencies?.ai_engine === 'up') count += 1;
    if (health.dependencies?.optimization_engine === 'up') count += 1;
    return count;
  });

  const breadcrumbLabel = $derived.by(() => {
    const path = page.url.pathname;
    if (path === '/') return 'Overview';
    if (path.startsWith('/balance-sheet')) return 'Balance Sheet';
    if (path.startsWith('/capital-allocation')) return 'Capital Allocation';
    if (path.startsWith('/risk-analysis')) return 'Risk Analysis';
    if (path.startsWith('/optimization')) return 'Optimization';
    if (path.startsWith('/reports')) return 'Reports';
    if (path.startsWith('/settings')) return 'Settings';
    if (path.startsWith('/treasuries/')) return 'Treasuries / Portfolio';
    if (path.startsWith('/treasuries')) return 'Treasuries';
    if (path.startsWith('/admin/users')) return 'Governance / Users';
    if (path.startsWith('/admin/audit-logs')) return 'Governance / Audit Log';
    return 'Workspace';
  });

  function closeAllMenus() {
    isStatusOpen = false;
    isProfileOpen = false;
  }

  function toggleStatus() {
    isStatusOpen = !isStatusOpen;
    if (isStatusOpen) {
      isProfileOpen = false;
      checkServices();
    }
  }

  function toggleProfile() {
    isProfileOpen = !isProfileOpen;
    if (isProfileOpen) isStatusOpen = false;
  }

  function toggleMobile() {
    isMobileOpen = !isMobileOpen;
  }
</script>

<svelte:window
  onclick={(e) => {
    const target = e.target as HTMLElement;
    if (!target.closest('.status-menu-wrap') && !target.closest('.user-block-wrap')) {
      closeAllMenus();
    }
  }}
  onkeydown={(e) => {
    if (e.key === 'Escape') closeAllMenus();
  }}
/>

<header class="app-header" aria-label="Workspace Bar">
  <div class="header-left">
    <button
      type="button"
      class="mobile-menu-btn"
      onclick={toggleMobile}
      aria-label="Toggle navigation"
    >
      <svg class="hamburger-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" aria-hidden="true">
        <line x1="3" y1="6" x2="21" y2="6" />
        <line x1="3" y1="12" x2="21" y2="12" />
        <line x1="3" y1="18" x2="21" y2="18" />
      </svg>
    </button>

    <nav class="breadcrumb" aria-label="Breadcrumb">
      <a href="/" class="crumb-root">QUANTA</a>
      <span class="crumb-sep">/</span>
      <span class="crumb-current">{breadcrumbLabel}</span>
    </nav>
  </div>

  <div class="header-right">
    <div class="status-menu-wrap">
      <button
        type="button"
        class="status-trigger"
        onclick={toggleStatus}
        aria-expanded={isStatusOpen}
      >
        <span class="status-dot {activeServicesCount === 4 ? 'ok' : activeServicesCount > 0 ? 'warn' : 'err'}"></span>
        <span>Services {activeServicesCount}/4</span>
      </button>

      {#if isStatusOpen}
        <div class="menu-popover status-popover" role="region" aria-label="Service Diagnostics">
          <div class="popover-head">
            <span>Cluster status</span>
            {#if healthLatency !== null}
              <span class="font-mono">{healthLatency}ms</span>
            {/if}
          </div>
          <div class="service-rows">
            <div class="service-row">
              <span>Go Gateway (:8080)</span>
              <span class="svc-state {health ? 'up' : 'down'}">{health ? 'Online' : 'Offline'}</span>
            </div>
            <div class="service-row">
              <span>PostgreSQL (:5432)</span>
              <span class="svc-state {health?.database === 'up' ? 'up' : 'down'}">{health?.database === 'up' ? 'Connected' : 'Down'}</span>
            </div>
            <div class="service-row">
              <span>AI Engine (:8000)</span>
              <span class="svc-state {health?.dependencies?.ai_engine === 'up' ? 'up' : 'down'}">{health?.dependencies?.ai_engine === 'up' ? 'Active' : 'Offline'}</span>
            </div>
            <div class="service-row">
              <span>Optimizer (:8001)</span>
              <span class="svc-state {health?.dependencies?.optimization_engine === 'up' ? 'up' : 'down'}">{health?.dependencies?.optimization_engine === 'up' ? 'Active' : 'Offline'}</span>
            </div>
          </div>
        </div>
      {/if}
    </div>

    <ThemeToggle />

    {#if $user}
      <div class="user-block-wrap">
        <button
          type="button"
          class="user-block-btn"
          onclick={toggleProfile}
          aria-expanded={isProfileOpen}
        >
          <span class="user-avatar-badge">{$user.email.slice(0, 2).toUpperCase()}</span>
          <span class="user-name">{$user.full_name || $user.email.split('@')[0]}</span>
          <span class="user-role-text">{$user.role}</span>
          <svg class="dropdown-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true">
            <path d="M4 6l4 4 4-4" />
          </svg>
        </button>

        {#if isProfileOpen}
          <div class="menu-popover profile-popover" role="menu">
            <div class="profile-summary">
              <div class="p-name">{$user.full_name || 'Operator'}</div>
              <div class="p-email font-mono">{$user.email}</div>
            </div>
            <div class="popover-links">
              <a href="/settings" class="menu-item-link" onclick={() => (isProfileOpen = false)}>Settings</a>
              <a href="/treasuries" class="menu-item-link" onclick={() => (isProfileOpen = false)}>Treasuries</a>
              {#if $user.role === 'admin'}
                <a href="/admin/audit-logs" class="menu-item-link" onclick={() => (isProfileOpen = false)}>Audit Log</a>
              {/if}
            </div>
            <div class="popover-foot">
              <button type="button" class="btn-signout" onclick={logout}>Sign out</button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  .app-header {
    height: var(--header-height);
    background-color: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 2rem;
    position: sticky;
    top: 0;
    z-index: 100;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .mobile-menu-btn {
    display: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0.35rem;
    cursor: pointer;
    color: var(--color-text);
  }

  .hamburger-icon {
    width: 18px;
    height: 18px;
  }

  .breadcrumb {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 0.875rem;
  }

  .crumb-root {
    color: var(--color-text-muted);
    font-weight: 500;
  }

  .crumb-root:hover {
    color: var(--color-text);
  }

  .crumb-sep {
    color: var(--color-border-strong);
  }

  .crumb-current {
    color: var(--color-text);
    font-weight: 500;
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 0.625rem;
  }

  .status-menu-wrap,
  .user-block-wrap {
    position: relative;
  }

  .status-trigger {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    min-height: 32px;
    padding: 0 0.65rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: 0.8125rem;
    font-family: var(--font-mono);
    color: var(--color-text-secondary);
    cursor: pointer;
  }

  .status-trigger:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-strong);
    color: var(--color-text);
  }

  .menu-popover {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    z-index: 150;
    overflow: hidden;
  }

  .status-popover {
    width: 260px;
  }

  .popover-head {
    display: flex;
    justify-content: space-between;
    padding: 0.625rem 0.875rem;
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-bg-secondary);
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .service-rows {
    padding: 0.5rem 0.875rem;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }

  .service-row {
    display: flex;
    justify-content: space-between;
    font-size: 0.8125rem;
    color: var(--color-text-secondary);
  }

  .svc-state {
    font-family: var(--font-mono);
    font-weight: 500;
  }

  .svc-state.up { color: var(--color-success); }
  .svc-state.down { color: var(--color-danger); }

  .user-block-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.5rem;
    min-height: 32px;
    padding: 0 0.55rem 0 0.3rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    cursor: pointer;
    color: var(--color-text);
  }

  .user-block-btn:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-strong);
  }

  .user-avatar-badge {
    width: 22px;
    height: 22px;
    border-radius: 3px;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 600;
    font-family: var(--font-mono);
  }

  .user-name {
    font-size: 0.8125rem;
    font-weight: 500;
  }

  .user-role-text {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    text-transform: capitalize;
  }

  .dropdown-chevron {
    width: 12px;
    height: 12px;
    color: var(--color-text-muted);
  }

  .profile-popover {
    width: 220px;
  }

  .profile-summary {
    padding: 0.75rem 0.875rem;
    border-bottom: 1px solid var(--color-border);
    background-color: var(--color-bg-secondary);
  }

  .p-name {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .p-email {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    margin-top: 0.15rem;
    word-break: break-all;
  }

  .popover-links {
    padding: 0.35rem;
    display: flex;
    flex-direction: column;
  }

  .menu-item-link {
    padding: 0.45rem 0.55rem;
    font-size: 0.8125rem;
    color: var(--color-text);
    border-radius: var(--radius-sm);
  }

  .menu-item-link:hover {
    background-color: var(--color-surface-hover);
    color: var(--color-text);
  }

  .popover-foot {
    padding: 0.35rem;
    border-top: 1px solid var(--color-border);
  }

  .btn-signout {
    width: 100%;
    text-align: left;
    padding: 0.45rem 0.55rem;
    background: transparent;
    border: none;
    border-radius: var(--radius-sm);
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-danger);
    cursor: pointer;
  }

  .btn-signout:hover {
    background-color: var(--color-danger-bg);
  }

  @media (max-width: 1024px) {
    .mobile-menu-btn {
      display: inline-flex;
    }
    .status-menu-wrap {
      display: none;
    }
  }

  @media (max-width: 640px) {
    .app-header {
      padding: 0 1rem;
    }
    .user-name,
    .user-role-text {
      display: none;
    }
  }
</style>
