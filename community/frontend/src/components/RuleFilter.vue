<script setup lang="ts">
defineProps<{
  categories: string[]
  category: string
  sort: string
}>()
const emit = defineEmits<{
  'categoryChange': [value: string]
  'sortChange': [value: string]
}>()

const categoryMap: Record<string, string> = {
  avatar: '头像',
  anime: '二次元',
  landscape: '风景',
  portrait: '人像',
  adaptive: '自适应',
  'ai-generated': 'AI生成'
}
</script>

<template>
  <div class="flex items-center gap-2.5 w-full sm:w-auto">
    <div class="relative flex-1 sm:w-40">
      <select
        :value="category"
        @change="emit('categoryChange', ($event.target as HTMLSelectElement).value)"
        class="morandi-input w-full px-3 py-1.5 text-xs appearance-none cursor-pointer pr-7"
      >
        <option value="">全部分类标签</option>
        <option v-for="cat in categories" :key="cat" :value="cat">
          {{ categoryMap[cat] || cat }}
        </option>
      </select>
      <div class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-morandi-muted text-[10px]">▼</div>
    </div>

    <div class="relative flex-1 sm:w-36">
      <select
        :value="sort"
        @change="emit('sortChange', ($event.target as HTMLSelectElement).value)"
        class="morandi-input w-full px-3 py-1.5 text-xs appearance-none cursor-pointer pr-7 font-medium"
      >
        <option value="popular">🔥 最受欢迎</option>
        <option value="newest">✨ 最新发布</option>
        <option value="oldest">⌛ 最早发布</option>
      </select>
      <div class="pointer-events-none absolute right-2.5 top-1/2 -translate-y-1/2 text-morandi-muted text-[10px]">▼</div>
    </div>
  </div>
</template>

