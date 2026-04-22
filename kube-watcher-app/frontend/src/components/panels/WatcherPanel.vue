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
        <div v-if="rules.length > 0" class="rules-grid">
           <div v-for="rule in rules" :key="rule.name" class="rule-card" @click="selectedRule = rule">
             <div class="rule-card-header">
               <strong>{{ rule.name }}</strong>
               <span class="rule-kind">{{ rule.kind }}</span>
             </div>
             <div class="rule-card-body">
               <div class="rule-actions-count">
                 <span>{{ rule.actions?.length }} actions</span>
               </div>
               <div class="rule-sparkline">
                 <div class="sparkline-bars">
                   <div 
                     v-for="(value, idx) in getRuleTrend(rule)" 
                     :key="idx" 
                     class="sparkline-bar"
                     :style="{ height: Math.max(10, value * 0.5) + '%' }"
                     :title="`Day ${idx + 1}: ${value}`"
                   ></div>
                 </div>
               </div>
             </div>
           </div>
        </div>
        <div v-else class="empty">No rules defined</div>
      </div>

      <div class="watcher-section">
        <h3>Tracked Resources ({{ resources.length }})</h3>
        <div v-if="groupedResources.length > 0" class="resources-list">
            <div v-for="group in groupedResources" :key="group.key" class="resource-item"
                 @mouseenter="(e) => { hoveredResource = group.key; updateTooltipPosition(e) }"
                 @mousemove="updateTooltipPosition"
                 @mouseleave="hoveredResource = null"
                 @click="selectedResource = group">
             <div class="resource-main">
               <span class="resource-name">{{ group.kind }}/{{ group.namespace }}/{{ group.prefix }}</span>
               <span class="resource-count" v-if="group.count > 1">({{ group.count }})</span>
             </div>
             <div class="resource-sparkline">
               <div class="sparkline-bars">
                 <div 
                   v-for="(value, idx) in getResourceTrend(group)" 
                   :key="idx" 
                   class="sparkline-bar"
                   :style="{ height: Math.max(10, value * 0.5) + '%' }"
                   :title="`Day ${idx + 1}: ${value}`"
                 ></div>
               </div>
             </div>
              <div v-if="hoveredResource === group.key" class="resource-tooltip" :style="{ left: tooltipPosition.x + 'px', top: tooltipPosition.y + 'px' }">
                <div v-for="item in group.items" :key="item" class="tooltip-item">{{ item }}</div>
              </div>
           </div>
        </div>
        <div v-else class="empty">No resources being tracked</div>
      </div>

      <!-- Resource Details Modal -->
      <div v-if="selectedResource" class="modal-overlay" @click.self="selectedResource = null">
        <div class="modal-content" @click.stop>
          <div class="modal-header">
            <h3>Resource Details: {{ selectedResource.prefix }}</h3>
            <button class="modal-close" @click="selectedResource = null">×</button>
          </div>
          <div class="modal-body">
            <div class="resource-details">
              <p><strong>Kind:</strong> {{ selectedResource.kind }}</p>
              <p><strong>Namespace:</strong> {{ selectedResource.namespace }}</p>
              <p><strong>Count:</strong> {{ selectedResource.count }}</p>
              <h4>Instances:</h4>
              <ul>
                <li v-for="item in selectedResource.items" :key="item">{{ item }}</li>
              </ul>
              <h4>Rules Applied:</h4>
              <div v-if="rulesForResource.length > 0">
                <div v-for="rule in rulesForResource" :key="rule.name" class="rule-item">
                  {{ rule.name }} ({{ rule.kind }})
                </div>
              </div>
              <div v-else>No specific rules for this resource.</div>
            </div>
          </div>
        </div>
       </div>

       <!-- Rule Details Modal -->
       <div v-if="selectedRule" class="modal-overlay" @click.self="selectedRule = null">
         <div class="modal-content rule-modal" @click.stop>
           <div class="modal-header">
             <h3>Rule Details: {{ selectedRule.name }}</h3>
             <button class="modal-close" @click="selectedRule = null">×</button>
           </div>
           <div class="modal-body">
             <div class="rule-details">
               <div class="detail-row">
                 <div class="detail-col">
                   <p><strong>Kind:</strong> {{ selectedRule.kind }}</p>
                   <p><strong>Actions:</strong> {{ selectedRule.actions?.length || 0 }}</p>
                   <div v-if="selectedRule.actions?.length > 0">
                     <strong>Action Types:</strong>
                     <ul>
                       <li v-for="(action, idx) in selectedRule.actions" :key="idx">{{ action }}</li>
                     </ul>
                   </div>
                 </div>
                 <div class="detail-col">
                   <h4>Metrics Trend (7 days)</h4>
                   <div class="metric-chart">
                     <canvas ref="ruleChartCanvas"></canvas>
                   </div>
                 </div>
               </div>
               
               <div class="detail-row full-width">
                 <h4>Rule Activity Timeline</h4>
                 <div class="timeline-chart">
                   <div class="timeline-bars">
                     <div v-for="(value, idx) in getRuleTrend(selectedRule)" :key="idx" class="timeline-bar-container">
                       <div class="timeline-bar-label">Day {{ idx + 1 }}</div>
                       <div class="timeline-bar" :style="{ height: Math.max(20, value * 0.8) + 'px' }" :title="`Value: ${value}`">
                         <span class="timeline-value">{{ value }}</span>
                       </div>
                     </div>
                   </div>
                 </div>
               </div>
               
               <div class="detail-row full-width">
                 <h4>Recent Matches</h4>
                 <div class="recent-matches">
                   <div v-for="match in getRecentMatches(selectedRule)" :key="match.id" class="match-item">
                     <span class="match-time">{{ formatTime(match.timestamp) }}</span>
                     <span class="match-resource">{{ match.resource }}</span>
                     <span class="match-severity" :class="match.severity">{{ match.severity }}</span>
                   </div>
                   <div v-if="getRecentMatches(selectedRule).length === 0" class="no-matches">
                     No recent matches for this rule.
                   </div>
                 </div>
               </div>
             </div>
           </div>
         </div>
       </div>

       <div class="watcher-section logs-section">
        <LogSentinelPanel />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { GetWatcherStatus, GetWatcherRules, GetWatcherResources, StreamWatcherLogs } from '../../../wailsjs/go/main/App'
import LogSentinelPanel from './LogSentinelPanel.vue'

const status = ref<any>(null)
const rules = ref<any[]>([])
const resources = ref<string[]>([])
const hoveredResource = ref<string | null>(null)
const selectedResource = ref<any>(null)
const selectedRule = ref<any>(null)
const ruleChartCanvas = ref<HTMLCanvasElement | null>(null)
const refreshInterval = ref<ReturnType<typeof setInterval> | null>(null)

const groupedResources = computed(() => {
  const groups = new Map()
  resources.value.forEach(resource => {
    const parts = resource.split('/')
    if (parts.length < 3) return
    const [kind, namespace, name] = parts
    let groupKey = resource
    let displayName = name
    if (kind === 'pod') {
      // Extract deployment prefix: remove hash suffix like "-13e9b1e1-9fbmt"
      const match = name.match(/^(.+)-[a-z0-9]{6,10}-[a-z0-9]{5}$/)
      if (match) {
        const prefix = match[1]
        groupKey = `${kind}/${namespace}/${prefix}`
        displayName = prefix
      }
    }
    const group = groups.get(groupKey) || {
      key: groupKey,
      kind,
      namespace,
      prefix: displayName,
      count: 0,
      items: []
    }
    group.count++
    group.items.push(resource)
    groups.set(groupKey, group)
  })
  return Array.from(groups.values()).sort((a, b) => b.count - a.count)
})

const rulesForResource = computed(() => {
  if (!selectedResource.value) return []
  const resourceKind = selectedResource.value.kind
  // Filter rules that match the kind
  return rules.value.filter(rule => rule.kind.toLowerCase() === resourceKind.toLowerCase())
})

const logs = ref<string[]>([])
const logsActive = ref(false)
const logsContainer = ref<HTMLElement>()
let eventSource: EventSource | null = null

const tooltipPosition = ref({ x: 0, y: 0 })

onMounted(() => {
  loadData()
  // Refresh data every 10 seconds to pick up new rules/resources
  refreshInterval.value = setInterval(loadData, 10000)
  
  // Add resize listener for chart
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  stopLogs()
  if (refreshInterval.value) {
    clearInterval(refreshInterval.value)
    refreshInterval.value = null
  }
  // Remove resize listener
  window.removeEventListener('resize', handleResize)
})

async function loadData() {
  try {
    status.value = await GetWatcherStatus()
  } catch (err) {
    console.error('Failed to load watcher status:', err)
  }
  try {
    const rawRules = await GetWatcherRules()
    rules.value = rawRules.map((rule: any) => ({
      name: rule.Name || rule.name,
      kind: rule.Kind || rule.kind,
      actions: rule.Actions || rule.actions || []
    }))
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

function generateMockTrend(seed: string): number[] {
  // Simple deterministic pseudo-random trend based on seed
  let hash = 0
  for (let i = 0; i < seed.length; i++) {
    hash = ((hash << 5) - hash) + seed.charCodeAt(i)
    hash |= 0
  }
  const trend = []
  for (let i = 0; i < 7; i++) {
    const val = Math.abs(Math.sin(hash + i * 0.5)) * 100
    trend.push(Math.round(val))
  }
  return trend
}

function getRuleTrend(rule: any): number[] {
  return generateMockTrend(rule.name)
}

function getResourceTrend(resource: any): number[] {
  return generateMockTrend(resource.key)
}

function getRecentMatches(rule: any): any[] {
  // Mock recent matches data
  const severities = ['critical', 'warning', 'info']
  const matches = []
  for (let i = 0; i < 5; i++) {
    const timestamp = new Date(Date.now() - Math.random() * 7 * 24 * 60 * 60 * 1000)
    const severity = severities[Math.floor(Math.random() * severities.length)]
    const resourceTypes = ['pod', 'node', 'deployment']
    const resourceType = resourceTypes[Math.floor(Math.random() * resourceTypes.length)]
    matches.push({
      id: `match_${Date.now()}_${i}`,
      timestamp,
      resource: `${resourceType}/namespace-${Math.floor(Math.random() * 10)}/instance-${Math.floor(Math.random() * 100)}`,
      severity
    })
  }
  return matches.sort((a, b) => b.timestamp - a.timestamp)
}

function drawChart() {
  if (!ruleChartCanvas.value || !selectedRule.value) return
  
  const canvas = ruleChartCanvas.value
  const ctx = canvas.getContext('2d')
  if (!ctx) return
  
  // Set canvas dimensions based on client size and device pixel ratio
  const rect = canvas.getBoundingClientRect()
  const dpr = window.devicePixelRatio || 1
  const width = Math.round(rect.width * dpr)
  const height = Math.round(rect.height * dpr)
  
  // Only update if dimensions changed
  if (canvas.width !== width || canvas.height !== height) {
    canvas.width = width
    canvas.height = height
  }
  
  // Scale context for high DPI displays
  ctx.scale(dpr, dpr)
  const scaledWidth = width / dpr
  const scaledHeight = height / dpr
  
  // Clear canvas
  ctx.clearRect(0, 0, scaledWidth, scaledHeight)
  
  // Draw background
  ctx.fillStyle = 'var(--bg-subtle)'
  ctx.fillRect(0, 0, scaledWidth, scaledHeight)
  
  // Get trend data
  const trend = getRuleTrend(selectedRule.value)
  const max = Math.max(...trend)
  const min = Math.min(...trend)
  const range = max - min || 1
  
  // Draw grid lines
  ctx.strokeStyle = 'var(--border)'
  ctx.lineWidth = 1
  for (let i = 0; i <= 4; i++) {
    const y = scaledHeight * 0.1 + (i * scaledHeight * 0.8) / 4
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.lineTo(scaledWidth, y)
    ctx.stroke()
  }
  
  // Draw trend line
  ctx.strokeStyle = 'var(--accent)'
  ctx.lineWidth = 3
  ctx.beginPath()
  for (let i = 0; i < trend.length; i++) {
    const x = (i / (trend.length - 1)) * scaledWidth * 0.9 + scaledWidth * 0.05
    const y = scaledHeight * 0.9 - ((trend[i] - min) / range) * scaledHeight * 0.8
    if (i === 0) ctx.moveTo(x, y)
    else ctx.lineTo(x, y)
  }
  ctx.stroke()
  
  // Draw points
  for (let i = 0; i < trend.length; i++) {
    const x = (i / (trend.length - 1)) * scaledWidth * 0.9 + scaledWidth * 0.05
    const y = scaledHeight * 0.9 - ((trend[i] - min) / range) * scaledHeight * 0.8
    ctx.fillStyle = 'var(--accent)'
    ctx.beginPath()
    ctx.arc(x, y, 4, 0, Math.PI * 2)
    ctx.fill()
  }
  
  // Draw labels
  ctx.fillStyle = 'var(--text-secondary)'
  ctx.font = '12px sans-serif'
  ctx.textAlign = 'center'
  ctx.textBaseline = 'top'
  for (let i = 0; i < trend.length; i++) {
    const x = (i / (trend.length - 1)) * scaledWidth * 0.9 + scaledWidth * 0.05
    const label = `Day ${i + 1}`
    ctx.fillText(label, x, scaledHeight * 0.9 + 5)
  }
}

// Watch for rule selection to draw chart
watch(selectedRule, (newRule) => {
  if (newRule) {
    nextTick(() => {
      drawChart()
    })
  }
})

function handleResize() {
  if (selectedRule.value) {
    drawChart()
  }
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

function updateTooltipPosition(event: MouseEvent) {
  tooltipPosition.value = { x: event.clientX, y: event.clientY }
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

.rules-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 1rem;
}

.resources-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.rule-card {
  padding: 1rem;
  background: var(--bg-subtle);
  border-radius: 8px;
  border: 1px solid var(--border);
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  position: relative;
}

.rule-card:hover {
  background: var(--bg-subtle-hover, rgba(255, 255, 255, 0.03));
  border-color: var(--accent);
  border-left-width: 4px;
  border-left-color: #9181F4;
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
}

.rule-card:hover::before {
  content: "";
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 50px;
  background: linear-gradient(to right, rgba(145, 129, 244, 0.1), transparent);
  pointer-events: none;
  border-radius: 8px 0 0 8px;
}

.rule-card-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  width: 100%;
}

.rule-card-body {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.rule-actions-count {
  font-size: 0.85rem;
  color: var(--text-tertiary);
  background: var(--bg-tag);
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  align-self: flex-start;
}

.resource-item {
  padding: 0.75rem;
  background: var(--bg-subtle);
  border-radius: 4px;
  border-left: 4px solid var(--accent);
  display: flex;
  position: relative;
  flex-direction: column;
  gap: 0.5rem;
  cursor: pointer;
  transition: all 0.2s ease;
}

.resource-item:hover {
  background: var(--bg-subtle-hover, rgba(255, 255, 255, 0.03));
  border-left-color: #9181F4;
  transform: translateX(2px);
}

.resource-item:hover::before {
  content: "";
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 50px;
  background: linear-gradient(to right, rgba(145, 129, 244, 0.1), transparent);
  pointer-events: none;
  border-radius: 4px 0 0 4px;
}

.resource-item:hover .resource-count {
  color: #9181F4;
}

.rule-main {
  display: flex;
  align-items: center;
  gap: 1rem;
  width: 100%;
}

.resource-main {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  width: 100%;
}

.resource-sparkline {
  width: 100%;
  height: 20px;
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

.rule-sparkline {
  width: 100%;
  height: 20px;
}

.sparkline-bars {
  display: flex;
  align-items: flex-end;
  height: 100%;
  gap: 2px;
}

.sparkline-bar {
  flex: 1;
  background: var(--accent);
  opacity: 0.6;
  border-radius: 1px;
  transition: opacity 0.2s;
}

.sparkline-bar:hover {
  opacity: 1;
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

/* Resource grouping styles */
.resource-name {
  flex: 1;
}
.resource-count {
  font-size: 0.85rem;
  color: var(--text-secondary);
  background: var(--bg-tag);
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
}
.resource-tooltip {
  position: fixed;
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-top: 4px solid #9181F4;
  border-radius: 6px;
  padding: 0.75rem;
  box-shadow: 0 4px 12px rgba(0,0,0,0.1);
  z-index: 1000;
  max-width: 300px;
  max-height: 200px;
  overflow-y: auto;
  transform: translate(-50%, -100%);
  margin-top: -10px;
}
.tooltip-item {
  padding: 0.25rem 0;
  border-bottom: 1px solid var(--border-light);
}
.tooltip-item:last-child {
  border-bottom: none;
}

/* Modal styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}
.modal-content {
  background: var(--bg-panel);
  border-radius: 8px;
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow-y: auto;
  box-shadow: 0 8px 32px rgba(0,0,0,0.2);
}
.modal-header {
  padding: 1.5rem;
  border-bottom: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.modal-header h3 {
  margin: 0;
}
.modal-close {
  background: none;
  border: none;
  font-size: 1.5rem;
  cursor: pointer;
  color: var(--text-secondary);
}
.modal-body {
  padding: 1.5rem;
}
.resource-details h4 {
  margin-top: 1.5rem;
  margin-bottom: 0.75rem;
}
.resource-details ul {
  padding-left: 1.5rem;
  margin: 0.5rem 0;
}

/* Rule modal specific styles */
.rule-modal {
  max-width: 800px !important;
}

.detail-row {
  display: flex;
  gap: 2rem;
  margin-bottom: 2rem;
}

.detail-col {
  flex: 1;
  min-width: 0;
}

.detail-col h4 {
  margin-top: 0;
  margin-bottom: 1rem;
}

.full-width {
  width: 100%;
  flex-basis: 100%;
}

.metric-chart {
  width: 100%;
  height: 200px;
  background: var(--bg-subtle);
  border-radius: 6px;
  padding: 1rem;
  border: 1px solid var(--border);
}

.metric-chart canvas {
  width: 100% !important;
  height: 100% !important;
}

.timeline-chart {
  width: 100%;
  padding: 1rem;
  background: var(--bg-subtle);
  border-radius: 6px;
  border: 1px solid var(--border);
}

.timeline-bars {
  display: flex;
  align-items: flex-end;
  gap: 1rem;
  height: 200px;
}

.timeline-bar-container {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
}

.timeline-bar-label {
  font-size: 0.8rem;
  color: var(--text-secondary);
}

.timeline-bar {
  width: 30px;
  background: linear-gradient(to top, var(--accent), #9181F4);
  border-radius: 4px 4px 0 0;
  display: flex;
  align-items: flex-end;
  justify-content: center;
  transition: height 0.3s ease;
  position: relative;
}

.timeline-value {
  position: absolute;
  top: -20px;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-primary);
}

.recent-matches {
  background: var(--bg-subtle);
  border-radius: 6px;
  border: 1px solid var(--border);
  padding: 1rem;
}

.match-item {
  display: flex;
  align-items: center;
  gap: 1rem;
  padding: 0.75rem;
  border-bottom: 1px solid var(--border-light);
}

.match-item:last-child {
  border-bottom: none;
}

.match-time {
  font-size: 0.85rem;
  color: var(--text-secondary);
  min-width: 180px;
}

.match-resource {
  flex: 1;
  font-family: monospace;
  font-size: 0.9rem;
}

.match-severity {
  padding: 0.25rem 0.5rem;
  border-radius: 4px;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

.match-severity.critical {
  background: var(--error-bg);
  color: var(--error);
}

.match-severity.warning {
  background: var(--warning-bg);
  color: var(--warning);
}

.match-severity.info {
  background: var(--info-bg);
  color: var(--info);
}

.no-matches {
  text-align: center;
  padding: 2rem;
  color: var(--text-secondary);
  font-style: italic;
}
</style>