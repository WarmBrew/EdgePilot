<template>
  <div class="devices-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">Devices</h1>
        <p class="page-subtitle">Manage and monitor your edge devices</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-primary" @click="showAddDialog = true">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor">
            <path d="M10.75 4.75a.75.75 0 00-1.5 0v4.5h-4.5a.75.75 0 000 1.5h4.5v4.5a.75.75 0 001.5 0v-4.5h4.5a.75.75 0 000-1.5h-4.5v-4.5z" />
          </svg>
          Add Device
        </button>
      </div>
    </div>

    <div class="card">
      <div class="search-bar">
        <div class="search-wrapper">
          <svg class="search-icon" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11zM2 9a7 7 0 1112.452 4.391l3.328 3.329a.75.75 0 11-1.06 1.06l-3.329-3.328A7 7 0 012 9z" clip-rule="evenodd" />
          </svg>
          <input
            v-model="searchQuery"
            class="form-input"
            type="text"
            placeholder="Search devices by name..."
          />
        </div>
        <div class="filter-buttons">
          <button
            v-for="filter in filters"
            :key="filter.value"
            class="filter-btn"
            :class="{ active: statusFilter === filter.value }"
            @click="statusFilter = filter.value"
          >
            {{ filter.label }}
          </button>
        </div>
      </div>

      <div v-if="loading" class="loading-overlay">
        <div class="spinner"></div>
        <span>Loading...</span>
      </div>

      <div v-else-if="filteredDevices.length === 0" class="empty-state">
        <div class="empty-state-icon">
          <svg viewBox="0 0 20 20" fill="currentColor" style="width: 40px; height: 40px">
            <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V9zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1v-2z" />
          </svg>
        </div>
        <h3 class="empty-state-title">No devices found</h3>
        <p class="empty-state-desc">Add your first device to get started</p>
      </div>

      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Status</th>
            <th>Name</th>
            <th>IP Address</th>
            <th>Platform</th>
            <th>Last Seen</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="device in paginatedDevices" :key="device.id" class="device-row">
            <td>
              <span class="badge" :class="device.status === 'online' ? 'badge-success' : 'badge-danger'">
                <span class="badge-dot"></span>
                {{ device.status === 'online' ? 'Online' : 'Offline' }}
              </span>
            </td>
            <td>
              <span class="device-name">{{ device.name }}</span>
              <span class="device-id">{{ device.id?.slice(0, 8) }}</span>
            </td>
            <td>{{ device.ip_address || 'N/A' }}</td>
            <td>{{ device.platform || 'N/A' }}</td>
            <td>{{ device.last_seen ? formatTime(device.last_seen) : 'Never' }}</td>
            <td>
              <div class="actions">
                <template v-if="device.status === 'online'">
                  <router-link :to="`/terminal/${device.id}`" class="action-btn" title="Terminal">
                    <svg viewBox="0 0 20 20" fill="currentColor">
                      <path d="M3 4a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V4zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1V9zm0 5a1 1 0 011-1h12a1 1 0 011 1v2a1 1 0 01-1 1H4a1 1 0 01-1-1v-2z" />
                    </svg>
                  </router-link>
                  <router-link :to="`/files/${device.id}/`" class="action-btn" title="Files">
                    <svg viewBox="0 0 20 20" fill="currentColor">
                      <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                    </svg>
                  </router-link>
                </template>
                <button class="action-btn action-btn-danger" @click="confirmDelete(device)" title="Delete">
                  <svg viewBox="0 0 20 20" fill="currentColor">
                    <path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 006.5 19h7a2.75 2.75 0 002.742-2.001l.841-10.52.149.022a.75.75 0 00.23-1.482A42.259 42.259 0 0015.777 4.5V3.75A2.75 2.75 0 0013 1h-1.5a2.75 2.75 0 00-2.75-2.75h-.5zM8 3.75A1.25 1.25 0 019.25 2.5h1.5A1.25 1.25 0 0112 3.75V4.5H8V3.75zM6.234 6.5l.837 10.462A1.25 1.25 0 008.316 18h3.368a1.25 1.25 0 001.244-1.148L13.766 6.5H6.234z" clip-rule="evenodd" />
                  </svg>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>

      <div v-if="totalPages > 1" class="pagination">
        <button class="btn btn-ghost" :disabled="currentPage <= 1" @click="currentPage--">
          Previous
        </button>
        <span class="page-info">Page {{ currentPage }} of {{ totalPages }}</span>
        <button class="btn btn-ghost" :disabled="currentPage >= totalPages" @click="currentPage++">
          Next
        </button>
      </div>
    </div>

    <div v-if="showDeleteConfirm" class="modal-backdrop" @click.self="showDeleteConfirm = false">
      <div class="modal">
        <div class="modal-header">
          <h3>Delete Device</h3>
        </div>
        <div class="modal-body">
          <p class="modal-warning">
            <svg viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd" />
            </svg>
            This action cannot be undone.
          </p>
          <p class="modal-text">Are you sure you want to delete <strong>{{ deleteTarget?.name }}</strong>?</p>
        </div>
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="showDeleteConfirm = false">Cancel</button>
          <button class="btn btn-danger" :disabled="deleting" @click="doDelete">
            {{ deleting ? 'Deleting...' : 'Delete' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { deviceApi } from '@/api/device'
import { ElMessage } from 'element-plus'

const devices = ref<any[]>([])
const loading = ref(false)
const searchQuery = ref('')
const statusFilter = ref('all')
const currentPage = ref(1)
const pageSize = 20

const showDeleteConfirm = ref(false)
const showAddDialog = ref(false)
const deleteTarget = ref<any>(null)
const deleting = ref(false)

const filters = [
  { label: 'All', value: 'all' },
  { label: 'Online', value: 'online' },
  { label: 'Offline', value: 'offline' },
]

const filteredDevices = computed(() => {
  let list = devices.value
  if (statusFilter.value !== 'all') {
    list = list.filter((d) => d.status === statusFilter.value)
  }
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter((d) => d.name?.toLowerCase().includes(q))
  }
  return list
})

const totalPages = computed(() => Math.ceil(filteredDevices.value.length / pageSize))

const paginatedDevices = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredDevices.value.slice(start, start + pageSize)
})

function formatTime(dateStr: string) {
  const diff = Date.now() - new Date(dateStr).getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function confirmDelete(device: any) {
  deleteTarget.value = device
  showDeleteConfirm.value = true
}

async function doDelete() {
  if (!deleteTarget.value) return
  deleting.value = true
  try {
    await deviceApi.delete(deleteTarget.value.id)
    ElMessage.success('Device deleted')
    devices.value = devices.value.filter((d) => d.id !== deleteTarget.value.id)
  } catch {
    ElMessage.error('Failed to delete device')
  } finally {
    deleting.value = false
    showDeleteConfirm.value = false
    deleteTarget.value = null
  }
}

async function loadData() {
  loading.value = true
  try {
    const res = await deviceApi.list({ page: 1, page_size: 200 })
    devices.value = res.devices || []
  } catch (e: any) {
    console.error('Failed to load devices:', e)
    devices.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.devices-page {
  max-width: 1400px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.search-wrapper {
  position: relative;
  flex: 1;
  max-width: 400px;
}

.search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
  width: 16px;
  height: 16px;
  color: var(--text-muted);
}

.search-wrapper .form-input {
  width: 100%;
  padding: 10px 12px 10px 36px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: 14px;
}

.filter-buttons {
  display: flex;
  gap: 4px;
}

.filter-btn {
  padding: 8px 14px;
  border-radius: var(--radius-md);
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.filter-btn:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.filter-btn.active {
  background: var(--accent-primary);
  color: white;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
}

.data-table thead th {
  text-align: left;
  padding: 12px 16px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-color);
}

.data-table tbody tr {
  border-bottom: 1px solid var(--border-color);
  transition: background var(--transition-fast);
}

.data-table tbody tr:hover {
  background: var(--bg-hover);
}

.data-table td {
  padding: 14px 16px;
  font-size: 14px;
  color: var(--text-primary);
}

.device-name {
  font-weight: 500;
  display: block;
}

.device-id {
  font-size: 12px;
  color: var(--text-muted);
  font-family: 'SF Mono', Monaco, monospace;
}

.actions {
  display: flex;
  gap: 6px;
}

.action-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-secondary);
  text-decoration: none;
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
  width: 16px;
  height: 16px;
}

.pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  padding: 16px 0;
}

.page-info {
  font-size: 13px;
  color: var(--text-secondary);
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal {
  background: var(--bg-secondary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  width: 100%;
  max-width: 420px;
  box-shadow: var(--shadow-xl);
}

.modal-header {
  padding: 20px 24px;
  border-bottom: 1px solid var(--border-color);
}

.modal-header h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.modal-body {
  padding: 24px;
}

.modal-warning {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
  padding: 12px;
  background: rgba(245, 158, 11, 0.1);
  border: 1px solid rgba(245, 158, 11, 0.2);
  border-radius: var(--radius-md);
  color: var(--accent-warning);
  font-size: 14px;
}

.modal-warning svg {
  width: 18px;
  height: 18px;
  flex-shrink: 0;
}

.modal-text {
  font-size: 14px;
  color: var(--text-secondary);
}

.modal-text strong {
  color: var(--text-primary);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px;
  border-top: 1px solid var(--border-color);
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
</style>
