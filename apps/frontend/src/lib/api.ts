import { browser } from '$app/env';
import type {
  OptimizationRequestPayload,
  OptimizationCompareRequestPayload,
  OptimizationResult,
  OptimizationCompareResponse
} from './types';

export const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

export class ApiError extends Error {
  status: number;
  details: string[];

  constructor(status: number, message: string, details: string[] = []) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.details = details;
  }
}

export interface ApiRequestOptions extends Omit<RequestInit, 'body'> {
  body?: any;
}

export async function api<T = any>(path: string, options: ApiRequestOptions = {}): Promise<T> {
  const url = path.startsWith('http') ? path : `${API_BASE_URL}${path}`;

  const headers = new Headers(options.headers || {});

  let body = options.body;
  if (body !== undefined && body !== null && typeof body === 'object' && !(body instanceof FormData)) {
    if (!headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }
    body = JSON.stringify(body);
  }

  if (browser) {
    const token = localStorage.getItem('quanta_token');
    if (token && !headers.has('Authorization')) {
      headers.set('Authorization', `Bearer ${token}`);
    }
  }

  const response = await fetch(url, {
    ...options,
    headers,
    body
  });

  if (response.status === 401) {
    if (browser) {
      localStorage.removeItem('quanta_token');
      localStorage.removeItem('quanta_user');
      if (window.location.pathname !== '/login' && window.location.pathname !== '/register') {
        window.location.href = '/login';
      }
    }
  }

  let data: any = null;
  const text = await response.text();
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = text;
    }
  }

  if (!response.ok) {
    let message = 'Request failed';
    let details: string[] = [];

    if (data && typeof data === 'object') {
      if (typeof data.error === 'string') {
        message = data.error;
      }
      if (Array.isArray(data.details)) {
        details = data.details.map((item: any) => String(item));
      }
    } else if (typeof data === 'string' && data.trim().length > 0) {
      message = data;
    } else {
      message = response.statusText || `HTTP ${response.status}`;
    }

    throw new ApiError(response.status, message, details);
  }

  return data as T;
}

export async function runClassicalOptimization(
  payload: OptimizationRequestPayload
): Promise<OptimizationResult> {
  return api<OptimizationResult>('/api/v1/optimization/classical', {
    method: 'POST',
    body: payload
  });
}

export async function runQuantumOptimization(
  payload: OptimizationRequestPayload
): Promise<OptimizationResult> {
  return api<OptimizationResult>('/api/v1/optimization/quantum', {
    method: 'POST',
    body: payload
  });
}

export async function compareOptimizations(
  payload: OptimizationCompareRequestPayload
): Promise<OptimizationCompareResponse> {
  return api<OptimizationCompareResponse>('/api/v1/optimization/compare', {
    method: 'POST',
    body: payload
  });
}
