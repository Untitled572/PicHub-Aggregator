import { ref } from 'vue'
import type { Source, Settings, DetectResult, ExportData, ImportResult, HealthResult, Tag } from '../types'

const API_BASE = ''

export function useApi() {
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function request<T>(url: string, options?: RequestInit): Promise<T> {
    loading.value = true
    error.value = null
    try {
      const res = await fetch(`${API_BASE}${url}`, {
        headers: { 'Content-Type': 'application/json', ...options?.headers },
        ...options,
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error || `HTTP ${res.status}`)
      }
      if (res.status === 204) return undefined as T
      if (res.headers.get('content-type')?.includes('json')) return res.json()
      return undefined as T
    } catch (e: any) {
      error.value = e.message
      throw e
    } finally {
      loading.value = false
    }
  }

  function listSources() {
    return request<Source[]>('/api/sources')
  }

  function getSource(id: number) {
    return request<Source>(`/api/sources/${id}`)
  }

  function createSource(data: Partial<Source>) {
    return request<Source>('/api/sources', { method: 'POST', body: JSON.stringify(data) })
  }

  function updateSource(id: number, data: Partial<Source>) {
    return request<Source>(`/api/sources/${id}`, { method: 'PUT', body: JSON.stringify(data) })
  }

  function deleteSource(id: number) {
    return request<void>(`/api/sources/${id}`, { method: 'DELETE' })
  }

  function toggleSource(id: number) {
    return request<Source>(`/api/sources/${id}/toggle`, { method: 'POST' })
  }

  function getSettings() {
    return request<Settings>('/api/settings')
  }

  function updateSettings(data: Settings) {
    return request<Settings>('/api/settings', { method: 'PUT', body: JSON.stringify(data) })
  }

  function getTags() {
    return request<Tag[]>('/api/tags')
  }

  function updateTags(data: Tag[]) {
    return request<Tag[]>('/api/tags', { method: 'PUT', body: JSON.stringify(data) })
  }

  function detectURL(url: string) {
    return request<DetectResult>('/random/detect', { method: 'POST', body: JSON.stringify({ url }) })
  }

  function healthCheck() {
    return request<HealthResult[]>('/api/sources/health-check', { method: 'POST' })
  }

  function exportRules() {
    return request<ExportData>('/api/export', { method: 'POST' })
  }

  function importRules(data: ExportData) {
    return request<ImportResult>('/api/import', { method: 'POST', body: JSON.stringify(data) })
  }

  return {
    loading, error,
    listSources, getSource, createSource, updateSource, deleteSource, toggleSource,
    getSettings, updateSettings, getTags, updateTags,
    detectURL, healthCheck,
    exportRules, importRules,
  }
}
