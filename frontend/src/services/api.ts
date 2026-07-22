const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'
const CSRF_STORAGE_KEY = 'cinema.csrf_token'
const CSRF_COOKIE_SUFFIX = '_csrf'

export interface ApiErrorBody {
  error?: string
  message?: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code?: string

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || `Request failed with status ${status}`)
    this.name = 'ApiError'
    this.status = status
    this.code = body.error
  }
}

export interface SessionUser {
  id: string
  email: string
  email_verified: boolean
  name: string
  picture?: string
  role: 'USER' | 'ADMIN'
}

export interface SessionResponse {
  user_id: string
  role: 'USER' | 'ADMIN'
  user: SessionUser
}

export interface LoginResponse {
  message: string
  csrf_token: string
  expires_at: string
  expires_in: number
  user: SessionUser
}

function isStateChanging(method: string): boolean {
  return !['GET', 'HEAD', 'OPTIONS'].includes(method.toUpperCase())
}

function readCookie(name: string): string | undefined {
  const prefix = `${encodeURIComponent(name)}=`
  const match = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))

  return match ? decodeURIComponent(match.slice(prefix.length)) : undefined
}

function readCSRFToken(): string | undefined {
  const storedToken = sessionStorage.getItem(CSRF_STORAGE_KEY)?.trim()
  if (storedToken) return storedToken

  const accessCookieName = import.meta.env.VITE_AUTH_COOKIE_NAME ?? 'cinema_access_token'
  return readCookie(`${accessCookieName}${CSRF_COOKIE_SUFFIX}`)?.trim() || undefined
}

export function setCSRFToken(token: string | undefined): void {
  const normalizedToken = token?.trim()
  if (normalizedToken) {
    sessionStorage.setItem(CSRF_STORAGE_KEY, normalizedToken)
    return
  }

  sessionStorage.removeItem(CSRF_STORAGE_KEY)
}

export async function apiRequest<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const method = (init.method ?? 'GET').toUpperCase()
  const headers = new Headers(init.headers)

  if (init.body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }

  if (isStateChanging(method) && !headers.has('X-CSRF-Token')) {
    const csrfToken = readCSRFToken()
    if (csrfToken) headers.set('X-CSRF-Token', csrfToken)
  }

  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...init,
    method,
    headers,
    credentials: 'include',
  })

  const body = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new ApiError(response.status, body as ApiErrorBody)
  }

  return body as T
}

export async function loginWithGoogle(idToken: string): Promise<LoginResponse> {
  const result = await apiRequest<LoginResponse>('/auth/google', {
    method: 'POST',
    body: JSON.stringify({ id_token: idToken }),
  })
  setCSRFToken(result.csrf_token)
  return result
}

export async function logout(): Promise<void> {
  await apiRequest<{ message: string }>('/auth/logout', { method: 'POST' })
  setCSRFToken(undefined)
}

export function getSession(): Promise<SessionResponse> {
  return apiRequest<SessionResponse>('/auth/me')
}
