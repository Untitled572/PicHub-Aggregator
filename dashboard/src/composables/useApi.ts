import { ref } from 'vue'
import type { Source, Settings, DetectResult, ExportData, ImportResult, HealthResult, Tag, StatsResponse, ImageHistoryRecord, SavedImage } from '../types'


const API_BASE = ''

function getAuthToken(): string {
  try {
    return localStorage.getItem('pichub_admin_token') || ''
  } catch {
    return ''
  }
}

export function setAuthToken(token: string) {
  try {
    if (token) localStorage.setItem('pichub_admin_token', token)
    else localStorage.removeItem('pichub_admin_token')
  } catch {}
}

export function useApi() {
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function request<T>(url: string, options?: RequestInit): Promise<T> {
    loading.value = true
    error.value = null
    const headers: Record<string, string> = { 'Content-Type': 'application/json' }
    const token = getAuthToken()
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }
    try {
      const res = await fetch(`${API_BASE}${url}`, {
        headers: { ...headers, ...options?.headers as Record<string, string> },
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

  function getStats(range = 'today', startDate = '', endDate = '') {
    const params = new URLSearchParams()
    if (range) params.append('range', range)
    if (startDate) params.append('start_date', startDate)
    if (endDate) params.append('end_date', endDate)
    const query = params.toString() ? `?${params.toString()}` : ''
    return request<StatsResponse>(`/api/stats${query}`)
  }


  function getImageHistory(limit = 20, offset = 0) {
    return request<{ history: ImageHistoryRecord[]; total: number; limit: number; offset: number }>(`/api/stats/history?limit=${limit}&offset=${offset}`)
  }

  function saveImage(id: number) {
    return request<{ message: string }>(`/api/images/${id}/save`, { method: 'POST' })
  }

  function unsaveImage(id: number) {
    return request<{ message: string }>(`/api/images/${id}/save`, { method: 'DELETE' })
  }

  function listSavedImages(limit = 20, offset = 0) {
    return request<{ images: SavedImage[]; total: number; limit: number; offset: number }>(`/api/images/saved?limit=${limit}&offset=${offset}`)
  }


  return {
    loading, error,
    listSources, getSource, createSource, updateSource, deleteSource, toggleSource,
    getSettings, updateSettings, getTags, updateTags,
    detectURL, healthCheck,
    exportRules, importRules,
    getStats, getImageHistory,
    saveImage, unsaveImage, listSavedImages,
  }
}

