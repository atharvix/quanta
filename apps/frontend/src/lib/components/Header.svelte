<script lang="ts">
  import { page } from '$app/state';
  import { user, logout } from '$lib/auth';
  import { isAdmin } from '$lib/roles';
  import ThemeToggle from './ThemeToggle.svelte';

  let {
    isMobileOpen = $bindable(false)
  }: {
    isMobileOpen?: boolean;
  } = $props();

  // Notification state
  interface SystemNotification {
    id: string;
    title: string;
    message: string;
    timestamp: string;
    type: 'info' | 'success' | 'warning';
    read: boolean;
  }

  let isNotifOpen = $state(false);
  let isProfileOpen = $state(false);

  let notifications = $state<SystemNotification[]>([
    {
      id: 'n1',
      title: 'Gateway Cluster Active',
      message: 'Go Core API Gateway on port 8080 managing authenticated requests.',
      timestamp: 'Live',
      type: 'success',
      read: false
    },
    {
      id: 'n2',
      title: 'Storage Concurrency Protection',
      message: 'PostgreSQL advisory lock isolation enabled for treasury updates.',
      timestamp: 'Active',
      type: 'info',
      read: true
    }
  ]);

  const unreadCount = $derived(notifications.filter((n) => !n.read).length);

  function markAllRead() {
    notifications = notifications.map((n) => ({ ...n, read: true }));
  }

  function toggleNotif() {
    isNotifOpen = !isNotifOpen;
    if (isNotifOpen) isProfileOpen = false;
  }

  function toggleProfile() {
    isProfileOpen = !isProfileOpen;
    if (isProfileOpen) isNotifOpen = false;
  }

  function closeAllMenus() {
    isNotifOpen = false;
    isProfileOpen = false;
  }

  // Compute section title from current pathname
  const sectionInfo = $derived.by(() => {
    const path = page.url.pathname;
    if (path === '/') {
      return { title: 'Dashboard Overview', subtitle: 'Capital, Balance Sheet & Telemetry' };
    }
    if (path.startsWith('/balance-sheet')) {
      return { title: 'Balance Sheet Engine', subtitle: 'Consolidated Assets, Liabilities & Liquidity' };
    }
    if (path.startsWith('/capital-allocation')) {
      return { title: 'Capital Allocation', subtitle: 'Target Weights, Rebalancing & Asset Distribution' };
    }
    if (path.startsWith('/risk-analysis')) {
      return { title: 'Risk Intelligence', subtitle: 'Bayesian Inference, Minimax & Stress Testing' };
    }
    if (path.startsWith('/optimization')) {
      return { title: 'Optimization Engine', subtitle: 'Heuristic Search & Numerical Solvers' };
    }
    if (path.startsWith('/reports')) {
      return { title: 'Financial Reports', subtitle: 'Institutional Statements & Audit Exports' };
    }
    if (path.startsWith('/settings')) {
      return { title: 'Platform Settings', subtitle: 'Operator Preferences & Telemetry Configuration' };
    }
    if (path.startsWith('/treasuries/')) {
      return { title: 'Treasury Portfolio', subtitle: 'Holdings, Balance Sheet & Risk' };
    }
    if (path.startsWith('/treasuries')) {
      return { title: 'Corporate Treasuries', subtitle: 'Active Portfolios & Allocation Profiles' };
    }
    if (path.startsWith('/admin/users')) {
      return { title: 'User Registry', subtitle: 'Role-Based Access Control' };
    }
    if (path.startsWith('/admin/audit-logs')) {
      return { title: 'Security Audit Logs', subtitle: 'Immutable System Event Journal' };
    }
    return { title: 'QUANTA', subtitle: 'Autonomous Financial Treasury' };
  });

  function toggleMobile() {
    isMobileOpen = !isMobileOpen;
  }
</script>

<svelte:window onclick={(e) => {
  // Close popups if clicked outside
  const target = e.target as HTMLElement;
  if (!target.closest('.notification-wrap') && !target.closest('.user-block-wrap')) {
    closeAllMenus();
  }
}} onkeydown={(e) => {
  if (e.key === 'Escape') closeAllMenus();
}} />

<header class="app-header" aria-label="Top Navigation">
  <div class="header-left">
    <!-- Mobile hamburger button -->
    <button
      type="button"
      class="mobile-menu-btn"
      onclick={toggleMobile}
      aria-label="Toggle navigation menu"
      title="Toggle navigation menu"
    >
      <svg class="hamburger-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <line x1="3" y1="6" x2="21" y2="6" stroke-linecap="round" />
        <line x1="3" y1="12" x2="21" y2="12" stroke-linecap="round" />
        <line x1="3" y1="18" x2="21" y2="18" stroke-linecap="round" />
      </svg>
    </button>

    <!-- Section Context / Breadcrumb -->
    <div class="header-breadcrumb">
      <div class="breadcrumb-details">
        <h1 class="header-title">{sectionInfo.title}</h1>
        <span class="header-subtitle">{sectionInfo.subtitle}</span>
      </div>
    </div>
  </div>

  <div class="header-right">
    <!-- Telemetry status pill -->
    <div class="telemetry-pill" title="Cluster Gateway Active">
      <span class="status-pulse-dot"></span>
      <span class="telemetry-text">GATEWAY ONLINE</span>
    </div>

    <!-- Notification Indicator with Dropdown Panel -->
    <div class="notification-wrap">
      <button
        type="button"
        class="notification-btn"
        onclick={toggleNotif}
        aria-label="Notifications"
        aria-expanded={isNotifOpen}
        title="{unreadCount} unread system notifications"
      >
        <svg class="header-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8">
          <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
          <path d="M13.73 21a2 2 0 0 1-3.46 0" />
        </svg>
        {#if unreadCount > 0}
          <span class="notif-badge">{unreadCount}</span>
        {/if}
      </button>

      {#if isNotifOpen}
        <div class="notifications-dropdown" role="region" aria-label="System Notifications">
          <div class="notif-dropdown-header">
            <div class="notif-header-title">
              <span class="notif-heading">Notifications</span>
              <span class="notif-count-pill">{unreadCount} new</span>
            </div>
            {#if unreadCount > 0}
              <button type="button" class="notif-mark-read-btn" onclick={markAllRead}>
                Mark all read
              </button>
            {/if}
          </div>

          <div class="notif-items-list">
            {#each notifications as notif}
              <div class="notif-item" class:unread={!notif.read}>
                <div class="notif-item-top">
                  <span class="notif-status-dot dot-{notif.type}"></span>
                  <span class="notif-item-title">{notif.title}</span>
                  <span class="notif-item-time">{notif.timestamp}</span>
                </div>
                <p class="notif-item-msg">{notif.message}</p>
              </div>
            {/each}
          </div>

          {#if isAdmin($user?.role)}
            <div class="notif-dropdown-footer">
              <a href="/admin/audit-logs" class="notif-footer-link" onclick={() => (isNotifOpen = false)}>
                View Security Audit Journal &rarr;
              </a>
            </div>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Theme Switcher -->
    <ThemeToggle />

    <!-- User Profile & Dropdown -->
    {#if $user}
      <div class="user-block-wrap">
        <button
          type="button"
          class="user-block-btn"
          onclick={toggleProfile}
          aria-expanded={isProfileOpen}
          title="Account profile and settings"
        >
          <div class="user-avatar-badge">
            {$user.email.slice(0, 2).toUpperCase()}
          </div>
          <div class="user-meta">
            <span class="user-name">{$user.full_name || $user.email.split('@')[0]}</span>
            <span class="badge badge-{$user.role}">{$user.role}</span>
          </div>
          <svg class="dropdown-chevron" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.8">
            <path d="M4 6l4 4 4-4" />
          </svg>
        </button>

        {#if isProfileOpen}
          <div class="profile-dropdown-menu" role="menu">
            <div class="profile-dropdown-header">
              <span class="p-name">{$user.full_name || 'System Operator'}</span>
              <span class="p-email">{$user.email}</span>
              <div class="p-role-row">
                <span class="role-caption">Role:</span>
                <span class="badge badge-{$user.role}">{$user.role.toUpperCase()}</span>
              </div>
            </div>

            <div class="profile-dropdown-links">
              <a href="/settings" class="profile-menu-item" onclick={() => (isProfileOpen = false)}>
                <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
                  <circle cx="8" cy="8" r="2.5" />
                  <path d="M8 1v2m0 10v2M1 8h2m10 0h2m-2.5-4.5l-1.4 1.4m-7 7l-1.4 1.4m0-9.8l1.4 1.4m7 7l1.4 1.4" />
                </svg>
                <span>Platform Settings</span>
              </a>

              <a href="/treasuries" class="profile-menu-item" onclick={() => (isProfileOpen = false)}>
                <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
                  <rect x="2" y="3" width="12" height="10" rx="1.5" />
                  <line x1="2" y1="7" x2="14" y2="7" />
                </svg>
                <span>Corporate Portfolios</span>
              </a>

              {#if $user.role === 'admin'}
                <a href="/admin/audit-logs" class="profile-menu-item" onclick={() => (isProfileOpen = false)}>
                  <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
                    <path d="M8 1v14M3 6h10M3 11h10" />
                  </svg>
                  <span>Audit Journal</span>
                </a>
              {/if}
            </div>

            <div class="profile-dropdown-footer">
              <button type="button" class="btn-dropdown-logout" onclick={logout}>
                <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.75">
                  <path d="M6 14H3a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1h3" />
                  <path d="M11 11l3-3-3-3" />
                  <line x1="14" y1="8" x2="6" y2="8" />
                </svg>
                <span>Sign Out</span>
              </button>
            </div>
          </div>
        {/if}
      </div>
    {/if}
  </div>
</header>

<style>
  .app-header {
    height: var(--header-height, 64px);
    background-color: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 1.75rem;
    position: sticky;
    top: 0;
    z-index: 100;
    box-sizing: border-box;
  }

  .header-left {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .mobile-menu-btn {
    display: none;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    padding: 0.45rem;
    cursor: pointer;
    color: var(--color-text);
  }

  .hamburger-icon {
    width: 20px;
    height: 20px;
  }

  .header-breadcrumb {
    display: flex;
    align-items: center;
  }

  .breadcrumb-details {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .header-title {
    font-size: 1.0625rem;
    font-weight: 700;
    color: var(--color-text);
    margin: 0;
    line-height: 1.2;
  }

  .header-subtitle {
    font-size: 0.75rem;
    color: var(--color-text-secondary);
  }

  .header-right {
    display: flex;
    align-items: center;
    gap: 1rem;
    position: relative;
  }

  .telemetry-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.35rem 0.65rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    font-size: 0.6875rem;
    font-weight: 700;
    color: var(--color-text-secondary);
    letter-spacing: 0.04em;
    font-family: var(--font-mono, monospace);
  }

  .status-pulse-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-success);
  }

  /* Notifications */
  .notification-wrap {
    position: relative;
  }

  .notification-btn {
    position: relative;
    background: transparent;
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    padding: 0.45rem;
    cursor: pointer;
    color: var(--color-text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    transition: background-color 0.15s ease, color 0.15s ease;
  }

  .notification-btn:hover {
    background-color: var(--color-surface-hover);
    color: var(--color-text);
  }

  .header-icon {
    width: 18px;
    height: 18px;
  }

  .notif-badge {
    position: absolute;
    top: -4px;
    right: -4px;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
    font-size: 0.5625rem;
    font-weight: 800;
    width: 15px;
    height: 15px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    line-height: 1;
  }

  /* Notifications Dropdown Panel */
  .notifications-dropdown {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    width: 320px;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    z-index: 150;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .notif-dropdown-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    background-color: var(--color-bg-secondary);
  }

  .notif-header-title {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .notif-heading {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .notif-count-pill {
    font-size: 0.625rem;
    font-weight: 700;
    padding: 0.1rem 0.35rem;
    border-radius: 3px;
    background-color: var(--color-primary);
    color: var(--color-primary-text);
  }

  .notif-mark-read-btn {
    background: transparent;
    border: none;
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--color-primary);
    cursor: pointer;
    padding: 0;
  }

  .notif-mark-read-btn:hover {
    text-decoration: underline;
  }

  .notif-items-list {
    max-height: 280px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
  }

  .notif-item {
    padding: 0.75rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
    transition: background-color 0.15s ease;
  }

  .notif-item:last-child {
    border-bottom: none;
  }

  .notif-item.unread {
    background-color: rgba(49, 95, 234, 0.04);
  }

  .notif-item-top {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .notif-status-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
  }

  .dot-success { background-color: var(--color-success); }
  .dot-info { background-color: var(--color-primary); }
  .dot-warning { background-color: var(--color-warning); }

  .notif-item-title {
    font-size: 0.75rem;
    font-weight: 700;
    color: var(--color-text);
    flex: 1;
  }

  .notif-item-time {
    font-size: 0.625rem;
    color: var(--color-text-muted);
  }

  .notif-item-msg {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
    line-height: 1.35;
    margin: 0;
  }

  .notif-dropdown-footer {
    padding: 0.625rem 1rem;
    border-top: 1px solid var(--color-border-subtle);
    background-color: var(--color-bg-secondary);
    text-align: center;
  }

  .notif-footer-link {
    font-size: 0.6875rem;
    font-weight: 600;
    color: var(--color-primary);
    text-decoration: none;
  }

  .notif-footer-link:hover {
    text-decoration: underline;
  }

  /* User Block & Dropdown */
  .user-block-wrap {
    position: relative;
  }

  .user-block-btn {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.35rem 0.6rem;
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .user-block-btn:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-focus);
  }

  .user-avatar-badge {
    width: 26px;
    height: 26px;
    border-radius: 4px;
    background-color: #0F172A;
    color: #F8FAFC;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.6875rem;
    font-weight: 700;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="dark"]) .user-avatar-badge {
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
  }

  .user-meta {
    display: flex;
    align-items: center;
    gap: 0.4rem;
  }

  .user-name {
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .dropdown-chevron {
    width: 12px;
    height: 12px;
    color: var(--color-text-muted);
  }

  /* Profile Dropdown Menu */
  .profile-dropdown-menu {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    width: 240px;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    z-index: 150;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .profile-dropdown-header {
    padding: 0.875rem 1rem;
    border-bottom: 1px solid var(--color-border-subtle);
    background-color: var(--color-bg-secondary);
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
  }

  .p-name {
    font-size: 0.8125rem;
    font-weight: 700;
    color: var(--color-text);
  }

  .p-email {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
    font-family: var(--font-mono, monospace);
    word-break: break-all;
  }

  .p-role-row {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-top: 0.35rem;
  }

  .role-caption {
    font-size: 0.6875rem;
    color: var(--color-text-secondary);
  }

  .profile-dropdown-links {
    padding: 0.5rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .profile-menu-item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.5rem 0.625rem;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-text);
    text-decoration: none;
    border-radius: var(--radius-sm);
    transition: background-color 0.15s ease;
  }

  .profile-menu-item:hover {
    background-color: var(--color-bg-secondary);
  }

  .profile-menu-item svg {
    width: 14px;
    height: 14px;
    color: var(--color-text-secondary);
  }

  .profile-dropdown-footer {
    padding: 0.5rem;
    border-top: 1px solid var(--color-border-subtle);
  }

  .btn-dropdown-logout {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    width: 100%;
    padding: 0.5rem 0.625rem;
    background: transparent;
    border: none;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--color-danger);
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: background-color 0.15s ease;
  }

  .btn-dropdown-logout:hover {
    background-color: var(--color-danger-bg);
  }

  .btn-dropdown-logout svg {
    width: 14px;
    height: 14px;
  }

  @media (max-width: 1024px) {
    .mobile-menu-btn {
      display: flex;
    }

    .telemetry-pill {
      display: none;
    }
  }

  @media (max-width: 640px) {
    .app-header {
      padding: 0 1rem;
    }

    .user-name {
      display: none;
    }
  }
</style>
