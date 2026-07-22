import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { createPinia } from 'pinia'

import SiteHeader from '@/components/SiteHeader.vue'
import { getSession, logout } from '@/services/api'

vi.mock('@/services/api', () => ({
  ApiError: class ApiError extends Error {
    status = 500
  },
  logout: vi.fn<() => Promise<void>>(),
  getSession: vi.fn<() => Promise<unknown>>(),
  setCSRFToken: vi.fn<(token: string | undefined) => void>(),
}))

describe('SiteHeader', () => {
  beforeEach(() => {
    vi.mocked(logout).mockReset().mockResolvedValue()
    vi.mocked(getSession).mockReset().mockResolvedValue({
      user_id: 'user-id',
      role: 'USER',
      user: {
        id: 'user-id',
        email: 'chidchanun@example.com',
        email_verified: true,
        name: 'chidchanun',
        role: 'USER',
      },
    })
  })

  it('logs out and returns to the login route', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/login', component: { template: '<div>Login</div>' } },
        { path: '/', component: { template: '<div>Movies</div>' } },
      ],
    })
    await router.push('/')
    await router.isReady()

    const wrapper = mount(SiteHeader, {
      props: { section: 'ภาพยนตร์' },
      global: { plugins: [createPinia(), router] },
    })

    await flushPromises()
    expect(wrapper.get('.avatar').text()).toBe('C')
    await wrapper.get('.profile-toggle').trigger('click')
    await wrapper.get('.logout-item').trigger('click')
    await flushPromises()

    expect(logout).toHaveBeenCalledOnce()
    expect(router.currentRoute.value.path).toBe('/login')
  })
})
