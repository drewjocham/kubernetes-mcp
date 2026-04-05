<template>
  <div v-if="show" class="settings-modal-overlay" @click="closeModal">
    <div class="settings-modal" @click.stop>
      <div class="settings-modal-header">
        <h3>Settings</h3>
         <button class="settings-close-btn" @click="closeModal">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="settings-modal-content">
        <div class="settings-section">
          <label class="settings-label">Kubernetes Context</label>
          <input
            v-model="localSettings.context"
            type="text"
            class="settings-input"
            placeholder="Enter Kubernetes context name"
          >
        </div>

        <div class="settings-section">
          <label class="settings-label">Prometheus Scraping Routes</label>
          <div class="settings-array-input">
            <div v-for="(route, index) in localSettings.prometheusRoutes" :key="index" class="settings-array-item">
              <input
                v-model="localSettings.prometheusRoutes[index]"
                type="url"
                class="settings-input"
                placeholder="https://prometheus.example.com"
              >
              <button class="settings-remove-btn" @click="removeRoute(index)">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                </svg>
              </button>
            </div>
            <button class="settings-add-btn" @click="addRoute">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 5V19M5 12H19" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              Add Route
            </button>
          </div>
        </div>

        <div class="settings-section">
          <label class="settings-label">Webhook URLs</label>
          <div class="settings-array-input">
            <div v-for="(url, index) in localSettings.webhookUrls" :key="index" class="settings-array-item">
              <input
                v-model="localSettings.webhookUrls[index]"
                type="url"
                class="settings-input"
                placeholder="https://webhook.example.com"
              >
              <button class="settings-remove-btn" @click="removeWebhook(index)">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                </svg>
              </button>
            </div>
            <button class="settings-add-btn" @click="addWebhook">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 5V19M5 12H19" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              Add URL
            </button>
          </div>
        </div>
      </div>

      <div class="settings-modal-footer">
         <button class="ghost-btn" @click="closeModal">Cancel</button>
        <button class="primary-btn" @click="saveSettings">Save Settings</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

interface Settings {
  context: string
  prometheusRoutes: string[]
  webhookUrls: string[]
}

interface Props {
  show: boolean
  settings: Settings
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:show': [value: boolean]
  close: []
  save: [settings: Settings]
}>()

function closeModal() {
  emit('update:show', false)
  emit('close')
}

const localSettings = ref<Settings>({ ...props.settings })

watch(() => props.settings, (newSettings) => {
  localSettings.value = { ...newSettings }
}, { deep: true })

function addRoute() {
  localSettings.value.prometheusRoutes.push('')
}

function removeRoute(index: number) {
  localSettings.value.prometheusRoutes.splice(index, 1)
}

function addWebhook() {
  localSettings.value.webhookUrls.push('')
}

function removeWebhook(index: number) {
  localSettings.value.webhookUrls.splice(index, 1)
}

function saveSettings() {
  emit('save', localSettings.value)
}
</script>

<style scoped>
.settings-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.settings-modal {
  background: var(--modal-bg);
  border-radius: 24px;
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border);
}

.settings-modal-header {
  padding: 24px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.settings-modal-header h3 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.settings-close-btn {
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.settings-close-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.settings-modal-content {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.settings-section {
  margin-bottom: 32px;
}

.settings-section:last-child {
  margin-bottom: 0;
}

.settings-label {
  display: block;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  margin-bottom: 8px;
}

.settings-input {
  width: 100%;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-family: inherit;
  font-size: 14px;
  transition: border-color 0.2s;
}

.settings-input:focus {
  outline: none;
  border-color: var(--border-active);
}

.settings-input::placeholder {
  color: var(--text-tertiary);
}

.settings-array-input {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.settings-array-item {
  display: flex;
  gap: 8px;
  align-items: center;
}

.settings-array-item .settings-input {
  flex: 1;
}

.settings-remove-btn {
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.settings-remove-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.settings-add-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
  align-self: flex-start;
}

.settings-add-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.settings-modal-footer {
  padding: 24px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.ghost-btn {
  padding: 10px 20px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.ghost-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.primary-btn {
  padding: 10px 20px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.primary-btn:hover {
  background: var(--primary-hover);
}
</style>