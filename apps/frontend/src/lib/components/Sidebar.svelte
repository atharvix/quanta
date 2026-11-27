<script lang="ts">
  import { page } from "$app/state";
  import { user } from "$lib/auth";
  import { isAdmin } from "$lib/roles";
  import { browser } from "$app/env";
  import QuantaMonogram from "./QuantaMonogram.svelte";

  let {
    isCollapsed = $bindable(false),
    isMobileOpen = $bindable(false),
  }: {
    isCollapsed?: boolean;
    isMobileOpen?: boolean;
  } = $props();

  function toggleCollapse() {
    isCollapsed = !isCollapsed;
    if (browser) {
      localStorage.setItem(
        "quanta_sidebar_collapsed",
        isCollapsed ? "true" : "false",
      );
    }
  }

  function closeMobile() {
    isMobileOpen = false;
  }
</script>

{#if isMobileOpen}
  <div
    class="mobile-backdrop"
    onclick={closeMobile}
    onkeydown={(e) => {
      if (e.key === "Escape") closeMobile();
    }}
    role="presentation"
  ></div>
{/if}

<aside
  class="sidebar"
  class:collapsed={isCollapsed}
  class:mobile-open={isMobileOpen}
  aria-label="Primary Sidebar"
>
  <div class="sidebar-brand">
    <a href="/" class="brand-link" onclick={closeMobile}>
      <QuantaMonogram size={28} />
      {#if !isCollapsed}
        <span class="brand-name">QUANTA</span>
      {/if}
    </a>
  </div>

  <nav class="sidebar-nav" aria-label="Workspace Navigation">
    <div class="nav-section-label">
      {#if !isCollapsed}
        <span>Platform</span>
      {:else}
        <span class="collapsed-divider"></span>
      {/if}
    </div>

    <a
      href="/"
      class="nav-item"
      class:active={page.url.pathname === "/"}
      onclick={closeMobile}
      title="Overview"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <rect x="3" y="3" width="7" height="9" rx="1" />
        <rect x="14" y="3" width="7" height="5" rx="1" />
        <rect x="14" y="12" width="7" height="9" rx="1" />
        <rect x="3" y="16" width="7" height="5" rx="1" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Overview</span>
      {/if}
    </a>

    <a
      href="/treasuries"
      class="nav-item"
      class:active={page.url.pathname === "/treasuries" || page.url.pathname.startsWith("/treasuries/")}
      onclick={closeMobile}
      title="Treasuries"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <rect x="3" y="6" width="18" height="14" rx="1.5" />
        <path d="M8 6V4h8v2" />
        <path d="M3 11h18" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Treasuries</span>
      {/if}
    </a>

    <a
      href="/balance-sheet"
      class="nav-item"
      class:active={page.url.pathname === "/balance-sheet"}
      onclick={closeMobile}
      title="Balance Sheet"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <line x1="12" y1="3" x2="12" y2="21" />
        <path d="M5 8h14" />
        <path d="M3 13l3-6 3 6H3z" />
        <path d="M15 17l3-6 3 6h-6z" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Balance Sheet</span>
      {/if}
    </a>

    <a
      href="/capital-allocation"
      class="nav-item"
      class:active={page.url.pathname === "/capital-allocation"}
      onclick={closeMobile}
      title="Capital Allocation"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <path d="M21.21 15.89A10 10 0 1 1 8 2.83" />
        <path d="M22 12A10 10 0 0 0 12 2v10z" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Capital Allocation</span>
      {/if}
    </a>

    <a
      href="/risk-analysis"
      class="nav-item"
      class:active={page.url.pathname === "/risk-analysis"}
      onclick={closeMobile}
      title="Risk Analysis"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        <line x1="12" y1="8" x2="12" y2="12" />
        <line x1="12" y1="16" x2="12.01" y2="16" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Risk Analysis</span>
      {/if}
    </a>

    <a
      href="/optimization"
      class="nav-item"
      class:active={page.url.pathname === "/optimization"}
      onclick={closeMobile}
      title="Optimization"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <polyline points="4 17 10 11 13 14 20 7" />
        <polyline points="14 7 20 7 20 13" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Optimization</span>
      {/if}
    </a>

    <a
      href="/reports"
      class="nav-item"
      class:active={page.url.pathname === "/reports"}
      onclick={closeMobile}
      title="Reports"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
        <line x1="16" y1="13" x2="8" y2="13" />
        <line x1="16" y1="17" x2="8" y2="17" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Reports</span>
      {/if}
    </a>

    <div class="nav-section-label admin-section">
      {#if !isCollapsed}
        <span>Governance</span>
      {:else}
        <span class="collapsed-divider"></span>
      {/if}
    </div>

    {#if $user && isAdmin($user.role)}
      <a
        href="/admin/audit-logs"
        class="nav-item"
        class:active={page.url.pathname.startsWith("/admin/audit-logs")}
        onclick={closeMobile}
        title="Audit Log"
      >
        <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
          <path d="M9 11l3 3L22 4" />
          <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
        </svg>
        {#if !isCollapsed}
          <span class="nav-label">Audit Log</span>
        {/if}
      </a>

      <a
        href="/admin/users"
        class="nav-item"
        class:active={page.url.pathname.startsWith("/admin/users")}
        onclick={closeMobile}
        title="Users"
      >
        <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
          <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
        </svg>
        {#if !isCollapsed}
          <span class="nav-label">Users</span>
        {/if}
      </a>
    {/if}

    <a
      href="/settings"
      class="nav-item"
      class:active={page.url.pathname === "/settings"}
      onclick={closeMobile}
      title="Settings"
    >
      <svg class="nav-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" aria-hidden="true">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 1 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 1 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 1 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 1 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Settings</span>
      {/if}
    </a>
  </nav>

  <div class="sidebar-footer">
    {#if $user}
      <div class="operator-row" class:compact={isCollapsed}>
        <div class="operator-avatar" title="{$user.email} ({$user.role})">
          {$user.email.slice(0, 2).toUpperCase()}
        </div>
        {#if !isCollapsed}
          <div class="operator-info">
            <span class="operator-name">{$user.full_name || $user.email.split('@')[0]}</span>
            <span class="operator-meta">{$user.role} &middot; {$user.email}</span>
          </div>
        {/if}
      </div>
    {/if}

    <button
      type="button"
      class="collapse-btn"
      onclick={toggleCollapse}
      aria-label={isCollapsed ? "Expand rail" : "Collapse rail"}
      title={isCollapsed ? "Expand rail" : "Collapse rail"}
    >
      <svg class="collapse-icon" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        {#if isCollapsed}
          <path fill-rule="evenodd" d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z" clip-rule="evenodd" />
        {:else}
          <path fill-rule="evenodd" d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z" clip-rule="evenodd" />
        {/if}
      </svg>
      {#if !isCollapsed}
        <span>Collapse sidebar</span>
      {/if}
    </button>
  </div>
</aside>

<style>
  .mobile-backdrop {
    position: fixed;
    inset: 0;
    background-color: rgba(11, 13, 16, 0.55);
    z-index: 199;
  }

  .sidebar {
    width: var(--sidebar-width);
    height: 100vh;
    background-color: var(--color-surface);
    border-right: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 200;
    transition: width 160ms var(--ease), transform 160ms var(--ease);
  }

  .sidebar.collapsed {
    width: var(--sidebar-collapsed-width);
  }

  .sidebar-brand {
    height: var(--header-height);
    display: flex;
    align-items: center;
    padding: 0 1rem;
    border-bottom: 1px solid var(--color-border);
    flex-shrink: 0;
  }

  .brand-link {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    color: var(--color-text);
    overflow: hidden;
  }

  .brand-name {
    font-size: 1rem;
    font-weight: 600;
    letter-spacing: -0.02em;
  }

  .sidebar-nav {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 0.75rem 0.625rem;
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .nav-section-label {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-text-muted);
    padding: 0.5rem 0.625rem 0.25rem;
  }

  .admin-section {
    margin-top: 0.625rem;
  }

  .collapsed-divider {
    display: block;
    height: 1px;
    background-color: var(--color-border);
    margin: 0.35rem 0;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    min-height: 36px;
    padding: 0.35rem 0.625rem;
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-size: 0.875rem;
    font-weight: 450;
    transition: background-color 140ms var(--ease), color 140ms var(--ease);
    white-space: nowrap;
    border-left: 2px solid transparent;
  }

  .nav-item:hover {
    background-color: var(--color-surface-hover);
    color: var(--color-text);
  }

  .nav-item.active {
    background-color: var(--color-bg-secondary);
    color: var(--color-text);
    font-weight: 500;
    border-left-color: var(--color-primary);
  }

  .nav-icon {
    width: 17px;
    height: 17px;
    flex-shrink: 0;
  }

  .nav-item.active .nav-icon {
    color: var(--color-primary);
  }

  .nav-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sidebar-footer {
    padding: 0.625rem;
    border-top: 1px solid var(--color-border);
    display: flex;
    flex-direction: column;
    gap: 0.375rem;
    flex-shrink: 0;
  }

  .operator-row {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    padding: 0.35rem 0.4rem;
  }

  .operator-row.compact {
    justify-content: center;
  }

  .operator-avatar {
    width: 28px;
    height: 28px;
    border-radius: var(--radius-sm);
    background-color: var(--color-bg-secondary);
    border: 1px solid var(--color-border);
    color: var(--color-text);
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 600;
    font-family: var(--font-mono);
    flex-shrink: 0;
  }

  .operator-info {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  .operator-name {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .operator-meta {
    font-size: 0.75rem;
    color: var(--color-text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    text-transform: capitalize;
  }

  .collapse-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    min-height: 32px;
    padding: 0.3rem 0.5rem;
    background: none;
    border: none;
    border-radius: var(--radius-sm);
    color: var(--color-text-muted);
    cursor: pointer;
    font-size: 0.8125rem;
    font-weight: 450;
    transition: background-color 140ms var(--ease), color 140ms var(--ease);
  }

  .collapse-btn:hover {
    background-color: var(--color-surface-hover);
    color: var(--color-text);
  }

  .collapse-icon {
    width: 15px;
    height: 15px;
    flex-shrink: 0;
  }

  @media (max-width: 1024px) {
    .sidebar {
      transform: translateX(-100%);
    }

    .sidebar.mobile-open {
      transform: translateX(0);
    }
  }
</style>
