<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import { useTags } from '../composables/useTags'
import type { Source, QueryParam } from '../types'
import { X, Plus, Trash2, Sliders, Globe, Check, Save } from 'lucide-vue-next'

const props = defineProps<{
  source: Source
}>()

const emit = defineEmits<{ saved: [], close: [] }>()

const { updateSource } = useApi()
const { tags, getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())
const categories = computed(() => tags.value.map(t => t.id))

const saving = ref(false)

interface ParamRow {
  key: string
  value: string
  weight: number
  categories: string[]
}

const paramRows = ref<ParamRow[]>([])

onMounted(() => {
  if (props.source.params && props.source.params.length > 0) {
    for (const p of props.source.params) {
      paramRows.value.push({
        key: p.key,
        value: p.value,
        weight: p.weight || 3,
        categories: [...(p.categories || [])]
      })
    }
  } else {
    // Add default row if empty
    paramRows.value.push({ key: '', value: '', weight: 3, categories: [] })
  }
})

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

async function handleSave() {
  saving.value = true
  try {
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
    await updateSource(props.source.id, {
      ...props.source,
      params
    })
    emit('saved')
    emit('close')
  } catch {}
  saving.value = false
}
</script>

<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-900/40 backdrop-blur-md transition-all" @click.self="emit('close')">
    <div class="bg-white rounded-2xl shadow-morandi-lg w-full max-w-xl border border-morandi-borderSoft max-h-[90vh] flex flex-col overflow-hidden animate-in fade-in zoom-in-95 duration-200">
      <!-- Modal Header -->
      <div class="p-5 border-b border-morandi-border/60 flex justify-between items-center bg-morandi-bg/50">
        <div>
          <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
            <Sliders class="w-4 h-4 text-morandi-sage" /> API 图源衍生分支管理
          </h2>
          <p class="text-xs text-morandi-muted mt-0.5 flex items-center gap-1 font-mono truncate max-w-md">
            <Globe class="w-3.5 h-3.5 shrink-0 text-morandi-light" />
            <span class="font-semibold text-morandi-text mr-1">{{ source.name }}:</span>
            <span class="truncate">{{ source.url }}</span>
          </p>
        </div>
        <button @click="emit('close')" class="p-1.5 text-morandi-light hover:text-morandi-text hover:bg-morandi-hover rounded-lg transition-colors cursor-pointer">
          <X class="w-5 h-5" />
        </button>
      </div>

      <!-- Modal Body -->
      <div class="p-5 space-y-4 overflow-y-auto flex-1 text-xs">
        <div class="p-3 bg-morandi-sage-light/30 rounded-xl border border-morandi-sage/20 text-morandi-sage-dark space-y-1">
          <p class="font-semibold flex items-center gap-1.5 text-xs">
            <Sliders class="w-3.5 h-3.5" /> 多参数与路径分支功能说明
          </p>
          <p class="text-[11px] leading-relaxed text-morandi-text/80">
            支持形如 <code class="font-mono bg-white px-1 py-0.5 rounded border border-morandi-sage/30">type=pc</code>、<code class="font-mono bg-white px-1 py-0.5 rounded border border-morandi-sage/30">return=302&type=mobile</code> 或路径式 <code class="font-mono bg-white px-1 py-0.5 rounded border border-morandi-sage/30">/pc</code> 分类。<br/>
            每个分支独立绑定分类 Tag 与权重并参与分发，总体仍作为一个统一的图源进行连通性探针与健康统计。
          </p>
        </div>

        <!-- Param Rows List -->
        <div class="space-y-3">
          <div class="flex items-center justify-between">
            <span class="font-bold text-morandi-text text-xs flex items-center gap-1">
              分支列表 (已配置 {{ paramRows.length }} 个)
            </span>
            <button
              type="button"
              @click="addParamRow"
              class="px-3 py-1 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-lg font-medium text-xs flex items-center gap-1 transition-colors cursor-pointer shadow-xs"
            >
              <Plus class="w-3.5 h-3.5" /> 添加分支
            </button>
          </div>

          <div v-if="paramRows.length === 0" class="text-center py-8 text-morandi-muted bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
            暂未配置参数分支，点击右上角【添加分支】按钮新增
          </div>

          <div
            v-for="(row, idx) in paramRows"
            :key="idx"
            class="p-3.5 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft space-y-2.5 hover:border-morandi-sage/40 transition-colors"
          >
            <div class="flex items-center gap-2">
              <span class="font-mono text-morandi-muted font-bold text-xs shrink-0">#{{ idx + 1 }}</span>
              <input
                v-model="row.key"
                placeholder="参数键 Key (如 type 或 return=302&type)"
                class="morandi-input px-2.5 py-1.5 font-mono text-xs flex-1"
              />
              <span class="text-morandi-muted font-bold text-xs shrink-0">=</span>
              <input
                v-model="row.value"
                placeholder="参数值 Value (如 pc 或 mobile)"
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
            <div class="flex items-center justify-between gap-2 pt-2 border-t border-morandi-border/30 flex-wrap">
              <div class="flex items-center gap-1.5 flex-wrap">
                <span class="text-[11px] font-medium text-morandi-muted mr-1">绑定 Tag:</span>
                <button
                  v-for="cat in categories"
                  :key="cat"
                  type="button"
                  @click="toggleParamCategory(row, cat)"
                  class="px-2.5 py-0.5 rounded-md text-[10px] font-semibold border transition-all cursor-pointer"
                  :class="row.categories.includes(cat) ? 'bg-morandi-sage text-white border-morandi-sage shadow-xs' : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
                >
                  #{{ categoryMap[cat] || cat }}
                </button>
              </div>

              <div class="flex items-center gap-1.5">
                <span class="text-[11px] font-medium text-morandi-muted">权重:</span>
                <input
                  v-model.number="row.weight"
                  type="number"
                  min="1"
                  max="100"
                  class="morandi-input px-2 py-0.5 text-xs font-mono w-14 text-center"
                />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="p-4 border-t border-morandi-border/60 flex justify-end gap-2 bg-morandi-bg/30">
        <button
          type="button"
          @click="emit('close')"
          class="px-4 py-2 text-xs font-medium text-morandi-muted hover:bg-morandi-hover rounded-xl transition-colors cursor-pointer"
        >
          取消
        </button>
        <button
          type="button"
          @click="handleSave"
          :disabled="saving"
          class="px-5 py-2 text-xs font-semibold bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-xs disabled:opacity-50 flex items-center gap-1.5 transition-colors cursor-pointer"
        >
          <Save class="w-3.5 h-3.5" />
          <span>{{ saving ? '保存中...' : '保存分支配置' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
