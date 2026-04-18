import apiClient from './client'

export interface FileInfo {
  name: string
  path: string
  size: number
  is_dir: boolean
  mod_time: string
  mode: string
  owner: string
  group: string
}

export interface FileContentResponse {
  content: string
  mimetype: string
  size: number
}

export const fileApi = {
  listDir: (deviceId: string, path: string, page = 1, pageSize = 50): Promise<{ files: FileInfo[]; total: number; page: number }> => {
    return apiClient.get(`/devices/${deviceId}/files`, { params: { path, page, page_size: pageSize } })
  },

  getFileContent: (deviceId: string, filePath: string): Promise<FileContentResponse | { download_url: string; message: string }> => {
    return apiClient.get(`/devices/${deviceId}/files/${encodeURIComponent(filePath)}`)
  },

  updateFile: (deviceId: string, filePath: string, content: string, version?: number): Promise<{ success: boolean }> => {
    return apiClient.put(`/devices/${deviceId}/files/${encodeURIComponent(filePath)}`, { content, version })
  },

  deleteFile: (deviceId: string, filePath: string): Promise<void> => {
    return apiClient.delete(`/devices/${deviceId}/files/${encodeURIComponent(filePath)}`)
  },

  getFileInfo: (deviceId: string, filePath: string): Promise<FileInfo> => {
    return apiClient.get(`/devices/${deviceId}/files/${encodeURIComponent(filePath)}/info`)
  },
}
