<template>
  <div v-if="alert" class="alert-modal-overlay" @click="emit('close')">
    <div class="alert-modal" @click.stop v-if="alert">
      <div class="alert-modal-header">
        <h3>{{ alertTitle }}</h3>
        <button class="alert-modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="alert-modal-content">
        <!-- Basic Info -->
        <div class="alert-modal-summary">
          <span :class="['pill', alert.severity]">{{ alert.severity }}</span>
          <p class="alert-message">{{ alert.message }}</p>
          <div class="alert-meta">
            <div><strong>Kind:</strong> {{ alert.kind }}</div>
            <div><strong>Namespace:</strong> {{ alert.namespace || 'cluster-wide' }}</div>
            <div><strong>Name:</strong> {{ alert.name }}</div>
            <div><strong>Cluster:</strong> {{ alert.cluster }}</div>
            <div><strong>Reason:</strong> {{ alert.reason }}</div>
            <div><strong>Status:</strong> {{ alert.status }}</div>
            <div><strong>Received:</strong> {{ formatWhen(alert.receivedAt) }}</div>
            <div v-if="alert.confidence"><strong>Confidence:</strong> {{ alert.confidence }}%</div>
          </div>
        </div>

        <!-- Alert State -->
        <div class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M9 12L11 14L15 10M21 12C21 16.9706 16.9706 21 12 21C7.02944 21 3 16.9706 3 12C3 7.02944 7.02944 3 12 3C16.9706 3 21 7.02944 21 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            Alert State
          </h4>
          <div class="alert-section-content">
            <div class="state-management">
              <select v-model="selectedState" class="state-select">
                <option v-for="opt in stateOptions" :value="opt.value">{{ opt.label }}</option>
              </select>
              <button @click="updateState" :disabled="updatingState || !selectedState" class="small-btn">
                {{ updatingState ? 'Updating...' : 'Update State' }}
              </button>
            </div>
            <div v-if="alert.state" class="current-state">
              Current state: <strong>{{ alert.state }}</strong>
            </div>
          </div>
        </div>

        <!-- Comments -->
        <div class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M21 15C21 15.5304 20.7893 16.0391 20.4142 16.4142C20.0391 16.7893 19.5304 17 19 17H7L3 21V5C3 4.46957 3.21071 3.96086 3.58579 3.58579C3.96086 3.21071 4.46957 3 5 3H19C19.5304 3 20.0391 3.21071 20.4142 3.58579C20.7893 3.96086 21 4.46957 21 5V15Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            Comments
          </h4>
          <div class="alert-section-content">
            <div v-if="alert.comments && alert.comments.length > 0" class="comments-list">
              <div v-for="comment in alert.comments" :key="comment.id" class="comment-item">
                <div class="comment-header">
                  <span class="comment-author">{{ comment.author }}</span>
                  <span class="comment-date">{{ formatWhen(comment.created_at) }}</span>
                </div>
                <p class="comment-content">{{ comment.content }}</p>
              </div>
            </div>
            <div v-else class="no-comments">
              No comments yet.
            </div>
            <div class="add-comment">
              <input v-model="newCommentAuthor" placeholder="Your name" class="comment-author-input" />
              <textarea v-model="newCommentContent" placeholder="Add a comment..." rows="3" class="comment-textarea"></textarea>
              <button @click="addComment" :disabled="addingComment || !newCommentContent.trim()" class="small-btn">
                {{ addingComment ? 'Adding...' : 'Add Comment' }}
              </button>
            </div>
          </div>
        </div>

        <!-- Root Cause -->
        <div v-if="alert.rootCause" class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 8V12M12 16H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            Root Cause
          </h4>
          <div class="alert-section-content">{{ alert.rootCause }}</div>
        </div>

        <!-- Summary -->
        <div v-if="alert.summary" class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 8V12M12 16H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            Summary
          </h4>
          <div class="alert-section-content">{{ alert.summary }}</div>
        </div>

        <!-- Actions (Commands) -->
        <div v-if="alert.actions && alert.actions.length > 0" class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M6 13L12 19L18 13M6 5L12 11L18 5" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            Recommended Actions
          </h4>
          <div class="alert-actions">
            <CommandBlock
              v-for="action in alert.actions"
              :key="action"
              :command="action"
              class="alert-command"
            />
          </div>
        </div>

        <!-- Logs & Diagnostics -->
        <div v-if="isPod" class="alert-section">
          <h4 class="alert-section-header">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M9 12L11 14L15 10M21 12C21 16.9706 16.9706 21 12 21C7.02944 21 3 16.9706 3 12C3 7.02944 7.02944 3 12 3C16.9706 3 21 7.02944 21 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            Logs & Diagnostics
          </h4>
          <div class="alert-section-content">
            <!-- Logs viewer with colored border -->
            <div :class="['logs-viewer', statusBorderColor]">
              <div class="logs-header">
                <span>Pod Logs</span>
                <button class="small-btn" @click="fetchLogs" :disabled="logsLoading">
                  {{ logsLoading ? 'Fetching...' : 'View Logs' }}
                </button>
              </div>
              <div v-if="logsError" class="logs-error">{{ logsError }}</div>
              <pre v-if="logsContent" class="logs-content">{{ logsContent }}</pre>
              <div v-else class="logs-placeholder">Click "View Logs" to fetch pod logs</div>
            </div>
            
            <!-- YAML and Describe buttons -->
            <div class="diagnostic-buttons">
              <button class="small-btn" @click="fetchYAML" :disabled="yamlLoading">
                {{ yamlLoading ? 'Fetching...' : 'Pod YAML' }}
              </button>
              <button class="small-btn" @click="fetchDescribe" :disabled="describeLoading">
                {{ describeLoading ? 'Fetching...' : 'Describe' }}
              </button>
              <button class="small-btn" @click="fetchAIHelp" :disabled="aiHelpLoading">
                {{ aiHelpLoading ? 'Fetching...' : 'AI Help' }}
              </button>
            </div>
          </div>
        </div>

        <!-- AI Investigation -->
        <div class="alert-modal-footer">
          <button class="investigate-btn" @click="handleInvestigate">
            Investigate with AI
          </button>
        </div>
      </div>
    </div>

    <!-- Text Modals for logs, YAML, describe, AI help -->
    <TextModal
      v-if="showLogsModal"
      title="Pod Logs"
      :content="logsContent"
      :is-pre="true"
      @close="showLogsModal = false"
    />
    <TextModal
      v-if="showYAMLModal"
      title="Pod YAML"
      :content="yamlContent"
      :is-pre="true"
      @close="showYAMLModal = false"
    />
    <TextModal
      v-if="showDescribeModal"
      title="Pod Describe"
      :content="describeContent"
      :is-pre="true"
      @close="showDescribeModal = false"
    />
    <TextModal
      v-if="showAIHelpModal"
      title="AI Help"
      :content="aiHelpContent"
      :is-pre="false"
      @close="showAIHelpModal = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import CommandBlock from '../CommandBlock.vue'
import TextModal from './TextModal.vue'
import { data } from '../../../wailsjs/go/models'
import { GetPodLogs, GetPodYAML, DescribePod, GetPodAIHelp, UpdateAlertState, AddAlertComment } from '../../../wailsjs/go/main/App'

const stateOptions = [
  { value: '', label: '— Select state —' },
  { value: 'new', label: 'New' },
  { value: 'acknowledged', label: 'Acknowledged' },
  { value: 'silenced', label: 'Silenced' },
  { value: 'being_investigated', label: 'Being Investigated' },
  { value: 'false_positive', label: 'False Positive' },
  { value: 'deleted', label: 'Deleted' }
]

interface Props {
  alert: data.AlertRecord | null
  formatWhen: (value: unknown) => string
}

const props = defineProps<Props>()
// State for logs and diagnostics
const logsContent = ref('')
const logsLoading = ref(false)
const logsError = ref('')
const showLogsModal = ref(false)
const showYAMLModal = ref(false)
const yamlContent = ref('')
const yamlLoading = ref(false)
const showDescribeModal = ref(false)
const describeContent = ref('')
const describeLoading = ref(false)
const showAIHelpModal = ref(false)
const aiHelpContent = ref('')
const aiHelpLoading = ref(false)

// State for alert management
const selectedState = ref('')
const newCommentAuthor = ref('user')
const newCommentContent = ref('')
const updatingState = ref(false)
const addingComment = ref(false)

const emit = defineEmits<{
  close: []
  investigate: [alert: data.AlertRecord]
}>()

// Reset state when alert changes
watch(() => props.alert, () => {
  logsContent.value = ''
  logsError.value = ''
  logsLoading.value = false
  yamlContent.value = ''
  yamlLoading.value = false
  describeContent.value = ''
  describeLoading.value = false
  aiHelpContent.value = ''
  aiHelpLoading.value = false
  showLogsModal.value = false
  showYAMLModal.value = false
  showDescribeModal.value = false
  showAIHelpModal.value = false
  selectedState.value = props.alert?.state || ''
  newCommentAuthor.value = 'user'
  newCommentContent.value = ''
})

const alertTitle = computed(() => {
  if (!props.alert) return ''
  return `${props.alert.kind}: ${props.alert.name}`
})

const isPod = computed(() => props.alert?.kind?.toLowerCase() === 'pod')

const statusBorderColor = computed(() => {
  if (!props.alert) return 'border-gray'
  const status = props.alert.status?.toLowerCase()
  if (status.includes('running') || status.includes('ready')) {
    return 'border-green'
  } else {
    return 'border-gray'
  }
})

const handleInvestigate = () => {
  if (props.alert) {
    emit('investigate', props.alert)
  }
}

async function fetchLogs() {
  if (!props.alert || !isPod.value) return
  if (!props.alert.namespace || !props.alert.name) {
    logsError.value = 'Pod namespace or name missing'
    return
  }
  logsLoading.value = true
  logsError.value = ''
  try {
    logsContent.value = await GetPodLogs(props.alert.namespace, props.alert.name, '')
    showLogsModal.value = true
  } catch (err) {
    logsError.value = 'Failed to fetch logs: ' + (err instanceof Error ? err.message : String(err))
    console.error(err)
  } finally {
    logsLoading.value = false
  }
}

async function fetchYAML() {
  if (!props.alert || !isPod.value) return
  if (!props.alert.namespace || !props.alert.name) {
    yamlContent.value = 'Pod namespace or name missing'
    showYAMLModal.value = true
    return
  }
  yamlLoading.value = true
  try {
    yamlContent.value = await GetPodYAML(props.alert.namespace, props.alert.name)
    showYAMLModal.value = true
  } catch (err) {
    yamlContent.value = 'Error: ' + (err instanceof Error ? err.message : String(err))
    showYAMLModal.value = true
  } finally {
    yamlLoading.value = false
  }
}

async function fetchDescribe() {
  if (!props.alert || !isPod.value) return
  if (!props.alert.namespace || !props.alert.name) {
    describeContent.value = 'Pod namespace or name missing'
    showDescribeModal.value = true
    return
  }
  describeLoading.value = true
  try {
    describeContent.value = await DescribePod(props.alert.namespace, props.alert.name)
    showDescribeModal.value = true
  } catch (err) {
    describeContent.value = 'Error: ' + (err instanceof Error ? err.message : String(err))
    showDescribeModal.value = true
  } finally {
    describeLoading.value = false
  }
}

async function fetchAIHelp() {
  if (!props.alert || !isPod.value) return
  if (!props.alert.namespace || !props.alert.name) {
    aiHelpContent.value = 'Pod namespace or name missing'
    showAIHelpModal.value = true
    return
  }
  aiHelpLoading.value = true
  try {
    aiHelpContent.value = await GetPodAIHelp(props.alert.namespace, props.alert.name)
    showAIHelpModal.value = true
  } catch (err) {
    aiHelpContent.value = 'Error: ' + (err instanceof Error ? err.message : String(err))
    showAIHelpModal.value = true
  } finally {
    aiHelpLoading.value = false
  }
}

async function updateState() {
  if (!props.alert || !selectedState.value) return
  updatingState.value = true
  try {
    await UpdateAlertState(props.alert.id, selectedState.value)
    // Update local alert state (optional, parent may refetch)
    if (props.alert) {
      props.alert.state = selectedState.value
    }
  } catch (err) {
    console.error('Failed to update alert state:', err)
  } finally {
    updatingState.value = false
  }
}

async function addComment() {
  if (!props.alert || !newCommentContent.value.trim()) return
  addingComment.value = true
  try {
    await AddAlertComment(props.alert.id, newCommentAuthor.value, newCommentContent.value.trim())
    // Refresh comments by refetching alerts (parent will handle)
    // For now, just clear input
    newCommentContent.value = ''
  } catch (err) {
    console.error('Failed to add comment:', err)
  } finally {
    addingComment.value = false
  }
}

// Auto-send alert context to AI when modal opens
onMounted(() => {
  if (props.alert) {
    emit('investigate', props.alert)
  }
})
</script>

<style scoped>
.alert-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
   background: rgba(0, 0, 0, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(2px);
}

.alert-modal {
  background: var(--modal-bg);
  border-radius: 24px;
  width: 100%;
  max-width: 800px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border);
}

.alert-modal-header {
  padding: 24px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.alert-modal-header h3 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.alert-modal-close-btn {
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

.alert-modal-close-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.alert-modal-content {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.alert-modal-summary {
  margin-bottom: 32px;
}

.pill {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 16px;
}

.pill.critical,
.pill.error {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
  border: 1px solid rgba(239, 68, 68, 0.2);
}

.pill.warning {
  background: rgba(245, 158, 11, 0.1);
  color: var(--warning);
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.pill.info {
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.pill.low {
  background: rgba(34, 197, 94, 0.1);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.alert-message {
  font-size: 16px;
  line-height: 1.6;
  color: var(--text-secondary);
  margin: 16px 0;
}

.alert-meta {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
  font-size: 14px;
  color: var(--text-secondary);
}

.alert-meta div {
  padding: 8px;
  background: var(--hover);
  border-radius: 8px;
}

.alert-meta strong {
  color: var(--text-primary);
  margin-right: 6px;
}

.alert-section {
  margin-bottom: 32px;
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
}

.alert-section-header {
  padding: 16px;
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  border-bottom: 1px solid rgba(59, 130, 246, 0.2);
}

.alert-section-content {
  padding: 16px;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-secondary);
  background: var(--hover);
}

.alert-actions {
  padding: 16px;
  background: var(--hover);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.alert-command {
  margin-bottom: 0;
}

.alert-modal-footer {
  padding: 24px 0 0;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
}

.investigate-btn {
  padding: 12px 24px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}

.investigate-btn:hover {
  background: var(--primary-hover);
}

.logs-viewer {
  border: 2px solid;
  border-radius: 8px;
  padding: 12px;
  margin-bottom: 16px;
  background: var(--panel-bg);
}

.logs-viewer.border-green {
  border-color: var(--success);
}

.logs-viewer.border-gray {
  border-color: var(--border);
}

.logs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
}

.logs-error {
  color: var(--error);
  font-size: 12px;
  margin-bottom: 8px;
}

.logs-content {
  font-family: monospace;
  font-size: 12px;
  white-space: pre-wrap;
  max-height: 300px;
  overflow-y: auto;
  background: rgba(0,0,0,0.2);
  padding: 8px;
  border-radius: 4px;
}

.logs-placeholder {
  color: var(--text-tertiary);
  font-style: italic;
  font-size: 14px;
  text-align: center;
  padding: 20px;
}

.diagnostic-buttons {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.small-btn {
  padding: 6px 12px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.small-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.small-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>