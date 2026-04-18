import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string | null>(null)
  const user = ref<{ id: string; email: string; role: string; tenant_id: string } | null>(null)

  const isAuthenticated = computed(() => !!token.value)
  const isAdmin = computed(() => user.value?.role === 'admin')
  const isOperator = computed(() => user.value?.role === 'operator' || user.value?.role === 'admin')

  function setToken(t: string) {
    token.value = t
    axios.defaults.headers.common['Authorization'] = `Bearer ${t}`
    localStorage.setItem('access_token', t)
  }

  function setUser(u: typeof user.value) {
    user.value = u
  }

  function logout() {
    token.value = null
    user.value = null
    delete axios.defaults.headers.common['Authorization']
    localStorage.removeItem('access_token')
  }

  const savedToken = localStorage.getItem('access_token')
  if (savedToken) {
    token.value = savedToken
    axios.defaults.headers.common['Authorization'] = `Bearer ${savedToken}`
  }

  return { token, user, isAuthenticated, isAdmin, isOperator, setToken, setUser, logout }
})
