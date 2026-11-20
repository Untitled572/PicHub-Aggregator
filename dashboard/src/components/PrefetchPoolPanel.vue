<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Layers, RefreshCw } from 'lucide-vue-next'
import { useApi } from '../composables/useApi'
import type { PrefetchStatus } from '../types'

const { getPrefetchStatus } = useApi()
const pool = ref<PrefetchStatus>()
const loading = ref(false)
const error = ref('')

async function loadPool() {
  if (loading.value) return
  loading.value = true
  error.value = ''
  try {
    pool.value = await getPrefetchStatus()
  } catch (err) {
    error.value = err instanceof Error ? err.message : '无法读取预取池状态'
  } finally {
    loading.value = false
  }
}

onMounted(loadPool)
</script>

<template>
  <section class="morandi-card p-4 sm:p-5 space-y-4" aria-label="预取池">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h2 class="flex items-center gap-2 font-bold text-morandi-text">
          <Layers class="w-5 h-5 text-morandi-sage" /> 预取池
        </h2>
        <p v-if="pool" class="mt-1 text-xs text-morandi-muted">
          {{ !pool.enabled ? '代理预取未启用' : pool.paused ? '调试期间暂停' : '可分发库存' }}
          <span class="ml-1">· {{ new Date(pool.checked_at).toLocaleTimeString() }}</span>
        </p>
      </div>
      <button
        type="button"
        class="flex items-center gap-1.5 px-3 py-2 border border-morandi-borderSoft rounded-lg text-xs hover:bg-morandi-hover disabled:opacity-50"
        :disabled="loading"
        @click="loadPool"
      >
        <RefreshCw class="w-3.5 h-3.5" :class="{ 'animate-spin': loading }" />
        {{ loading ? '读取中…' : '刷新库存' }}
      </button>
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
