import { afterEach, describe, expect, it, vi } from 'vitest'

import { listMyBookings } from '@/services/bookings'

describe('booking filters', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('sends pagination, movie, and date filters to the API', async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      new Response(JSON.stringify({ data: [], page: 2, limit: 10, total: 0, total_pages: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    )
    vi.stubGlobal('fetch', fetchMock)

    await listMyBookings({
      page: 2,
      limit: 10,
      movieID: '507f1f77bcf86cd799439011',
      from: '2026-07-23T00:00:00.000Z',
      to: '2026-07-24T00:00:00.000Z',
    })

    const url = new URL(fetchMock.mock.calls[0]![0] as string, window.location.origin)
    expect(url.pathname).toBe('/api/v1/bookings')
    expect(Object.fromEntries(url.searchParams)).toEqual({
      page: '2',
      limit: '10',
      movie_id: '507f1f77bcf86cd799439011',
      from: '2026-07-23T00:00:00.000Z',
      to: '2026-07-24T00:00:00.000Z',
    })
  })
})
