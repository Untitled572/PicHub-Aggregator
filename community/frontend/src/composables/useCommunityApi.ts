import { ref } from 'vue'
import type { Rule, Comment } from '../types'

const API_BASE = import.meta.env.VITE_API_BASE || ''

export function useCommunityApi() {
  const loading = ref(false)

  async function request<T>(url: string, options?: RequestInit): Promise<T> {
    loading.value = true
    try {
      const res = await fetch(`${API_BASE}${url}`, {
        headers: { 'Content-Type': 'application/json', ...options?.headers },
        ...options,
      })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      return res.json()
    } finally {
      loading.value = false
    }
  }

  function getRules(category?: string, sort?: string) {
    const params = new URLSearchParams()
    if (category) params.set('category', category)
    if (sort) params.set('sort', sort)
    return request<Rule[]>(`/api/rules?${params.toString()}`)
  }

  function createRule(data: Partial<Rule> & { turnstile_token: string }) {
    return request<Rule>('/api/rules', { method: 'POST', body: JSON.stringify(data) })
  }

  function vote(id: string, data: { type: 'up' | 'down'; turnstile_token: string }) {
    return request<{ upvotes: number; downvotes: number }>(`/api/rules/${id}/vote`, { method: 'POST', body: JSON.stringify(data) })
  }

  function getComments(ruleId: string) {
    return request<Comment[]>(`/api/rules/${ruleId}/comments`)
  }

  function addComment(ruleId: string, data: { author: string; content: string; turnstile_token: string }) {
    return request<Comment>(`/api/rules/${ruleId}/comments`, { method: 'POST', body: JSON.stringify(data) })
  }

  return { loading, getRules, createRule, vote, getComments, addComment }
}
