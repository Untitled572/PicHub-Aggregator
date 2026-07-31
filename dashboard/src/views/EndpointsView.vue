<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Source, Settings, Endpoint } from '../types'
import { useTags } from '../composables/useTags'
import { useApi } from '../composables/useApi'
import EndpointTagBinding from '../components/EndpointTagBinding.vue'
import {
  Link2,
  Copy,
  Check,
  ExternalLink,
  Plus,
  Edit3,
  Trash2,
  Sparkles,
  Sliders,
  Code2,
  Globe,
  Tag as TagIcon,
  X,
  CheckCircle2,
  Lock,
  Save,
  Power
} from 'lucide-vue-next'

const { tags, addTag, renameTag, deleteTag, toggleExclusive } = useTags()
const { listSources, getSettings, updateSettings, listEndpoints, createEndpoint, updateEndpoint, deleteEndpoint, toggleEndpoint } = useApi()

const systemTags = computed(() => tags.value.filter(t => t.system))
const customTags = computed(() => tags.value.filter(t => !t.system))


const sources = ref<Source[]>([])
const copiedState = ref<Record<string, boolean>>({})
const showAddTagModal = ref(false)
const newTagId = ref('')
const newTagName = ref('')
const editingTagId = ref<string | null>(null)
const editingTagName = ref('')
const boundTags = ref<string[]>([])
const cachedSettings = ref<Settings | null>(null)
const saveSuccess = ref(false)

const endpoints = ref<Endpoint[]>([])
const drafts = ref<{ name: string; bound_tags: string[] }[]>([])
const savingDraftIdx = ref(-1)

onMounted(async () => {
  try {
    sources.value = await listSources()
  } catch {}
  try {
    const settings = await getSettings()
    cachedSettings.value = settings
    if (settings.bound_tags) {
      boundTags.value = settings.bound_tags
    }
  } catch {}
  try {
    endpoints.value = await listEndpoints() || []
  } catch {}
})

const origin = computed(() => window.location.origin)

// Count sources per tag (含参数分支 categories, 每源每标签去重)
const tagSourceCounts = computed(() => {
  const counts: Record<string, number> = {}
  for (const t of tags.value) {
    counts[t.id] = 0
  }
  for (const src of sources.value) {
    const seen = new Set<string>()
    const add = (cat?: string) => {
      if (cat && cat !== '__uncategorized__' && !seen.has(cat)) {
        seen.add(cat)
        counts[cat] = (counts[cat] || 0) + 1
      }
    }
    ;(src.categories || []).forEach(add)
    for (const p of src.params || []) {
      ;(p.categories || []).forEach(add)
    }
  }
  return counts
})

const masterUrl = computed(() => {
  return `${origin.value}/random`
})

const masterJsonUrl = computed(() => {
  return `${origin.value}/random?format=json`
})

const isAllTagsBound = computed(() => {
  if (boundTags.value.length === 0) return false
  const visibleTags = tags.value.map(t => t.id)
  return visibleTags.every(t => boundTags.value.includes(t))
})

async function saveBoundTags(tags: string[]) {
  const base = cachedSettings.value || { proxy_mode: false, cache_max_mb: 200, cache_max_images: 60, cache_ttl: 60, min_resolution: '640x480', rate_limit: 60, timeout: 3000 }
  try {
    const updated = await updateSettings({ ...base, bound_tags: tags })
    cachedSettings.value = updated
    saveSuccess.value = true
    setTimeout(() => saveSuccess.value = false, 1200)
  } catch {}
}

function onMasterBoundChange(v: string[]) {
  boundTags.value = v
  saveBoundTags(v)
}

function endpointUrl(ep: Endpoint): string {
  return `${origin.value}/e/${ep.name}`
}

function addEndpointDraft() {
  drafts.value.push({ name: '', bound_tags: [] })
}

function removeDraft(idx: number) {
  drafts.value.splice(idx, 1)
}

async function saveDraft(idx: number) {
  const draft = drafts.value[idx]
  if (!draft.name.trim() || savingDraftIdx.value === idx) return
  savingDraftIdx.value = idx
  try {
    const created = await createEndpoint({ name: draft.name.trim(), bound_tags: draft.bound_tags })
    if (created) {
      endpoints.value.push(created)
      drafts.value.splice(idx, 1)
    }
  } catch {}
  savingDraftIdx.value = -1
}

async function onEndpointTagsChange(ep: Endpoint, v: string[]) {
  ep.bound_tags = v
  try {
    await updateEndpoint(ep.id, { bound_tags: v })
  } catch {}
}

async function onEndpointEnabledChange(ep: Endpoint) {
  try {
    const updated = await toggleEndpoint(ep.id)
    if (updated) ep.enabled = updated.enabled
  } catch {}
}

async function onEndpointRename(ep: Endpoint) {
  const name = ep.name.trim()
  if (!name || name === ep.name) return
  try {
    const updated = await updateEndpoint(ep.id, { name })
    if (updated) Object.assign(ep, updated)
  } catch {}
}

async function handleEndpointDelete(ep: Endpoint) {
  try {
    await deleteEndpoint(ep.id)
    endpoints.value = endpoints.value.filter(e => e.id !== ep.id)
  } catch {}
}

function copyToClipboard(key: string, text: string) {
  navigator.clipboard.writeText(text)
  copiedState.value[key] = true
  setTimeout(() => {
    copiedState.value[key] = false
  }, 2000)
}

function handleAddTag() {
  if (!newTagId.value.trim()) return
  addTag(newTagId.value, newTagName.value)
  newTagId.value = ''
  newTagName.value = ''
  showAddTagModal.value = false
}

function startRename(tag: { id: string; name: string }) {
  editingTagId.value = tag.id
  editingTagName.value = tag.name
}

function saveRename(tagId: string) {
  if (editingTagName.value.trim()) {
    renameTag(tagId, editingTagName.value.trim())
  }
  editingTagId.value = null
}

const confirmingTagId = ref<string | null>(null)

function handleTagDelete(id: string) {
  deleteTag(id)
  confirmingTagId.value = null
}
</script>




<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="morandi-card p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
      <div>
        <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
          <Link2 class="w-5 h-5 text-morandi-sage" /> 分发接口与 Tag 标签管理
        </h2>
        <p class="text-xs text-morandi-muted mt-0.5">管理分类 Tag 标签、配置绑定范围与主服务聚合分发接口</p>
      </div>
    </div>


    <!-- Master Endpoint Card -->
    <div class="morandi-card p-4 sm:p-6 space-y-4 border-l-4 border-l-morandi-sage">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2.5">
        <div class="flex items-center gap-2">
          <div class="w-8 h-8 rounded-lg bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0">
            <Sparkles class="w-4 h-4" />
          </div>
          <div>
            <h3 class="font-bold text-sm text-morandi-text">主聚合分发接口 (Master Endpoint)</h3>
            <p class="text-[11px] text-morandi-muted">统一调用的主服务接口，支持绑定特定分类 Tags</p>
          </div>
        </div>
        <span class="text-[10px] px-2.5 py-1 bg-emerald-50 text-emerald-700 border border-emerald-200 rounded-full font-semibold shrink-0 whitespace-nowrap">
          ● 运行正常
        </span>
      </div>

      <!-- Master URL display -->
      <div class="space-y-2 bg-morandi-bg/80 p-3.5 rounded-xl border border-morandi-borderSoft">
        <div class="flex items-center justify-between flex-wrap gap-2">
          <span class="text-xs font-semibold text-morandi-text flex items-center gap-1">
            <Globe class="w-3.5 h-3.5 text-morandi-sage" /> 直链 / 302 重定向总接口：
          </span>
          <div class="flex items-center gap-2">
            <button
              @click="copyToClipboard('master', masterUrl)"
              class="flex items-center gap-1 text-xs font-medium text-morandi-sage-dark hover:underline cursor-pointer"
            >
              <component :is="copiedState['master'] ? Check : Copy" class="w-3.5 h-3.5" />
              <span>{{ copiedState['master'] ? '已复制!' : '一键复制' }}</span>
            </button>
            <a
              :href="masterUrl"
              target="_blank"
              class="flex items-center gap-1 text-xs font-medium text-morandi-muted hover:text-morandi-text hover:underline"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>测试访问</span>
            </a>
          </div>
        </div>
        <div class="font-mono text-xs text-morandi-sage-dark font-semibold bg-white p-2.5 rounded-lg border border-morandi-borderSoft truncate selection:bg-morandi-sage-light">
          {{ masterUrl }}
        </div>
      </div>

      <!-- Master JSON URL display -->
      <div class="space-y-2 bg-morandi-bg/80 p-3.5 rounded-xl border border-morandi-borderSoft">
        <div class="flex items-center justify-between flex-wrap gap-2">
          <span class="text-xs font-semibold text-morandi-text flex items-center gap-1">
            <Code2 class="w-3.5 h-3.5 text-morandi-ocean" /> JSON 格式返回总接口：
          </span>
          <div class="flex items-center gap-2">
            <button
              @click="copyToClipboard('masterJson', masterJsonUrl)"
              class="flex items-center gap-1 text-xs font-medium text-morandi-ocean-dark hover:underline cursor-pointer"
            >
              <component :is="copiedState['masterJson'] ? Check : Copy" class="w-3.5 h-3.5" />
              <span>{{ copiedState['masterJson'] ? '已复制!' : '一键复制' }}</span>
            </button>
            <a
              :href="masterJsonUrl"
              target="_blank"
              class="flex items-center gap-1 text-xs font-medium text-morandi-muted hover:text-morandi-text hover:underline"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>预览 JSON</span>
            </a>
          </div>
        </div>
        <div class="font-mono text-xs text-morandi-ocean-dark font-semibold bg-white p-2.5 rounded-lg border border-morandi-borderSoft truncate">
          {{ masterJsonUrl }}
        </div>
      </div>

      <!-- Tag Binding Filter -->
      <div class="pt-2 border-t border-morandi-border/40 space-y-2">
        <div class="flex items-center justify-between flex-wrap gap-1">
          <label class="text-xs font-bold text-morandi-text flex items-center gap-1.5">
            <Sliders class="w-3.5 h-3.5 text-morandi-sage" /> <span class="relative">主接口绑定的 Tag 标签范围：
            <Transition name="fade">
              <span v-if="saveSuccess" class="absolute left-full top-1/2 -translate-y-1/2 ml-1.5 whitespace-nowrap flex items-center gap-1 text-xs text-morandi-sage-dark font-medium">
                <CheckCircle2 class="w-2.5 h-2.5" /> 已保存
              </span>
            </Transition></span>
          </label>
          <span class="text-[11px] text-morandi-muted">
            {{ isAllTagsBound ? '全选 (无过滤条件)' : `已绑定 ${boundTags.length} 个标签` }}
          </span>
        </div>

        <EndpointTagBinding
          :model-value="boundTags"
          :tags="tags"
          @update:model-value="onMasterBoundChange"
        />

        <div v-if="boundTags.length === 0" class="mt-2 px-3 py-2 bg-morandi-bg/80 border border-morandi-borderSoft rounded-xl text-[11px] text-morandi-muted flex items-center gap-2">
          <span class="text-amber-500">⚠</span>
          <span>未绑定任何 Tag 标签，所有图源均会输出</span>
        </div>
      </div>
    </div>

    <!-- Custom Endpoints Section -->
    <div class="morandi-card p-4 sm:p-6 space-y-5">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-1.5">
            <Link2 class="w-4 h-4 text-morandi-sage" /> 自定义分发端点 (共 {{ endpoints.length + drafts.length }} 个)
          </h3>
          <p class="text-xs text-morandi-muted mt-0.5">创建与主接口 /random 完全同功能的多条分发接口 <code class="font-mono bg-white px-1 py-0.5 rounded border border-morandi-borderSoft">/e/{name}</code>，可独立绑定标签</p>
        </div>

        <button
          @click="addEndpointDraft"
          class="w-full sm:w-auto flex items-center justify-center gap-1.5 px-3.5 py-2 sm:py-1.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-xs transition-colors cursor-pointer shrink-0 whitespace-nowrap"
        >
          <Plus class="w-3.5 h-3.5" />
          <span class="whitespace-nowrap">添加端点</span>
        </button>
      </div>

      <div v-if="endpoints.length === 0 && drafts.length === 0" class="text-center py-8 text-morandi-muted bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border text-xs">
        暂未配置自定义端点，点击右上角【添加端点】新增
      </div>

      <!-- Draft Rows (未保存的新端点) -->
      <div
        v-for="(draft, idx) in drafts"
        :key="'draft-' + idx"
        class="p-4 bg-morandi-sage-light/20 rounded-xl border border-dashed border-morandi-sage/40 space-y-3"
      >
        <div class="flex items-center gap-2">
          <span class="font-mono text-morandi-muted font-bold text-xs shrink-0">新端点</span>
          <input
            v-model="draft.name"
            placeholder="端点名称 (小写字母/数字/连字符, 如: anime)"
            class="morandi-input px-2.5 py-1.5 font-mono text-xs flex-1"
          />
          <button
            type="button"
            @click="removeDraft(idx)"
            class="p-1.5 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors shrink-0 cursor-pointer"
            title="取消此端点"
          >
            <X class="w-4 h-4" />
          </button>
        </div>

        <EndpointTagBinding v-model="draft.bound_tags" :tags="tags" />

        <div class="flex justify-end pt-2 border-t border-morandi-sage/20">
          <button
            type="button"
            @click="saveDraft(idx)"
            :disabled="!draft.name.trim() || savingDraftIdx === idx"
            class="px-3.5 py-1.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-xs transition-colors cursor-pointer disabled:opacity-50 flex items-center gap-1.5"
          >
            <Save class="w-3.5 h-3.5" />
            <span>{{ savingDraftIdx === idx ? '创建中...' : '创建端点' }}</span>
          </button>
        </div>
      </div>

      <!-- Saved Endpoint Rows -->
      <div
        v-for="(ep, idx) in endpoints"
        :key="ep.id"
        class="p-4 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft space-y-3 hover:border-morandi-sage/40 transition-colors"
      >
        <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
          <div class="flex items-center gap-2 flex-wrap">
            <span class="font-mono text-morandi-muted font-bold text-xs shrink-0">#{{ idx + 1 }}</span>
            <span class="font-mono text-xs font-bold text-morandi-sage-dark bg-white px-2 py-1 rounded-lg border border-morandi-borderSoft">
              {{ endpointUrl(ep) }}
            </span>
            <button
              type="button"
              @click="copyToClipboard('ep-' + ep.id, endpointUrl(ep))"
              class="p-1.5 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors cursor-pointer"
              title="复制端点 URL"
            >
              <component :is="copiedState['ep-' + ep.id] ? Check : Copy" class="w-3.5 h-3.5" />
            </button>
            <a
              :href="endpointUrl(ep)"
              target="_blank"
              rel="noopener noreferrer"
              class="p-1.5 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors"
              title="测试访问"
            >
              <ExternalLink class="w-3.5 h-3.5" />
            </a>
          </div>

          <div class="flex items-center gap-2">
            <button
              type="button"
              @click="onEndpointEnabledChange(ep)"
              class="flex items-center gap-1.5 px-2.5 py-1 rounded-lg text-[11px] font-semibold border transition-colors cursor-pointer"
              :class="ep.enabled
                ? 'bg-emerald-50 text-emerald-700 border-emerald-200'
                : 'bg-morandi-bg text-morandi-muted border-morandi-borderSoft'"
            >
              <Power class="w-3 h-3" />
              <span>{{ ep.enabled ? '启用中' : '已停用' }}</span>
            </button>
            <button
              type="button"
              @click="handleEndpointDelete(ep)"
              class="flex items-center gap-1 px-2.5 py-1 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors cursor-pointer"
              title="删除此端点"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>删除</span>
            </button>
          </div>
        </div>

        <div class="flex items-center gap-2">
          <span class="text-[11px] font-medium text-morandi-muted shrink-0">端点名称:</span>
          <input
            v-model="ep.name"
            @keyup.enter="onEndpointRename(ep)"
            @blur="onEndpointRename(ep)"
            class="morandi-input px-2.5 py-1.5 font-mono text-xs w-56"
          />
        </div>

        <div class="pt-2 border-t border-morandi-border/30">
          <span class="text-[11px] font-medium text-morandi-muted block mb-1">绑定 Tag 范围 (勾选即时保存):</span>
          <EndpointTagBinding
            :model-value="ep.bound_tags"
            :tags="tags"
            @update:model-value="v => onEndpointTagsChange(ep, v)"
          />
        </div>
      </div>
    </div>

    <!-- Pure Tag Management Section -->
    <div class="morandi-card p-4 sm:p-6 space-y-5">
      <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3">
        <div>
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-1.5">
            <TagIcon class="w-4 h-4 text-morandi-sage" /> Tag 标签库管理 (共 {{ tags.length }} 个)
          </h3>
          <p class="text-xs text-morandi-muted mt-0.5">系统的内置只读标签与自定义分发标签管理</p>
        </div>

        <button
          @click="showAddTagModal = true"
          class="w-full sm:w-auto flex items-center justify-center gap-1.5 px-3.5 py-2 sm:py-1.5 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-xs transition-colors cursor-pointer shrink-0 whitespace-nowrap"
        >
          <Plus class="w-3.5 h-3.5" />
          <span class="whitespace-nowrap">添加自定义标签</span>
        </button>
      </div>

      <!-- System Tags Unified Box (系统内置标签框) -->
      <div v-if="systemTags.length > 0" class="p-4 bg-morandi-bg/80 rounded-2xl border border-morandi-borderSoft space-y-3">
        <div class="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-2">
          <div class="flex items-center gap-2">
            <div class="w-6 h-6 rounded-lg bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center shrink-0">
              <Lock class="w-3.5 h-3.5 text-morandi-sage-dark" />
            </div>
            <h4 class="text-xs font-bold text-morandi-text">系统内置标签 (只读 / 规则判定)</h4>
          </div>
          <span class="text-[10px] text-morandi-muted bg-white px-2 py-0.5 rounded-md border border-morandi-borderSoft">自动按图片真实宽高比例或系统规则匹配</span>
        </div>


        <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3">
          <div
            v-for="tag in systemTags"
            :key="tag.id"
            class="p-3 bg-white rounded-xl border border-morandi-borderSoft/80 flex items-center justify-between shadow-2xs"
          >
            <div class="flex items-center gap-2">
              <span class="px-2.5 py-0.5 bg-morandi-sage-light text-morandi-sage-dark rounded-lg font-bold text-xs border border-morandi-sage/20">
                #{{ tag.name }}
              </span>
              <span class="font-mono text-xs text-morandi-muted">({{ tag.id }})</span>
            </div>

            <div class="flex items-center gap-1.5">
              <span class="text-[10px] px-2 py-0.5 bg-morandi-bg text-morandi-muted rounded-md font-medium">
                {{ tagSourceCounts[tag.id] || 0 }} 源关联
              </span>
              <span class="text-[10px] px-1.5 py-0.5 bg-slate-100 text-slate-500 rounded font-mono">系统</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Custom Tag Grid Cards -->
      <div class="space-y-2.5">
        <h4 class="text-xs font-bold text-morandi-text flex items-center gap-1.5">
          <Sliders class="w-3.5 h-3.5 text-morandi-sage" /> 自定义分类 Tag 标签 (共 {{ customTags.length }} 个)
        </h4>

        <div v-if="customTags.length === 0" class="text-center py-6 text-morandi-muted bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border text-xs">
          暂无自定义标签，点击右上角【添加自定义标签】新增
        </div>

        <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 pt-1">
          <div
            v-for="tag in customTags"
            :key="tag.id"
            class="p-3.5 bg-morandi-bg/60 hover:bg-white rounded-xl border border-morandi-borderSoft flex flex-col justify-between space-y-2.5 transition-all hover:shadow-sm"
          >
            <!-- Top Tag Identifier & Count -->
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="px-2.5 py-0.5 bg-morandi-sage-light text-morandi-sage-dark rounded-lg font-bold text-xs border border-morandi-sage/20">
                  #{{ tag.name }}
                </span>
                <span class="font-mono text-xs text-morandi-muted">({{ tag.id }})</span>
              </div>

              <span class="text-[10px] px-2 py-0.5 bg-morandi-sidebar text-morandi-muted rounded-md font-medium">
                {{ tagSourceCounts[tag.id] || 0 }} 源关联
              </span>
            </div>

            <!-- Edit Mode / Normal Action Buttons -->
            <div v-if="editingTagId === tag.id" class="flex gap-2 pt-1">
              <input
                v-model="editingTagName"
                placeholder="新名称"
                class="morandi-input px-2 py-1 text-xs flex-1"
                @keyup.enter="saveRename(tag.id)"
              />
              <button
                @click="saveRename(tag.id)"
                class="px-2.5 py-1 bg-morandi-sage text-white rounded-lg text-xs font-semibold cursor-pointer"
              >
                保存
              </button>
              <button
                @click="editingTagId = null"
                class="px-2 py-1 text-morandi-muted hover:bg-morandi-hover rounded-lg text-xs cursor-pointer"
              >
                取消
              </button>
            </div>

            <div v-else class="flex items-center justify-end gap-2 pt-1 border-t border-morandi-border/30 text-xs">
              <button
                type="button"
                @click="toggleExclusive(tag.id)"
                class="px-2 py-0.5 rounded-md text-[10px] font-medium border transition-all cursor-pointer select-none"
                :class="tag.exclusive
                  ? 'bg-morandi-sand-light/60 text-morandi-sand-dark border-morandi-sand/30 font-semibold'
                  : 'bg-morandi-bg text-morandi-muted/80 border-morandi-borderSoft/60 hover:text-morandi-text'"
                title="Exclusive 独占标签"
              >
                Exclusive {{ tag.exclusive ? '✓' : '' }}
              </button>

              <button
                @click="startRename(tag)"
                class="flex items-center gap-1 px-2 py-1 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/50 rounded-lg transition-colors cursor-pointer"
              >
                <Edit3 class="w-3.5 h-3.5" />
                <span>重命名</span>
              </button>

              <!-- Inline Tag Delete Confirmation -->
              <div v-if="confirmingTagId === tag.id" class="flex items-center gap-1.5 bg-rose-50 px-2.5 py-1 rounded-xl border border-rose-200 animate-in fade-in zoom-in-95 duration-150">
                <span class="text-[11px] font-bold text-rose-700">确认删除?</span>
                <button
                  @click="handleTagDelete(tag.id)"
                  class="px-2 py-0.5 bg-rose-600 hover:bg-rose-700 text-white font-bold text-[11px] rounded-lg transition-colors cursor-pointer"
                >
                  删除
                </button>
                <button
                  @click="confirmingTagId = null"
                  class="px-2 py-0.5 bg-white hover:bg-slate-100 text-slate-600 font-medium text-[11px] rounded-lg border border-slate-200 transition-colors cursor-pointer"
                >
                  取消
                </button>
              </div>

              <button
                v-else
                @click="confirmingTagId = tag.id"
                class="flex items-center gap-1 px-2 py-1 text-morandi-muted hover:text-rose-600 hover:bg-rose-50 rounded-lg transition-colors cursor-pointer"
              >
                <Trash2 class="w-3.5 h-3.5" />
                <span>删除</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>


    <!-- Add Tag Modal -->
    <div
      v-if="showAddTagModal"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-900/40 backdrop-blur-md"
      @click.self="showAddTagModal = false"
    >
      <div class="bg-white rounded-2xl shadow-morandi-lg w-full max-w-md border border-morandi-borderSoft p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-3">
          <h3 class="font-bold text-base text-morandi-text flex items-center gap-1.5">
            <Plus class="w-4 h-4 text-morandi-sage" /> 新增分类 Tag 标签
          </h3>
          <button @click="showAddTagModal = false" class="p-1 text-morandi-light hover:text-morandi-text rounded-lg">
            <X class="w-5 h-5" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="font-medium text-morandi-text block mb-1">标签标识 Identifier (仅英文字母/数字)</label>
            <input
              v-model="newTagId"
              placeholder="如: anime, wallpaper, portrait"
              class="morandi-input w-full px-3 py-2 font-mono"
            />
          </div>
          <div>
            <label class="font-medium text-morandi-text block mb-1">标签显示名称 Name</label>
            <input
              v-model="newTagName"
              placeholder="如: 二次元, 壁纸, 人像"
              class="morandi-input w-full px-3 py-2"
            />
          </div>
        </div>

        <div class="flex justify-end gap-2 pt-2 border-t border-morandi-border/40">
          <button
            @click="showAddTagModal = false"
            class="px-4 py-2 text-xs font-medium text-morandi-muted hover:bg-morandi-hover rounded-xl"
          >
            取消
          </button>
          <button
            @click="handleAddTag"
            :disabled="!newTagId.trim()"
            class="px-5 py-2 text-xs font-semibold bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl shadow-sm disabled:opacity-50"
          >
            确认添加
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

