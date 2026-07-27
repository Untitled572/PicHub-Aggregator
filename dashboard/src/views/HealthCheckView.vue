<script setup lang="ts">
import { onMounted } from 'vue'
import { useHealthCheck } from '../composables/useHealthCheck'
import HealthStatusBadge from '../components/HealthStatusBadge.vue'
import { Activity, RefreshCw, CheckCircle2, AlertTriangle, Clock, Layers } from 'lucide-vue-next'

const { results, lastRun, running, progress, summary, loadCached, runCheck } = useHealthCheck()

onMounted(async () => {
  await loadCached()
  if (results.value.length === 0) runCheck()
})
</script>

<template>
  <div class="space-y-6">
    <!-- Header & Action -->
    <div class="morandi-card p-5 flex items-center justify-between gap-4">
      <div>
        <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
          <Activity class="w-5 h-5 text-morandi-sage" /> 全局图源健康度与延迟排查
        </h2>
        <p class="text-xs text-morandi-muted mt-0.5">并发检测所有开启状态图源的响应延迟、HTTP 状态码及连通性</p>
      </div>

      <button
        @click="runCheck"
        :disabled="running"
        class="flex items-center gap-2 px-4 py-2 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all disabled:opacity-50 shrink-0"
      >
        <RefreshCw class="w-4 h-4" :class="{ 'animate-spin': running }" />
        <span>{{ running ? '检测诊断中...' : '重新发起检测' }}</span>
      </button>
    </div>

    <!-- Progress bar -->
    <div v-if="running" class="morandi-card p-4 space-y-2">
      <div class="flex justify-between text-xs text-morandi-muted">
        <span>健康探测进度</span>
        <span class="font-mono font-semibold text-morandi-sage-dark">{{ progress }}%</span>
      </div>
      <div class="w-full bg-morandi-sidebar rounded-full h-2 overflow-hidden">
        <div class="bg-morandi-sage h-2 rounded-full transition-all duration-300" :style="{ width: progress + '%' }"></div>
      </div>
    </div>



    <!-- Summary KPI Cards -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-4">
      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-sidebar text-morandi-muted flex items-center justify-center shrink-0">
          <Layers class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">节点总数</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5">{{ summary.total }}</p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-sage-light text-morandi-sage-dark flex items-center justify-center shrink-0">
          <CheckCircle2 class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">健康正常</p>
          <p class="text-xl font-bold text-morandi-sage-dark mt-0.5">{{ summary.available }}</p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-rose-light text-morandi-rose-dark flex items-center justify-center shrink-0">
          <AlertTriangle class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">异常挂起</p>
          <p class="text-xl font-bold text-morandi-rose-dark mt-0.5">{{ summary.failed }}</p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-sand-light text-morandi-sand-dark flex items-center justify-center shrink-0">
          <Clock class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">平均网络延迟</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5 font-mono">{{ summary.avgLatency }} <span class="text-xs font-normal text-morandi-muted">ms</span></p>
        </div>
      </div>
    </div>

    <!-- Health Results Table / Cards -->
    <div class="morandi-card overflow-hidden">
      <div class="p-4 border-b border-morandi-border/60 bg-morandi-bg/40 font-medium text-xs text-morandi-muted grid grid-cols-12 gap-2">
        <div class="col-span-3">节点名称</div>
        <div class="col-span-4">目标 API URL</div>
        <div class="col-span-2">连通状态</div>
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
            <HealthStatusBadge :available="r.available" />
          </div>
          <div class="col-span-1 text-right font-mono text-morandi-text font-medium">
            {{ r.status_code || '-' }}
          </div>
          <div class="col-span-2 text-right font-mono text-morandi-text font-bold">
            {{ r.latency_ms }} <span class="text-[10px] font-normal text-morandi-muted">ms</span>
          </div>
        </div>
      </div>

      <div v-else class="p-12 text-center text-xs text-morandi-muted">
        正在获取健康诊断数据或尚未配置开启的图源...
      </div>
    </div>
  </div>
</template>

