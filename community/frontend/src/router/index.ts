import { createRouter, createWebHashHistory } from 'vue-router'

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: '/', name: 'square', component: () => import('../views/SquareView.vue') },
    { path: '/submit', name: 'submit', component: () => import('../views/SubmitView.vue') },
  ],
})

export default router
