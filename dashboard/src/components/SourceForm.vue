<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { Source } from '../types'
import { X, Plus, Trash2, Sliders, ChevronDown, Sparkles, Loader2, Check } from 'lucide-vue-next'

const props = defineProps<{
  source?: Source
  initialData?: Record<string, any>
}>()
const emit = defineEmits<{ saved: [], close: [] }>()

const { createSource, updateSource } = useApi()
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

onMounted(() => {
  if (props.source) {
    form.value = { ...props.source, headers: { ...props.source.headers } }
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
})

function handleUrlInput() {
  if (userManuallySetType.value) return
  const val = form.value.url.trim().toLowerCase()
  if (!val) return

  if (val.includes('.json') || val.includes('format=json') || val.includes('type=json') || val.includes('json=1')) {
    form.value.resp_type = 'json'
    if (!form.value.json_path) form.value.json_path = 'url'
  } else if (val.includes('redirect') || val.includes('302') || val.includes('picsum.photos') || val.includes('unsplash.com')) {
    form.value.resp_type = 'redirect'
  } else {
    form.value.resp_type = 'image'
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
    const res = await fetch(form.value.url.trim(), { method: 'HEAD', mode: 'no-cors' })
    const contentType = res.headers.get('content-type') || ''
    if (contentType.includes('image/')) {
      form.value.resp_type = 'image'
      testDetected.value = '图片二进制流'
    } else if (contentType.includes('json')) {
      form.value.resp_type = 'json'
      if (!form.value.json_path) form.value.json_path = 'url'
      testDetected.value = 'JSON 节点提取'
    } else if (res.redirected) {
      form.value.resp_type = 'redirect'
      testDetected.value = '302 重定向直链'
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
              <span>自动测类型</span>
            </button>
          </div>
          <input
            v-model="form.url"
            @input="handleUrlInput"
            placeholder="https://api.example.com/v1/photos/random"
            class="morandi-input w-full px-3 py-2 font-mono text-xs"
          />

          <!-- Auto-detected status pill -->
          <div v-if="form.url.trim()" class="mt-1.5 flex items-center gap-2 text-[11px]">
            <span class="text-morandi-muted">智能推导格式：</span>
            <span class="px-2 py-0.5 bg-morandi-sage-light text-morandi-sage-dark rounded-md font-medium border border-morandi-sage/20 flex items-center gap-1">
              <Check class="w-3 h-3 text-morandi-sage font-bold" />
              {{ form.resp_type === 'image' ? '图片二进制流' : form.resp_type === 'json' ? 'JSON 节点提取' : '302 重定向直链' }}
            </span>
            <span v-if="testDetected" class="text-morandi-muted text-[10px]">({{ testDetected }})</span>
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
              <span>{{ showAdvanced ? '折叠高级参数设置' : '展开高级选项 (响应类型、权重、分类、请求头)' }}</span>
            </span>
            <ChevronDown class="w-4 h-4 transition-transform duration-200" :class="{ 'rotate-180': showAdvanced }" />
          </button>
        </div>

        <!-- 4. Advanced Section (Collapsible) -->
        <div v-if="showAdvanced" class="space-y-4 pt-2 border-t border-morandi-border/40 animate-in fade-in duration-200">
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
              <label class="font-medium text-morandi-text block mb-1">JSON 提取 Key 路径</label>
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

          <!-- Categories Tag Selector -->
          <div>
            <label class="font-medium text-morandi-text block mb-1.5">关联分类标签</label>
            <div class="flex flex-wrap gap-1.5">
              <button
                v-for="cat in categories"
                :key="cat"
                type="button"
                @click="toggleCategory(cat)"
                class="px-2.5 py-1 rounded-lg transition-colors font-medium text-xs"
                :class="form.categories.includes(cat)
                  ? 'bg-morandi-sage text-white shadow-xs'
                  : 'bg-morandi-sidebar text-morandi-muted hover:bg-morandi-hover'"
              >
                #{{ categoryMap[cat] || cat }}
              </button>
            </div>
          </div>

          <!-- Custom Headers (Dynamic Rows: Add row / Remove row) -->
          <div class="border-t border-morandi-border/40 pt-3">
            <div class="flex items-center justify-between mb-2">
              <label class="font-medium text-morandi-text">自定义请求头 (Headers 防盗链 / Referer)</label>
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
