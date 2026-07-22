import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import { ApiError, getSession, logout, setCSRFToken, type SessionUser } from '@/services/api'

export const useSessionStore = defineStore('session', () => {
  const user = ref<SessionUser | null>(null)
  const checked = ref(false)
  const loading = ref(false)

  const isAuthenticated = computed(() => user.value !== null)

  async function load(force = false): Promise<boolean> {
    if (checked.value && !force) return isAuthenticated.value
    if (loading.value) return false

    loading.value = true
    try {
      const session = await getSession()
      user.value = session.user
      return true
    } catch (error) {
      user.value = null
      if (error instanceof ApiError && error.status === 401) {
        setCSRFToken(undefined)
      }
      return false
    } finally {
      checked.value = true
      loading.value = false
    }
  }

  function setUser(nextUser: SessionUser): void {
    user.value = nextUser
    checked.value = true
  }

  async function signOut(): Promise<void> {
    try {
      await logout()
    } finally {
      user.value = null
      checked.value = true
      setCSRFToken(undefined)
    }
  }

  return { user, checked, loading, isAuthenticated, load, setUser, signOut }
})
