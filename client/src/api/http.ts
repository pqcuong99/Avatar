// HTTP client for the Game Server API (docs/07). Every asset command carries an
// Idempotency-Key; when the outcome is unknown (network error, 5xx marked
// retryable) the SAME key is retried, never a new one (docs/05 §6), so a slow
// network cannot make a purchase happen twice.

export interface ApiErrorBody {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  retryable?: boolean;
}

export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly details: Record<string, unknown>;
  readonly retryable: boolean;
  constructor(status: number, body: ApiErrorBody) {
    super(body.message);
    this.code = body.code;
    this.status = status;
    this.details = body.details ?? {};
    this.retryable = !!body.retryable;
  }
}

export const NETWORK_ERROR: ApiErrorBody = {
  code: 'NETWORK_ERROR',
  message: 'Mất kết nối tới máy chủ. Kết quả chưa rõ — hãy thử lại.',
  retryable: true,
};

export interface Envelope<T> {
  ok: boolean;
  result: T;
  replayed?: boolean;
  state?: unknown;
  server_time_utc?: string;
}

export function newRequestId(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) return crypto.randomUUID();
  return Array.from({ length: 32 }, () => Math.floor(Math.random() * 16).toString(16)).join('');
}

async function once<T>(method: string, path: string, body: unknown, key?: string): Promise<Envelope<T>> {
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (key) headers['Idempotency-Key'] = key;
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers,
      credentials: 'same-origin',
      body: body === undefined ? undefined : JSON.stringify(body),
    });
  } catch {
    throw new ApiError(0, NETWORK_ERROR);
  }
  let data: unknown = null;
  try {
    data = await res.json();
  } catch {
    // fall through
  }
  const env = data as { ok?: boolean; error?: ApiErrorBody } | null;
  if (!res.ok || !env || env.ok === false) {
    throw new ApiError(res.status, env?.error ?? { code: 'HTTP_' + res.status, message: 'Máy chủ trả lỗi ' + res.status + '.', retryable: res.status >= 500 });
  }
  return data as Envelope<T>;
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms));

export function get<T>(path: string): Promise<Envelope<T>> {
  return once<T>('GET', path, undefined);
}

export function post<T>(path: string, body: unknown = {}): Promise<Envelope<T>> {
  return once<T>('POST', path, body);
}

// A JSON body keeps the Content-Type the CSRF check expects on every write.
export function del<T>(path: string): Promise<Envelope<T>> {
  return once<T>('DELETE', path, {});
}

/**
 * Sends an asset command. `key` lets a caller retry a command whose outcome
 * it does not know yet with the same Idempotency-Key.
 */
export async function command<T>(path: string, body: Record<string, unknown> = {}, key = newRequestId()): Promise<Envelope<T>> {
  let last: ApiError | null = null;
  for (let attempt = 0; attempt < 3; attempt++) {
    try {
      return await once<T>('POST', path, body, key);
    } catch (e) {
      if (!(e instanceof ApiError) || !e.retryable) throw e;
      last = e;
      await sleep(400 * 2 ** attempt);
    }
  }
  throw last!;
}
