import { ref, watch } from 'vue'

export interface Tag {
  id: string
  name: string
}

const DEFAULT_TAGS: Tag[] = [
  { id: 'horizontal', name: '横屏' },
  { id: 'vertical', name: '竖屏' },
  { id: 'adaptive', name: '自适应' },
]


const STORAGE_KEY = 'pichub_tags_v1'

function loadTags(): Tag[] {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) {
    try {
      return JSON.parse(saved)
    } catch {}
  }
  return DEFAULT_TAGS
}

const tags = ref<Tag[]>(loadTags())

watch(tags, (newTags) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(newTags))
}, { deep: true })

export function useTags() {
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
  }

  function renameTag(id: string, newName: string) {
    const target = tags.value.find(t => t.id === id)
    if (target) {
      target.name = newName.trim()
    }
  }

  function deleteTag(id: string) {
    const idx = tags.value.findIndex(t => t.id === id)
    if (idx >= 0) {
      tags.value.splice(idx, 1)
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
    addTag,
    renameTag,
    deleteTag,
    getCategoryMap
  }
}
