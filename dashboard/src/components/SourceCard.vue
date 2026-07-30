<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Source } from '../types'
import HealthStatusBadge from './HealthStatusBadge.vue'
import { Edit3, Trash2, Globe, Sliders } from 'lucide-vue-next'
import { useTags } from '../composables/useTags'

defineProps<{ source: Source; available?: boolean }>()
const emit = defineEmits<{ edit: [], delete: [], toggle: [], openParams: [source: Source] }>()


const confirmingDelete = ref(false)
const { getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())

const respTypeMap: Record<string, { label: string; class: string }> = {
  json: { label: 'JSON 提取', class: 'bg-morandi-ocean-light text-morandi-ocean-dark border-morandi-ocean/20' },
  image: { label: '图片直链', class: 'bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/20' },
  redirect: { label: '302 重定向', class: 'bg-morandi-sand-light text-morandi-sand-dark border-morandi-sand/20' },
}

function handleDeleteConfirm() {
  confirmingDelete.value = false
  emit('delete')
}
</script>

<template>
  <div class="morandi-card p-4 sm:p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
    <!-- Left Info Section -->
    <div class="flex-1 min-w-0 space-y-2">
      <div class="flex items-center gap-2.5 flex-wrap">
        <h3 class="font-semibold text-sm sm:text-base text-morandi-text truncate">{{ source.name }}</h3>
        <HealthStatusBadge :status="source.status" :available="available" :fail-count="source.fail_count" />

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

      <!-- Default Query Params -->
      <div v-if="source.default_query" class="flex items-center gap-1.5">
        <span class="text-[10px] px-2 py-0.5 bg-morandi-sand-light/80 text-morandi-sand-dark rounded-md font-medium border border-morandi-sand/20 font-mono">
          默认参数: {{ source.default_query }}
        </span>
      </div>

      <!-- Category Tags -->
      <div v-if="(source.categories && source.categories.length > 0) || (source.params && source.params.length > 0)" class="flex items-center gap-1.5 flex-wrap pt-0.5">
        <span
          v-for="cat in source.categories"
          :key="cat"
          class="text-[10px] px-2 py-0.5 bg-morandi-sage-light/70 text-morandi-sage-dark rounded-md font-medium border border-morandi-sage/20"
        >
          #{{ categoryMap[cat] || cat }}
        </span>

        <span v-if="source.params && source.params.length > 0" class="text-[10px] px-2 py-0.5 bg-morandi-ocean-light/80 text-morandi-ocean-dark rounded-md font-bold border border-morandi-ocean/20">
          ⚡ {{ source.params.length }} 个子分支/API 链接
        </span>
        <span
          v-for="p in (source.params || []).slice(0, 4)"
          :key="p.key + '=' + p.value"
          class="text-[10px] font-mono bg-morandi-bg px-1.5 py-0.5 rounded border border-morandi-borderSoft/60 text-morandi-muted"
        >
          <template v-if="p.key.startsWith('/') || p.key.startsWith('http')">
            🔗 {{ p.value ? p.value + ' (' + p.key + ')' : p.key }}
          </template>
          <template v-else>
            {{ p.key }}{{ p.value ? '=' + p.value : '' }}
          </template>
        </span>

      </div>
    </div>


    <!-- Right Metrics & Actions -->
    <div class="flex items-center gap-4 sm:gap-6 shrink-0 w-full sm:w-auto justify-between sm:justify-end border-t sm:border-t-0 pt-3 sm:pt-0 border-morandi-borderSoft/60">
      <!-- Latency & Success rate metrics -->
      <div class="flex items-center gap-4 text-xs">
        <div class="text-right">
          <div class="text-[10px] text-morandi-muted">权重</div>
          <div class="font-bold text-morandi-text text-sm mt-0.5 font-mono">{{ source.weight }}</div>
        </div>

        <div class="text-right">
          <div class="text-[10px] text-morandi-muted">成功率</div>
          <div class="font-bold text-morandi-text text-sm mt-0.5 font-mono">
            {{ Math.round(source.success_rate || 0) }}%
          </div>
        </div>

        <div class="text-right">
          <div class="text-[10px] text-morandi-muted">平均延迟</div>
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

        <!-- Manage Parameter Variants Button -->
        <button
          @click="$emit('openParams', source)"
          class="flex items-center gap-1 px-2 py-1 text-xs text-morandi-ocean-dark bg-morandi-ocean-light/80 hover:bg-morandi-ocean-light rounded-lg border border-morandi-ocean/20 transition-colors font-medium cursor-pointer"
          title="管理多参数与路径分支"
        >
          <Sliders class="w-3.5 h-3.5" />
          <span>分支 ({{ source.params?.length || 0 }})</span>
        </button>

        <!-- Edit Button -->
        <button
          @click="$emit('edit')"
          class="p-2 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors cursor-pointer"
          title="编辑图源"
        >

          <Edit3 class="w-4 h-4" />
        </button>

        <!-- Inline Delete Confirmation -->
        <div v-if="confirmingDelete" class="flex items-center gap-2 bg-rose-50 px-3 py-1.5 rounded-xl border border-rose-200 animate-in fade-in zoom-in-95 duration-150">
          <span class="text-xs font-bold text-rose-700">确认删除?</span>
          <button
            @click="handleDeleteConfirm"
            class="px-2.5 py-1 bg-rose-600 hover:bg-rose-700 text-white font-bold text-xs rounded-lg transition-colors cursor-pointer"
          >
            删除
          </button>
          <button
            @click="confirmingDelete = false"
            class="px-2.5 py-1 bg-white hover:bg-slate-100 text-slate-600 font-medium text-xs rounded-lg border border-slate-200 transition-colors cursor-pointer"
          >
            取消
          </button>
        </div>


        <button
          v-else
          @click="confirmingDelete = true"
          class="p-2 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors cursor-pointer"
          title="删除图源"
        >
          <Trash2 class="w-4 h-4" />
        </button>
      </div>
    </div>
  </div>
</template>
