import { ref, watch } from 'vue'

export interface Tag {
  id: string
  name: string
}

const DEFAULT_TAGS: Tag[] = [
  { id: 'avatar', name: '头像' },
  { id: 'anime', name: '二次元' },
  { id: 'landscape', name: '风景' },
  { id: 'portrait', name: '人像' },
  { id: 'adaptive', name: '自适应' },
  { id: 'ai-generated', name: 'AI生成' },
]

const STORAGE_KEY = 'pichub_tags_v1'
const MASTER_BOUND_KEY = 'pichub_master_bound_tags_v1'

function loadTags(): Tag[] {
  const saved = localStorage.getItem(STORAGE_KEY)
  if (saved) {
    try {
      return JSON.parse(saved)
    } catch {}
  }
  return DEFAULT_TAGS
}

function loadMasterBound(): string[] {
  const saved = localStorage.getItem(MASTER_BOUND_KEY)
  if (saved) {
    try {
      return JSON.parse(saved)
    } catch {}
  }
  return [] // Empty list means all tags bound
}

const tags = ref<Tag[]>(loadTags())
const masterBoundTags = ref<string[]>(loadMasterBound())

watch(tags, (newTags) => {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(newTags))
}, { deep: true })

watch(masterBoundTags, (newBound) => {
  localStorage.setItem(MASTER_BOUND_KEY, JSON.stringify(newBound))
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
    const bIdx = masterBoundTags.value.indexOf(id)
    if (bIdx >= 0) {
      masterBoundTags.value.splice(bIdx, 1)
    }
  }

  function setMasterBoundTags(boundIds: string[]) {
    masterBoundTags.value = boundIds
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
    masterBoundTags,
    addTag,
    renameTag,
    deleteTag,
    setMasterBoundTags,
    getCategoryMap
  }
}
