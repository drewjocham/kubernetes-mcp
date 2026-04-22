<template>
  <div class="floating-bottom-toolbar">
    <div class="toolbar-section">
      <div class="cluster-info">
        <div class="cluster-header">
          <p class="meta-label">Active cluster</p>
          <div class="cluster-title-row">
            <h3>{{ clusterLabel || currentContext }}</h3>
            <div class="status-indicator inline">
              <span :class="['status-dot', connectivityStatus]"></span>
              <span class="status-text">{{ connectivityStatus === 'green' ? 'Connected' : 'Degraded' }}</span>
            </div>
          </div>
        </div>
        <button v-if="clusterLabel" class="ghost-btn subtle small" @click="emit('edit-cluster-label')">
          Edit label
        </button>
        <button v-else class="ghost-btn subtle small" @click="emit('edit-cluster-label')">
          Add label
        </button>
      </div>
    </div>

    <div class="toolbar-section">
      <div class="endpoint-section">
        <p class="meta-label">MCP endpoint</p>
        <div class="endpoint-row">
          <code class="endpoint">{{ endpoint }}</code>
          <button class="copy-btn" @click="copyEndpoint" title="Copy to clipboard">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect x="8" y="8" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.5"/>
              <rect x="4" y="4" width="12" height="12" rx="2" stroke="currentColor" stroke-width="1.5"/>
            </svg>
          </button>
        </div>
      </div>
    </div>

    <div class="toolbar-section actions">
      <button class="theme-toggle ghost-btn subtle" @click="toggleTheme && toggleTheme()" :title="currentTheme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'">
        <span class="theme-icon">{{ currentTheme === 'light' ? '🌙' : '☀️' }}</span>
        <span class="theme-label">{{ currentTheme === 'light' ? 'Dark' : 'Light' }} mode</span>
      </button>
      <button class="settings-btn" @click="emit('open-settings')" title="Settings">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M12 15C13.6569 15 15 13.6569 15 12C15 10.3431 13.6569 9 12 9C10.3431 9 9 10.3431 9 12C9 13.6569 10.3431 15 12 15Z" stroke="currentColor" stroke-width="1.5"/>
          <path d="M19.4 15C19.2669 15.3052 19.1339 15.6104 19.0008 15.9155C18.914 16.1171 18.8272 16.3187 18.7404 16.5203C18.6073 16.8255 18.4743 17.1306 18.3412 17.4358C17.8074 18.6241 17.2736 19.8124 16.7398 21.0008C16.499 21.552 16.2582 22.1032 16.0174 22.6544C15.883 22.9577 15.7486 23.261 15.6142 23.5643C15.5274 23.7659 15.4406 23.9675 15.3538 24.1691C15.2207 24.4743 15.0877 24.7794 14.9546 25.0846C14.8215 25.3898 14.6885 25.6949 14.5554 26.0001H9.44458C9.31146 25.6949 9.17835 25.3898 9.04523 25.0846C8.91212 24.7794 8.779 24.4743 8.64589 24.1691C8.55909 23.9675 8.47229 23.7659 8.38549 23.5643C8.25109 23.261 8.11669 22.9577 7.98229 22.6544C7.74149 22.1032 7.50069 21.552 7.25989 21.0008C6.72609 19.8124 6.19229 18.6241 5.65849 17.4358C5.52538 17.1306 5.39226 16.8255 5.25915 16.5203C5.17235 16.3187 5.08555 16.1171 4.99875 15.9155C4.86564 15.6104 4.73252 15.3052 4.59941 15C4.73252 14.6948 4.86564 14.3896 4.99875 14.0845C5.08555 13.8829 5.17235 13.6813 5.25915 13.4797C5.39226 13.1745 5.52538 12.8694 5.65849 12.5642C6.19229 11.3759 6.72609 10.1876 7.25989 9C7.50069 8.448 7.74149 7.8968 7.98229 7.3456C8.11669 7.0423 8.25109 6.739 8.38549 6.4357C8.47229 6.2341 8.55909 6.0325 8.64589 5.8309C8.779 5.5257 8.91212 5.2206 9.04523 4.9154C9.17835 4.6102 9.31146 4.3051 9.44458 4H14.5554C14.6885 4.3051 14.8215 4.6102 14.9546 4.9154C15.0877 5.2206 15.2207 5.5257 15.3538 5.8309C15.4406 6.0325 15.5274 6.2341 15.6142 6.4357C15.7486 6.739 15.883 7.0423 16.0174 7.3456C16.2582 7.8968 16.499 8.448 16.7398 9C17.2736 10.1876 17.8074 11.3759 18.3412 12.5642C18.4743 12.8694 18.6073 13.1745 18.7404 13.4797C18.8272 13.6813 18.914 13.8829 19.0008 14.0845C19.1339 14.3896 19.2669 14.6948 19.4 15Z" stroke="currentColor" stroke-width="1.5"/>
        </svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  currentContext: string
  clusterLabel?: string
  endpoint: string
  connectivityStatus: string
  toggleTheme?: () => void
  currentTheme?: string
}

const props = withDefaults(defineProps<Props>(), {
  clusterLabel: undefined,
  toggleTheme: undefined,
  currentTheme: undefined
})

const emit = defineEmits<{
  'edit-cluster-label': []
  'open-settings': []
}>()

const copyEndpoint = async () => {
  try {
    await navigator.clipboard.writeText(props.endpoint)
  } catch (err) {
    console.error('Failed to copy endpoint:', err)
  }
}
</script>

<style scoped>
.floating-bottom-toolbar {
  position: absolute;
  bottom: -40px;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  gap: 24px;
  align-items: center;
  padding: 12px 24px;
  background: var(--glass-card-surface);
  backdrop-filter: blur(25px);
  -webkit-backdrop-filter: blur(25px);
  border-radius: 20px;
  border: var(--glass-border-rim);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.15);
  z-index: 5;
}

.theme-light .floating-bottom-toolbar {
  background: var(--glass-light-bg);
  border: var(--glass-light-border);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.1);
}

.toolbar-section {
  display: flex;
  align-items: center;
  gap: 16px;
}

.toolbar-section.actions {
  gap: 12px;
}

.cluster-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cluster-header {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.meta-label {
  margin: 0;
  color: var(--text-secondary);
  font-size: 0.7rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

.cluster-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cluster-title-row h3 {
  margin: 0;
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--text);
}

.status-indicator.inline {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.green {
  background: var(--good);
}

.status-dot.yellow {
  background: var(--warning);
}

.status-dot.red {
  background: var(--danger);
}

.status-text {
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.ghost-btn.subtle.small {
  padding: 4px 8px;
  font-size: 0.75rem;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
}

.ghost-btn.subtle.small:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.endpoint-section {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.endpoint-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.endpoint {
  font-size: 0.8rem;
  color: var(--text);
  background: var(--input-bg);
  padding: 4px 8px;
  border-radius: 6px;
  font-family: "SFMono-Regular", "SF Mono", Consolas, "Liberation Mono", Menlo, monospace;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.copy-btn {
  padding: 4px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.copy-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.theme-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
}

.theme-toggle:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.theme-icon {
  font-size: 16px;
}

.theme-label {
  font-size: 0.8rem;
  font-weight: 500;
}

.settings-btn {
  padding: 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
}

.settings-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}
</style>