<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useApi } from '../composables/useApi'
import { useTags } from '../composables/useTags'
import type { StatsResponse, ImageHistoryRecord, Settings } from '../types'
import {
  TrendingUp,
  Activity,
  Tag as TagIcon,
  Sparkles,
  RefreshCw,
  Clock,
  Globe,
  Copy,
  Check,
  ExternalLink,
  Search,
  Eye,
  X,
  Layers,
  Calendar,
  ChevronLeft,
  ChevronRight,
  LineChart,
  ThumbsUp,
  ThumbsDown
} from 'lucide-vue-next'

const { getStats, getImageHistory, getSettings, saveImage, unsaveImage, likeImage, dislikeImage, listSavedImages } = useApi()
const { getCategoryMap } = useTags()
const categoryMap = computed(() => getCategoryMap())

const loading = ref(false)
const refreshing = ref(false)
const stats = ref<StatsResponse | null>(null)
const settings = ref<Settings | null>(null)

// History Pagination State
const history = ref<ImageHistoryRecord[]>([])
const savedFileIDs = ref(new Set<string>())
const savingImageId = ref<number | null>(null)
const likedIds = ref(new Set<number>())
const likingId = ref<number | null>(null)
const dislikedIds = ref(new Set<number>())
const dislikingId = ref<number | null>(null)

const historyLoadingMore = ref(false)
const historySearch = ref('')
const currentPage = ref(1)
const pageSize = ref(20) // Default 20 per page
const totalHistoryCount = ref(0)

const activeRange = ref<'today' | '7d' | '30d' | 'all' | 'custom'>('7d') // Default 7d for nice charts
const customStartDate = ref('')
const customEndDate = ref('')

const copiedUrl = ref<string | null>(null)
const previewImage = ref<{ url: string; sourceName: string; time: string } | null>(null)

// SVG Chart Hover States
const hoveredDailyPoint = ref<{ x: number; y: number; date: string; total: number } | null>(null)
const hoveredSourcePoint = ref<{ x: number; y: number; date: string; sourceName: string; count: number } | null>(null)

const rangeOptions = [
  { id: 'today', label: '今日' },
  { id: '7d', label: '近 7 天' },
  { id: '30d', label: '近 30 天' },
  { id: 'all', label: '全部历史' },
  { id: 'custom', label: '自定义日期' },
]

const rangeSubtext = computed(() => {
  switch (activeRange.value) {
    case 'today': return '今日请求数'
    case '7d': return '近 7 天请求数'
    case '30d': return '近 30 天请求数'
    case 'all': return '全部历史请求数'
    case 'custom': return '自定义区间请求数'
    default: return '选定区间请求数'
  }
})

const totalPages = computed(() => {
  if (totalHistoryCount.value === 0) return 1
  return Math.ceil(totalHistoryCount.value / pageSize.value)
})

onMounted(async () => {
  await loadAllData()
})

async function loadAllData() {
  loading.value = true
  try {
    const range = activeRange.value === 'custom' ? '' : activeRange.value
    const sDate = activeRange.value === 'custom' ? customStartDate.value : ''
    const eDate = activeRange.value === 'custom' ? customEndDate.value : ''

    const [statsRes, historyRes, settingsRes] = await Promise.all([
      getStats(range, sDate, eDate),
      getImageHistory(pageSize.value, (currentPage.value - 1) * pageSize.value),
      getSettings().catch(() => null)
    ])
    stats.value = statsRes
    history.value = historyRes?.history || []
    totalHistoryCount.value = historyRes?.total || 0
    if (settingsRes) settings.value = settingsRes

    for (const item of history.value) {
      if (item.is_saved) {
        if (item.file_id) savedFileIDs.value.add(item.file_id)
        if (item.image_id) savedKeys.value.add(item.image_id)
        if (item.id) savedKeys.value.add(item.id)
      }
    }
    savedKeys.value = new Set(savedKeys.value)
    savedFileIDs.value = new Set(savedFileIDs.value)

    const savedRes = await listSavedImages(1000, 0).catch(() => null)
    if (savedRes?.images) {
      for (const img of savedRes.images) {
        if (img.file_id) savedFileIDs.value.add(img.file_id)
      }
      savedFileIDs.value = new Set(savedFileIDs.value)
    }


  } catch (e) {
    console.error('Failed to load stats:', e)
  } finally {
    loading.value = false
  }
}


async function fetchHistoryPage(page = 1) {
  currentPage.value = page
  const offset = (page - 1) * pageSize.value
  historyLoadingMore.value = true
  try {
    const res = await getImageHistory(pageSize.value, offset)
    history.value = res?.history || []
    totalHistoryCount.value = res?.total || 0
  } catch (e) {
    console.error('Failed to fetch history page:', e)
  } finally {
    historyLoadingMore.value = false
  }
}

async function changePageSize(newSize: number) {
  pageSize.value = newSize
  currentPage.value = 1
  await fetchHistoryPage(1)
}

function prevPage() {
  if (currentPage.value > 1) {
    fetchHistoryPage(currentPage.value - 1)
  }
}

function nextPage() {
  if (currentPage.value < totalPages.value) {
    fetchHistoryPage(currentPage.value + 1)
  }
}

async function changeRange(range: 'today' | '7d' | '30d' | 'all' | 'custom') {
  activeRange.value = range
  if (range !== 'custom') {
    loading.value = true
    try {
      stats.value = await getStats(range)
    } catch (e) {
      console.error(e)
    } finally {
      loading.value = false
    }
  }
}

async function applyCustomDateRange() {
  if (!customStartDate.value || !customEndDate.value) return
  loading.value = true
  try {
    stats.value = await getStats('', customStartDate.value, customEndDate.value)
  } catch (e) {
    console.error(e)
  } finally {
    loading.value = false
  }
}

async function refreshData() {
  refreshing.value = true
  await loadAllData()
  setTimeout(() => refreshing.value = false, 500)
}

function copyToClipboard(text: string) {
  navigator.clipboard.writeText(text)
  copiedUrl.value = text
  setTimeout(() => copiedUrl.value = null, 2000)
}

function parseCategories(catStr: string): string[] {
  if (!catStr) return []
  try {
    const parsed = JSON.parse(catStr)
    if (Array.isArray(parsed)) {
      return parsed.filter(c => c && c !== '__uncategorized__')
    }
  } catch {}
  return catStr === '__uncategorized__' ? [] : [catStr]
}

function thumbnailUrl(item: ImageHistoryRecord): string {
  if (item.file_id) return `/images/${item.file_id}`
  if (item.image_id && item.image_id > 0) return `/images/${item.image_id}`
  return item.image_url
}

const savedKeys = ref(new Set<string | number>())

function isItemSaved(item: ImageHistoryRecord): boolean {
  if (!item) return false
  if (item.is_saved) return true
  if (item.file_id && savedFileIDs.value.has(item.file_id)) return true
  if (item.image_id && savedKeys.value.has(item.image_id)) return true
  if (item.id && savedKeys.value.has(item.id)) return true
  return false
}

async function toggleSaveImage(item: ImageHistoryRecord) {
  const targetId = item.file_id || item.image_id || item.id
  if (!targetId) return
  savingImageId.value = typeof targetId === 'number' ? targetId : 9999
  try {
    const currentlySaved = isItemSaved(item)
    if (currentlySaved) {
      await unsaveImage(targetId)
      if (item.image_id) savedKeys.value.delete(item.image_id)
      if (item.file_id) savedKeys.value.delete(item.file_id)
      if (item.id) savedKeys.value.delete(item.id)
      item.is_saved = false
    } else {
      await saveImage(targetId)
      if (item.image_id) savedKeys.value.add(item.image_id)
      if (item.file_id) savedKeys.value.add(item.file_id)
      if (item.id) savedKeys.value.add(item.id)
      item.is_saved = true
    }
    savedKeys.value = new Set(savedKeys.value)
  } catch (e) {
    console.error('Failed to toggle save image:', e)
  }
  savingImageId.value = null
}



async function handleLike(item: ImageHistoryRecord) {
  const targetId = item.image_id || item.source_id
  if (!targetId || likingId.value === targetId) return
  likingId.value = targetId
  try {
    await likeImage(targetId)
    likedIds.value.add(targetId)
    likedIds.value = new Set(likedIds.value)
    dislikedIds.value.delete(targetId)
    dislikedIds.value = new Set(dislikedIds.value)
  } catch {}
  likingId.value = null
}

async function handleDislike(item: ImageHistoryRecord) {
  const targetId = item.image_id || item.source_id
  if (!targetId || dislikingId.value === targetId) return
  dislikingId.value = targetId
  try {
    await dislikeImage(targetId)
    dislikedIds.value.add(targetId)
    dislikedIds.value = new Set(dislikedIds.value)
    likedIds.value.delete(targetId)
    likedIds.value = new Set(likedIds.value)
  } catch {}
  dislikingId.value = null
}




function formatDate(isoStr: string): string {
  if (!isoStr) return '-'
  try {
    const d = new Date(isoStr)
    return d.toLocaleString('zh-CN', {
      timeZone: 'Asia/Shanghai',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit'
    })
  } catch {
    return isoStr
  }
}

// Filter out internal hidden tag __uncategorized__ from frontend stats!
const validTags = computed(() => {
  if (!stats.value?.today?.tags) return []
  return stats.value.today.tags.filter(t => t.tag_id && t.tag_id !== '__uncategorized__')
})

const topTag = computed(() => {
  if (validTags.value.length === 0) return null
  return validTags.value[0]
})

const topSource = computed(() => {
  if (!stats.value?.today?.sources || stats.value.today.sources.length === 0) return null
  return stats.value.today.sources[0]
})

const filteredHistory = computed(() => {
  if (!historySearch.value.trim()) return history.value
  const q = historySearch.value.toLowerCase()
  return history.value.filter(h =>
    h.source_name.toLowerCase().includes(q) ||
    h.image_url.toLowerCase().includes(q) ||
    h.categories.toLowerCase().includes(q)
  )
})

const tagColors = ['bg-morandi-sage', 'bg-morandi-ocean', 'bg-morandi-sand', 'bg-slate-600', 'bg-zinc-500']

// --- SVG Chart Calculations ---
const chartWidth = 600
const chartHeight = 180
const padding = { top: 20, right: 20, bottom: 30, left: 40 }

const dailyTrendData = computed(() => {
  return stats.value?.today?.daily_trends || []
})

const dailyChartPoints = computed(() => {
  const data = dailyTrendData.value
  if (!data || data.length === 0) return []
  const maxVal = Math.max(...data.map(d => d.total), 1)
  const innerW = chartWidth - padding.left - padding.right
  const innerH = chartHeight - padding.top - padding.bottom

  return data.map((item, idx) => {
    const x = padding.left + (data.length > 1 ? (idx / (data.length - 1)) * innerW : innerW / 2)
    const y = chartHeight - padding.bottom - (item.total / maxVal) * innerH
    return { x, y, date: item.date, total: item.total }
  })
})

const dailyLinePath = computed(() => {
  const points = dailyChartPoints.value
  if (points.length === 0) return ''
  return points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
})

const dailyAreaPath = computed(() => {
  const points = dailyChartPoints.value
  if (points.length === 0) return ''
  const lineD = dailyLinePath.value
  const lastP = points[points.length - 1]
  const firstP = points[0]
  return `${lineD} L ${lastP.x} ${chartHeight - padding.bottom} L ${firstP.x} ${chartHeight - padding.bottom} Z`
})

// Source Hit Multi-Line Chart
const sourceColors = ['#10B981', '#3B82F6', '#F59E0B', '#EC4899', '#8B5CF6']

const topSourcesList = computed(() => {
  const raw = stats.value?.today?.source_trends || []
  const nameSet = new Set<string>()
  raw.forEach(r => nameSet.add(r.source_name))
  return Array.from(nameSet).slice(0, 5)
})

const sourceTrendLines = computed(() => {
  const raw = stats.value?.today?.source_trends || []
  const daily = dailyTrendData.value
  if (daily.length === 0 || topSourcesList.value.length === 0) return []

  const dates = daily.map(d => d.date)
  const maxHit = Math.max(...raw.map(r => r.hit_count), 1)

  const innerW = chartWidth - padding.left - padding.right
  const innerH = chartHeight - padding.top - padding.bottom

  return topSourcesList.value.map((srcName, sIdx) => {
    const color = sourceColors[sIdx % sourceColors.length]
    const srcRecords = raw.filter(r => r.source_name === srcName)
    const recordMap = new Map<string, number>()
    srcRecords.forEach(r => recordMap.set(r.date, r.hit_count))

    const points = dates.map((dateStr, dIdx) => {
      const count = recordMap.get(dateStr) || 0
      const x = padding.left + (dates.length > 1 ? (dIdx / (dates.length - 1)) * innerW : innerW / 2)
      const y = chartHeight - padding.bottom - (count / maxHit) * innerH
      return { x, y, date: dateStr, sourceName: srcName, count }
    })

    const linePath = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x} ${p.y}`).join(' ')
    return { name: srcName, color, points, linePath }
  })
})
</script>

<template>
  <div class="space-y-6">
    <!-- Top Header & Controls -->
    <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-white/60 p-5 rounded-2xl border border-morandi-borderSoft shadow-xs backdrop-blur-sm">
      <div>
        <h2 class="font-bold text-lg text-morandi-text flex items-center gap-2">
          <TrendingUp class="w-5 h-5 text-morandi-sage" />
          使用统计与调取历史
        </h2>
        <p class="text-xs text-morandi-muted mt-1">
          实时监控图片分发热度、分类 Tag 命中占比、图源排行榜及分发历史记录
        </p>
      </div>

      <button
        @click="refreshData"
        :disabled="refreshing || loading"
        class="px-4 py-2 text-xs font-semibold bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft shadow-xs flex items-center gap-2 transition-all cursor-pointer disabled:opacity-50 shrink-0 self-start sm:self-auto"
      >
        <RefreshCw class="w-3.5 h-3.5 text-morandi-sage" :class="{ 'animate-spin': refreshing }" />
        <span>刷新统计数据</span>
      </button>
    </div>

    <!-- Time Range Selector Bar -->
    <div class="flex flex-wrap items-center justify-between gap-3 bg-white p-3.5 rounded-2xl border border-morandi-borderSoft shadow-xs">
      <div class="flex items-center gap-1.5 flex-wrap">
        <span class="text-xs font-semibold text-morandi-muted mr-1.5 flex items-center gap-1">
          <Calendar class="w-3.5 h-3.5 text-morandi-sage" /> 统计时间范围:
        </span>
        <button
          v-for="r in rangeOptions"
          :key="r.id"
          @click="changeRange(r.id as any)"
          class="px-3 py-1.5 rounded-xl text-xs font-medium transition-all cursor-pointer"
          :class="activeRange === r.id
            ? 'bg-morandi-sage text-white font-semibold shadow-xs'
            : 'bg-morandi-bg text-morandi-muted hover:bg-morandi-hover hover:text-morandi-text'"
        >
          {{ r.label }}
        </button>
      </div>

      <!-- Custom Date Inputs -->
      <div v-if="activeRange === 'custom'" class="flex items-center gap-2 text-xs flex-wrap">
        <input
          v-model="customStartDate"
          type="date"
          class="morandi-input px-2.5 py-1 text-xs font-mono"
        />
        <span class="text-morandi-muted font-bold">至</span>
        <input
          v-model="customEndDate"
          type="date"
          class="morandi-input px-2.5 py-1 text-xs font-mono"
        />
        <button
          @click="applyCustomDateRange"
          :disabled="!customStartDate || !customEndDate"
          class="px-3 py-1 bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-lg font-semibold text-xs transition-colors disabled:opacity-50 cursor-pointer"
        >
          查询
        </button>
      </div>

      <!-- Active Range Explanation Badge -->
      <div v-else-if="stats?.start_date" class="text-[11px] text-morandi-muted font-mono flex items-center gap-1 bg-morandi-bg px-2.5 py-1 rounded-lg">
        <Clock class="w-3 h-3 text-morandi-sage" />
        <span>区间：{{ stats.start_date }} 至 {{ stats.end_date }}</span>
      </div>
    </div>

    <!-- 1. Key Metric Cards Grid -->
    <div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
      <!-- Total Requests All Time -->
      <div class="morandi-card p-4 flex items-center gap-4">
        <div class="w-12 h-12 rounded-2xl bg-morandi-sage-light text-morandi-sage-dark flex items-center justify-center shrink-0">
          <TrendingUp class="w-6 h-6" />
        </div>
        <div class="space-y-0.5">
          <div class="text-xs text-morandi-muted font-medium">历史总调用次数</div>
          <div class="text-2xl font-extrabold text-morandi-text tracking-tight font-mono">
            {{ stats?.total.total_requests.toLocaleString() || 0 }}
          </div>
          <div class="text-[10px] text-morandi-sage font-medium flex items-center gap-1">
            <span>全站累计分发</span>
          </div>
        </div>
      </div>

      <!-- Selected Range Requests -->
      <div class="morandi-card p-4 flex items-center gap-4">
        <div class="w-12 h-12 rounded-2xl bg-morandi-ocean-light text-morandi-ocean-dark flex items-center justify-center shrink-0">
          <Activity class="w-6 h-6" />
        </div>
        <div class="space-y-0.5">
          <div class="text-xs text-morandi-muted font-medium">选定区间请求数</div>
          <div class="text-2xl font-extrabold text-morandi-text tracking-tight font-mono">
            {{ stats?.today.total.toLocaleString() || 0 }}
          </div>
          <div class="text-[10px] text-morandi-ocean font-medium flex items-center gap-1">
            <span>{{ rangeSubtext }}</span>
          </div>
        </div>
      </div>

      <!-- Top Tag in Selected Range -->
      <div class="morandi-card p-4 flex items-center gap-4">
        <div class="w-12 h-12 rounded-2xl bg-morandi-sand-light text-morandi-sand-dark flex items-center justify-center shrink-0">
          <TagIcon class="w-6 h-6" />
        </div>
        <div class="space-y-0.5">
          <div class="text-xs text-morandi-muted font-medium">区间最热分类 Tag</div>
          <div class="text-lg font-bold text-morandi-text truncate max-w-[140px]">
            {{ topTag ? `#${categoryMap[topTag.tag_id] || topTag.tag_id}` : '无数据' }}
          </div>
          <div class="text-[10px] text-morandi-sand-dark font-medium font-mono">
            {{ topTag ? `${topTag.count} 次请求` : '等待分发' }}
          </div>
        </div>
      </div>

      <!-- Top Source in Selected Range -->
      <div class="morandi-card p-4 flex items-center gap-4">
        <div class="w-12 h-12 rounded-2xl bg-rose-50 text-rose-600 flex items-center justify-center shrink-0">
          <Sparkles class="w-6 h-6" />
        </div>
        <div class="space-y-0.5">
          <div class="text-xs text-morandi-muted font-medium">区间最高命中图源</div>
          <div class="text-lg font-bold text-morandi-text truncate max-w-[140px]" :title="topSource?.source_name">
            {{ topSource ? topSource.source_name : '无数据' }}
          </div>
          <div class="text-[10px] text-rose-600 font-medium font-mono">
            {{ topSource ? `${topSource.hit_count} 次命中` : '等待分发' }}
          </div>
        </div>
      </div>
    </div>

    <!-- 2. Line Charts Section (NEW) -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Daily Requests Trend Line Chart -->
      <div class="morandi-card p-5 space-y-3">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-2.5">
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
            <LineChart class="w-4 h-4 text-morandi-sage" />
            每日分发总数趋势折线图
          </h3>
          <span class="text-xs text-morandi-muted font-mono font-medium">
            {{ dailyTrendData.length }} 天数据
          </span>
        </div>

        <div v-if="dailyChartPoints.length > 0" class="relative w-full overflow-hidden">
          <svg viewBox="0 0 600 180" class="w-full h-44 overflow-visible">
            <defs>
              <linearGradient id="sageGradient" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stop-color="#4FC08D" stop-opacity="0.35" />
                <stop offset="100%" stop-color="#4FC08D" stop-opacity="0.0" />
              </linearGradient>
            </defs>

            <!-- Horizontal Grid lines -->
            <line x1="40" y1="20" x2="580" y2="20" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="75" x2="580" y2="75" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="130" x2="580" y2="130" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="150" x2="580" y2="150" stroke="#d1d5db" />

            <!-- Gradient area & line -->
            <path :d="dailyAreaPath" fill="url(#sageGradient)" />
            <path :d="dailyLinePath" fill="none" stroke="#4FC08D" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />

            <!-- Data circles & hover hits -->
            <g v-for="pt in dailyChartPoints" :key="pt.date">
              <circle
                :cx="pt.x"
                :cy="pt.y"
                r="4"
                fill="#ffffff"
                stroke="#4FC08D"
                stroke-width="2.5"
                class="transition-all hover:r-6 cursor-pointer"
                @mouseenter="hoveredDailyPoint = pt"
                @mouseleave="hoveredDailyPoint = null"
              />
              <!-- Date Label at bottom -->
              <text :x="pt.x" y="166" text-anchor="middle" font-size="9" fill="#9ca3af" font-family="monospace">
                {{ pt.date.slice(5) }}
              </text>
            </g>

            <!-- Active Hover Guide Line & Tooltip -->
            <g v-if="hoveredDailyPoint">
              <line :x1="hoveredDailyPoint.x" y1="20" :x2="hoveredDailyPoint.x" y2="150" stroke="#4FC08D" stroke-dasharray="2 2" />
              <circle :cx="hoveredDailyPoint.x" :cy="hoveredDailyPoint.y" r="6" fill="#4FC08D" />
            </g>
          </svg>

          <!-- Floating Tooltip -->
          <div
            v-if="hoveredDailyPoint"
            class="absolute top-2 right-4 bg-stone-900/90 text-white text-[11px] font-mono px-3 py-1.5 rounded-lg shadow-md border border-stone-700 pointer-events-none"
          >
            <span>{{ hoveredDailyPoint.date }}</span>：<span class="font-bold text-emerald-400">{{ hoveredDailyPoint.total }} 次请求</span>
          </div>
        </div>

        <div v-else class="text-center py-10 text-morandi-muted text-xs bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
          选定时间范围内暂无趋势折线数据
        </div>
      </div>

      <!-- Top Sources Daily Hit Trend Line Chart -->
      <div class="morandi-card p-5 space-y-3">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-2.5">
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-morandi-ocean" />
            热门图源每日命中趋势折线图
          </h3>
          <span class="text-xs text-morandi-muted font-mono font-medium">
            Top {{ topSourcesList.length }} 图源
          </span>
        </div>

        <!-- Legend Badges -->
        <div v-if="topSourcesList.length > 0" class="flex flex-wrap items-center gap-2 pb-1 text-[11px]">
          <div v-for="(line, idx) in sourceTrendLines" :key="line.name" class="flex items-center gap-1.5 font-medium text-morandi-text bg-morandi-bg/80 px-2 py-0.5 rounded-md border border-morandi-borderSoft">
            <span class="w-2.5 h-2.5 rounded-full" :style="{ backgroundColor: line.color }"></span>
            <span class="truncate max-w-[100px]">{{ line.name }}</span>
          </div>
        </div>

        <div v-if="sourceTrendLines.length > 0" class="relative w-full overflow-hidden">
          <svg viewBox="0 0 600 180" class="w-full h-44 overflow-visible">
            <!-- Grid lines -->
            <line x1="40" y1="20" x2="580" y2="20" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="75" x2="580" y2="75" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="130" x2="580" y2="130" stroke="#e5e7eb" stroke-dasharray="3 3" />
            <line x1="40" y1="150" x2="580" y2="150" stroke="#d1d5db" />

            <!-- Multi-Source Trend Lines -->
            <g v-for="line in sourceTrendLines" :key="line.name">
              <path :d="line.linePath" fill="none" :stroke="line.color" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" />
              <g v-for="pt in line.points" :key="pt.date">
                <circle
                  :cx="pt.x"
                  :cy="pt.y"
                  r="3.5"
                  fill="#ffffff"
                  :stroke="line.color"
                  stroke-width="2"
                  class="transition-all hover:r-5 cursor-pointer"
                  @mouseenter="hoveredSourcePoint = pt"
                  @mouseleave="hoveredSourcePoint = null"
                />
              </g>
            </g>

            <!-- X-axis Date Labels -->
            <g v-if="sourceTrendLines[0]">
              <text v-for="pt in sourceTrendLines[0].points" :key="pt.date" :x="pt.x" y="166" text-anchor="middle" font-size="9" fill="#9ca3af" font-family="monospace">
                {{ pt.date.slice(5) }}
              </text>
            </g>

            <!-- Active Hover Circle -->
            <g v-if="hoveredSourcePoint">
              <circle :cx="hoveredSourcePoint.x" :cy="hoveredSourcePoint.y" r="5.5" fill="#3B82F6" />
            </g>
          </svg>

          <!-- Floating Tooltip -->
          <div
            v-if="hoveredSourcePoint"
            class="absolute top-2 right-4 bg-stone-900/90 text-white text-[11px] font-mono px-3 py-1.5 rounded-lg shadow-md border border-stone-700 pointer-events-none"
          >
            <span>{{ hoveredSourcePoint.sourceName }}</span> ({{ hoveredSourcePoint.date }})：<span class="font-bold text-sky-400">{{ hoveredSourcePoint.count }} 次命中</span>
          </div>
        </div>

        <div v-else class="text-center py-10 text-morandi-muted text-xs bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
          选定时间范围内暂无图源命中趋势数据
        </div>
      </div>
    </div>

    <!-- 3. Tag Distribution & Leaderboard Grid -->
    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
      <!-- Tag Hit Distribution -->
      <div class="morandi-card p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-3">
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
            <TagIcon class="w-4 h-4 text-morandi-sage" /> 分类 Tag 分发比例
          </h3>
          <span class="text-xs text-morandi-muted font-mono font-medium">
            共 {{ validTags.length }} 个关联 Tag
          </span>
        </div>

        <div v-if="validTags.length > 0" class="space-y-3.5 max-h-96 overflow-y-auto pr-1.5">
          <div v-for="(t, idx) in validTags" :key="t.tag_id" class="space-y-1.5">
            <div class="flex items-center justify-between text-xs">
              <span class="font-semibold text-morandi-text flex items-center gap-1.5">
                <span class="w-2 h-2 rounded-full" :class="tagColors[idx % tagColors.length]"></span>
                #{{ categoryMap[t.tag_id] || t.tag_id }}
              </span>
              <span class="font-mono text-morandi-muted font-medium">
                {{ t.count }} 次 ({{ (stats?.today.total || 0) > 0 ? ((t.count / stats!.today.total) * 100).toFixed(1) : 0 }}%)
              </span>
            </div>
            <!-- Progress Bar -->
            <div class="h-2 w-full bg-morandi-bg rounded-full overflow-hidden">
              <div
                class="h-full rounded-full transition-all duration-500"
                :class="tagColors[idx % tagColors.length]"
                :style="{ width: `${(stats?.today.total || 0) > 0 ? (t.count / stats!.today.total) * 100 : 0}%` }"
              ></div>
            </div>
          </div>
        </div>

        <div v-else class="text-center py-12 text-morandi-muted text-xs bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
          选定时间范围内尚无关联 Tag 请求分发数据
        </div>
      </div>

      <!-- Source Hit Leaderboard -->
      <div class="morandi-card p-5 space-y-4">
        <div class="flex items-center justify-between border-b border-morandi-border/60 pb-3">
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
            <Sparkles class="w-4 h-4 text-morandi-ocean" /> 图源热度排行榜
          </h3>
          <span class="text-xs text-morandi-muted font-mono font-medium">
            共 {{ stats?.today.sources.length || 0 }} 个命中图源
          </span>
        </div>

        <div v-if="stats?.today.sources && stats.today.sources.length > 0" class="space-y-2.5 max-h-36 overflow-y-auto pr-1.5">
          <div
            v-for="(s, idx) in stats.today.sources"
            :key="s.source_id"
            class="p-2.5 bg-morandi-bg/60 rounded-xl border border-morandi-borderSoft flex items-center justify-between gap-3 text-xs"
          >
            <div class="flex items-center gap-2.5 min-w-0">
              <span
                class="w-6 h-6 rounded-lg font-bold font-mono text-[11px] flex items-center justify-center shrink-0"
                :class="idx === 0 ? 'bg-amber-400 text-white shadow-xs' : idx === 1 ? 'bg-slate-400 text-white' : idx === 2 ? 'bg-amber-600 text-white' : 'bg-morandi-border/60 text-morandi-muted'"
              >
                {{ idx + 1 }}
              </span>
              <span class="font-bold text-morandi-text truncate">{{ s.source_name }}</span>
            </div>

            <div class="flex items-center gap-3 shrink-0">
              <div class="text-right">
                <span class="font-mono font-bold text-morandi-text">{{ s.hit_count }}</span>
                <span class="text-[10px] text-morandi-muted ml-1">次</span>
              </div>
              <span class="px-2 py-0.5 bg-white text-morandi-sage-dark font-mono text-[10px] font-bold rounded-md border border-morandi-borderSoft">
                {{ stats.today.total > 0 ? ((s.hit_count / stats.today.total) * 100).toFixed(1) : 0 }}%
              </span>
            </div>
          </div>
        </div>


        <div v-else class="text-center py-12 text-morandi-muted text-xs bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
          选定时间范围内尚无图源命中数据
        </div>
      </div>
    </div>

    <!-- 4. Image History Log Feed -->
    <div class="morandi-card p-5 space-y-4">
      <div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-morandi-border/60 pb-3">
        <div>
          <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
            <History class="w-4 h-4 text-morandi-sage" /> 近期分发图片历史
          </h3>
          <p class="text-xs text-morandi-muted mt-0.5">控制台近 {{ history.length }} 次分发的图片历史日志</p>
        </div>

        <div class="flex items-center gap-2">
          <!-- History Search Input -->
          <div class="relative w-full sm:w-60">
            <Search class="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-morandi-light" />
            <input
              v-model="historySearch"
              placeholder="搜索图源名称或图片 URL..."
              class="morandi-input w-full pl-8 pr-3 py-1.5 text-xs font-mono"
            />
          </div>
        </div>
      </div>

      <!-- History Table / Feed -->
      <div v-if="filteredHistory.length > 0" class="overflow-x-auto">
        <table class="w-full text-left text-xs">
          <thead>
            <tr class="border-b border-morandi-border/60 text-morandi-muted font-medium text-[11px]">
              <th class="py-2 font-medium w-12">#</th>
              <th class="py-2 font-medium w-24">预览</th>
              <th class="py-2 font-medium">调取时间</th>
              <th class="py-2 font-medium">命中图源</th>
              <th class="py-2 font-medium">请求 Tag</th>
              <th class="py-2 font-medium text-right pr-2">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-morandi-border/30">
            <tr
              v-for="(item, idx) in filteredHistory"
              :key="item.id || idx"
              class="hover:bg-morandi-bg/40 transition-colors"
            >
              <td class="py-2.5 font-mono text-morandi-muted text-[11px]">
                {{ idx + 1 }}
              </td>

              <!-- Preview Thumbnail -->
              <td class="py-2.5">
                <div
                  v-if="item.file_id || (item.image_id && item.image_id > 0)"
                  class="w-10 h-10 rounded-lg overflow-hidden bg-morandi-bg border border-morandi-borderSoft relative cursor-pointer group/thumb shadow-xs"
                  @click="previewImage = { url: thumbnailUrl(item), sourceName: item.source_name, time: formatDate(item.created_at) }"
                >
                  <img
                    :src="thumbnailUrl(item)"
                    loading="lazy"
                    class="w-full h-full object-cover transition-transform duration-300 group-hover/thumb:scale-110"
                    @error="(e) => (e.target as HTMLElement).style.display = 'none'"
                  />
                  <div class="absolute inset-0 bg-black/30 opacity-0 group-hover/thumb:opacity-100 transition-opacity flex items-center justify-center text-white">
                    <Eye class="w-3.5 h-3.5" />
                  </div>
                </div>
                <span
                  v-else
                  class="inline-flex items-center gap-1 px-2 py-1 bg-morandi-bg text-morandi-muted rounded-md text-[10px] font-mono border border-morandi-borderSoft/60 select-none"
                  title="未开启代理中转模式，直链不加载外部缩略图"
                >
                  🔗 302 直链
                </span>
              </td>

              <!-- Created At -->
              <td class="py-2.5 font-mono text-[11px] text-morandi-muted whitespace-nowrap">
                {{ formatDate(item.created_at) }}
              </td>

              <!-- Source Name -->
              <td class="py-2.5 font-semibold text-morandi-text whitespace-nowrap">
                <div class="flex items-center gap-1.5">
                  <span class="w-1.5 h-1.5 rounded-full bg-morandi-sage"></span>
                  <span>{{ item.source_name }}</span>
                </div>
              </td>

              <!-- Category Tags -->
              <td class="py-2.5 whitespace-nowrap">
                <div class="flex items-center gap-1 flex-wrap">
                  <span
                    v-for="cat in parseCategories(item.categories)"
                    :key="cat"
                    class="px-1.5 py-0.2 bg-morandi-sage-light text-morandi-sage-dark rounded font-medium text-[10px]"
                  >
                    #{{ categoryMap[cat] || cat }}
                  </span>
                </div>
              </td>

              <!-- Actions -->
              <td class="py-2.5 pr-2 text-right whitespace-nowrap">
                <div class="flex items-center justify-end gap-1.5">
                  <!-- Save File Button (Only when local cached image exists) -->
                  <button
                    v-if="item.file_id || (item.image_id && item.image_id > 0)"
                    @click="toggleSaveImage(item)"
                    :disabled="savingImageId === (item.image_id || 9999)"
                    class="p-1.5 rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                    :class="isItemSaved(item) ? 'text-rose-500 hover:bg-rose-50' : 'text-morandi-muted hover:text-rose-400 hover:bg-rose-50/60'"
                    :title="isItemSaved(item) ? '取消收藏保存' : '收藏保存图片到本地'"
                  >
                    <svg class="w-3.5 h-3.5" :class="isItemSaved(item) ? 'fill-rose-500' : 'fill-none'" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/>
                    </svg>
                  </button>



                  <!-- Like / Increase Weight Button (+1) -->
                  <button
                    v-if="item.source_id || item.image_id"
                    @click="handleLike(item)"
                    :disabled="likingId === (item.image_id || item.source_id) || likedIds.has(item.image_id || item.source_id)"
                    class="p-1.5 rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                    :class="likedIds.has(item.image_id || item.source_id) ? 'text-emerald-600 bg-emerald-50 font-bold' : 'text-morandi-muted hover:text-emerald-600 hover:bg-emerald-50'"
                    :title="likedIds.has(item.image_id || item.source_id) ? '已点赞 (图源权重 +1)' : '喜欢（图源权重 +1）'"
                  >
                    <ThumbsUp class="w-3.5 h-3.5" :class="{ 'fill-emerald-600': likedIds.has(item.image_id || item.source_id) }" />
                  </button>

                  <!-- Dislike / Decrease Weight Button (-1) -->
                  <button
                    v-if="item.source_id || item.image_id"
                    @click="handleDislike(item)"
                    :disabled="dislikingId === (item.image_id || item.source_id) || dislikedIds.has(item.image_id || item.source_id)"
                    class="p-1.5 rounded-lg transition-colors cursor-pointer disabled:opacity-50"
                    :class="dislikedIds.has(item.image_id || item.source_id) ? 'text-orange-500 bg-orange-50 font-bold' : 'text-morandi-muted hover:text-orange-500 hover:bg-orange-50'"
                    :title="dislikedIds.has(item.image_id || item.source_id) ? '已点不喜欢 (图源权重 -1)' : '不喜欢（图源权重 -1）'"
                  >
                    <ThumbsDown class="w-3.5 h-3.5" :class="{ 'fill-orange-500': dislikedIds.has(item.image_id || item.source_id) }" />
                  </button>




                  <button
                    @click="copyToClipboard(item.image_url)"
                    class="p-1.5 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors cursor-pointer"
                    title="复制图片链接"
                  >
                    <Check v-if="copiedUrl === item.image_url" class="w-3.5 h-3.5 text-emerald-600" />
                    <Copy v-else class="w-3.5 h-3.5" />
                  </button>

                  <a
                    :href="item.image_url"
                    target="_blank"
                    rel="noopener noreferrer"
                    class="p-1.5 text-morandi-muted hover:text-morandi-sage-dark hover:bg-morandi-sage-light/60 rounded-lg transition-colors"
                    title="新窗口打开"
                  >
                    <ExternalLink class="w-3.5 h-3.5" />
                  </a>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div v-else class="text-center py-12 text-morandi-muted text-xs bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-border">
        暂无符合条件的分发历史记录
      </div>

      <!-- Pagination Controls Bar -->
      <div v-if="filteredHistory.length > 0" class="pt-3 border-t border-morandi-border/60 flex flex-col sm:flex-row items-center justify-between gap-3 text-xs">
        <!-- Per page size selector -->
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
            共 {{ totalHistoryCount }} 条记录
          </span>
        </div>

        <!-- Page Flip Buttons -->
        <div class="flex items-center gap-2 font-mono">
          <button
            @click="prevPage"
            :disabled="currentPage <= 1 || historyLoadingMore"
            class="px-3 py-1.5 bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft font-semibold disabled:opacity-40 transition-all cursor-pointer flex items-center gap-1 shadow-xs"
          >
            <ChevronLeft class="w-3.5 h-3.5" /> 上一页
          </button>

          <span class="px-3 py-1.5 text-morandi-text font-bold text-xs bg-morandi-bg rounded-xl border border-morandi-borderSoft">
            {{ currentPage }} / {{ totalPages }} 页
          </span>

          <button
            @click="nextPage"
            :disabled="currentPage >= totalPages || historyLoadingMore"
            class="px-3 py-1.5 bg-white hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft font-semibold disabled:opacity-40 transition-all cursor-pointer flex items-center gap-1 shadow-xs"
          >
            下一页 <ChevronRight class="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
    </div>

    <!-- Image Lightbox Preview Modal -->
    <div
      v-if="previewImage"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-950/70 backdrop-blur-md animate-in fade-in duration-200"
      @click.self="previewImage = null"
    >
      <div class="bg-white rounded-2xl shadow-2xl max-w-3xl w-full overflow-hidden border border-morandi-borderSoft flex flex-col max-h-[90vh]">
        <!-- Lightbox Header -->
        <div class="p-4 border-b border-morandi-border/60 flex items-center justify-between bg-morandi-bg/50">
          <div>
            <h3 class="font-bold text-sm text-morandi-text flex items-center gap-2">
              <Eye class="w-4 h-4 text-morandi-sage" /> 分发图片原图预览
            </h3>
            <p class="text-[11px] text-morandi-muted mt-0.5">
              命中图源：<span class="font-semibold text-morandi-text">{{ previewImage.sourceName }}</span> • 分发时间：{{ previewImage.time }}
            </p>
          </div>
          <button @click="previewImage = null" class="p-1 text-morandi-light hover:text-morandi-text rounded-lg cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </div>

        <!-- Lightbox Image View -->
        <div class="p-4 bg-stone-900 flex items-center justify-center min-h-[300px] max-h-[60vh] overflow-hidden relative">
          <img
            :src="previewImage.url"
            class="max-w-full max-h-[55vh] object-contain rounded-lg shadow-lg"
          />
        </div>

        <!-- Lightbox Footer -->
        <div class="p-4 border-t border-morandi-border/60 flex items-center justify-between bg-white text-xs gap-3">
          <span class="font-mono text-morandi-muted text-[11px] truncate flex-1" :title="previewImage.url">
            {{ previewImage.url }}
          </span>
          <div class="flex items-center gap-2 shrink-0">
            <button
              @click="copyToClipboard(previewImage.url)"
              class="px-3 py-1.5 font-medium bg-morandi-bg hover:bg-morandi-hover text-morandi-text rounded-xl border border-morandi-borderSoft flex items-center gap-1.5 transition-colors cursor-pointer"
            >
              <Check v-if="copiedUrl === previewImage.url" class="w-3.5 h-3.5 text-emerald-600" />
              <Copy v-else class="w-3.5 h-3.5" />
              <span>{{ copiedUrl === previewImage.url ? '已复制' : '复制地址' }}</span>
            </button>
            <a
              :href="previewImage.url"
              target="_blank"
              rel="noopener noreferrer"
              class="px-3.5 py-1.5 font-semibold bg-morandi-sage hover:bg-morandi-sage-dark text-white rounded-xl flex items-center gap-1.5 transition-colors shadow-xs"
            >
              <ExternalLink class="w-3.5 h-3.5" />
              <span>打开原图</span>
            </a>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
