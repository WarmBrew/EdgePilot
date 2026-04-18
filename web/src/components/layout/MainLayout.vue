<template>
  <div class="main-layout">
    <el-container>
      <el-header class="header">
        <div class="header-left">
          <el-icon class="collapse-btn" @click="collapseSidebar">
            <component :is="isCollapse ? Expand : Fold" />
          </el-icon>
          <h1>Robot Remote Maintenance</h1>
        </div>
        <div class="header-right">
          <el-dropdown trigger="click" @command="handleUserCommand">
            <span class="user-info">
              <el-avatar :size="28">{{ userInitial }}</el-avatar>
              <span class="user-email">{{ authStore.user?.email }}</span>
              <el-icon><arrow-down /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="logout">
                  <el-icon><switch-button /></el-icon>
                  Logout
                </el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </el-header>
      <el-container>
        <el-aside :width="isCollapse ? '64px' : '220px'" class="aside">
          <el-menu
            :default-active="currentRoute"
            :collapse="isCollapse"
            router
            class="side-menu"
          >
            <el-menu-item index="/dashboard">
              <el-icon><monitor /></el-icon>
              <template #title>Dashboard</template>
            </el-menu-item>
            <el-menu-item index="/devices">
              <el-icon><cellphone /></el-icon>
              <template #title>Devices</template>
            </el-menu-item>
            <el-menu-item index="/audit" v-if="authStore.isOperator">
              <el-icon><document /></el-icon>
              <template #title>Audit Logs</template>
            </el-menu-item>
          </el-menu>
        </el-aside>
        <el-main class="main-content">
          <router-view />
        </el-main>
      </el-container>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  Fold,
  Expand,
  ArrowDown,
  SwitchButton,
  Monitor,
  Cellphone,
  Document,
} from '@element-plus/icons-vue'

const route = useRoute()
const authStore = useAuthStore()
const isCollapse = ref(false)

const currentRoute = computed(() => route.path)

const userInitial = computed(() => {
  const email = authStore.user?.email ?? ''
  return email.charAt(0).toUpperCase()
})

function collapseSidebar() {
  isCollapse.value = !isCollapse.value
}

function handleUserCommand(command: string) {
  if (command === 'logout') {
    authStore.logout()
    window.location.href = '/login'
  }
}
</script>

<style scoped>
.main-layout {
  height: 100vh;
}

.header {
  background-color: #001529;
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 16px;
  height: 56px;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-left h1 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  white-space: nowrap;
}

.collapse-btn {
  font-size: 20px;
  cursor: pointer;
  color: rgba(255, 255, 255, 0.85);
  transition: color 0.2s;
}

.collapse-btn:hover {
  color: #fff;
}

.header-right {
  display: flex;
  align-items: center;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: rgba(255, 255, 255, 0.85);
}

.user-email {
  font-size: 14px;
  max-width: 150px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.aside {
  background-color: #fff;
  border-right: 1px solid #e8e8e8;
  transition: width 0.2s;
}

.side-menu {
  border-right: none;
  height: 100%;
}

.main-content {
  background-color: #f0f2f5;
  padding: 24px;
}
</style>
