<script setup lang="ts">
import { ref } from 'vue'
import { useApi } from '../composables/useApi'
import type { ExportData } from '../types'
import { X, Download, Upload, Copy, Check } from 'lucide-vue-next'

const emit = defineEmits<{ close: [] }>()
const { exportRules, importRules } = useApi()
const tab = ref<'export' | 'import'>('export')
const exportedData = ref('')
const importText = ref('')
const importResult = ref<{ imported: number } | null>(null)
const loading = ref(false)
const copied = ref(false)

async function handleExport() {
  loading.value = true
  try {
    const data = await exportRules()
    exportedData.value = JSON.stringify(data, null, 2)
  } catch {}
  loading.value = false
}

async function handleImport() {
  loading.value = true
  importResult.value = null
  try {
    const data = JSON.parse(importText.value) as ExportData
    importResult.value = await importRules(data)
  } catch {
    importResult.value = { imported: -1 }
  }
  loading.value = false
}

function copyExport() {
  navigator.clipboard.writeText(exportedData.value)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-900/40 backdrop-blur-md transition-all" @click.self="emit('close')">
    <div class="bg-white rounded-2xl shadow-morandi-lg w-full max-w-lg border border-morandi-borderSoft overflow-hidden animate-in fade-in zoom-in-95 duration-200">
      <!-- Modal Header -->
      <div class="p-5 border-b border-morandi-border/60 flex justify-between items-center bg-morandi-bg/50">
        <div>
          <h2 class="font-bold text-base text-morandi-text">规则导出 / 导入</h2>
          <p class="text-xs text-morandi-muted mt-0.5">将节点配置备份为 JSON 文件或快速从社区导入</p>
        </div>
        <button @click="emit('close')" class="p-1 text-morandi-light hover:text-morandi-text hover:bg-morandi-hover rounded-lg transition-colors">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Tabs -->
      <div class="flex border-b border-morandi-border/60 bg-morandi-bg/20 text-xs font-medium">
        <button
          @click="tab = 'export'"
          class="flex-1 py-3 flex items-center justify-center gap-2 border-b-2 transition-colors"
          :class="tab === 'export'
            ? 'text-morandi-sage-dark border-morandi-sage bg-white font-semibold'
            : 'text-morandi-muted border-transparent hover:text-morandi-text'"
        >
          <Download class="w-4 h-4" /> 导出节点规则
        </button>
        <button
          @click="tab = 'import'"
          class="flex-1 py-3 flex items-center justify-center gap-2 border-b-2 transition-colors"
          :class="tab === 'import'
            ? 'text-morandi-sage-dark border-morandi-sage bg-white font-semibold'
            : 'text-morandi-muted border-transparent hover:text-morandi-text'"
        >
          <Upload class="w-4 h-4" /> 批量导入规则
        </button>
      </div>

      <!-- Tab Content -->
      <div class="p-5 text-xs">
        <div v-if="tab === 'export'" class="space-y-3">
          <p class="text-morandi-muted">点击下方按钮生成当前全部已配置 API 图源的 JSON 导出代码：</p>

          <button
            @click="handleExport"
            :disabled="loading"
            class="flex items-center gap-1.5 px-4 py-2 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl font-medium shadow-sm transition-all disabled:opacity-50"
          >
            <Download class="w-4 h-4" />
            {{ loading ? '正在导出中...' : '生成 JSON 配置文件' }}
          </button>

          <div v-if="exportedData" class="mt-4 space-y-2">
            <div class="flex justify-between items-center">
              <span class="text-morandi-muted">导出预览：</span>
              <button @click="copyExport" class="flex items-center gap-1 text-morandi-sage-dark hover:underline font-medium">
                <component :is="copied ? Check : Copy" class="w-3.5 h-3.5" />
                {{ copied ? '已复制到剪贴板！' : '一键复制 JSON' }}
              </button>
            </div>
            <pre class="bg-morandi-bg p-3 rounded-xl border border-morandi-borderSoft font-mono text-[11px] max-h-60 overflow-auto text-morandi-text">{{ exportedData }}</pre>
          </div>
        </div>

        <div v-else class="space-y-3">
          <p class="text-morandi-muted">请在此粘贴从社区或其他节点导出的 JSON 规则代码：</p>
          <textarea
            v-model="importText"
            rows="6"
            placeholder="请在此粘贴 JSON 文本..."
            class="morandi-input w-full p-3 font-mono text-xs"
          ></textarea>

          <div class="flex items-center justify-between pt-1">
            <button
              @click="handleImport"
              :disabled="loading || !importText.trim()"
              class="flex items-center gap-1.5 px-4 py-2 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl font-medium shadow-sm transition-all disabled:opacity-50"
            >
              <Upload class="w-4 h-4" />
              {{ loading ? '导入中...' : '确认导入' }}
            </button>

            <div v-if="importResult" class="text-xs font-medium" :class="importResult.imported >= 0 ? 'text-morandi-sage-dark' : 'text-morandi-rose-dark'">
              {{ importResult.imported >= 0 ? `✓ 成功导入 ${importResult.imported} 条规则` : '✕ JSON 格式错误或提取失败' }}
            </div>
          </div>
        </div>
      </div>


    </div>
  </div>
</template>

