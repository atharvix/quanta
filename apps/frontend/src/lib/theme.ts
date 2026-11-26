import { writable, derived } from 'svelte/store';
import { browser } from '$app/env';

export type Theme = 'dark' | 'light' | 'system';

function createThemeStore() {
  const defaultTheme: Theme = 'dark';
  const initialTheme: Theme = browser
    ? ((localStorage.getItem('quanta_theme') as Theme) || 'dark')
    : defaultTheme;

  const { subscribe, set } = writable<Theme>(initialTheme);

  return {
    subscribe,
    setTheme: (theme: Theme) => {
      if (browser) {
        localStorage.setItem('quanta_theme', theme);
        applyTheme(theme);
      }
      set(theme);
    },
    toggleTheme: () => {
      if (!browser) return;
      const current = (localStorage.getItem('quanta_theme') as Theme) || 'dark';
      const next: Theme = current === 'dark' ? 'light' : 'dark';
      localStorage.setItem('quanta_theme', next);
      applyTheme(next);
      set(next);
    },
    init: () => {
      if (!browser) return;
      const saved = (localStorage.getItem('quanta_theme') as Theme) || 'dark';
      applyTheme(saved);
      set(saved);

      // Listen for system theme changes if set to system
      const mql = window.matchMedia('(prefers-color-scheme: dark)');
      mql.addEventListener('change', () => {
        const current = (localStorage.getItem('quanta_theme') as Theme) || 'dark';
        if (current === 'system') {
          applyTheme('system');
        }
      });
    }
  };
}

export function applyTheme(theme: Theme) {
  if (!browser) return;
  const root = document.documentElement;
  let resolved: 'dark' | 'light' = 'dark';

  if (theme === 'system') {
    resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  } else {
    resolved = theme;
  }

  root.setAttribute('data-theme', resolved);
  root.classList.remove('theme-dark', 'theme-light');
  root.classList.add(`theme-${resolved}`);
  root.style.colorScheme = resolved;
}

export const theme = createThemeStore();

export const isDarkMode = derived(theme, ($theme) => {
  if (!browser) return true;
  if ($theme === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }
  return $theme === 'dark';
});
