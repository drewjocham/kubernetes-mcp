import { ref, watch, onMounted } from 'vue'

type Theme = 'dark' | 'light'

export function useTheme() {
  const theme = ref<Theme>('dark')

  const setTheme = (newTheme: Theme) => {
    theme.value = newTheme
    localStorage.setItem('theme', newTheme)
    updateHtmlClass()
  }

  const toggleTheme = () => {
    setTheme(theme.value === 'dark' ? 'light' : 'dark')
  }

  const updateHtmlClass = () => {
    const html = document.documentElement
    if (theme.value === 'light') {
      html.classList.add('theme-light')
    } else {
      html.classList.remove('theme-light')
    }
  }

  onMounted(() => {
    const saved = localStorage.getItem('theme') as Theme | null
    if (saved && (saved === 'dark' || saved === 'light')) {
      theme.value = saved
    } else if (window.matchMedia && window.matchMedia('(prefers-color-scheme: light)').matches) {
      theme.value = 'light'
    }
    updateHtmlClass()
  })

  watch(theme, updateHtmlClass)

  return {
    theme,
    setTheme,
    toggleTheme,
  }
}