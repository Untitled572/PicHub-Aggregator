<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { Bug, ExternalLink, RefreshCw, X } from 'lucide-vue-next'
import { useSourceDebug } from '../composables/useSourceDebug'

const { state, loading, initialized, changing, busy, error, refresh, disable } = useSourceDebug()
let pollTimer: ReturnType<typeof setInterval> | null = null

function onFocus() {
  void refresh()
}

onMounted(() => {
  void refresh()
  pollTimer = setInterval(() => void refresh(), 5000)
  window.addEventListener('focus', onFocus)
})

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer)
  window.removeEventListener('focus', onFocus)
})

async function handleDisable() {
  try {
    await disable()
  } catch {
    // Shared error state is displayed below.
  }
}

function openDebugImage() {
  window.open('/random', '_blank', 'noopener,noreferrer')
}
</script>

<template>
  <div v-if="state.enabled" class="rounded-2xl border border-amber-300 bg-amber-50 px-4 py-4 sm:px-5 shadow-sm mb-5">
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
      <div class="flex gap-3 min-w-0">
        <div class="w-9 h-9 rounded-xl bg-amber-200/70 text-amber-800 flex items-center justify-center shrink-0"><Bug class="w-4 h-4" /></div>
        <div class="min-w-0">
          <p class="text-sm font-bold text-amber-950">全局图源调试已开启：{{ state.source_name || `图源 #${state.source_id ?? '未知'}` }}</p>
          <p class="text-xs text-amber-900/80 mt-1">/random 和 /e 只使用此图源，按分类选择主地址或匹配分支，并应用方向筛选。预取池补充已暂停，不读取已有缓存，也不写入新缓存；成功取图会计入使用统计和图片历史。退出或服务重启后恢复普通分发。</p>
          <p v-if="state.switching || changing" class="mt-2 text-xs font-semibold text-amber-900" role="status">正在等待已有取图任务结束…</p>
        </div>
      </div>
      <div class="flex items-center gap-2 sm:shrink-0 pl-12 sm:pl-0">
        <button @click="openDebugImage" class="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold bg-white hover:bg-amber-100 text-amber-950 rounded-xl border border-amber-300 transition-colors cursor-pointer">
          <ExternalLink class="w-3.5 h-3.5" /> 调试取图
        </button>
        <button @click="handleDisable" :disabled="busy" class="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold bg-amber-700 hover:bg-amber-800 disabled:opacity-50 text-white rounded-xl transition-colors cursor-pointer">
          <X class="w-3.5 h-3.5" /> {{ changing || state.switching ? '等待中…' : '退出调试' }}
        </button>
      </div>
    </div>
  </div>
  <div v-if="!state.enabled && (changing || state.switching)" class="mb-5 rounded-xl border border-amber-300 bg-amber-50 px-4 py-3 text-sm text-amber-900" role="status">正在切换调试图源，等待已有取图任务结束…</div>
  <div v-if="error" role="alert" class="mb-5 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-800">
    <span>{{ error }}</span>
    <button @click="refresh" :disabled="loading || busy" class="inline-flex items-center gap-1.5 rounded-lg border border-rose-200 bg-white px-2.5 py-1.5 font-semibold disabled:opacity-50">
      <RefreshCw class="w-3.5 h-3.5" :class="loading ? 'animate-spin' : ''" /> 重试
    </button>
  </div>
  <div v-else-if="loading && !initialized && !state.enabled" class="mb-5 text-xs text-morandi-muted" role="status">正在读取调试状态…</div>
</template>
