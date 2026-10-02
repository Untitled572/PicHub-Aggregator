import { ref } from 'vue'
import { useApi } from './useApi'
import type { Tag } from '../types'

const DEFAULT_TAGS: Tag[] = [
  { id: 'horizontal', name: '横屏', system: true },
  { id: 'vertical', name: '竖屏', system: true },
  { id: 'adaptive', name: '自适应', system: true },
]


const tags = ref<Tag[]>([...DEFAULT_TAGS])
let loaded = false

export function useTags() {
  async function loadTags() {
    if (loaded) return
    try {
      const data = await useApi().getTags()
      {
        if (Array.isArray(data) && data.length > 0) {
          tags.value = data
        }
      }
      loaded = true
    } catch {}
  }

  async function saveTags() {
    try {
      await useApi().updateTags(tags.value)
    } catch {}
  }

  function addTag(id: string, name: string) {
    const cleanId = id.trim().toLowerCase()
    const cleanName = name.trim() || cleanId
    if (!cleanId) return
    const existing = tags.value.find(t => t.id === cleanId)
    if (existing) {
      existing.name = cleanName
    } else {
      tags.value.push({ id: cleanId, name: cleanName })
    }
    saveTags()
  }

  function renameTag(id: string, newName: string) {
    const target = tags.value.find(t => t.id === id)
    if (target) {
      target.name = newName.trim()
      saveTags()
    }
  }

  function deleteTag(id: string) {
    const idx = tags.value.findIndex(t => t.id === id)
    if (idx >= 0 && !tags.value[idx].system) {
      tags.value.splice(idx, 1)
      saveTags()
    }
  }

  function toggleExclusive(id: string) {
    const target = tags.value.find(t => t.id === id)
    if (target) {
      target.exclusive = !target.exclusive
      saveTags()
    }
  }

  function getCategoryMap(): Record<string, string> {
    const map: Record<string, string> = {}
    for (const t of tags.value) {
      map[t.id] = t.name
    }
    return map
  }

  return {
    tags,
    loadTags,
    addTag,
    renameTag,
    deleteTag,
    toggleExclusive,
    getCategoryMap
  }
}
