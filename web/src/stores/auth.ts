import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { authApi, type AuthResponse } from '@/api/auth'
import apiClient from '@/api/client'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const refreshToken = ref<string | null>(null)
  const user = ref<{ id: string; email: string; role: string; tenant_id: string; must_change_password: boolean } | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isOperator = computed(() => user.value?.role === 'operator' || user.value?.role === 'admin')
  const isViewer = computed(() => user.value?.role === 'viewer' || isAdmin.value || isOperator.value)
  const mustChangePassword = computed(() => user.value?.must_change_password ?? false)

  function setTokens(auth: { access_token: string; refresh_token: string }) {
    token.value = auth.access_token
    refreshToken.value = auth.refresh_token
    localStorage.setItem('access_token', auth.access_token)
    localStorage.setItem('refresh_token', auth.refresh_token)
  }

  function clearTokens() {
    token.value = null
    refreshToken.value = null
    localStorage.removeItem('access_token')
    localStorage.removeItem('refresh_token')
  }

  async function login(email: string, password: string) {
    loading.value = true
    try {
      const response: AuthResponse = await authApi.login({ email, password })
      setTokens(response)
      user.value = response.user
      return response.user
    } finally {
      loading.value = false
    }
  }

  async function forceChangePassword(newPassword: string) {
    loading.value = true
    try {
      const response = await authApi.forceChangePassword(newPassword)
      if (user.value) {
        user.value.must_change_password = false
      }
      return response
    } finally {
      loading.value = false
    }
  }

  async function refreshSession() {
    const savedRefresh = localStorage.getItem('refresh_token')
    if (!savedRefresh) {
      logout()
      return
    }

    try {
      const response = await authApi.refreshToken(savedRefresh)
      setTokens(response)
    } catch {
      logout()
    }
  }

  function logout() {
    clearTokens()
    user.value = null
  }

  function hydrate() {
    const savedToken = localStorage.getItem('access_token')
    const savedRefreshToken = localStorage.getItem('refresh_token')
    if (savedToken) {
      token.value = savedToken
      refreshToken.value = savedRefreshToken
    }
  }

  return {
    token, refreshToken, user, loading,
    isAuthenticated, isAdmin, isOperator, isViewer, mustChangePassword,
    login, logout, refreshSession, hydrate, forceChangePassword
  }
})
