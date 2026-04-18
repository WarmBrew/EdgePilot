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
      {
        path: 'devices',
        name: 'Devices',
        component: () => import('@/views/Devices.vue'),
        meta: { role: 'operator' },
      },
      {
        path: 'terminal/:deviceId',
        name: 'Terminal',
        component: () => import('@/views/Terminal.vue'),
        meta: { role: 'operator' },
      },
      {
        path: 'files/:deviceId/:path*',
        name: 'Files',
        component: () => import('@/views/Files.vue'),
        meta: { role: 'operator' },
      },
      {
        path: 'audit',
        name: 'Audit',
        component: () => import('@/views/Audit.vue'),
        meta: { role: 'admin' },
      },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

const roleHierarchy: Record<string, number> = {
  viewer: 0,
  operator: 1,
  admin: 2,
}

function hasRequiredRole(userRole: string, requiredRole: string): boolean {
  return (roleHierarchy[userRole] ?? 0) >= (roleHierarchy[requiredRole] ?? 0)
}

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

  if (to.meta.role && authStore.user) {
    if (!hasRequiredRole(authStore.user.role, to.meta.role as string)) {
      next({ path: '/dashboard' })
      return
    }
  }

  next()
})

export default router
