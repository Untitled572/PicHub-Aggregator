<script setup lang="ts">
import { ref } from 'vue'
import { Send, AlertTriangle } from 'lucide-vue-next'

defineProps<{
  categories: string[]
  error: string
}>()
const emit = defineEmits<{ submit: [data: any] }>()

const form = ref({
  name: '',
  url: '',
  description: '',
  resp_type: 'json',
  json_path: '',
  categories: [] as string[],
  author: '',
  public: true,
  turnstile_token: 'placeholder',
})

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}

function toggleCategory(cat: string) {
  const idx = form.value.categories.indexOf(cat)
  if (idx >= 0) form.value.categories.splice(idx, 1)
  else form.value.categories.push(cat)
}

function handleSubmit() {
  emit('submit', { ...form.value })
}
</script>

<template>
  <div class="morandi-card p-6 md:p-8 space-y-5">
    <div class="space-y-4 text-xs">
      <div>
        <label class="font-medium text-morandi-text block mb-1">规则/图源名称 <span class="text-morandi-rose">*</span></label>
        <input v-model="form.name" placeholder="例如: Bing 每日一图 API" class="morandi-input w-full px-3 py-2" />
      </div>

      <div>
        <label class="font-medium text-morandi-text block mb-1">目标 API URL <span class="text-morandi-rose">*</span></label>
        <input v-model="form.url" placeholder="https://api.example.com/v1/random" class="morandi-input w-full px-3 py-2 font-mono" />
      </div>

      <div>
        <label class="font-medium text-morandi-text block mb-1">规则描述说明</label>
        <input v-model="form.description" placeholder="简要描述该图源的图片画质、响应速度或特点" class="morandi-input w-full px-3 py-2" />
      </div>

      <div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div>
          <label class="font-medium text-morandi-text block mb-1">响应解析类型</label>
          <select v-model="form.resp_type" class="morandi-input w-full px-3 py-2 appearance-none cursor-pointer">
            <option value="image">图片二进制流 (Image)</option>
            <option value="redirect">302 重定向直链 (Redirect)</option>
            <option value="json">JSON 结构提取 (JSON)</option>
          </select>
        </div>

        <div>
          <label class="font-medium text-morandi-text block mb-1">JSON 提取节点路径</label>
          <input
            v-model="form.json_path"
            placeholder="例如: data.image_url"
            :disabled="form.resp_type !== 'json'"
            class="morandi-input w-full px-3 py-2 font-mono disabled:opacity-50"
          />
        </div>
      </div>

      <div>
        <label class="font-medium text-morandi-text block mb-1.5">分类标签选择</label>
        <div class="flex flex-wrap gap-1.5">
          <button
            v-for="cat in categories"
            :key="cat"
            type="button"
            @click="toggleCategory(cat)"
            class="px-3 py-1 rounded-lg transition-colors font-medium text-xs"
            :class="form.categories.includes(cat)
              ? 'bg-morandi-ocean text-white shadow-xs'
              : 'bg-morandi-sidebar text-morandi-muted hover:bg-morandi-hover'"
          >
            #{{ categoryMap[cat] || cat }}
          </button>
        </div>
      </div>

      <div>
        <label class="font-medium text-morandi-text block mb-1">贡献者签名/作者</label>
        <input v-model="form.author" placeholder="您的昵称或 GitHub 用户名" class="morandi-input w-full px-3 py-2" />
      </div>

      <div class="flex items-center gap-2 pt-1">
        <input type="checkbox" v-model="form.public" id="public" class="rounded accent-morandi-ocean cursor-pointer" />
        <label for="public" class="text-xs text-morandi-text cursor-pointer select-none">公开分享 (发布至社区广场供其他人点赞及导入)</label>
      </div>
    </div>

    <!-- Error notice -->
    <div v-if="error" class="p-3 bg-morandi-rose-light text-morandi-rose-dark rounded-xl text-xs flex items-center gap-2">
      <AlertTriangle class="w-4 h-4 shrink-0" />
      <span>{{ error }}</span>
    </div>

    <!-- Action -->
    <div class="pt-3 border-t border-morandi-border/60 flex justify-end">
      <button
        @click="handleSubmit"
        :disabled="!form.name.trim() || !form.url.trim()"
        class="flex items-center gap-2 px-6 py-2.5 bg-morandi-ocean hover:bg-morandi-ocean-dark text-white rounded-xl text-xs font-semibold shadow-sm transition-all disabled:opacity-50"
      >
        <Send class="w-4 h-4" />
        <span>提交规则到社区</span>
      </button>
    </div>
  </div>
</template>

