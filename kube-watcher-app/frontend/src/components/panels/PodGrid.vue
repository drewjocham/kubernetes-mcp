<template>
  <section class="pod-grid-panel">
    <div class="panel-head">
      <div>
        <p class="meta-label">Pod cluster view</p>
        <h3>All pods across namespaces</h3>
      </div>
      <div class="head-actions">
        <div class="namespace-filter">
          <label for="namespace-select">Namespace:</label>
          <select id="namespace-select" v-model="selectedNamespace" @change="fetchPods">
            <option value="">All namespaces</option>
            <option v-for="ns in namespaces" :key="ns" :value="ns">{{ ns }}</option>
          </select>
        </div>
        <button class="ghost-btn refresh-btn" @click="fetchPods" :disabled="loading">
          {{ loading ? 'Refreshing...' : 'Refresh' }}
        </button>
        <button class="primary-btn deploy-btn" @click="emit('deploy-anomstack')">
          Deploy Anomstack
        </button>
      </div>
    </div>
    <div v-if="loading" class="loading">Loading pods...</div>
    <div v-else-if="error" class="error">{{ error }}</div>
    <div v-else class="pods-container">
      <div class="pods-grid">
        <div v-for="pod in pods" :key="pod.namespace + '/' + pod.name" :class="['pod-card', getStatusClass(pod)]">
          <div class="pod-header">
            <span :class="['status-dot', getStatusClass(pod)]"></span>
            <div class="pod-title">
              <strong>{{ pod.name }}</strong>
              <span class="pod-namespace">{{ pod.namespace }}</span>
            </div>
            <button class="icon-btn" @click="toggleExpand(pod)">
               {{ expandedPod === `${pod.namespace}/${pod.name}` ? '−' : '+' }}
            </button>
          </div>
          <div class="pod-info">
            <div class="pod-row">
              <span class="label">Status:</span>
              <span class="value">{{ pod.status }}</span>
            </div>
            <div class="pod-row">
              <span class="label">Phase:</span>
              <span class="value">{{ pod.phase }}</span>
            </div>
            <div class="pod-row">
              <span class="label">Node:</span>
              <span class="value">{{ pod.nodeName || 'N/A' }}</span>
            </div>
            <div class="pod-row">
              <span class="label">Age:</span>
              <span class="value">{{ formatAge(pod.age) }}</span>
            </div>
            <div class="pod-row">
              <span class="label">Restarts:</span>
              <span class="value">{{ pod.restartCount }}</span>
            </div>
          </div>
           <div v-if="expandedPod === `${pod.namespace}/${pod.name}`" class="pod-expanded">
            <div class="containers">
              <h4>Containers</h4>
              <div v-for="container in pod.containers" :key="container.name" class="container">
                <div class="container-row">
                  <span class="label">{{ container.name }}</span>
                  <span :class="['container-status', container.ready ? 'ready' : 'not-ready']">
                    {{ container.ready ? 'Ready' : 'Not Ready' }}
                  </span>
                  <span class="container-image">{{ container.image }}</span>
                  <span class="container-restarts">Restarts: {{ container.restartCount }}</span>
                </div>
              </div>
            </div>
            <div class="pod-actions">
              <button class="small-btn" @click="viewLogs(pod)">Logs</button>
              <button class="small-btn" @click="viewYAML(pod)">YAML</button>
              <button class="small-btn" @click="describePod(pod)">Describe</button>
              <button class="small-btn ai-help" @click="getAIHelp(pod)">AI Help</button>
              <button class="small-btn shell-btn" @click="openShell(pod)">Shell</button>
              <button v-if="getStatusClass(pod) === 'red'" class="small-btn restart-btn" @click="restartPod(pod)" :disabled="restartingPod === `${pod.namespace}/${pod.name}`" title="Delete pod to trigger recreation by its deployment">{{ restartingPod === `${pod.namespace}/${pod.name}` ? 'Restarting...' : 'Restart' }}</button>
            </div>
          </div>
        </div>
      </div>
       <div v-if="pods.length > 5" class="pods-message">
         {{ pods.length }} pods total. Scroll to see all.
       </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { GetNamespaces, GetPods, GetPodLogs, GetPodYAML, DescribePod, GetPodAIHelp, RunCommand } from '../../../wailsjs/go/main/App'
import { data } from '../../../wailsjs/go/models'

const props = defineProps<{
  initialNamespace?: string
}>()

const emit = defineEmits<{
  'deploy-anomstack': []
  'open-logs': [pod: data.PodInfo, logs: string]
  'open-yaml': [pod: data.PodInfo, yaml: string]
  'open-describe': [pod: data.PodInfo, describe: string]
  'open-ai-help': [pod: data.PodInfo, help: string]
  'open-shell': [pod: data.PodInfo]
}>()

const namespaces = ref<string[]>([])
const selectedNamespace = ref(props.initialNamespace || '')
const pods = ref<data.PodInfo[]>([])
const loading = ref(false)
const error = ref('')
const expandedPod = ref('')
const restartingPod = ref('')


onMounted(() => {
  fetchNamespaces()
  fetchPods()
})

async function fetchNamespaces() {
  try {
    namespaces.value = await GetNamespaces()
  } catch (err) {
    console.error('Failed to fetch namespaces:', err)
  }
}

async function fetchPods() {
  loading.value = true
  error.value = ''
  try {
    pods.value = await GetPods(selectedNamespace.value)
  } catch (err) {
    error.value = 'Failed to fetch pods: ' + (err instanceof Error ? err.message : String(err))
    console.error(err)
  } finally {
    loading.value = false
  }
}

function getStatusClass(pod: data.PodInfo): string {
  if (pod.phase === 'Running' && pod.status?.toLowerCase().includes('running')) {
    return 'green'
  } else if (pod.phase === 'Pending' || pod.phase === 'ContainerCreating') {
    return 'yellow'
  } else {
    return 'red'
  }
}

function formatAge(duration: any): string {
  // duration is a string like "5h30m" or a number of nanoseconds?
  if (typeof duration === 'string') return duration
  if (typeof duration === 'number') {
    // Convert nanoseconds to hours/minutes/seconds
    const seconds = Math.floor(duration / 1e9)
    if (seconds < 60) return `${seconds}s`
    const minutes = Math.floor(seconds / 60)
    if (minutes < 60) return `${minutes}m`
    const hours = Math.floor(minutes / 60)
    const remainingMinutes = minutes % 60
    return `${hours}h${remainingMinutes}m`
  }
  return '?'
}

function toggleExpand(pod: data.PodInfo) {
  const podKey = `${pod.namespace}/${pod.name}`
  console.log('toggleExpand called for pod:', podKey, 'current expandedPod:', expandedPod.value)
  expandedPod.value = expandedPod.value === podKey ? '' : podKey
  console.log('new expandedPod:', expandedPod.value)
}

async function viewLogs(pod: data.PodInfo) {
  console.log('viewLogs called for pod:', pod.namespace, pod.name)
  try {
    const logs = await GetPodLogs(pod.namespace, pod.name, '')
    console.log('Logs fetched, length:', logs.length)
    emit('open-logs', pod, logs)
  } catch (err) {
    console.error('Failed to fetch logs:', err)
    emit('open-logs', pod, 'Error: ' + (err instanceof Error ? err.message : String(err)))
  }
}

async function viewYAML(pod: data.PodInfo) {
  console.log('viewYAML called for pod:', pod.namespace, pod.name)
  try {
    const yaml = await GetPodYAML(pod.namespace, pod.name)
    console.log('YAML fetched, length:', yaml.length)
    emit('open-yaml', pod, yaml)
  } catch (err) {
    console.error('Failed to fetch YAML:', err)
    emit('open-yaml', pod, 'Error: ' + (err instanceof Error ? err.message : String(err)))
  }
}

async function describePod(pod: data.PodInfo) {
  console.log('describePod called for pod:', pod.namespace, pod.name)
  try {
    const describe = await DescribePod(pod.namespace, pod.name)
    console.log('Describe fetched, length:', describe.length)
    emit('open-describe', pod, describe)
  } catch (err) {
    console.error('Failed to describe pod:', err)
    emit('open-describe', pod, 'Error: ' + (err instanceof Error ? err.message : String(err)))
  }
}

async function getAIHelp(pod: data.PodInfo) {
  console.log('getAIHelp called for pod:', pod.namespace, pod.name)
  try {
    const help = await GetPodAIHelp(pod.namespace, pod.name)
    console.log('AI Help fetched, length:', help.length)
    emit('open-ai-help', pod, help)
  } catch (err) {
    console.error('Failed to get AI help:', err)
    emit('open-ai-help', pod, 'Error: ' + (err instanceof Error ? err.message : String(err)))
  }
}

function openShell(pod: data.PodInfo) {
  console.log('openShell called for pod:', pod.namespace, pod.name)
  emit('open-shell', pod)
}

async function restartPod(pod: data.PodInfo) {
  console.log('restartPod called for pod:', pod.namespace, pod.name)
  const podKey = `${pod.namespace}/${pod.name}`
  const confirmed = window.confirm(`Restart (delete) pod "${pod.name}" in namespace "${pod.namespace}"? This will trigger recreation by its deployment.`)
  if (!confirmed) {
    return
  }
  restartingPod.value = podKey
  const command = `kubectl delete pod ${pod.name} -n ${pod.namespace} --wait=false`
  try {
    await RunCommand(command)
    // Refresh pods after a short delay
    setTimeout(() => {
      fetchPods()
    }, 1000)
  } catch (err) {
    console.error('Failed to restart pod:', err)
    alert(`Failed to restart pod: ${err instanceof Error ? err.message : String(err)}`)
  } finally {
    restartingPod.value = ''
  }
}
</script>

<style scoped>
.pod-grid-panel {
  width: 100%;
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  --pod-card-min-height: 180px;
  --grid-gap: 16px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.head-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.namespace-filter {
  display: flex;
  align-items: center;
  gap: 8px;
}

.namespace-filter label {
  font-size: 14px;
  color: var(--text-secondary);
}

.namespace-filter select {
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
}

.pods-container {
  max-height: calc(2 * (var(--pod-card-min-height) + var(--grid-gap)) + 20px);
  overflow-y: auto;
  overflow-x: hidden;
  padding-right: 8px;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}

.pods-container::-webkit-scrollbar {
  width: 8px;
}

.pods-container::-webkit-scrollbar-track {
  background: transparent;
}

.pods-container::-webkit-scrollbar-thumb {
  background-color: var(--border);
  border-radius: 4px;
}

.pods-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(300px, 1fr));
  gap: var(--grid-gap);
}

@media (max-width: 1200px) {
  .pods-grid {
    grid-template-columns: repeat(2, minmax(300px, 1fr));
  }
}

@media (max-width: 800px) {
  .pods-grid {
    grid-template-columns: 1fr;
  }
}

.pods-message {
  margin-top: 12px;
  font-size: 14px;
  color: var(--text-secondary);
  text-align: center;
  padding: 8px;
  border-top: 1px dashed var(--border);
}

.pod-card {
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
  background: var(--card-bg);
}

.pod-card.green {
  border-color: #10b981;
  box-shadow: 0 0 0 1px #10b98120;
}

.pod-card.yellow {
  border-color: #f59e0b;
  box-shadow: 0 0 0 1px #f59e0b20;
}

.pod-card.red {
  border-color: #ef4444;
  box-shadow: 0 0 0 1px #ef444420;
}

.pod-header {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 12px;
}

.status-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-dot.green {
  background-color: #10b981;
}

.status-dot.yellow {
  background-color: #f59e0b;
}

.status-dot.red {
  background-color: #ef4444;
}

.pod-title {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.pod-title strong {
  font-size: 16px;
  line-height: 1.2;
}

.pod-namespace {
  font-size: 12px;
  color: var(--text-secondary);
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 18px;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
}

.icon-btn:hover {
  background: var(--hover);
}

.pod-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}

.pod-row {
  display: flex;
  justify-content: space-between;
}

.pod-row .label {
  color: var(--text-secondary);
}

.pod-row .value {
  font-family: 'Monaco', monospace;
  color: var(--text-primary);
}

.pod-expanded {
  margin-top: 16px;
  padding-top: 16px;
  border-top: 1px solid var(--border);
}

.containers h4 {
  margin: 0 0 8px 0;
  font-size: 14px;
}

.container {
  margin-bottom: 8px;
}

.container-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.container-status {
  padding: 2px 6px;
  border-radius: 4px;
  font-weight: bold;
}

.container-status.ready {
  background: #10b98120;
  color: #10b981;
}

.container-status.not-ready {
  background: #ef444420;
  color: #ef4444;
}

.container-image {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--text-secondary);
}

.pod-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  margin-top: 12px;
  position: relative;
  z-index: 1;
}

.small-btn {
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.small-btn:hover {
  background: var(--hover);
  transform: scale(1.05);
}

.small-btn.ai-help {
  background: var(--accent);
  color: white;
  border-color: var(--accent);
}

.small-btn.shell-btn {
  background: #10b981;
  color: white;
  border-color: #10b981;
}

.small-btn.restart-btn {
  background: #ef4444;
  color: white;
  border-color: #ef4444;
}

.small-btn.restart-btn:hover {
  background: #dc2626;
  border-color: #dc2626;
}

.loading, .error {
  text-align: center;
  padding: 40px;
  color: var(--text-secondary);
}

.error {
  color: #ef4444;
}

.primary-btn {
  padding: 8px 16px;
  border-radius: 8px;
  background: var(--primary);
  color: white;
  border: none;
  font-weight: 500;
  cursor: pointer;
  white-space: nowrap;
  min-width: 120px;
  text-align: center;
}

.ghost-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  white-space: nowrap;
  min-width: 80px;
  text-align: center;
}

.meta-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin: 0 0 4px 0;
}

h3 {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  line-height: 1.3;
}
</style>