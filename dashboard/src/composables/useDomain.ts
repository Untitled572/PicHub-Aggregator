import { ref, watch, computed } from 'vue'

const DOMAIN_STORAGE_KEY = 'pichub_custom_domain_v1'

const customDomain = ref<string>(localStorage.getItem(DOMAIN_STORAGE_KEY) || '')

watch(customDomain, (val) => {
  localStorage.setItem(DOMAIN_STORAGE_KEY, val)
})

export function useDomain() {
  function setCustomDomain(domain: string) {
    customDomain.value = domain.trim()
  }

  function getEffectiveDomain(): string {
    const raw = customDomain.value.trim()
    if (!raw) return window.location.origin
    let formatted = raw
    if (!formatted.startsWith('http://') && !formatted.startsWith('https://')) {
      formatted = 'https://' + formatted
    }
    return formatted.replace(/\/+$/, '')
  }

  const effectiveDomain = computed(() => getEffectiveDomain())

  return {
    customDomain,
    effectiveDomain,
    setCustomDomain,
    getEffectiveDomain
  }
}
