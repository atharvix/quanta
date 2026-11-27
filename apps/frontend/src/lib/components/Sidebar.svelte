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

<!-- Mobile backdrop -->
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
  aria-label="Application Sidebar"
>
  <!-- Brand Section -->
  <div class="sidebar-brand">
    <a href="/" class="brand-link" onclick={closeMobile}>
      <QuantaMonogram size={30} />
      {#if !isCollapsed}
        <div class="brand-text">
          <span class="brand-name">QUANTA</span>
          <span class="brand-tagline">TREASURY & CAPITAL</span>
        </div>
      {/if}
    </a>
  </div>

  <!-- Main Navigation List -->
  <nav class="sidebar-nav" aria-label="Main Navigation">
    <div class="nav-section-label">
      {#if !isCollapsed}
        <span>CORE PLATFORM</span>
      {:else}
        <span class="collapsed-divider"></span>
      {/if}
    </div>

    <!-- 1. Overview / Dashboard -->
    <a
      href="/"
      class="nav-item"
      class:active={page.url.pathname === "/"}
      onclick={closeMobile}
      title="Overview Dashboard"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <rect x="3" y="3" width="7" height="9" rx="1" />
        <rect x="14" y="3" width="7" height="5" rx="1" />
        <rect x="14" y="12" width="7" height="9" rx="1" />
        <rect x="3" y="16" width="7" height="5" rx="1" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Overview</span>
      {/if}
    </a>

    <!-- 2. Treasury Portfolios -->
    <a
      href="/treasuries"
      class="nav-item"
      class:active={page.url.pathname === "/treasuries"}
      onclick={closeMobile}
      title="Treasury Portfolios"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <path d="M4 7V4h16v3" />
        <path d="M9 20h6" />
        <path d="M12 4v16" />
        <rect x="4" y="7" width="16" height="13" rx="2" />
        <circle cx="12" cy="13.5" r="2" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Treasury</span>
      {/if}
    </a>

    <!-- 3. Balance Sheet -->
    <a
      href="/balance-sheet"
      class="nav-item"
      class:active={page.url.pathname === "/balance-sheet"}
      onclick={closeMobile}
      title="Balance Sheet & Assets/Liabilities"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <line x1="12" y1="3" x2="12" y2="21" />
        <path d="M5 8h14" />
        <path d="M3 13l3-6 3 6H3z" />
        <path d="M15 17l3-6 3 6h-6z" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Balance Sheet</span>
      {/if}
    </a>

    <!-- 4. Capital Allocation -->
    <a
      href="/capital-allocation"
      class="nav-item"
      class:active={page.url.pathname === "/capital-allocation"}
      onclick={closeMobile}
      title="Capital Allocation Architecture"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <path d="M21.21 15.89A10 10 0 1 1 8 2.83" />
        <path d="M22 12A10 10 0 0 0 12 2v10z" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Capital Allocation</span>
      {/if}
    </a>

    <!-- 5. Risk Analysis -->
    <a
      href="/risk-analysis"
      class="nav-item"
      class:active={page.url.pathname === "/risk-analysis"}
      onclick={closeMobile}
      title="Bayesian Risk Inference & Stress Testing"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
        <line x1="12" y1="8" x2="12" y2="12" />
        <line x1="12" y1="16" x2="12.01" y2="16" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Risk Analysis</span>
      {/if}
    </a>

    <!-- 6. Optimization Solvers -->
    <a
      href="/optimization"
      class="nav-item"
      class:active={page.url.pathname === "/optimization"}
      onclick={closeMobile}
      title="Optimization Engine & Solvers"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="3" />
        <path
          d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
        />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Optimization</span>
      {/if}
    </a>

    <!-- 7. Financial Reports -->
    <a
      href="/reports"
      class="nav-item"
      class:active={page.url.pathname === "/reports"}
      onclick={closeMobile}
      title="Audit & Financial Reports"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <polyline points="14 2 14 8 20 8" />
        <line x1="16" y1="13" x2="8" y2="13" />
        <line x1="16" y1="17" x2="8" y2="17" />
        <polyline points="10 9 9 9 8 9" />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Reports</span>
      {/if}
    </a>

    <!-- Administration & Governance -->
    <div class="nav-section-label admin-section">
      {#if !isCollapsed}
        <span>GOVERNANCE</span>
      {:else}
        <span class="collapsed-divider"></span>
      {/if}
    </div>

    <!-- Audit Log -->
    {#if $user && isAdmin($user.role)}
      <a
        href="/admin/audit-logs"
        class="nav-item"
        class:active={page.url.pathname.startsWith("/admin/audit-logs")}
        onclick={closeMobile}
        title="Audit Logs"
      >
        <svg
          class="nav-icon"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          aria-hidden="true"
        >
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
          <path d="m9 12 2 2 4-4" />
        </svg>
        {#if !isCollapsed}
          <span class="nav-label">Audit Log</span>
        {/if}
      </a>

      <!-- Users -->
      <a
        href="/admin/users"
        class="nav-item"
        class:active={page.url.pathname.startsWith("/admin/users")}
        onclick={closeMobile}
        title="User Governance"
      >
        <svg
          class="nav-icon"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          stroke-width="1.8"
          aria-hidden="true"
        >
          <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2" />
          <circle cx="9" cy="7" r="4" />
          <path d="M22 21v-2a4 4 0 0 0-3-3.87" />
          <path d="M16 3.13a4 4 0 0 1 0 7.75" />
        </svg>
        {#if !isCollapsed}
          <span class="nav-label">User Registry</span>
        {/if}
      </a>
    {/if}

    <!-- Settings -->
    <a
      href="/settings"
      class="nav-item"
      class:active={page.url.pathname === "/settings"}
      onclick={closeMobile}
      title="Platform Settings"
    >
      <svg
        class="nav-icon"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        aria-hidden="true"
      >
        <circle cx="12" cy="12" r="3" />
        <path
          d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"
        />
      </svg>
      {#if !isCollapsed}
        <span class="nav-label">Settings</span>
      {/if}
    </a>
  </nav>

  <!-- Sidebar Footer / Operator Profile -->
  <div class="sidebar-footer">
    {#if $user}
      <div class="operator-card" class:compact={isCollapsed}>
        {#if !isCollapsed}
          <div class="operator-info">
            <span class="operator-name">{$user.full_name || "Operator"}</span>
            <span class="operator-email">{$user.email}</span>
            <div class="operator-badge-wrap">
              <span class="badge badge-{$user.role}">{$user.role}</span>
            </div>
          </div>
        {:else}
          <div class="operator-avatar" title="{$user.email} ({$user.role})">
            {$user.email.slice(0, 2).toUpperCase()}
          </div>
        {/if}
      </div>
    {/if}

    <!-- Collapse / Expand Toggle Button -->
    <button
      type="button"
      class="collapse-btn"
      onclick={toggleCollapse}
      aria-label={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      title={isCollapsed ? "Expand sidebar" : "Collapse sidebar"}
    >
      <svg class="collapse-icon" viewBox="0 0 20 20" fill="currentColor">
        {#if isCollapsed}
          <path
            fill-rule="evenodd"
            d="M7.293 14.707a1 1 0 010-1.414L10.586 10 7.293 6.707a1 1 0 011.414-1.414l4 4a1 1 0 010 1.414l-4 4a1 1 0 01-1.414 0z"
            clip-rule="evenodd"
          />
        {:else}
          <path
            fill-rule="evenodd"
            d="M12.707 5.293a1 1 0 010 1.414L9.414 10l3.293 3.293a1 1 0 01-1.414 1.414l-4-4a1 1 0 010-1.414l4-4a1 1 0 011.414 0z"
            clip-rule="evenodd"
          />
        {/if}
      </svg>
      {#if !isCollapsed}
        <span class="collapse-label">Collapse Sidebar</span>
      {/if}
    </button>
  </div>
</aside>

<style>
  .mobile-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(15, 23, 42, 0.6);
    z-index: 199;
  }

  .sidebar {
    width: var(--sidebar-width);
    height: 100vh;
    background-color: #ffffff;
    border-right: 1px solid #e2e8f0;
    display: flex;
    flex-direction: column;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 200;
    transition:
      width 0.2s ease,
      transform 0.2s ease;
    box-sizing: border-box;
  }

  :global([data-theme="dark"]) .sidebar {
    background-color: #0f1724;
    border-right: 1px solid #1e2b3e;
  }

  .sidebar.collapsed {
    width: var(--sidebar-collapsed-width);
  }

  /* Brand Section */
  .sidebar-brand {
    height: var(--header-height);
    display: flex;
    align-items: center;
    padding: 0 1.25rem;
    border-bottom: 1px solid #e2e8f0;
    flex-shrink: 0;
  }

  :global([data-theme="dark"]) .sidebar-brand {
    border-bottom: 1px solid #1e2b3e;
  }

  .brand-link {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    text-decoration: none;
    color: #0f172a;
    overflow: hidden;
  }

  :global([data-theme="dark"]) .brand-link {
    color: #f8fafc;
  }

  .brand-text {
    display: flex;
    flex-direction: column;
  }

  .brand-name {
    font-size: 1.05rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    line-height: 1.1;
  }

  .brand-tagline {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #b8860b;
    font-family: var(--font-mono, monospace);
    margin-top: 0.15rem;
  }

  :global([data-theme="dark"]) .brand-tagline {
    color: #d4af37;
  }

  /* Navigation */
  .sidebar-nav {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    padding: 1rem 0.75rem;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .nav-section-label {
    font-size: 0.5625rem;
    font-weight: 700;
    letter-spacing: 0.08em;
    color: #94a3b8;
    padding: 0.75rem 0.65rem 0.35rem;
    font-family: var(--font-mono, monospace);
  }

  :global([data-theme="dark"]) .nav-section-label {
    color: #64748b;
  }

  .admin-section {
    margin-top: 0.5rem;
  }

  .collapsed-divider {
    display: block;
    height: 1px;
    background-color: #e2e8f0;
    margin: 0.5rem 0;
  }

  :global([data-theme="dark"]) .collapsed-divider {
    background-color: #1e2b3e;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.55rem 0.75rem;
    border-radius: 6px;
    color: #475569;
    text-decoration: none;
    font-size: 0.8125rem;
    font-weight: 500;
    transition:
      background-color 0.15s ease,
      color 0.15s ease;
    white-space: nowrap;
    border-left: 2px solid transparent;
  }

  :global([data-theme="dark"]) .nav-item {
    color: #94a3b8;
  }

  .nav-item:hover {
    background-color: #f1f5f9;
    color: #0f172a;
  }

  :global([data-theme="dark"]) .nav-item:hover {
    background-color: #141d2c;
    color: #f8fafc;
  }

  .nav-item.active {
    background-color: #f1f5f9;
    color: #0f172a;
    font-weight: 600;
    border-left-color: #b8860b;
  }

  :global([data-theme="dark"]) .nav-item.active {
    background-color: #162032;
    color: #f8fafc;
    border-left-color: #d4af37;
  }

  .nav-icon {
    width: 18px;
    height: 18px;
    flex-shrink: 0;
  }

  .nav-item.active .nav-icon {
    color: #b8860b;
  }

  :global([data-theme="dark"]) .nav-item.active .nav-icon {
    color: #d4af37;
  }

  .nav-label {
    overflow: hidden;
    text-overflow: ellipsis;
  }

  /* Footer & User Card */
  .sidebar-footer {
    padding: 0.75rem;
    border-top: 1px solid #e2e8f0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    flex-shrink: 0;
  }

  :global([data-theme="dark"]) .sidebar-footer {
    border-top: 1px solid #1e2b3e;
  }

  .operator-card {
    padding: 0.65rem;
    background-color: #f8fafc;
    border: 1px solid #e2e8f0;
    border-radius: 6px;
  }

  :global([data-theme="dark"]) .operator-card {
    background-color: #141d2c;
    border: 1px solid #1e2b3e;
  }

  .operator-card.compact {
    padding: 0.35rem;
    display: flex;
    justify-content: center;
  }

  .operator-info {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }

  .operator-name {
    font-size: 0.8125rem;
    font-weight: 600;
    color: #0f172a;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  :global([data-theme="dark"]) .operator-name {
    color: #f8fafc;
  }

  .operator-email {
    font-size: 0.6875rem;
    color: #64748b;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .operator-badge-wrap {
    margin-top: 0.25rem;
  }

  .operator-avatar {
    width: 32px;
    height: 32px;
    border-radius: 4px;
    background-color: #1e293b;
    color: #f8fafc;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 0.75rem;
    font-weight: 700;
    font-family: var(--font-mono, monospace);
  }

  .collapse-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    width: 100%;
    padding: 0.45rem 0.65rem;
    background: none;
    border: 1px solid transparent;
    border-radius: 5px;
    color: #64748b;
    cursor: pointer;
    font-size: 0.75rem;
    font-weight: 500;
    transition:
      background-color 0.15s ease,
      color 0.15s ease;
  }

  .collapse-btn:hover {
    background-color: #f1f5f9;
    color: #0f172a;
  }

  :global([data-theme="dark"]) .collapse-btn:hover {
    background-color: #141d2c;
    color: #f8fafc;
  }

  .collapse-icon {
    width: 16px;
    height: 16px;
    flex-shrink: 0;
  }

  /* Mobile Media Queries */
  @media (max-width: 1024px) {
    .sidebar {
      transform: translateX(-100%);
    }

    .sidebar.mobile-open {
      transform: translateX(0);
    }
  }
</style>
