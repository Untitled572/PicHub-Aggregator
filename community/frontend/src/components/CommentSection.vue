<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useCommunityApi } from '../composables/useCommunityApi'
import type { Comment } from '../types'
import { Send, User } from 'lucide-vue-next'

const props = defineProps<{ ruleId: string }>()
const { getComments, addComment } = useCommunityApi()

const comments = ref<Comment[]>([])
const author = ref('')
const content = ref('')
const submitting = ref(false)

onMounted(loadComments)

async function loadComments() {
  try {
    comments.value = await getComments(props.ruleId)
  } catch {}
}

async function handleSubmit() {
  if (!author.value.trim() || !content.value.trim()) return
  submitting.value = true
  try {
    await addComment(props.ruleId, {
      author: author.value,
      content: content.value,
      turnstile_token: 'placeholder',
    })
    content.value = ''
    loadComments()
  } catch {}
  submitting.value = false
}
</script>

<template>
  <div class="mt-3 pt-3 border-t border-morandi-border/60 space-y-3">
    <h4 class="text-xs font-bold text-morandi-text">社区讨论与测试反馈</h4>

    <!-- Comment list -->
    <div v-if="comments.length > 0" class="space-y-2 max-h-48 overflow-y-auto pr-1">
      <div v-for="c in comments" :key="c.id" class="bg-morandi-bg/80 p-2.5 rounded-xl border border-morandi-borderSoft text-xs space-y-0.5">
        <div class="flex items-center justify-between text-morandi-muted text-[10px]">
          <span class="font-semibold text-morandi-text flex items-center gap-1">
            <User class="w-3 h-3 text-morandi-sage" /> {{ c.author }}
          </span>
          <span v-if="c.created_at">{{ c.created_at }}</span>
        </div>
        <p class="text-morandi-text leading-normal">{{ c.content }}</p>
      </div>
    </div>

    <div v-else class="text-center py-4 text-xs text-morandi-muted bg-morandi-bg/40 rounded-xl border border-dashed border-morandi-borderSoft">
      暂无评论，留下第一个使用心得吧！
    </div>

    <!-- Input Form -->
    <div class="flex flex-col sm:flex-row gap-2 pt-1">
      <input
        v-model="author"
        placeholder="您的称呼"
        class="morandi-input px-3 py-1.5 text-xs w-full sm:w-28"
      />
      <input
        v-model="content"
        placeholder="发表关于此 API 规则的使用评价/连通反馈..."
        class="morandi-input px-3 py-1.5 text-xs flex-1"
        @keyup.enter="handleSubmit"
      />
      <button
        @click="handleSubmit"
        :disabled="submitting || !author.trim() || !content.trim()"
        class="flex items-center justify-center gap-1 px-3 py-1.5 bg-morandi-ocean hover:bg-morandi-ocean-dark text-white rounded-xl text-xs font-medium transition-all disabled:opacity-50 shrink-0"
      >
        <Send class="w-3 h-3" />
        <span>发布</span>
      </button>
    </div>
  </div>
</template>

