<template>
  <article class="panel span-wide">
    <div class="panel-head">
      <div>
        <p class="meta-label">Workspace</p>
        <h3>AI-Powered Documentation and Analysis</h3>
      </div>
    </div>

    <!-- Tabs Navigation -->
    <div class="tabs-navigation">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="['tab-button', { active: activeTab === tab.id }]"
        @click="activeTab = tab.id"
      >
        <component :is="tab.icon" size="16" weight="regular" class="tab-icon" />
        <span class="tab-label">{{ tab.label }}</span>
      </button>
    </div>

    <!-- Tab Content -->
    <div class="tab-content">
      <transition name="tab-fade" mode="out-in">
        <!-- History Tab -->
        <div v-if="activeTab === 'history'" key="history" class="history-tab">

          <div class="timeline">
            <IncidentTimelineItem
              v-for="incident in filteredTimeline"
              :key="incident.id"
              :incident="incident"
              :format-when="formatWhen"
            />
            <div v-if="filteredTimeline.length === 0" class="empty-state">
              <div class="empty-state-content">
                <p>No incidents match the selected filters.</p>
                <div class="empty-state-actions">
                  <button class="ghost-btn" @click="clearFilters">
                    Clear All Filters
                  </button>
                  <button class="primary-btn" @click="activeTab = 'agent'">
                    Ask AI to Broaden Search
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Terminal Tab -->
        <div v-else-if="activeTab === 'terminal'" key="terminal" class="terminal-tab">
          <div class="terminal-container">
            <div class="terminal-header">
              <span class="terminal-title">Command Terminal</span>
              <button class="ghost-btn small" @click="clearTerminal">
                Clear
              </button>
            </div>
            <div class="terminal-output" ref="terminalOutput">
              <div v-for="(line, index) in terminalLines" :key="index" class="terminal-line">
                <span :class="['line-type', line.type]">{{ line.type === 'command' ? '$' : '>' }}</span>
                <span class="line-text">{{ line.text }}</span>
              </div>
            </div>
            <div class="terminal-input-section" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
              <div class="terminal-input-wrapper">
                <div class="terminal-input-line">
                  <span class="terminal-prompt">$</span>
                  <textarea
                    v-model="terminalInput"
                    placeholder="Enter command or type /agent to talk to the agent..."
                    @keydown.enter.exact.prevent="executeTerminalCommand"
                    @keydown.up.prevent="navigateHistory(-1)"
                    @keydown.down.prevent="navigateHistory(1)"
                    @keydown.tab.prevent="handleTabCompletion"
                    @keydown.ctrl.c.prevent="cancelCommand"
                    @keydown.ctrl.l.prevent="clearTerminal"
                    rows="1"
                    class="terminal-textarea"
                    ref="terminalInputEl"
                  />
                </div>
                <button
                  @click="executeTerminalCommand"
                  :disabled="!terminalInput.trim()"
                  class="primary-btn send-btn"
                >
                  Execute
                </button>
              </div>
              <div class="widget-sidebar" :class="{ collapsed: sidebarCollapsed }">
                <div class="widget-sidebar-header">
                  <button class="sidebar-toggle-btn" @click="sidebarCollapsed = !sidebarCollapsed" :title="sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'">
                    {{ sidebarCollapsed ? '>>' : '<<' }}
                  </button>
                  <span v-if="!sidebarCollapsed" class="widget-sidebar-title">Widgets</span>
                  <button v-if="!sidebarCollapsed" class="add-widget-btn" @click="addWidget" title="Add new widget">
                    ➕
                  </button>
                </div>
                <div v-if="!sidebarCollapsed" class="widget-stack">
                  <div
                    v-for="widget in widgets"
                    :key="widget.id"
                    class="widget"
                  >
                    <div class="widget-header">
                      <span class="widget-icon">{{ widget.icon }}</span>
                      <span class="widget-title">{{ widget.title }}</span>
                      <button class="widget-edit-btn" @click="editWidget(widget)" title="Edit widget">
                        ✏️
                      </button>
                    </div>
                    <div class="widget-labels">
                      <button
                        v-for="label in widget.labels"
                        :key="label.id"
                        class="widget-label"
                        :style="{ backgroundColor: label.color }"
                        @click="insertCommand(label.command)"
                        @dblclick="executeWidgetCommand(label.command)"
                        :title="label.command + ' (Click to insert, double-click to execute)'"
                      >
                        <span class="label-icon">{{ label.icon }}</span>
                        <span class="label-text">{{ label.text }}</span>
                      </button>

                     </div>
                   </div>
                 </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Agent Tab -->
        <div v-else-if="activeTab === 'agent'" key="agent" class="agent-tab">
          <div class="agent-container">
            <div class="agent-header">
              <span class="agent-title">AI Agent Interaction</span>
              <span class="agent-subtitle">Type /agent in terminal or use direct chat</span>
            </div>
            <div class="agent-chat">
              <div class="agent-toolbar">
                <button class="toolbar-btn" @click="quickAction('trends')">
                  <PhTrendUp size="14" weight="bold" />
                  <span>Document trends</span>
                </button>
                <button class="toolbar-btn" @click="quickAction('vulnerabilities')">
                  <PhShieldWarning size="14" weight="bold" />
                  <span>Scan vulnerabilities</span>
                </button>
                <button class="toolbar-btn" @click="quickAction('changes')">
                  <PhGitBranch size="14" weight="bold" />
                  <span>Track changes</span>
                </button>
                <button class="toolbar-btn" @click="quickAction('updates')">
                   <PhNotebook size="14" weight="bold" />
                  <span>Version updates</span>
                </button>
              </div>
              <div class="chat-messages">
                <div v-for="(message, index) in agentMessages" :key="index" class="chat-message">
                  <div class="message-avatar">
                     <PhRobot v-if="message.role === 'assistant'" size="20" weight="regular" />
                     <PhUser v-else size="20" weight="regular" />
                  </div>
                  <div class="message-content">
                    <div class="message-text">{{ message.content }}</div>
                    <div class="message-time">{{ formatTime(message.timestamp) }}</div>
                  </div>
                </div>
              </div>
              <div class="chat-input-section">
                <textarea
                  v-model="agentInput"
                  placeholder="Ask the agent for analysis or type /terminal to switch..."
                  @keydown.enter.exact.prevent="sendAgentMessage"
                  rows="3"
                  class="chat-textarea"
                />
                <button
                  @click="sendAgentMessage"
                  :disabled="!agentInput.trim()"
                  class="primary-btn"
                >
                  Send
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Workspace Tab -->
        <div v-else-if="activeTab === 'workspace'" key="workspace" class="workspace-tab">
          <div class="workspace-grid">
            <!-- Left Column: Insights -->
            <div class="workspace-insights">
              <div class="insights-header">
                <p class="meta-label">Argus Intelligence</p>
                <h4>Workspace Insights</h4>
              </div>
              
              <!-- KPI Metrics -->
              <div class="insights-kpi-grid">
                <div class="kpi-card">
                  <div class="kpi-label">Total Incidents</div>
                  <div class="kpi-value">{{ timeline.length }}</div>
                  <div class="kpi-trend" v-if="incidentTrend > 0">↑ {{ incidentTrend }}%</div>
                  <div class="kpi-trend negative" v-else-if="incidentTrend < 0">↓ {{ Math.abs(incidentTrend) }}%</div>
                </div>
                <div class="kpi-card">
                  <div class="kpi-label">Critical</div>
                  <div class="kpi-value">{{ criticalIncidents.length }}</div>
                  <div class="kpi-subtext">{{ Math.round((criticalIncidents.length / timeline.length) * 100) || 0 }}% of total</div>
                </div>
                <div class="kpi-card">
                  <div class="kpi-label">Avg Resolution</div>
                  <div class="kpi-value">{{ avgResolutionTime }}</div>
                  <div class="kpi-subtext">hours</div>
                </div>
                <div class="kpi-card">
                  <div class="kpi-label">Top Namespace</div>
                  <div class="kpi-value">{{ topNamespace.name || 'N/A' }}</div>
                  <div class="kpi-subtext">{{ topNamespace.count || 0 }} incidents</div>
                </div>
              </div>
              
              <!-- Trend Sparkline -->
              <div class="insights-trend">
                <div class="trend-header">
                  <h5>Incident Trend (Last 7 days)</h5>
                  <div class="trend-legend">
                    <span class="legend-item">
                      <span class="legend-color" style="background-color: #8a82e0;"></span>
                      <span class="legend-label">Incidents</span>
                    </span>
                  </div>
                </div>
                <div class="trend-chart">
                  <div class="sparkline-bars">
                    <div 
                      v-for="(count, idx) in dailyIncidentCounts" 
                      :key="idx" 
                      class="sparkline-bar"
                      :style="{ height: getBarHeight(count) + '%', backgroundColor: '#8a82e0' }"
                      :title="`Day ${idx + 1}: ${count} incidents`"
                    ></div>
                  </div>
                  <div class="sparkline-labels">
                    <span v-for="day in ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']" :key="day" class="sparkline-label">{{ day }}</span>
                  </div>
                </div>
              </div>
              
              <!-- Pattern Analysis -->
              <div class="insights-patterns">
                <div class="patterns-header">
                  <h5>Pattern Analysis</h5>
                  <button class="ghost-btn small" @click="refreshInsights">
                    Refresh
                  </button>
                </div>
                <div class="patterns-list">
                  <div v-if="commonPatterns.length > 0" class="patterns-content">
                    <div v-for="pattern in commonPatterns.slice(0, 3)" :key="pattern.pattern" class="pattern-item">
                      <div class="pattern-icon"><PhMagnifyingGlass size="16" weight="regular" /></div>
                      <div class="pattern-details">
                        <div class="pattern-title">{{ pattern.pattern }}</div>
                        <div class="pattern-meta">{{ pattern.count }} occurrences</div>
                      </div>
                      <div class="pattern-confidence">
                        <span class="confidence-badge" :class="getConfidenceClass(pattern.confidence)">
                          {{ pattern.confidence }}% confidence
                        </span>
                      </div>
                    </div>
                  </div>
                  <div v-else class="patterns-empty">
                    <p>No patterns detected. Add more incident data.</p>
                  </div>
                </div>
              </div>
              
              <!-- Recent Activity -->
              <div class="insights-activity">
                <div class="activity-header">
                  <h5>Recent Activity</h5>
                  <span class="activity-count">{{ recentIncidents.length }} items</span>
                </div>
                <div class="activity-list">
                  <div v-for="incident in recentIncidents.slice(0, 5)" :key="incident.id" class="activity-item">
                    <div class="activity-severity" :class="incident.severity"></div>
                    <div class="activity-content">
                      <div class="activity-title">{{ incident.name }}</div>
                      <div class="activity-meta">{{ incident.namespace }} • {{ formatWhen(incident.timestamp) }}</div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
            
            <!-- Right Column: Notes Editor -->
            <div class="workspace-notes">
              <div class="notes-container">
                <div class="notes-header">
                  <div class="notes-title-section">
                    <input
                      v-model="noteTitle"
                      placeholder="Note Title"
                      class="note-title-input"
                    />
                    <div class="notes-actions">
                      <button class="ghost-btn" @click="saveNote" :disabled="!noteContent.trim()">
                        {{ isSaving ? 'Saving...' : 'Save' }}
                      </button>
                      <button class="ghost-btn" @click="lockNote" :disabled="noteLocked">
                         <template v-if="noteLocked"><PhLock size="14" weight="regular" /> Locked</template>
                         <template v-else><PhLockOpen size="14" weight="regular" /> Lock</template>
                      </button>
                      <button class="ghost-btn danger" @click="deleteNote" v-if="noteId">
                        Delete
                      </button>
                    </div>
                  </div>
                </div>
                <div class="markdown-editor">
                  <textarea
                    v-model="noteContent"
                    placeholder="Write your notes in Markdown..."
                    :disabled="noteLocked"
                    rows="15"
                    class="markdown-textarea"
                  />
                  <div class="markdown-preview">
                    <div class="preview-header">
                      <span class="preview-title">Preview</span>
                    </div>
                    <div class="preview-content" v-html="renderMarkdown(noteContent)"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </transition>
    </div>
  </article>

  <!-- Widget Editor Modal -->
  <div v-if="showWidgetEditor" class="widget-editor-modal-overlay" @click="cancelEdit">
    <div class="widget-editor-modal" @click.stop>
      <div class="widget-editor-header">
        <h3>{{ editingWidget?.id ? 'Edit Widget' : 'Add Widget' }}</h3>
        <button class="widget-editor-close-btn" @click="cancelEdit">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="widget-editor-content">
        <!-- Widget Fields -->
        <div class="widget-editor-section">
          <label class="widget-editor-label">Title</label>
           <input v-model="editingWidget!.title" type="text" class="widget-editor-input" placeholder="Widget title">
        </div>
        <div class="widget-editor-section">
          <label class="widget-editor-label">Icon</label>
           <input v-model="editingWidget!.icon" type="text" class="widget-editor-input" placeholder="Emoji icon">
        </div>
        <div class="widget-editor-section">
          <label class="widget-editor-label">Position</label>
           <input v-model="editingWidget!.position" type="number" min="0" class="widget-editor-input" placeholder="0">
        </div>

        <!-- Labels Section -->
        <div class="widget-editor-section">
          <div class="widget-editor-section-header">
            <label class="widget-editor-label">Labels</label>
            <button type="button" class="widget-editor-add-btn" @click="addLabel">
              Add Label
            </button>
          </div>
           <div v-if="editingWidget!.labels.length === 0" class="empty-labels">
            <p>No labels. Add a label to make the widget useful.</p>
          </div>
          <div v-else class="label-list">
             <div v-for="(label, index) in editingWidget!.labels" :key="label.id" class="label-editor">
              <div class="label-editor-fields">
                <input v-model="label.text" type="text" class="widget-editor-input small" placeholder="Label text">
                <input v-model="label.icon" type="text" class="widget-editor-input small" placeholder="Icon">
                <input v-model="label.command" type="text" class="widget-editor-input" placeholder="Command to insert">
                <input v-model="label.color" type="text" class="widget-editor-input small color-input" placeholder="Color">
                <button type="button" class="widget-editor-remove-btn" @click="removeLabel(index)" title="Remove label">
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                    <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
                  </svg>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <div class="widget-editor-footer">
        <button v-if="editingWidget?.id" class="widget-editor-delete-btn" @click="deleteWidget(editingWidget.id)">
          Delete Widget
        </button>
        <div class="widget-editor-footer-actions">
          <button class="widget-editor-cancel-btn" @click="cancelEdit">
            Cancel
          </button>
          <button class="widget-editor-save-btn" @click="saveWidget">
            Save Widget
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, nextTick, onMounted, watch } from 'vue'
import { PhScroll, PhTerminal, PhRobot, PhNotebook, PhTrendUp, PhShieldWarning, PhGitBranch, PhMagnifyingGlass, PhUser, PhLock, PhLockOpen } from '@phosphor-icons/vue'
import IncidentTimelineItem from '../cards/IncidentTimelineItem.vue'
import { data } from '../../../wailsjs/go/models'
import { GetWidgets, SaveWidget, DeleteWidget, RunCommand, AskAI } from '../../../wailsjs/go/main/App'

const NOTE_STORAGE_KEY = 'kube-watcher-note'

// Simple debounce function
function debounce<T extends (...args: any[]) => any>(fn: T, delay: number): (...args: Parameters<T>) => void {
  let timeout: ReturnType<typeof setTimeout> | null = null
  return (...args: Parameters<T>) => {
    if (timeout) clearTimeout(timeout)
    timeout = setTimeout(() => fn(...args), delay)
  }
}

interface Props {
  timeline: data.Incident[]
  formatWhen: (value: unknown) => string
  renderMarkdown?: (text: string) => string
  selectedSeverity?: string
  selectedState?: string
  selectedTimeRange?: string
}

const props = withDefaults(defineProps<Props>(), {
  renderMarkdown: (text: string) => text,
  selectedSeverity: 'all',
  selectedState: 'all',
  selectedTimeRange: '24h'
})

const emit = defineEmits<{
  'clear-filters': []
}>()

// Tabs
type TabId = 'history' | 'terminal' | 'agent' | 'workspace'
const tabs = [
  { id: 'history' as TabId, label: 'History', icon: PhScroll },
  { id: 'terminal' as TabId, label: 'Terminal', icon: PhTerminal },
  { id: 'agent' as TabId, label: 'Agent', icon: PhRobot },
  { id: 'workspace' as TabId, label: 'Workspace', icon: PhNotebook }
]
const activeTab = ref<TabId>('history')

// History Tab State (filters now come from parent via props)



const filteredTimeline = computed(() => {
  let filtered = props.timeline

  if (props.selectedSeverity !== 'all') {
    filtered = filtered.filter(incident => incident.severity === props.selectedSeverity)
  }

  // Time range filtering
  if (props.selectedTimeRange !== 'all') {
    const now = new Date()
    let cutoff = new Date()
    switch (props.selectedTimeRange) {
      case '1h':
        cutoff.setHours(now.getHours() - 1)
        break
      case '24h':
        cutoff.setDate(now.getDate() - 1)
        break
      case '7d':
        cutoff.setDate(now.getDate() - 7)
        break
      case '30d':
        cutoff.setDate(now.getDate() - 30)
        break
      default:
        cutoff = new Date(0) // beginning of time
    }
    filtered = filtered.filter(incident => {
      const incidentDate = new Date(incident.timestamp)
      return incidentDate >= cutoff
    })
  }

  // State filtering (placeholder - incidents don't have state field yet)
  if (props.selectedState !== 'all') {
    // TODO: Implement state filtering when incidents have state field
    // For now, pass all incidents
    console.warn('State filtering not yet implemented')
  }

  return filtered
})

// Workspace insights computed properties
const totalIncidents = computed(() => props.timeline.length)

const criticalIncidents = computed(() => 
  props.timeline.filter(incident => incident.severity === 'critical')
)

const incidentTrend = computed(() => {
  // Mock trend: calculate percentage change in incidents over last 7 days vs previous 7 days
  const now = new Date()
  const lastWeekEnd = new Date(now)
  const lastWeekStart = new Date(now)
  lastWeekStart.setDate(lastWeekEnd.getDate() - 7)
  const prevWeekEnd = new Date(lastWeekStart)
  const prevWeekStart = new Date(prevWeekEnd)
  prevWeekStart.setDate(prevWeekEnd.getDate() - 7)
  
  const countLastWeek = props.timeline.filter(incident => {
    const date = new Date(incident.timestamp)
    return date >= lastWeekStart && date <= lastWeekEnd
  }).length
  
  const countPrevWeek = props.timeline.filter(incident => {
    const date = new Date(incident.timestamp)
    return date >= prevWeekStart && date <= prevWeekEnd
  }).length
  
  if (countPrevWeek === 0) return countLastWeek > 0 ? 100 : 0
  return Math.round(((countLastWeek - countPrevWeek) / countPrevWeek) * 100)
})

const avgResolutionTime = computed(() => {
  // Mock: average resolution time in hours
  // Since incidents don't have resolution state, compute based on severity
  if (props.timeline.length === 0) return 'N/A'
  // Assume critical incidents take longer to resolve
  const totalHours = props.timeline.reduce((sum, incident) => {
    const base = incident.severity === 'critical' ? 12 : 4
    return sum + base + (Math.random() * 6)
  }, 0)
  const avg = totalHours / props.timeline.length
  return avg < 1 ? `${Math.round(avg * 60)}m` : `${avg.toFixed(1)}h`
})

const topNamespace = computed(() => {
  // Group incidents by namespace
  const namespaceMap: Record<string, number> = {}
  props.timeline.forEach(incident => {
    const ns = incident.namespace || 'default'
    namespaceMap[ns] = (namespaceMap[ns] || 0) + 1
  })
  const entries = Object.entries(namespaceMap)
  if (entries.length === 0) return { name: 'N/A', count: 0 }
  const [name, count] = entries.reduce((max, entry) => entry[1] > max[1] ? entry : max)
  return { name, count }
})

const dailyIncidentCounts = computed(() => {
  // Generate mock daily counts for last 7 days
  const counts = []
  for (let i = 6; i >= 0; i--) {
    const date = new Date()
    date.setDate(date.getDate() - i)
    const dayCount = props.timeline.filter(incident => {
      const incidentDate = new Date(incident.timestamp)
      return incidentDate.toDateString() === date.toDateString()
    }).length
    counts.push(dayCount)
  }
  return counts
})

const getBarHeight = (count: number) => {
  const max = Math.max(...dailyIncidentCounts.value, 1)
  return (count / max) * 100
}

const refreshInsights = () => {
  // Force recomputation by touching a reactive variable
  // This is a no-op; computed properties will update automatically when timeline changes
  console.log('Refreshing insights')
}

const commonPatterns = computed(() => {
  // Mock patterns detected from incidents
  return [
    { pattern: 'Pod restart loops', confidence: 85, count: 12 },
    { pattern: 'Memory spikes after deployment', confidence: 72, count: 8 },
    { pattern: 'Network timeouts during peak hours', confidence: 64, count: 5 },
    { pattern: 'CPU throttling in monitoring namespace', confidence: 58, count: 4 }
  ]
})

const getConfidenceClass = (confidence: number) => {
  if (confidence >= 80) return 'confidence-high'
  if (confidence >= 60) return 'confidence-medium'
  return 'confidence-low'
}

const recentIncidents = computed(() => {
  // Return most recent incidents (sorted by timestamp)
  return [...props.timeline]
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    .slice(0, 10)
})

const clearFilters = () => {
  emit('clear-filters')
}

// Persist hidden options to localStorage






// Terminal Tab State
const terminalInput = ref('')
const terminalLines = ref<Array<{type: 'command' | 'output' | 'error', text: string}>>([])
const terminalOutput = ref<HTMLElement | null>(null)
const terminalInputEl = ref<HTMLTextAreaElement | null>(null)
const commandHistory = ref<string[]>([])
const historyIndex = ref(-1)
const sidebarCollapsed = ref(false)

watch(sidebarCollapsed, (newValue) => {
  localStorage.setItem('sidebarCollapsed', JSON.stringify(newValue))
})

onMounted(() => {
  const savedSidebarCollapsed = localStorage.getItem('sidebarCollapsed')
  if (savedSidebarCollapsed !== null) {
    try {
      sidebarCollapsed.value = JSON.parse(savedSidebarCollapsed)
    } catch (e) {
      console.error('Failed to parse sidebarCollapsed', e)
    }
  }
})

// Widgets (types imported from data)
const widgets = ref<data.Widget[]>([])
const loadingWidgets = ref(false)

async function loadWidgets() {
  loadingWidgets.value = true
  try {
    const loaded = await GetWidgets()
    widgets.value = loaded.sort((a, b) => (a.position || 0) - (b.position || 0))
  } catch (error) {
    console.error('Failed to load widgets:', error)
    // Fallback to default widgets if needed
    widgets.value = []
  } finally {
    loadingWidgets.value = false
  }
}

// Agent Tab State
const agentInput = ref('')
const agentMessages = ref<Array<{role: 'assistant' | 'user', content: string, timestamp: Date}>>([])

// Notes Tab State
const noteId = ref<string | null>(null)
const noteTitle = ref('Untitled Note')
const noteContent = ref('')
const noteLocked = ref(false)
const isSaving = ref(false)

// Widget Editor State
const showWidgetEditor = ref(false)
const editingWidget = ref<data.Widget | null>(null)
const editingWidgetIndex = ref(-1)

// Functions
const executeTerminalCommand = async () => {
  const command = terminalInput.value.trim()
  if (!command) return

  // Add command to history
  commandHistory.value.push(command)
  historyIndex.value = -1
  terminalLines.value.push({ type: 'command', text: command })
  terminalInput.value = ''

  // Handle /agent command
  if (command.startsWith('/agent')) {
    const message = command.substring(6).trim()
    if (message) {
      agentMessages.value.push({
        role: 'user',
        content: message,
        timestamp: new Date()
      })
      // Add thinking indicator
      const thinkingIndex = agentMessages.value.length
      agentMessages.value.push({
        role: 'assistant',
        content: 'Thinking...',
        timestamp: new Date()
      })
      // Call AI
      try {
        const context = JSON.stringify(props.timeline.slice(0, 5))
        const response = await AskAI(message, context)
        agentMessages.value[thinkingIndex] = {
          role: 'assistant',
          content: response,
          timestamp: new Date()
        }
      } catch (error) {
        agentMessages.value[thinkingIndex] = {
          role: 'assistant',
          content: `Error: ${error instanceof Error ? error.message : String(error)}`,
          timestamp: new Date()
        }
      }
    }
    // Switch to agent tab
    activeTab.value = 'agent'
    return
  }

  // Execute command via backend
  try {
    const output = await RunCommand(command)
    terminalLines.value.push({ type: 'output', text: output })
  } catch (error) {
    terminalLines.value.push({ type: 'error', text: error instanceof Error ? error.message : String(error) })
  }
  scrollTerminalToBottom()
  nextTick(() => {
    terminalInputEl.value?.focus()
  })
}

const navigateHistory = (direction: number) => {
  if (commandHistory.value.length === 0) return
  let newIndex = historyIndex.value + direction
  if (newIndex < -1) newIndex = -1
  if (newIndex >= commandHistory.value.length) newIndex = commandHistory.value.length - 1
  historyIndex.value = newIndex
  if (newIndex === -1) {
    terminalInput.value = ''
  } else {
    terminalInput.value = commandHistory.value[commandHistory.value.length - 1 - newIndex]
  }
}

const handleTabCompletion = () => {
  const input = terminalInput.value.trim()
  if (!input) return
  // Collect possible completions from widget labels and common commands
  const completions: string[] = []
  // Add widget label commands
  widgets.value.forEach(widget => {
    widget.labels?.forEach(label => {
      if (label.command && label.command.startsWith(input)) {
        completions.push(label.command)
      }
    })
  })
  // Add common commands
  const commonCommands = ['kubectl', 'docker', 'helm', 'make', 'go', 'npm', 'node', 'ls', 'cd', 'pwd', 'cat', 'echo', 'grep', 'awk', 'sed', 'find', 'ps', 'top', 'vi', 'nano', 'curl', 'wget', 'tar', 'gzip', 'kill', 'ping', 'netstat', 'ifconfig', 'df', 'du', 'mount', 'uname', 'whoami', 'id', 'env', 'export', 'source', 'sh', 'bash', 'python', 'python3', 'java']
  commonCommands.forEach(cmd => {
    if (cmd.startsWith(input)) {
      completions.push(cmd)
    }
  })
  // Deduplicate
  const unique = [...new Set(completions)]
  if (unique.length === 0) return
  // For now, just pick the first completion
  terminalInput.value = unique[0]
}

const cancelCommand = () => {
  terminalInput.value = ''
}

const insertCommand = (command: string) => {
  terminalInput.value = command
  nextTick(() => {
    terminalInputEl.value?.focus()
    // Move cursor to end
    if (terminalInputEl.value) {
      const len = terminalInputEl.value.value.length
      terminalInputEl.value.setSelectionRange(len, len)
    }
  })
}

const executeWidgetCommand = async (command: string) => {
  // Add command to history
  commandHistory.value.push(command)
  historyIndex.value = -1
  terminalLines.value.push({ type: 'command', text: command })

  // Handle /agent command
  if (command.startsWith('/agent')) {
    const message = command.substring(6).trim()
    if (message) {
      agentMessages.value.push({
        role: 'user',
        content: message,
        timestamp: new Date()
      })
      // Add thinking indicator
      const thinkingIndex = agentMessages.value.length
      agentMessages.value.push({
        role: 'assistant',
        content: 'Thinking...',
        timestamp: new Date()
      })
      try {
        const context = JSON.stringify(props.timeline.slice(0, 5))
        const response = await AskAI(message, context)
        agentMessages.value[thinkingIndex] = {
          role: 'assistant',
          content: response,
          timestamp: new Date()
        }
      } catch (error) {
        agentMessages.value[thinkingIndex] = {
          role: 'assistant',
          content: 'Error: ' + (error instanceof Error ? error.message : String(error)),
          timestamp: new Date()
        }
      }
    }
    activeTab.value = 'agent'
    return
  }

  // Execute command via backend
  try {
    const output = await RunCommand(command)
    terminalLines.value.push({ type: 'output', text: output })
  } catch (error) {
    terminalLines.value.push({ type: 'error', text: 'Error: ' + (error instanceof Error ? error.message : String(error)) })
  }
  scrollTerminalToBottom()
}
const clearTerminal = () => {
  terminalLines.value = []
}

const scrollTerminalToBottom = () => {
  nextTick(() => {
    if (terminalOutput.value) {
      terminalOutput.value.scrollTop = terminalOutput.value.scrollHeight
    }
  })
}

const sendAgentMessage = async () => {
  const message = agentInput.value.trim()
  if (!message) return

  agentMessages.value.push({
    role: 'user',
    content: message,
    timestamp: new Date()
  })
  agentInput.value = ''

  // Add a thinking indicator
  const thinkingIndex = agentMessages.value.length
  agentMessages.value.push({
    role: 'assistant',
    content: 'Thinking...',
    timestamp: new Date()
  })

  try {
    // Provide context: maybe timeline data
    const context = JSON.stringify(props.timeline.slice(0, 5)) // limit context
    const response = await AskAI(message, context)
    // Replace thinking message with actual response
    agentMessages.value[thinkingIndex] = {
      role: 'assistant',
      content: response,
      timestamp: new Date()
    }
  } catch (error) {
    agentMessages.value[thinkingIndex] = {
      role: 'assistant',
      content: `Error: ${error instanceof Error ? error.message : String(error)}`,
      timestamp: new Date()
    }
  }
}

function quickAction(action: string) {
  const prompts: Record<string, string> = {
    trends: 'Document recent trends and patterns in the cluster metrics and logs.',
    vulnerabilities: 'Scan for known vulnerabilities in running containers and cluster components.',
    changes: 'Track and document recent changes to resources, deployments, and configurations.',
    updates: 'Identify available major version updates for cluster components and applications.'
  }
  const prompt = prompts[action] || `Analyze ${action}`
  agentInput.value = prompt
  // Optionally auto-send
  // sendAgentMessage()
}

const loadNote = () => {
  const saved = localStorage.getItem(NOTE_STORAGE_KEY)
  if (saved) {
    try {
      const note = JSON.parse(saved)
      noteId.value = note.id || null
      noteTitle.value = note.title || 'Untitled Note'
      noteContent.value = note.content || ''
      noteLocked.value = note.locked || false
    } catch (e) {
      console.error('Failed to parse saved note', e)
    }
  }
}

const saveNote = () => {
  isSaving.value = true
  const note = {
    id: noteId.value || Date.now().toString(),
    title: noteTitle.value,
    content: noteContent.value,
    locked: noteLocked.value,
    updatedAt: new Date().toISOString()
  }
  localStorage.setItem(NOTE_STORAGE_KEY, JSON.stringify(note))
  noteId.value = note.id
  setTimeout(() => {
    isSaving.value = false
  }, 300)
}

const autoSaveNote = debounce(() => {
  if (!noteLocked.value && noteContent.value.trim()) {
    saveNote()
  }
}, 1000)

const lockNote = () => {
  noteLocked.value = !noteLocked.value
  // Save when locking/unlocking
  saveNote()
}

const deleteNote = () => {
  if (confirm('Delete this note?')) {
    localStorage.removeItem(NOTE_STORAGE_KEY)
    noteId.value = null
    noteTitle.value = 'Untitled Note'
    noteContent.value = ''
    noteLocked.value = false
  }
}

// Widget Editor Functions
const editWidget = (widget: data.Widget) => {
  editingWidget.value = data.Widget.createFrom(widget) // deep clone via factory
  editingWidgetIndex.value = widgets.value.findIndex(w => w.id === widget.id)
  showWidgetEditor.value = true
}

const addWidget = () => {
  if (widgets.value.length >= 10) {
    alert('Maximum of 10 widgets allowed. Please delete a widget before adding another.')
    return
  }
  editingWidget.value = data.Widget.createFrom({
    id: '',
    title: 'New Widget',
    icon: '📦',
    position: widgets.value.length,
    labels: []
  })
  editingWidgetIndex.value = -1
  showWidgetEditor.value = true
}

const addLabel = () => {
  if (!editingWidget.value) return
  editingWidget.value.labels.push(data.WidgetLabel.createFrom({
    id: 'label_' + Date.now() + '_' + Math.random().toString(36).substring(2),
    text: 'New Label',
    icon: '🏷️',
    command: '',
    color: 'rgba(59, 130, 246, 0.15)'
  }))
}

const removeLabel = (index: number) => {
  if (!editingWidget.value) return
  editingWidget.value.labels.splice(index, 1)
}

const saveWidget = async () => {
  if (!editingWidget.value) return
  
  // Ensure ID is set for new widgets
  if (!editingWidget.value.id) {
    editingWidget.value.id = 'widget_' + Date.now()
  }
  
  try {
    await SaveWidget(editingWidget.value)
    await loadWidgets() // Reload from backend
    cancelEdit()
  } catch (error) {
    console.error('Failed to save widget:', error)
    alert('Failed to save widget')
  }
}

const deleteWidget = async (widgetId: string) => {
  if (!confirm('Delete this widget?')) return
  
  try {
    await DeleteWidget(widgetId)
    await loadWidgets() // Reload from backend
  } catch (error) {
    console.error('Failed to delete widget:', error)
    alert('Failed to delete widget')
  }
}

const cancelEdit = () => {
  showWidgetEditor.value = false
  editingWidget.value = null
  editingWidgetIndex.value = -1
}

const formatTime = (date: Date) => {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// Auto-scroll terminal and load widgets
onMounted(() => {
  scrollTerminalToBottom()
  loadWidgets()
  loadNote()
})

// Load widgets when switching to terminal tab
watch(activeTab, (newTab) => {
  if (newTab === 'terminal') {
    if (widgets.value.length === 0) {
      loadWidgets()
    }
    nextTick(() => {
      terminalInputEl.value?.focus()
    })
  }
})

// Auto-save note when content or title changes
watch([() => noteContent.value, () => noteTitle.value], () => {
  autoSaveNote()
}, { deep: true })
</script>

<style scoped>
.panel {
  padding: 24px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border);
  border-radius: 12px;
  margin-bottom: 24px;
  min-width: 0;
}

.panel.span-wide {
  grid-column: 1 / -1;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
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

/* Tabs Navigation - iOS Segmented Control */
.tabs-navigation {
  display: inline-flex;
  gap: 0;
  margin-bottom: 20px;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: transparent;
  padding: 6px;
  overflow: hidden;
}

.tab-button {
  padding: 12px 24px;
  border-radius: 999px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.2, 0.8, 0.4, 1);
  display: flex;
  align-items: center;
  gap: 8px;
  position: relative;
  white-space: nowrap;
}

.tab-button + .tab-button {
  margin-left: 0;
}

.tab-button:hover {
  background: rgba(255, 255, 255, 0.1);
}

.tab-button.active {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text);
  border: none;
}

.tab-icon {
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 16px;
  height: 16px;
}

.tab-icon svg {
  width: 100%;
  height: 100%;
}

.tab-label {
  font-size: 14px;
  letter-spacing: 0.01em;
}

/* Tab Transition */
.tab-fade-enter-active,
.tab-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.tab-fade-enter-from,
.tab-fade-leave-to {
  opacity: 0;
  transform: translateY(8px);
}

/* Tab Content Container */
.tab-content {
  position: relative;
  min-height: 500px;
}

/* History Tab */


.timeline {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.empty-state {
  padding: 40px 30px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 14px;
}

.empty-state-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  max-width: 400px;
  margin: 0 auto;
  padding: 30px;
  border-radius: 12px;
  background: transparent;
  border: 1px solid var(--border);
}

.empty-state-actions {
  display: flex;
  gap: 12px;
  justify-content: center;
  flex-wrap: wrap;
}

/* Terminal Tab */
.terminal-container {
  display: flex;
  flex-direction: column;
  gap: 0;
}

.terminal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
}

.terminal-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.ghost-btn.small {
  padding: 4px 8px;
  font-size: 12px;
}

.terminal-output {
  padding: 16px;
  border-radius: 12px 12px 0 0;
  background: #000;
  border: 1px solid var(--border);
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  line-height: 1.5;
  max-height: 300px;
  overflow-y: auto;
  color: #d1d5db;
}

.terminal-line {
  margin-bottom: 4px;
  display: flex;
  gap: 8px;
}

.line-type {
  font-weight: 600;
  min-width: 16px;
}

.line-type.command {
  color: var(--success);
}

.line-type.output {
  color: var(--text-secondary);
}

.line-type.error {
  color: var(--error);
}

.terminal-input-section {
  display: grid;
  grid-template-columns: 3fr 1fr;
  gap: 16px;
  background: #000;
  border: 1px solid var(--border);
  border-top: none;
  border-radius: 0 0 12px 12px;
  padding: 16px;
}

.terminal-input-section.sidebar-collapsed {
  grid-template-columns: 1fr auto;
  gap: 8px;
}

.widget-sidebar {
  padding: 16px;
  border-radius: 12px;
  background: transparent;
  border: 1px solid var(--border);
  box-sizing: border-box;
}

.widget-sidebar.collapsed {
  padding: 8px;
  width: 40px;
  min-width: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  justify-self: start; /* Prevent stretching in grid cell */
  align-self: start; /* Align to top of grid cell */
}

.widget-sidebar.collapsed .widget-sidebar-header {
  flex-direction: column;
  gap: 4px;
  margin-bottom: 0;
}

.widget-sidebar.collapsed .sidebar-toggle-btn {
  padding: 6px;
  font-size: 10px;
}

.widget-sidebar-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
}

.sidebar-toggle-btn {
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.sidebar-toggle-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.widget-sidebar-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.add-widget-btn {
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  flex-shrink: 0;
}

.add-widget-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.widget-stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-height: 320px; /* Show 4 widgets, scroll after */
  overflow-y: auto;
  scrollbar-width: thin;
  scrollbar-color: var(--border) transparent;
}

.widget-stack::-webkit-scrollbar {
  width: 6px;
}

.widget-stack::-webkit-scrollbar-track {
  background: transparent;
}

.widget-stack::-webkit-scrollbar-thumb {
  background-color: var(--border);
  border-radius: 3px;
}

.widget {
  padding: 12px;
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(10px);
}

.widget-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
  font-size: 12px;
  color: var(--text-secondary);
}

.widget-icon {
  font-size: 14px;
}

.widget-title {
  font-weight: 500;
}

.widget-labels {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.widget-label {
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.2);
  background: transparent;
  color: var(--text-primary);
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.widget-label:hover {
  background: rgba(255, 255, 255, 0.1);
  border-color: var(--border-active);
}

.label-icon {
  font-size: 10px;
}

.terminal-input-wrapper {
  display: flex;
  flex-direction: row;
  align-items: center;
  gap: 8px;
  min-width: 0; /* Allow shrinking in grid */
}

.terminal-input-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
}

.terminal-prompt {
  color: var(--success);
  font-weight: 600;
  flex-shrink: 0;
}

.terminal-textarea {
  padding: 0;
  border: none;
  background: transparent;
  color: #d1d5db;
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
  min-width: 0; /* Allow shrinking */
  flex: 1;
  line-height: 1.5;
}

.terminal-textarea:focus {
  border-color: var(--primary);
}

.send-btn {
  align-self: flex-end;
}

/* Agent Tab */
.agent-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.agent-header {
  padding: 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
}

.agent-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  display: block;
  margin-bottom: 4px;
}

.agent-subtitle {
  font-size: 12px;
  color: var(--text-secondary);
}

.agent-chat {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.chat-messages {
  padding: 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
  max-height: 300px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chat-message {
  display: flex;
  gap: 12px;
}

.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: rgba(59, 130, 246, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}

.message-content {
  flex: 1;
  padding: 8px 12px;
  border-radius: 12px;
  background: var(--card-bg);
  border: 1px solid var(--border);
}

.message-text {
  font-size: 14px;
  line-height: 1.5;
  margin-bottom: 4px;
}

.message-time {
  font-size: 11px;
  color: var(--text-tertiary);
  text-align: right;
}

.chat-input-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.chat-textarea {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-size: 14px;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
}

.chat-textarea:focus {
  border-color: var(--primary);
}

/* Notes Tab */
.notes-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.notes-header {
  padding: 16px;
  border-radius: 12px;
  background: rgba(0, 0, 0, 0.05);
  border: 1px solid var(--border);
}

.notes-title-section {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
}

.note-title-input {
  flex: 1;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-size: 16px;
  font-weight: 600;
  outline: none;
}

.note-title-input:focus {
  border-color: var(--primary);
}

.notes-actions {
  display: flex;
  gap: 8px;
}

.ghost-btn.danger {
  color: var(--error);
  border-color: var(--error);
}

.ghost-btn.danger:hover {
  background: rgba(239, 68, 68, 0.1);
}

.markdown-editor {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.markdown-textarea {
  padding: 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-family: 'SF Mono', monospace;
  font-size: 14px;
  line-height: 1.6;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
}

.markdown-textarea:focus {
  border-color: var(--primary);
}

.markdown-textarea:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.markdown-preview {
  padding: 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  overflow-y: auto;
}

.preview-header {
  margin-bottom: 16px;
  padding-bottom: 8px;
  border-bottom: 1px solid var(--border);
}

.preview-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.preview-content {
  font-size: 14px;
  line-height: 1.6;
}

.preview-content :deep(*) {
  margin: 0;
}

.preview-content :deep(ul),
.preview-content :deep(ol) {
  padding-left: 1.5em;
  margin: 0.5em 0;
}

.preview-content :deep(code) {
  background: rgba(0, 0, 0, 0.1);
  padding: 2px 4px;
  border-radius: 4px;
  font-family: 'SF Mono', monospace;
  font-size: 0.9em;
}

.preview-content :deep(pre) {
  background: rgba(0, 0, 0, 0.1);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 0.5em 0;
}

.preview-content :deep(pre code) {
  background: none;
  padding: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .filter-section {
    flex-direction: column;
  }
}
@media (max-width: 1024px) {
  .terminal-input-section {
    grid-template-columns: 1fr;
  }
  
  .terminal-input-section.sidebar-collapsed {
    grid-template-columns: 1fr auto;
  }
  
  .markdown-editor {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .tabs-navigation {
    flex-wrap: wrap;
  }
  
  .tab-button {
    padding: 6px 12px;
    font-size: 12px;
  }
  

  

  

  
  .notes-title-section {
    flex-direction: column;
    align-items: stretch;
  }
  
  .notes-actions {
    justify-content: flex-start;
  }
}

/* Widget Editor Modal */
.widget-editor-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
   background: var(--modal-overlay-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
}

.widget-editor-modal {
  background: var(--modal-bg);
  border-radius: 24px;
  width: 100%;
  max-width: 700px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border);
}

.widget-editor-header {
  padding: 24px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.widget-editor-header h3 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.widget-editor-close-btn {
  padding: 8px;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.widget-editor-close-btn:hover {
  background: var(--hover);
}

.widget-editor-content {
  padding: 24px;
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.widget-editor-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.widget-editor-section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.widget-editor-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-secondary);
}

.widget-editor-input {
  padding: 10px 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-size: 14px;
  outline: none;
  transition: border-color 0.2s;
}

.widget-editor-input:focus {
  border-color: var(--primary);
}

.widget-editor-input.small {
  flex: 1;
  min-width: 80px;
}

.widget-editor-input.color-input {
  width: 120px;
}

.label-editor {
  margin-bottom: 12px;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
}

.label-editor-fields {
  display: grid;
  grid-template-columns: 1fr 1fr 2fr 1fr auto;
  gap: 8px;
  align-items: center;
}

.widget-editor-add-btn,
.widget-editor-cancel-btn,
.widget-editor-save-btn {
  padding: 10px 16px;
  border-radius: 8px;
  border: none;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.widget-editor-add-btn {
  background: transparent;
  color: var(--primary);
  border: 1px solid var(--primary);
}

.widget-editor-add-btn:hover {
  background: rgba(59, 130, 246, 0.1);
}

.empty-labels {
  padding: 20px;
  text-align: center;
  color: var(--text-secondary);
  font-size: 14px;
  border: 1px dashed var(--border);
  border-radius: 8px;
}

.label-list {
  max-height: 300px;
  overflow-y: auto;
}

.widget-editor-remove-btn {
  padding: 6px;
  border-radius: 6px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.widget-editor-remove-btn:hover {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
}

.widget-editor-footer {
  padding: 24px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.widget-editor-footer-actions {
  display: flex;
  gap: 12px;
}

.widget-editor-delete-btn {
  padding: 10px 16px;
  border-radius: 8px;
  border: 1px solid var(--error);
  background: transparent;
  color: var(--error);
  font-weight: 600;
  cursor: pointer;
}

.widget-editor-delete-btn:hover {
  background: rgba(239, 68, 68, 0.1);
}

.widget-editor-cancel-btn {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
}

.widget-editor-cancel-btn:hover {
  background: var(--hover);
}

.widget-editor-save-btn {
  background: var(--primary);
  color: white;
}

.widget-editor-save-btn:hover {
  background: var(--primary-hover);
}

/* Responsive adjustments for modal */
@media (max-width: 768px) {
  .label-editor-fields {
    grid-template-columns: 1fr 1fr;
    gap: 12px;
  }
}

@media (max-width: 640px) {
  .widget-editor-footer {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
  
  .widget-editor-footer-actions {
    justify-content: stretch;
  }
  
  .widget-editor-delete-btn,
  .widget-editor-cancel-btn,
  .widget-editor-save-btn {
    flex: 1;
  }
}
.agent-toolbar {
  display: flex;
  gap: 8px;
  padding: 12px;
  border-bottom: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.03);
  border-radius: 8px 8px 0 0;
  flex-wrap: wrap;
}

.toolbar-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.toolbar-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text);
}

</style>