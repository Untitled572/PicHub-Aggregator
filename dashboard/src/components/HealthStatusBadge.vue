<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status?: string
  available?: boolean
  failCount?: number
}>()

const isFailed = computed(() => {
  if (props.available === false) return true
  if (props.failCount !== undefined && props.failCount >= 5) return true
  if (props.status === 'error' && (props.failCount === undefined || props.failCount >= 5)) return true
  return false
})

const isNormal = computed(() => {
  if (isFailed.value) return false
  if (props.available === true) return true
  if (props.status === 'normal' || props.status === 'ok') return true
  if (props.failCount !== undefined && props.failCount < 5) return true
  if (!props.status || props.status === 'undefined') return true
  return true
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-semibold rounded-full transition-colors shrink-0"
    :class="{
      'bg-emerald-50 text-emerald-700 border border-emerald-300/80': isNormal,
      'bg-amber-50 text-amber-700 border border-amber-300/80': !isNormal && status === 'warning',
      'bg-rose-50 text-rose-700 border border-rose-300/80': isFailed,
      'bg-slate-100 text-slate-600 border border-slate-200': !isNormal && !isFailed && status !== 'warning',
    }"
  >
    <span
      class="w-1.5 h-1.5 rounded-full shrink-0"
      :class="{
        'bg-emerald-500': isNormal,
        'bg-amber-500': !isNormal && status === 'warning',
        'bg-rose-500': isFailed,
        'bg-slate-400': !isNormal && !isFailed && status !== 'warning',
      }"
    ></span>
    {{
      isNormal
        ? '正常运行'
        : status === 'warning'
        ? '延迟偏高'
        : isFailed
        ? '节点故障'
        : '未检测'
    }}
  </span>
</template>
