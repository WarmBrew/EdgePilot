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
  }
}

export const authApi = {
  login: (data: LoginRequest): Promise<AuthResponse> => {
    return apiClient.post('/auth/login', data)
  },

  register: (data: RegisterRequest): Promise<AuthResponse> => {
    return apiClient.post('/auth/register', data)
  },

  refreshToken: (refreshToken: string): Promise<{ access_token: string; refresh_token: string }> => {
    return apiClient.post('/auth/refresh', { refresh_token: refreshToken })
  },

  getProfile: (): Promise<{ id: string; email: string; role: string; tenant_id: string }> => {
    return apiClient.get('/auth/profile')
  },
}
