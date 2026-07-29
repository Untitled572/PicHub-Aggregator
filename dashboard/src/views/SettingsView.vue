<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi, setAuthToken } from '../composables/useApi'
import type { Settings } from '../types'
import {
  Sliders,
  Save,
  CheckCircle2,
  Shield,
  HardDrive,
  Clock,
  Gauge,
  Image,
  Wrench,
  AlertCircle,
  Key
} from 'lucide-vue-next'

const { getSettings, updateSettings } = useApi()

const activeTab = ref<'cache' | 'security' | 'admin'>('cache')

const settings = ref<Settings>({
  proxy_mode: false,
  cache_max_mb: 200,
  cache_ttl: 60,
  min_resolution: '640x480',
  rate_limit: 60,
  timeout: 3000,
  health_check_interval: 360,
})

const saving = ref(false)
const saved = ref(false)

onMounted(async () => {
  try {
    const s = await getSettings()
    settings.value = s
  } catch {}
})

async function handleSave() {
  saving.value = true
  saved.value = false
  try {
    const updated = await updateSettings(settings.value)
    if (updated && updated.admin_token) {
      setAuthToken(updated.admin_token)
    }
    saved.value = true
    setTimeout(() => saved.value = false, 3000)
  } catch {}
  saving.value = false
}

const tabs = [
  { id: 'cache', label: '代理中转与缓存', icon: HardDrive },
  { id: 'security', label: '速率与安全限制', icon: Shield },
  { id: 'admin', label: '管理员认证', icon: Key },
]
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->

    <div class="morandi-card p-5">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <Sliders class="w-5 h-5" />
        </div>
        <div>
          <h2 class="font-bold text-base text-morandi-text">系统全局中转策略与设置</h2>
          <p class="text-xs text-morandi-muted mt-0.5">配置代理缓存中转模式以及 Rate Limit 防刷保护</p>
        </div>
      </div>
    </div>

    <!-- Tabbed Settings Form Container -->
    <div class="morandi-card p-6 space-y-6">
      <!-- Top Paginated Navigation Tabs -->
      <div class="flex items-center gap-2 border-b border-morandi-border/60 pb-3 overflow-x-auto">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          type="button"
          @click="activeTab = tab.id as any"
          class="flex items-center gap-2 px-4 py-2.5 rounded-xl text-xs font-semibold transition-all shrink-0 cursor-pointer"
          :class="activeTab === tab.id
            ? 'bg-morandi-sage text-white shadow-xs'
            : 'bg-morandi-bg text-morandi-muted hover:bg-morandi-hover hover:text-morandi-text border border-morandi-borderSoft/60'"
        >
          <component :is="tab.icon" class="w-4 h-4" />
          <span>{{ tab.label }}</span>
        </button>
      </div>

      <!-- Tab Page 1: Proxy Mode & Cache Settings -->
      <div v-if="activeTab === 'cache'" class="space-y-4 animate-in fade-in duration-200">
        <div class="flex items-center justify-between pb-2 border-b border-morandi-border/40">
          <h3 class="text-xs font-bold text-morandi-sage-dark uppercase tracking-wider flex items-center gap-1.5">
            <HardDrive class="w-4 h-4" /> 代理中转与磁盘缓存策略
          </h3>
          <span class="text-[10px] px-2 py-0.5 bg-amber-100 text-amber-800 font-bold rounded-md flex items-center gap-1">
            <Wrench class="w-3 h-3" /> 功能待开发完善 (研发中)
          </span>
        </div>

        <!-- Development Notice Alert -->
        <div class="p-3.5 bg-amber-50/90 border border-amber-200 rounded-xl text-xs text-amber-900 flex items-start gap-2.5">
          <AlertCircle class="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
          <div class="space-y-0.5">
            <p class="font-bold">⚠️ 提示：本地磁盘代理缓存模式目前处于待开发/内部测试阶段</p>
            <p class="text-[11px] text-amber-800 leading-relaxed">
              当前 PicHub 引擎默认使用 302 重定向/直链中转分发。开启代理缓存模式后可能由于目标 API 限制导致无法下载缓存，高可用本地缓存模块正在加速开发中。
            </p>
          </div>
        </div>

        <!-- Proxy Mode Toggle -->
        <div class="flex items-center justify-between p-3.5 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft">
          <div class="space-y-0.5">
            <div class="text-xs font-semibold text-morandi-text flex items-center gap-1.5">
              <span>代理中转 / 缓存模式 (Proxy Mode)</span>
              <span class="text-[10px] px-1.5 py-0.2 bg-amber-100 text-amber-800 font-medium rounded">待开发</span>
            </div>
            <div class="text-[11px] text-morandi-muted leading-relaxed">
              开启后主机下载第三方图片并做本地磁盘缓存，隐藏客户端 IP，完美解决第三方 API 跨域与防盗链限制。
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer shrink-0 ml-4">
            <input type="checkbox" v-model="settings.proxy_mode" class="sr-only peer" />
            <div class="w-10 h-5 bg-morandi-sidebar peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-morandi-borderSoft after:border after:rounded-full after:h-4 after:w-4 after:transition-all duration-200 peer-checked:bg-morandi-sage"></div>
          </label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs pt-2">
          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <HardDrive class="w-3.5 h-3.5 text-morandi-light" /> 最大磁盘缓存容量 (MB)
            </label>
            <input v-model.number="settings.cache_max_mb" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">超过设定容量将自动清除最早过期的本地缓存文件</p>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Clock class="w-3.5 h-3.5 text-morandi-light" /> 缓存过期时间 TTL (分钟)
            </label>
            <input v-model.number="settings.cache_ttl" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">缓存图片的保存保留时长（默认 60 分钟）</p>
          </div>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
            <Image class="w-3.5 h-3.5 text-morandi-light" /> 最小渲染分辨率过滤 (Min Resolution)
          </label>
          <input v-model="settings.min_resolution" placeholder="例如: 1920x1080 或 800x600" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">代理模式下自动丢弃低于该分辨率的低清图片源。仅在代理模式开启时生效。</p>
        </div>
      </div>

      <!-- Tab Page 2: Security & Rate Limit Settings -->
      <div v-if="activeTab === 'security'" class="space-y-4 animate-in fade-in duration-200">
        <div class="flex items-center justify-between pb-2 border-b border-morandi-border/40">
          <h3 class="text-xs font-bold text-morandi-ocean-dark uppercase tracking-wider flex items-center gap-1.5">
            <Shield class="w-4 h-4" /> 速率限制与容错阈值设置
          </h3>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Gauge class="w-3.5 h-3.5 text-morandi-light" /> 全局请求速率限制 (次/分钟)
            </label>
            <input v-model.number="settings.rate_limit" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">防刷流量防护 Rate Limit (单个 IP 限制)</p>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Clock class="w-3.5 h-3.5 text-morandi-light" /> 单次请求响应超时 (毫秒 ms)
            </label>
            <input v-model.number="settings.timeout" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">请求第三方源的超时间隔，超时重试下一个源</p>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Clock class="w-3.5 h-3.5 text-morandi-sage font-bold" /> 后台健康检查轮询周期 (分钟)
            </label>
            <input v-model.number="settings.health_check_interval" type="number" class="morandi-input w-full px-3 py-2 font-mono font-bold text-morandi-sage-dark" />
            <p class="text-[10px] text-morandi-muted mt-1">后台巡检所有图源连通性的时间间隔 (默认 360 分钟 / 6 小时)</p>
          </div>
        </div>

      </div>

      <!-- Tab Page 3: Admin Authentication -->
      <div v-if="activeTab === 'admin'" class="space-y-4 animate-in fade-in duration-200">
        <div class="flex items-center justify-between pb-2 border-b border-morandi-border/40">
          <h3 class="text-xs font-bold text-morandi-sage-dark uppercase tracking-wider flex items-center gap-1.5">
            <Key class="w-4 h-4" /> 管理接口认证令牌
          </h3>
        </div>

        <div class="p-3.5 bg-blue-50/90 border border-blue-200 rounded-xl text-xs text-blue-900 flex items-start gap-2.5">
          <Key class="w-4 h-4 text-blue-600 shrink-0 mt-0.5" />
          <div class="space-y-0.5">
            <p class="font-bold">🔑 可选的 Admin Token 安全认证</p>
            <p class="text-[11px] text-blue-800 leading-relaxed">
              设置 token 后，Dashboard 的所有 POST/PUT/DELETE 写操作都需要在请求头携带 <code class="font-mono bg-blue-100 px-1 rounded">Authorization: Bearer &lt;token&gt;</code>。
              GET 读操作不受影响。留空表示不启用认证。
            </p>
          </div>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Key class="w-3.5 h-3.5 text-morandi-light" /> Admin Token
          </label>
          <input v-model="settings.admin_token" type="text" placeholder="留空则不启用认证" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">建议使用至少 16 位的随机字符串。设置后请保存，页面刷新后写操作将自动携带此 token。</p>
        </div>
      </div>

      <!-- Save Actions Footer -->
      <div class="pt-4 border-t border-morandi-border/60 flex items-center gap-3">
        <button
          @click="handleSave"
          :disabled="saving"
          class="flex items-center gap-2 px-6 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all disabled:opacity-50 cursor-pointer"
        >
          <Save class="w-4 h-4" />
          <span>{{ saving ? '保存设置中...' : '保存系统设置' }}</span>
        </button>

        <Transition name="fade">
          <span v-if="saved" class="flex items-center gap-1.5 text-xs text-morandi-sage-dark font-medium bg-morandi-sage-light/60 px-3 py-1.5 rounded-lg border border-morandi-sage/20">
            <CheckCircle2 class="w-4 h-4 text-morandi-sage" /> 系统设置更新已成功应用！
          </span>
        </Transition>
      </div>
    </div>
  </div>
</template>