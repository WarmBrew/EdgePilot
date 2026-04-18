<template>
  <div class="dashboard">
    <div class="page-header">
      <div>
        <h1 class="page-title">Dashboard</h1>
        <p class="page-subtitle">Overview of your EdgePilot infrastructure</p>
      </div>
      <div class="header-actions">
        <button class="btn btn-ghost" @click="refreshData">
          <svg class="btn-icon" viewBox="0 0 20 20" fill="currentColor">
            <path fill-rule="evenodd" d="M4.75 2.5a.75.75 0 00-.75.75v2.634c-2.175.507-3.9 2.173-4.538 4.316l-.382 1.286a.75.75 0 001.442.428l.382-1.286c.51-1.717 1.963-3.046 3.78-3.356V7.25a.75.75 0 00.75-.75H9.75a.75.75 0 000-1.5H5.5a.75.75 0 00-.75.75z" clip-rule="evenodd" />
            <path fill-rule="evenodd" d="M15.25 17.5a.75.75 0 00.75-.75v-2.634c2.175-.507 3.9-2.173 4.538-4.316l.382-1.286a.75.75 0 00-1.442-.428l-.382 1.286c-.51 1.717-1.963 3.046-3.78 3.356v-.778a.75.75 0 00-.75-.75h-4.25a.75.75 0 000 1.5h4.25a.75.75 0 00.75.75z" clip-rule="evenodd" />
          </svg>
          Refresh
        </button>
      </div>
    </div>

    <div class="stats-grid">
      <div class="stat-card" v-for="stat in stats" :key="stat.label">
        <div class="stat-header">
          <div class="stat-icon-wrapper" :style="{ background: stat.color + '20', color: stat.color }">
            <component :is="stat.iconSvg" />
          </div>
          <span v-if="stat.trend" class="stat-trend" :class="stat.trend > 0 ? 'up' : 'down'">
            {{ stat.trend > 0 ? '+' : '' }}{{ stat.trend }}%
          </span>
        </div>
        <div class="stat-value">{{ stat.value }}</div>
        <div class="stat-label">{{ stat.label }}</div>
      </div>
    </div>

    <div class="content-grid">
      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Device Status</h3>
        </div>
        <div class="chart-container">
          <div v-if="deviceStatus.length" class="status-bars">
            <div v-for="item in deviceStatus" :key="item.name" class="status-bar-row">
              <span class="status-bar-label">{{ item.name }}</span>
              <div class="status-bar-track">
                <div class="status-bar-fill" :style="{ width: item.percent + '%', backgroundColor: item.color }"></div>
              </div>
              <span class="status-bar-value">{{ item.count }}</span>
            </div>
          </div>
          <div v-else class="empty-state">
            <p class="empty-state-desc">No device data available</p>
          </div>
        </div>
      </div>

      <div class="card">
        <div class="card-header">
          <h3 class="card-title">Recent Activity</h3>
          <router-link to="/audit" class="link-view-all">View All</router-link>
        </div>
        <div class="activity-list">
          <div v-if="recentActivity.length" class="activity-item" v-for="item in recentActivity" :key="item.id">
            <div class="activity-dot" :class="'dot-' + item.type"></div>
            <div class="activity-content">
              <span class="activity-text">{{ item.text }}</span>
              <span class="activity-time">{{ item.time }}</span>
            </div>
          </div>
          <div v-else class="empty-state">
            <p class="empty-state-desc">No recent activity</p>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { deviceApi } from '@/api/device'
import { auditApi } from '@/api/audit'

const devices = ref<any[]>([])
const auditLogs = ref<any[]>([])
const loading = ref(false)

const stats = computed(() => [
  {
    label: 'Total Devices',
    value: devices.value.length.toString(),
    iconSvg: 'IconDevices',
    color: '#3b82f6',
    trend: null,
  },
  {
    label: 'Online Devices',
    value: devices.value.filter((d) => d.status === 'online').length.toString(),
    iconSvg: 'IconOnline',
    color: '#10b981',
    trend: null,
  },
  {
    label: 'Offline Devices',
    value: devices.value.filter((d) => d.status !== 'online').length.toString(),
    iconSvg: 'IconOffline',
    color: '#ef4444',
    trend: null,
  },
  {
    label: 'Active Sessions',
    value: '0',
    iconSvg: 'IconTerminal',
    color: '#f59e0b',
    trend: null,
  },
])

const deviceStatus = computed(() => {
  const total = devices.value.length || 1
  const online = devices.value.filter((d) => d.status === 'online').length
  const offline = total - online
  return [
    { name: 'Online', count: online, percent: (online / total) * 100, color: '#10b981' },
    { name: 'Offline', count: offline, percent: (offline / total) * 100, color: '#ef4444' },
  ]
})

const recentActivity = computed(() => {
  return auditLogs.value.slice(0, 5).map((log) => ({
    id: log.id,
    text: `${log.user_id?.slice(0, 8)}... ${formatAction(log.action)}`,
    time: formatTime(log.created_at),
    type: getActivityType(log.action),
  }))
})

function formatAction(action: string) {
  return action.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatTime(dateStr: string) {
  const date = new Date(dateStr)
  const diff = Date.now() - date.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

function getActivityType(action: string) {
  if (action?.includes('terminal') || action?.includes('pty')) return 'info'
  if (action?.includes('device')) return 'success'
  if (action?.includes('file')) return 'warning'
  return 'info'
}

async function refreshData() {
  loading.value = true
  try {
    const [deviceRes, auditRes] = await Promise.allSettled([
      deviceApi.list({ page: 1, page_size: 100 }),
      auditApi.list({ page: 1, page_size: 10 }),
    ])
    if (deviceRes.status === 'fulfilled') {
      devices.value = deviceRes.value.devices || []
    }
    if (auditRes.status === 'fulfilled') {
      auditLogs.value = auditRes.value.logs || []
    }
  } finally {
    loading.value = false
  }
}

onMounted(refreshData)
</script>

<style scoped>
.dashboard {
  max-width: 1200px;
}

.header-actions {
  display: flex;
  gap: 8px;
}

.btn-icon {
  width: 16px;
  height: 16px;
}

.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 24px;
}

.stat-card {
  background: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 20px;
  transition: all var(--transition-fast);
}

.stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
  border-color: var(--border-light);
}

.stat-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.stat-icon-wrapper {
  width: 40px;
  height: 40px;
  border-radius: var(--radius-md);
  display: flex;
  align-items: center;
  justify-content: center;
}

.stat-icon-wrapper svg {
  width: 20px;
  height: 20px;
}

.stat-trend {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 4px;
}

.stat-trend.up {
  color: var(--accent-success);
  background: rgba(16, 185, 129, 0.1);
}

.stat-trend.down {
  color: var(--accent-danger);
  background: rgba(239, 68, 68, 0.1);
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}

.stat-label {
  font-size: 13px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.content-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 768px) {
  .content-grid {
    grid-template-columns: 1fr;
  }
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid var(--border-color);
}

.card-title {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.link-view-all {
  font-size: 13px;
  color: var(--accent-primary);
  text-decoration: none;
}

.link-view-all:hover {
  text-decoration: underline;
}

.status-bars {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.status-bar-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-bar-label {
  font-size: 13px;
  color: var(--text-secondary);
  min-width: 60px;
}

.status-bar-track {
  flex: 1;
  height: 8px;
  background: var(--bg-tertiary);
  border-radius: 4px;
  overflow: hidden;
}

.status-bar-fill {
  height: 100%;
  border-radius: 4px;
  transition: width var(--transition-normal);
}

.status-bar-value {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  min-width: 24px;
  text-align: right;
}

.activity-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.activity-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-color);
}

.activity-item:last-child {
  border-bottom: none;
}

.activity-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-top: 6px;
  flex-shrink: 0;
}

.dot-info {
  background: var(--accent-info);
}

.dot-success {
  background: var(--accent-success);
}

.dot-warning {
  background: var(--accent-warning);
}

.dot-danger {
  background: var(--accent-danger);
}

.activity-content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.activity-text {
  font-size: 13px;
  color: var(--text-primary);
}

.activity-time {
  font-size: 12px;
  color: var(--text-muted);
}

.empty-state {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 40px 0;
}

.empty-state-desc {
  font-size: 14px;
  color: var(--text-muted);
}
</style>
