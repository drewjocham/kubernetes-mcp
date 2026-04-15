<template>
  <div class="watcher-panel">
    <div class="panel-head">
      <h2>Watcher</h2>
      <p class="subtitle">Monitor Kubernetes operations, rules, and resources</p>
    </div>

    <div class="watcher-content">
      <div class="watcher-section">
        <h3>Status</h3>
        <div v-if="status" class="status-box">
          <div class="status-item">
            <span class="label">Ready:</span>
            <span :class="['badge', status.ready ? 'success' : 'error']">
              {{ status.ready ? 'Yes' : 'No' }}
            </span>
          </div>
          <div class="status-item">
            <span class="label">Store:</span>
            <span class="badge">{{ status.store ? 'Available' : 'Unavailable' }}</span>
          </div>
          <div class="status-item">
            <span class="label">Config:</span>
            <span class="badge">{{ status.config ? 'Loaded' : 'Missing' }}</span>
          </div>
          <div class="status-item">
            <span class="label">Started:</span>
            <span>{{ formatTime(status.started) }}</span>
          </div>
        </div>
        <div v-else class="loading">Loading status...</div>
      </div>

      <div class="watcher-section">
        <h3>Rules ({{ rules.length }})</h3>
        <div v-if="rules.length > 0" class="rules-list">
          <div v-for="rule in rules" :key="rule.name" class="rule-item">
            <strong>{{ rule.name }}</strong>
            <span class="rule-kind">{{ rule.kind }}</span>
            <span class="rule-actions">{{ rule.actions?.length }} actions</span>
          </div>
        </div>
        <div v-else class="empty">No rules defined</div>
      </div>

      <div class="watcher-section">
        <h3>Tracked Resources ({{ resources.length }})</h3>
        <div v-if="resources.length > 0" class="resources-list">
          <div v-for="resource in resources" :key="resource" class="resource-item">
            {{ resource }}
          </div>
        </div>
        <div v-else class="empty">No resources being tracked</div>
      </div>

      <div class="watcher-section logs-section">
        <LogSentinelPanel />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, nextTick } from 'vue'
import { GetWatcherStatus, GetWatcherRules, GetWatcherResources, StreamWatcherLogs } from '../../../wailsjs/go/main/App'
import LogSentinelPanel from './LogSentinelPanel.vue'

const status = ref<any>(null)
const rules = ref<any[]>([])
const resources = ref<string[]>([])
const logs = ref<string[]>([])
const logsActive = ref(false)
const logsContainer = ref<HTMLElement>()
let eventSource: EventSource | null = null

onMounted(() => {
  loadData()
})

onUnmounted(() => {
  stopLogs()
})

async function loadData() {
  try {
    status.value = await GetWatcherStatus()
  } catch (err) {
    console.error('Failed to load watcher status:', err)
  }
  try {
    rules.value = await GetWatcherRules()
  } catch (err) {
    console.error('Failed to load watcher rules:', err)
  }
  try {
    resources.value = await GetWatcherResources()
  } catch (err) {
    console.error('Failed to load watcher resources:', err)
  }
}

function formatTime(timestamp: string | Date) {
  const date = new Date(timestamp)
  return date.toLocaleString()
}

function toggleLogs() {
  if (logsActive.value) {
    stopLogs()
  } else {
    startLogs()
  }
}

async function startLogs() {
  try {
    // Using EventSource for SSE
    const baseUrl = 'http://localhost:8085' // TODO: make configurable
    eventSource = new EventSource(`${baseUrl}/api/logs/stream`)
    eventSource.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        logs.value.push(JSON.stringify(data, null, 2))
        if (logs.value.length > 100) {
          logs.value.shift()
        }
        scrollToBottom()
      } catch (e) {
        logs.value.push(event.data)
      }
    }
    eventSource.onerror = (err) => {
      console.error('SSE error:', err)
      stopLogs()
    }
    logsActive.value = true
  } catch (err) {
    console.error('Failed to start logs:', err)
    logsActive.value = false
  }
}

function stopLogs() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
  }
  logsActive.value = false
}

function clearLogs() {
  logs.value = []
}

function scrollToBottom() {
  nextTick(() => {
    if (logsContainer.value) {
      logsContainer.value.scrollTop = logsContainer.value.scrollHeight
    }
  })
}
</script>

<style scoped>
.watcher-panel {
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

.watcher-section {
  margin-bottom: 2rem;
  padding: 1rem;
  background: var(--bg-card);
  border-radius: 6px;
  border: 1px solid var(--border);
}

.watcher-section h3 {
  margin-top: 0;
  margin-bottom: 1rem;
  font-size: 1.2rem;
  font-weight: 500;
}

.status-box {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 1rem;
}

.status-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--border-light);
}

.status-item:last-child {
  border-bottom: none;
}

.label {
  font-weight: 500;
}

.badge {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.8rem;
  font-weight: 600;
}

.badge.success {
  background: var(--success-bg);
  color: var(--success);
}

.badge.error {
  background: var(--error-bg);
  color: var(--error);
}

.rules-list, .resources-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rule-item, .resource-item {
  padding: 0.75rem;
  background: var(--bg-subtle);
  border-radius: 4px;
  border-left: 4px solid var(--accent);
  display: flex;
  align-items: center;
  gap: 1rem;
}

.rule-kind {
  font-size: 0.85rem;
  color: var(--text-secondary);
  background: var(--bg-tag);
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}

.rule-actions {
  margin-left: auto;
  font-size: 0.85rem;
  color: var(--text-tertiary);
}

.logs-section {
  padding: 1rem;
}

.logs-controls {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1rem;
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

.btn.secondary {
  background: var(--bg-subtle);
  color: var(--text);
}

.btn.danger {
  background: var(--error);
  color: white;
}

.logs-output {
  max-height: 400px;
  overflow-y: auto;
  background: var(--bg-code);
  border-radius: 4px;
  padding: 1rem;
  font-family: monospace;
  font-size: 0.9rem;
  white-space: pre-wrap;
}

.log-entry {
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--border-light);
}

.log-entry:last-child {
  border-bottom: none;
}

.loading, .empty {
  padding: 1rem;
  text-align: center;
  color: var(--text-secondary);
  font-style: italic;
}
</style>