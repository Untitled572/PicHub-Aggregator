<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status?: string
  available?: boolean
}>()

const isNormal = computed(() => {
  if (props.available === true) return true
  if (props.available === false) return false
  if (props.status === 'normal' || props.status === 'ok') return true
  if (!props.status) return true // Default fallback to normal green
  return false
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-semibold rounded-full transition-colors shrink-0"
    :class="{
      'bg-emerald-50 text-emerald-700 border border-emerald-300/80': isNormal,
      'bg-amber-50 text-amber-700 border border-amber-300/80': status === 'warning',
      'bg-rose-50 text-rose-700 border border-rose-300/80': status === 'error' || available === false,
      'bg-slate-100 text-slate-600 border border-slate-200': !isNormal && status !== 'warning' && status !== 'error',
    }"
  >
    <span
      class="w-1.5 h-1.5 rounded-full shrink-0"
      :class="{
        'bg-emerald-500': isNormal,
        'bg-amber-500': status === 'warning',
        'bg-rose-500': status === 'error' || available === false,
        'bg-slate-400': !isNormal && status !== 'warning' && status !== 'error',
      }"
    ></span>
    {{
      isNormal
        ? '正常运行'
        : status === 'warning'
        ? '延迟偏高'
        : status === 'error' || available === false
        ? '节点故障'
        : '未检测'
    }}
  </span>
</template>



