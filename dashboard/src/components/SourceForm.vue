<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { Source, QueryParam } from '../types'
import { X, Plus, Trash2, Sliders, ChevronDown, Sparkles, Loader2, Check } from 'lucide-vue-next'

const props = defineProps<{
  source?: Source
  initialData?: Record<string, any>
}>()
const emit = defineEmits<{ saved: [], close: [] }>()

const { createSource, updateSource, detectURL } = useApi()

const saving = ref(false)
const testingUrl = ref(false)
const testDetected = ref<string | null>(null)
const userManuallySetType = ref(false)
const showAdvanced = ref(false)
const showHeaderInput = ref(false)

interface HeaderRow {
  key: string
  value: string
}

const headerRows = ref<HeaderRow[]>([])

const form = ref({
  name: '',
  url: '',
  resp_type: 'image',
  json_path: '',
  weight: 3,
  categories: [] as string[],
  headers: {} as Record<string, string>,
  params: [] as QueryParam[],
  default_query: '',
  enabled: true,
})


import { computed } from 'vue'
import { useTags } from '../composables/useTags'

const { tags, getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())
const categories = computed(() => tags.value.map(t => t.id))


const weightLabels: Record<number, string> = {
  1: '极低',
  2: '较低',
  3: '标准',
  4: '较高',
  5: '极高'
}

interface ParamRow {
  key: string
  value: string
  weight: number
  categories: string[]
}

const paramRows = ref<ParamRow[]>([])

function addParamRow() {
  paramRows.value.push({ key: '', value: '', weight: 3, categories: [] })
}

function removeParamRow(index: number) {
  paramRows.value.splice(index, 1)
}

function toggleParamCategory(paramRow: ParamRow, cat: string) {
  const idx = paramRow.categories.indexOf(cat)
  if (idx >= 0) paramRow.categories.splice(idx, 1)
  else paramRow.categories.push(cat)
}

function syncParamRowsToForm() {
  const params: QueryParam[] = []
  for (const row of paramRows.value) {
    if (row.key.trim() && row.value.trim()) {
      params.push({
        key: row.key.trim(),
        value: row.value.trim(),
        weight: row.weight || 3,
        categories: [...row.categories]
      })
    }
  }
  form.value.params = params
}

onMounted(() => {
  if (props.source) {
    form.value = {
      ...props.source,
      headers: { ...props.source.headers },
      params: props.source.params ? [...props.source.params] : []
    }
    showAdvanced.value = true
  } else if (props.initialData) {

    Object.assign(form.value, props.initialData)
  }

  // Initialize header rows array
  headerRows.value = []
  if (form.value.headers && Object.keys(form.value.headers).length > 0) {
    showHeaderInput.value = true
    for (const [k, v] of Object.entries(form.value.headers)) {
      headerRows.value.push({ key: k, value: v })
    }
  }

  // Initialize param rows array
  paramRows.value = []
  if (form.value.params && form.value.params.length > 0) {
    for (const p of form.value.params) {
      paramRows.value.push({
        key: p.key,
        value: p.value,
        weight: p.weight || 3,
        categories: [...(p.categories || [])]
      })
    }
  }
})


function handleUrlInput() {

  if (userManuallySetType.value) return
  const val = form.value.url.trim().toLowerCase()
  if (!val) return

  if (val.includes('.json') || val.includes('format=json') || val.includes('type=json') || val.includes('json=1')) {
    form.value.resp_type = 'json'
    if (!form.value.json_path) form.value.json_path = 'url'
  } else if (
    val.endsWith('.jpg') ||
    val.endsWith('.jpeg') ||
    val.endsWith('.png') ||
    val.endsWith('.webp') ||
    val.endsWith('.gif') ||
    val.endsWith('.svg')
  ) {
    form.value.resp_type = 'image'
  } else {
    // PHP, ASP, API endpoints (e.g. index.php, picsum.photos, xl0408.top) perform 302 redirect
    form.value.resp_type = 'redirect'
  }
}

function selectRespType(type: 'image' | 'redirect' | 'json') {
  form.value.resp_type = type
  userManuallySetType.value = true
}

async function autoDetectUrlType() {
  if (!form.value.url.trim()) return
  testingUrl.value = true
  testDetected.value = null
  try {
    const res = await detectURL(form.value.url.trim())
    if (res && res.resp_type && res.resp_type !== 'unknown') {
      form.value.resp_type = res.resp_type as any
      if (res.resp_type === 'json' && res.url_hints && res.url_hints.length > 0) {
        form.value.json_path = res.url_hints[0]
      }
      testDetected.value = res.resp_type === 'image' ? '图片二进制流' : res.resp_type === 'json' ? 'JSON 节点提取' : '302 重定向直链'
    } else {
      handleUrlInput()
      testDetected.value = form.value.resp_type === 'image' ? '图片直链' : form.value.resp_type === 'json' ? 'JSON提取' : '302重定向'
    }
  } catch {
    handleUrlInput()
    testDetected.value = form.value.resp_type === 'image' ? '图片直链' : form.value.resp_type === 'json' ? 'JSON提取' : '302重定向'
  } finally {
    testingUrl.value = false
  }
}


function parseFallbackName(rawUrl: string): string {
  try {
    const u = new URL(rawUrl)
    return u.hostname + ' 图源'
  } catch {
    return '自定义 API 图源'
  }
}

function startHeaderInput() {
  showHeaderInput.value = true
  if (headerRows.value.length === 0) {
    headerRows.value.push({ key: '', value: '' })
  }
}

function addHeaderRow() {
  headerRows.value.push({ key: '', value: '' })
}

function removeHeaderRow(index: number) {
  headerRows.value.splice(index, 1)
  if (headerRows.value.length === 0) {
    showHeaderInput.value = false
  }
}

function syncHeaderRowsToForm() {
  const headersObj: Record<string, string> = {}
  for (const row of headerRows.value) {
    if (row.key.trim()) {
      headersObj[row.key.trim()] = row.value.trim()
    }
  }
  form.value.headers = headersObj
}

function toggleCategory(cat: string) {
  const idx = form.value.categories.indexOf(cat)
  if (idx >= 0) form.value.categories.splice(idx, 1)
  else form.value.categories.push(cat)
}

async function handleSave() {
  if (!form.value.url.trim()) return
  saving.value = true
  try {
    syncHeaderRowsToForm()
    syncParamRowsToForm()
    const finalForm = { ...form.value }

    if (!finalForm.name.trim()) {
      finalForm.name = parseFallbackName(finalForm.url)
    }

    if (props.source) {
      await updateSource(props.source.id, finalForm)
    } else {
      await createSource(finalForm)
    }
    emit('saved')
  } catch {}
  saving.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-900/40 backdrop-blur-md transition-all" @click.self="emit('close')">
    <div class="bg-white rounded-2xl shadow-morandi-lg w-full max-w-lg border border-morandi-borderSoft max-h-[90vh] flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-200">
      <!-- Modal Header -->
      <div class="p-5 border-b border-morandi-border/60 flex justify-between items-center bg-morandi-bg/50">
        <div>
          <h2 class="font-bold text-base text-morandi-text">{{ source ? '编辑 API 图源' : '添加 API 图源' }}</h2>
          <p class="text-xs text-morandi-muted mt-0.5">支持简易一键添加，自动检测响应格式</p>
        </div>
        <button @click="emit('close')" class="p-1 text-morandi-light hover:text-morandi-text hover:bg-morandi-hover rounded-lg transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Modal Body (Easy Mode by default) -->
      <div class="p-5 space-y-4 overflow-y-auto flex-1 text-xs">
        <!-- 1. API URL (Required) -->
        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="font-medium text-morandi-text flex items-center gap-1">
              API 请求地址 (URL) <span class="text-morandi-rose">*</span>
            </label>
            <button
              type="button"
              @click="autoDetectUrlType"
              :disabled="testingUrl || !form.url"
              class="text-[11px] text-morandi-sage-dark hover:underline flex items-center gap-1 disabled:opacity-40 font-medium"
            >
              <Loader2 v-if="testingUrl" class="w-3 h-3 animate-spin" />
              <Sparkles v-else class="w-3 h-3 text-morandi-sage" />
              <span>自动检测类型</span>
            </button>
          </div>
          <input
            v-model="form.url"
            placeholder="https://api.example.com/v1/photos/random"
            class="morandi-input w-full px-3 py-2 font-mono text-xs"
          />

          <!-- Auto-detected status pill (Only displays after manually clicking 自动检测类型) -->
          <div v-if="testDetected" class="mt-1.5 flex items-center gap-2 text-[11px] animate-in fade-in duration-200">
            <span class="text-morandi-muted">自动检测结果：</span>
            <span class="px-2.5 py-0.5 bg-morandi-sage-light text-morandi-sage-dark rounded-md font-bold border border-morandi-sage/20 flex items-center gap-1">
              <Check class="w-3.5 h-3.5 text-morandi-sage font-bold" />
              {{ testDetected }}
            </span>
          </div>
        </div>

        <!-- 2. Source Name (Optional) -->
        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="font-medium text-morandi-text">图源名称 <span class="text-morandi-muted font-normal">(选填)</span></label>
            <span class="text-[10px] text-morandi-muted">留空自动解析 Host 域名</span>
          </div>
          <input
            v-model="form.name"
            placeholder="例如：Unsplash 高清风景源 (可不填)"
            class="morandi-input w-full px-3 py-2 text-xs"
          />
        </div>

        <!-- 3. Advanced Toggle Button -->
        <div class="pt-1">
          <button
            type="button"
            @click="showAdvanced = !showAdvanced"
            class="w-full py-2.5 px-3.5 bg-morandi-bg/80 hover:bg-morandi-hover rounded-xl border border-morandi-borderSoft/80 flex items-center justify-between text-morandi-muted hover:text-morandi-text transition-all font-medium text-xs group"
          >
            <span class="flex items-center gap-2">
              <Sliders class="w-3.5 h-3.5 text-morandi-sage" />
              <span>{{ showAdvanced ? '折叠高级参数设置' : '展开高级选项 (分类Tags、响应类型、权重、参数分支、请求头)' }}</span>
            </span>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': showAdvanced }" />
          </button>
        </div>

        <!-- 4. Advanced Section (Collapsible) -->
        <div v-if="showAdvanced" class="space-y-4 pt-2 border-t border-morandi-border/40 animate-in fade-in duration-200">
          <!-- Main Categories Tag Selector (Inside Advanced Settings) -->
          <div>
            <label class="font-medium text-morandi-text block mb-1.5 flex items-center justify-between">
              <span>关联分类标签 (Tags)</span>
              <span class="text-[10px] text-morandi-muted font-normal">点击选择/取消关联 Tag</span>
            </label>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="cat in categories"
                :key="cat"
                type="button"
                @click="toggleCategory(cat)"
                class="px-2.5 py-1 rounded-lg font-medium text-xs border transition-all cursor-pointer"
                :class="form.categories.includes(cat)
                  ? 'bg-morandi-sage text-white border-morandi-sage shadow-xs'
                  : 'bg-morandi-sidebar text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
              >
                #{{ categoryMap[cat] || cat }}
              </button>
            </div>
          </div>

          <!-- Response Type Selector -->
          <div class="grid grid-cols-2 gap-3">
            <div>
              <label class="font-medium text-morandi-text block mb-1">响应解析格式</label>
              <select
                :value="form.resp_type"
                @change="e => selectRespType((e.target as HTMLSelectElement).value as any)"
                class="morandi-input w-full px-3 py-2 appearance-none cursor-pointer"
              >
                <option value="image">图片二进制流</option>
                <option value="redirect">302 重定向直链</option>
                <option value="json">JSON 节点提取</option>
              </select>
            </div>

            <div>
              <label class="font-medium text-morandi-text block mb-1">JSON 提取路径</label>
              <input
                v-model="form.json_path"
                placeholder="例如: data.image_url"
                :disabled="form.resp_type !== 'json'"
                class="morandi-input w-full px-3 py-2 font-mono disabled:opacity-50"
              />
            </div>
          </div>

          <!-- Weight Selector (Simplified to 1 - 5) -->
          <div>
            <div class="flex items-center justify-between mb-1.5">
              <label class="font-medium text-morandi-text">抽选权重 (1 ~ 5 级)</label>
              <span class="font-mono text-morandi-sage-dark font-bold">Level {{ form.weight }} ({{ weightLabels[form.weight] }})</span>
            </div>
            <div class="grid grid-cols-5 gap-2">
              <button
                v-for="w in 5"
                :key="w"
                type="button"
                @click="form.weight = w"
                class="py-1.5 rounded-lg border font-medium text-center transition-all text-xs"
                :class="form.weight === w
                  ? 'bg-morandi-sage text-white border-morandi-sage shadow-xs'
                  : 'bg-morandi-bg text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
              >
                {{ w }} ({{ weightLabels[w] }})
              </button>
            </div>
          </div>

          <!-- Default Query Parameters (默认请求参数) -->
          <div>
            <label class="font-medium text-morandi-text block mb-1">默认请求参数 <span class="text-morandi-muted font-normal">(追加到所有子分支 URL)</span></label>
            <p class="text-[10px] text-morandi-muted mb-1.5">如 <code class="font-mono bg-morandi-bg px-1 py-0.5 rounded">num=1</code> 或 <code class="font-mono bg-morandi-bg px-1 py-0.5 rounded">num=1&size=500</code>，格式为 key=value，多个参数用 & 连接。</p>
            <input
              v-model="form.default_query"
              placeholder="如 num=1 或 num=1&size=500"
              class="morandi-input w-full px-3 py-2 font-mono text-xs"
            />
          </div>

          <!-- Query Parameter Variants (请求参数与衍生分支) -->
          <div class="border-t border-morandi-border/40 pt-3 space-y-2.5">
            <div class="flex items-center justify-between">
              <label class="font-medium text-morandi-text text-xs flex items-center gap-1.5">
                <Sliders class="w-3.5 h-3.5 text-morandi-sage" /> 请求参数与衍生分支 (Query Params)
              </label>
              <button
                type="button"
                @click="addParamRow"
                class="text-[11px] text-morandi-sage-dark hover:underline flex items-center gap-1 font-semibold cursor-pointer"
              >
                <Plus class="w-3.5 h-3.5 text-morandi-sage" /> 添加参数分支
              </button>
            </div>
            <p class="text-[10px] text-morandi-muted leading-relaxed">
              如 <code class="font-mono bg-morandi-bg px-1 py-0.5 rounded">type=pc</code> 或 <code class="font-mono bg-morandi-bg px-1 py-0.5 rounded">type=mobile</code>。每个分支可独立指定 Tags 与权重并参与分发，总体仍作为一个源统一进行健康检测。
            </p>


            <div v-if="paramRows.length > 0" class="space-y-2.5">
              <div
                v-for="(row, idx) in paramRows"
                :key="idx"
                class="p-3 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft space-y-2"
              >
                <div class="flex items-center gap-2">
                  <input
                    v-model="row.key"
                    placeholder="参数键 (如 type)"
                    class="morandi-input px-2.5 py-1.5 font-mono text-xs w-1/3"
                  />
                  <span class="text-morandi-muted font-bold text-xs">=</span>
                  <input
                    v-model="row.value"
                    placeholder="参数值 (如 pc)"
                    class="morandi-input px-2.5 py-1.5 font-mono text-xs flex-1"
                  />
                  <button
                    type="button"
                    @click="removeParamRow(idx)"
                    class="p-1.5 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors shrink-0 cursor-pointer"
                    title="删除此分支"
                  >
                    <Trash2 class="w-4 h-4" />
                  </button>
                </div>

                <!-- Branch Tag & Weight -->
                <div class="flex items-center justify-between gap-2 pt-1 border-t border-morandi-border/30 flex-wrap">
                  <div class="flex items-center gap-1 flex-wrap">
                    <span class="text-[10px] text-morandi-muted mr-1">分支 Tag:</span>
                    <button
                      v-for="cat in categories"
                      :key="cat"
                      type="button"
                      @click="toggleParamCategory(row, cat)"
                      class="px-2 py-0.5 rounded-md text-[10px] font-semibold border transition-all cursor-pointer"
                      :class="row.categories.includes(cat) ? 'bg-morandi-sage text-white border-morandi-sage' : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
                    >
                      #{{ categoryMap[cat] || cat }}
                    </button>
                  </div>

                  <div class="flex items-center gap-1.5">
                    <span class="text-[10px] text-morandi-muted">权重:</span>
                    <input
                      v-model.number="row.weight"
                      type="number"
                      min="1"
                      max="100"
                      class="morandi-input px-2 py-0.5 text-[11px] font-mono w-14 text-center"
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Custom Headers (Dynamic Rows: Add row / Remove row) -->
          <div class="border-t border-morandi-border/40 pt-3">

            <div class="flex items-center justify-between mb-2">
              <label class="font-medium text-morandi-text">自定义请求头</label>
              <span v-if="headerRows.length > 0" class="text-[10px] text-morandi-muted">已配置 {{ headerRows.length }} 行</span>
            </div>

            <!-- Header Input Rows -->
            <div v-if="showHeaderInput && headerRows.length > 0" class="space-y-2">
              <div
                v-for="(row, idx) in headerRows"
                :key="idx"
                class="flex items-center gap-2"
              >
                <input
                  v-model="row.key"
                  placeholder="Header 名称 (如 Referer)"
                  class="morandi-input flex-1 px-2.5 py-1.5 font-mono text-xs"
                />
                <input
                  v-model="row.value"
                  placeholder="Header 内容"
                  class="morandi-input flex-1 px-2.5 py-1.5 font-mono text-xs"
                />
                <button
                  type="button"
                  @click="removeHeaderRow(idx)"
                  class="p-1.5 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors shrink-0"
                  title="删除此行 Header"
                >
                  <Trash2 class="w-4 h-4" />
                </button>
              </div>

              <!-- Add New Row Button -->
              <button
                type="button"
                @click="addHeaderRow"
                class="w-full py-1.5 px-3 border border-dashed border-morandi-border hover:border-morandi-sage text-morandi-muted hover:text-morandi-sage-dark rounded-xl flex items-center justify-center gap-1.5 transition-colors font-medium text-xs mt-1"
              >
                <Plus class="w-3.5 h-3.5" />
                <span>新增一行 Header</span>
              </button>
            </div>

            <!-- Initial Add Button when collapsed -->
            <button
              v-else
              type="button"
              @click="startHeaderInput"
              class="w-full py-2 border border-dashed border-morandi-border hover:border-morandi-sage text-morandi-muted hover:text-morandi-sage-dark rounded-xl flex items-center justify-center gap-1.5 transition-colors font-medium"
            >
              <Plus class="w-4 h-4" />
              <span>添加 Request Header (如 User-Agent / Referer)</span>
            </button>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="p-4 border-t border-morandi-border/60 bg-morandi-bg/30 flex justify-end gap-2">
        <button
          type="button"
          @click="emit('close')"
          class="px-4 py-2 text-xs font-medium text-morandi-muted hover:text-morandi-text hover:bg-morandi-hover rounded-xl transition-colors"
        >
          取消
        </button>
        <button
          type="button"
          @click="handleSave"
          :disabled="saving || !form.url.trim()"
          class="px-5 py-2 text-xs font-semibold bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-sm transition-all disabled:opacity-50"
        >
          {{ saving ? '保存中...' : '确认保存图源' }}
        </button>
      </div>
    </div>
  </div>
</template>
