import { writable } from 'svelte/store';
import { browser } from '$app/env';
import { api } from './api';

export type Role = 'admin' | 'analyst' | 'viewer';

export interface User {
  id: string;
  email: string;
  full_name?: string;
  role: Role;
  created_at?: string;
}

export interface LoginResponse {
  token: string;
  expires_at: string;
  user: User;
}

const initialToken = browser ? localStorage.getItem('quanta_token') : null;
let initialUser: User | null = null;

if (browser) {
  try {
    const raw = localStorage.getItem('quanta_user');
    if (raw) {
      initialUser = JSON.parse(raw);
    }
  } catch {
    initialUser = null;
  }
}

export const token = writable<string | null>(initialToken);
export const user = writable<User | null>(initialUser);

if (browser) {
  token.subscribe((val) => {
    if (val) {
      localStorage.setItem('quanta_token', val);
    } else {
      localStorage.removeItem('quanta_token');
    }
  });

  user.subscribe((val) => {
    if (val) {
      localStorage.setItem('quanta_user', JSON.stringify(val));
    } else {
      localStorage.removeItem('quanta_user');
    }
  });
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const res = await api<LoginResponse>('/api/v1/auth/login', {
    method: 'POST',
    body: { email, password }
  });

  token.set(res.token);
  user.set(res.user);
  return res;
}

export async function register(email: string, password: string, fullName?: string): Promise<User> {
  const res = await api<User>('/api/v1/auth/register', {
    method: 'POST',
    body: {
      email,
      password,
      full_name: fullName || ''
    }
  });
  return res;
}

export function logout(): void {
  token.set(null);
  user.set(null);
  if (browser) {
    localStorage.removeItem('quanta_token');
    localStorage.removeItem('quanta_user');
    window.location.href = '/login';
  }
}

export async function me(): Promise<User> {
  const profile = await api<User>('/api/v1/auth/me');
  user.set(profile);
  return profile;
}
