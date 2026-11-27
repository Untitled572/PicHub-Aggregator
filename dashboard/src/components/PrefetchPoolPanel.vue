<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { Layers } from 'lucide-vue-next'
import { useApi } from '../composables/useApi'
import type { PrefetchRefill, PrefetchStatus } from '../types'

const { getPrefetchStatus } = useApi()
const pool = ref<PrefetchStatus>()
const lastRefill = ref<PrefetchRefill>()
let hideRefill: ReturnType<typeof setTimeout> | undefined
const error = ref('')
let timer: ReturnType<typeof setInterval> | undefined
let request: AbortController | undefined

function showLatestRefill(status: PrefetchStatus) {
  clearTimeout(hideRefill)
  const refill = status.last_refill
  const remaining = refill && 20_000 - (Date.parse(status.checked_at) - Date.parse(refill.added_at))
  lastRefill.value = remaining && remaining > 0 ? refill : undefined
  if (remaining && remaining > 0) {
    hideRefill = setTimeout(() => { lastRefill.value = undefined }, remaining)
  }
}

async function loadPool() {
  if (document.hidden || request) return
  const controller = new AbortController()
  request = controller
  try {
    const status = await getPrefetchStatus(controller.signal)
    if (!controller.signal.aborted) {
      pool.value = status
      showLatestRefill(status)
      error.value = ''
    }
  } catch (err) {
    if (!controller.signal.aborted) error.value = err instanceof Error ? err.message : '无法读取预取池状态'
  } finally {
    if (request === controller) request = undefined
  }
}

function stopRefresh() {
  clearInterval(timer)
  request?.abort()
  request = undefined
}

function updateRefresh() {
  stopRefresh()
  if (!document.hidden) {
    void loadPool()
    timer = setInterval(() => { void loadPool() }, 1000)
  }
}

onMounted(() => {
  document.addEventListener('visibilitychange', updateRefresh)
  updateRefresh()
})
onUnmounted(() => {
  document.removeEventListener('visibilitychange', updateRefresh)
  clearTimeout(hideRefill)
  stopRefresh()
})
</script>

<template>
  <section class="morandi-card p-4 sm:p-5 space-y-4" aria-label="预取池">
    <div class="grid grid-cols-[7rem_minmax(0,1fr)] items-start gap-3 sm:grid-cols-[10rem_minmax(0,1fr)]">
      <div :class="{ 'col-span-2': !lastRefill }">
        <h2 class="flex items-center gap-2 font-bold text-morandi-text">
          <Layers class="w-5 h-5 text-morandi-sage" /> 预取池
        </h2>
        <p v-if="pool" class="mt-1 text-xs text-morandi-muted">
          <span v-if="!pool.enabled || pool.paused" class="mr-2">{{ !pool.enabled ? '代理预取未启用' : '调试期间暂停' }}</span>
          <time :datetime="pool.checked_at">{{ new Date(pool.checked_at).toLocaleTimeString() }}</time>
        </p>
      </div>
      <div v-if="lastRefill" class="flex min-w-0 gap-1 text-left text-xs leading-5 text-morandi-muted">
        <span class="shrink-0">刚补池：</span>
        <div class="grid min-w-0 overflow-hidden" aria-live="polite">
          <Transition name="pool-refill">
            <span
              :key="lastRefill.image_id"
              class="col-start-1 row-start-1 truncate"
              :title="lastRefill.source_name"
            >{{ lastRefill.source_name }}</span>
          </Transition>
        </div>
      </div>
    </div>

    <p v-if="error" role="alert" class="text-xs text-rose-700">{{ error }}</p>
    <template v-if="pool">
      <div>
        <p class="font-mono text-2xl font-bold text-morandi-text">
          {{ pool.count }} <span class="font-medium text-morandi-muted">/ {{ pool.target }}</span>
        </p>
        <progress :value="pool.count" :max="pool.target || 1" class="w-full h-2 accent-slate-600" aria-label="库存占用" />
        <p class="text-xs text-morandi-muted mt-1">横图 {{ pool.horizontal }} · 竖图 {{ pool.vertical }} · 方图 {{ pool.square }}</p>
      </div>

      <div class="grid gap-4 lg:grid-cols-2">
        <div class="overflow-x-auto">
          <h3 class="mb-2 text-sm font-semibold">按标签</h3>
          <table class="w-full text-xs [&_td]:py-1.5 [&_th]:pb-1.5">
            <thead class="text-morandi-muted">
              <tr><th class="text-left">标签</th><th>库存 / 保底</th><th>横图</th><th>竖图</th><th>方图</th></tr>
            </thead>
            <tbody class="divide-y divide-morandi-border/40">
              <tr v-for="row in pool.by_tag" :key="row.id" class="text-center">
                <td class="text-left">{{ row.name }}</td>
                <td class="font-mono">{{ row.count }} / {{ row.target }}</td>
                <td>{{ row.horizontal }}</td><td>{{ row.vertical }}</td><td>{{ row.square }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <details class="min-w-0" open>
          <summary class="cursor-pointer mb-2 text-sm font-semibold">按图源</summary>
          <div v-if="pool.by_source.length" class="max-h-52 overflow-auto">
            <table class="w-full text-xs [&_td]:py-1.5 [&_th]:pb-1.5">
              <thead class="text-morandi-muted">
                <tr><th class="text-left">图源</th><th>库存</th><th>横图</th><th>竖图</th><th>方图</th></tr>
              </thead>
              <tbody class="divide-y divide-morandi-border/40">
                <tr v-for="row in pool.by_source" :key="row.id" class="text-center">
                  <td class="text-left break-all">{{ row.name }}</td>
                  <td class="font-mono">{{ row.count }}</td>
                  <td>{{ row.horizontal }}</td><td>{{ row.vertical }}</td><td>{{ row.square }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <p v-else class="text-xs text-morandi-muted">暂无可分发图片</p>
        </details>
      </div>
    </template>
  </section>
</template>


<style scoped>
.pool-refill-enter-active,
.pool-refill-leave-active {
  transition: transform 180ms ease-out;
}
.pool-refill-enter-from {
  transform: translateY(100%);
}
.pool-refill-leave-to {
  transform: translateY(-100%);
}
@media (prefers-reduced-motion: reduce) {
  .pool-refill-enter-active,
  .pool-refill-leave-active {
    transition: none;
  }
}
</style>
