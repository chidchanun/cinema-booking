import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import MovieCatalogView from '@/views/MovieCatalogView.vue'
import { listMovies } from '@/services/movies'

vi.mock('@/services/movies', () => ({ listMovies: vi.fn<typeof listMovies>() }))

describe('MovieCatalogView', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.mocked(listMovies).mockReset().mockResolvedValue({
      data: [], page: 1, limit: 24, total: 0, total_pages: 0,
    })
  })

  afterEach(() => vi.useRealTimers())

  it('searches automatically after the user types', async () => {
    const wrapper = mount(MovieCatalogView, {
      global: {
        stubs: {
          SiteHeader: true,
          RouterLink: { template: '<a><slot /></a>' },
        },
      },
    })
    await flushPromises()

    await wrapper.get('input[type="search"]').setValue('A')
    await vi.advanceTimersByTimeAsync(299)
    expect(listMovies).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(1)
    await flushPromises()
    expect(listMovies).toHaveBeenLastCalledWith('A', expect.any(AbortSignal))
  })
})
