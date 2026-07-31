import { ref } from 'vue'
import CryptoJS from 'crypto-js'
import type { Source, Settings, DetectResult, ExportData, ImportResult, HealthResult, Tag, StatsResponse, ImageHistoryRecord, SavedImage } from '../types'


const API_BASE = ''

// 注意: 不要用 @vueuse useLocalStorage 在模块级创建 token ref ——
// 非组件上下文中其 ref 赋值不会同步到 localStorage, 导致登录后守卫读不到 token
const TOKEN_KEY = 'pichub_admin_token'
const authToken = ref(localStorage.getItem(TOKEN_KEY) || '')

export function getAuthToken(): string {
  return authToken.value
}

export function setAuthToken(token: string) {
  authToken.value = token
  try {
    if (token) localStorage.setItem(TOKEN_KEY, token)
    else localStorage.removeItem(TOKEN_KEY)
  } catch { /* storage 不可用时降级为内存态 */ }
}

const MAX_DRIFT_MS = 30 * 60 * 1000 // 30 分钟

function checkTimeDrift(serverTimeHeader: string | null) {
  if (!serverTimeHeader) return
  const serverTs = Number(serverTimeHeader)
  if (Number.isNaN(serverTs)) return
  const diff = Math.abs(Date.now() - serverTs)
  if (diff > MAX_DRIFT_MS) {
    console.warn(`[PicHub] 客户端与服务端时间差超过 30 分钟 (${Math.round(diff / 60000)}min)，可能影响会话有效期`)
  }
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
      checkTimeDrift(res.headers.get('x-server-time'))
      if (res.status === 401 && url !== '/api/login') {
        // 会话失效: 清 token 并回登录页
        setAuthToken('')
        if (!window.location.pathname.startsWith('/login')) {
          window.location.href = '/login'
        }
        throw new Error('未登录或会话已过期')
      }
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

  function login(username: string, password: string) {
    return request<{ token: string }>('/api/login', {
      method: 'POST',
      body: JSON.stringify({
        username,
        password: CryptoJS.MD5(password).toString(),
      }),
    }).then((res) => {
      // 登录成功即持久化 token (调用方无需再手动 setAuthToken)
      setAuthToken(res.token)
      return res
    })
  }

  function logout() {
    return request<{ message: string }>('/api/logout', { method: 'POST' })
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
    // admin_password 语义为 MD5 摘要: 提交前统一转换, 不污染原对象
    const payload = { ...data }
    if (payload.admin_password) {
      payload.admin_password = CryptoJS.MD5(payload.admin_password).toString()
    }
    return request<Settings>('/api/settings', { method: 'PUT', body: JSON.stringify(payload) })
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

  function saveImage(id: number | string) {
    return request<{ message: string }>(`/api/images/${id}/save`, { method: 'POST' })
  }

  function unsaveImage(id: number | string) {
    return request<{ message: string }>(`/api/images/${id}/save`, { method: 'DELETE' })
  }

  function likeImage(id: number | string) {
    return request<{ message: string }>(`/api/images/${id}/like`, { method: 'POST' })
  }

  function dislikeImage(id: number | string) {
    return request<{ message: string }>(`/api/images/${id}/dislike`, { method: 'POST' })
  }


  function listSavedImages(limit = 20, offset = 0) {
    return request<{ images: SavedImage[]; total: number; limit: number; offset: number }>(`/api/images/saved?limit=${limit}&offset=${offset}`)
  }


  function exportCustomData(scopes: string[]) {
    const scopeParam = scopes.join(',')
    return window.open(`/api/export?scope=${scopeParam}`, '_blank')
  }

  function importCustomData(payload: FormData | object) {
    const isFormData = payload instanceof FormData
    return request<{ message: string; imported_sources: number; imported_stats: number; imported_images: number }>(
      '/api/import',
      {
        method: 'POST',
        headers: isFormData ? {} : { 'Content-Type': 'application/json' },
        body: isFormData ? payload : JSON.stringify(payload)
      }
    )
  }

  return {
    loading, error,
    login, logout,
    listSources, getSource, createSource, updateSource, deleteSource, toggleSource,
    getSettings, updateSettings, getTags, updateTags,
    detectURL, healthCheck,
    exportRules, importRules, exportCustomData, importCustomData,
    getStats, getImageHistory,
    saveImage, unsaveImage, likeImage, dislikeImage, listSavedImages,
  }
}
