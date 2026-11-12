import { computed, ref } from 'vue'
import type { SourceDebugState } from '../types'
import { useApi } from './useApi'

const state = ref<SourceDebugState>({ enabled: false })
const loading = ref(false)
const initialized = ref(false)
const changing = ref(false)
const error = ref('')
let readInFlight: Promise<void> | null = null
let revision = 0

const api = useApi()

function validateState(value: unknown): SourceDebugState {
  if (!value || typeof value !== 'object' || Array.isArray(value) || typeof (value as SourceDebugState).enabled !== 'boolean') {
    throw new Error('服务端返回的调试状态无效')
  }
  const candidate = value as SourceDebugState
  if (
    (candidate.source_id !== undefined && typeof candidate.source_id !== 'number') ||
    (candidate.source_name !== undefined && typeof candidate.source_name !== 'string') ||
    (candidate.switching !== undefined && typeof candidate.switching !== 'boolean')
  ) {
    throw new Error('服务端返回的调试状态无效')
  }
  return candidate
}

async function refresh() {
  if (changing.value) return
  if (readInFlight) return readInFlight

  const readRevision = revision
  loading.value = true
  error.value = ''
  let pending: Promise<void> = Promise.resolve()
  pending = (async () => {
    try {
      const result = validateState(await api.getSourceDebug(AbortSignal.timeout(8000)))
      if (readRevision === revision) state.value = result
    } catch (e: any) {
      if (readRevision === revision) error.value = e?.name === 'TimeoutError' || e?.name === 'AbortError' ? '读取调试状态超时，请重试' : `读取调试状态失败：${e?.message || '请求失败'}`
    } finally {
      if (readInFlight === pending) {
        readInFlight = null
        loading.value = false
        initialized.value = true
      }
    }
  })()
  readInFlight = pending
  return pending
}

async function enable(sourceId: number) {
  if (changing.value) return
  revision++
  changing.value = true
  error.value = ''
  try {
    state.value = validateState(await api.enableSourceDebug(sourceId))
  } catch (e: any) {
    error.value = `开启调试失败：${e?.message || '请求失败'}`
    throw e
  } finally {
    changing.value = false
  }
}

async function disable() {
  if (changing.value) return
  revision++
  changing.value = true
  error.value = ''
  try {
    state.value = validateState(await api.disableSourceDebug())
  } catch (e: any) {
    error.value = `退出调试失败：${e?.message || '请求失败'}`
    throw e
  } finally {
    changing.value = false
  }
}

export function useSourceDebug() {
  return {
    state,
    loading,
    initialized,
    changing,
    switching: computed(() => !!state.value.switching),
    busy: computed(() => (loading.value && !initialized.value) || changing.value || !!state.value.switching),
    error,
    refresh,
    enable,
    disable,
  }
}
