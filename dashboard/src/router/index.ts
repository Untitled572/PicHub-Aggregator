import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'sources', component: () => import('../views/SourcesView.vue') },
    { path: '/endpoints', name: 'endpoints', component: () => import('../views/EndpointsView.vue') },
    { path: '/health', name: 'health', component: () => import('../views/HealthCheckView.vue') },
    { path: '/stats', name: 'stats', component: () => import('../views/StatsView.vue') },
    { path: '/settings', name: 'settings', component: () => import('../views/SettingsView.vue') },
  ],
})



export default router

