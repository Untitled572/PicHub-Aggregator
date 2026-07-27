<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Source } from '../types'
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
  Square
} from 'lucide-vue-next'

import { useDomain } from '../composables/useDomain'

const { tags, masterBoundTags, addTag, renameTag, deleteTag, setMasterBoundTags } = useTags()
const { listSources } = useApi()
const { effectiveDomain } = useDomain()

const sources = ref<Source[]>([])

const copiedState = ref<Record<string, boolean>>({})
const showAddTagModal = ref(false)
const newTagId = ref('')
const newTagName = ref('')
const editingTagId = ref<string | null>(null)
const editingTagName = ref('')

onMounted(async () => {
  try {
    sources.value = await listSources()
  } catch {}
})

const origin = computed(() => effectiveDomain.value)


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

// Master API URL
const masterUrl = computed(() => {
  if (masterBoundTags.value.length === 0 || masterBoundTags.value.length === tags.value.length) {
    return `${origin.value}/random`
  }
  return `${origin.value}/random?category=${masterBoundTags.value.join(',')}`
})

const masterJsonUrl = computed(() => {
  if (masterBoundTags.value.length === 0 || masterBoundTags.value.length === tags.value.length) {
    return `${origin.value}/random?format=json`
  }
  return `${origin.value}/random?category=${masterBoundTags.value.join(',')}&format=json`
})

// Check if all tags bound
const isAllTagsBound = computed(() => {
  return masterBoundTags.value.length === 0 || masterBoundTags.value.length === tags.value.length
})

function toggleAllTagsBound() {
  if (isAllTagsBound.value) {
    // Uncheck all -> select empty
    setMasterBoundTags([])
  } else {
    // Check all -> select empty (means all)
    setMasterBoundTags([])
  }
}

function toggleTagBound(tagId: string) {
  let current = [...masterBoundTags.value]
  if (current.length === 0) {
    // If currently 'all', populate all except this one
    current = tags.value.map(t => t.id).filter(id => id !== tagId)
  } else {
    const idx = current.indexOf(tagId)
    if (idx >= 0) {
      current.splice(idx, 1)
    } else {
      current.push(tagId)
    }
  }
  setMasterBoundTags(current)
}

function isTagBound(tagId: string): boolean {
  if (masterBoundTags.value.length === 0) return true
  return masterBoundTags.value.includes(tagId)
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
</script>

<template>
  <div class="space-y-6">
    <!-- Header -->
    <div class="morandi-card p-5 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
      <div>
        <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
          <Link2 class="w-5 h-5 text-morandi-sage" /> 分布式 API 接口分发中心
        </h2>
        <p class="text-xs text-morandi-muted mt-0.5">管理分类 Tag 标签，配置独立接口与全局总聚合分发接口</p>
      </div>

      <button
        @click="showAddTagModal = true"
        class="flex items-center gap-1.5 px-4 py-2 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all shrink-0"
      >
        <Plus class="w-4 h-4" />
        <span>新增分类 Tag 标签</span>
      </button>
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
            {{ isAllTagsBound ? '全选 (无过滤条件)' : `已绑定 ${masterBoundTags.length} 个标签` }}
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

    <!-- Tags List & Standalone Endpoints -->
    <div class="space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="font-bold text-sm text-morandi-text flex items-center gap-1.5">
          <TagIcon class="w-4 h-4 text-morandi-sage" /> 分类 Tag 独立分发接口列表 (共 {{ tags.length }} 个)
        </h3>
        <span class="text-xs text-morandi-muted">每个 Tag 标签提供专属独占 API 分发链接</span>
      </div>

      <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
        <div
          v-for="tag in tags"
          :key="tag.id"
          class="morandi-card p-4 space-y-3 flex flex-col justify-between hover:shadow-morandi transition-all"
        >
          <div class="space-y-2">
            <!-- Tag Header & Actions -->
            <div class="flex items-center justify-between">
              <div class="flex items-center gap-2">
                <span class="px-2.5 py-0.5 bg-morandi-sage-light text-morandi-sage-dark rounded-lg font-bold text-xs border border-morandi-sage/30">
                  #{{ tag.name }}
                </span>
                <span class="font-mono text-xs text-morandi-muted">({{ tag.id }})</span>
              </div>

              <div class="flex items-center gap-1.5">
                <!-- Count badge -->
                <span class="text-[10px] px-2 py-0.5 bg-morandi-sidebar text-morandi-muted rounded-md font-medium">
                  {{ tagSourceCounts[tag.id] || 0 }} 个关联源
                </span>

                <!-- Edit / Rename -->
                <button
                  v-if="editingTagId !== tag.id"
                  @click="startRename(tag)"
                  class="p-1 text-morandi-muted hover:text-morandi-sage-dark rounded hover:bg-morandi-hover transition-colors"
                  title="重命名标签"
                >
                  <Edit3 class="w-3.5 h-3.5" />
                </button>

                <!-- Delete -->
                <button
                  @click="deleteTag(tag.id)"
                  class="p-1 text-morandi-muted hover:text-rose-600 rounded hover:bg-rose-50 transition-colors"
                  title="删除标签"
                >
                  <Trash2 class="w-3.5 h-3.5" />
                </button>
              </div>
            </div>

            <!-- Rename Input if editing -->
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

            <!-- Standalone Direct Link -->
            <div class="space-y-1 bg-morandi-bg/60 p-2.5 rounded-xl border border-morandi-borderSoft/60 text-xs">
              <div class="flex items-center justify-between text-[11px] text-morandi-muted">
                <span>直链分发接口:</span>
                <div class="flex items-center gap-2">
                  <button
                    @click="copyToClipboard(tag.id + '_direct', `${origin}/random?category=${tag.id}`)"
                    class="text-morandi-sage-dark font-medium hover:underline flex items-center gap-0.5"
                  >
                    <component :is="copiedState[tag.id + '_direct'] ? Check : Copy" class="w-3 h-3" />
                    <span>{{ copiedState[tag.id + '_direct'] ? '已复制' : '复制' }}</span>
                  </button>
                  <a
                    :href="`${origin}/random?category=${tag.id}`"
                    target="_blank"
                    class="hover:underline flex items-center gap-0.5"
                  >
                    <ExternalLink class="w-3 h-3" />
                  </a>
                </div>
              </div>
              <div class="font-mono text-[11px] text-morandi-text truncate selection:bg-morandi-sage-light">
                {{ origin }}/random?category={{ tag.id }}
              </div>
            </div>

            <!-- Standalone JSON Link -->
            <div class="space-y-1 bg-morandi-bg/60 p-2.5 rounded-xl border border-morandi-borderSoft/60 text-xs">
              <div class="flex items-center justify-between text-[11px] text-morandi-muted">
                <span>JSON 节点接口:</span>
                <div class="flex items-center gap-2">
                  <button
                    @click="copyToClipboard(tag.id + '_json', `${origin}/random?category=${tag.id}&format=json`)"
                    class="text-morandi-ocean-dark font-medium hover:underline flex items-center gap-0.5"
                  >
                    <component :is="copiedState[tag.id + '_json'] ? Check : Copy" class="w-3 h-3" />
                    <span>{{ copiedState[tag.id + '_json'] ? '已复制' : '复制' }}</span>
                  </button>
                  <a
                    :href="`${origin}/random?category=${tag.id}&format=json`"
                    target="_blank"
                    class="hover:underline flex items-center gap-0.5"
                  >
                    <ExternalLink class="w-3 h-3" />
                  </a>
                </div>
              </div>
              <div class="font-mono text-[11px] text-morandi-text truncate">
                {{ origin }}/random?category={{ tag.id }}&format=json
              </div>
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
          <button @click="showAddTagModal = false" class="text-morandi-muted hover:text-morandi-text">
            <X class="w-4 h-4" />
          </button>
        </div>

        <div class="space-y-3 text-xs">
          <div>
            <label class="font-medium text-morandi-text block mb-1">标签标识 Identifier (英文或拼音) <span class="text-rose-500">*</span></label>
            <input
              v-model="newTagId"
              placeholder="例如: wallpaper 或 bz"
              class="morandi-input w-full px-3 py-2 font-mono text-xs"
            />
          </div>

          <div>
            <label class="font-medium text-morandi-text block mb-1">中文显示名称</label>
            <input
              v-model="newTagName"
              placeholder="例如: 高清壁纸"
              class="morandi-input w-full px-3 py-2 text-xs"
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
