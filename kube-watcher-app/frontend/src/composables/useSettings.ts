import { ref } from 'vue'
import { UpdateAIConfig } from '../../wailsjs/go/main/App'

export function useSettings() {
  const showSettings = ref(false)
  const settings = ref({
    context: '',
    prometheusRoutes: [] as string[],
    webhookUrls: [] as string[],
    aiProvider: 'openai',
    aiApiKey: '',
    aiBaseUrl: '',
    aiModel: 'gpt-3.5-turbo',
    aiBackend: 'openai',
    clusterType: 'real', // 'real' or 'local'
  })

  async function loadSettings() {
    const savedSettings = localStorage.getItem('kube-watcher-settings')
    if (savedSettings) {
      try {
        settings.value = { ...settings.value, ...JSON.parse(savedSettings) }
        await UpdateAIConfig(
          settings.value.aiProvider,
          settings.value.aiApiKey,
          settings.value.aiBaseUrl,
          settings.value.aiModel,
          settings.value.aiBackend
        )
      } catch (e) {
        console.error('Failed to parse settings or update AI config', e)
      }
    }
  }

  async function saveSettings(newSettings?: typeof settings.value) {
    if (newSettings) {
      settings.value = newSettings
    }
    localStorage.setItem('kube-watcher-settings', JSON.stringify(settings.value))
    try {
      await UpdateAIConfig(
        settings.value.aiProvider,
        settings.value.aiApiKey,
        settings.value.aiBaseUrl,
        settings.value.aiModel,
        settings.value.aiBackend
      )
    } catch (err) {
      console.error('Failed to update AI config in backend:', err)
    }
    showSettings.value = false
  }

  return {
    showSettings,
    settings,
    loadSettings,
    saveSettings,
  }
}