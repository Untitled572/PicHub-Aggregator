import { createRouter, createWebHistory } from 'vue-router'

// 登录态缓存: 守卫内避免每次导航都请求 /api/settings
let loginCache = { checkedAt: 0, enabled: false, configured: false }
const CACHE_TTL = 30 * 1000

async function getLoginState() {
  const now = Date.now()
  if (now - loginCache.checkedAt < CACHE_TTL) return loginCache
  try {
    const res = await fetch('/api/settings')
    if (res.ok) {
      const s = await res.json()
      loginCache = {
        checkedAt: now,
        enabled: !!s.login_enabled,
        configured: !!(s.admin_username && s.login_enabled),
      }
    }
  } catch {
    // 后端不可达: 不缓存失败, 下次导航立即重试
    // (否则会错误放行 30 秒, 导致登录保护延迟生效)
  }
  return loginCache
}

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
  const token = localStorage.getItem('pichub_admin_token') || ''
  const state = await getLoginState()

  if (!state.enabled) return true

  if (to.path === '/login') {
    // 已登录访问登录页 → 回主页
    if (token) return '/'
    return true
  }
  // 受保护页面: 未登录 → 登录页
  if (!token) return '/login'
  return true
})



export default router
