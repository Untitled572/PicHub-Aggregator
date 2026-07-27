<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { Settings } from '../types'
import { Sliders, Save, CheckCircle2, Shield, HardDrive, Clock, Gauge, Image } from 'lucide-vue-next'

const { getSettings, updateSettings } = useApi()
const settings = ref<Settings>({
  proxy_mode: false,
  cache_max_mb: 200,
  cache_ttl: 60,
  min_resolution: '640x480',
  rate_limit: 60,
  timeout: 3000,
})
const saving = ref(false)
const saved = ref(false)

onMounted(async () => {
  try {
    settings.value = await getSettings()
  } catch {}
})

async function handleSave() {
  saving.value = true
  saved.value = false
  try {
    await updateSettings(settings.value)
    saved.value = true
    setTimeout(() => saved.value = false, 3000)
  } catch {}
  saving.value = false
}
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <!-- Header -->
    <div class="morandi-card p-5">
      <div class="flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <Sliders class="w-5 h-5" />
        </div>
        <div>
          <h2 class="font-bold text-base text-morandi-text">系统全局中转策略与设置</h2>
          <p class="text-xs text-morandi-muted mt-0.5">控制底层代理中转模式、防盗链解决、本地磁盘缓存及 Rate Limit 速率限制</p>
        </div>
      </div>
    </div>

    <!-- Form Panel -->
    <div class="morandi-card p-6 space-y-6">
      <!-- Section 1: Proxy Mode & Cache -->
      <div class="space-y-4">
        <h3 class="text-xs font-bold text-morandi-sage-dark uppercase tracking-wider flex items-center gap-1.5 pb-2 border-b border-morandi-border/60">
          <HardDrive class="w-4 h-4" /> 代理中转与缓存策略
        </h3>

        <!-- Proxy Mode Toggle -->
        <div class="flex items-center justify-between p-3.5 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft">
          <div class="space-y-0.5">
            <div class="text-xs font-semibold text-morandi-text">代理中转 / 缓存模式 (Proxy Mode)</div>
            <div class="text-[11px] text-morandi-muted leading-relaxed">
              开启后主机下载第三方图片并做本地磁盘缓存，隐藏客户端 IP，完美解决第三方 API 跨域与防盗链限制。
            </div>
          </div>
          <label class="relative inline-flex items-center cursor-pointer shrink-0 ml-4">
            <input type="checkbox" v-model="settings.proxy_mode" class="sr-only peer" />
            <div class="w-10 h-5 bg-morandi-sidebar peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-morandi-borderSoft after:border after:rounded-full after:h-4 after:w-4 after:transition-all duration-200 peer-checked:bg-morandi-sage"></div>
          </label>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
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
          <p class="text-[10px] text-morandi-muted mt-1">代理模式下自动丢弃低于该分辨率的低清图片源</p>
        </div>
      </div>

      <!-- Section 2: Rate Limit & Failover -->
      <div class="space-y-4 pt-2">
        <h3 class="text-xs font-bold text-morandi-ocean-dark uppercase tracking-wider flex items-center gap-1.5 pb-2 border-b border-morandi-border/60">
          <Shield class="w-4 h-4" /> 速率限制与容错阈值
        </h3>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
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
            <p class="text-[10px] text-morandi-muted mt-1">请求第三方源的超时间隔，超时自动重试下一个源</p>
          </div>
        </div>
      </div>

      <!-- Save Actions -->
      <div class="pt-4 border-t border-morandi-border/60 flex items-center gap-3">
        <button
          @click="handleSave"
          :disabled="saving"
          class="flex items-center gap-2 px-6 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all disabled:opacity-50"
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

