import type { Role } from './auth';

export function canWrite(role?: Role | string | null): boolean {
  return role === 'admin' || role === 'analyst';
}

export function isAdmin(role?: Role | string | null): boolean {
  return role === 'admin';
}

