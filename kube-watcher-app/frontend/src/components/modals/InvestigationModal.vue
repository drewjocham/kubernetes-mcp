<template>
  <div v-show="show" class="investigation-modal-overlay" @click="emit('close')">
    <div class="investigation-modal" @click.stop>
      <div class="investigation-modal-header">
        <div class="header-left">
          <h3>AI Investigation Session</h3>
          <p class="subtitle">Interactive analysis of anomalies with topic-focused assistance</p>
        </div>
        <button class="modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="investigation-modal-content">
        <!-- Left: Chat Session -->
        <div class="chat-section">
          <div class="chat-header">
            <h4>Arguskube Investigator</h4>
            <div class="context-badge">
              <span class="badge-icon">📊</span>
              <span class="badge-text">{{ contextSummary }}</span>
            </div>
          </div>
          
          <div class="chat-messages" ref="chatMessagesRef">
            <div v-for="(message, index) in chatMessages" :key="index" :class="['chat-message', message.role]">
              <div class="message-avatar">
                <span v-if="message.role === 'assistant'">🤖</span>
                <span v-else>👤</span>
              </div>
              <div class="message-content">
                <div class="message-text" v-html="renderMarkdown(message.content)"></div>
                <div class="message-time">{{ formatTime(message.timestamp) }}</div>
              </div>
            </div>
            <div v-if="isThinking" class="chat-message assistant thinking">
              <div class="message-avatar">🤖</div>
              <div class="message-content">
                <div class="thinking-indicator">
                  <span class="dot"></span>
                  <span class="dot"></span>
                  <span class="dot"></span>
                </div>
              </div>
            </div>
          </div>

          <div class="chat-input-section">
            <div class="quick-prompts">
              <button
                v-for="prompt in quickPrompts"
                :key="prompt.label"
                class="quick-prompt-btn"
                @click="sendQuickPrompt(prompt.prompt)"
                :title="prompt.description"
              >
                {{ prompt.icon }} {{ prompt.label }}
              </button>
            </div>
            <div class="input-wrapper">
              <textarea
                v-model="userInput"
                placeholder="Ask follow-up questions or request specific analysis..."
                @keydown.enter.exact.prevent="() => sendMessage()"
                @keydown.enter.shift.exact="userInput += '\n'"
                rows="3"
                class="chat-textarea"
              ></textarea>
              <button
                @click="() => sendMessage()"
                :disabled="!userInput.trim() || isThinking"
                class="send-btn"
              >
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M22 2L11 13M22 2L15 22L11 13M22 2L2 9L11 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
                </svg>
              </button>
            </div>
          </div>
        </div>

        <!-- Right: Topic Selection -->
        <div class="topics-section">
          <div class="topics-header">
            <h4>Focus Areas</h4>
            <p class="topics-subtitle">Select a topic for targeted assistance</p>
          </div>
          
          <div class="topics-grid">
            <button
              v-for="topic in topics"
              :key="topic.id"
              :class="['topic-card', { active: selectedTopic === topic.id }]"
              @click="selectTopic(topic.id)"
            >
              <div class="topic-icon">{{ topic.icon }}</div>
              <div class="topic-content">
                <h5 class="topic-title">{{ topic.title }}</h5>
                <p class="topic-description">{{ topic.description }}</p>
                <div class="topic-actions">
                  <span class="topic-actions-label">Suggested actions:</span>
                  <div class="topic-action-tags">
                    <span v-for="action in topic.suggestedActions" :key="action" class="action-tag">{{ action }}</span>
                  </div>
                </div>
              </div>
            </button>
          </div>

          <div class="context-panel">
            <h5>Investigation Context</h5>
            <div class="context-stats">
              <div class="context-stat">
                <span class="stat-label">Alerts</span>
                <span class="stat-value">{{ context.alertsCount }}</span>
              </div>
              <div class="context-stat">
                <span class="stat-label">Critical</span>
                <span class="stat-value critical">{{ context.criticalCount }}</span>
              </div>
              <div class="context-stat">
                <span class="stat-label">Recs</span>
                <span class="stat-value">{{ context.recommendationsCount }}</span>
              </div>
              <div class="context-stat">
                <span class="stat-label">Incidents</span>
                <span class="stat-value">{{ context.incidentsCount }}</span>
              </div>
            </div>
            <div class="context-summary">
              <p>{{ context.summary }}</p>
            </div>
            <button class="refresh-context-btn" @click="refreshContext" :disabled="isRefreshing">
              {{ isRefreshing ? 'Refreshing...' : '🔄 Refresh Context' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { data } from '../../../wailsjs/go/models'
import { AskAI } from '../../../wailsjs/go/main/App'
import { renderMarkdown } from '../../utils'

interface Props {
  context: {
    alerts: data.AlertRecord[]
    recommendations: data.Recommendation[]
    timeline: data.Incident[]
  }
  show: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  refreshContext: []
}>()

// Chat state
const userInput = ref('')
const chatMessages = ref<Array<{
  role: 'assistant' | 'user'
  content: string
  timestamp: Date
}>>([])
const isThinking = ref(false)
const isRefreshing = ref(false)
const chatMessagesRef = ref<HTMLDivElement | null>(null)
const hasSentInitialAnalysis = ref(false)

// Topic selection
const selectedTopic = ref<string>('')
const topics = ref([
  {
    id: 'root-cause',
    title: 'Root Cause Analysis',
    icon: '🔍',
    description: 'Identify underlying issues causing the anomalies',
    suggestedActions: ['Trace dependencies', 'Check logs', 'Review metrics']
  },
  {
    id: 'remediation',
    title: 'Remediation Steps',
    icon: '🛠️',
    description: 'Step-by-step guidance to fix identified issues',
    suggestedActions: ['Apply fixes', 'Rollback', 'Scale resources']
  },
  {
    id: 'prevention',
    title: 'Prevention Strategies',
    icon: '🛡️',
    description: 'Prevent recurrence with improved configurations',
    suggestedActions: ['Update policies', 'Add monitoring', 'Improve tests']
  },
  {
    id: 'impact',
    title: 'Impact Assessment',
    icon: '📊',
    description: 'Evaluate blast radius and service dependencies',
    suggestedActions: ['Map dependencies', 'Assess risk', 'Prioritize fixes']
  },
  {
    id: 'resources',
    title: 'Resource Optimization',
    icon: '⚡',
    description: 'Optimize resource usage and performance',
    suggestedActions: ['Adjust limits', 'Right-size', 'Balance load']
  },
  {
    id: 'monitoring',
    title: 'Monitoring Gaps',
    icon: '👁️',
    description: 'Identify missing observability and alerting',
    suggestedActions: ['Add alerts', 'Improve dashboards', 'Set SLOs']
  }
])

// Quick prompts for common questions
const quickPrompts = ref([
  {
    label: 'Summarize',
    icon: '📋',
    prompt: 'Provide a concise summary of the current anomalies and their impact.',
    description: 'Get a high-level overview'
  },
  {
    label: 'Prioritize',
    icon: '🎯',
    prompt: 'What should I fix first and why?',
    description: 'Get prioritized remediation steps'
  },
  {
    label: 'Root Cause',
    icon: '🔍',
    prompt: 'What is the most likely root cause of these anomalies?',
    description: 'Identify underlying issues'
  },
  {
    label: 'Action Plan',
    icon: '📝',
    prompt: 'Create a step-by-step action plan to resolve these issues.',
    description: 'Get actionable steps'
  }
])

// Computed properties
const contextSummary = computed(() => {
  const total = props.context.alerts.length
  const critical = props.context.alerts.filter(a => a.severity === 'critical').length
  return `${total} alerts (${critical} critical)`
})

const context = computed(() => ({
  alertsCount: props.context.alerts.length,
  criticalCount: props.context.alerts.filter(a => a.severity === 'critical').length,
  recommendationsCount: props.context.recommendations.length,
  incidentsCount: props.context.timeline.length,
  summary: `Investigating ${props.context.alerts.length} anomalies with ${props.context.recommendations.length} recommendations and ${props.context.timeline.length} recent incidents.`
}))

// Format context for AI prompt
const formatContext = (options?: {
  topic?: string
  previousMessages?: Array<{ role: string; content: string }>
  instruction?: string
}) => {
  const { topic, previousMessages, instruction } = options || {}
  const alerts = props.context.alerts
  const recommendations = props.context.recommendations
  const timeline = props.context.timeline
  
  const lines: string[] = []
  
  lines.push(`INVESTIGATION CONTEXT:`)
  lines.push(`======================`)
  
  // Alerts summary
  const totalAlerts = alerts.length
  const criticalAlerts = alerts.filter(a => a.severity === 'critical').length
  const warningAlerts = alerts.filter(a => a.severity === 'warning').length
  const infoAlerts = alerts.filter(a => a.severity === 'info').length
  
  lines.push(`ALERTS: ${totalAlerts} total (${criticalAlerts} critical, ${warningAlerts} warning, ${infoAlerts} info)`)
  
  if (totalAlerts > 0) {
    // List top 5 alerts by severity
    const sortedAlerts = [...alerts].sort((a, b) => {
      const severityOrder: Record<string, number> = { critical: 3, warning: 2, info: 1 }
      return (severityOrder[b.severity] || 0) - (severityOrder[a.severity] || 0)
    }).slice(0, 5)
    
    lines.push('')
    lines.push('Top alerts:')
    sortedAlerts.forEach(alert => {
      const message = alert.message ? ` - ${alert.message.substring(0, 60)}${alert.message.length > 60 ? '...' : ''}` : ''
      lines.push(`- ${alert.severity.toUpperCase()}: ${alert.kind} "${alert.name}" in ${alert.namespace || 'default'}: ${alert.reason}${message}`)
    })
  }
  
  // Recommendations
  lines.push('')
  lines.push(`RECOMMENDATIONS: ${recommendations.length} available`)
  if (recommendations.length > 0) {
    const topRecs = recommendations.slice(0, 3)
    lines.push('Top recommendations:')
    topRecs.forEach(rec => {
      const summary = rec.summary ? ` - ${rec.summary.substring(0, 80)}${rec.summary.length > 80 ? '...' : ''}` : ''
      lines.push(`- ${rec.severity.toUpperCase()}: ${rec.title}${summary}`)
    })
  }
  
  // Timeline incidents
  lines.push('')
  lines.push(`RECENT INCIDENTS: ${timeline.length} in history`)
  if (timeline.length > 0) {
    const recentIncidents = timeline.slice(0, 3)
    lines.push('Recent incidents:')
    recentIncidents.forEach(incident => {
      const reason = incident.reason ? ` - ${incident.reason.substring(0, 80)}${incident.reason.length > 80 ? '...' : ''}` : ''
      lines.push(`- ${incident.severity.toUpperCase()}: ${incident.kind} "${incident.name}" (${incident.occurrences}x)${reason}`)
    })
  }
  
  // Topic focus
  if (topic) {
    lines.push('')
    lines.push(`FOCUS AREA: ${topic}`)
  }
  
  // Previous conversation context
  if (previousMessages && previousMessages.length > 0) {
    lines.push('')
    lines.push('PREVIOUS CONVERSATION:')
    previousMessages.forEach(msg => {
      const prefix = msg.role === 'user' ? 'User' : 'Assistant'
      lines.push(`${prefix}: ${msg.content.substring(0, 100)}${msg.content.length > 100 ? '...' : ''}`)
    })
  }
  
  lines.push('')
  if (instruction) {
    lines.push(instruction)
  } else {
    lines.push('Please provide analysis and recommendations based on this context.')
  }
  
  return lines.join('\n')
}

// Functions
const scrollToBottom = () => {
  nextTick(() => {
    if (chatMessagesRef.value) {
      chatMessagesRef.value.scrollTop = chatMessagesRef.value.scrollHeight
    }
  })
}

const formatTime = (date: Date) => {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

const selectTopic = (topicId: string) => {
  selectedTopic.value = topicId
  const topic = topics.value.find(t => t.id === topicId)
  if (topic) {
    sendMessage(`Please focus on ${topic.title.toLowerCase()}: ${topic.description}`)
  }
}

const sendQuickPrompt = (prompt: string) => {
  sendMessage(prompt)
}

const sendMessage = async (customMessage?: string) => {
  const message = customMessage?.trim() || userInput.value.trim()
  if (!message || isThinking.value) return

  // Add user message
  chatMessages.value.push({
    role: 'user',
    content: message,
    timestamp: new Date()
  })
  // Only clear user input if we used it
  if (!customMessage) {
    userInput.value = ''
  }
  scrollToBottom()

  // Prepare context for AI
  const topic = selectedTopic.value ? topics.value.find(t => t.id === selectedTopic.value)?.title : undefined
  const previousMessages = chatMessages.value.slice(-5).map(m => ({ role: m.role, content: m.content }))
  const aiContext = formatContext({
    topic,
    previousMessages
  })

  isThinking.value = true
  
  try {
    const response = await AskAI(message, aiContext)
    chatMessages.value.push({
      role: 'assistant',
      content: response,
      timestamp: new Date()
    })
  } catch (error) {
    chatMessages.value.push({
      role: 'assistant',
      content: `Error: ${error instanceof Error ? error.message : String(error)}`,
      timestamp: new Date()
    })
  } finally {
    isThinking.value = false
    scrollToBottom()
  }
}

const sendInitialAnalysis = async () => {
  if (hasSentInitialAnalysis.value) return
  hasSentInitialAnalysis.value = true
  
  const initialContext = formatContext({
    instruction: 'Analyze these anomalies and provide an initial assessment. Focus on identifying patterns, critical issues, and immediate actions.'
  })

  isThinking.value = true
  
  try {
    const response = await AskAI('Analyze the current anomalies and provide an initial assessment.', initialContext)
    chatMessages.value.push({
      role: 'assistant',
      content: response,
      timestamp: new Date()
    })
  } catch (error) {
    chatMessages.value.push({
      role: 'assistant',
      content: `I'm ready to help investigate the ${props.context.alerts.length} anomalies. What would you like to focus on?`,
      timestamp: new Date()
    })
  } finally {
    isThinking.value = false
    scrollToBottom()
  }
}

const refreshContext = () => {
  isRefreshing.value = true
  emit('refreshContext')
  setTimeout(() => {
    isRefreshing.value = false
  }, 1000)
}

// Initialize with AI greeting
// Send initial analysis when modal is shown
watch(() => props.show, (newVal) => {
  if (newVal && chatMessages.value.length === 0) {
    sendInitialAnalysis()
  }
}, { immediate: true })

// Auto-scroll when new messages arrive
watch(chatMessages, () => {
  scrollToBottom()
}, { deep: true })
</script>

<style scoped>
.investigation-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1001;
  padding: 20px;
  backdrop-filter: blur(4px);
}

.investigation-modal {
  background: var(--modal-bg);
  border-radius: 24px;
  width: 100%;
  max-width: 1400px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border);
  overflow: hidden;
}

.investigation-modal-header {
  padding: 24px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  background: rgba(59, 130, 246, 0.05);
}

.header-left h3 {
  font-size: 28px;
  font-weight: 700;
  margin: 0 0 8px 0;
  background: linear-gradient(135deg, var(--primary) 0%, var(--info) 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
}

.subtitle {
  font-size: 14px;
  color: var(--text-secondary);
  margin: 0;
  opacity: 0.8;
}

.modal-close-btn {
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

.modal-close-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.investigation-modal-content {
  display: grid;
  grid-template-columns: 2fr 1fr;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* Chat Section */
.chat-section {
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border);
  background: var(--panel-bg);
}

.chat-header {
  padding: 20px;
  border-bottom: 1px solid var(--border);
  background: rgba(59, 130, 246, 0.05);
}

.chat-header h4 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 8px 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.chat-header h4::before {
  content: '🤖';
  font-size: 20px;
}

.context-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  background: rgba(59, 130, 246, 0.1);
  border-radius: 12px;
  font-size: 12px;
  font-weight: 500;
}

.badge-icon {
  font-size: 14px;
}

.chat-messages {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.chat-message {
  display: flex;
  gap: 12px;
  max-width: 85%;
}

.chat-message.user {
  align-self: flex-end;
  flex-direction: row-reverse;
}

.chat-message.assistant {
  align-self: flex-start;
}

.message-avatar {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  background: rgba(59, 130, 246, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  flex-shrink: 0;
}

.chat-message.user .message-avatar {
  background: rgba(139, 92, 246, 0.1);
}

.message-content {
  padding: 12px 16px;
  border-radius: 16px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  max-width: 100%;
}

.chat-message.user .message-content {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
}

.message-text {
  font-size: 14px;
  line-height: 1.6;
}

.message-text :deep(*) {
  margin: 0;
}

.message-text :deep(ul),
.message-text :deep(ol) {
  padding-left: 1.5em;
  margin: 0.5em 0;
}

.message-text :deep(code) {
  background: rgba(255, 255, 255, 0.1);
  padding: 2px 4px;
  border-radius: 4px;
  font-family: 'SF Mono', monospace;
  font-size: 0.9em;
}

.message-text :deep(pre) {
  background: rgba(0, 0, 0, 0.2);
  padding: 12px;
  border-radius: 8px;
  overflow-x: auto;
  margin: 0.5em 0;
}

.message-text :deep(pre code) {
  background: none;
  padding: 0;
}

.message-time {
  font-size: 11px;
  opacity: 0.6;
  margin-top: 6px;
  text-align: right;
}

.thinking-indicator {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 8px 0;
}

.dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--primary);
  animation: pulse 1.5s infinite;
}

.dot:nth-child(2) {
  animation-delay: 0.2s;
}

.dot:nth-child(3) {
  animation-delay: 0.4s;
}

@keyframes pulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 1; }
}

.chat-input-section {
  padding: 20px;
  border-top: 1px solid var(--border);
  background: var(--panel-bg);
}

.quick-prompts {
  display: flex;
  gap: 8px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.quick-prompt-btn {
  padding: 6px 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 4px;
}

.quick-prompt-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.input-wrapper {
  display: flex;
  gap: 12px;
}

.chat-textarea {
  flex: 1;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-size: 14px;
  font-family: inherit;
  resize: none;
  outline: none;
  transition: all 0.2s;
}

.chat-textarea:focus {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(59, 130, 246, 0.2);
}

.send-btn {
  padding: 12px 24px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  cursor: pointer;
  transition: background 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.send-btn:hover:not(:disabled) {
  background: var(--primary-hover);
}

.send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Topics Section */
.topics-section {
  display: flex;
  flex-direction: column;
  background: var(--panel-bg);
  overflow-y: auto;
}

.topics-header {
  padding: 20px;
  border-bottom: 1px solid var(--border);
  background: rgba(139, 92, 246, 0.05);
}

.topics-header h4 {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 4px 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.topics-header h4::before {
  content: '🎯';
  font-size: 20px;
}

.topics-subtitle {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0;
  opacity: 0.8;
}

.topics-grid {
  padding: 20px;
  display: grid;
  grid-template-columns: 1fr;
  gap: 12px;
  flex: 1;
}

.topic-card {
  padding: 16px;
  border-radius: 16px;
  border: 2px solid var(--border);
  background: var(--card-bg);
  text-align: left;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.topic-card:hover {
  border-color: var(--border-active);
  background: var(--hover);
  transform: translateY(-2px);
}

.topic-card.active {
  border-color: var(--primary);
  background: rgba(59, 130, 246, 0.1);
  box-shadow: 0 4px 12px rgba(59, 130, 246, 0.2);
}

.topic-icon {
  font-size: 24px;
  flex-shrink: 0;
}

.topic-content {
  flex: 1;
}

.topic-title {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 4px 0;
  color: var(--text-primary);
}

.topic-description {
  font-size: 12px;
  color: var(--text-secondary);
  margin: 0 0 8px 0;
  line-height: 1.4;
}

.topic-actions {
  margin-top: 8px;
}

.topic-actions-label {
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  display: block;
  margin-bottom: 4px;
}

.topic-action-tags {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.action-tag {
  padding: 2px 6px;
  border-radius: 10px;
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  font-size: 10px;
  font-weight: 500;
}

/* Context Panel */
.context-panel {
  padding: 20px;
  border-top: 1px solid var(--border);
  background: rgba(0, 0, 0, 0.05);
}

.context-panel h5 {
  font-size: 14px;
  font-weight: 600;
  margin: 0 0 12px 0;
  display: flex;
  align-items: center;
  gap: 6px;
}

.context-panel h5::before {
  content: '📋';
  font-size: 16px;
}

.context-stats {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 8px;
  margin-bottom: 16px;
}

.context-stat {
  padding: 8px;
  border-radius: 8px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  text-align: center;
}

.stat-label {
  display: block;
  font-size: 10px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  margin-bottom: 4px;
}

.stat-value {
  display: block;
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
}

.stat-value.critical {
  color: var(--error);
}

.context-summary {
  padding: 12px;
  border-radius: 8px;
  background: var(--card-bg);
  border: 1px solid var(--border);
  margin-bottom: 12px;
}

.context-summary p {
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
  margin: 0;
}

.refresh-context-btn {
  width: 100%;
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.refresh-context-btn:hover:not(:disabled) {
  background: var(--hover);
  border-color: var(--border-active);
}

.refresh-context-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Responsive */
@media (max-width: 1024px) {
  .investigation-modal-content {
    grid-template-columns: 1fr;
  }
  
  .chat-section {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }
  
  .investigation-modal {
    max-width: 800px;
  }
}

@media (max-width: 640px) {
  .investigation-modal-header {
    flex-direction: column;
    gap: 12px;
  }
  
  .chat-message {
    max-width: 95%;
  }
  
  .quick-prompts {
    flex-direction: column;
  }
  
  .input-wrapper {
    flex-direction: column;
  }
  
  .send-btn {
    width: 100%;
  }
}
</style>