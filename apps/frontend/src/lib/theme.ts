import { writable, derived } from 'svelte/store';
import { browser } from '$app/env';

export type Theme = 'dark' | 'light' | 'system';

function createThemeStore() {
  const defaultTheme: Theme = 'light';
  const initialTheme: Theme = browser
    ? ((localStorage.getItem('quanta_theme') as Theme) || defaultTheme)
    : defaultTheme;

  const { subscribe, set } = writable<Theme>(initialTheme);

  return {
    subscribe,
    setTheme: (nextTheme: Theme) => {
      if (browser) {
        localStorage.setItem('quanta_theme', nextTheme);
        applyTheme(nextTheme);
      }
      set(nextTheme);
    },
    toggleTheme: () => {
      if (!browser) return;
      const current = (localStorage.getItem('quanta_theme') as Theme) || defaultTheme;
      const next: Theme = current === 'dark' ? 'light' : 'dark';
      localStorage.setItem('quanta_theme', next);
      applyTheme(next);
      set(next);
    },
    init: () => {
      if (!browser) return;
      const saved = (localStorage.getItem('quanta_theme') as Theme) || defaultTheme;
      applyTheme(saved);
      set(saved);

      const mql = window.matchMedia('(prefers-color-scheme: dark)');
      mql.addEventListener('change', () => {
        const current = (localStorage.getItem('quanta_theme') as Theme) || defaultTheme;
        if (current === 'system') {
          applyTheme('system');
        }
      });
    }
  };
}

export function applyTheme(nextTheme: Theme) {
  if (!browser) return;
  const root = document.documentElement;
  let resolved: 'dark' | 'light' = 'light';

  if (nextTheme === 'system') {
    resolved = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
  } else {
    resolved = nextTheme;
  }

  root.setAttribute('data-theme', resolved);
  root.classList.remove('theme-dark', 'theme-light');
  root.classList.add(`theme-${resolved}`);
  root.style.colorScheme = resolved;
}

export const theme = createThemeStore();

export const isDarkMode = derived(theme, ($theme) => {
  if (!browser) return false;
  if ($theme === 'system') {
    return window.matchMedia('(prefers-color-scheme: dark)').matches;
  }
  return $theme === 'dark';
});
