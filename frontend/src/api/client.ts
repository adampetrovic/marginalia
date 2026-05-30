// Small typed fetch wrapper for the Marginalia API.
// Uses cookie-based session auth via credentials: 'include'.

export const API_BASE = '/api/v1';

export class ApiError extends Error {
  status: number;
  body: unknown;

  constructor(status: number, message: string, body?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.body = body;
  }
}

interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  // Skip JSON parsing of the response (e.g. for 204 endpoints).
  rawResponse?: boolean;
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, headers, rawResponse, ...rest } = options;

  const init: RequestInit = {
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
      ...headers,
    },
    ...rest,
  };

  if (body !== undefined) {
    init.body = JSON.stringify(body);
  }

  const res = await fetch(`${API_BASE}${path}`, init);

  if (!res.ok) {
    let parsed: unknown = undefined;
    let message = `Request failed with status ${res.status}`;
    try {
      const text = await res.text();
      if (text) {
        try {
          parsed = JSON.parse(text);
          const maybe = parsed as { error?: string; message?: string };
          message = maybe.error || maybe.message || message;
        } catch {
          parsed = text;
          message = text;
        }
      }
    } catch {
      // ignore body read errors
    }
    throw new ApiError(res.status, message, parsed);
  }

  if (rawResponse || res.status === 204) {
    return undefined as T;
  }

  const text = await res.text();
  if (!text) {
    return undefined as T;
  }
  return JSON.parse(text) as T;
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'GET' }),
  post: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'POST', body }),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'PUT', body }),
  delete: <T>(path: string, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'DELETE', rawResponse: true }),
};
