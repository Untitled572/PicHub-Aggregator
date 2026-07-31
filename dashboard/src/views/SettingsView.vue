<script setup lang="ts">
declare const __APP_VERSION__: string
import { ref, onMounted } from 'vue'
import { useApi, setAuthToken } from '../composables/useApi'
import type { Settings } from '../types'

const appVersion = __APP_VERSION__
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
  Key,
  History,
  Zap,
  Globe,
  Network,
  Cpu,
  X
} from 'lucide-vue-next'


const { getSettings, updateSettings } = useApi()

const settings = ref<Settings>({
  proxy_mode: false,
  proxy_enabled: false,
  proxy_url: 'http://127.0.0.1:7890',
  cache_max_mb: 500,
  cache_max_images: 120,
  cache_ttl: 0,
  min_resolution: '1920x1080',
  pool_size: 10,
  rate_limit: 60,
  rate_limit_window: 60,
  timeout: 3000,
  health_check_interval: 360,
  max_history_records: 60,
  saved_images_dir: './data/saved',
  admin_token: ''
})

const saving = ref(false)
const saved = ref(false)

onMounted(async () => {
  try {
    const s = await getSettings()
    if (s.cache_ttl === undefined || s.cache_ttl === null) s.cache_ttl = 0
    if (!s.min_resolution) s.min_resolution = '1920x1080'
    if (s.pool_size === undefined || s.pool_size === null) s.pool_size = 10
    if (!s.saved_images_dir) s.saved_images_dir = './data/saved'
    if (s.proxy_url === undefined || s.proxy_url === null) s.proxy_url = 'http://127.0.0.1:7890'
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
</script>

<template>
  <div class="space-y-6 pb-12">
    <!-- Header Banner -->
    <div class="morandi-card p-5 flex flex-col sm:flex-row items-center justify-between gap-4">
      <div class="flex items-center gap-3.5 min-w-0">
        <div class="w-11 h-11 rounded-2xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0 shadow-2xs">
          <Sliders class="w-5 h-5" />
        </div>
        <div class="min-w-0">
          <h2 class="font-bold text-base text-morandi-text">系统全局策略与设置中心</h2>
          <p class="text-xs text-morandi-muted mt-0.5">配置中转模式、网络代理、限流阀值与管理员安全认证</p>
        </div>
      </div>

      <div class="flex items-center gap-3 shrink-0">
        <Transition name="fade">
          <span v-if="saved" class="flex items-center gap-1.5 text-xs text-morandi-sage-dark font-medium bg-morandi-sage-light/60 px-3 py-1.5 rounded-xl border border-morandi-sage/20 animate-in fade-in">
            <CheckCircle2 class="w-3.5 h-3.5 text-morandi-sage" /> 设置已保存
          </span>
        </Transition>

        <button
          @click="handleSave"
          :disabled="saving"
          class="flex items-center gap-2 px-5 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-xs transition-all disabled:opacity-50 cursor-pointer active:scale-95"
        >
          <Save class="w-4 h-4" />
          <span>{{ saving ? '保存中...' : '保存全局设置' }}</span>
        </button>
        <span class="px-3 py-1 bg-morandi-sage/10 text-morandi-sage-dark text-xs font-bold rounded-xl font-mono border border-morandi-sage/20 shrink-0 whitespace-nowrap">
          {{ appVersion }}
        </span>
      </div>
    </div>


    <!-- Group 1: 🌐 网络与外网代理服务 -->
    <div class="morandi-card p-6 space-y-5">
      <div class="flex items-center justify-between pb-3 border-b border-morandi-border/60">
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center">
            <Globe class="w-4 h-4" />
          </div>
          <h3 class="text-sm font-bold text-morandi-text">网络抓取与 HTTP 代理</h3>
        </div>
      </div>

      <!-- Outbound Proxy Switch Card -->
      <div class="p-4 bg-morandi-bg/60 rounded-2xl border border-morandi-borderSoft flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div class="space-y-1">
          <div class="text-xs font-bold text-morandi-text flex items-center gap-2">
            <span>HTTP 代理服务器 (Outbound Proxy)</span>
            <span
              class="text-[10px] px-2 py-0.5 font-bold rounded-md transition-colors"
              :class="settings.proxy_enabled ? 'bg-morandi-sage-light text-morandi-sage-dark border border-morandi-sage/30' : 'bg-morandi-bg text-morandi-muted border border-morandi-borderSoft'"
            >
              {{ settings.proxy_enabled ? '已启用' : '已关闭' }}
            </span>
          </div>
          <p class="text-[11px] text-morandi-muted leading-relaxed">
            开启后，后台从第三方 API 抓取图片或测试探针时将统一通过指定的 HTTP 代理转发，适用于需代理访问的第三方图源。
          </p>
        </div>

        <button
          type="button"
          @click="settings.proxy_enabled = !settings.proxy_enabled"
          class="relative w-14 h-8 rounded-full p-1 transition-all duration-300 ease-in-out shrink-0 cursor-pointer focus:outline-none flex items-center select-none shadow-xs border"
          :class="settings.proxy_enabled
            ? 'bg-morandi-sage border-morandi-sage-dark/30 shadow-morandi-sage/20'
            : 'bg-stone-200/90 hover:bg-stone-300/80 border-stone-300/80'"
          role="switch"
          :aria-checked="settings.proxy_enabled"
          :title="settings.proxy_enabled ? '已启用 HTTP 代理，点击关闭' : '已关闭 HTTP 代理，点击开启'"
        >
          <span
            class="w-6 h-6 bg-white rounded-full shadow-md transition-all duration-300 ease-out transform flex items-center justify-center"
            :class="settings.proxy_enabled ? 'translate-x-6' : 'translate-x-0'"
          >
            <Check v-if="settings.proxy_enabled" class="w-3.5 h-3.5 text-morandi-sage-dark stroke-[3]" />
            <X v-else class="w-3 h-3 text-stone-400 font-bold" />
          </span>
        </button>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
        <div v-if="settings.proxy_enabled" class="sm:col-span-1">
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Network class="w-3.5 h-3.5 text-morandi-sage" /> 代理服务器地址 (URL)
          </label>
          <input v-model="settings.proxy_url" placeholder="http://127.0.0.1:7890" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">支持 HTTP / HTTPS 代理，如 http://127.0.0.1:7890</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Clock class="w-3.5 h-3.5 text-morandi-light" /> 单次请求超时时间 (毫秒 ms)
          </label>
          <input v-model.number="settings.timeout" type="number" step="500" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">抓取第三方图源超时自动切换下一备用源</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Clock class="w-3.5 h-3.5 text-morandi-light" /> 后台巡检轮询周期 (分钟)
          </label>
          <input v-model.number="settings.health_check_interval" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">定时健康探针后台自动检测图源连通性周期</p>
        </div>
      </div>
    </div>

    <!-- Group 2: 💾 本地缓存与秒级分发池 -->
    <div class="morandi-card p-6 space-y-5">
      <div class="flex items-center justify-between pb-3 border-b border-morandi-border/60">
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center">
            <HardDrive class="w-4 h-4" />
          </div>
          <h3 class="text-sm font-bold text-morandi-text">本地缓存模式</h3>
        </div>
      </div>

      <!-- Local Proxy Mode Switch Card -->
      <div class="p-4 bg-morandi-bg/60 rounded-2xl border border-morandi-borderSoft flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
        <div class="space-y-1">
          <div class="text-xs font-bold text-morandi-text flex items-center gap-2">
            <span>本地缓存中转模式</span>
            <span
              class="text-[10px] px-2 py-0.5 font-bold rounded-md transition-colors"
              :class="settings.proxy_mode ? 'bg-morandi-sage-light text-morandi-sage-dark border border-morandi-sage/30' : 'bg-morandi-bg text-morandi-muted border border-morandi-borderSoft'"
            >
              {{ settings.proxy_mode ? '本地磁盘缓存' : '302 直链重定向' }}
            </span>
          </div>
          <p class="text-[11px] text-morandi-muted leading-relaxed">
            开启后图片拉取存储至本地磁盘，支持精准计算像素像素尺寸过滤横竖屏、收藏图片；关闭时返回 302 重定向直链。
          </p>
        </div>

        <button
          type="button"
          @click="settings.proxy_mode = !settings.proxy_mode"
          class="relative w-14 h-8 rounded-full p-1 transition-all duration-300 ease-in-out shrink-0 cursor-pointer focus:outline-none flex items-center select-none shadow-xs border"
          :class="settings.proxy_mode
            ? 'bg-morandi-sage border-morandi-sage-dark/30 shadow-morandi-sage/20'
            : 'bg-stone-200/90 hover:bg-stone-300/80 border-stone-300/80'"
          role="switch"
          :aria-checked="settings.proxy_mode"
          :title="settings.proxy_mode ? '已开启本地磁盘缓存，点击切换 302 直链' : '已开启 302 直链，点击开启磁盘缓存'"
        >
          <span
            class="w-6 h-6 bg-white rounded-full shadow-md transition-all duration-300 ease-out transform flex items-center justify-center"
            :class="settings.proxy_mode ? 'translate-x-6' : 'translate-x-0'"
          >
            <Check v-if="settings.proxy_mode" class="w-3.5 h-3.5 text-morandi-sage-dark stroke-[3]" />
            <X v-else class="w-3 h-3 text-stone-400 font-bold" />
          </span>
        </button>
      </div>


      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Zap class="w-3.5 h-3.5 text-morandi-sage-dark font-bold" /> 预缓存分发池大小 (张)
          </label>
          <input v-model.number="settings.pool_size" type="number" min="0" max="50" class="morandi-input w-full px-3 py-2 font-mono text-xs font-bold text-morandi-sage-dark" />
          <p class="text-[10px] text-morandi-muted mt-1">后台提前拉取的图片数量，建议大于图片源数量</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Image class="w-3.5 h-3.5 text-morandi-light" /> 最小渲染分辨率过滤
          </label>
          <input v-model="settings.min_resolution" placeholder="1920x1080" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">自动丢弃像素低于该尺寸的低清图（0 表示关闭过滤）</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <HardDrive class="w-3.5 h-3.5 text-morandi-light" /> 永久归档图片保存目录
          </label>
          <input v-model="settings.saved_images_dir" placeholder="./data/saved" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">控制台点收藏保存后图片复制到的本地路径</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <HardDrive class="w-3.5 h-3.5 text-morandi-light" /> 最大磁盘缓存容量 (MB)
          </label>
          <input v-model.number="settings.cache_max_mb" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">超出限制后自动淘汰最老缓存文件</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Cpu class="w-3.5 h-3.5 text-morandi-light" /> 缓存图片数量上限 (张)
          </label>
          <input v-model.number="settings.cache_max_images" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">临界限制，超出时自动清理最早图片 (0 不限制)</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Clock class="w-3.5 h-3.5 text-morandi-light" /> 缓存过期时间 TTL (分钟)
          </label>
          <input v-model.number="settings.cache_ttl" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">缓存图片生存周期 (0 表示不随时间过期)</p>
        </div>
      </div>
    </div>

    <!-- Group 3: 🛡️ 防刷限流与历史日志 -->
    <div class="morandi-card p-6 space-y-5">
      <div class="flex items-center justify-between pb-3 border-b border-morandi-border/60">
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-morandi-sand/20 text-morandi-sand-dark flex items-center justify-center">
            <Shield class="w-4 h-4" />
          </div>
          <h3 class="text-sm font-bold text-morandi-text">防刷限流与历史保留</h3>
        </div>
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Gauge class="w-3.5 h-3.5 text-morandi-light" /> 单 IP 允许最大请求数 (次)
          </label>
          <input v-model.number="settings.rate_limit" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">一段时间内允许单个客户端 IP 发起的最大分发请求数</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <Clock class="w-3.5 h-3.5 text-morandi-light" /> 限流时长 (秒)
          </label>
          <input v-model.number="settings.rate_limit_window" type="number" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          <p class="text-[10px] text-morandi-muted mt-1">限流计算的滑动时间范围</p>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1">
            <History class="w-3.5 h-3.5 text-morandi-light" /> 分发历史流水保存上限 (条)
          </label>
          <input v-model.number="settings.max_history_records" type="number" min="10" max="1000" class="morandi-input w-full px-3 py-2 font-mono text-xs font-bold text-morandi-text" />
          <p class="text-[10px] text-morandi-muted mt-1">保留的历史记录数</p>
        </div>
      </div>
    </div>

    <!-- Group 4: 🔑 控制台安全鉴权 -->
    <div class="morandi-card p-6 space-y-5">
      <div class="flex items-center justify-between pb-3 border-b border-morandi-border/60">
        <div class="flex items-center gap-2">
          <div class="w-7 h-7 rounded-lg bg-morandi-rose/15 text-morandi-rose-dark flex items-center justify-center">
            <Key class="w-4 h-4" />
          </div>
          <h3 class="text-sm font-bold text-morandi-text">控制台管理员安全鉴权</h3>
        </div>
      </div>

      <div class="p-4 bg-morandi-sage-light/40 border border-morandi-sage/20 rounded-2xl text-xs text-morandi-text flex items-start gap-3">
        <div class="w-8 h-8 rounded-xl bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0 mt-0.5">
          <ShieldCheck class="w-4.5 h-4.5 text-morandi-sage-dark" />
        </div>
        <div class="space-y-1">
          <div class="font-bold text-morandi-sage-dark flex items-center gap-2">
            <span>管理员 Token 防护说明</span>
            <span class="text-[10px] px-2 py-0.2 bg-white text-morandi-sage-dark font-medium rounded-md border border-morandi-borderSoft">安全可选项</span>
          </div>
          <p class="text-[11px] text-morandi-muted leading-relaxed">
            配置密码后，控制台的所有修改、添加与删除写操作均必须在 Header 中携带 <code class="font-mono bg-white px-1.5 py-0.5 rounded text-morandi-sage-dark border border-morandi-borderSoft font-semibold">Authorization: Bearer &lt;Token&gt;</code>。公共 API 分发路径不受影响。留空表示开放匿名管理员操作。
          </p>
        </div>
      </div>

      <div>
        <label class="font-medium text-morandi-text block mb-1.5 flex items-center gap-1 text-xs">
          <Key class="w-3.5 h-3.5 text-morandi-light" /> Admin Token 密码
        </label>
        <input v-model="settings.admin_token" type="text" placeholder="留空表示公开访问与配置" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
      </div>
    </div>
  </div>
</template>

