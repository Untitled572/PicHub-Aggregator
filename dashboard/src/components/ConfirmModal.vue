<script setup lang="ts">
import { AlertTriangle } from 'lucide-vue-next'

defineProps<{
  show: boolean
  title?: string
  message?: string
  confirmText?: string
  cancelText?: string
  loading?: boolean
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()
</script>

<template>
  <Teleport to="body">
    <div
      v-if="show"
      class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-stone-900/40 backdrop-blur-md transition-all"
      @click.self="emit('cancel')"
    >
      <div class="bg-white rounded-2xl shadow-morandi-lg w-full max-w-sm border border-morandi-borderSoft overflow-hidden animate-in fade-in zoom-in-95 duration-200">
        <div class="p-5 space-y-4">
          <!-- Header section -->
          <div class="flex items-start gap-3.5">
            <div class="w-10 h-10 rounded-xl bg-rose-50 border border-rose-200/60 text-rose-600 flex items-center justify-center shrink-0">
              <AlertTriangle class="w-5 h-5" />
            </div>
            <div class="flex-1 min-w-0">
              <h3 class="font-bold text-sm text-morandi-text">{{ title || '确认删除' }}</h3>
              <p class="text-xs text-morandi-muted mt-1 leading-relaxed">
                {{ message || '您确定要执行删除操作吗？操作完成后无法撤销。' }}
              </p>
            </div>
          </div>

          <!-- Action buttons -->
          <div class="flex items-center justify-end gap-2.5 pt-2 border-t border-morandi-border/40 text-xs">
            <button
              type="button"
              @click="emit('cancel')"
              class="px-4 py-2 bg-morandi-bg hover:bg-morandi-hover text-morandi-text font-medium rounded-xl border border-morandi-borderSoft transition-colors cursor-pointer"
            >
              {{ cancelText || '取消' }}
            </button>
            <button
              type="button"
              @click="emit('confirm')"
              :disabled="loading"
              class="px-4 py-2 bg-rose-600 hover:bg-rose-700 text-white font-semibold rounded-xl shadow-sm transition-colors disabled:opacity-50 cursor-pointer"
            >
              <span>{{ confirmText || '确认删除' }}</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>
