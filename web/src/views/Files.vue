<template>
  <div class="files-page">
    <div class="file-header">
      <div>
        <div class="breadcrumb-files">
          <button class="breadcrumb-btn" @click="navigateTo(currentPath)">
            Root
          </button>
          <template v-for="(part, idx) in pathParts" :key="idx">
            <span class="breadcrumb-sep">/</span>
            <button class="breadcrumb-btn" @click="navigateToPath(idx)">
              {{ part }}
            </button>
          </template>
        </div>
      </div>
      <div class="file-actions">
        <button class="btn btn-ghost" @click="refreshFiles">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M4.75 2.5a.75.75 0 00-.75.75v2.634c-2.175.507-3.9 2.173-4.538 4.316l-.382 1.286a.75.75 0 001.442.428l.382-1.286c.51-1.717 1.963-3.046 3.78-3.356V7.25a.75.75 0 00.75-.75H9.75a.75.75 0 000-1.5H5.5a.75.75 0 00-.75.75z" clip-rule="evenodd" />
            <path fill-rule="evenodd" d="M15.25 17.5a.75.75 0 00.75-.75v-2.634c2.175-.507 3.9-2.173 4.538-4.316l.382-1.286a.75.75 0 00-1.442-.428l-.382 1.286c-.51 1.717-1.963 3.046-3.78 3.356v-.778a.75.75 0 00-.75-.75h-4.25a.75.75 0 000 1.5h4.25a.75.75 0 00.75.75z" clip-rule="evenodd" />
          </svg>
          Refresh
        </button>
        <label class="btn btn-primary">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10.75 2a.75.75 0 01.75.75v5.59l1.95-2.1a.75.75 0 111.1 1.02l-3.25 3.5a.75.75 0 01-1.1 0L6.95 7.26a.75.75 0 111.1-1.02l1.95 2.1V2.75a.75.75 0 01.75-.75z" />
            <path d="M5.273 4.5a1.25 1.25 0 00-1.205.918l-1.523 5.52c-.006.02-.01.041-.015.062H6a1.5 1.5 0 010 3H2.465c.005.02.01.042.015.062l1.523 5.52c.143.522.62.918 1.205.918h9.554a1.25 1.25 0 001.205-.918l1.523-5.52c.006-.02.01-.041.015-.062H13.5a1.5 1.5 0 010-3h3.535c-.005-.02-.01-.042-.015-.062L15.495 5.418a1.25 1.25 0 00-1.205-.918H5.273z" />
          </svg>
          Upload
          <input type="file" hidden @change="handleUpload" />
        </label>
        <router-link :to="{ name: 'Devices' }" class="btn btn-ghost">
          <svg viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M17 10a.75.75 0 01-.75.75H5.612l4.158 3.96a.75.75 0 11-1.04 1.08l-5.5-5.25a.75.75 0 010-1.08l5.5-5.25a.75.75 0 111.04 1.08L5.612 9.25h10.638A.75.75 0 0117 10z" clip-rule="evenodd" />
          </svg>
          Back
        </router-link>
      </div>
    </div>

    <div v-if="loading" class="loading-overlay">
      <div class="spinner"></div>
      <span>Loading files...</span>
    </div>

    <div v-else-if="files.length === 0" class="empty-state">
      <div class="empty-state-icon">
        <svg viewBox="0 0 20 20" fill="currentColor">
          <path d="M2 4a2 2 0 012-2h3.586a2 2 0 011.414.586l1.586 1.586A2 2 0 0012 2.586V4h5.586a2 2 0 012 2v9.586a2 2 0 01-2 2H4a2 2 0 01-2-2V4z" />
        </svg>
      </div>
      <h3 class="empty-state-title">This directory is empty</h3>
      <p class="empty-state-desc">Upload files or navigate to a different directory</p>
    </div>

    <div v-else class="files-grid">
      <div class="card file-browser">
        <table class="file-table">
          <thead>
            <tr>
              <th style="width: 40%"></th>
              <th style="width: 15%">Size</th>
              <th style="width: 25%">Modified</th>
              <th style="width: 20%">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="file in directories" :key="file.path" class="file-row folder" @click="navigateTo(file.path)">
              <td>
                <svg class="file-icon" viewBox="0 0 20 20" fill="currentColor">
                  <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                </svg>
                <span class="file-name">{{ file.name }}</span>
              </td>
              <td class="file-size">--</td>
              <td class="file-time">{{ formatTime(file.mod_time) }}</td>
              <td class="file-acts"></td>
            </tr>
            <tr v-for="file in regularFiles" :key="file.path" class="file-row">
              <td @click="openFile(file)">
                <svg class="file-icon" viewBox="0 0 20 20" fill="currentColor">
                  <path fill-rule="evenodd" d="M4 4a2 2 0 012-2h4.586A2 2 0 0112 2.586L15.414 6A2 2 0 0116 7.414V16a2 2 0 01-2 2H6a2 2 0 01-2-2V4z" clip-rule="evenodd" />
                </svg>
                <span class="file-name">{{ file.name }}</span>
              </td>
              <td class="file-size">{{ formatSize(file.size) }}</td>
              <td class="file-time">{{ formatTime(file.mod_time) }}</td>
              <td class="file-acts">
                <button class="action-btn" @click="downloadFile(file)" title="Download">
                  <svg viewBox="0 0 20 20" fill="currentColor">
                    <path d="M10.75 2.75a.75.75 0 00-1.5 0v8.614L6.295 8.235a.75.75 0 10-1.09 1.03l4.25 4.5a.75.75 0 001.09 0l4.25-4.5a.75.75 0 00-1.09-1.03l-2.955 3.129V2.75z" />
                    <path d="M3.5 12.75a.75.75 0 00-1.5 0v2.5A2.75 2.75 0 004.75 18h10.5A2.75 2.75 0 0018 15.25v-2.5a.75.75 0 00-1.5 0v2.5c0 .69-.56 1.25-1.25 1.25H4.75c-.69 0-1.25-.56-1.25-1.25v-2.5z" />
                  </svg>
                </button>
                <button class="action-btn action-btn-danger" @click="deleteFile(file)" title="Delete">
                  <svg viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 006.5 19h7a2.75 2.75 0 002.742-2.001l.841-10.52.149.022a.75.75 0 00.23-1.482A42.259 42.259 0 0015.777 4.5V3.75A2.75 2.75 0 0013 1h-1.5a2.75 2.75 0 00-2.75-2.75h-.5zM8 3.75A1.25 1.25 0 019.25 2.5h1.5A1.25 1.25 0 0112 3.75V4.5H8V3.75zM6.234 6.5l.837 10.462A1.25 1.25 0 008.316 18h3.368a1.25 1.25 0 001.244-1.148l.837-10.462L13.766 6.5H6.234z" clip-rule="evenodd" />
                  </svg>
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="showEditor" class="editor-overlay">
      <div class="editor-card">
        <div class="editor-header">
          <h3 class="editor-title">{{ editingFile?.name }}</h3>
          <div class="editor-actions">
            <button class="btn btn-ghost" @click="saveFile">
              <svg viewBox="0 0 20 20" fill="currentColor">
                <path d="M2.073 1.153A.75.75 0 012.75.75h8.5c.209 0 .396.103.51.258l4.75 6a.75.75 0 01.16.492v10.75A.75.75 0 0115.25 19h-10A2.75 2.75 0 012.5 16.25v-2a.75.75 0 01.75-.75c.414 0 .75-.336.75-.75V9.5a.75.75 0 01.75-.75V5.25a.75.75 0 01.368-.644l3.41-2.15V.75a.75.75 0 01.593-.575L2.073 1.153zM15.25 7.25H9.75a.75.75 0 00-.75.75v2.5c0 .138-.112.25-.25.25h-.72C7.92 10.75 7.25 10.42 6.653 9.855 6.056 9.29 5.64 8.555 5.5 7.874a2.002 2.002 0 00-1.366 1.186A2 2 0 003.25 11v6a.75.75 0 00.75.75h11a.75.75 0 00.75-.75v-6a.75.75 0 00-.75-.75h.25z" />
              </svg>
              Save
            </button>
            <button class="btn btn-ghost" @click="showEditor = false">
              <svg viewBox="0 0 20 20" fill="currentColor">
                <path d="M6.28 5.22a.75.75 0 00-1.06 1.06L8.94 10l-3.72 3.72a.75.75 0 101.06 1.06L10 11.06l3.72 3.72a.75.75 0 101.06-1.06L11.06 10l3.72-3.72a.75.75 0 00-1.06-1.06L10 8.94 6.28 5.22z" />
              </svg>
              Close
            </button>
          </div>
        </div>
        <div class="editor-body" ref="editorRef"></div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { fileApi } from '@/api/file'
import type * as monaco from 'monaco-editor'
import { ElMessage } from 'element-plus'

const route = useRoute()
const router = useRouter()

const files = ref<any[]>([])
const loading = ref(false)
const showEditor = ref(false)
const editingFile = ref<any>(null)
const editorRef = ref<HTMLElement | null>(null)
let editorInstance: monaco.editor.IStandaloneCodeEditor | null = null

const deviceId = computed(() => route.params.deviceId as string)
const currentPath = computed(() => (route.params.path as string) || '')

const pathParts = computed(() => {
  const path = currentPath.value || ''
  return path.split('/').filter(Boolean)
})

const directories = computed(() => files.value.filter((f) => f.is_dir))
const regularFiles = computed(() => files.value.filter((f) => !f.is_dir))

function formatSize(bytes: number): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(size < 10 ? 1 : 0)} ${units[i]}`
}

function formatTime(dateStr: string): string {
  if (!dateStr) return 'N/A'
  const date = new Date(dateStr)
  const diff = Date.now() - date.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return date.toLocaleDateString()
}

async function loadFiles(path: string = currentPath.value) {
  loading.value = true
  try {
    const res = await fileApi.listDir(deviceId.value, '/' + path, 1, 200)
    files.value = res.files || []
  } catch (e: any) {
    console.error('Failed to load files:', e)
    files.value = []
  } finally {
    loading.value = false
  }
}

function navigateTo(path: string) {
  router.replace({
    name: 'Files',
    params: { deviceId: deviceId.value, path: path || undefined },
  })
}

function navigateToPath(idx: number) {
  const path = pathParts.value.slice(0, idx + 1).join('/')
  navigateTo(path)
}

async function refreshFiles() {
  await loadFiles()
}

async function openFile(file: any) {
  editingFile.value = file
  showEditor.value = true
  try {
    const res = await fileApi.getFileContent(deviceId.value, file.path)
    if ('content' in res) {
      initEditor(res.content)
    }
  } catch {
    ElMessage.error('Failed to load file content')
  }
}

function initEditor(content: string) {
  setTimeout(async () => {
    if (!editorRef.value) return

    const { editor } = await import('monaco-editor')
    editorInstance = editor.create(editorRef.value, {
      value: content,
      language: 'plaintext',
      theme: 'vs-dark',
      automaticLayout: true,
      fontSize: 14,
      fontFamily: "'JetBrains Mono', 'SF Mono', Monaco, monospace",
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      padding: { top: 16 },
    })

    const ext = editingFile.value.name.split('.').pop()?.toLowerCase()
    const langMap: Record<string, string> = {
      js: 'javascript', ts: 'typescript', py: 'python', go: 'go',
      json: 'json', yaml: 'yaml', yml: 'yaml', xml: 'xml',
      html: 'html', css: 'css', md: 'markdown', sh: 'shell',
      bash: 'shell', rb: 'ruby', rs: 'rust', java: 'java',
    }
    if (langMap[ext]) {
      editorInstance?.getAction('editor.action.formatDocument')
      editorInstance?.getModel()?.setValue(content)
    }
  }, 100)
}

async function saveFile() {
  if (!editorInstance || !editingFile.value) return
  const content = editorInstance.getValue()
  try {
    const encoded = btoa(content)
    await fileApi.updateFile(deviceId.value, editingFile.value.path, encoded)
    ElMessage.success('File saved')
    showEditor.value = false
    await loadFiles()
  } catch {
    ElMessage.error('Failed to save file')
  }
}

function downloadFile(file: any) {
  window.open(`/api/v1/devices/${deviceId.value}/files/${encodeURIComponent(file.path)}/download`, '_blank')
}

async function deleteFile(file: any) {
  if (!confirm(`Delete "${file.name}"? This cannot be undone.`)) return
  try {
    await fileApi.deleteFile(deviceId.value, file.path)
    ElMessage.success('File deleted')
    await loadFiles()
  } catch {
    ElMessage.error('Failed to delete file')
  }
}

async function handleUpload(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  const formData = new FormData()
  formData.append('file', file)
  formData.append('directory', '/' + currentPath.value)

  try {
    ElMessage.info('Uploading...')
    await fetch(`/api/v1/devices/${deviceId.value}/files/upload`, {
      method: 'POST',
      body: formData,
      headers: { Authorization: `Bearer ${localStorage.getItem('access_token')}` },
    })
    ElMessage.success('Uploaded')
    await loadFiles()
  } catch {
    ElMessage.error('Failed to upload')
  }
  target.value = ''
}

watch(() => route.fullPath, () => loadFiles(), { immediate: false })
onMounted(() => loadFiles())
</script>

<style scoped>
.files-page {
  margin: -24px;
  min-height: calc(100vh - var(--header-height));
  padding: 20px 24px;
}

.file-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 20px;
  flex-wrap: wrap;
  gap: 12px;
}

.breadcrumb-files {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 4px;
  font-size: 14px;
}

.breadcrumb-btn {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 4px 6px;
  border-radius: var(--radius-sm);
  font-size: 14px;
  transition: all var(--transition-fast);
}

.breadcrumb-btn:hover {
  color: var(--accent-primary);
  background: rgba(59, 130, 246, 0.1);
}

.breadcrumb-sep {
  color: var(--text-muted);
}

.file-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.file-browser {
  padding: 0;
  overflow-x: auto;
}

.file-table {
  width: 100%;
  border-collapse: collapse;
}

.file-table thead th {
  text-align: left;
  padding: 12px 16px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-color);
}

.file-table tbody tr {
  border-bottom: 1px solid var(--border-color);
  transition: background var(--transition-fast);
}

.file-table tbody tr:hover {
  background: var(--bg-hover);
}

.file-row.folder {
  cursor: pointer;
}

.file-table td {
  padding: 10px 16px;
  font-size: 14px;
  vertical-align: middle;
}

.file-icon {
  width: 18px;
  height: 18px;
  vertical-align: middle;
  margin-right: 8px;
  color: var(--accent-primary);
}

.folder .file-icon {
  color: var(--accent-warning);
}

.file-name {
  vertical-align: middle;
}

.file-size, .file-time {
  color: var(--text-secondary);
  font-size: 13px;
}

.file-acts {
  display: flex;
  gap: 4px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  border: none;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.action-btn:hover {
  background: var(--bg-tertiary);
  color: var(--text-primary);
}

.action-btn-danger:hover {
  background: rgba(239, 68, 68, 0.15);
  color: var(--accent-danger);
}

.action-btn svg {
  width: 15px;
  height: 15px;
}

.editor-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px;
}

.editor-card {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 1000px;
  height: 80vh;
  display: flex;
  flex-direction: column;
  box-shadow: var(--shadow-xl);
}

.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  border-bottom: 1px solid var(--border-color);
}

.editor-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.editor-actions {
  display: flex;
  gap: 8px;
}

.editor-body {
  flex: 1;
}

.loading-overlay {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 60px 0;
  color: var(--text-secondary);
}

.spinner {
  width: 20px;
  height: 20px;
  border: 2px solid var(--border-color);
  border-top-color: var(--accent-primary);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
}

.empty-state-icon {
  width: 60px;
  height: 60px;
  border-radius: 50%;
  background: var(--bg-tertiary);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
  color: var(--text-muted);
}

.empty-state-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.empty-state-desc {
  font-size: 14px;
  color: var(--text-secondary);
}
</style>
