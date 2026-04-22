<template>
  <section class="log-sentinel-panel">
    <div class="panel-head">
      <div>
        <p class="meta-label">Argus Intelligence</p>
        <h3>Log Sentinel</h3>
      </div>
      <div class="panel-actions">
        <button class="glass-btn" @click="showRuleModal = true">
          <PhEye size="16" weight="bold" class="btn-icon" />
          Add Watch Rule
        </button>
        <button class="glass-btn refresh-btn" @click="refreshData" :disabled="isLoading">
          <PhArrowClockwise size="16" weight="bold" class="btn-icon" />
          Refresh
        </button>
      </div>
    </div>

    <!-- PromQL Input & Controls -->
    <div class="controls-section">
      <div class="promql-input-group">
        <label class="control-label">PromQL Queries</label>
        <div class="query-inputs">
          <div v-for="(query, idx) in queries" :key="idx" class="query-row">
            <div class="color-indicator" :style="{ backgroundColor: query.color }"></div>
            <input
              v-model="query.text"
              type="text"
              placeholder="e.g., rate(http_requests_total{status=~'5..'}[5m])"
              class="promql-input"
              @keyup.enter="executeQueries"
            />
            <button class="remove-query-btn" @click="removeQuery(idx)" title="Remove query">
              ×
            </button>
          </div>
        </div>
        <div class="query-actions">
           <button class="ghost-btn small" @click="addQuery">
             <PhPlus size="14" weight="bold" class="btn-icon" />
             Add Query
           </button>
           <button class="glass-btn execute-btn" @click="executeQueries" :disabled="isLoading">
             <PhLightning size="14" weight="bold" class="btn-icon" />
             Execute
           </button>
        </div>
      </div>

      <div class="cluster-selector">
        <label class="control-label">Cluster / Namespace</label>
        <select v-model="selectedCluster" class="select-input">
          <option value="">All Clusters</option>
          <option v-for="cluster in clusters" :key="cluster">{{ cluster }}</option>
        </select>
        <select v-model="selectedNamespace" class="select-input">
          <option value="">All Namespaces</option>
          <option v-for="ns in namespaces" :key="ns">{{ ns }}</option>
        </select>
      </div>
    </div>

    <!-- Sparkline Chart -->
    <div class="sparkline-section">
      <div class="sparkline-header">
        <h4>Telemetry Trends</h4>
        <div class="legend">
          <div v-for="query in queries" :key="query.color" class="legend-item">
            <span class="legend-color" :style="{ backgroundColor: query.color }"></span>
            <span class="legend-label">Series {{ query.color }}</span>
            <button class="legend-toggle" @click="toggleSeries(query)" :class="{ off: !query.visible }">
              {{ query.visible ? '●' : '○' }}
            </button>
          </div>
        </div>
      </div>
      <div class="sparkline-container">
        <canvas ref="sparklineCanvas" @click="handleSparklineClick"></canvas>
        <div v-if="hoverPoint" class="sparkline-tooltip" :style="{ left: hoverPoint.x + 'px', top: hoverPoint.y + 'px' }">
          <div v-for="(value, seriesIdx) in hoverPoint.values" :key="seriesIdx" class="tooltip-series">
            <span class="tooltip-color" :style="{ backgroundColor: queries[seriesIdx].color }"></span>
            <span class="tooltip-value">{{ value.toFixed(2) }}</span>
          </div>
          <div class="tooltip-time">{{ hoverPoint.time }}</div>
        </div>
      </div>
    </div>

    <!-- Log Table -->
    <div class="log-section">
      <div class="log-header">
        <h4>Log Evidence (Last 4 rows)</h4>
        <div class="log-controls">
          <label class="toggle-label">
            <input type="checkbox" v-model="autoScroll" /> Auto-follow
          </label>
          <button class="ghost-btn small" @click="scrollToTop">Top</button>
        </div>
      </div>
      <div class="log-table-container" ref="logContainer" @scroll="handleScroll">
        <table class="log-table">
          <thead>
            <tr>
              <th style="width: fit-content;">Timestamp</th>
              <th style="width: 90px;">Level</th>
              <th style="flex: 0 1 200px;">Source</th>
              <th style="flex: 1 1 auto;">Message</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="log in visibleLogs"
              :key="log.id"
              :class="{ highlighted: log.timestamp === highlightedTimestamp }"
              @click="highlightLog(log)"
            >
              <td class="timestamp">{{ formatTime(log.timestamp) }}</td>
              <td>
                <span :class="['level-badge', log.level]">{{ log.level }}</span>
              </td>
              <td class="source">{{ log.source }}</td>
              <td class="message">{{ log.message }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="log-footer">
        <span class="log-count">{{ logs.length }} logs total</span>
        <span class="log-position" v-if="logScrollPosition > 0">
          Scrolled {{ logScrollPosition }}%
        </span>
       </div>
     </div>

     <!-- Watch Rule Modal -->
     <div v-if="showRuleModal" class="modal-overlay" @click="showRuleModal = false">
       <div class="modal-content" @click.stop>
         <div class="modal-header">
           <h3>Create Watch Rule</h3>
           <button class="modal-close" @click="showRuleModal = false">×</button>
         </div>
         <div class="modal-body">
           <!-- Search input with autocomplete -->
           <div class="search-section">
             <label>What do you want to watch?</label>
             <input
               type="text"
               v-model="ruleSearch"
               placeholder="Type file path, log pattern, metric, resource..."
               class="search-input"
               @input="onRuleSearch"
             />
             <div v-if="searchSuggestions.length > 0" class="suggestions-dropdown">
               <div
                 v-for="suggestion in searchSuggestions"
                 :key="suggestion.id"
                 class="suggestion-item"
                 @click="selectSuggestion(suggestion)"
               >
                 <component :is="suggestion.icon" size="16" weight="bold" />
                 <span>{{ suggestion.label }}</span>
                 <span class="suggestion-category">{{ suggestion.category }}</span>
               </div>
             </div>
           </div>

           <!-- Category buttons -->
           <div class="categories-section" v-if="!selectedCategory">
             <p class="section-label">Or choose a category:</p>
             <div class="category-buttons">
               <button
                 v-for="category in categories"
                 :key="category.id"
                 class="category-btn"
                 @click="selectedCategory = category.id"
               >
                 <component :is="category.icon" size="24" weight="bold" />
                 <span>{{ category.label }}</span>
               </button>
             </div>
           </div>

           <!-- Category options -->
           <div v-if="selectedCategory" class="category-options">
             <div class="category-header">
               <button class="back-btn" @click="selectedCategory = null">
                 ← Back
               </button>
               <h4>{{ getCategoryLabel(selectedCategory) }}</h4>
             </div>
             <div class="options-grid">
               <button
                 v-for="option in getCategoryOptions(selectedCategory)"
                 :key="option.id"
                 class="option-btn"
                 @click="selectOption(option)"
               >
                  <component :is="option.icon" size="20" weight="bold" class="option-icon" />
                 <span class="option-label">{{ option.label }}</span>
                 <span class="option-desc">{{ option.description }}</span>
               </button>
             </div>
           </div>

           <!-- Rule configuration (simplified) -->
           <div v-if="selectedOption" class="rule-config">
             <h4>Configure Rule: {{ selectedOption.label }}</h4>
             <div class="form-group">
               <label>Rule Name</label>
               <input v-model="ruleName" placeholder="e.g., Monitor API logs" />
             </div>
             <div class="form-group">
               <label>Condition</label>
               <input v-model="ruleCondition" :placeholder="selectedOption.conditionPlaceholder" />
             </div>
             <div class="form-group">
               <label>Actions</label>
               <select v-model="ruleActions" multiple>
                 <option value="alert">Send Alert</option>
                 <option value="log">Log Change</option>
                 <option value="webhook">Trigger Webhook</option>
               </select>
             </div>
             <div class="modal-actions">
               <button class="btn-secondary" @click="cancelRule">Cancel</button>
               <button class="btn-primary" @click="saveRule">Save Rule</button>
             </div>
           </div>
         </div>
       </div>
     </div>
   </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import { PhArrowClockwise, PhPlus, PhLightning, PhEye, PhFile, PhChartLine, PhGear, PhMagnifyingGlass, PhWarning, PhBrain, PhCube, PhDesktop, PhRocket, PhNotePencil, PhFileText } from '@phosphor-icons/vue'
import { GetWatcherRules, GetWatcherResources, GetWatcherStatus, AddWatcherRule, StreamWatcherLogs } from '../../../wailsjs/go/main/App'

interface Query {
  text: string
  color: string
  visible: boolean
  data: number[]
}

interface LogEntry {
  id: string
  timestamp: number
  level: string
  source: string
  message: string
}

const queries = ref<Query[]>([
  { text: 'rate(http_requests_total{status=~"5.."}[5m])', color: '#8a82e0', visible: true, data: [] },
  { text: 'rate(http_requests_total{status=~"2.."}[5m])', color: '#5ac8c5', visible: true, data: [] },
])

const clusters = ref<string[]>(['minikube', 'prod-cluster'])
const namespaces = ref<string[]>(['default', 'monitoring', 'kube-system'])
const selectedCluster = ref('')
const selectedNamespace = ref('')

const logs = ref<LogEntry[]>([])
const logsActive = ref(false)
let eventSource: EventSource | null = null

const isLoading = ref(false)
const showRuleModal = ref(false)
const autoScroll = ref(true)
const logScrollPosition = ref(0)
const highlightedTimestamp = ref<number | null>(null)
const hoverPoint = ref<{ x: number, y: number, values: number[], time: string } | null>(null)

// Rule modal state
const ruleSearch = ref('')
const searchSuggestions = ref<any[]>([])
const categories = ref([
  { id: 'logs', label: 'Log Patterns', icon: 'PhFile' },
  { id: 'metrics', label: 'Metrics', icon: 'PhChartLine' },
  { id: 'resources', label: 'Resources', icon: 'PhGear' },
  { id: 'files', label: 'File Changes', icon: 'PhMagnifyingGlass' }
])
const selectedCategory = ref<string | null>(null)
const selectedOption = ref<any>(null)
const ruleName = ref('')
const ruleCondition = ref('')
const ruleActions = ref<string[]>([])

const sparklineCanvas = ref<HTMLCanvasElement | null>(null)
const logContainer = ref<HTMLDivElement | null>(null)
const sparklinePoints = ref<Array<{x: number, y: number, seriesIdx: number, dataIdx: number, value: number}>>([])

const visibleLogs = computed(() => {
  return logs.value.slice(-4)
})

function addQuery() {
  const colors = ['#8a82e0', '#5ac8c5', '#ff6b7a', '#ffc145', '#5cd4a3']
  const nextColor = colors[queries.value.length % colors.length]
  queries.value.push({
    text: '',
    color: nextColor,
    visible: true,
    data: []
  })
}

function removeQuery(idx: number) {
  queries.value.splice(idx, 1)
}

function toggleSeries(query: Query) {
  query.visible = !query.visible
  drawSparkline()
}

async function executeQueries() {
  isLoading.value = true
  // Mock data for now
  for (const query of queries.value) {
    query.data = Array.from({ length: 20 }, () => Math.random() * 100)
  }
  await nextTick()
  drawSparkline()
  isLoading.value = false
}

function setupCanvas(canvas: HTMLCanvasElement): { width: number, height: number, ctx: CanvasRenderingContext2D } {
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('Canvas context not available')
  
  const dpr = window.devicePixelRatio || 1
  const rect = canvas.getBoundingClientRect()
  const width = rect.width * dpr
  const height = rect.height * dpr
  
  canvas.width = width
  canvas.height = height
  ctx.scale(dpr, dpr)
  ctx.clearRect(0, 0, rect.width, rect.height)
  
  return { width: rect.width, height: rect.height, ctx }
}

function drawSparkline() {
  const canvas = sparklineCanvas.value
  if (!canvas) return
  const { width, height, ctx } = setupCanvas(canvas)
  
  const activeQueries = queries.value.filter(q => q.visible && q.data.length > 0)
  if (activeQueries.length === 0) return

  const maxValue = Math.max(...activeQueries.flatMap(q => q.data))
  const padding = 10
  const chartWidth = width - padding * 2
  const chartHeight = height - padding * 2
  const baseline = height - padding
  
  // Clear points
  sparklinePoints.value = []
  
  activeQueries.forEach((query, seriesIdx) => {
    const data = query.data
    const step = chartWidth / (data.length - 1)
    
    // Create gradient for area fill
    const gradient = ctx.createLinearGradient(0, padding, 0, baseline)
    gradient.addColorStop(0, query.color + '80') // 50% opacity
    gradient.addColorStop(1, query.color + '00') // 0% opacity
    
    // Draw area
    ctx.beginPath()
    ctx.moveTo(padding, baseline)
    for (let i = 0; i < data.length; i++) {
      const x = padding + i * step
      const y = baseline - (data[i] / maxValue) * chartHeight
      ctx.lineTo(x, y)
      sparklinePoints.value.push({ x, y, seriesIdx, dataIdx: i, value: data[i] })
    }
    ctx.lineTo(padding + chartWidth, baseline)
    ctx.closePath()
    ctx.fillStyle = gradient
    ctx.fill()
    
    // Draw line on top
    ctx.beginPath()
    ctx.strokeStyle = query.color
    ctx.lineWidth = 2
    ctx.moveTo(padding, baseline - (data[0] / maxValue) * chartHeight)
    for (let i = 1; i < data.length; i++) {
      const x = padding + i * step
      const y = baseline - (data[i] / maxValue) * chartHeight
      ctx.lineTo(x, y)
    }
    ctx.stroke()
  })
}

function handleSparklineClick(event: MouseEvent) {
  if (!sparklineCanvas.value) return
  const rect = sparklineCanvas.value.getBoundingClientRect()
  const clickX = event.clientX - rect.left
  const clickY = event.clientY - rect.top
  
  // Find closest point within 20px radius
  let closestDist = Infinity
  let closestPoint = null
  for (const point of sparklinePoints.value) {
    const dist = Math.sqrt((point.x - clickX) ** 2 + (point.y - clickY) ** 2)
    if (dist < closestDist && dist < 20) {
      closestDist = dist
      closestPoint = point
    }
  }
  if (closestPoint) {
    // Map data index to timestamp
    const now = Date.now()
    const timestamp = now - (closestPoint.dataIdx * 5000) // 5 seconds per point
    // Highlight log with closest timestamp
    const closestLog = logs.value.reduce((prev, curr) => {
      const prevDiff = Math.abs(prev.timestamp - timestamp)
      const currDiff = Math.abs(curr.timestamp - timestamp)
      return currDiff < prevDiff ? curr : prev
    })
    highlightLog(closestLog)
    // Scroll to that log in table
    const logIndex = logs.value.findIndex(log => log.id === closestLog.id)
    if (logContainer.value) {
      const rowHeight = 60 // approximate row height
      const scrollTop = Math.max(0, logIndex * rowHeight - rowHeight * 2)
      logContainer.value.scrollTop = scrollTop
    }
  }
}

function handleScroll() {
  if (!logContainer.value) return
  const { scrollTop, scrollHeight, clientHeight } = logContainer.value
  logScrollPosition.value = Math.round((scrollTop / (scrollHeight - clientHeight)) * 100)
}

function scrollToTop() {
  if (logContainer.value) {
    logContainer.value.scrollTop = 0
  }
}

function highlightLog(log: LogEntry) {
  highlightedTimestamp.value = log.timestamp
  // In a real app, would sync sparkline to this timestamp
}

function refreshData() {
  executeQueries()
  // In a real app, would fetch latest logs
}

function formatTime(timestamp: number): string {
  return new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

// Rule modal methods
function onRuleSearch() {
  if (!ruleSearch.value.trim()) {
    searchSuggestions.value = []
    return
  }
  
  // Mock suggestions based on search
  const mockSuggestions = [
    { id: '1', label: 'Error logs in namespace', category: 'Logs', icon: 'PhFile' },
    { id: '2', label: 'HTTP 5xx rate increase', category: 'Metrics', icon: 'PhChartLine' },
    { id: '3', label: 'Pod restarts > 5/min', category: 'Resources', icon: 'PhGear' },
    { id: '4', label: 'Config file changes', category: 'File Changes', icon: 'PhMagnifyingGlass' },
    { id: '5', label: 'Memory usage > 90%', category: 'Metrics', icon: 'PhChartLine' }
  ].filter(s => 
    s.label.toLowerCase().includes(ruleSearch.value.toLowerCase()) ||
    s.category.toLowerCase().includes(ruleSearch.value.toLowerCase())
  )
  
  searchSuggestions.value = mockSuggestions
}

function selectSuggestion(suggestion: any) {
  ruleSearch.value = suggestion.label
  searchSuggestions.value = []
  // Auto-select the corresponding category
  const category = categories.value.find(c => c.label.toLowerCase().includes(suggestion.category.toLowerCase()))
  if (category) {
    selectedCategory.value = category.id
  }
}

function getCategoryLabel(categoryId: string): string {
  const category = categories.value.find(c => c.id === categoryId)
  return category?.label || categoryId
}

function getCategoryOptions(categoryId: string): any[] {
  // Mock options based on category
  const options: Record<string, any[]> = {
    logs: [
      { id: 'log-error', label: 'Error Patterns', description: 'Monitor for error-level logs', icon: 'PhWarning', conditionPlaceholder: 'e.g., level=ERROR, message contains "timeout"' },
      { id: 'log-warning', label: 'Warning Patterns', description: 'Monitor for warning-level logs', icon: 'PhWarning', conditionPlaceholder: 'e.g., level=WARN, message contains "slow"' },
      { id: 'log-custom', label: 'Custom Pattern', description: 'Define a custom regex pattern', icon: 'PhMagnifyingGlass', conditionPlaceholder: 'e.g., regex pattern like ".*error.*"' }
    ],
    metrics: [
      { id: 'metric-threshold', label: 'Threshold Alert', description: 'Alert when metric exceeds threshold', icon: 'PhChartLine', conditionPlaceholder: 'e.g., cpu_usage > 90%' },
      { id: 'metric-rate', label: 'Rate Change', description: 'Alert on rate of change', icon: 'PhLightning', conditionPlaceholder: 'e.g., rate increase > 50%' },
      { id: 'metric-anomaly', label: 'Anomaly Detection', description: 'Detect statistical anomalies', icon: 'PhBrain', conditionPlaceholder: 'e.g., anomaly score > 3' }
    ],
    resources: [
      { id: 'resource-pod', label: 'Pod Status', description: 'Monitor pod restarts, status', icon: 'PhCube', conditionPlaceholder: 'e.g., pod restarts > 5' },
      { id: 'resource-node', label: 'Node Resources', description: 'Monitor node CPU/memory', icon: 'PhDesktop', conditionPlaceholder: 'e.g., node memory < 10% free' },
      { id: 'resource-deployment', label: 'Deployment Health', description: 'Monitor deployment replicas', icon: 'PhRocket', conditionPlaceholder: 'e.g., available replicas < desired' }
    ],
    files: [
      { id: 'file-change', label: 'File Changes', description: 'Monitor file modifications', icon: 'PhNotePencil', conditionPlaceholder: 'e.g., file modified in /etc/config' },
      { id: 'file-exists', label: 'File Existence', description: 'Check if file exists or not', icon: 'PhMagnifyingGlass', conditionPlaceholder: 'e.g., file /var/log/app.log missing' },
      { id: 'file-content', label: 'File Content', description: 'Monitor file content changes', icon: 'PhFileText', conditionPlaceholder: 'e.g., file contains "password"' }
    ]
  }
  
  return options[categoryId] || []
}

function selectOption(option: any) {
  selectedOption.value = option
  ruleCondition.value = ''
  ruleName.value = option.label
  ruleActions.value = ['alert']
}

function cancelRule() {
  showRuleModal.value = false
  resetRuleModal()
}

async function saveRule() {
  if (!ruleName.value.trim() || !ruleCondition.value.trim()) {
    alert('Please fill in rule name and condition')
    return
  }
  
  // Map frontend action IDs to watcher action IDs
  const actionMap: Record<string, string> = {
    'alert': 'alert_critical',
    'log': 'log_change',
    'webhook': 'dashboard-webhook'
  }
  
  const watcherActions = ruleActions.value
    .map(action => actionMap[action] || action)
    .filter(Boolean)
  
  if (watcherActions.length === 0) {
    alert('Please select at least one valid action')
    return
  }
  
  // Build rule object matching watcher engine's Rule struct
  const rule = {
    name: ruleName.value.trim(),
    kind: 'Pod', // default, could be inferred from category
    namespace: '', // empty for all namespaces
    selector: { matchLabels: null },
    logic: 'all',
    duration: 0,
    expression: ruleCondition.value.trim(),
    condition: ruleCondition.value.trim(),
    conditions: null,
    actions: watcherActions
  }
  
  try {
    console.log('Saving rule:', rule)
    const result = await AddWatcherRule(rule)
    console.log('Rule saved successfully:', result)
    
    // Show success message
    alert(`Rule "${ruleName.value}" saved successfully!`)
    
    showRuleModal.value = false
    resetRuleModal()
    
    // Optionally refresh rules in WatcherPanel (could emit event)
    // For now, we'll just log
  } catch (error) {
    console.error('Failed to save rule:', error)
    alert(`Failed to save rule: ${error instanceof Error ? error.message : String(error)}`)
  }
}

function resetRuleModal() {
  ruleSearch.value = ''
  searchSuggestions.value = []
  selectedCategory.value = null
  selectedOption.value = null
  ruleName.value = ''
  ruleCondition.value = ''
  ruleActions.value = []
}

function mapToLogEntry(data: any): LogEntry {
  // Extract fields from watcher event data
  const id = data.id || data.uid || Date.now().toString()
  const timestamp = data.timestamp || data.time || data.created || Date.now()
  // Determine level from severity, level, or event type
  let level = 'INFO'
  if (data.level) level = data.level
  else if (data.severity) level = data.severity.toUpperCase()
  else if (data.eventType) level = data.eventType.toUpperCase()
  // Determine source from component, source, kind, or namespace/name
  let source = 'unknown'
  if (data.source) source = data.source
  else if (data.component) source = data.component
  else if (data.kind && data.namespace && data.name) source = `${data.kind}/${data.namespace}/${data.name}`
  else if (data.kind) source = data.kind
  // Determine message from message, reason, or full data
  let message = data.message || data.reason || JSON.stringify(data)
  // Truncate long messages
  if (message.length > 200) message = message.substring(0, 200) + '...'
  
  return {
    id: id.toString(),
    timestamp: typeof timestamp === 'string' ? new Date(timestamp).getTime() : timestamp,
    level: level.toUpperCase(),
    source,
    message
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
         // Transform watcher log entry to LogEntry format
         const logEntry = mapToLogEntry(data)
         logs.value.push(logEntry)
         if (logs.value.length > 100) {
           logs.value.shift()
         }
       } catch (e) {
         console.error('Failed to parse log entry:', e)
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

onMounted(() => {
  executeQueries()
  startLogs()
  window.addEventListener('resize', drawSparkline)
})

onUnmounted(() => {
  stopLogs()
  window.removeEventListener('resize', drawSparkline)
})
</script>

<style scoped>
.log-sentinel-panel {
  padding: 24px;
  background: var(--glass-card-surface);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: var(--glass-border-rim);
  border-radius: var(--radius-lg);
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 24px;
}

.controls-section {
  display: flex;
  gap: 32px;
  margin-bottom: 24px;
  flex-wrap: wrap;
}

.promql-input-group {
  flex: 1;
  min-width: 300px;
}

.control-label {
  display: block;
  margin-bottom: 8px;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  font-weight: 500;
}

.query-inputs {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 12px;
}

.query-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.color-indicator {
  width: 12px;
  height: 12px;
  border-radius: 50%;
  flex-shrink: 0;
}

.promql-input {
  flex: 1;
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text);
  font-family: 'SF Mono', monospace;
  font-size: 13px;
}

.promql-input:focus {
  outline: none;
  border-color: var(--border-active);
}

.remove-query-btn {
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.remove-query-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.query-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.cluster-selector {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 200px;
}

.select-input {
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}

.select-input:focus {
  outline: none;
  border-color: var(--border-active);
}

.sparkline-section {
  margin-bottom: 24px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.03);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
}

.sparkline-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.sparkline-header h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.legend {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 6px;
}

.legend-color {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.legend-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.legend-toggle {
  width: 20px;
  height: 20px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.legend-toggle.off {
  opacity: 0.4;
}

.sparkline-container {
  position: relative;
  height: 120px;
  background: rgba(0, 0, 0, 0.1);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.sparkline-container canvas {
  width: 100%;
  height: 100%;
}

.sparkline-tooltip {
  position: absolute;
  background: rgba(0, 0, 0, 0.8);
  backdrop-filter: blur(10px);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 12px;
  transform: translate(-50%, -100%);
  pointer-events: none;
  z-index: 10;
}

.tooltip-series {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 4px;
}

.tooltip-color {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.tooltip-value {
  font-size: 12px;
  color: var(--text);
  font-family: 'SF Mono', monospace;
}

.tooltip-time {
  font-size: 11px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.log-section {
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.log-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px;
  background: rgba(255, 255, 255, 0.03);
  border-bottom: 1px solid var(--border);
}

.log-header h4 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text);
}

.log-controls {
  display: flex;
  gap: 12px;
  align-items: center;
}

.toggle-label {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
}

.log-table-container {
  max-height: 240px;
  overflow-y: auto;
}

.log-table {
  width: 100%;
  border-collapse: collapse;
  display: table;
}

.log-table thead {
  position: sticky;
  top: 0;
  background: var(--glass-card-surface);
  z-index: 1;
  border-bottom: 1px solid var(--border);
}

.log-table th {
  text-align: left;
  padding: 12px 16px;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  font-weight: 500;
}

.log-table tbody tr {
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  transition: background 0.2s ease;
}

.log-table tbody tr:hover {
  background: rgba(255, 255, 255, 0.05);
}

.log-table tbody tr.highlighted {
  background: rgba(138, 130, 224, 0.1);
}

.log-table td {
  padding: 12px 16px;
  font-size: 13px;
  vertical-align: top;
}

.timestamp {
  font-family: 'SF Mono', monospace;
  color: var(--text-secondary);
  white-space: nowrap;
}

.level-badge {
  display: inline-block;
  padding: 2px 8px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.level-badge.ERROR {
  background: rgba(255, 107, 122, 0.16);
  color: #ffd4d9;
}

.level-badge.WARN {
  background: rgba(255, 193, 69, 0.16);
  color: #fff2d6;
}

.level-badge.INFO {
  background: rgba(92, 212, 163, 0.16);
  color: #d9f7ec;
}

.level-badge.DEBUG {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-secondary);
}

.source {
  color: var(--text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.message {
  color: var(--text-secondary);
  line-height: 1.4;
}

.log-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  background: rgba(255, 255, 255, 0.03);
  border-top: 1px solid var(--border);
  font-size: 12px;
  color: var(--text-secondary);
}

.log-count, .log-position {
  font-size: 11px;
  opacity: 0.7;
}

.glass-btn {
  padding: 8px 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  color: var(--text);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
  display: flex;
  align-items: center;
  gap: 8px;
}

.glass-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.2);
  border-color: var(--border-active);
  transform: translateY(-1px);
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ghost-btn {
  padding: 6px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  gap: 6px;
}

.ghost-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.ghost-btn.small {
  padding: 4px 8px;
  font-size: 11px;
}

.btn-icon {
  font-size: 14px;
  line-height: 1;
}

.refresh-btn .btn-icon {
  font-size: 12px;
}

.execute-btn {
  background: var(--accent);
  color: white;
  border: none;
}

.execute-btn:hover:not(:disabled) {
  background: var(--accent);
  filter: brightness(1.1);
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.modal-content {
  background: var(--glass-card-surface);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: var(--glass-border-rim);
  border-radius: var(--radius-lg);
  width: 90%;
  max-width: 600px;
  max-height: 80vh;
  overflow-y: auto;
  animation: modalSlideIn 0.3s ease;
}

@keyframes modalSlideIn {
  from {
    opacity: 0;
    transform: translateY(20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid var(--border);
}

.modal-header h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.modal-close {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  font-size: 20px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-close:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.modal-body {
  padding: 24px;
}

.search-section {
  margin-bottom: 24px;
  position: relative;
}

.search-section label {
  display: block;
  margin-bottom: 8px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.search-input {
  width: 100%;
  padding: 12px 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text);
  font-size: 14px;
}

.search-input:focus {
  outline: none;
  border-color: var(--border-active);
}

.suggestions-dropdown {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--glass-card-surface);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  margin-top: 4px;
  max-height: 200px;
  overflow-y: auto;
  z-index: 10;
}

.suggestion-item {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  cursor: pointer;
  transition: background 0.2s;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.suggestion-item:hover {
  background: var(--hover);
}

.suggestion-item:last-child {
  border-bottom: none;
}

.suggestion-category {
  margin-left: auto;
  font-size: 12px;
  color: var(--text-secondary);
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 8px;
  border-radius: 4px;
}

.categories-section {
  margin-bottom: 24px;
}

.section-label {
  font-size: 14px;
  color: var(--text-secondary);
  margin-bottom: 12px;
}

.category-buttons {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px;
}

.category-btn {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  transition: all 0.2s;
}

.category-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  transform: translateY(-2px);
}

.category-options {
  margin-bottom: 24px;
}

.category-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 16px;
}

.back-btn {
  padding: 8px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
}

.back-btn:hover {
  background: var(--hover);
}

.category-header h4 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
}

.options-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 12px;
}

.option-btn {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text);
  cursor: pointer;
  text-align: left;
  transition: all 0.2s;
}

.option-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.option-icon {
  width: 24px;
  height: 24px;
  margin-bottom: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.option-label {
  font-size: 14px;
  font-weight: 600;
}

.option-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.rule-config {
  border-top: 1px solid var(--border);
  padding-top: 24px;
}

.rule-config h4 {
  margin: 0 0 16px 0;
  font-size: 18px;
  font-weight: 600;
}

.form-group {
  margin-bottom: 16px;
}

.form-group label {
  display: block;
  margin-bottom: 6px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.form-group input,
.form-group select {
  width: 100%;
  padding: 10px 12px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text);
  font-size: 14px;
}

.form-group select[multiple] {
  height: 100px;
  resize: vertical;
}

.form-group input:focus,
.form-group select:focus {
  outline: none;
  border-color: var(--border-active);
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
}

.btn-primary,
.btn-secondary {
  padding: 10px 20px;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.btn-primary {
  background: var(--primary);
  color: white;
}

.btn-primary:hover {
  background: var(--primary-dark);
}

.btn-secondary {
  background: transparent;
  color: var(--text);
  border-color: var(--border);
}

.btn-secondary:hover {
  background: var(--hover);
}

@media (max-width: 768px) {
  .controls-section {
    flex-direction: column;
  }
  
  .cluster-selector {
    min-width: auto;
  }
  
  .legend {
    flex-direction: column;
    align-items: flex-start;
    gap: 8px;
  }

  .category-buttons,
  .options-grid {
    grid-template-columns: 1fr;
  }
  
  .modal-content {
    width: 95%;
    margin: 10px;
  }
}
</style>