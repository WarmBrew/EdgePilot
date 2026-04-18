import apiClient from './client'

export interface AuditLog {
  id: string
  tenant_id: string
  user_id: string
  device_id: string
  action: string
  detail: Record<string, unknown>
  ip_address: string
  created_at: string
}

export const auditApi = {
  list: (params: {
    page?: number
    page_size?: number
    user_id?: string
    device_id?: string
    action?: string
    start_date?: string
    end_date?: string
  }): Promise<{ logs: AuditLog[]; total: number; page: number }> => {
    return apiClient.get('/audit/logs', { params })
  },

  getById: (id: string): Promise<AuditLog> => {
    return apiClient.get(`/audit/logs/${id}`)
  },
}
