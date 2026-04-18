<template>
  <div class="app-layout">
    <aside class="sidebar" :class="{ collapsed: isCollapsed }">
      <div class="sidebar-header">
        <svg class="sidebar-logo" viewBox="0 0 32 32" fill="none">
          <rect x="1" y="1" width="30" height="30" rx="6" fill="#3b82f6"/>
          <path d="M9 16L14 21L23 11" stroke="white" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/>
        </svg>
        <span class="sidebar-title" v-show="!isCollapsed">EdgePilot</span>
      </div>

      <nav class="sidebar-nav">
        <router-link
          v-for="item in navItems"
          :key="item.path"
          v-show="!item.hidden && (!item.role || hasRole(item.role))"
          :to="item.path"
          class="nav-item"
          :class="{ active: isActive(item.path) }"
        >
          <component :is="item.icon" class="nav-icon" />
          <span class="nav-label">{{ item.label }}</span>
        </router-link>
      </nav>

      <div class="sidebar-footer">
        <button class="nav-item collapse-btn" @click="isCollapsed = !isCollapsed">
          <svg v-if="isCollapsed" viewBox="0 0 20 20" fill="currentColor" class="nav-icon">
            <path fill-rule="evenodd" d="M7.21 14.77a.75.75 0 01.02-1.06L11.168 10 7.23 6.29a.75.75 0 111.04-1.08l4.5 4.25a.75.75 0 010 1.08l-4.5 4.25a.75.75 0 01-1.06-.02z" clip-rule="evenodd" />
          </svg>
          <svg v-else viewBox="0 0 20 20" fill="currentColor" class="nav-icon">
            <path fill-rule="evenodd" d="M12.79 5.23a.75.75 0 01-.02 1.06L8.832 10l3.938 3.71a.75.75 0 11-1.04 1.08l-4.5-4.25a.75.75 0 010-1.08l4.5-4.25a.75.75 0 011.06.02z" clip-rule="evenodd" />
          </svg>
          <span class="nav-label" v-show="!isCollapsed">Collapse</span>
        </button>
      </div>
    </aside>

    <div class="main-area">
      <header class="topbar">
        <div class="topbar-left">
          <nav class="breadcrumb">
            <span class="breadcrumb-item">
              <svg viewBox="0 0 20 20" fill="currentColor">
                <path d="M10.707 2.293a1 1 0 00-1.414 0l-7 7a1 1 0 001.414 1.414L4 10.414V17a1 1 0 001 1h2a1 1 0 001-1v-2a1 1 0 011-1h2a1 1 0 011 1v2a1 1 0 001 1h2a1 1 0 001-1v-6.586l.293.293a1 1 0 001.414-1.414l-7-7z"/>
              </svg>
              Home
            </span>
            <template v-if="currentRoute.matched.length">
              <span class="breadcrumb-sep">/</span>
              <span class="breadcrumb-item active">{{ currentRoute.meta.title || currentRoute.name }}</span>
            </template>
          </nav>
        </div>
        <div class="topbar-right">
          <div class="user-menu">
            <span class="user-role">{{ authStore.user?.role }}</span>
            <div class="user-avatar">{{ userInitial }}</div>
            <button class="logout-btn" @click="handleLogout">
              <svg viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M3 4.25A2.25 2.25 0 015.25 2h5.5A2.25 2.25 0 0113 4.25v2a.75.75 0 01-1.5 0v-2a.75.75 0 00-.75-.75h-5.5a.75.75 0 00-.75.75v11.5c0 .414.336.75.75.75h5.5a.75.75 0 00.75-.75v-2a.75.75 0 011.5 0v2A2.25 2.25 0 0110.75 18h-5.5A2.25 2.25 0 013 15.75V4.25z" clip-rule="evenodd" />
                <path fill-rule="evenodd" d="M18.784 10.344a.75.75 0 000-1.06l-3.25-3.25a.75.75 0 10-1.06 1.06l1.72 1.72H8a.75.75 0 000 1.5h8.194l-1.72 1.72a.75.75 0 101.06 1.06l3.25-3.25z" clip-rule="evenodd" />
              </svg>
              Logout
            </button>
          </div>
        </div>
      </header>

      <main class="content">
        <router-view />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const authStore = useAuthStore()
const isCollapsed = ref(false)

const currentRoute = computed(() => route)

const userInitial = computed(() => {
  const email = authStore.user?.email ?? ''
  return email.charAt(0).toUpperCase()
})

const navItems = [
  { path: '/dashboard', label: 'Dashboard', icon: 'IconDashboard', role: '' },
  { path: '/devices', label: 'Devices', icon: 'IconDevices', role: '' },
  { path: '/audit', label: 'Audit Logs', icon: 'IconAudit', role: 'operator' },
]

const hasRole = (requiredRole: string) => {
  const roleOrder: Record<string, number> = { viewer: 0, operator: 1, admin: 2 }
  return (roleOrder[authStore.user?.role ?? ''] ?? 0) >= (roleOrder[requiredRole] ?? 0)
}

const isActive = (path: string) => route.path === path || route.path.startsWith(path + '/')

function handleLogout() {
  authStore.logout()
  window.location.href = '/login'
}
</script>

<style scoped>
.app-layout {
  display: flex;
  height: 100vh;
  background: var(--bg-primary);
}

.sidebar {
  width: var(--sidebar-width);
  background: var(--bg-secondary);
  border-right: 1px solid var(--border-color);
  display: flex;
  flex-direction: column;
  transition: width var(--transition-normal);
  flex-shrink: 0;
}

.sidebar.collapsed {
  width: var(--sidebar-collapsed);
}

.sidebar-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px 16px;
  border-bottom: 1px solid var(--border-color);
  height: var(--header-height);
}

.sidebar-logo {
  width: 32px;
  height: 32px;
  flex-shrink: 0;
}

.sidebar-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
  white-space: nowrap;
}

.sidebar-nav {
  flex: 1;
  padding: 12px 8px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.nav-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  color: var(--text-secondary);
  text-decoration: none;
  font-size: 14px;
  font-weight: 500;
  transition: all var(--transition-fast);
  border: none;
  background: none;
  cursor: pointer;
  width: 100%;
}

.nav-item:hover {
  background: var(--bg-hover);
  color: var(--text-primary);
}

.nav-item.active {
  background: rgba(59, 130, 246, 0.15);
  color: var(--accent-primary);
}

.nav-icon {
  width: 20px;
  height: 20px;
  flex-shrink: 0;
}

.nav-label {
  white-space: nowrap;
}

.sidebar-footer {
  padding: 12px 8px;
  border-top: 1px solid var(--border-color);
}

.collapse-btn {
  color: var(--text-muted);
}

.collapse-btn:hover {
  color: var(--text-primary);
}

.main-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.topbar {
  height: var(--header-height);
  background: var(--bg-secondary);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  flex-shrink: 0;
}

.topbar-left {
  display: flex;
  align-items: center;
}

.breadcrumb {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
}

.breadcrumb-item {
  display: flex;
  align-items: center;
  gap: 6px;
  color: var(--text-muted);
}

.breadcrumb-item svg {
  width: 16px;
  height: 16px;
}

.breadcrumb-item.active {
  color: var(--text-primary);
  font-weight: 500;
}

.breadcrumb-sep {
  color: var(--text-muted);
}

.topbar-right {
  display: flex;
  align-items: center;
}

.user-menu {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-role {
  font-size: 12px;
  padding: 4px 8px;
  background: var(--bg-tertiary);
  border-radius: var(--radius-sm);
  color: var(--text-secondary);
  text-transform: uppercase;
  font-weight: 500;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: var(--accent-primary);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 14px;
  font-weight: 600;
}

.logout-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-color);
  background: transparent;
  color: var(--text-secondary);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.logout-btn:hover {
  background: var(--bg-tertiary);
  color: var(--accent-danger);
  border-color: var(--accent-danger);
}

.logout-btn svg {
  width: 16px;
  height: 16px;
}

.content {
  flex: 1;
  overflow: auto;
  padding: 24px;
}
</style>
