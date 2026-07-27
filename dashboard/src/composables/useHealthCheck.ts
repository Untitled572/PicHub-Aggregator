import { ref, computed } from 'vue'
import type { HealthResult } from '../types'
import { useApi } from './useApi'

export function useHealthCheck() {
  const results = ref<HealthResult[]>([])
  const running = ref(false)
  const progress = ref(0)
  const { healthCheck: apiHealthCheck } = useApi()

  const summary = computed(() => {
    const total = results.value.length
    const available = results.value.filter(r => r.available).length
    const avgLatency = total > 0
      ? Math.round(results.value.reduce((s, r) => s + r.latency_ms, 0) / total)
      : 0
    return { total, available, failed: total - available, avgLatency }
  })

  async function runCheck() {
    running.value = true
    progress.value = 0
    try {
      results.value = await apiHealthCheck()
      progress.value = 100
    } finally {
      running.value = false
    }
  }

  return { results, running, progress, summary, runCheck }
}
