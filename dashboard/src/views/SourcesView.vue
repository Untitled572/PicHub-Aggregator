<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import { isHealthResultCurrent } from '../utils/healthStatus'
import type { Source, HealthResult, SourceDebugState } from '../types'
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
  Inbox,
  Bug,
  ExternalLink,
  X
} from 'lucide-vue-next'

const { listSources, deleteSource, toggleSource, getHealthStatus, getSourceDebug, enableSourceDebug, disableSourceDebug } = useApi()
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
const sourceDebug = ref<SourceDebugState>({ enabled: false })
const debugLoading = ref(false)
const debugStateLoading = ref(true)
const debugError = ref('')
const debugSource = computed(() => sources.value.find(s => s.id === sourceDebug.value.source_id))

function handleOpenParams(src: Source) {
  paramsSource.value = src
  showParamsModal.value = true
}


const { tags, getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())
const categories = computed(() => tags.value.map(t => t.id))
const healthStatusMap = ref<Record<number, HealthResult>>({})

onMounted(() => {
  loadSources()
  loadSourceDebug()
})

async function loadSourceDebug() {
  debugStateLoading.value = true
  debugError.value = ''
  try {
    sourceDebug.value = await getSourceDebug()
  } catch (e: any) {
    debugError.value = `读取调试状态失败：${e.message || '请求失败'}`
  } finally {
    debugStateLoading.value = false
  }
}

async function handleDebug(src: Source) {
  debugLoading.value = true
  debugError.value = ''
  try {
    await enableSourceDebug(src.id)
    sourceDebug.value = { enabled: true, source_id: src.id, source_name: src.name }
  } catch (e: any) {
    debugError.value = `开启调试失败：${e.message || '请求失败'}`
  } finally {
    debugLoading.value = false
  }
}

async function handleDisableDebug() {
  debugLoading.value = true
  debugError.value = ''
  try {
    await disableSourceDebug()
    sourceDebug.value = { enabled: false }
  } catch (e: any) {
    debugError.value = `退出调试失败：${e.message || '请求失败'}`
  } finally {
    debugLoading.value = false
  }
}

function openDebugImage() {
  window.open('/random', '_blank', 'noopener,noreferrer')
}

async function loadSources() {
  try {
    const srcs = await listSources()
    sources.value = srcs || []
  } catch {}
  healthStatusMap.value = {}
  getHealthStatus()
    .catch(() => null)
    .then(healthRes => {
      if (!healthRes?.results) return
      const map: Record<number, HealthResult> = {}
      for (const result of healthRes.results as HealthResult[]) {
        const source = sources.value.find(s => s.id === result.id)
        if (source && isHealthResultCurrent(result.checked_at, source.updated_at)) map[result.id] = result
      }
      healthStatusMap.value = map
    })
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
    <div v-if="sourceDebug.enabled" class="rounded-2xl border border-amber-300 bg-amber-50 px-4 py-4 sm:px-5 shadow-sm">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div class="flex gap-3 min-w-0">
          <div class="w-9 h-9 rounded-xl bg-amber-200/70 text-amber-800 flex items-center justify-center shrink-0"><Bug class="w-4 h-4" /></div>
          <div class="min-w-0">
            <p class="text-sm font-bold text-amber-950">全局图源调试已开启：{{ debugSource?.name || sourceDebug.source_name || `图源 #${sourceDebug.source_id ?? '未知'}` }}</p>
            <p class="text-xs text-amber-900/80 mt-1">/random 和 /e 将统一使用此图源的主图地址与默认参数；分类与方向参数会被忽略。预取池补充已暂停，不读取已有缓存，也不写入新缓存或正常统计。退出或服务重启后恢复普通分发。</p>
            <p v-if="debugSource?.url" class="mt-2 text-xs font-mono text-amber-950 break-all">主图地址：{{ debugSource.url }}</p>
          </div>
        </div>
        <div class="flex items-center gap-2 sm:shrink-0 pl-12 sm:pl-0">
          <button @click="openDebugImage" class="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold bg-white hover:bg-amber-100 text-amber-950 rounded-xl border border-amber-300 transition-colors cursor-pointer">
            <ExternalLink class="w-3.5 h-3.5" /> 调试取图
          </button>
          <button @click="handleDisableDebug" :disabled="debugLoading" class="inline-flex items-center gap-1.5 px-3 py-2 text-xs font-semibold bg-amber-700 hover:bg-amber-800 disabled:opacity-50 text-white rounded-xl transition-colors cursor-pointer">
            <X class="w-3.5 h-3.5" /> {{ debugLoading ? '处理中…' : '退出调试' }}
          </button>
        </div>
      </div>
    </div>
    <div v-else-if="debugError" role="alert" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-800">{{ debugError }}</div>
    <div v-if="sourceDebug.enabled && debugError" role="alert" class="rounded-xl border border-rose-200 bg-rose-50 px-4 py-3 text-xs text-rose-800">{{ debugError }}</div>
    <div v-if="debugStateLoading" class="text-xs text-morandi-muted" role="status">正在读取调试状态…</div>

    <!-- Stat Overview Banner -->
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3.5">
      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0">
          <Layers class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium">图源总数</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5">{{ stats.total }} <span class="text-[10px] sm:text-xs font-normal text-morandi-muted">个 API</span></p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-ocean/15 text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <CheckCircle2 class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium">已启用图源</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5">{{ stats.enabled }} <span class="text-[10px] sm:text-xs font-normal text-morandi-muted">已启用</span></p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-sand/20 text-morandi-sand-dark flex items-center justify-center shrink-0">
          <Zap class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium">平均延迟</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5">{{ stats.avgLatency }} <span class="text-[10px] sm:text-xs font-normal text-morandi-muted">ms</span></p>
        </div>
      </div>

      <div class="morandi-card p-3.5 sm:p-4 flex items-center gap-3">
        <div class="w-9 h-9 sm:w-10 sm:h-10 rounded-xl bg-morandi-rose/15 text-morandi-rose-dark flex items-center justify-center shrink-0">
          <Tag class="w-4 h-4 sm:w-5 sm:h-5" />
        </div>
        <div>
          <p class="text-[11px] sm:text-xs text-morandi-muted font-medium">近期平均成功率</p>
          <p class="text-lg sm:text-xl font-bold text-morandi-text mt-0.5">{{ stats.avgSuccess }}%</p>
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
      <div class="flex items-center gap-2 w-full sm:w-auto justify-end flex-wrap sm:flex-nowrap">
        <button
          @click="showExportImport = true"
          class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-3.5 py-2 text-xs font-medium bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft transition-colors cursor-pointer"
        >
          <ArrowUpDown class="w-3.5 h-3.5 text-morandi-muted" />
          导入 / 导出规则
        </button>
        <button
          @click="addSource"
          class="flex-1 sm:flex-initial flex items-center justify-center gap-1.5 px-4 py-2 text-xs font-medium bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-sm transition-all duration-200 active:scale-95 cursor-pointer"
        >
          <Plus class="w-4 h-4" />
          添加图源
        </button>
      </div>
    </div>


    <!-- Cards List -->
    <div v-if="filteredSources.length > 0" class="grid gap-4">
      <SourceCard
        v-for="src in filteredSources"
        :key="src.id"
        :source="src"
        :debug-selected="sourceDebug.enabled && sourceDebug.source_id === src.id"
        :debug-loading="debugLoading || debugStateLoading"
        :available="healthStatusMap[src.id]?.available"
        :health-detail="healthStatusMap[src.id] ? '检测时间：' + new Date(healthStatusMap[src.id].checked_at!).toLocaleString() + (healthStatusMap[src.id].error ? '；' + healthStatusMap[src.id].error : '；本次响应检测通过') : undefined"
        @edit="editSource(src)"
        @delete="handleDelete(src.id)"
        @toggle="handleToggle(src.id)"
        @open-params="handleOpenParams"
        @debug="handleDebug"
      />
    </div>

    <!-- Empty State -->
    <div v-else class="morandi-card py-16 text-center space-y-3">
      <div class="w-12 h-12 rounded-full bg-morandi-sidebar mx-auto flex items-center justify-center text-morandi-light">
        <Inbox class="w-6 h-6" />
      </div>
      <div>
        <p class="text-sm font-medium text-morandi-text">未找到符合条件的图源 API</p>
        <p class="text-xs text-morandi-muted mt-1">您可以点击右上角“添加图源”开始配置</p>
      </div>
      <button
        @click="addSource"
        class="inline-flex items-center gap-1.5 px-3.5 py-1.5 text-xs font-medium bg-morandi-sage text-white rounded-xl shadow-sm hover:bg-morandi-sage-dark transition-colors"
      >
        <Plus class="w-3.5 h-3.5" /> 添加图源
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
