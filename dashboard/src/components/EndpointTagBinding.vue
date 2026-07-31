<script setup lang="ts">
import { computed } from 'vue'
import { CheckSquare, Square } from 'lucide-vue-next'
import type { Tag } from '../types'

const props = defineProps<{
  modelValue: string[]
  tags: Tag[]
}>()

const emit = defineEmits<{ 'update:modelValue': [string[]] }>()

const allTagsBound = computed(() => {
  if (props.modelValue.length === 0) return false
  const visible = props.tags.map(t => t.id)
  return visible.every(id => props.modelValue.includes(id))
})

function toggleAll() {
  if (allTagsBound.value) {
    emit('update:modelValue', [])
  } else {
    emit('update:modelValue', [...props.tags.map(t => t.id), '__uncategorized__'])
  }
}

function toggleTag(id: string) {
  let current = props.modelValue.filter(t => t !== '__uncategorized__')
  if (current.length === 0) {
    current = [id]
  } else {
    const idx = current.indexOf(id)
    if (idx >= 0) current.splice(idx, 1)
    else current.push(id)
  }
  const visible = props.tags.map(t => t.id)
  if (visible.every(t => current.includes(t))) current.push('__uncategorized__')
  emit('update:modelValue', current)
}

function isBound(id: string) {
  return props.modelValue.includes(id)
}
</script>

<template>
  <div class="flex flex-wrap gap-2 pt-1">
    <button
      type="button"
      @click="toggleAll"
      class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-semibold transition-all border cursor-pointer"
      :class="allTagsBound
        ? 'bg-morandi-sage text-white border-morandi-sage shadow-xs'
        : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover'"
    >
      <component :is="allTagsBound ? CheckSquare : Square" class="w-3.5 h-3.5" />
      <span>全部 Tags 标签</span>
    </button>

    <button
      v-for="t in tags"
      :key="t.id"
      type="button"
      @click="toggleTag(t.id)"
      class="flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium transition-all border cursor-pointer"
      :class="isBound(t.id)
        ? 'bg-morandi-sage-light text-morandi-sage-dark border-morandi-sage/40 font-semibold'
        : 'bg-white text-morandi-muted border-morandi-borderSoft hover:bg-morandi-hover opacity-60'"
    >
      <component :is="isBound(t.id) ? CheckSquare : Square" class="w-3.5 h-3.5 text-morandi-sage" />
      <span>#{{ t.name }} ({{ t.id }})</span>
    </button>
  </div>
</template>
