<script setup lang="ts">
import { computed } from 'vue'
import { healthBadge } from '../utils/healthStatus'
const props = withDefaults(defineProps<{ status?: string; available?: boolean; failCount?: number; enabled?: boolean; detail?: string }>(), { enabled: true, available: undefined })
const badge = computed(() => healthBadge(props))
const colors = {
 success: 'bg-emerald-50 text-emerald-700 border-emerald-300/80',
 warning: 'bg-amber-50 text-amber-700 border-amber-300/80',
 muted: 'bg-slate-100 text-slate-600 border-slate-200',
}
</script>
<template>
  <span class="inline-flex items-center gap-1.5 px-2.5 py-1 text-[11px] font-semibold rounded-full border shrink-0 whitespace-nowrap"
    :class="colors[badge.tone]" :title="detail || badge.detail">
    <span class="w-1.5 h-1.5 rounded-full shrink-0 bg-current"></span>
    {{ badge.label }}
  </span>
</template>
