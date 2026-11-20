import { ref, computed } from 'vue'
import type { HealthResult } from '../types'
import { useApi } from './useApi'

export interface HealthCache {
  results: HealthResult[]
  last_run: string
}

export function useHealthCheck() {
  const results = ref<HealthResult[]>([])
  const lastRun = ref('')
  const running = ref(false)
  const error = ref('')
  const checkingIds = ref(new Set<number>())
  const rowErrors = ref<Record<number, string>>({})
  const { healthCheck: apiHealthCheck, healthCheckSource, getHealthStatus } = useApi()

  const summary = computed(() => {
    const total = results.value.length
    const available = results.value.filter(r => r.available).length
    const avgLatency = total > 0
      ? Math.round(results.value.reduce((s, r) => s + r.latency_ms, 0) / total)
      : 0
    return { total, available, failed: total - available, avgLatency }
  })

  async function loadCached() {
    try {
      const data: HealthCache = await getHealthStatus()
      if (data.results) {
        results.value = data.results
        lastRun.value = data.last_run
      }
      error.value = ''
    } catch (e) { error.value = e instanceof Error ? e.message : '无法读取检测结果。' }
  }

  async function runCheck() {
    if (running.value || checkingIds.value.size > 0) return
    running.value = true
    error.value = ''
    rowErrors.value = {}
    try {
      results.value = await apiHealthCheck()
      lastRun.value = new Date().toISOString()
    } catch (e) {
      error.value = e instanceof Error ? e.message : '检测请求失败，请稍后重试。'
    } finally {
      running.value = false
    }
  }

  async function runSourceCheck(id: number) {
    if (running.value || checkingIds.value.has(id)) return
    checkingIds.value.add(id)
    delete rowErrors.value[id]
    try {
      const result = await healthCheckSource(id)
      const index = results.value.findIndex(r => r.id === id)
      if (index >= 0) results.value[index] = result
      else results.value.push(result)
      if (result.checked_at && (!lastRun.value || new Date(result.checked_at).getTime() > new Date(lastRun.value).getTime())) lastRun.value = result.checked_at
    } catch (e) {
      rowErrors.value[id] = e instanceof Error ? e.message : '检测请求失败，请稍后重试。'
    } finally {
      checkingIds.value.delete(id)
    }
  }

  return { results, lastRun, running, error, summary, loadCached, runCheck, checkingIds, rowErrors, runSourceCheck }
}
