import axios, { type AxiosInstance, type AxiosError, type InternalAxiosRequestConfig, type AxiosResponse } from 'axios'
import { ElMessage } from 'element-plus'

const apiClient: AxiosInstance = axios.create({
  baseURL: '/api/v1',
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
  },
})

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const token = localStorage.getItem('access_token')
    if (token && config.headers) {
      config.headers.Authorization = `Bearer ${token}`
    }
    return config
  },
  (error: AxiosError) => {
    return Promise.reject(error)
  }
)

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    return response.data
  },
  (error: AxiosError) => {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data as { error?: string } | undefined
      const message = data?.error || 'Unknown error'

      switch (status) {
        case 401:
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = '/login'
          ElMessage.error('Session expired, please login again')
          break
        case 403:
          ElMessage.error('Insufficient permissions')
          break
        case 429:
          ElMessage.warning('Too many requests, please wait')
          break
        case 500:
          ElMessage.error('Internal server error')
          break
        default:
          ElMessage.error(message)
      }
    } else if (error.request) {
      ElMessage.error('Network error, please check your connection')
    } else {
      ElMessage.error(error.message)
    }

    return Promise.reject(error)
  }
)

export default apiClient
