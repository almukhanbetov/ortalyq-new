// Single API client: credentials, JSON, unified error handling.

export const API_URL = (process.env.NEXT_PUBLIC_API_URL || 'http://localhost:8448').replace(/\/$/, '');

export class ApiError extends Error {
  constructor(
    public status: number,
    public code: string,
    message: string,
    public fields: Record<string, string> = {},
  ) {
    super(message);
  }
  get isNetwork() {
    return this.status === 0;
  }
}

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';

function qs(params?: Record<string, string | number | undefined | null>) {
  if (!params) return '';
  const p = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== null && v !== '') p.set(k, String(v));
  }
  const s = p.toString();
  return s ? `?${s}` : '';
}

async function parseError(res: Response): Promise<ApiError> {
  try {
    const body = await res.json();
    const e = body?.error ?? {};
    return new ApiError(res.status, e.code ?? 'error', e.message ?? res.statusText, e.fields ?? {});
  } catch {
    return new ApiError(res.status, 'error', res.statusText);
  }
}

export async function request<T>(
  method: Method,
  path: string,
  opts: { body?: unknown; query?: Record<string, string | number | undefined | null>; form?: FormData; signal?: AbortSignal } = {},
): Promise<T> {
  let res: Response;
  try {
    res = await fetch(`${API_URL}/api${path}${qs(opts.query)}`, {
      method,
      credentials: 'include',
      headers: opts.form ? undefined : { 'Content-Type': 'application/json' },
      body: opts.form ?? (opts.body !== undefined ? JSON.stringify(opts.body) : undefined),
      signal: opts.signal,
    });
  } catch (err) {
    if ((err as Error).name === 'AbortError') throw err;
    throw new ApiError(0, 'network', 'network');
  }
  if (!res.ok) throw await parseError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const api = {
  get: <T>(path: string, query?: Record<string, string | number | undefined | null>) => request<T>('GET', path, { query }),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, { body }),
  put: <T>(path: string, body?: unknown) => request<T>('PUT', path, { body }),
  patch: <T>(path: string, body?: unknown) => request<T>('PATCH', path, { body }),
  del: <T>(path: string) => request<T>('DELETE', path),
  upload: <T>(path: string, form: FormData) => request<T>('POST', path, { form }),
};

export function attachmentUrl(id: string, inline = false) {
  return `${API_URL}/api/attachments/${id}${inline ? '?inline=1' : ''}`;
}

export function wsUrl(classroomId: string) {
  return `${API_URL.replace(/^http/, 'ws')}/api/classrooms/${classroomId}/ws`;
}
