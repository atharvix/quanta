<script lang="ts">
  import "../app.css";
  import { page } from "$app/state";
  import { goto } from "$app/navigation";
  import { browser } from "$app/env";
  import { onMount } from "svelte";
  import { token, user, logout, me } from "$lib/auth";
  import { isAdmin } from "$lib/roles";
  import { theme } from "$lib/theme";
  import Sidebar from "$lib/components/Sidebar.svelte";
  import Header from "$lib/components/Header.svelte";
  import type { LayoutProps } from "./$types";

  let { children }: LayoutProps = $props();

  let isCollapsed = $state(false);
  let isMobileOpen = $state(false);

  const isAuthPage = $derived(
    page.url.pathname === "/login" || page.url.pathname === "/register",
  );

  onMount(async () => {
    theme.init();

    if (browser) {
      const savedCollapsed = localStorage.getItem("quanta_sidebar_collapsed");
      if (savedCollapsed === "true") {
        isCollapsed = true;
      }
    }

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
    const isPublic = pathname === "/login" || pathname === "/register";

    if (!$token && !isPublic) {
      goto("/login");
    } else if ($token && isPublic) {
      goto("/");
    } else if (
      $token &&
      pathname.startsWith("/admin") &&
      $user &&
      !isAdmin($user.role)
    ) {
      goto("/");
    }
  });

  $effect(() => {
    const _ = page.url.pathname;
    isMobileOpen = false;
  });
</script>

<svelte:head>
  <title>QUANTA — Corporate Treasury &amp; Capital Optimization</title>
</svelte:head>

<div class="quanta-shell">
  {#if $token && $user && !isAuthPage}
    <Sidebar bind:isCollapsed bind:isMobileOpen />

    <div class="shell-body" class:sidebar-collapsed={isCollapsed}>
      <Header bind:isMobileOpen />
      <main class="main-content">
        {@render children()}
      </main>
    </div>
  {:else}
    <div class="auth-fullscreen">
      {@render children()}
    </div>
  {/if}
</div>

<style>
  .quanta-shell {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background-color: var(--color-bg);
  }

  .shell-body {
    display: flex;
    flex-direction: column;
    min-height: 100vh;
    margin-left: var(--sidebar-width);
    transition: margin-left 160ms var(--ease);
  }

  .shell-body.sidebar-collapsed {
    margin-left: var(--sidebar-collapsed-width);
  }

  .main-content {
    flex: 1;
    padding: 1.75rem 2rem 3rem;
    max-width: 1400px;
    width: 100%;
    margin: 0 auto;
  }

  .auth-fullscreen {
    min-height: 100vh;
    width: 100%;
    display: flex;
    flex-direction: column;
    background-color: var(--color-bg);
  }

  @media (max-width: 1024px) {
    .shell-body,
    .shell-body.sidebar-collapsed {
      margin-left: 0;
    }

    .main-content {
      padding: 1.25rem 1rem 2rem;
    }
  }
</style>
