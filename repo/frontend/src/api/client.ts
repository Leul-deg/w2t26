// API client for the LMS backend.
// Uses fetch with credentials: 'include' so the session cookie is sent on every request.
// Throws HttpError for non-2xx responses — callers handle specific status codes.
//
// 401 interception: any 401 from a domain call triggers the registered handler
// (set by AuthContext on mount) so the user is redirected to login automatically.

export interface ApiErrorBody {
  error: string;
  detail?: string;
  field?: string;
  retry_after_seconds?: number;
  challenge_key?: string;
  challenge?: string;
}

export class HttpError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: ApiErrorBody,
  ) {
    super(`HTTP ${status}: ${body.error}`);
    this.name = 'HttpError';
  }
}

// ── Global 401 handler ────────────────────────────────────────────────────────
// AuthContext registers this callback on mount. When any authenticated API call
// returns 401, the handler clears auth state and triggers a login redirect.
// The initial /auth/me call on page load is expected to return 401 when not
// logged in; the handler is safe because AuthContext only acts if already authed.

let _onUnauthorized: (() => void) | null = null;

export function setUnauthorizedHandler(fn: () => void): void {
  _onUnauthorized = fn;
}

export function clearUnauthorizedHandler(): void {
  _onUnauthorized = null;
}

// ── Core request ──────────────────────────────────────────────────────────────

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const options: RequestInit = {
    method,
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      Accept: 'application/json',
    },
  };

  if (body !== undefined) {
    options.body = JSON.stringify(body);
  }

  const res = await fetch(`/api/v1${path}`, options);

  if (res.status === 204) {
    return undefined as T;
  }

  const data = await res
    .json()
    .catch(() => ({ error: 'parse_error', detail: 'unexpected response format' }));

  if (!res.ok) {
    if (res.status === 401) {
      // Notify auth context so it can clear state and redirect to login.
      _onUnauthorized?.();
    }
    throw new HttpError(res.status, data as ApiErrorBody);
  }

  return data as T;
}

export const apiClient = {
  get: <T>(path: string): Promise<T> => request<T>('GET', path),
  post: <T>(path: string, body?: unknown): Promise<T> => request<T>('POST', path, body),
  patch: <T>(path: string, body?: unknown): Promise<T> => request<T>('PATCH', path, body),
  delete: <T>(path: string): Promise<T> => request<T>('DELETE', path),
};
