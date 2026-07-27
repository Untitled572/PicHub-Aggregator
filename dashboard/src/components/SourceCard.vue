<script setup lang="ts">
import { computed } from 'vue'
import type { Source } from '../types'
import HealthStatusBadge from './HealthStatusBadge.vue'
import { Edit3, Trash2, Globe, Activity, Scale, Code } from 'lucide-vue-next'
import { useTags } from '../composables/useTags'


defineProps<{ source: Source }>()
defineEmits<{ edit: [], delete: [], toggle: [] }>()

const { getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())


const respTypeMap: Record<string, { label: string; class: string }> = {
  json: { label: 'JSON 提取', class: 'bg-morandi-ocean-light text-morandi-ocean-dark border-morandi-ocean/20' },
  image: { label: '图片直链', class: 'bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/20' },
  redirect: { label: '302 重定向', class: 'bg-morandi-sand-light text-morandi-sand-dark border-morandi-sand/30' },
}
</script>

<template>
  <div class="morandi-card p-4 sm:p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
    <!-- Left Info Section -->
    <div class="flex-1 min-w-0 space-y-2">
      <div class="flex items-center gap-2.5 flex-wrap">
        <h3 class="font-semibold text-sm sm:text-base text-morandi-text truncate">{{ source.name }}</h3>
        <HealthStatusBadge :status="source.status" />

        <span
          class="text-[11px] px-2 py-0.5 rounded-full border font-medium"
          :class="respTypeMap[source.resp_type]?.class || 'bg-morandi-sidebar text-morandi-muted border-morandi-borderSoft'"
        >
          {{ respTypeMap[source.resp_type]?.label || source.resp_type }}
        </span>
      </div>

      <!-- URL display -->
      <div class="flex items-center gap-1.5 text-xs text-morandi-muted truncate font-mono bg-morandi-bg/80 px-2.5 py-1 rounded-lg border border-morandi-borderSoft/60 max-w-xl">
        <Globe class="w-3.5 h-3.5 text-morandi-light shrink-0" />
        <span class="truncate">{{ source.url }}</span>
      </div>

      <!-- Category Badges & JSON Path -->
      <div class="flex items-center gap-1.5 flex-wrap pt-0.5">
        <span
          v-for="cat in source.categories"
          :key="cat"
          class="text-[11px] px-2 py-0.5 bg-morandi-lavender-light text-morandi-lavender-dark rounded-md font-medium"
        >
          #{{ categoryMap[cat] || cat }}
        </span>

        <span v-if="source.json_path" class="text-[11px] px-2 py-0.5 bg-morandi-sidebar text-morandi-muted rounded-md font-mono flex items-center gap-1">
          <Code class="w-3 h-3 text-morandi-light" />
          {{ source.json_path }}
        </span>
      </div>
    </div>

    <!-- Right Metrics & Actions -->
    <div class="flex items-center justify-between sm:justify-end w-full sm:w-auto gap-4 pt-3 sm:pt-0 border-t sm:border-t-0 border-morandi-border/40 shrink-0">
      <!-- Weight & Latency Stats -->
      <div class="flex items-center gap-3 text-xs text-morandi-muted border-r border-morandi-border/60 pr-4">
        <div class="text-right">
          <div class="flex items-center gap-1 text-[11px] text-morandi-light justify-end">
            <Scale class="w-3 h-3" /> 权重
          </div>
          <div class="font-bold text-morandi-text text-sm mt-0.5">{{ source.weight }}</div>
        </div>

        <div class="text-right">
          <div class="flex items-center gap-1 text-[11px] text-morandi-light justify-end">
            <Activity class="w-3 h-3" /> 延迟
          </div>
          <div class="font-bold text-morandi-text text-sm mt-0.5 font-mono">{{ source.avg_latency || 0 }}<span class="text-[10px] font-normal text-morandi-muted">ms</span></div>
        </div>
      </div>

      <!-- Toggle Switch & Action Buttons -->
      <div class="flex items-center gap-3">
        <!-- Switch Toggle -->
        <label class="relative inline-flex items-center cursor-pointer" title="启用/禁用图源">
          <input
            type="checkbox"
            :checked="source.enabled"
            @change="$emit('toggle')"
            class="sr-only peer"
          />
          <div class="w-9 h-5 bg-morandi-sidebar peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-morandi-borderSoft after:border after:rounded-full after:h-4 after:w-4 after:transition-all duration-200 peer-checked:bg-morandi-sage"></div>
        </label>

        <!-- Edit Button -->
        <button
          @click="$emit('edit')"
          class="p-2 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors"
          title="编辑图源"
        >
          <Edit3 class="w-4 h-4" />
        </button>

        <!-- Delete Button -->
        <button
          @click="$emit('delete')"
          class="p-2 text-morandi-muted hover:text-morandi-rose-dark hover:bg-morandi-rose-light/60 rounded-lg transition-colors"
          title="删除图源"
        >
          <Trash2 class="w-4 h-4" />
        </button>
      </div>
    </div>
  </div>
</template>



