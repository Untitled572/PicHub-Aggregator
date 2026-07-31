import { useColorMode } from '@vueuse/core'

// 主题骨架: 基于 @vueuse 的 useColorMode
// mode: 'light' | 'dark' | 'auto' (跟随系统), 自动持久化到 localStorage (vueuse-color-scheme)
// 深色配色适配尚未完成, 仅提供切换基础设施
export function useTheme() {
  const colorMode = useColorMode()

  function toggleDark() {
    colorMode.value = colorMode.value === 'dark' ? 'light' : 'dark'
  }

  function isDark() {
    return colorMode.value === 'dark'
  }

  return { colorMode, toggleDark, isDark }
}
