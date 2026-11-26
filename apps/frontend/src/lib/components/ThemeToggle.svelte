<script lang="ts">
  import { theme, type Theme } from '$lib/theme';

  let isOpen = $state(false);

  function selectTheme(t: Theme) {
    theme.setTheme(t);
    isOpen = false;
  }
</script>

<div class="theme-menu-container">
  <button
    type="button"
    class="theme-toggle-btn"
    onclick={() => (isOpen = !isOpen)}
    aria-label="Change theme"
    title="Change color theme"
  >
    {#if $theme === 'dark'}
      <!-- Moon SVG -->
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor">
        <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
      </svg>
      <span class="theme-text">Command Dark</span>
    {:else if $theme === 'light'}
      <!-- Sun SVG -->
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
      </svg>
      <span class="theme-text">Executive Light</span>
    {:else}
      <!-- Monitor / System SVG -->
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor">
        <path fill-rule="evenodd" d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2h-2.22l.123.489.804.804A1 1 0 0113 18H7a1 1 0 01-.707-1.707l.804-.804L7.22 15H5a2 2 0 01-2-2V5zm2 0v8h10V5H5z" clip-rule="evenodd" />
      </svg>
      <span class="theme-text">System</span>
    {/if}
    <svg class="chevron-icon" viewBox="0 0 16 16" fill="currentColor">
      <path fill-rule="evenodd" d="M4.293 5.293a1 1 0 011.414 0L8 7.586l2.293-2.293a1 1 0 111.414 1.414l-3 3a1 1 0 01-1.414 0l-3-3a1 1 0 010-1.414z" clip-rule="evenodd" />
    </svg>
  </button>

  {#if isOpen}
    <!-- Dropdown backdrop -->
    <div
      class="theme-backdrop"
      onclick={() => (isOpen = false)}
      onkeydown={(e) => { if (e.key === 'Escape') isOpen = false; }}
      role="presentation"
    ></div>

    <div class="theme-dropdown" role="menu">
      <div class="dropdown-header">Appearance</div>
      <button
        type="button"
        class="dropdown-item"
        class:active={$theme === 'dark'}
        onclick={() => selectTheme('dark')}
        role="menuitem"
      >
        <svg class="item-icon" viewBox="0 0 20 20" fill="currentColor">
          <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
        </svg>
        <div class="item-text">
          <span class="item-title">Command Center</span>
          <span class="item-desc">Dark, high-density charcoal & gold</span>
        </div>
        {#if $theme === 'dark'}
          <span class="active-dot"></span>
        {/if}
      </button>

      <button
        type="button"
        class="dropdown-item"
        class:active={$theme === 'light'}
        onclick={() => selectTheme('light')}
        role="menuitem"
      >
        <svg class="item-icon" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
        </svg>
        <div class="item-text">
          <span class="item-title">Executive</span>
          <span class="item-desc">Light, crisp enterprise workspace</span>
        </div>
        {#if $theme === 'light'}
          <span class="active-dot"></span>
        {/if}
      </button>

      <button
        type="button"
        class="dropdown-item"
        class:active={$theme === 'system'}
        onclick={() => selectTheme('system')}
        role="menuitem"
      >
        <svg class="item-icon" viewBox="0 0 20 20" fill="currentColor">
          <path fill-rule="evenodd" d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2h-2.22l.123.489.804.804A1 1 0 0113 18H7a1 1 0 01-.707-1.707l.804-.804L7.22 15H5a2 2 0 01-2-2V5zm2 0v8h10V5H5z" clip-rule="evenodd" />
        </svg>
        <div class="item-text">
          <span class="item-title">System Match</span>
          <span class="item-desc">Sync with operating system</span>
        </div>
        {#if $theme === 'system'}
          <span class="active-dot"></span>
        {/if}
      </button>
    </div>
  {/if}
</div>

<style>
  .theme-menu-container {
    position: relative;
    display: inline-block;
  }

  .theme-toggle-btn {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding: 0.4rem 0.75rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius);
    color: var(--color-text);
    font-size: 0.8125rem;
    font-weight: 600;
    cursor: pointer;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .theme-toggle-btn:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-focus);
  }

  .theme-icon {
    width: 15px;
    height: 15px;
    color: var(--color-accent-gold);
  }

  .chevron-icon {
    width: 12px;
    height: 12px;
    color: var(--color-text-muted);
  }

  .theme-backdrop {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 100;
  }

  .theme-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    width: 250px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-lg);
    box-shadow: var(--shadow-md);
    padding: 0.5rem;
    z-index: 101;
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }

  .dropdown-header {
    font-size: 0.6875rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--color-text-muted);
    padding: 0.35rem 0.65rem;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    width: 100%;
    padding: 0.5rem 0.65rem;
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius);
    color: var(--color-text);
    cursor: pointer;
    text-align: left;
    transition: background-color 0.15s ease, border-color 0.15s ease;
  }

  .dropdown-item:hover {
    background-color: var(--color-surface-hover);
  }

  .dropdown-item.active {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border);
  }

  .item-icon {
    width: 16px;
    height: 16px;
    color: var(--color-accent-gold);
    flex-shrink: 0;
  }

  .item-text {
    flex: 1;
    display: flex;
    flex-direction: column;
  }

  .item-title {
    font-size: 0.8125rem;
    font-weight: 600;
    color: var(--color-text);
  }

  .item-desc {
    font-size: 0.6875rem;
    color: var(--color-text-muted);
  }

  .active-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background-color: var(--color-accent-gold);
  }

  @media (max-width: 768px) {
    .theme-text {
      display: none;
    }
  }
</style>
