import { createRouter, createWebHistory } from 'vue-router'

import { useApi, setAuthToken } from '../composables/useApi'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { bare: true } },
    { path: '/', name: 'sources', component: () => import('../views/SourcesView.vue') },
    { path: '/endpoints', name: 'endpoints', component: () => import('../views/EndpointsView.vue') },
    { path: '/health', name: 'health', component: () => import('../views/HealthCheckView.vue') },
    { path: '/stats', name: 'stats', component: () => import('../views/StatsView.vue') },
    { path: '/saved', name: 'saved', component: () => import('../views/SavedView.vue') },
    { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
  ],
})

router.beforeEach(async (to) => {
  try {
    const state = await useApi().checkAuth()
    if (state.auth_required && !state.valid) {
      setAuthToken('')
      return to.path === '/login' ? true : '/login'
    }
    if (to.path === '/login' && state.auth_required && state.valid) return '/'
  } catch {
    // 连接恢复后，下次导航会重新检查登录态。
  }
  return true
})

export default router
