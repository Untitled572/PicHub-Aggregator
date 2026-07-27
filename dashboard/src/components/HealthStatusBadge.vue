<script setup lang="ts">
defineProps<{
  status?: string
  available?: boolean
}>()
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-medium rounded-full transition-colors shrink-0"
    :class="{
      'bg-emerald-50 text-emerald-700 border border-emerald-200': status === 'normal' || available === true,
      'bg-amber-50 text-amber-700 border border-amber-200': status === 'warning',
      'bg-rose-50 text-rose-700 border border-rose-200': status === 'error' || available === false,
      'bg-slate-100 text-slate-600 border border-slate-200': !status && available === undefined,
    }"
  >
    <span
      class="w-1.5 h-1.5 rounded-full shrink-0"
      :class="{
        'bg-emerald-500': status === 'normal' || available === true,
        'bg-amber-500': status === 'warning',
        'bg-rose-500': status === 'error' || available === false,
        'bg-slate-400': !status && available === undefined,
      }"
    ></span>
    {{
      status === 'normal' || available === true
        ? '正常运行'
        : status === 'warning'
        ? '延迟偏高'
        : status === 'error' || available === false
        ? '节点故障'
        : '未检测'
    }}
  </span>
</template>


