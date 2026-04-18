import apiClient from './client'

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  tenant_id: string
}

export interface AuthResponse {
  access_token: string
  refresh_token: string
  user: {
    id: string
    email: string
    role: string
    tenant_id: string
    must_change_password: boolean
  }
}

export const authApi = {
  login: (data: LoginRequest): Promise<AuthResponse> => {
    return apiClient.post('/auth/login', data)
  },

  refreshToken: (refreshToken: string): Promise<{ access_token: string; refresh_token: string }> => {
    return apiClient.post('/auth/refresh', { refresh_token: refreshToken })
  },

  forceChangePassword: (newPassword: string): Promise<{ message: string }> => {
    return apiClient.post('/auth/change-password', { new_password: newPassword })
  },

  getProfile: (): Promise<{ id: string; email: string; role: string; tenant_id: string }> => {
    return apiClient.get('/auth/profile')
  },
}
