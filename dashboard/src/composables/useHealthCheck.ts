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
  const { healthCheck: apiHealthCheck, getHealthStatus } = useApi()

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
    if (running.value) return
    running.value = true
    error.value = ''
    try {
      results.value = await apiHealthCheck()
      lastRun.value = new Date().toISOString()
    } catch (e) {
      error.value = e instanceof Error ? e.message : '检测请求失败，请稍后重试。'
    } finally {
      running.value = false
    }
  }

  return { results, lastRun, running, error, summary, loadCached, runCheck }
}
