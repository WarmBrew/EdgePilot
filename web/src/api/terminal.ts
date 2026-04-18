import apiClient from './client'

export interface TerminalSession {
  id: string
  device_id: string
  user_id: string
  pty_path: string
  status: string
  started_at: string
  closed_at: string | null
}

export const terminalApi = {
  createSession: (deviceId: string): Promise<{ session_id: string; device_id: string; pty_path: string }> => {
    return apiClient.post(`/devices/${deviceId}/terminal`)
  },

  listSessions: (params: { device_id?: string; status?: string; page?: number; page_size?: number }): Promise<{ sessions: TerminalSession[]; total: number }> => {
    return apiClient.get('/terminal/sessions', { params })
  },

  closeSession: (sessionId: string): Promise<void> => {
    return apiClient.post(`/terminal/sessions/${sessionId}/close`)
  },
}
