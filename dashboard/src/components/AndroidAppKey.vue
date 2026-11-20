<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Key, Copy } from 'lucide-vue-next'
import { useApi } from '../composables/useApi'

const { getAppKeyState, generateAppKey, revokeAppKey } = useApi()
const configured = ref<boolean | null>(null)
const issuedKey = ref('')
const busy = ref(false)
const error = ref('')
const copied = ref(false)
const keyField = ref<HTMLInputElement>()

async function run(action: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await action()
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'App 密钥操作失败'
  } finally {
    busy.value = false
  }
}

function load() {
  return run(async () => { configured.value = (await getAppKeyState()).configured })
}

async function generate() {
  if (configured.value && !window.confirm('生成新密钥后，当前密钥会立即失效。继续吗？')) return
  await run(async () => {
    issuedKey.value = (await generateAppKey()).key
    configured.value = true
    copied.value = false
  })
}

async function revoke() {
  if (!window.confirm('撤销后，使用当前密钥的手机将无法赞踩。继续吗？')) return
  await run(async () => {
    await revokeAppKey()
    configured.value = false
    issuedKey.value = ''
    copied.value = false
  })
}

async function copyKey() {
  try {
    await navigator.clipboard.writeText(issuedKey.value)
    copied.value = true
  } catch {
    keyField.value?.focus()
    keyField.value?.select()
  }
}

onMounted(load)
</script>

<template>
  <section class="p-4 space-y-3 bg-morandi-bg/60 border border-morandi-borderSoft rounded-2xl">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h4 class="flex items-center gap-1.5 text-xs font-bold text-morandi-text">
          <Key class="w-3.5 h-3.5" /> 安卓 App 密钥
        </h4>
        <p class="text-[11px] text-morandi-muted">密钥用于安卓 App 的点赞和点踩。生成时会显示一次。</p>
      </div>
      <div class="flex gap-2 text-xs">
        <button v-if="configured === null && error" type="button" class="rounded-lg border border-morandi-borderSoft px-3 py-2 hover:bg-white" @click="load">重试</button>
        <button v-if="configured !== null" type="button" class="rounded-lg bg-morandi-sage-light px-3 py-2 font-medium text-morandi-sage-dark hover:bg-morandi-sage/20 disabled:opacity-50" :disabled="busy" @click="generate">
          {{ configured ? '重新生成' : '生成密钥' }}
        </button>
        <button v-if="configured" type="button" class="rounded-lg border border-morandi-borderSoft px-3 py-2 text-rose-600 hover:bg-rose-50 disabled:opacity-50" :disabled="busy" @click="revoke">撤销</button>
      </div>
    </div>
    <div v-if="issuedKey" class="flex items-center gap-2">
      <input ref="keyField" :value="issuedKey" readonly aria-label="新生成的 App 密钥" class="morandi-input min-w-0 flex-1 px-3 py-2 font-mono text-xs" />
      <button type="button" class="flex items-center gap-1 rounded-lg border border-morandi-borderSoft px-3 py-2 text-xs hover:bg-white" @click="copyKey">
        <Copy class="w-3 h-3" /> {{ copied ? '已复制' : '复制' }}
      </button>
    </div>
    <p v-if="error" role="alert" class="text-xs text-rose-600">{{ error }}</p>
  </section>
</template>
