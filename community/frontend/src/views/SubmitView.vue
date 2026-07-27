<script setup lang="ts">
import { ref } from 'vue'
import { useCommunityApi } from '../composables/useCommunityApi'
import SubmitForm from '../components/SubmitForm.vue'
import { PlusCircle, CheckCircle2, ArrowLeft } from 'lucide-vue-next'

const { createRule } = useCommunityApi()
const submitted = ref(false)
const error = ref('')

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}

const categories = Object.keys(categoryMap)

async function handleSubmit(data: any) {
  error.value = ''
  try {
    await createRule(data)
    submitted.value = true
  } catch (e: any) {
    error.value = e.message || '提交失败，请重试'
  }
}
</script>

<template>
  <div class="max-w-3xl mx-auto space-y-6">
    <!-- Header -->
    <div class="morandi-card p-6 flex items-center justify-between">
      <div class="space-y-1">
        <div class="flex items-center gap-2">
          <PlusCircle class="w-5 h-5 text-morandi-ocean" />
          <h1 class="text-base font-bold text-morandi-text">提交新 API 解析规则</h1>
        </div>
        <p class="text-xs text-morandi-muted">向社区全网贡献优质第三方图片接口规则，经过存储后展示于社区广场</p>
      </div>

      <RouterLink
        to="/"
        class="flex items-center gap-1 px-3 py-1.5 bg-morandi-bg hover:bg-morandi-hover text-morandi-muted hover:text-morandi-text text-xs font-medium rounded-xl border border-morandi-borderSoft transition-colors shrink-0"
      >
        <ArrowLeft class="w-3.5 h-3.5" /> 返回规则广场
      </RouterLink>
    </div>

    <!-- Success Alert -->
    <div v-if="submitted" class="morandi-card p-5 bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/30 flex items-center justify-between gap-4">
      <div class="flex items-center gap-3">
        <CheckCircle2 class="w-6 h-6 shrink-0 text-morandi-sage" />
        <div>
          <h3 class="font-bold text-sm">规则提交成功！</h3>
          <p class="text-xs opacity-90 mt-0.5">感谢您的贡献！新规则将呈现在社区广场供大家使用与评分。</p>
        </div>
      </div>

      <RouterLink to="/" class="px-4 py-2 bg-morandi-sage text-white text-xs font-semibold rounded-xl shadow-xs hover:bg-morandi-sage-dark transition-colors shrink-0">
        查看广场
      </RouterLink>
    </div>

    <!-- Form -->
    <SubmitForm
      v-else
      :categories="categories"
      :error="error"
      @submit="handleSubmit"
    />
  </div>
</template>

