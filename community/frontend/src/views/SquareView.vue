<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useCommunityApi } from '../composables/useCommunityApi'
import type { Rule } from '../types'
import RuleCard from '../components/RuleCard.vue'
import RuleFilter from '../components/RuleFilter.vue'
import { Sparkles, Globe, Inbox, Wrench, AlertTriangle } from 'lucide-vue-next'

const { getRules } = useCommunityApi()
const rules = ref<Rule[]>([])
const category = ref('')
const sort = ref('popular')

const categoryMap: Record<string, string> = {
  horizontal: '横屏',
  vertical: '竖屏',
  adaptive: '自适应',
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
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
      <div class="max-w-2xl space-y-3 relative z-10">
        <div class="flex items-center gap-2 flex-wrap">
          <div class="inline-flex items-center gap-1.5 px-3 py-1 bg-white/80 rounded-full text-xs font-semibold text-morandi-ocean-dark border border-morandi-ocean/20">
            <Globe class="w-3.5 h-3.5" />
            <span>无服务器图源规则中心</span>
          </div>
          <span class="inline-flex items-center gap-1 px-2.5 py-1 bg-amber-100 text-amber-800 text-xs font-bold rounded-full border border-amber-200">
            <Wrench class="w-3 h-3" />
            功能待开发 / 公测预览中
          </span>
        </div>

        <h1 class="text-xl md:text-2xl font-bold text-morandi-text tracking-tight">
          PicHub 社区规则广场
        </h1>
        <p class="text-xs md:text-sm text-morandi-muted leading-relaxed">
          探索、共享与一键导入全网优质第三方图片 API 提取规则。
        </p>

        <!-- Development Warning Banner -->
        <div class="mt-3 p-3 bg-amber-50/90 border border-amber-200 rounded-xl text-xs text-amber-900 flex items-start gap-2 max-w-xl">
          <AlertTriangle class="w-4 h-4 text-amber-600 shrink-0 mt-0.5" />
          <div class="space-y-0.5">
            <p class="font-bold">⚠️ 提示：社区规则广场目前处于公测与待开发阶段</p>
            <p class="text-[11px] text-amber-800 leading-relaxed">
              云端规则一键自动同步、在线解析沙盒及用户贡献积分系统正在研发上线中。当前展示为静态演示预览数据。
            </p>
          </div>
        </div>
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

