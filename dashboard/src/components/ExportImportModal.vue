<script setup lang="ts">
import { ref, computed } from 'vue'
import { useApi } from '../composables/useApi'
import {
  X,
  Download,
  Upload,
  Copy,
  Check,
  Package,
  FileText,
  Archive,
  Layers,
  BarChart3,
  Heart,
  CheckSquare,
  Square,
  AlertCircle
} from 'lucide-vue-next'

const emit = defineEmits<{ close: [] }>()
const { exportCustomData, importCustomData, exportRules } = useApi()

const tab = ref<'export' | 'import'>('export')
const loading = ref(false)
const copied = ref(false)

// Export Scope Checkboxes State
const exportScope = ref({
  config: true,
  stats: true,
  images: true,
})

// Import State
const importMode = ref<'file' | 'text'>('file')
const selectedFile = ref<File | null>(null)
const importJsonText = ref('')
const importResult = ref<{
  success: boolean
  message: string
  sources?: number
  stats?: number
  images?: number
} | null>(null)

const isZipExport = computed(() => exportScope.value.images)

function toggleScope(key: 'config' | 'stats' | 'images') {
  exportScope.value[key] = !exportScope.value[key]
}

function handleExport() {
  const selectedScopes: string[] = []
  if (exportScope.value.config) selectedScopes.push('config')
  if (exportScope.value.stats) selectedScopes.push('stats')
  if (exportScope.value.images) selectedScopes.push('images')

  if (selectedScopes.length === 0) return

  exportCustomData(selectedScopes)
}

const exportedJsonText = ref('')
async function generateJsonCode() {
  loading.value = true
  try {
    const data = await exportRules()
    exportedJsonText.value = JSON.stringify(data, null, 2)
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

function copyJsonCode() {
  if (!exportedJsonText.value) return
  navigator.clipboard.writeText(exportedJsonText.value)
  copied.value = true
  setTimeout(() => (copied.value = false), 2000)
}

function onFileSelected(e: Event) {
  const input = e.target as HTMLInputElement
  if (input.files && input.files[0]) {
    selectedFile.value = input.files[0]
  }
}

async function handleImport() {
  loading.value = true
  importResult.value = null
  try {
    let payload: FormData | object

    if (importMode.value === 'file' && selectedFile.value) {
      const formData = new FormData()
      formData.append('file', selectedFile.value)
      payload = formData
    } else if (importMode.value === 'text' && importJsonText.value.trim()) {
      payload = JSON.parse(importJsonText.value)
    } else {
      loading.value = false
      return
    }

    const res = await importCustomData(payload)
    importResult.value = {
      success: true,
      message: res.message || '数据导入恢复成功！',
      sources: res.imported_sources || 0,
      stats: res.imported_stats || 0,
      images: res.imported_images || 0,
    }
  } catch (e: any) {
    importResult.value = {
      success: false,
      message: e.message || 'JSON / ZIP 备份文件解析失败，请检查格式是否合法',
    }
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div
    class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-950/60 backdrop-blur-md transition-all animate-in fade-in duration-200"
    @click.self="emit('close')"
  >
    <div class="bg-white rounded-2xl shadow-2xl w-full max-w-xl border border-morandi-borderSoft overflow-hidden flex flex-col max-h-[90vh]">
      <!-- Modal Header -->
      <div class="p-5 border-b border-morandi-border/60 flex justify-between items-center bg-morandi-bg/50">
        <div class="flex items-center gap-3">
          <div class="w-9 h-9 rounded-xl bg-morandi-sage-light text-morandi-sage-dark flex items-center justify-center shrink-0 shadow-xs">
            <Archive class="w-5 h-5" />
          </div>
          <div>
            <h2 class="font-bold text-base text-morandi-text">数据导入 / 导出中心</h2>
            <p class="text-xs text-morandi-muted mt-0.5">自选范围备份 API 规则、统计日志与离线图片物理文件</p>
          </div>
        </div>
        <button
          @click="emit('close')"
          class="p-1.5 text-morandi-light hover:text-morandi-text hover:bg-morandi-hover rounded-xl transition-colors cursor-pointer"
        >
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Navigation Tabs -->
      <div class="flex border-b border-morandi-border/60 bg-morandi-bg/20 text-xs font-semibold">
        <button
          @click="tab = 'export'"
          class="flex-1 py-3 flex items-center justify-center gap-2 border-b-2 transition-all cursor-pointer"
          :class="tab === 'export'
            ? 'text-morandi-sage-dark border-morandi-sage bg-white'
            : 'text-morandi-muted border-transparent hover:text-morandi-text'"
        >
          <Download class="w-4 h-4" /> 自选范围数据导出
        </button>
        <button
          @click="tab = 'import'"
          class="flex-1 py-3 flex items-center justify-center gap-2 border-b-2 transition-all cursor-pointer"
          :class="tab === 'import'
            ? 'text-morandi-sage-dark border-morandi-sage bg-white'
            : 'text-morandi-muted border-transparent hover:text-morandi-text'"
        >
          <Upload class="w-4 h-4" /> 备份恢复与导入
        </button>
      </div>

      <!-- Modal Body Content -->
      <div class="p-6 text-xs overflow-y-auto space-y-4">
        <!-- 1. EXPORT TAB -->
        <div v-if="tab === 'export'" class="space-y-4">
          <p class="text-morandi-muted font-medium">请勾选需要导出的数据模块范围：</p>

          <!-- Scope Selection Checkboxes Grid -->
          <div class="grid grid-cols-1 gap-2.5">
            <!-- Scope 1: Config & Sources -->
            <div
              @click="toggleScope('config')"
              class="p-3.5 rounded-xl border transition-all cursor-pointer flex items-start gap-3"
              :class="exportScope.config
                ? 'bg-morandi-sage-light/40 border-morandi-sage/50 text-morandi-text shadow-2xs'
                : 'bg-morandi-bg/40 border-morandi-borderSoft text-morandi-muted hover:border-morandi-border'"
            >
              <component :is="exportScope.config ? CheckSquare : Square" class="w-4 h-4 text-morandi-sage mt-0.5 shrink-0" />
              <div class="space-y-0.5">
                <div class="font-bold flex items-center gap-1.5 text-morandi-text">
                  <Layers class="w-3.5 h-3.5 text-morandi-sage" />
                  <span>API 图源列表与系统设置 (Sources & Settings)</span>
                </div>
                <p class="text-[11px] text-morandi-muted leading-relaxed">
                  包含全量第三方图源、衍生参数分支、权重设置、分类 Tag 绑定关系及防刷安全等配置。
                </p>
              </div>
            </div>

            <!-- Scope 2: Stats & Logs -->
            <div
              @click="toggleScope('stats')"
              class="p-3.5 rounded-xl border transition-all cursor-pointer flex items-start gap-3"
              :class="exportScope.stats
                ? 'bg-morandi-ocean-light/40 border-morandi-ocean/50 text-morandi-text shadow-2xs'
                : 'bg-morandi-bg/40 border-morandi-borderSoft text-morandi-muted hover:border-morandi-border'"
            >
              <component :is="exportScope.stats ? CheckSquare : Square" class="w-4 h-4 text-morandi-ocean mt-0.5 shrink-0" />
              <div class="space-y-0.5">
                <div class="font-bold flex items-center gap-1.5 text-morandi-text">
                  <BarChart3 class="w-3.5 h-3.5 text-morandi-ocean" />
                  <span>历史统计与调取日志 (Usage Statistics & Logs)</span>
                </div>
                <p class="text-[11px] text-morandi-muted leading-relaxed">
                  包含每日请求 Hits 趋势、Tag 命中分布、图源热度排行榜及近期分发图片历史流水记录。
                </p>
              </div>
            </div>

            <!-- Scope 3: Saved Images & Binary Files -->
            <div
              @click="toggleScope('images')"
              class="p-3.5 rounded-xl border transition-all cursor-pointer flex items-start gap-3"
              :class="exportScope.images
                ? 'bg-rose-50/80 border-rose-200 text-morandi-text shadow-2xs'
                : 'bg-morandi-bg/40 border-morandi-borderSoft text-morandi-muted hover:border-morandi-border'"
            >
              <component :is="exportScope.images ? CheckSquare : Square" class="w-4 h-4 text-rose-500 mt-0.5 shrink-0" />
              <div class="space-y-0.5">
                <div class="font-bold flex items-center gap-1.5 text-morandi-text">
                  <Heart class="w-3.5 h-3.5 text-rose-500 fill-rose-500" />
                  <span>离线保存图片与物理文件 (Saved Images & Files)</span>
                </div>
                <p class="text-[11px] text-morandi-muted leading-relaxed">
                  包含保存的离线偏好图片数据库元数据，并将存储在本地磁盘中的实际图片二进制文件打包导出。
                </p>
              </div>
            </div>
          </div>

          <!-- Format Hint Banner -->
          <div class="p-3 bg-morandi-bg rounded-xl border border-morandi-borderSoft text-[11px] text-morandi-muted flex items-center justify-between">
            <span class="flex items-center gap-1.5">
              <Package v-if="isZipExport" class="w-4 h-4 text-rose-500" />
              <FileText v-else class="w-4 h-4 text-morandi-sage" />
              <span>导出文件格式：<strong class="text-morandi-text font-mono">{{ isZipExport ? '.ZIP 压缩包 (内含二进制图片)' : '.JSON 配置文件' }}</strong></span>
            </span>
          </div>

          <!-- Export Action Buttons -->
          <div class="pt-2 flex items-center gap-3">
            <button
              @click="handleExport"
              :disabled="!exportScope.config && !exportScope.stats && !exportScope.images"
              class="flex-1 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl font-bold shadow-xs transition-all flex items-center justify-center gap-2 cursor-pointer disabled:opacity-40"
            >
              <Download class="w-4 h-4" />
              <span>导出选定备份文件 ({{ isZipExport ? 'ZIP' : 'JSON' }})</span>
            </button>

            <button
              v-if="!isZipExport"
              @click="generateJsonCode"
              :disabled="loading"
              class="px-4 py-2.5 bg-morandi-bg hover:bg-morandi-hover text-morandi-text rounded-xl font-semibold border border-morandi-borderSoft transition-all flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <FileText class="w-4 h-4" />
              <span>预览代码</span>
            </button>
          </div>

          <!-- JSON Code Preview Block -->
          <div v-if="exportedJsonText" class="space-y-2 pt-2 border-t border-morandi-border/40 animate-in fade-in duration-200">
            <div class="flex justify-between items-center text-xs">
              <span class="text-morandi-muted font-medium">JSON 规则代码预览：</span>
              <button @click="copyJsonCode" class="flex items-center gap-1 text-morandi-sage-dark hover:underline font-bold cursor-pointer">
                <component :is="copied ? Check : Copy" class="w-3.5 h-3.5 text-emerald-600" />
                <span>{{ copied ? '已复制到剪贴板！' : '一键复制 JSON' }}</span>
              </button>
            </div>
            <pre class="bg-stone-900 text-stone-100 p-3 rounded-xl border border-stone-800 font-mono text-[11px] max-h-48 overflow-auto leading-relaxed">{{ exportedJsonText }}</pre>
          </div>
        </div>

        <!-- 2. IMPORT TAB -->
        <div v-else class="space-y-4">
          <!-- Import Mode Switcher -->
          <div class="flex items-center gap-3 text-xs">
            <label class="flex items-center gap-1.5 cursor-pointer font-medium text-morandi-text">
              <input type="radio" value="file" v-model="importMode" class="text-morandi-sage focus:ring-morandi-sage" />
              <span>上传 ZIP / JSON 备份文件</span>
            </label>
            <label class="flex items-center gap-1.5 cursor-pointer font-medium text-morandi-text">
              <input type="radio" value="text" v-model="importMode" class="text-morandi-sage focus:ring-morandi-sage" />
              <span>粘贴 JSON 文本代码</span>
            </label>
          </div>

          <!-- File Upload Option -->
          <div v-if="importMode === 'file'" class="space-y-2">
            <label class="block p-6 border-2 border-dashed border-morandi-borderSoft hover:border-morandi-sage rounded-2xl text-center bg-morandi-bg/30 hover:bg-morandi-bg transition-colors cursor-pointer">
              <input type="file" accept=".zip,.json" @change="onFileSelected" class="hidden" />
              <div class="space-y-2">
                <Archive class="w-8 h-8 mx-auto text-morandi-sage" />
                <div class="text-xs font-bold text-morandi-text">
                  {{ selectedFile ? selectedFile.name : '点击选择或拖拽 ZIP 压缩包 / JSON 备份文件' }}
                </div>
                <p class="text-[11px] text-morandi-muted">
                  {{ selectedFile ? `文件大小: ${(selectedFile.size / 1024).toFixed(1)} KB` : '支持全量数据与物理离线图片解压恢复' }}
                </p>
              </div>
            </label>
          </div>

          <!-- Pure Text Option -->
          <div v-else class="space-y-2">
            <textarea
              v-model="importJsonText"
              rows="7"
              placeholder="请在此粘贴备份或导出的 JSON 文本代码..."
              class="morandi-input w-full p-3 font-mono text-xs leading-relaxed"
            ></textarea>
          </div>

          <!-- Submit Import Action -->
          <div class="pt-2 flex items-center justify-between gap-3">
            <button
              @click="handleImport"
              :disabled="loading || (importMode === 'file' && !selectedFile) || (importMode === 'text' && !importJsonText.trim())"
              class="px-6 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl font-bold shadow-xs transition-all flex items-center gap-2 cursor-pointer disabled:opacity-40"
            >
              <Upload class="w-4 h-4" />
              <span>{{ loading ? '恢复导入中...' : '确认导入备份数据' }}</span>
            </button>
          </div>

          <!-- Import Result Badge Alert -->
          <div
            v-if="importResult"
            class="p-4 rounded-xl text-xs flex items-start gap-3 border shadow-xs animate-in fade-in duration-200"
            :class="importResult.success ? 'bg-emerald-50 border-emerald-200 text-emerald-900' : 'bg-rose-50 border-rose-200 text-rose-900'"
          >
            <component :is="importResult.success ? Check : AlertCircle" class="w-4 h-4 shrink-0 mt-0.5" :class="importResult.success ? 'text-emerald-600' : 'text-rose-500'" />
            <div class="space-y-1">
              <p class="font-bold">{{ importResult.message }}</p>
              <p v-if="importResult.success" class="text-[11px] opacity-90">
                已成功导入：<strong class="font-mono">{{ importResult.sources }}</strong> 个图源，
                <strong class="font-mono">{{ importResult.stats }}</strong> 条历史日志，
                <strong class="font-mono">{{ importResult.images }}</strong> 张离线图片。
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
