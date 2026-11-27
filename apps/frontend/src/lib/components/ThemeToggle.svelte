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
    aria-label="Change color theme"
    aria-expanded={isOpen}
  >
    {#if $theme === 'dark'}
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
      </svg>
      <span class="theme-text">Carbon</span>
    {:else if $theme === 'light'}
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
      </svg>
      <span class="theme-text">Paper</span>
    {:else}
      <svg class="theme-icon" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M3 5a2 2 0 012-2h10a2 2 0 012 2v8a2 2 0 01-2 2h-2.22l.123.489.804.804A1 1 0 0113 18H7a1 1 0 01-.707-1.707l.804-.804L7.22 15H5a2 2 0 01-2-2V5zm2 0v8h10V5H5z" clip-rule="evenodd" />
      </svg>
      <span class="theme-text">System</span>
    {/if}
    <svg class="chevron-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.6" aria-hidden="true">
      <path d="M4 6l4 4 4-4" />
    </svg>
  </button>

  {#if isOpen}
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
        class:active={$theme === 'light'}
        onclick={() => selectTheme('light')}
        role="menuitem"
      >
        <span class="item-swatch swatch-light"></span>
        <div class="item-text">
          <span class="item-title">Paper &amp; Ink</span>
          <span class="item-desc">Neutral daylight worksheet</span>
        </div>
      </button>

      <button
        type="button"
        class="dropdown-item"
        class:active={$theme === 'dark'}
        onclick={() => selectTheme('dark')}
        role="menuitem"
      >
        <span class="item-swatch swatch-dark"></span>
        <div class="item-text">
          <span class="item-title">Carbon Terminal</span>
          <span class="item-desc">Low-glare soot workspace</span>
        </div>
      </button>

      <button
        type="button"
        class="dropdown-item"
        class:active={$theme === 'system'}
        onclick={() => selectTheme('system')}
        role="menuitem"
      >
        <span class="item-swatch swatch-system"></span>
        <div class="item-text">
          <span class="item-title">System</span>
          <span class="item-desc">Match operating system</span>
        </div>
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
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    min-height: 32px;
    padding: 0 0.65rem;
    background-color: var(--color-surface);
    border: 1px solid var(--color-border);
    border-radius: var(--radius-sm);
    color: var(--color-text-secondary);
    font-size: 0.8125rem;
    font-weight: 500;
    cursor: pointer;
    transition: background-color 140ms var(--ease), border-color 140ms var(--ease), color 140ms var(--ease);
  }

  .theme-toggle-btn:hover {
    background-color: var(--color-surface-hover);
    border-color: var(--color-border-strong);
    color: var(--color-text);
  }

  .theme-icon {
    width: 14px;
    height: 14px;
    color: var(--color-text-secondary);
  }

  .chevron-icon {
    width: 12px;
    height: 12px;
    color: var(--color-text-muted);
  }

  .theme-backdrop {
    position: fixed;
    inset: 0;
    z-index: 140;
  }

  .theme-dropdown {
    position: absolute;
    top: calc(100% + 6px);
    right: 0;
    width: 220px;
    background-color: var(--color-surface-elevated);
    border: 1px solid var(--color-border-strong);
    border-radius: var(--radius);
    box-shadow: var(--shadow-md);
    padding: 0.375rem;
    z-index: 150;
    display: flex;
    flex-direction: column;
    gap: 0.125rem;
  }

  .dropdown-header {
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-text-muted);
    padding: 0.35rem 0.5rem;
  }

  .dropdown-item {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    width: 100%;
    padding: 0.45rem 0.55rem;
    background: none;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    color: var(--color-text);
    cursor: pointer;
    text-align: left;
    transition: background-color 140ms var(--ease), border-color 140ms var(--ease);
  }

  .dropdown-item:hover {
    background-color: var(--color-surface-hover);
  }

  .dropdown-item.active {
    background-color: var(--color-bg-secondary);
    border-color: var(--color-border);
  }

  .item-swatch {
    width: 14px;
    height: 14px;
    border-radius: 3px;
    border: 1px solid var(--color-border-strong);
    flex-shrink: 0;
  }

  .swatch-light { background-color: #ffffff; }
  .swatch-dark { background-color: #0b0d10; }
  .swatch-system { background: linear-gradient(135deg, #ffffff 50%, #0b0d10 50%); }

  .item-text {
    display: flex;
    flex-direction: column;
  }

  .item-title {
    font-size: 0.8125rem;
    font-weight: 500;
    color: var(--color-text);
  }

  .item-desc {
    font-size: 0.75rem;
    color: var(--color-text-muted);
  }

  @media (max-width: 768px) {
    .theme-text {
      display: none;
    }
  }
</style>
