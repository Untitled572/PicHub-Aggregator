<script setup lang="ts">
import { ref } from 'vue'

const props = defineProps<{
  data: unknown
  urlHints?: string[]
  prefix?: string
}>()
const emit = defineEmits<{ select: [path: string] }>()

const expanded = ref<Set<string>>(new Set())

function isObject(val: unknown): val is Record<string, unknown> {
  return val !== null && typeof val === 'object' && !Array.isArray(val)
}

function isArray(val: unknown): val is unknown[] {
  return Array.isArray(val)
}

function isUrlHint(path: string): boolean {
  return props.urlHints?.includes(path) ?? false
}

function toggle(key: string) {
  if (expanded.value.has(key)) expanded.value.delete(key)
  else expanded.value.add(key)
}

function keys() {
  if (isObject(props.data)) return Object.keys(props.data)
  if (isArray(props.data)) return props.data.map((_, i) => String(i))
  return []
}

function childValue(key: string): unknown {
  if (isObject(props.data)) return (props.data as Record<string, unknown>)[key]
  if (isArray(props.data)) return (props.data as unknown[])[Number(key)]
  return null
}
</script>

<template>
  <div class="font-mono text-xs space-y-1">
    <div v-for="key in keys()" :key="key" class="py-0.5">
      <div class="flex items-center gap-1.5 flex-wrap">
        <button
          v-if="isObject(childValue(key)) || isArray(childValue(key))"
          @click="toggle(key)"
          class="w-4 h-4 rounded text-morandi-muted hover:bg-morandi-hover flex items-center justify-center text-[10px]"
        >
          {{ expanded.has(key) ? '▼' : '▶' }}
        </button>
        <span v-else class="w-4"></span>

        <span class="text-morandi-ocean-dark font-semibold">{{ key }}:</span>

        <template v-if="isObject(childValue(key)) || isArray(childValue(key))">
          <span class="text-morandi-light italic">{{ isArray(childValue(key)) ? '[ Array ]' : '{ Object }' }}</span>
        </template>
        <template v-else>
          <span class="text-morandi-sage-dark font-mono truncate max-w-xs bg-white px-1.5 py-0.5 rounded border border-morandi-borderSoft">{{ String(childValue(key)) }}</span>

          <button
            v-if="isUrlHint(prefix ? `${prefix}.${key}` : key)"
            @click="emit('select', prefix ? `${prefix}.${key}` : key)"
            class="ml-1 px-2 py-0.5 bg-morandi-sand text-white font-sans text-[10px] rounded-md shadow-xs hover:bg-morandi-sand-dark transition-colors font-medium"
          >
            绑定选择此字段
          </button>
        </template>
      </div>

      <div v-if="expanded.has(key)" class="ml-5 pl-2 border-l border-morandi-border/60 mt-1">
        <JsonTreeSelector
          :data="childValue(key)"
          :url-hints="urlHints"
          :prefix="prefix ? `${prefix}.${key}` : key"
          @select="(p: string) => emit('select', p)"
        />
      </div>
    </div>
  </div>
</template>

