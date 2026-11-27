<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { ExternalLink, Link } from 'lucide-vue-next'
import { useApi } from '../composables/useApi'
import type { CuratorFeedbackSettings } from '../types'

defineProps<{ saving: boolean }>()

const { getCuratorFeedbackSettings, updateCuratorFeedbackSettings, testCuratorFeedbackConnection } = useApi()
const enabled = ref(false)
const address = ref('')
const token = ref('')
const savedSettings = ref<CuratorFeedbackSettings>()
const busy = ref(false)
const error = ref('')
const connected = ref(false)

function applySettings(settings: CuratorFeedbackSettings) {
  savedSettings.value = settings
  enabled.value = settings.enabled
  address.value = settings.address
  token.value = ''
}

async function run(action: () => Promise<void>) {
  busy.value = true
  error.value = ''
  try {
    await action()
    return true
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : 'Curator 联动操作失败'
    return false
  } finally {
    busy.value = false
  }
}

async function load() {
  await run(async () => applySettings(await getCuratorFeedbackSettings()))
}

async function save() {
  const saved = savedSettings.value
  const origin = address.value.trim()
  if (!saved || (enabled.value === saved.enabled && origin === saved.address && !token.value.trim())) return true
  return run(async () => applySettings(await updateCuratorFeedbackSettings({
    enabled: Boolean(origin) && enabled.value,
    address: origin,
    token: origin ? token.value : '',
  })))
}

async function testConnection() {
  connected.value = false
  await run(async () => {
    await testCuratorFeedbackConnection({ address: address.value.trim(), token: token.value.trim() || undefined })
    connected.value = true
  })
}

watch([address, token], () => { connected.value = false })
defineExpose({ save, busy })
onMounted(load)
</script>

<template>
  <section class="p-4 space-y-3 bg-morandi-bg/60 border border-morandi-borderSoft rounded-2xl">
    <div class="space-y-1">
      <h4 class="flex items-center gap-1.5 text-xs font-bold text-morandi-text">
        <a href="https://github.com/Untitled572/pixiv-curator" target="_blank" rel="noopener noreferrer"
          class="inline-flex items-center gap-1.5 hover:underline">
          <Link class="w-3.5 h-3.5" /> Pixiv Curator 赞踩联动 <ExternalLink class="w-3 h-3" aria-hidden="true" />
        </a>
      </h4>
      <p class="text-[11px] text-morandi-muted">把来自此地址的图片赞踩同步到 Curator，由它更新画师和标签偏好。</p>
    </div>
    <div v-if="savedSettings" class="space-y-3">
      <fieldset :disabled="busy || saving" class="space-y-3 disabled:opacity-60">
        <label class="flex items-center gap-2 text-xs">
          <input v-model="enabled" type="checkbox" class="accent-morandi-sage" />
          启用联动
        </label>
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="space-y-1 text-xs">
            <span class="block font-medium">Curator 地址</span>
            <input v-model="address" type="url" placeholder="输入 HTTPS 地址" maxlength="2048" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          </label>
          <label class="space-y-1 text-xs">
            <span class="block font-medium">Curator 管理令牌</span>
            <input v-model="token" type="password" autocomplete="new-password" maxlength="4096"
              :placeholder="savedSettings.token_configured ? '已设置，留空保留' : '输入管理令牌'" class="morandi-input w-full px-3 py-2 font-mono text-xs" />
          </label>
        </div>
        <p class="text-[11px] text-morandi-muted">更换地址后请填写对应令牌。保存后只显示是否已配置。</p>
        <div class="flex flex-wrap items-center gap-2 text-xs">
          <button type="button" :disabled="!address.trim()"
            class="rounded-lg border border-morandi-borderSoft px-3 py-2 hover:bg-white disabled:opacity-50"
            @click="testConnection">测试连接</button>
          <span v-if="connected" role="status" class="rounded-md bg-morandi-sage-light/70 px-2 py-1 text-morandi-sage-dark">连接正常，令牌有效</span>
        </div>
      </fieldset>
    </div>
    <button v-else-if="error" type="button" class="rounded-lg border border-morandi-borderSoft px-3 py-2 text-xs hover:bg-white" @click="load">重试</button>
    <p v-if="error" role="alert" class="rounded-md bg-rose-50 px-2 py-1 text-xs text-rose-700">{{ error }}</p>
  </section>
</template>
