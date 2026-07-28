<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Source, Settings } from '../types'
import { useTags } from '../composables/useTags'
import { useApi } from '../composables/useApi'
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
  CheckSquare,
  Square,
  X
} from 'lucide-vue-next'

const { tags, addTag, renameTag, deleteTag } = useTags()
const { listSources, getSettings, updateSettings } = useApi()

const sources = ref<Source[]>([])
const copiedState = ref<Record<string, boolean>>({})
const showAddTagModal = ref(false)
const newTagId = ref('')
const newTagName = ref('')
const editingTagId = ref<string | null>(null)
const editingTagName = ref('')
const boundTags = ref<string[]>([])
const cachedSettings = ref<Settings | null>(null)

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
})

const origin = computed(() => window.location.origin)

// Count sources per tag
const tagSourceCounts = computed(() => {
  const counts: Record<string, number> = {}
  for (const t of tags.value) {
    counts[t.id] = 0
  }
  for (const src of sources.value) {
    if (src.categories) {
      for (const cat of src.categories) {
        counts[cat] = (counts[cat] || 0) + 1
      }
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
  return boundTags.value.length === 0 || boundTags.value.length === tags.value.length
})

async function saveBoundTags(tags: string[]) {
  const base = cachedSettings.value || { proxy_mode: false, cache_max_mb: 200, cache_ttl: 60, min_resolution: '640x480', rate_limit: 60, timeout: 3000 }
  try {
    const updated = await updateSettings({ ...base, bound_tags: tags })
    cachedSettings.value = updated
  } catch {}
}

function toggleAllTagsBound() {
  boundTags.value = []
  saveBoundTags([])
}

function toggleTagBound(tagId: string) {
  let current = [...boundTags.value]
  if (current.length === 0) {
    current = tags.value.map(t => t.id).filter(id => id !== tagId)
  } else {
    const idx = current.indexOf(tagId)
    if (idx >= 0) {
      current.splice(idx, 1)
    } else {
      current.push(tagId)
    }
  }
  boundTags.value = current
  saveBoundTags(current)
}

function isTagBound(tagId: string): boolean {
  if (boundTags.value.length === 0) return true
  return boundTags.value.includes(tagId)
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
    <div class="morandi-card p-6 space-y-4 border-l-4 border-l-morandi-sage">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-2">
          <div class="w-8 h-8 rounded-lg bg-morandi-sage/15 text-morandi-sage-dark flex items-center justify-center">
            <Sparkles class="w-4 h-4" />
          </div>
          <div>
            <h3 class="font-bold text-sm text-morandi-text">主聚合分发接口 (Master Endpoint)</h3>
            <p class="text-[11px] text-morandi-muted">统一调用的主服务接口，支持绑定绑定特定分类 Tags</p>
          </div>
        </div>
        <span class="text-[10px] px-2 py-0.5 bg-emerald-50 text-emerald-700 border border-emerald-200 rounded-full font-semibold">
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
              class="flex items-center gap-1 text-xs font-medium text-morandi-sage-dark hover:underline"
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
              class="flex items-center gap-1 text-xs font-medium text-morandi-ocean-dark hover:underline"
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
        <div class="flex items-center justify-between">
          <label class="text-xs font-bold text-morandi-text flex items-center gap-1.5">
            <Sliders class="w-3.5 h-3.5 text-morandi-sage" /> 主接口绑定的 Tag 标签范围：
          </label>
          <span class="text-[11px] text-morandi-muted">
            {{ isAllTagsBound ? '全选 (无过滤条件)' : `已绑定 ${boundTags.length} 个标签` }}
          </span>
        </div>

        <div class="flex flex-wrap gap-2 pt-1">
          <!-- All Tags Toggle -->
          <button
            type="button"
            @click="toggleAllTagsBound"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-all border"
            :class="isAllTagsBound
              ? 'bg-morandi-sage text-white border-morandi-sage shadow-xs'
              : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
          >
            <component :is="isAllTagsBound ? CheckSquare : Square" class="w-3.5 h-3.5" />
            <span>全部 Tags 标签</span>
          </button>

          <!-- Individual Tags -->
          <button
            v-for="t in tags"
            :key="t.id"
            type="button"
            @click="toggleTagBound(t.id)"
            class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium transition-all border"
            :class="isTagBound(t.id)
              ? 'bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/40 font-semibold'
              : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover opacity-60'"
          >
            <component :is="isTagBound(t.id) ? CheckSquare : Square" class="w-3.5 h-3.5 text-morandi-sage" />
            <span>#{{ t.name }} ({{ t.id }})</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Pure Tag Management Section -->
    <div class="morandi-card p-6 space-y-4">
      <div class="flex items-center justify-between">
        <div>
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-1.5">
            <TagIcon class="w-4 h-4 text-morandi-sage" /> Tag 标签库管理 (共 {{ tags.length }} 个)
          </h3>
          <p class="text-xs text-morandi-muted mt-0.5">可在此增加、修改或删除系统 Tag 标签，关联到对应的图源或主分发范围</p>
        </div>

        <button
          @click="showAddTagModal = true"
          class="flex items-center gap-1.5 px-3.5 py-1.5 bg-morandi-sidebar hover:bg-morandi-hover border border-morandi-borderSoft text-morandi-text rounded-xl text-xs font-medium transition-colors"
        >
          <Plus class="w-3.5 h-3.5 text-morandi-sage" />
          <span>添加标签</span>
        </button>
      </div>

      <!-- Tag Grid Cards -->
      <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-3 pt-2">
        <div
          v-for="tag in tags"
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
              class="px-2.5 py-1 bg-morandi-sage text-white rounded-lg text-xs font-semibold"
            >
              保存
            </button>
            <button
              @click="editingTagId = null"
              class="px-2 py-1 text-morandi-muted hover:bg-morandi-hover rounded-lg text-xs"
            >
              取消
            </button>
          </div>

          <div v-else class="flex items-center justify-end gap-2 pt-1 border-t border-morandi-border/30 text-xs">
            <button
              @click="startRename(tag)"
              class="flex items-center gap-1 px-2 py-1 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/50 rounded-lg transition-colors"
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

