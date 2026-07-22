import { describe, it, expect } from 'vitest'

import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import App from '../App.vue'
import router from '../router'

describe('App', () => {
  it('renders the cinema login experience', async () => {
    await router.push('/login')
    await router.isReady()
    const wrapper = mount(App, { global: { plugins: [createPinia(), router] } })
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(wrapper.get('h1').text()).toBe('Cinema Booking')
    expect(wrapper.text()).toContain('เข้าสู่ระบบ')
    expect(wrapper.find('.login-panel').exists()).toBe(true)
    expect(wrapper.find('.panel-heading').isVisible()).toBe(true)
  })
})
