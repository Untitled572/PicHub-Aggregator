<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { Source } from '../types'
import { X, Plus, Trash2 } from 'lucide-vue-next'

const props = defineProps<{
  source?: Source
  initialData?: Record<string, any>
}>()
const emit = defineEmits<{ saved: [], close: [] }>()

const { createSource, updateSource } = useApi()
const saving = ref(false)

const form = ref({
  name: '',
  url: '',
  resp_type: 'json',
  json_path: '',
  weight: 10,
  categories: [] as string[],
  headers: {} as Record<string, string>,
  enabled: true,
})

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}

const categories = Object.keys(categoryMap)
const headerKey = ref('')
const headerValue = ref('')

onMounted(() => {
  if (props.source) {
    form.value = { ...props.source, headers: { ...props.source.headers } }
  } else if (props.initialData) {
    Object.assign(form.value, props.initialData)
  }
})

function addHeader() {
  if (headerKey.value) {
    form.value.headers[headerKey.value] = headerValue.value
    headerKey.value = ''
    headerValue.value = ''
  }
}

function removeHeader(key: string) {
  delete form.value.headers[key]
}

function toggleCategory(cat: string) {
  const idx = form.value.categories.indexOf(cat)
  if (idx >= 0) form.value.categories.splice(idx, 1)
  else form.value.categories.push(cat)
}

async function handleSave() {
  saving.value = true
  try {
    if (props.source) {
      await updateSource(props.source.id, form.value)
    } else {
      await createSource(form.value)
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
          <p class="text-xs text-morandi-muted mt-0.5">配置第三方图片接口参数与提取解析逻辑</p>
        </div>
        <button @click="emit('close')" class="p-1 text-morandi-light hover:text-morandi-text hover:bg-morandi-hover rounded-lg transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-5 space-y-4 overflow-y-auto flex-1 text-xs">
        <div>
          <label class="font-medium text-morandi-text block mb-1">图源名称 <span class="text-morandi-rose">*</span></label>
          <input v-model="form.name" placeholder="例如：Unsplash 高清风景 API" class="morandi-input w-full px-3 py-2" />
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1">API 请求地址 (URL) <span class="text-morandi-rose">*</span></label>
          <input v-model="form.url" placeholder="https://api.example.com/v1/photos/random" class="morandi-input w-full px-3 py-2 font-mono" />
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="font-medium text-morandi-text block mb-1">响应解析类型</label>
            <select v-model="form.resp_type" class="morandi-input w-full px-3 py-2 appearance-none cursor-pointer">
              <option value="image">图片二进制流</option>
              <option value="redirect">302 重定向直链</option>
              <option value="json">JSON 节点提取</option>
            </select>
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1">JSON 提取路径</label>
            <input v-model="form.json_path" placeholder="例如: data.image_url" :disabled="form.resp_type !== 'json'" class="morandi-input w-full px-3 py-2 font-mono disabled:opacity-50" />
          </div>
        </div>

        <div>
          <div class="flex items-center justify-between mb-1">
            <label class="font-medium text-morandi-text">抽选权重 (1 ~ 100)</label>
            <span class="font-mono text-morandi-sage-dark font-bold">{{ form.weight }}</span>
          </div>
          <input v-model.number="form.weight" type="range" min="1" max="100" class="w-full accent-morandi-sage cursor-pointer" />
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1.5">关联分类标签</label>
          <div class="flex flex-wrap gap-1.5">
            <button
              v-for="cat in categories" :key="cat"
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

        <!-- Headers configuration -->
        <div class="border-t border-morandi-border/40 pt-3">
          <label class="font-medium text-morandi-text block mb-1.5">自定义请求头 (Headers 防盗链 / User-Agent)</label>
          <div class="flex gap-2 mb-2">
            <input v-model="headerKey" placeholder="Header 名称 (如 Referer)" class="morandi-input flex-1 px-2.5 py-1.5 font-mono" />
            <input v-model="headerValue" placeholder="Header 内容" class="morandi-input flex-1 px-2.5 py-1.5 font-mono" />
            <button type="button" @click="addHeader" class="px-3 py-1.5 bg-morandi-sidebar hover:bg-morandi-hover text-morandi-text font-medium rounded-lg flex items-center gap-1 shrink-0 transition-colors">
              <Plus class="w-3.5 h-3.5" /> 添加
            </button>
          </div>

          <div v-if="Object.keys(form.headers).length > 0" class="space-y-1.5 max-h-32 overflow-y-auto">
            <div v-for="(v, k) in form.headers" :key="k" class="flex items-center justify-between bg-morandi-bg p-2 rounded-lg border border-morandi-borderSoft font-mono">
              <span class="text-morandi-sage-dark font-medium">{{ k }}: <span class="text-morandi-text font-normal">{{ v }}</span></span>
              <button type="button" @click="removeHeader(k)" class="text-morandi-muted hover:text-morandi-rose-dark p-0.5 rounded transition-colors">
                <Trash2 class="w-3.5 h-3.5" />
              </button>
            </div>
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
          :disabled="saving || !form.name || !form.url"
          class="px-5 py-2 text-xs font-medium bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-sm transition-all disabled:opacity-50"
        >
          {{ saving ? '保存中...' : '保存配置' }}
        </button>
      </div>


    </div>
  </div>
</template>

