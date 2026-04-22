<template>
  <div class="config-editor">
    <div class="panel-head">
      <h2>Watcher Configuration</h2>
      <p class="subtitle">Edit and update watcher engine configuration</p>
    </div>

    <div class="config-content">
      <div class="config-controls">
        <button @click="loadConfig" class="btn primary">Load Config</button>
        <button @click="saveConfig" :disabled="!configYaml" class="btn success">Save Config</button>
        <button @click="resetConfig" class="btn secondary">Reset</button>
        <span v-if="message" :class="['message', messageType]">{{ message }}</span>
      </div>

      <div class="editor-section">
        <label for="config-yaml">Configuration YAML</label>
        <textarea
          id="config-yaml"
          v-model="configYaml"
          placeholder="Load configuration first..."
          rows="30"
          spellcheck="false"
        ></textarea>
      </div>

      <div class="info-section">
        <h3>Notes</h3>
        <ul>
          <li>Configuration is stored in the <code>watcher-config</code> ConfigMap in the <code>kube-watcher</code> namespace.</li>
          <li>After saving, the watcher engine will automatically reload the configuration (if configured to watch for ConfigMap changes).</li>
          <li>Ensure YAML syntax is valid before saving.</li>
          <li>Use the <code>Load Config</code> button to fetch the current configuration.</li>
        </ul>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetWatcherConfig, ExecuteTool } from '../../../wailsjs/go/main/App'

const configYaml = ref('')
const message = ref('')
const messageType = ref('info')

onMounted(() => {
  // Optionally load config on mount
  // loadConfig()
})

async function loadConfig() {
  try {
    const config = await GetWatcherConfig()
    // config is a JavaScript object; convert to YAML
    // For simplicity, we'll stringify as JSON, but we need YAML.
    // The watcher config endpoint returns JSON representation of config.
    // We'll need to convert to YAML. For now, just use JSON.stringify.
    configYaml.value = JSON.stringify(config, null, 2)
    showMessage('Configuration loaded successfully', 'success')
  } catch (err) {
    console.error('Failed to load config:', err)
    showMessage('Failed to load configuration: ' + (err as Error).message, 'error')
  }
}

async function saveConfig() {
  if (!configYaml.value.trim()) {
    showMessage('Configuration YAML cannot be empty', 'error')
    return
  }
  try {
    // Validate YAML syntax by parsing
    // We'll rely on the tool's validation.
    const result = await ExecuteTool('update_watcher_config', {
      config: configYaml.value,
      namespace: 'kube-watcher',
      configmap: 'watcher-config'
    })
    if (result.error) {
      throw new Error(result.error)
    }
    showMessage('Configuration saved successfully', 'success')
  } catch (err) {
    console.error('Failed to save config:', err)
    showMessage('Failed to save configuration: ' + (err as Error).message, 'error')
  }
}

function resetConfig() {
  configYaml.value = ''
  message.value = ''
}

function showMessage(text: string, type: 'info' | 'success' | 'error') {
  message.value = text
  messageType.value = type
  setTimeout(() => {
    message.value = ''
  }, 5000)
}
</script>

<style scoped>
.config-editor {
  padding: 1.5rem;
  background: var(--bg-panel);
  border-radius: 8px;
  height: 100%;
  overflow-y: auto;
}

.panel-head {
  margin-bottom: 2rem;
}

.panel-head h2 {
  margin: 0;
  font-size: 1.8rem;
  font-weight: 600;
}

.subtitle {
  margin: 0.5rem 0 0;
  color: var(--text-secondary);
  font-size: 0.9rem;
}

.config-content {
  display: flex;
  flex-direction: column;
  gap: 1.5rem;
}

.config-controls {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.btn {
  padding: 0.5rem 1rem;
  border: none;
  border-radius: 4px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.btn.primary {
  background: var(--primary);
  color: white;
}

.btn.success {
  background: var(--success);
  color: white;
}

.btn.secondary {
  background: var(--bg-subtle);
  color: var(--text);
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.message {
  padding: 0.5rem 1rem;
  border-radius: 4px;
  font-size: 0.9rem;
}

.message.info {
  background: var(--info-bg);
  color: var(--info);
}

.message.success {
  background: var(--success-bg);
  color: var(--success);
}

.message.error {
  background: var(--error-bg);
  color: var(--error);
}

.editor-section {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.editor-section label {
  font-weight: 500;
}

#config-yaml {
  font-family: monospace;
  font-size: 0.9rem;
  padding: 1rem;
  background: var(--bg-code);
  border: 1px solid var(--border);
  border-radius: 4px;
  resize: vertical;
  white-space: pre;
  overflow-wrap: normal;
  overflow-x: auto;
}

.info-section {
  padding: 1rem;
  background: var(--bg-subtle);
  border-radius: 4px;
  border-left: 4px solid var(--accent);
}

.info-section h3 {
  margin-top: 0;
  margin-bottom: 0.5rem;
}

.info-section ul {
  margin: 0;
  padding-left: 1.5rem;
}

.info-section li {
  margin-bottom: 0.25rem;
}

.info-section code {
  font-family: monospace;
  background: var(--bg-code);
  padding: 0.1rem 0.3rem;
  border-radius: 3px;
}
</style>