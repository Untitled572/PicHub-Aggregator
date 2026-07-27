<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useCommunityApi } from '../composables/useCommunityApi'
import type { Rule } from '../types'
import RuleCard from '../components/RuleCard.vue'
import RuleFilter from '../components/RuleFilter.vue'
import { Sparkles, Globe, Inbox } from 'lucide-vue-next'

const { getRules } = useCommunityApi()
const rules = ref<Rule[]>([])
const category = ref('')
const sort = ref('popular')

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}

const categories = Object.keys(categoryMap)

onMounted(loadRules)

async function loadRules() {
  try {
    rules.value = await getRules(category.value, sort.value)
  } catch {}
}

function onCategoryChange(cat: string) {
  category.value = cat
  loadRules()
}

function onSortChange(s: string) {
  sort.value = s
  loadRules()
}
</script>

<template>
  <div class="space-y-6">
    <!-- Hero Banner -->
    <div class="morandi-card p-6 md:p-8 bg-gradient-to-r from-morandi-ocean-light/50 via-morandi-bg to-morandi-sand-light/40 border border-morandi-borderSoft relative overflow-hidden">
      <div class="max-w-2xl space-y-2 relative z-10">
        <div class="inline-flex items-center gap-1.5 px-3 py-1 bg-white/80 rounded-full text-xs font-semibold text-morandi-ocean-dark border border-morandi-ocean/20">
          <Globe class="w-3.5 h-3.5" />
          <span>Serverless 云节点全网共享</span>
        </div>
        <h1 class="text-xl md:text-2xl font-bold text-morandi-text tracking-tight">探索全网优质图片 API 提取规则</h1>
        <p class="text-xs text-morandi-muted leading-relaxed">
          浏览由全球开发者贡献的公开 API 解析配置，支持一键复制规则 JSON 或直接订阅拉取到您的本地 Docker 节点。
        </p>
      </div>

      <div class="absolute -right-8 -bottom-8 w-48 h-48 rounded-full bg-morandi-ocean/10 blur-2xl pointer-events-none"></div>
    </div>

    <!-- Filter & Header -->
    <div class="morandi-card p-4 flex flex-col sm:flex-row items-center justify-between gap-4">
      <div class="flex items-center gap-2">
        <Sparkles class="w-5 h-5 text-morandi-ocean" />
        <h2 class="font-bold text-sm text-morandi-text">图源规则广场</h2>
        <span class="text-xs px-2 py-0.5 bg-morandi-sidebar text-morandi-muted rounded-full font-mono">{{ rules.length }} 条规则</span>
      </div>

      <RuleFilter
        :categories="categories"
        :category="category"
        :sort="sort"
        @category-change="onCategoryChange"
        @sort-change="onSortChange"
      />
    </div>

    <!-- Cards Grid -->
    <div v-if="rules.length > 0" class="grid gap-4 md:grid-cols-2">
      <RuleCard v-for="rule in rules" :key="rule.id" :rule="rule" @voted="loadRules" />
    </div>

    <!-- Empty State -->
    <div v-else class="morandi-card py-16 text-center space-y-3">
      <div class="w-12 h-12 rounded-full bg-morandi-sidebar mx-auto flex items-center justify-center text-morandi-light">
        <Inbox class="w-6 h-6" />
      </div>
      <div>
        <p class="text-sm font-medium text-morandi-text">暂无规则或未找到匹配项</p>
        <p class="text-xs text-morandi-muted mt-1">成为第一个提交此分类 API 规则的创作者吧！</p>
      </div>
      <RouterLink
        to="/submit"
        class="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-semibold bg-morandi-ocean text-white rounded-xl shadow-sm hover:bg-morandi-ocean-dark transition-colors"
      >
        提交新规则
      </RouterLink>
    </div>
  </div>
</template>

