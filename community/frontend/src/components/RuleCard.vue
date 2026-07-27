<script setup lang="ts">
import { ref } from 'vue'
import type { Rule } from '../types'
import CommentSection from './CommentSection.vue'
import { ThumbsUp, ThumbsDown, Copy, Check, MessageSquare, Globe, Code, User } from 'lucide-vue-next'

const props = defineProps<{ rule: Rule }>()
const emit = defineEmits<{ voted: [] }>()

const showComments = ref(false)
const copied = ref(false)

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}

const respTypeMap: Record<string, { label: string; class: string }> = {
  json: { label: 'JSON 提取', class: 'bg-morandi-ocean-light text-morandi-ocean-dark border-morandi-ocean/20' },
  image: { label: '图片直链', class: 'bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/20' },
  redirect: { label: '302 重定向', class: 'bg-morandi-sand-light text-morandi-sand-dark border-morandi-sand/30' },
}

function copyRule() {
  const json = JSON.stringify({
    name: props.rule.name,
    url: props.rule.url,
    resp_type: props.rule.resp_type,
    json_path: props.rule.json_path,
    categories: props.rule.categories,
    enabled: true,
  }, null, 2)
  navigator.clipboard.writeText(json)
  copied.value = true
  setTimeout(() => copied.value = false, 2000)
}

function handleVote(type: 'up' | 'down') {
  emit('voted')
}
</script>

<template>
  <div class="morandi-card p-5 flex flex-col justify-between space-y-4">
    <div class="space-y-2.5">
      <!-- Title & Votes -->
      <div class="flex items-start justify-between gap-3">
        <div>
          <h3 class="font-bold text-sm text-morandi-text leading-snug">{{ rule.name }}</h3>
          <p v-if="rule.description" class="text-xs text-morandi-muted mt-1 leading-relaxed">{{ rule.description }}</p>
        </div>

        <!-- Upvote / Downvote -->
        <div class="flex items-center gap-1.5 bg-morandi-bg p-1 rounded-xl border border-morandi-borderSoft shrink-0 text-xs">
          <button
            @click="handleVote('up')"
            class="flex items-center gap-1 px-2 py-1 rounded-lg text-morandi-muted hover:text-morandi-sage-dark hover:bg-white transition-all font-medium"
            title="推荐此规则"
          >
            <ThumbsUp class="w-3.5 h-3.5 text-morandi-sage" />
            <span>{{ rule.upvotes || 0 }}</span>
          </button>
          <div class="h-3 w-px bg-morandi-border/60"></div>
          <button
            @click="handleVote('down')"
            class="flex items-center gap-1 px-2 py-1 rounded-lg text-morandi-muted hover:text-morandi-rose-dark hover:bg-white transition-all font-medium"
            title="反馈失效/存在缺陷"
          >
            <ThumbsDown class="w-3.5 h-3.5 text-morandi-rose" />
            <span>{{ rule.downvotes || 0 }}</span>
          </button>
        </div>
      </div>

      <!-- URL Endpoint -->
      <div class="flex items-center gap-1.5 text-[11px] text-morandi-muted truncate font-mono bg-morandi-bg/80 px-2.5 py-1 rounded-lg border border-morandi-borderSoft/60">
        <Globe class="w-3.5 h-3.5 text-morandi-light shrink-0" />
        <span class="truncate">{{ rule.url }}</span>
      </div>

      <!-- Badges row -->
      <div class="flex items-center gap-1.5 flex-wrap">
        <span
          class="text-[11px] px-2 py-0.5 rounded-full border font-medium"
          :class="respTypeMap[rule.resp_type]?.class || 'bg-morandi-sidebar text-morandi-muted border-morandi-borderSoft'"
        >
          {{ respTypeMap[rule.resp_type]?.label || rule.resp_type }}
        </span>

        <span
          v-for="cat in rule.categories"
          :key="cat"
          class="text-[11px] px-2 py-0.5 bg-morandi-lavender-light text-morandi-lavender-dark rounded-md font-medium"
        >
          #{{ categoryMap[cat] || cat }}
        </span>

        <span v-if="rule.json_path" class="text-[11px] px-2 py-0.5 bg-morandi-sidebar text-morandi-muted rounded-md font-mono flex items-center gap-1">
          <Code class="w-3 h-3 text-morandi-light" />
          {{ rule.json_path }}
        </span>
      </div>
    </div>

    <!-- Card Footer -->
    <div class="pt-3 border-t border-morandi-border/60 flex items-center justify-between text-xs">
      <div class="flex items-center gap-1.5 text-morandi-muted text-[11px]">
        <User class="w-3.5 h-3.5 text-morandi-light" />
        <span>提交者: <strong class="text-morandi-text font-medium">{{ rule.author || '匿名贡献者' }}</strong></span>
      </div>

      <div class="flex items-center gap-2">
        <button
          @click="showComments = !showComments"
          class="flex items-center gap-1 px-2.5 py-1.5 bg-morandi-bg hover:bg-morandi-hover text-morandi-text rounded-lg border border-morandi-borderSoft font-medium transition-colors"
        >
          <MessageSquare class="w-3.5 h-3.5 text-morandi-ocean" />
          <span>评论 ({{ rule.comment_count || 0 }})</span>
        </button>

        <button
          @click="copyRule"
          class="flex items-center gap-1 px-3 py-1.5 bg-morandi-ocean hover:bg-morandi-ocean-dark text-white rounded-lg font-medium shadow-xs transition-all active:scale-95"
        >
          <component :is="copied ? Check : Copy" class="w-3.5 h-3.5" />
          <span>{{ copied ? '已复制规则' : '复制导入码' }}</span>
        </button>
      </div>
    </div>

    <!-- Comment Drawer / Section -->
    <CommentSection v-if="showComments" :rule-id="rule.id!" />
  </div>
</template>

