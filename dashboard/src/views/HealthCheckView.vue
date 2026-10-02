<script setup lang="ts">
import { onMounted } from 'vue'
import { useHealthCheck } from '../composables/useHealthCheck'
import HealthStatusBadge from '../components/HealthStatusBadge.vue'
import { Activity, RefreshCw, CheckCircle2, AlertTriangle, Clock, Layers } from 'lucide-vue-next'

const { results, lastRun, running, error, summary, loadCached, runCheck } = useHealthCheck()

onMounted(async () => {
  await loadCached()

})
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Action -->
    <div class="morandi-card p-4 sm:p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
      <div>
        <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
          <Activity class="w-5 h-5 text-morandi-sage" /> 图源检测
        </h2>
        <p class="text-xs text-morandi-muted mt-0.5">按图源的参数、请求头和代理设置，检查主地址与分支的响应。</p>
      </div>

      <button
        @click="runCheck"
        :disabled="running"
        class="w-full sm:w-auto px-4 py-2 text-xs font-semibold bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft shadow-xs flex items-center justify-center gap-2 transition-all cursor-pointer disabled:opacity-50 shrink-0 whitespace-nowrap"
      >
        <RefreshCw class="w-3.5 h-3.5 text-morandi-sage" :class="{ 'animate-spin': running }" />
        <span class="whitespace-nowrap">{{ running ? '检测中…' : '重新检测' }}</span>
      </button>
    </div>

    <p class="text-xs text-morandi-muted" v-if="lastRun && !lastRun.startsWith('0001')">
      上次检测：{{ new Date(lastRun).toLocaleString() }}。结果仅反映检测当时的响应；主地址或部分分支通过，不代表所有图片均可下载。
    </p>
    <div v-if="running" role="status" class="morandi-card p-4 text-xs text-morandi-muted flex items-center gap-2">
      <RefreshCw class="w-4 h-4 animate-spin" /> 正在检查已启用图源，请稍候…
    </div>
    <div v-if="error" role="alert" class="morandi-card p-4 text-xs text-rose-700">{{ error }}</div>

    <!-- Summary KPI Cards -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3.5">
      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-sidebar text-morandi-muted flex items-center justify-center shrink-0">
          <Layers class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium whitespace-nowrap">已检测图源</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5 font-mono">{{ summary.total }}</p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-sage-light text-morandi-sage-dark flex items-center justify-center shrink-0">
          <CheckCircle2 class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium whitespace-nowrap">检测通过</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-sage-dark mt-0.5 font-mono">{{ summary.available }}</p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-rose-light text-morandi-rose-dark flex items-center justify-center shrink-0">
          <AlertTriangle class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium whitespace-nowrap">本次未通过</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-rose-dark mt-0.5 font-mono">{{ summary.failed }}</p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-sand-light text-morandi-sand-dark flex items-center justify-center shrink-0">
          <Clock class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium whitespace-nowrap">平均延迟</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5 font-mono">{{ summary.avgLatency }} <span class="text-[10px] sm:text-xs font-normal text-morandi-muted">ms</span></p>
        </div>
      </div>
    </div>

    <!-- Health Results Table / Cards -->
    <div class="morandi-card overflow-x-auto">
      <div class="min-w-[640px]">
        <div class="p-4 border-b border-morandi-border/60 bg-morandi-bg/40 font-medium text-xs text-morandi-muted grid grid-cols-12 gap-2">
          <div class="col-span-3">图源名称</div>
          <div class="col-span-4">检测地址</div>
          <div class="col-span-2">检测结果</div>
          <div class="col-span-1 text-right">HTTP 状态</div>
          <div class="col-span-2 text-right">响应延迟</div>
        </div>

        <div v-if="results.length > 0" class="divide-y divide-morandi-border/40">
          <div
            v-for="r in results"
            :key="r.id"
            class="p-4 grid grid-cols-12 gap-2 text-xs items-center hover:bg-morandi-bg/50 transition-colors"
          >
            <div class="col-span-3 font-semibold text-morandi-text truncate">{{ r.name }}</div>
            <div class="col-span-4 text-morandi-muted font-mono truncate text-[11px]">{{ r.url }}</div>
            <div class="col-span-2">
              <HealthStatusBadge :available="r.available" :detail="r.error" />
            </div>
            <div class="col-span-1 text-right font-mono text-morandi-text font-medium">
              {{ r.status_code || '-' }}
            </div>
            <div class="col-span-2 text-right font-mono text-morandi-text font-bold">
              {{ r.latency_ms }} <span class="text-[10px] font-normal text-morandi-muted">ms</span>
            </div>
            <p v-if="r.error" class="col-span-12 text-amber-700 break-words mt-1">{{ r.error }}</p>
            <p v-if="r.checked_endpoints && r.checked_endpoints > 1" class="col-span-12 text-morandi-muted">通过 {{ r.available_endpoints }} / {{ r.checked_endpoints }} 个地址</p>
          </div>
        </div>

        <div v-else class="p-12 text-center text-xs text-morandi-muted">
          暂无检测结果。启用图源后，点击“重新检测”。
        </div>
      </div>
    </div>

  </div>
</template>

