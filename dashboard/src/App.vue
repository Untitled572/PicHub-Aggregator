<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { RouterView, RouterLink, useRoute } from 'vue-router'
import {
  Layers,
  Activity,
  Sliders,
  Sparkles,
  ExternalLink,
  ShieldCheck,
  Server,
  Copy,
  Check,
  Link2
} from 'lucide-vue-next'
import { useDomain } from './composables/useDomain'

const route = useRoute()
const copiedUrl = ref(false)
const backendConnected = ref(true)

const { getEffectiveDomain } = useDomain()

const navItems = [
  { path: '/', label: '图源管理', subtitle: 'API 源配置与监听', icon: Layers },
  { path: '/endpoints', label: '接口管理', subtitle: '分类分发与总接口配置', icon: Link2 },
  { path: '/health', label: '健康检测', subtitle: '可用性与延迟评估', icon: Activity },
  { path: '/settings', label: '系统设置', subtitle: '中转模式与缓存策略', icon: Sliders },
]

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function getTitle() {
  const current = navItems.find(item => isActive(item.path))
  return current ? current.label : '控制台'
}

async function checkBackendHealth() {
  try {
    const res = await fetch('/api/settings', { method: 'GET' })
    backendConnected.value = res.ok
  } catch {
    backendConnected.value = false
  }
}

let healthTimer: any = null

onMounted(() => {
  checkBackendHealth()
  healthTimer = setInterval(checkBackendHealth, 8000)
})

onUnmounted(() => {
  if (healthTimer) clearInterval(healthTimer)
})

function copyUserApiUrl() {
  const apiUrl = `${getEffectiveDomain()}/random`
  navigator.clipboard.writeText(apiUrl)
  copiedUrl.value = true
  setTimeout(() => copiedUrl.value = false, 2500)
}
</script>

<template>
  <div class="min-h-screen bg-morandi-bg flex flex-col md:flex-row text-morandi-text font-sans">
    <!-- Sidebar (Fixed Position & Height on Desktop) -->
    <aside class="w-full md:w-64 bg-morandi-sidebar/80 backdrop-blur-md border-r border-morandi-border/60 flex flex-col justify-between p-4 shrink-0 md:sticky md:top-0 md:h-screen md:overflow-y-auto z-20">
      <div>
        <!-- Brand / Header -->
        <div class="flex items-center gap-3 px-3 py-3 mb-6">
          <div class="w-10 h-10 rounded-xl bg-morandi-sage text-white flex items-center justify-center shadow-md shadow-morandi-sage/20">
            <Sparkles class="w-5 h-5" />
          </div>
          <div>
            <div class="flex items-center gap-1.5">
              <span class="font-bold text-base tracking-tight text-morandi-text">PicHub</span>
              <span class="text-[10px] px-1.5 py-0.2 bg-morandi-sage/15 text-morandi-sage-dark font-medium rounded-md">v0.1</span>
            </div>
            <p class="text-xs text-morandi-muted">图源聚合中转引擎</p>
          </div>
        </div>

        <!-- Navigation Links -->
        <nav class="space-y-1.5">
          <RouterLink
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            class="group flex items-center gap-3 px-3.5 py-2.5 rounded-xl transition-all duration-200"
            :class="isActive(item.path)
              ? 'bg-white text-morandi-sage-dark shadow-sm border border-morandi-borderSoft font-semibold'
              : 'text-morandi-muted hover:bg-white/60 hover:text-morandi-text'"
          >
            <component
              :is="item.icon"
              class="w-4 h-4 transition-transform duration-200 group-hover:scale-110"
              :class="isActive(item.path) ? 'text-morandi-sage' : 'text-morandi-light group-hover:text-morandi-muted'"
            />
            <div class="flex flex-col">
              <span class="text-xs tracking-wide leading-tight">{{ item.label }}</span>
              <span class="text-[10px] opacity-70 font-normal leading-tight hidden md:inline">{{ item.subtitle }}</span>
            </div>
          </RouterLink>
        </nav>
      </div>

      <!-- Footer Widget (Node Health Status - Click to Copy) -->
      <div class="mt-6 pt-4 border-t border-morandi-border/40 px-2 space-y-3">
        <div
          @click="copyUserApiUrl"
          class="bg-white/80 hover:bg-white rounded-xl p-3 border border-morandi-borderSoft flex items-center justify-between cursor-pointer transition-all duration-200 hover:shadow-sm group relative"
          :title="backendConnected ? '点击复制对外 API 分发接口 URL' : '后端服务未连接'"
        >
          <div class="flex items-center gap-2.5">
            <!-- Dynamic Breathing Dot (Green if connected, Red if disconnected) -->
            <span class="relative flex h-2.5 w-2.5">
              <span
                class="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75"
                :class="backendConnected ? 'bg-emerald-400' : 'bg-rose-400'"
              ></span>
              <span
                class="relative inline-flex rounded-full h-2.5 w-2.5"
                :class="backendConnected ? 'bg-emerald-500' : 'bg-rose-500'"
              ></span>
            </span>
            <div class="text-xs font-semibold flex items-center gap-1.5" :class="backendConnected ? 'text-morandi-text' : 'text-rose-600'">
              <span>{{ backendConnected ? '节点就绪' : '未就绪' }}</span>
              <span v-if="copiedUrl" class="text-[10px] text-emerald-600 font-medium">已复制!</span>
            </div>
          </div>
          <component :is="copiedUrl ? Check : Copy" class="w-4 h-4 text-morandi-muted group-hover:text-morandi-sage-dark transition-colors" />
        </div>

        <div
          class="flex items-center justify-between w-full px-3 py-2 text-xs text-morandi-ocean/70 bg-morandi-ocean/10 rounded-xl font-medium cursor-not-allowed select-none opacity-80"
          title="社区规则广场暂未开放 (功能研发中)"
        >
          <span class="flex items-center gap-1.5">
            <ShieldCheck class="w-3.5 h-3.5 text-morandi-ocean/60" /> 探索社区规则广场
          </span>
          <span class="text-[10px] px-1.5 py-0.2 bg-amber-100/90 text-amber-800 font-bold rounded">待开发</span>
        </div>

      </div>
    </aside>

    <!-- Main Content Area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top Bar -->
      <header class="h-16 border-b border-morandi-border/60 bg-morandi-bg/80 backdrop-blur-md px-6 flex items-center justify-between sticky top-0 z-30">
        <div class="flex items-center gap-2">
          <h1 class="text-base font-semibold text-morandi-text">{{ getTitle() }}</h1>
        </div>
      </header>

      <!-- Page Content -->
      <main class="flex-1 p-4 md:p-8 max-w-7xl w-full mx-auto">
        <RouterView />
      </main>
    </div>
  </div>
</template>
