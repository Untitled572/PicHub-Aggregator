<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import type { SavedImage } from '../types'
import {
  Heart,
  Image,
  ExternalLink,
  Trash2,
  Copy,
  Check,
  ChevronLeft,
  ChevronRight,
  Clock,
  Sparkles,
  RefreshCw,
  Eye,
  X,
  HardDrive,
  FileCode
} from 'lucide-vue-next'

const { listSavedImages, unsaveImage } = useApi()

const images = ref<SavedImage[]>([])
const total = ref(0)
const currentPage = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const refreshing = ref(false)
const copiedUrl = ref<string | null>(null)

// Lightbox Preview Modal State
const previewImage = ref<SavedImage | null>(null)

const totalPages = computed(() => {
  if (total.value === 0) return 1
  return Math.ceil(total.value / pageSize.value)
})

onMounted(async () => {
  await loadPage(1)
})

async function loadPage(page: number) {
  currentPage.value = page
  loading.value = true
  try {
    const res = await listSavedImages(pageSize.value, (page - 1) * pageSize.value)
    images.value = res?.images || []
    total.value = res?.total || 0
  } catch (e) {
    console.error('Failed to load saved images:', e)
  } finally {
    loading.value = false
  }
}

async function refreshData() {
  refreshing.value = true
  await loadPage(currentPage.value)
  setTimeout(() => refreshing.value = false, 500)
}

async function changePageSize(newSize: number) {
  pageSize.value = newSize
  currentPage.value = 1
  await loadPage(1)
}

async function handleUnsave(id: number) {
  try {
    await unsaveImage(id)
    if (previewImage.value?.id === id) {
      previewImage.value = null
    }
    await loadPage(currentPage.value)
  } catch (e) {
    console.error('Failed to unsave image:', e)
  }
}

function getLocalImageUrl(fileId: string): string {
  return `/images/${fileId}`
}

function getFullLocalImageUrl(fileId: string): string {
  return `${window.location.origin}/images/${fileId}`
}

function copyUrl(text: string) {
  navigator.clipboard.writeText(text)
  copiedUrl.value = text
  setTimeout(() => copiedUrl.value = null, 2000)
}

function formatSize(bytes: number): string {
  if (!bytes || bytes <= 0) return '-'
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function formatDate(dateStr: string): string {
  if (!dateStr) return '-'
  try {
    const d = new Date(dateStr)
    return d.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return dateStr
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Top Header Bar -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white/60 p-5 rounded-2xl border border-morandi-borderSoft shadow-xs backdrop-blur-sm">
      <div class="flex items-center gap-3.5">
        <div class="w-10 h-10 rounded-xl bg-rose-50 text-rose-500 flex items-center justify-center shrink-0 shadow-xs">
          <Heart class="w-5 h-5 fill-rose-500" />
        </div>
        <div>
          <h2 class="font-bold text-base text-morandi-text flex items-center gap-2">
            离线保存图片库
          </h2>
          <p class="text-xs text-morandi-muted mt-0.5">
            已存储 <span class="font-bold text-morandi-text font-mono">{{ total }}</span> 张图片至本地磁盘缓存
          </p>
        </div>
      </div>

      <button
        @click="refreshData"
        :disabled="refreshing || loading"
        class="px-4 py-2 text-xs font-semibold bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft shadow-xs flex items-center gap-2 transition-all cursor-pointer disabled:opacity-50 shrink-0 self-start sm:self-auto"
      >
        <RefreshCw class="w-3.5 h-3.5 text-morandi-sage" :class="{ 'animate-spin': refreshing }" />
        <span>刷新保存图库</span>
      </button>
    </div>

    <!-- Loading Skeleton State -->
    <div v-if="loading" class="morandi-card p-12 text-center text-morandi-muted text-xs flex flex-col items-center justify-center gap-2">
      <RefreshCw class="w-6 h-6 animate-spin text-morandi-sage" />
      <span>正在加载离线保存图片...</span>
    </div>

    <!-- Empty State Container -->
    <div v-else-if="images.length === 0" class="morandi-card p-12 text-center space-y-3">
      <div class="w-16 h-16 mx-auto rounded-2xl bg-rose-50 text-rose-400 flex items-center justify-center mb-2 shadow-xs">
        <Heart class="w-8 h-8" />
      </div>
      <h3 class="text-sm font-bold text-morandi-text">暂无离线保存图片</h3>
      <p class="text-xs text-morandi-muted max-w-md mx-auto leading-relaxed">
        在使用统计的分发历史流水中，点击任意记录右侧的 <span class="text-rose-500 font-semibold">❤️ 保存</span> 按钮，即可将喜爱的图片离线下载并长期存储在本地磁盘。
      </p>
    </div>

    <!-- Image Grid Gallery -->
    <div v-else class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
      <div
        v-for="img in images"
        :key="img.id"
        class="morandi-card overflow-hidden group relative flex flex-col justify-between hover:shadow-md transition-all duration-300 border border-morandi-borderSoft"
      >
        <!-- Image Box with Aspect Ratio & Hover Controls -->
        <div
          @click="previewImage = img"
          class="aspect-[4/3] bg-morandi-bg overflow-hidden relative cursor-pointer group/thumb"
        >
          <img
            :src="getLocalImageUrl(img.file_id)"
            loading="lazy"
            class="w-full h-full object-cover transition-transform duration-500 group-hover/thumb:scale-108"
            @error="(e) => {
              const target = e.target as HTMLElement
              target.style.display = 'none'
            }"
          />

          <!-- Format & Resolution Badges -->
          <div class="absolute top-2 left-2 flex items-center gap-1 z-10">
            <span v-if="img.format" class="px-1.5 py-0.5 bg-black/60 backdrop-blur-md text-white text-[9px] font-mono font-bold uppercase rounded-md shadow-xs">
              {{ img.format }}
            </span>
            <span v-if="img.width && img.height" class="px-1.5 py-0.5 bg-black/40 backdrop-blur-md text-white text-[9px] font-mono rounded-md">
              {{ img.width }}×{{ img.height }}
            </span>
          </div>

          <!-- Hover Overlay & Quick Actions -->
          <div class="absolute inset-0 bg-stone-950/40 opacity-0 group-hover/thumb:opacity-100 transition-opacity duration-200 flex items-center justify-center gap-2 text-white">
            <button
              @click.stop="previewImage = img"
              class="p-2 bg-white/90 text-morandi-text rounded-xl shadow-md hover:bg-white hover:scale-105 transition-all cursor-pointer"
              title="大图预览"
            >
              <Eye class="w-4 h-4" />
            </button>
            <button
              @click.stop="handleUnsave(img.id)"
              class="p-2 bg-white/90 text-rose-500 rounded-xl shadow-md hover:bg-rose-500 hover:text-white hover:scale-105 transition-all cursor-pointer"
              title="取消保存 / 移除缓存"
            >
              <Trash2 class="w-4 h-4" />
            </button>
          </div>
        </div>

        <!-- Card Footer Meta Info -->
        <div class="p-3 space-y-2 text-xs bg-white">
          <div class="flex items-center justify-between gap-2 text-morandi-text font-semibold">
            <span class="truncate text-[11px]" :title="img.source_name">{{ img.source_name || '离线缓存图片' }}</span>
            <span class="text-[10px] font-mono text-morandi-sage-dark bg-morandi-sage-light/60 px-1.5 py-0.5 rounded font-bold shrink-0">
              {{ formatSize(img.file_size) }}
            </span>
          </div>

          <div class="flex items-center justify-between text-[10px] text-morandi-muted font-mono pt-0.5 border-t border-morandi-border/30">
            <div class="flex items-center gap-1">
              <Clock class="w-3 h-3 text-morandi-light" />
              <span>{{ formatDate(img.saved_at) }}</span>
            </div>

            <button
              @click="copyUrl(getFullLocalImageUrl(img.file_id))"
              class="flex items-center gap-1 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 px-1.5 py-0.5 rounded transition-colors cursor-pointer"
              title="复制本地缓存 URL"
            >
              <Check v-if="copiedUrl === getFullLocalImageUrl(img.file_id)" class="w-3 h-3 text-emerald-600" />
              <Copy v-else class="w-3 h-3" />
              <span>{{ copiedUrl === getFullLocalImageUrl(img.file_id) ? '已复制' : '复制' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Pagination Controls Bar -->
    <div v-if="total > 0" class="morandi-card p-4 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
      <div class="flex items-center gap-3">
        <div class="flex items-center gap-1.5 text-morandi-muted font-medium">
          <span>每页显示:</span>
          <select
            :value="pageSize"
            @change="changePageSize(Number(($event.target as HTMLSelectElement).value))"
            class="morandi-input px-2 py-1 text-xs font-mono font-bold text-morandi-text bg-white"
          >
            <option :value="10">10 条/页</option>
            <option :value="20">20 条/页 (默认)</option>
            <option :value="50">50 条/页</option>
            <option :value="100">100 条/页</option>
          </select>
        </div>
        <span class="text-morandi-muted font-mono text-[11px]">
          共 {{ total }} 张离线保存图片
        </span>
      </div>

      <div class="flex items-center gap-2 font-mono">
        <button
          @click="loadPage(currentPage - 1)"
          :disabled="currentPage <= 1 || loading"
          class="px-3 py-1.5 bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft font-semibold disabled:opacity-40 transition-all cursor-pointer flex items-center gap-1 shadow-xs"
        >
          <ChevronLeft class="w-3.5 h-3.5" /> 上一页
        </button>

        <span class="px-3 py-1.5 text-morandi-text font-bold text-xs bg-morandi-bg rounded-xl border border-morandi-borderSoft">
          {{ currentPage }} / {{ totalPages }} 页
        </span>

        <button
          @click="loadPage(currentPage + 1)"
          :disabled="currentPage >= totalPages || loading"
          class="px-3 py-1.5 bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft font-semibold disabled:opacity-40 transition-all cursor-pointer flex items-center gap-1 shadow-xs"
        >
          下一页 <ChevronRight class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- Image Lightbox Preview Modal -->
    <div
      v-if="previewImage"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-950/75 backdrop-blur-md animate-in fade-in duration-200"
      @click.self="previewImage = null"
    >
      <div class="bg-white rounded-2xl shadow-2xl max-w-3xl w-full overflow-hidden border border-morandi-borderSoft flex flex-col max-h-[90vh]">
        <!-- Lightbox Header -->
        <div class="p-4 border-b border-morandi-border/60 flex items-center justify-between bg-morandi-bg/50">
          <div>
            <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
              <Heart class="w-4 h-4 text-rose-500 fill-rose-500" /> 离线图片全屏预览
            </h3>
            <p class="text-[11px] text-morandi-muted mt-0.5">
              来源图源：<span class="font-semibold text-morandi-text">{{ previewImage.source_name || '本地缓存' }}</span>
              <template v-if="previewImage.width && previewImage.height">
                • 分辨率：<span class="font-mono">{{ previewImage.width }} × {{ previewImage.height }}</span>
              </template>
              • 文件大小：<span class="font-mono">{{ formatSize(previewImage.file_size) }}</span>
            </p>
          </div>
          <button @click="previewImage = null" class="p-1 text-morandi-light hover:text-morandi-text rounded-lg cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Lightbox Image View -->
        <div class="p-4 bg-stone-900 flex items-center justify-center min-h-[300px] max-h-[60vh] overflow-hidden relative">
          <img
            :src="getLocalImageUrl(previewImage.file_id)"
            class="max-w-full max-h-[55vh] object-contain rounded-lg shadow-lg"
          />
        </div>

        <!-- Lightbox Footer -->
        <div class="p-4 border-t border-morandi-border/60 flex items-center justify-between bg-white text-xs gap-3 flex-wrap">
          <span class="font-mono text-morandi-muted text-[11px] truncate flex-1" :title="getFullLocalImageUrl(previewImage.file_id)">
            {{ getFullLocalImageUrl(previewImage.file_id) }}
          </span>

          <div class="flex items-center gap-2 shrink-0">
            <button
              @click="handleUnsave(previewImage.id)"
              class="px-3 py-1.5 font-medium bg-rose-50 hover:bg-rose-100 text-rose-600 rounded-xl border border-rose-200 flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <Trash2 class="w-3.5 h-3.5" />
              <span>取消保存</span>
            </button>

            <button
              @click="copyUrl(getFullLocalImageUrl(previewImage.file_id))"
              class="px-3 py-1.5 font-medium bg-morandi-bg hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <Check v-if="copiedUrl === getFullLocalImageUrl(previewImage.file_id)" class="w-3.5 h-3.5 text-emerald-600" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copiedUrl === getFullLocalImageUrl(previewImage.file_id) ? '已复制本地 URL' : '复制本地 URL' }}</span>
            </button>

            <a
              :href="getLocalImageUrl(previewImage.file_id)"
              target="_blank"
              rel="noopener noreferrer"
              class="px-3.5 py-1.5 font-semibold bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl flex items-center gap-1.5 transition-colors shadow-xs"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>打开图片</span>
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
