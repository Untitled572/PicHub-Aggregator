<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi, setAuthToken } from '../composables/useApi'
import type { Settings } from '../types'
import {
  Sliders,
  Save,
  CheckCircle2,
  Check,
  Shield,
  ShieldCheck,
  HardDrive,
  Clock,
  Gauge,
  Image,
  Wrench,
  AlertCircle,
  Key,
  History,
  Zap
} from 'lucide-vue-next'




const { getSettings, updateSettings } = useApi()

const activeTab = ref<'cache' | 'security' | 'admin'>('cache')

const settings = ref<Settings>({
  proxy_mode: false,
  cache_max_mb: 200,
  cache_max_images: 60,
  cache_ttl: 0,
  min_resolution: '1920x1080',
  rate_limit: 60,
  timeout: 3000,
  health_check_interval: 360,
  max_history_records: 60,
  saved_images_dir: './data/saved',
})


const saving = ref(false)
const saved = ref(false)

onMounted(async () => {
  try {
    const s = await getSettings()
    if (s.cache_ttl === undefined || s.cache_ttl === null) s.cache_ttl = 0
    if (!s.min_resolution) s.min_resolution = '1920x1080'
    if (!s.saved_images_dir) s.saved_images_dir = './data/saved'
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

    <div class="morandi-card p-5 flex flex-col sm:flex-row items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <Sliders class="w-5 h-5" />
        </div>
        <div>
          <h2 class="font-bold text-base text-morandi-text">系统全局中转策略与设置</h2>
          <p class="text-xs text-morandi-muted mt-0.5">配置代理缓存中转模式以及 Rate Limit 防刷保护</p>
        </div>
      </div>

      <!-- Version Badge (Vertically Centered) -->
      <div class="flex items-center justify-center shrink-0 my-auto">
        <span class="px-3 py-1 bg-morandi-sage text-white text-xs font-bold rounded-xl shadow-xs font-mono leading-none flex items-center justify-center">
          v0.5.0
        </span>
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
        </div>

        <!-- Proxy Mode Toggle (Redesigned) -->
        <div class="flex items-center justify-between p-4 bg-morandi-bg/60 rounded-2xl border border-morandi-borderSoft shadow-2xs">
          <div class="space-y-1 pr-4">
            <div class="text-xs font-bold text-morandi-text flex items-center gap-2">
              <span>本地缓存模式</span>
              <span
                class="text-[10px] px-2 py-0.5 font-bold rounded-md transition-colors"
                :class="settings.proxy_mode ? 'bg-morandi-sage-light text-morandi-sage-dark' : 'bg-morandi-bg text-morandi-muted border border-morandi-borderSoft'"
              >
                {{ settings.proxy_mode ? '已开启 (磁盘中转缓存)' : '已关闭 (302 重定向直链)' }}
              </span>
            </div>
            <div class="text-[11px] text-morandi-muted leading-relaxed">
              开启后服务器自动将第三方图片下载并存储在本地磁盘中转分发，支持按图片真实像素尺寸精准过滤横竖屏，支持保存收藏偏好图片。关闭时保持 302 重定向直链分发。
            </div>
          </div>

          <!-- Redesigned Custom Toggle Switch -->
          <button
            type="button"
            @click="settings.proxy_mode = !settings.proxy_mode"
            class="w-12 h-[26px] rounded-full p-[2px] transition-colors duration-300 ease-in-out shrink-0 cursor-pointer focus:outline-none flex items-center shadow-inner"
            :class="settings.proxy_mode ? 'bg-morandi-sage' : 'bg-morandi-border/80 hover:bg-morandi-border'"
            :aria-checked="settings.proxy_mode"
            role="switch"
          >
            <span
              class="w-[22px] h-[22px] bg-white rounded-full shadow-md transition-transform duration-300 ease-in-out transform flex items-center justify-center"
              :class="settings.proxy_mode ? 'translate-x-[22px]' : 'translate-x-0'"
            >
              <Check v-if="settings.proxy_mode" class="w-3 h-3 text-morandi-sage-dark font-bold" />
            </span>
          </button>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs pt-2">
          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <HardDrive class="w-3.5 h-3.5 text-morandi-light" /> 最大磁盘缓存容量 (MB)
            </label>
            <input v-model.number="settings.cache_max_mb" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">超过容量后淘汰最早的缓存文件</p>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Image class="w-3.5 h-3.5 text-morandi-light" /> 缓存图片数量上限
            </label>
            <input v-model.number="settings.cache_max_images" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">超过数量上限后自动淘汰最早的图片（0 表示不自动清理）</p>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
              <Clock class="w-3.5 h-3.5 text-morandi-light" /> 缓存过期时间 TTL (分钟)
            </label>
            <input v-model.number="settings.cache_ttl" type="number" class="morandi-input w-full px-3 py-2 font-mono" />
            <p class="text-[10px] text-morandi-muted mt-1">缓存图片的保存保留时长（0 表示不随时间自动过期）</p>
          </div>
        </div>


        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
            <Image class="w-3.5 h-3.5 text-morandi-light" /> 最小渲染分辨率过滤
          </label>
          <input v-model="settings.min_resolution" placeholder="例如: 1920x1080 或 0 关闭" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">代理模式下自动丢弃低于该分辨率的低清图片。 （0 表示关闭分辨率过滤）。</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
            <Zap class="w-3.5 h-3.5 text-morandi-sage-dark font-bold" /> 预缓存图片数量 (张)
          </label>
          <input v-model.number="settings.precache_count" type="number" min="0" max="50" class="morandi-input w-full px-3 py-2 font-mono text-xs font-bold text-morandi-sage-dark" />
          <p class="text-[10px] text-morandi-muted mt-1">开启代理模式后，后台自动提前从主接口中转下载并缓存指定数量的图片。</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
            <HardDrive class="w-3.5 h-3.5 text-morandi-light" /> 保存图片目录
          </label>
          <input v-model="settings.saved_images_dir" placeholder="./data/saved" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">保存图片时复制到的本地存储路径。</p>
        </div>



        <!-- History Log Storage Limit Section -->
        <div class="pt-4 border-t border-morandi-border/40 space-y-3">
          <div class="text-xs font-bold text-morandi-text flex items-center gap-1.5">
            <History class="w-4 h-4 text-morandi-sage" /> 分发历史流水日志保留策略
          </div>
          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
              <History class="w-3.5 h-3.5 text-morandi-light" /> 分发历史记录保存上限 (条)
            </label>
            <input v-model.number="settings.max_history_records" type="number" min="10" max="1000" class="morandi-input w-full px-3 py-2 font-mono text-xs font-bold text-morandi-text" />
            <p class="text-[10px] text-morandi-muted mt-1">控制台调取历史流水保存的最大条数（默认 60 条，超出部分将自动滚动删除清理）。</p>
          </div>
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
            <p class="text-[10px] text-morandi-muted mt-1">后台巡检所有图源连通性的时间间隔</p>
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

        <div class="p-4 bg-morandi-sage-light/60 border border-morandi-sage/20 rounded-xl text-xs text-morandi-text flex items-start gap-3 shadow-xs">
          <div class="w-8 h-8 rounded-lg bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0">
            <ShieldCheck class="w-4 h-4 text-morandi-sage-dark" />
          </div>
          <div class="space-y-1">
            <div class="font-bold text-morandi-sage-dark flex items-center gap-2">
              <span>控制台接口安全鉴权 (Admin Token)</span>
              <span class="text-[10px] px-2 py-0.2 bg-white text-morandi-sage-dark font-medium rounded-md border border-morandi-borderSoft">可选防护</span>
            </div>
            <p class="text-[11px] text-morandi-muted leading-relaxed">
              配置密钥后，聚合控制台的所有修改、添加与删除操作（POST / PUT / DELETE）均需校验请求头 <code class="font-mono bg-white px-1.5 py-0.5 rounded text-morandi-sage-dark border border-morandi-borderSoft font-semibold">Authorization: Bearer &lt;Token&gt;</code>。对外分发接口（/random）不受影响，留空表示开放匿名配置。
            </p>
          </div>
        </div>


        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Key class="w-3.5 h-3.5 text-morandi-light" /> Admin Token
          </label>
          <input v-model="settings.admin_token" type="text" placeholder="留空则不启用认证" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">设置后请保存，页面刷新后写操作将自动携带此 token。</p>
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
            <CheckCircle2 class="w-4 h-4 text-morandi-sage" /> 系统设置已更新
          </span>
        </Transition>
      </div>
    </div>
  </div>
</template>
