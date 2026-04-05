import { ref } from 'vue'

export function useSettings() {
  const showSettings = ref(false)
  const settings = ref({
    context: '',
    prometheusRoutes: [] as string[],
    webhookUrls: [] as string[],
  })

  function loadSettings() {
    const savedSettings = localStorage.getItem('kube-watcher-settings')
    if (savedSettings) {
      try {
        settings.value = { ...settings.value, ...JSON.parse(savedSettings) }
      } catch (e) {
        console.error('Failed to parse settings', e)
      }
    }
  }

  function saveSettings() {
    localStorage.setItem('kube-watcher-settings', JSON.stringify(settings.value))
    showSettings.value = false
  }

  return {
    showSettings,
    settings,
    loadSettings,
    saveSettings,
  }
}