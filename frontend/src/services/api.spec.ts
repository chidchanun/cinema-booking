import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiRequest, loginWithGoogle, setCSRFToken } from './api'

describe('API client', () => {
  beforeEach(() => {
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('stores the CSRF token returned by login', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn<typeof fetch>().mockResolvedValue(
        new Response(
          JSON.stringify({
            message: 'ok',
            csrf_token: 'login-csrf-token',
            expires_at: new Date().toISOString(),
            expires_in: 900,
            user: {
              id: '507f1f77bcf86cd799439011',
              email: 'user@example.com',
              email_verified: true,
              name: 'Test User',
              role: 'USER',
            },
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      ),
    )

    await loginWithGoogle('google-id-token')

    expect(sessionStorage.getItem('cinema.csrf_token')).toBe('login-csrf-token')
  })

  it('adds credentials and CSRF header to state-changing requests', async () => {
    setCSRFToken('csrf-token')
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await apiRequest('/bookings/confirm', {
      method: 'POST',
      body: JSON.stringify({ seat_code: 'A1' }),
    })

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = init.headers as Headers
    expect(init.credentials).toBe('include')
    expect(headers.get('X-CSRF-Token')).toBe('csrf-token')
    expect(headers.get('Content-Type')).toBe('application/json')
  })

  it('does not add a CSRF header to GET requests', async () => {
    setCSRFToken('csrf-token')
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await apiRequest('/movies')

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    expect((init.headers as Headers).has('X-CSRF-Token')).toBe(false)
  })
})
