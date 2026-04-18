import apiClient from './client'

export interface Device {
  id: string
  name: string
  status: string
  platform?: string
  arch?: string
  ip_address?: string
  last_seen?: string
  group_id: string | null
  tenant_id: string
  created_at: string
  updated_at: string
}

export interface DeviceRegistrationRequest {
  name: string
  platform: string
  arch: string
}

export interface DeviceListResponse {
  devices: Device[]
  total: number
  page: number
  page_size: number
}

export const deviceApi = {
  list: (params: { page?: number; page_size?: number; status?: string; group_id?: string; search?: string }): Promise<DeviceListResponse> => {
    return apiClient.get('/devices', { params })
  },

  getById: (id: string): Promise<Device> => {
    return apiClient.get(`/devices/${id}`)
  },

  create: (data: DeviceRegistrationRequest): Promise<{ device_id: string; agent_token: string }> => {
    return apiClient.post('/devices/register', data)
  },

  update: (id: string, data: { name?: string; group_id?: string }): Promise<Device> => {
    return apiClient.put(`/devices/${id}`, data)
  },

  delete: (id: string): Promise<void> => {
    return apiClient.delete(`/devices/${id}`)
  },

  batchOperation: (deviceIds: string[], action: string, params?: Record<string, unknown>): Promise<{ success: number; failed: number; errors: Array<{ device_id: string; reason: string }> }> => {
    return apiClient.post('/devices/batch', { device_ids: deviceIds, action, params })
  },
}
