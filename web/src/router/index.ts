import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

const routes = [
  { path: '/login', name: 'Login', component: () => import('@/views/Login.vue') },
  {
    path: '/',
    component: () => import('@/components/layout/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', redirect: '/dashboard' },
      { path: 'dashboard', name: 'Dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'devices', name: 'Devices', component: () => import('@/views/Devices.vue') },
      { path: 'terminal/:deviceId', name: 'Terminal', component: () => import('@/views/Terminal.vue') },
      { path: 'files/:deviceId/:path*', name: 'Files', component: () => import('@/views/Files.vue') },
      { path: 'audit', name: 'Audit', component: () => import('@/views/Audit.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to, from, next) => {
  const authStore = useAuthStore()

  if (to.meta.requiresAuth && !authStore.isAuthenticated) {
    next({ name: 'Login', query: { redirect: to.fullPath } })
    return
  }

  if (to.name === 'Login' && authStore.isAuthenticated) {
    next({ path: '/dashboard' })
    return
  }

  next()
})

export default router
