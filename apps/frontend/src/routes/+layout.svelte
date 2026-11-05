<script lang="ts">
  import '../app.css';
  import { page } from '$app/state';
  import { goto } from '$app/navigation';
  import { browser } from '$app/env';
  import { onMount } from 'svelte';
  import { token, user, logout, me } from '$lib/auth';
  import { isAdmin } from '$lib/roles';
  import type { LayoutProps } from './$types';

  let { children }: LayoutProps = $props();

  onMount(async () => {
    if ($token && !$user) {
      try {
        await me();
      } catch {
        logout();
      }
    }
  });

  $effect(() => {
    if (!browser) return;
    const pathname = page.url.pathname;
    const isPublic = pathname === '/login' || pathname === '/register';

    if (!$token && !isPublic) {
      goto('/login');
    } else if ($token && isPublic) {
      goto('/');
    } else if ($token && pathname.startsWith('/admin') && $user && !isAdmin($user.role)) {
      goto('/');
    }
  });
</script>

<svelte:head>
  <title>QUANTA - Treasury & Balance Sheet Management</title>
</svelte:head>

<div class="app-layout">
  {#if $token && $user}
    <header class="navbar">
      <div class="navbar-left">
        <a href="/" class="brand">QUANTA</a>
        <nav class="nav-links">
          <a href="/" class="nav-link" class:active={page.url.pathname === '/'}>Dashboard</a>
          <a href="/treasuries" class="nav-link" class:active={page.url.pathname.startsWith('/treasuries')}>Treasuries</a>
          {#if isAdmin($user.role)}
            <a href="/admin/users" class="nav-link" class:active={page.url.pathname.startsWith('/admin/users')}>Users</a>
            <a href="/admin/audit-logs" class="nav-link" class:active={page.url.pathname.startsWith('/admin/audit-logs')}>Audit Logs</a>
          {/if}
        </nav>
      </div>

      <div class="navbar-right">
        <span class="user-email">{$user.email}</span>
        <span class="badge badge-{$user.role}">{$user.role}</span>
        <button type="button" class="btn btn-secondary btn-sm" onclick={logout}>
          Logout
        </button>
      </div>
    </header>
  {/if}

  <main class="main-content" class:auth-mode={!$token || page.url.pathname === '/login' || page.url.pathname === '/register'}>
    {@render children()}
  </main>
</div>

<style>
  .app-layout {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
  }

  .navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    background-color: var(--color-surface);
    border-bottom: 1px solid var(--color-border);
    padding: 0.75rem 1.5rem;
    position: sticky;
    top: 0;
    z-index: 50;
  }

  .navbar-left {
    display: flex;
    align-items: center;
    gap: 2rem;
  }

  .brand {
    font-size: 1.125rem;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--color-text);
    text-decoration: none;
  }

  .brand:hover {
    text-decoration: none;
  }

  .nav-links {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .nav-link {
    font-size: 0.875rem;
    font-weight: 500;
    color: var(--color-text-muted);
    text-decoration: none;
    padding: 0.375rem 0.625rem;
    border-radius: var(--radius);
    transition: color 0.15s ease, background-color 0.15s ease;
  }

  .nav-link:hover {
    color: var(--color-text);
    background-color: var(--color-bg);
    text-decoration: none;
  }

  .nav-link.active {
    color: var(--color-primary);
    background-color: var(--color-badge-analyst-bg);
  }

  .navbar-right {
    display: flex;
    align-items: center;
    gap: 1rem;
  }

  .user-email {
    font-size: 0.875rem;
    color: var(--color-text-muted);
  }

  .btn-sm {
    padding: 0.35rem 0.75rem;
    font-size: 0.8125rem;
  }

  .main-content {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .main-content.auth-mode {
    align-items: center;
    justify-content: center;
    padding: 2rem 1rem;
  }
</style>
