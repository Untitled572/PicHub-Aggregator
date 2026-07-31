<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '../composables/useApi'
import type { DetectResult } from '../types'
import SourceForm from '../components/SourceForm.vue'
import JsonTreeSelector from '../components/JsonTreeSelector.vue'
import { Search, Sparkles, CheckCircle, AlertTriangle, Code, ArrowRight } from 'lucide-vue-next'

const { detectURL } = useApi()
const url = ref('')
const detecting = ref(false)
const result = ref<DetectResult | null>(null)
const error = ref('')
const showForm = ref(false)
const detectedData = ref<Record<string, any>>({})

async function handleDetect() {
  if (!url.value) return
  detecting.value = true
  error.value = ''
  result.value = null
  try {
    result.value = await detectURL(url.value)
    detectedData.value = {
      name: new URL(url.value).hostname,
      url: result.value.final_url || url.value,
      resp_type: result.value.resp_type,
      json_path: result.value.url_hints?.[0] || '',
    }
    if (result.value.resp_type === 'redirect' || result.value.resp_type === 'image') {
      showForm.value = true
    }
  } catch (e: any) {
    error.value = e.message || '探测失败，请检查网络或 URL 是否正确'
  } finally {
    detecting.value = false
  }
}

function onJsonPathSelected(path: string) {
  detectedData.value.json_path = path
  showForm.value = true
}

const respTypeMap: Record<string, string> = {
  image: '图片二进制流 (Image)',
  redirect: '302 重定向直链 (Redirect)',
  json: 'JSON 结构化节点 (JSON Body)',
}
</script>

<template>
  <div class="space-y-6">
    <!-- Header banner -->
    <div class="morandi-card p-6 bg-gradient-to-r from-morandi-sand-light/40 to-morandi-sage-light/40 border border-morandi-borderSoft">
      <div class="flex items-start gap-4">
        <div class="w-12 h-12 rounded-2xl bg-morandi-sage text-white flex items-center justify-center shadow-md shrink-0">
          <Sparkles class="w-6 h-6" />
        </div>
        <div>
          <h2 class="text-base font-bold text-morandi-text">API 智能探测引擎</h2>
          <p class="text-xs text-morandi-muted mt-1 leading-relaxed">
            粘贴任意第三方 API 链接，系统将自动检测响应标头与 Body 结构。若返回 JSON，可直接点击路径节点，系统将自动绑定并生成提取规则。
          </p>
        </div>
      </div>
    </div>

    <!-- Search / Input Section -->
    <div class="morandi-card p-5 space-y-3">
      <label class="text-xs font-medium text-morandi-text flex items-center gap-1.5">
        <Search class="w-4 h-4 text-morandi-sage" /> 输入需探测的第三方 API 目标 URL
      </label>

      <div class="flex flex-col sm:flex-row gap-3">
        <div class="relative flex-1">
          <input
            v-model="url"
            placeholder="https://api.example.com/v1/random-picture"
            class="morandi-input w-full px-4 py-2.5 text-xs font-mono"
            @keyup.enter="handleDetect"
          />
        </div>

        <button
          @click="handleDetect"
          :disabled="detecting || !url.trim()"
          class="flex items-center justify-center gap-2 px-6 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all disabled:opacity-50 shrink-0"
        >
          <Sparkles class="w-4 h-4" />
          {{ detecting ? '探测分析中...' : '一键智能识别' }}
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="morandi-card p-4 bg-morandi-rose-light text-morandi-rose-dark border-morandi-rose/30 flex items-center gap-3 text-xs">
      <AlertTriangle class="w-5 h-5 shrink-0" />
      <span>{{ error }}</span>
    </div>

    <!-- Detection Result Display -->
    <div v-if="result" class="space-y-4">
      <div class="morandi-card p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-3">
          <div class="flex items-center gap-2">
            <CheckCircle class="w-5 h-5 text-morandi-sage-dark" />
            <h3 class="font-bold text-sm text-morandi-text">分析检测成功</h3>
          </div>
          <span class="text-xs px-2.5 py-1 bg-morandi-sidebar rounded-full text-morandi-muted font-medium">
            HTTP Status Code: 200 OK
          </span>
        </div>

        <div class="grid grid-cols-1 sm:grid-cols-2 gap-4 text-xs">
          <div class="bg-morandi-bg p-3.5 rounded-xl border border-morandi-borderSoft">
            <span class="text-morandi-muted block text-[11px] mb-1">自动识别响应类型</span>
            <span class="font-bold text-morandi-sage-dark text-sm">{{ respTypeMap[result.resp_type] || result.resp_type }}</span>
          </div>

          <div class="bg-morandi-bg p-3.5 rounded-xl border border-morandi-borderSoft">
            <span class="text-morandi-muted block text-[11px] mb-1">图片 URL 候选提示 (Hints)</span>
            <span class="font-mono text-morandi-text text-xs break-all">
              {{ result.url_hints?.length ? result.url_hints.join(', ') : '无自动提示节点' }}
            </span>
          </div>
        </div>

        <!-- JSON Tree Picker -->
        <div v-if="result.resp_type === 'json' && result.body_tree" class="pt-2">
          <div class="flex items-center justify-between mb-3">
            <h4 class="text-xs font-semibold text-morandi-text flex items-center gap-1.5">
              <Code class="w-4 h-4 text-morandi-ocean" /> 点击 JSON 树状节点以绑定图片路径
            </h4>
            <span class="text-[11px] text-morandi-muted">高亮节点为系统智能判定的 URL 候选</span>
          </div>

          <div class="bg-morandi-bg p-4 rounded-xl border border-morandi-borderSoft max-h-96 overflow-auto">
            <JsonTreeSelector :data="result.body_tree" :url-hints="result.url_hints" @select="onJsonPathSelected" />
          </div>
        </div>

        <!-- Auto Add Action -->
        <div v-if="result.resp_type !== 'json'" class="pt-2 flex justify-end">
          <button
            @click="showForm = true"
            class="flex items-center gap-2 px-5 py-2.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-medium shadow-sm transition-all"
          >
            <span>填充并一键保存图源</span>
            <ArrowRight class="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>

    <!-- Modal Form -->
    <SourceForm
      v-if="showForm"
      :initial-data="detectedData"
      @saved="showForm = false"
      @close="showForm = false"
    />
  </div>
</template>

