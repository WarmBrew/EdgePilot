import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  { path: '/login', name: 'Login', component: () => import('@/views/Login.vue') },
  {
    path: '/',
    component: () => import('@/components/layout/MainLayout.vue'),
    meta: { requiresAuth: true },
    children: [
      { path: '', name: 'Dashboard', component: () => import('@/views/Dashboard.vue') },
      { path: 'devices', name: 'Devices', component: () => import('@/views/Devices.vue') },
      { path: 'terminal/:deviceId', name: 'Terminal', component: () => import('@/views/Terminal.vue') },
      { path: 'files/:deviceId/:path*', name: 'Files', component: () => import('@/views/Files.vue') },
      { path: 'audit', name: 'Audit', component: () => import('@/views/Audit.vue') },
    ],
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
})

export default router
