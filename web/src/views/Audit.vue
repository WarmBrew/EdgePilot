<template>
  <div class="audit-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">Audit Logs</h1>
        <p class="page-subtitle">Track actions and security events across devices</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-ghost" @click="loadData">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M4.75 2.5a.75.75 0 00-.75.75v2.634c-2.175.507-3.9 2.173-4.538 4.316l-.382 1.286a.75.75 0 001.442.428l.382-1.286c.51-1.717 1.963-3.046 3.78-3.356V7.25a.75.75 0 00.75-.75H9.75a.75.75 0 000-1.5H5.5a.75.75 0 00-.75.75z" clip-rule="evenodd" />
            <path fill-rule="evenodd" d="M15.25 17.5a.75.75 0 00.75-.75v-2.634c2.175-.507 3.9-2.173 4.538-4.316l.382-1.286a.75.75 0 00-1.442-.428l-.382 1.286c-.51 1.717-1.963 3.046-3.78 3.356v-.778a.75.75 0 00-.75-.75h-4.25a.75.75 0 000 1.5h4.25a.75.75 0 00.75.75z" clip-rule="evenodd" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <div class="card">
      <div class="search-bar">
        <div class="search-wrapper">
          <svg class="search-icon" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M9 3.5a5.5 5.5 0 100 11 5.5 5.5 0 000-11z" />
            <path d="M12.5 12.5a.5.5 0 01.5.5L15 15" stroke="currentColor" stroke-width="1.5" fill="none"/>
          </svg>
          <input
            v-model="searchQuery"
            class="form-input"
            type="text"
            placeholder="Search logs..."
          />
        </div>
        <select v-model="actionFilter" class="form-input">
          <option value="all">All Actions</option>
          <option value="terminal_open">Terminal Open</option>
          <option value="terminal_close">Terminal Close</option>
          <option value="command_blocked">Command Blocked</option>
          <option value="device_register">Device Registered</option>
          <option value="device_delete">Device Deleted</option>
        </select>
      </div>

      <div v-if="loading" class="loading-overlay">
        <div class="spinner"></div>
        <span>Loading audit logs...</span>
      </div>

      <div v-else-if="audits.length === 0" class="empty-state">
        <div class="empty-state-icon">
          <svg viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M4 2a2 2 0 00-2 2v11a3 3 0 106 0V4a2 2 0 00-2-2H4zm1 14a1 1 0 100-2 1 1 0 000 2zm5.5-4H8.25a.75.75 0 010-1.5h2.25a.75.75 0 010 1.5z" clip-rule="evenodd" />
          </svg>
        </div>
        <h3 class="empty-state-title">No audit logs</h3>
        <p class="empty-state-desc">No audit events have been recorded yet</p>
      </div>

      <table v-else class="data-table">
        <thead>
          <tr>
            <th>Timestamp</th>
            <th>Action</th>
            <th>User</th>
            <th>Device</th>
            <th>IP Address</th>
            <th>Details</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="log in paginatedLogs" :key="log.id" class="audit-row">
            <td class="audit-time">{{ formatTime(log.created_at) }}</td>
            <td>
              <span class="badge" :class="getBadgeClass(log.action)">
                {{ formatAction(log.action) }}
              </span>
            </td>
            <td class="audit-user">{{ log.user_id?.slice(0, 8) || 'system' }}</td>
            <td class="audit-device">{{ log.device_id?.slice(0, 8) || '-' }}</td>
            <td class="audit-ip">{{ log.ip_address || '-' }}</td>
            <td class="audit-detail">
              <span class="detail-text">{{ formatDetail(log.detail) }}</span>
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
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { auditApi } from '@/api/audit'

const audits = ref<any[]>([])
const loading = ref(false)
const searchQuery = ref('')
const actionFilter = ref('all')
const currentPage = ref(1)
const pageSize = 25

const filteredLogs = computed(() => {
  let list = audits.value
  if (actionFilter.value !== 'all') {
    list = list.filter((l) => l.action === actionFilter.value)
  }
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(
      (l) =>
        l.user_id?.toLowerCase().includes(q) ||
        l.device_id?.toLowerCase().includes(q) ||
        l.action?.toLowerCase().includes(q) ||
        JSON.stringify(l.detail).toLowerCase().includes(q)
    )
  }
  return list
})

const totalPages = computed(() => Math.ceil(filteredLogs.value.length / pageSize))

const paginatedLogs = computed(() => {
  const start = (currentPage.value - 1) * pageSize
  return filteredLogs.value.slice(start, start + pageSize)
})

function formatAction(action: string): string {
  if (!action) return 'unknown'
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatTime(dateStr: string): string {
  const date = new Date(dateStr)
  const diff = Date.now() - date.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return date.toLocaleString()
}

function formatDetail(detail: any): string {
  if (!detail) return ''
  if (typeof detail === 'string') {
    try {
      detail = JSON.parse(detail)
    } catch {
      return detail
    }
  }
  if (typeof detail === 'object') {
    const entries = Object.entries(detail)
    return entries.map(([k, v]) => `${k}: ${v}`).join(', ')
  }
  return String(detail)
}

function getBadgeClass(action: string): string {
  if (action?.includes('terminal') || action?.includes('open')) return 'badge-info'
  if (action?.includes('block') || action?.includes('reject')) return 'badge-danger'
  if (action?.includes('device')) return 'badge-success'
  if (action?.includes('file')) return 'badge-warning'
  return 'badge-info'
}

async function loadData() {
  loading.value = true
  try {
    const res = await auditApi.list({ page: 1, page_size: 500 })
    audits.value = res.logs || []
  } catch {
    console.error('Failed to load audit logs')
    audits.value = []
  } finally {
    loading.value = false
  }
}

onMounted(loadData)
</script>

<style scoped>
.audit-page {
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
  max-width: 300px;
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

.search-wrapper .form-input,
select.form-input {
  padding: 10px 12px;
  background: var(--bg-input);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  color: var(--text-primary);
  font-size: 14px;
}

.search-wrapper .form-input {
  padding-left: 36px;
}

select.form-input {
  min-width: 180px;
  cursor: pointer;
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
  padding: 12px 16px;
  font-size: 14px;
  color: var(--text-primary);
}

.audit-time {
  font-size: 13px;
  color: var(--text-secondary);
}

.audit-user, .audit-device {
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 13px;
}

.audit-ip {
  font-family: 'SF Mono', Monaco, monospace;
  font-size: 13px;
}

.audit-detail {
  max-width: 300px;
}

.detail-text {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.badge {
  display: inline-flex;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 500;
}

.badge-success {
  background: rgba(16, 185, 129, 0.15);
  color: var(--accent-success);
}

.badge-warning {
  background: rgba(245, 158, 11, 0.15);
  color: var(--accent-warning);
}

.badge-danger {
  background: rgba(239, 68, 68, 0.15);
  color: var(--accent-danger);
}

.badge-info {
  background: rgba(6, 182, 212, 0.15);
  color: var(--accent-info);
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
