<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { Source } from '../types'
import SourceCard from '../components/SourceCard.vue'
import SourceForm from '../components/SourceForm.vue'
import ExportImportModal from '../components/ExportImportModal.vue'
import {
  Plus,
  ArrowUpDown,
  Search,
  Layers,
  CheckCircle2,
  Zap,
  Tag,
  Inbox
} from 'lucide-vue-next'

const { listSources, deleteSource, toggleSource } = useApi()
import ParamVariantsModal from '../components/ParamVariantsModal.vue'
import { useTags } from '../composables/useTags'

const sources = ref<Source[]>([])
const search = ref('')
const categoryFilter = ref('')
const showForm = ref(false)
const editingSource = ref<Source | undefined>()
const showExportImport = ref(false)
const showParamsModal = ref(false)
const paramsSource = ref<Source | null>(null)

function handleOpenParams(src: Source) {
  paramsSource.value = src
  showParamsModal.value = true
}


const { tags, getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())
const categories = computed(() => tags.value.map(t => t.id))
const healthStatusMap = ref<Record<number, boolean>>({})

onMounted(loadSources)

async function loadSources() {
  try {
    const [srcs, healthRes] = await Promise.all([
      listSources(),
      fetch('/api/health').then(r => r.ok ? r.json() : null).catch(() => null)
    ])
    const map: Record<number, boolean> = {}
    if (healthRes && healthRes.results) {
      for (const r of healthRes.results) {
        map[r.id] = r.available
      }
    }
    if (srcs) {
      for (const s of srcs) {
        if (map[s.id] === undefined) {
          map[s.id] = s.status !== 'error'
        }
      }
    }
    healthStatusMap.value = map
    sources.value = srcs || []
  } catch {}

}



const filteredSources = computed(() => {
  return sources.value.filter(s => {
    if (search.value && !s.name.toLowerCase().includes(search.value.toLowerCase()) && !s.url.toLowerCase().includes(search.value.toLowerCase())) {
      return false
    }
    if (categoryFilter.value && !s.categories.includes(categoryFilter.value)) {
      return false
    }
    return true
  })
})

const stats = computed(() => {
  const total = sources.value.length
  const enabled = sources.value.filter(s => s.enabled).length
  const avgLatency = total > 0 ? Math.round(sources.value.reduce((acc, s) => acc + (s.avg_latency || 0), 0) / total) : 0
  const avgSuccess = total > 0 ? Math.round(sources.value.reduce((acc, s) => acc + (s.success_rate || 0), 0) / total) : 100
  return { total, enabled, avgLatency, avgSuccess }
})

async function handleDelete(id: number) {
  try {
    await deleteSource(id)
    sources.value = sources.value.filter(s => s.id !== id)
    await loadSources()
  } catch {}
}


async function handleToggle(id: number) {

  await toggleSource(id)
  loadSources()
}

function editSource(src: Source) {
  editingSource.value = { ...src }
  showForm.value = true
}

function addSource() {
  editingSource.value = undefined
  showForm.value = true
}

function onFormSaved() {
  showForm.value = false
  loadSources()
}
</script>

<template>
  <div class="space-y-6">
    <!-- Stat Overview Banner -->
    <div class="grid grid-cols-2 md:grid-cols-4 gap-4">
      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0">
          <Layers class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">总接入源</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5">{{ stats.total }} <span class="text-xs font-normal text-morandi-muted">个 API</span></p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <CheckCircle2 class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">服务中源</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5">{{ stats.enabled }} <span class="text-xs font-normal text-morandi-muted">已启用</span></p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-sand/20 text-morandi-sand-dark flex items-center justify-center shrink-0">
          <Zap class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">平均延迟</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5">{{ stats.avgLatency }} <span class="text-xs font-normal text-morandi-muted">ms</span></p>
        </div>
      </div>

      <div class="morandi-card p-4 flex items-center gap-3">
        <div class="w-10 h-10 rounded-xl bg-morandi-rose/15 text-morandi-rose-dark flex items-center justify-center shrink-0">
          <Tag class="w-5 h-5" />
        </div>
        <div>
          <p class="text-xs text-morandi-muted font-medium">平均可用率</p>
          <p class="text-xl font-bold text-morandi-text mt-0.5">{{ stats.avgSuccess }}%</p>
        </div>
      </div>
    </div>

    <!-- Filter & Toolbar -->
    <div class="morandi-card p-4 flex flex-col md:flex-row items-center justify-between gap-4">
      <!-- Search & Category -->
      <div class="flex flex-col sm:flex-row items-center gap-3 w-full md:w-auto flex-1">
        <div class="relative w-full sm:w-64">
          <Search class="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-morandi-light" />
          <input
            v-model="search"
            placeholder="搜索图源名称或 URL..."
            class="morandi-input w-full pl-9 pr-3 py-2 text-xs"
          />
        </div>

        <div class="relative w-full sm:w-44">
          <select
            v-model="categoryFilter"
            class="morandi-input w-full px-3 py-2 text-xs appearance-none cursor-pointer pr-8"
          >
            <option value="">全部分类标签</option>
            <option v-for="cat in categories" :key="cat" :value="cat">
              {{ categoryMap[cat] || cat }}
            </option>
          </select>
          <div class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-morandi-muted text-xs">▼</div>
        </div>
      </div>

      <!-- Action Buttons -->
      <div class="flex items-center gap-2 w-full md:w-auto justify-end">
        <button
          @click="showExportImport = true"
          class="flex items-center gap-1.5 px-3.5 py-2 text-xs font-medium bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft transition-colors"
        >
          <ArrowUpDown class="w-3.5 h-3.5 text-morandi-muted" />
          导入 / 导出规则
        </button>
        <button
          @click="addSource"
          class="flex items-center gap-1.5 px-4 py-2 text-xs font-medium bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-sm transition-all duration-200 active:scale-95"
        >
          <Plus class="w-4 h-4" />
          添加新图源
        </button>
      </div>
    </div>

    <!-- Cards List -->
    <div v-if="filteredSources.length > 0" class="grid gap-4">
      <SourceCard
        v-for="src in filteredSources"
        :key="src.id"
        :source="src"
        :available="healthStatusMap[src.id]"
        @edit="editSource(src)"
        @delete="handleDelete(src.id)"
        @toggle="handleToggle(src.id)"
        @open-params="handleOpenParams"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="morandi-card py-16 text-center space-y-3">
      <div class="w-12 h-12 rounded-full bg-morandi-sidebar mx-auto flex items-center justify-center text-morandi-light">
        <Inbox class="w-6 h-6" />
      </div>
      <div>
        <p class="text-sm font-medium text-morandi-text">未找到符合条件的图源 API</p>
        <p class="text-xs text-morandi-muted mt-1">您可以点击右上角“添加新图源”开始配置</p>
      </div>
      <button
        @click="addSource"
        class="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-medium bg-morandi-sage text-white rounded-xl shadow-sm hover:bg-morandi-sage-dark transition-colors"
      >
        <Plus class="w-3.5 h-3.5" /> 立即添加图源
      </button>
    </div>




    <!-- Modals -->
    <SourceForm
      v-if="showForm"
      :source="editingSource"
      @saved="onFormSaved"
      @close="showForm = false"
    />

    <ParamVariantsModal
      v-if="showParamsModal && paramsSource"
      :source="paramsSource"
      @saved="loadSources"
      @close="showParamsModal = false"
    />

    <ExportImportModal
      v-if="showExportImport"
      @imported="loadSources"
      @close="showExportImport = false"
    />
  </div>
</template>
