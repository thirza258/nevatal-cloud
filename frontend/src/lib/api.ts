const API_BASE = '/api/v1';

export class ApiError extends Error {
  code: string;
  details?: Record<string, unknown>;
  /** HTTP status of the failed response (undefined for errors raised outside a response). */
  status?: number;

  constructor(message: string, code = 'ERROR', details?: Record<string, unknown>, status?: number) {
    super(message);
    this.name = 'ApiError';
    this.code = code;
    this.details = details;
    this.status = status;
  }
}

export async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = localStorage.getItem('nevatal_token');

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const url = `${API_BASE}${endpoint}`;
  const response = await fetch(url, {
    ...options,
    headers,
  });

  const data = await response.json().catch(() => null);

  if (!response.ok) {
    const errorInfo = data?.error;
    const message = errorInfo?.message || `Request failed with status ${response.status}`;
    const code = errorInfo?.code || 'API_ERROR';
    throw new ApiError(message, code, errorInfo?.details, response.status);
  }

  return data?.data as T;
}
