<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { RouterView, RouterLink, useRoute, useRouter } from 'vue-router'
import { useTags } from './composables/useTags'
import { useApi, getAuthToken, setAuthToken } from './composables/useApi'
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
  Link2,
  BarChart3,
  Heart,
  Menu,
  X
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const copiedUrl = ref(false)
const backendConnected = ref(true)
const mobileMenuOpen = ref(false)
const { loadTags } = useTags()
const { getSettings } = useApi()

// 无布局路由 (如 /login): 全屏渲染, 不显示侧边栏/顶栏
const isBarePage = computed(() => !!route.meta.bare)

// 登录态确认前不渲染控制台, 避免未登录闪现
const authReady = ref(false)

const navItems = [
  { path: '/', label: '图源管理', subtitle: 'API 源配置与监听', icon: Layers },
  { path: '/endpoints', label: '接口管理', subtitle: '分类分发与总接口配置', icon: Link2 },
  { path: '/health', label: '健康检测', subtitle: '可用性与延迟评估', icon: Activity },
  { path: '/stats', label: '使用统计', subtitle: '请求数、Tag/图源命中与记录', icon: BarChart3 },
  { path: '/saved', label: '保存图片', subtitle: '已保存的图片', icon: Heart },
  { path: '/settings', label: '系统设置', subtitle: '中转模式与缓存策略', icon: Sliders },
]

// Close mobile drawer when route changes
watch(() => route.path, () => {
  mobileMenuOpen.value = false
  // 登录后从 /login 回跳受保护页面: 组件不重新挂载, 需手动解除渲染锁
  if (!isBarePage.value && !authReady.value) {
    authReady.value = true
  }
})

function isActive(path: string) {
  if (path === '/') return route.path === '/'
  return route.path.startsWith(path)
}

function getTitle() {
  const current = navItems.find(item => isActive(item.path))
  return current ? current.label : '控制台'
}

const isMobileDevice = ref(false)

function checkMobileDevice() {
  const ua = navigator.userAgent.toLowerCase()
  const isMobileUA = /mobile|android|iphone|ipad|phone|ipod|blackberry|windows phone/i.test(ua)
  const isSmallScreen = window.innerWidth < 768
  isMobileDevice.value = isMobileUA || isSmallScreen
}

async function checkBackendHealth() {
  try {
    const res = await fetch('/ping', { method: 'GET' })
    backendConnected.value = res.ok
  } catch {
    backendConnected.value = false
  }
}

async function checkLoginState() {
  try {
    if (getAuthToken()) {
      // 有 token: 验证服务端会话是否仍有效 (服务重启后内存会话清空 → 失效)
      const res = await fetch('/api/auth/check', {
        headers: { Authorization: `Bearer ${getAuthToken()}` },
      })
      if (res.ok) {
        const r = await res.json()
        if (r.login_enabled && !r.valid) {
          setAuthToken('')
          router.replace('/login')
          return
        }
        authReady.value = true
        return
      }
    } else {
      const s = await getSettings()
      if (s.login_enabled) {
        router.replace('/login')
        return
      }
      authReady.value = true
      return
    }
  } catch {
    // 后端不可达: 保持占位, 2 秒后重试 (服务就绪后自动判定登录态)
    setTimeout(checkLoginState, 2000)
    return
  }
  authReady.value = true
}

let healthTimer: any = null


onMounted(() => {
  checkLoginState().then(() => {
    if (!authReady.value) return // 跳登录中, 不初始化控制台逻辑
    loadTags()
    checkBackendHealth()
  })
  checkMobileDevice()
  window.addEventListener('resize', checkMobileDevice)
  healthTimer = setInterval(checkBackendHealth, 8000)
})

onUnmounted(() => {
  if (healthTimer) clearInterval(healthTimer)
  window.removeEventListener('resize', checkMobileDevice)
})


function copyUserApiUrl() {
  const apiUrl = `${window.location.origin}/random`
  navigator.clipboard.writeText(apiUrl)
  copiedUrl.value = true
  setTimeout(() => copiedUrl.value = false, 2500)
}
</script>

<template>
  <!-- 无布局路由 (登录页): 全屏独立渲染 -->
  <div v-if="isBarePage" class="min-h-screen">
    <RouterView />
  </div>

  <!-- 登录态确认中: 轻量占位, 不闪现控制台 -->
  <div v-else-if="!authReady" class="min-h-screen bg-morandi-bg flex items-center justify-center">
    <div class="flex flex-col items-center gap-3">
      <div class="w-10 h-10 rounded-2xl bg-morandi-sage text-white flex items-center justify-center shadow-md shadow-morandi-sage/20">
        <Sparkles class="w-5 h-5 animate-pulse" />
      </div>
      <p class="text-xs text-morandi-muted">正在验证登录状态…</p>
    </div>
  </div>

  <!-- 控制台布局 -->
  <div v-else class="min-h-screen bg-morandi-bg flex flex-col md:flex-row text-morandi-text font-sans">
    <!-- Desktop Sidebar (Hidden on Mobile) -->
    <aside class="hidden md:flex w-64 bg-morandi-sidebar/80 backdrop-blur-md border-r border-morandi-border/60 flex-col justify-between p-4 shrink-0 sticky top-0 h-screen overflow-y-auto z-20">
      <div>
        <!-- Brand / Header -->
        <div class="flex items-center gap-3 px-3 py-3 mb-6">
          <div class="w-10 h-10 rounded-xl bg-morandi-sage text-white flex items-center justify-center shadow-md shadow-morandi-sage/20">
            <Sparkles class="w-5 h-5" />
          </div>
          <div>
            <div class="flex items-center gap-1.5">
              <span class="font-bold text-base tracking-tight text-morandi-text">PicHub</span>
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

    <!-- Mobile Slide-Over Backdrop Overlay -->
    <div
      v-if="mobileMenuOpen"
      @click="mobileMenuOpen = false"
      class="fixed inset-0 bg-stone-950/40 backdrop-blur-xs z-40 md:hidden transition-opacity"
    ></div>

    <!-- Mobile Slide-Over Drawer Navigation -->
    <aside
      class="fixed top-0 left-0 bottom-0 w-72 bg-morandi-sidebar/95 backdrop-blur-xl border-r border-morandi-border/60 flex flex-col justify-between p-4 z-50 md:hidden transition-transform duration-300 shadow-2xl"
      :class="mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <div>
        <div class="flex items-center justify-between px-3 py-3 mb-6 border-b border-morandi-border/40">
          <div class="flex items-center gap-3">
            <div class="w-9 h-9 rounded-xl bg-morandi-sage text-white flex items-center justify-center shadow-md shadow-morandi-sage/20">
              <Sparkles class="w-4 h-4" />
            </div>
            <div>
              <span class="font-bold text-base tracking-tight text-morandi-text">PicHub</span>
              <p class="text-[10px] text-morandi-muted">图源聚合中转引擎</p>
            </div>
          </div>
          <button @click="mobileMenuOpen = false" class="p-2 text-morandi-muted hover:text-morandi-text rounded-xl">
            <X class="w-5 h-5" />
          </button>
        </div>

        <nav class="space-y-1.5">
          <RouterLink
            v-for="item in navItems"
            :key="item.path"
            :to="item.path"
            @click="mobileMenuOpen = false"
            class="group flex items-center gap-3 px-3.5 py-3 rounded-xl transition-all duration-200"
            :class="isActive(item.path)
              ? 'bg-white text-morandi-sage-dark shadow-sm border border-morandi-borderSoft font-semibold'
              : 'text-morandi-muted hover:bg-white/60 hover:text-morandi-text'"
          >
            <component
              :is="item.icon"
              class="w-4 h-4"
              :class="isActive(item.path) ? 'text-morandi-sage' : 'text-morandi-light'"
            />
            <div class="flex flex-col">
              <span class="text-xs tracking-wide leading-tight">{{ item.label }}</span>
              <span class="text-[10px] opacity-70 font-normal leading-tight mt-0.5">{{ item.subtitle }}</span>
            </div>
          </RouterLink>
        </nav>
      </div>

      <div class="pt-4 border-t border-morandi-border/40 px-2 space-y-2">
        <div
          @click="copyUserApiUrl"
          class="bg-white/80 rounded-xl p-3 border border-morandi-borderSoft flex items-center justify-between cursor-pointer"
        >
          <div class="flex items-center gap-2.5">
            <span class="relative flex h-2.5 w-2.5">
              <span class="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" :class="backendConnected ? 'bg-emerald-400' : 'bg-rose-400'"></span>
              <span class="relative inline-flex rounded-full h-2.5 w-2.5" :class="backendConnected ? 'bg-emerald-500' : 'bg-rose-500'"></span>
            </span>
            <span class="text-xs font-semibold text-morandi-text">{{ backendConnected ? '节点就绪' : '未就绪' }}</span>
          </div>
          <component :is="copiedUrl ? Check : Copy" class="w-4 h-4 text-morandi-muted" />
        </div>
      </div>
    </aside>

    <!-- Main Content Area -->
    <div class="flex-1 flex flex-col min-w-0">
      <!-- Top Sticky Header Bar -->
      <header class="h-14 md:h-16 border-b border-morandi-border/60 bg-morandi-bg/80 backdrop-blur-md px-4 sm:px-6 flex items-center justify-between sticky top-0 z-30">
        <div class="flex items-center gap-2.5">
          <button
            @click="mobileMenuOpen = !mobileMenuOpen"
            class="p-2 -ml-1 text-morandi-text hover:bg-white/60 rounded-xl transition-colors md:hidden cursor-pointer"
            title="展开菜单"
          >
            <component :is="mobileMenuOpen ? X : Menu" class="w-5 h-5" />
          </button>
          <h1 class="text-sm md:text-base font-bold text-morandi-text">{{ getTitle() }}</h1>
        </div>

        <!-- Node status quick indicator on mobile header -->
        <div
          @click="copyUserApiUrl"
          class="flex items-center gap-1.5 px-2.5 py-1 bg-white/80 hover:bg-white rounded-xl border border-morandi-borderSoft cursor-pointer md:hidden text-xs font-semibold text-morandi-text shadow-xs"
        >
          <span class="relative flex h-2 w-2">
            <span class="animate-ping absolute inline-flex h-full w-full rounded-full opacity-75" :class="backendConnected ? 'bg-emerald-400' : 'bg-rose-400'"></span>
            <span class="relative inline-flex rounded-full h-2 w-2" :class="backendConnected ? 'bg-emerald-500' : 'bg-rose-500'"></span>
          </span>
          <span class="text-[11px] font-mono">{{ copiedUrl ? '已复制' : (backendConnected ? '节点就绪' : '离线') }}</span>
        </div>
      </header>

      <!-- Page Content Area -->
      <main class="flex-1 p-3 sm:p-5 md:p-8 max-w-7xl w-full mx-auto pb-20 md:pb-8">
        <RouterView />
      </main>

      <!-- Fixed Mobile Bottom Navigation Quick Bar -->
      <nav class="fixed bottom-0 left-0 right-0 h-14 bg-white/95 backdrop-blur-md border-t border-morandi-borderSoft/80 flex items-center justify-around z-30 md:hidden px-1 shadow-lg">
        <RouterLink
          v-for="item in navItems"
          :key="item.path"
          :to="item.path"
          @click="mobileMenuOpen = false"
          class="flex flex-col items-center justify-center py-1 px-2 rounded-xl transition-all select-none"
          :class="isActive(item.path) ? 'text-morandi-sage-dark font-bold' : 'text-morandi-muted opacity-70 hover:opacity-100'"
        >
          <component :is="item.icon" class="w-4 h-4" :class="isActive(item.path) ? 'text-morandi-sage' : ''" />
          <span class="text-[10px] mt-0.5 font-medium leading-none">{{ item.label }}</span>
        </RouterLink>
      </nav>
    </div>
  </div>
</template>

