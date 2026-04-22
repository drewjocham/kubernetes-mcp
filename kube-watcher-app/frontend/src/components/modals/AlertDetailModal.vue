<template>
  <div v-if="alert" class="alert-modal-overlay" @click="emit('close')">
    <div class="alert-modal" @click.stop v-if="alert">
      <!-- Header -->
      <div class="alert-modal-header">
        <div class="header-left">
          <span :class="['pill', alert.severity]">{{ alert.severity }}</span>
          <h3>{{ alertTitle }}</h3>
          <p class="alert-subtitle">{{ alert.message }}</p>
        </div>
        <div class="header-actions">
          <button class="alert-modal-share-btn" @click="shareAlert" title="Share alert details">
            <PhShareNetwork size="18" weight="fill" />
          </button>
          <button class="alert-modal-close-btn" @click="emit('close')">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </button>
        </div>
      </div>

      <!-- Context Bar (shown for describable resources) -->
      <div class="context-bar" v-if="isDescribable">
        <div class="context-bar-content">
          <span class="context-item">
            <strong>{{ resourceType.toUpperCase() }}:</strong> {{ alert.name }}
          </span>
          <span class="context-item" v-if="!isNode">
            <strong>Namespace:</strong> {{ alert.namespace || 'default' }}
          </span>
          <span class="context-item" v-if="alert.status">
            <strong>Status:</strong> {{ alert.status }}
          </span>
          <span class="context-item" v-if="isPod && alert.status && alert.status.toLowerCase().includes('restart')">
            <strong>Restarts:</strong> {{ extractRestartCount(alert.status) }}
          </span>
        </div>
      </div>

      <!-- Tab Navigation -->
      <div class="alert-tabs">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :class="['alert-tab', { active: activeTab === tab.id }]"
          @click="activeTab = tab.id"
        >
          <component :is="tab.icon" size="18" weight="fill" />
          {{ tab.label }}
        </button>
      </div>

      <!-- Tab Content -->
      <div class="alert-tab-content">
        <!-- Logs View -->
        <div v-if="activeTab === 'logs'" class="tab-pane logs-view">
          <div class="logs-view-header">
            <div class="logs-controls">
              <!-- iOS-style segmented filter -->
              <div class="segmented-control">
                <button
                  v-for="filter in [
                    { id: 'all', label: 'All' },
                    { id: 'errors', label: 'Errors Only' },
                    { id: 'warnings', label: 'Warnings Only' },
                    { id: 'info', label: 'Info' }
                  ]"
                  :key="filter.id"
                  :class="['segment', { active: logFilterType === filter.id }]"
                  @click="logFilterType = filter.id as 'all' | 'errors' | 'warnings' | 'info'"
                >
                  {{ filter.label }}
                </button>
              </div>
              
              <!-- Search/grep bar -->
              <div class="logs-search">
                <PhMagnifyingGlass size="16" weight="bold" class="search-icon" />
                <input
                  type="text"
                  v-model="grepFilter"
                  placeholder="Filter logs (grep)..."
                  class="logs-search-input"
                />
                <button
                  v-if="grepFilter"
                  @click="grepFilter = ''"
                  class="clear-search-btn"
                  title="Clear filter"
                >
                  <PhXCircle size="16" weight="fill" />
                </button>
              </div>

              <!-- Live streaming toggle -->
              <button
                :class="['streaming-toggle-btn', { active: streamingEnabled }]"
                @click="toggleStreaming"
                :title="streamingEnabled ? 'Stop live streaming' : 'Start live streaming'"
              >
                <span class="toggle-indicator"></span>
                <span class="toggle-label">{{ streamingEnabled ? 'Live' : 'Paused' }}</span>
              </button>

              <!-- Fetch logs button -->
              <button
                @click="fetchLogs"
                :disabled="logsLoading || !isPod"
                class="fetch-logs-btn"
                :title="!isPod ? 'Logs only available for pods' : ''"
              >
                <PhArrowClockwise v-if="logsLoading" size="16" weight="bold" class="spinning" />
                <span v-else>{{ !isPod ? 'Not a Pod' : (logsContent ? 'Refresh Logs' : 'View Logs') }}</span>
              </button>
            </div>
          </div>

          <!-- Logs display -->
          <div class="logs-display-container">
            <div v-if="logsError" class="logs-error">{{ logsError }}</div>
            <div v-if="logsContent" class="logs-display">
              <div class="logs-display-wrapper">
                <pre class="logs-content">{{ filteredLogs }}</pre>
                <button 
                  v-if="filteredLogs"
                  @click="copyLogsToClipboard"
                  class="logs-copy-btn"
                  title="Copy logs to clipboard"
                >
                  <PhClipboard size="16" weight="bold" />
                </button>
              </div>
              <div v-if="filteredLogs === ''" class="logs-placeholder">
                No logs match the current filter.
              </div>
            </div>
             <div v-else class="logs-placeholder">
               <PhScroll size="48" weight="light" class="placeholder-icon" />
               <p v-if="isPod">Click "View Logs" to fetch pod logs</p>
               <p v-else>Logs are only available for pod alerts</p>
               <p class="placeholder-sub" v-if="isPod">Logs will appear here with line numbers and timestamps</p>
             </div>
          </div>
        </div>

        <!-- Describe View -->
        <div v-if="activeTab === 'describe'" class="tab-pane describe-view">
          <div class="describe-header">
            <div class="describe-actions">
              <button
                class="describe-menu-btn"
                @click.stop="showDescribeMenu = !showDescribeMenu"
                :disabled="describeLoading || yamlLoading || !isDescribable"
                :title="!isDescribable ? 'Describe only available for pods, services, and nodes' : ''"
              >
                <PhDotsThree size="20" weight="bold" />
              </button>
              <div v-if="showDescribeMenu" class="describe-menu">
                 <button
                   @click="fetchDescribe(); showDescribeMenu = false"
                   :disabled="describeLoading || !isDescribable"
                   class="describe-menu-item"
                   :title="!isDescribable ? 'Describe only available for pods, services, and nodes' : ''"
                 >
                   <PhArrowClockwise v-if="describeLoading" size="16" weight="bold" class="spinning" />
                   <span v-else>{{ !isDescribable ? 'Not Describable' : (describeContent ? 'Refresh Describe' : 'Fetch Describe') }}</span>
                 </button>
                 <button
                   @click="fetchYAML(); showDescribeMenu = false"
                   :disabled="yamlLoading || !isPod"
                   class="describe-menu-item"
                   :title="!isPod ? 'YAML only available for pods' : ''"
                 >
                   <PhFileText size="16" weight="bold" />
                   <span>{{ !isPod ? 'Not a Pod' : (yamlLoading ? 'Fetching...' : 'Pod YAML') }}</span>
                 </button>
                <button
                  v-if="describeContent"
                  @click="showRawDescribe = !showRawDescribe; showDescribeMenu = false"
                  class="describe-menu-item"
                >
                  <PhList size="16" weight="bold" />
                  <span>{{ showRawDescribe ? 'View Structured' : 'View Raw Output' }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="describe-content">
            <!-- Structured view (when not showing raw) -->
            <div v-if="describeContent && !showRawDescribe" class="structured-describe">
              <!-- Status Cards -->
              <div class="status-cards">
                <div class="status-card" :class="getStatusClass(alert.status)">
                  <div class="status-card-header">
                    <PhPlayCircle size="24" weight="fill" />
                    <h4>STATUS</h4>
                  </div>
                  <div class="status-card-content">
                    <div class="status-value">{{ alert.status || 'Unknown' }}</div>
                    <div class="status-detail" v-if="alert.status && alert.status.toLowerCase().includes('running')">
                      Uptime: {{ extractUptime(alert.status) }}
                    </div>
                  </div>
                </div>

                <div class="status-card">
                  <div class="status-card-header">
                    <PhWarning size="24" weight="fill" />
                    <h4>EVENTS</h4>
                  </div>
                  <div class="status-card-content">
                    <div class="events-summary" v-if="extractEventsCount(describeContent)">
                      {{ extractEventsCount(describeContent) }} Events
                    </div>
                    <div class="events-detail" v-if="extractEventsSummary(describeContent)">
                      {{ extractEventsSummary(describeContent) }}
                    </div>
                    <div v-else class="events-placeholder">
                      No recent events
                    </div>
                  </div>
                </div>

                <div class="status-card">
                  <div class="status-card-header">
                    <PhGear size="24" weight="fill" />
                    <h4>VOLUMES</h4>
                  </div>
                  <div class="status-card-content">
                    <div class="volumes-list" v-if="extractVolumes(describeContent).length > 0">
                      <div v-for="volume in extractVolumes(describeContent).slice(0, 3)" :key="volume" class="volume-item">
                        {{ volume }}
                      </div>
                    </div>
                    <div v-else class="volumes-placeholder">
                      No volumes mounted
                    </div>
                  </div>
                </div>
              </div>

              <!-- Key/Value Grids -->
              <div class="kv-grids">
                <!-- Containers -->
                <div class="kv-card" v-if="extractContainers(describeContent).length > 0">
                  <h5 class="kv-card-header">
                    <PhList size="18" weight="bold" />
                    Containers
                  </h5>
                  <div class="kv-card-content">
                    <div v-for="container in extractContainers(describeContent)" :key="container.name" class="container-item">
                      <div class="container-header">
                        <strong>{{ container.name }}</strong>
                        <span :class="['container-state', container.state]">{{ container.state }}</span>
                      </div>
                      <div class="container-details">
                        <div><strong>Image:</strong> {{ container.image }}</div>
                        <div v-if="container.reason"><strong>Reason:</strong> {{ container.reason }}</div>
                        <div v-if="container.exitCode !== undefined"><strong>Exit Code:</strong> {{ container.exitCode }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Conditions -->
                <div class="kv-card" v-if="extractConditions(describeContent).length > 0">
                  <h5 class="kv-card-header">
                    <PhCheckCircle size="18" weight="bold" />
                    Conditions
                  </h5>
                  <div class="kv-card-content">
                    <div v-for="condition in extractConditions(describeContent)" :key="condition.type" class="condition-item">
                      <div class="condition-header">
                        <strong>{{ condition.type }}</strong>
                        <span :class="['condition-status', condition.status.toLowerCase()]">
                          {{ condition.status }}
                        </span>
                      </div>
                      <div class="condition-details">
                        <div><strong>Last Probe:</strong> {{ condition.lastProbeTime || 'N/A' }}</div>
                        <div v-if="condition.message"><strong>Message:</strong> {{ condition.message }}</div>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Labels -->
                <div class="kv-card" v-if="Object.keys(extractLabels(describeContent)).length > 0">
                  <h5 class="kv-card-header">
                    <PhTag size="18" weight="bold" />
                    Labels
                  </h5>
                  <div class="kv-card-content">
                    <div class="labels-cloud">
                      <span
                        v-for="(value, key) in extractLabels(describeContent)"
                        :key="key"
                        class="label-tag"
                      >
                        {{ key }}: {{ value }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Raw output view -->
            <div v-if="describeContent && showRawDescribe" class="raw-describe">
              <pre class="raw-content">{{ describeContent }}</pre>
            </div>

            <div v-else-if="!describeContent" class="describe-placeholder">
              <PhFileText size="48" weight="light" class="placeholder-icon" />
              <p>Click "Fetch Describe" to get pod details</p>
              <p class="placeholder-sub">Structured view will show status cards and key/value grids</p>
            </div>
          </div>
        </div>

        <!-- AI Help View -->
        <div v-if="activeTab === 'ai-help'" class="tab-pane ai-help-view">
          <div class="ai-help-header">
            <div class="ai-help-actions">
               <button
                 @click="fetchAIHelp"
                 :disabled="aiHelpLoading || !isPod"
                 class="ai-help-action-btn"
                 :title="!isPod ? 'AI Help only available for pods' : ''"
               >
                 <PhArrowClockwise v-if="aiHelpLoading" size="16" weight="bold" class="spinning" />
                 <span v-else>{{ !isPod ? 'Not a Pod' : (aiHelpContent ? 'Refresh AI Help' : 'Get AI Help') }}</span>
               </button>
            </div>
          </div>

          <div class="ai-help-content">
            <!-- Interactive diagnostic assistant -->
            <div v-if="!aiHelpContent" class="ai-help-init">
              <div class="ai-help-prompt">
                <PhRobot size="48" weight="light" class="prompt-icon" />
                <h4>How can I help?</h4>
                <p>Based on the logs and describe output, what would you like to do?</p>
                <div class="quick-actions">
                  <button class="quick-action-btn" @click="requestAIAction('common-causes')">
                    <PhMagnifyingGlass size="20" weight="fill" />
                    <span>Check common causes</span>
                  </button>
                  <button class="quick-action-btn" @click="requestAIAction('suggest-fix')">
                    <PhWrench size="20" weight="fill" />
                    <span>Suggest fix</span>
                  </button>
                  <button class="quick-action-btn" @click="requestAIAction('draft-ticket')">
                    <PhPencil size="20" weight="fill" />
                    <span>Draft ticket</span>
                  </button>
                </div>
              </div>
            </div>

            <!-- AI Analysis -->
            <div v-if="aiHelpContent" class="ai-analysis">
              <!-- AHA! Moment Card -->
              <div class="aha-moment-card" v-if="extractAhaMoment(aiHelpContent)">
                <div class="aha-moment-header">
                  <PhLightning size="24" weight="fill" />
                  <h4>AHA! MOMENT</h4>
                </div>
                <div class="aha-moment-content">
                  {{ extractAhaMoment(aiHelpContent) }}
                </div>
                <div class="aha-moment-link" v-if="extractLogReference(aiHelpContent)">
                  Linked to log line: {{ extractLogReference(aiHelpContent) }}
                </div>
              </div>

              <!-- Related Issues -->
              <div class="related-issues" v-if="extractRelatedIssues(aiHelpContent).length > 0">
                <h5>RELATED ISSUES</h5>
                <div class="issues-list">
                  <div v-for="issue in extractRelatedIssues(aiHelpContent)" :key="issue" class="issue-item">
                    {{ issue }}
                  </div>
                </div>
              </div>

              <!-- Actionable Next Steps -->
              <div class="action-steps">
                <h5>ACTIONABLE NEXT STEPS</h5>
                <div class="action-buttons">
                  <button class="action-btn primary">
                    <PhPlayCircle size="20" weight="fill" />
                    <span>Restart Pod</span>
                  </button>
                  <button class="action-btn">
                    <PhWrench size="20" weight="fill" />
                    <span>Apply suggested patch</span>
                  </button>
                  <button class="action-btn">
                    <PhPencil size="20" weight="fill" />
                    <span>Draft Jira Ticket</span>
                  </button>
                </div>
              </div>

              <!-- Full AI Analysis -->
              <div class="full-analysis">
                <h5>FULL ANALYSIS</h5>
                <div class="analysis-content" v-html="formatAIHelp(aiHelpContent)"></div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer with alert management -->
      <div class="alert-modal-footer">
        <div class="footer-left">
          <select v-model="selectedState" class="state-select">
            <option v-for="opt in stateOptions" :value="opt.value">{{ opt.label }}</option>
          </select>
          <button @click="updateState" :disabled="updatingState || !selectedState" class="small-btn">
            {{ updatingState ? 'Updating...' : 'Update State' }}
          </button>
        </div>
        <div class="footer-right">
           <button class="investigate-btn" @click="handleInvestigate">
             <PhMagicWand size="18" weight="fill" />
             <span>Investigate with AI</span>
           </button>
        </div>
      </div>
    </div>

    <!-- Text Modals for logs, YAML, describe, AI help (fallback) -->
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

    <!-- Notification Components -->
    <ToastNotification
      :show="showToast"
      :message="toastMessage"
      :type="toastType"
      @close="showToast = false"
    />
    <ErrorModal
      :show="showErrorModal"
      :summary="errorDetails.summary"
      :error="errorDetails.error"
      :context="errorDetails.context"
      @update:show="showErrorModal = $event"
      @close="showErrorModal = false"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, type Component } from 'vue'
import CommandBlock from '../CommandBlock.vue'
import TextModal from './TextModal.vue'
import ToastNotification from '../ToastNotification.vue'
import ErrorModal from './ErrorModal.vue'
import { data } from '../../../wailsjs/go/models'
import { GetPodLogs, GetPodYAML, DescribePod, DescribeService, DescribeNode, GetPodAIHelp, UpdateAlertState, AddAlertComment, GetPods, GetNamespaces } from '../../../wailsjs/go/main/App'
import {
  PhScroll,
  PhFileText,
  PhRobot,
  PhMagnifyingGlass,
  PhWrench,
  PhShield,
  PhChartBar,
  PhLightning,
  PhEye,
  PhClipboard,
  PhTarget,
  PhPencil,
  PhUser,
  PhArrowClockwise,
  PhCaretRight,
  PhCaretLeft,
  PhWarning,
  PhInfo,
  PhCheckCircle,
  PhXCircle,
  PhPlayCircle,
  PhStopCircle,
  PhList,
  PhGridFour,
  PhTag,
  PhGear,
  PhClock,
  PhDotsThree,
  PhShareNetwork,
  PhMagicWand
} from '@phosphor-icons/vue'

// Helper function to extract deployment/service name from pod name
// Pod names from deployments follow pattern: <deployment-name>-<random-hash>
// Returns the prefix without the hash suffix
function extractDeploymentName(podName: string): string {
  // Remove everything after last dash if it looks like a hash (alphanumeric, 5-10 chars)
  const parts = podName.split('-')
  if (parts.length > 1) {
    const lastPart = parts[parts.length - 1]
    // Check if last part looks like a pod hash (e.g., "abc12", "xyz1234")
    const hashRegex = /^[a-z0-9]{5,10}$/
    if (hashRegex.test(lastPart)) {
      return parts.slice(0, -1).join('-')
    }
  }
  return podName
}

// Helper function to find similar pods when a pod is not found
async function findSimilarPods(namespace: string, podName: string): Promise<string> {
  try {
    const pods = await GetPods(namespace)
    const deploymentPrefix = extractDeploymentName(podName)
    const similarPods = pods.filter(pod => 
      pod.name.includes(deploymentPrefix) || extractDeploymentName(pod.name) === deploymentPrefix
    )
    if (similarPods.length > 0) {
      const podNames = similarPods.map(p => p.name).slice(0, 5)
      return `Pod not found. Similar pods in namespace ${namespace}: ${podNames.join(', ')}`
    } else {
      // Try default namespace fallback
      if (namespace !== 'default' && namespace !== 'kube-watcher') {
        try {
          const defaultPods = await GetPods('kube-watcher')
          const similarDefault = defaultPods.filter(pod => 
            pod.name.includes(deploymentPrefix) || extractDeploymentName(pod.name) === deploymentPrefix
          )
          if (similarDefault.length > 0) {
            const podNames = similarDefault.map(p => p.name).slice(0, 5)
            return `Pod not found in namespace ${namespace}. Found similar pods in namespace kube-watcher: ${podNames.join(', ')}`
          }
        } catch {
          // ignore
        }
      }
      return `Pod not found in namespace ${namespace}. No similar pods found.`
    }
  } catch (err) {
    return `Pod not found. Unable to search for similar pods: ${err instanceof Error ? err.message : String(err)}`
  }
}

type AlertDetailTab = 'logs' | 'describe' | 'ai-help'
const activeTab = ref<AlertDetailTab>('logs')

const tabs = [
  { id: 'logs' as AlertDetailTab, label: 'View Logs', icon: PhScroll },
  { id: 'describe' as AlertDetailTab, label: 'Describe', icon: PhFileText },
  { id: 'ai-help' as AlertDetailTab, label: 'AI Help', icon: PhRobot }
]

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
const streamingEnabled = ref(false)
const grepFilter = ref('')
const logFilterType = ref<'all' | 'errors' | 'warnings' | 'info'>('all')
const logsIntervalId = ref<number | null>(null)
const showYAMLModal = ref(false)
const yamlContent = ref('')
const yamlLoading = ref(false)
const showDescribeModal = ref(false)
const describeContent = ref('')
const describeLoading = ref(false)
const showRawDescribe = ref(false)
const showDescribeMenu = ref(false)
const showAIHelpModal = ref(false)
const aiHelpContent = ref('')
const aiHelpLoading = ref(false)

// State for alert management
const selectedState = ref('')
const newCommentAuthor = ref('user')
const newCommentContent = ref('')
const updatingState = ref(false)
const addingComment = ref(false)

// State for notifications
const showToast = ref(false)
const toastMessage = ref('')
const toastType = ref<'success' | 'error' | 'info'>('success')
const showErrorModal = ref(false)
const errorDetails = ref({
  summary: '',
  error: '',
  context: null as any
})

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
  showRawDescribe.value = false
  aiHelpContent.value = ''
  aiHelpLoading.value = false
  showLogsModal.value = false
  showYAMLModal.value = false
  showDescribeModal.value = false
  showAIHelpModal.value = false
  // Stop streaming if active
  if (logsIntervalId.value) {
    clearInterval(logsIntervalId.value)
    logsIntervalId.value = null
  }
  streamingEnabled.value = false
  grepFilter.value = ''
  logFilterType.value = 'all'
  selectedState.value = props.alert?.state || ''
  newCommentAuthor.value = 'user'
  newCommentContent.value = ''
})

const alertTitle = computed(() => {
  if (!props.alert) return ''
  return `${props.alert.kind}: ${props.alert.name}`
})

const resourceType = computed(() => props.alert?.kind?.toLowerCase() || '')
const isPod = computed(() => resourceType.value === 'pod')
const isService = computed(() => resourceType.value === 'service')
const isNode = computed(() => resourceType.value === 'node')
const isDescribable = computed(() => isPod.value || isService.value || isNode.value)

const statusBorderColor = computed(() => {
  if (!props.alert) return 'border-gray'
  const status = props.alert.status?.toLowerCase()
  if (status.includes('running') || status.includes('ready')) {
    return 'border-green'
  } else {
    return 'border-gray'
  }
})

const filteredLogs = computed(() => {
  if (!logsContent.value) return ''
  let lines = logsContent.value.split('\n')
  
  // Apply log level filter
  if (logFilterType.value !== 'all') {
    lines = lines.filter(line => {
      const lowerLine = line.toLowerCase()
      switch (logFilterType.value) {
        case 'errors':
          return lowerLine.includes('error') || lowerLine.includes('exception') || lowerLine.includes('fail') || lowerLine.includes('fatal')
        case 'warnings':
          return lowerLine.includes('warn') || lowerLine.includes('warning')
        case 'info':
          return lowerLine.includes('info') || (!lowerLine.includes('error') && !lowerLine.includes('warn') && !lowerLine.includes('warning'))
        default:
          return true
      }
    })
  }
  
  // Apply grep filter
  if (grepFilter.value) {
    lines = lines.filter(line => line.includes(grepFilter.value))
  }
  
  return lines.join('\n')
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
  } catch (err) {
    logsError.value = 'Failed to fetch logs: ' + (err instanceof Error ? err.message : String(err))
    console.error(err)
  } finally {
    logsLoading.value = false
  }
}

function startStreaming() {
  if (logsIntervalId.value) {
    clearInterval(logsIntervalId.value)
  }
  // Fetch logs immediately
  fetchLogs()
  // Then set up interval every 5 seconds
  logsIntervalId.value = setInterval(fetchLogs, 5000)
}

function stopStreaming() {
  if (logsIntervalId.value) {
    clearInterval(logsIntervalId.value)
    logsIntervalId.value = null
  }
}

function toggleStreaming() {
  if (streamingEnabled.value) {
    stopStreaming()
  } else {
    startStreaming()
  }
  streamingEnabled.value = !streamingEnabled.value
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
    const errorMsg = err instanceof Error ? err.message : String(err)
    yamlContent.value = `Error: ${errorMsg}`
    // Check if pod not found
    if (errorMsg.toLowerCase().includes('not found')) {
      const suggestion = await findSimilarPods(props.alert.namespace, props.alert.name)
      yamlContent.value += '\n\n' + suggestion
    }
    showYAMLModal.value = true
  } finally {
    yamlLoading.value = false
  }
}

async function fetchDescribe() {
  if (!props.alert || !isDescribable.value) return
  
  describeLoading.value = true
  try {
    if (isPod.value) {
      if (!props.alert.namespace || !props.alert.name) {
        describeContent.value = 'Pod namespace or name missing'
        return
      }
      
      const podName = props.alert.name
      const namespace = props.alert.namespace
      const deploymentName = extractDeploymentName(podName)
      
      // Try to describe service first (multiple possible service names)
      let serviceDescribed = false
      let serviceOutput = ''
      let serviceError = ''
      
      // Try service names in order of likelihood
      const serviceNamesToTry = [
        deploymentName, // Most likely: service matches deployment name
        podName,        // Less likely: service has same name as pod
      ]
      
      for (const serviceName of serviceNamesToTry) {
        if (serviceName === podName && serviceName === deploymentName) continue // Skip duplicate
        try {
          serviceOutput = await DescribeService(namespace, serviceName)
          serviceDescribed = true
          describeContent.value = `⚠️ Describing Service "${serviceName}" (associated with pod ${podName})\n\n` + serviceOutput
          break
        } catch (err) {
          serviceError = err instanceof Error ? err.message : String(err)
          // Continue to next candidate
        }
      }
      
      // If no service described, fall back to pod
      if (!serviceDescribed) {
        try {
          describeContent.value = await DescribePod(namespace, podName)
          describeContent.value = `⚠️ No associated service found, describing Pod "${podName}" instead\n\n` + describeContent.value
        } catch (podErr) {
          const errorMsg = podErr instanceof Error ? podErr.message : String(podErr)
          describeContent.value = `Error describing pod: ${errorMsg}`
          if (errorMsg.toLowerCase().includes('not found')) {
            const suggestion = await findSimilarPods(namespace, podName)
            describeContent.value += '\n\n' + suggestion
          }
        }
      }
    } else if (isService.value) {
      if (!props.alert.namespace || !props.alert.name) {
        describeContent.value = 'Service namespace or name missing'
        return
      }
      describeContent.value = await DescribeService(props.alert.namespace, props.alert.name)
    } else if (isNode.value) {
      if (!props.alert.name) {
        describeContent.value = 'Node name missing'
        return
      }
      describeContent.value = await DescribeNode(props.alert.name)
    } else {
      describeContent.value = 'Describe not available for this resource type'
    }
  } catch (err) {
    describeContent.value = 'Error: ' + (err instanceof Error ? err.message : String(err))
  } finally {
    describeLoading.value = false
  }
}

async function fetchAIHelp() {
  if (!props.alert || !isPod.value) return
  if (!props.alert.namespace || !props.alert.name) {
    aiHelpContent.value = 'Pod namespace or name missing'
    return
  }
  aiHelpLoading.value = true
  try {
    aiHelpContent.value = await GetPodAIHelp(props.alert.namespace, props.alert.name)
  } catch (err) {
    aiHelpContent.value = 'Error: ' + (err instanceof Error ? err.message : String(err))
  } finally {
    aiHelpLoading.value = false
  }
}

// Helper functions for notifications
function showSuccessToast(message: string) {
  toastMessage.value = message
  toastType.value = 'success'
  showToast.value = true
  setTimeout(() => {
    showToast.value = false
  }, 3000)
}

function showErrorDialog(summary: string, error: any, context?: any) {
  errorDetails.value = {
    summary,
    error: error instanceof Error ? error.message : String(error),
    context
  }
  showErrorModal.value = true
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
    showSuccessToast(`Alert state updated to ${selectedState.value}`)
  } catch (err) {
    console.error('Failed to update alert state:', err)
    showErrorDialog('Failed to update alert state', err, { alertId: props.alert?.id, state: selectedState.value })
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
    showSuccessToast('Comment added successfully')
  } catch (err) {
    console.error('Failed to add comment:', err)
    showErrorDialog('Failed to add comment', err, { alertId: props.alert?.id, author: newCommentAuthor.value })
  } finally {
    addingComment.value = false
  }
}

// Helper functions for describe parsing
function extractRestartCount(status: string): string {
  if (!status) return '0'
  const match = status.match(/(\d+)\s+restarts?/i)
  return match ? match[1] : '0'
}

function extractUptime(status: string): string {
  if (!status) return 'N/A'
  // Look for time patterns like "45m", "2h", "1d"
  const match = status.match(/(\d+[mhd])\s+uptime/i) || status.match(/uptime:\s*(\d+[mhd])/i)
  return match ? match[1] : 'N/A'
}

function getStatusClass(status: string): string {
  if (!status) return 'unknown'
  const lower = status.toLowerCase()
  if (lower.includes('running') || lower.includes('ready')) return 'running'
  if (lower.includes('pending') || lower.includes('waiting')) return 'pending'
  if (lower.includes('failed') || lower.includes('error') || lower.includes('crash')) return 'failed'
  return 'unknown'
}

function extractEventsCount(describeContent: string): string {
  if (!describeContent) return ''
  // Count lines with "Events:" section
  const lines = describeContent.split('\n')
  let inEvents = false
  let eventCount = 0
  for (const line of lines) {
    if (line.includes('Events:')) {
      inEvents = true
      continue
    }
    if (inEvents && line.trim() && !line.includes('  ')) {
      // New section starting
      break
    }
    if (inEvents && line.trim()) {
      eventCount++
    }
  }
  return eventCount > 0 ? `${eventCount} Events` : ''
}

function extractEventsSummary(describeContent: string): string {
  if (!describeContent) return ''
  const lines = describeContent.split('\n')
  let inEvents = false
  const events: string[] = []
  for (const line of lines) {
    if (line.includes('Events:')) {
      inEvents = true
      continue
    }
    if (inEvents && line.trim() && !line.includes('  ')) {
      break
    }
    if (inEvents && line.trim()) {
      events.push(line.trim())
    }
  }
  if (events.length === 0) return ''
  const normal = events.filter(e => e.toLowerCase().includes('normal')).length
  const warning = events.filter(e => e.toLowerCase().includes('warning')).length
  return `${normal} Normal, ${warning} Warning`
}

function extractVolumes(describeContent: string): string[] {
  if (!describeContent) return []
  const lines = describeContent.split('\n')
  let inVolumes = false
  const volumes: string[] = []
  for (const line of lines) {
    if (line.includes('Volumes:')) {
      inVolumes = true
      continue
    }
    if (inVolumes && line.trim() && !line.includes('  ')) {
      break
    }
    if (inVolumes && line.trim()) {
      // Extract volume name (first word before :)
      const match = line.match(/^\s*(\w+):/)
      if (match) volumes.push(match[1])
    }
  }
  return volumes
}

function extractContainers(describeContent: string): Array<{name: string, image: string, state: string, reason?: string, exitCode?: number}> {
  if (!describeContent) return []
  const containers: Array<{name: string, image: string, state: string, reason?: string, exitCode?: number}> = []
  const lines = describeContent.split('\n')
  let currentContainer: any = null
  
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    // Look for container sections
    if (line.includes('Containers:') || line.match(/^\s*(\w+):$/)) {
      // Container line like "  container-name:"
      const match = line.match(/^\s*(\w+):$/)
      if (match) {
        if (currentContainer) containers.push(currentContainer)
        currentContainer = { name: match[1], image: '', state: 'Unknown' }
        // Look for image in next few lines
        for (let j = i + 1; j < Math.min(i + 10, lines.length); j++) {
          if (lines[j].includes('Image:')) {
            currentContainer.image = lines[j].split('Image:')[1]?.trim() || ''
          }
          if (lines[j].includes('State:')) {
            currentContainer.state = lines[j].split('State:')[1]?.trim() || 'Unknown'
          }
          if (lines[j].includes('Reason:')) {
            currentContainer.reason = lines[j].split('Reason:')[1]?.trim()
          }
          if (lines[j].includes('Exit Code:')) {
            const exitMatch = lines[j].match(/Exit Code:\s*(\d+)/)
            if (exitMatch) currentContainer.exitCode = parseInt(exitMatch[1])
          }
        }
      }
    }
  }
  if (currentContainer) containers.push(currentContainer)
  return containers
}

function extractConditions(describeContent: string): Array<{type: string, status: string, lastProbeTime?: string, message?: string}> {
  if (!describeContent) return []
  const conditions: Array<{type: string, status: string, lastProbeTime?: string, message?: string}> = []
  const lines = describeContent.split('\n')
  let inConditions = false
  
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i]
    if (line.includes('Conditions:')) {
      inConditions = true
      continue
    }
    if (inConditions && line.trim() && !line.includes('  ')) {
      break
    }
    if (inConditions && line.trim()) {
      // Parse condition line like "  Type              Status"
      const parts = line.trim().split(/\s{2,}/)
      if (parts.length >= 2) {
        conditions.push({
          type: parts[0],
          status: parts[1],
          lastProbeTime: parts[2],
          message: parts[3]
        })
      }
    }
  }
  return conditions
}

function extractLabels(describeContent: string): Record<string, string> {
  if (!describeContent) return {}
  const labels: Record<string, string> = {}
  const lines = describeContent.split('\n')
  let inLabels = false
  
  for (const line of lines) {
    if (line.includes('Labels:')) {
      inLabels = true
      continue
    }
    if (inLabels && line.trim() && !line.includes('  ')) {
      break
    }
    if (inLabels && line.trim()) {
      // Parse label like "key=value"
      const parts = line.trim().split('=')
      if (parts.length >= 2) {
        labels[parts[0]] = parts.slice(1).join('=')
      }
    }
  }
  return labels
}

// AI Help parsing functions
function extractAhaMoment(aiHelpContent: string): string {
  if (!aiHelpContent) return ''
  // Look for patterns like "Root Cause:", "AHA!", "Potential issue"
  const lines = aiHelpContent.split('\n')
  for (const line of lines) {
    if (line.includes('Root Cause:') || line.includes('Potential') || line.includes('AHA!') || line.includes('aha!')) {
      return line.trim()
    }
  }
  return lines[0] || '' // Return first line as fallback
}

function extractLogReference(aiHelpContent: string): string {
  if (!aiHelpContent) return ''
  // Look for log line references
  const match = aiHelpContent.match(/line\s+(\d+)|L(\d+)/i)
  return match ? `Line ${match[1] || match[2]}` : ''
}

function extractRelatedIssues(aiHelpContent: string): string[] {
  if (!aiHelpContent) return []
  const issues: string[] = []
  const lines = aiHelpContent.split('\n')
  for (const line of lines) {
    if (line.includes('GitHub') || line.includes('issue') || line.includes('wiki') || line.includes('related')) {
      issues.push(line.trim())
    }
  }
  return issues.slice(0, 3) // Limit to 3
}

function formatAIHelp(content: string): string {
  // Simple formatting - replace markdown-like syntax
  return content
    .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
    .replace(/\*(.*?)\*/g, '<em>$1</em>')
    .replace(/\n/g, '<br>')
}

function copyLogsToClipboard() {
  if (!filteredLogs.value) return
  navigator.clipboard.writeText(filteredLogs.value)
    .then(() => {
      showSuccessToast('Logs copied to clipboard')
    })
    .catch(err => {
      console.error('Failed to copy logs:', err)
      showErrorDialog('Failed to copy logs', err)
    })
}

function shareAlert() {
  if (!props.alert) return
  const alert = props.alert as any
  const summary = `Alert: ${alert.kind}: ${alert.name}
Namespace: ${alert.namespace || 'default'}
Status: ${alert.status}
Severity: ${alert.severity}
Message: ${alert.message}
Timestamp: ${props.formatWhen(alert.created_at)}
State: ${alert.state || 'new'}

View details in Kube Watcher.`
  
  navigator.clipboard.writeText(summary)
    .then(() => {
      showSuccessToast('Alert details copied to clipboard')
    })
    .catch(err => {
      console.error('Failed to share alert:', err)
      showErrorDialog('Failed to share alert', err)
    })
}

function requestAIAction(action: 'common-causes' | 'suggest-fix' | 'draft-ticket') {
  // Trigger AI action based on selection
  if (!props.alert) return
  let prompt = ''
  switch (action) {
    case 'common-causes':
      prompt = 'Check common causes for this issue'
      break
    case 'suggest-fix':
      prompt = 'Suggest a fix for this issue'
      break
    case 'draft-ticket':
      prompt = 'Draft a Jira ticket for this issue'
      break
  }
  // For now, just fetch AI help with the prompt
  fetchAIHelp()
}

onUnmounted(() => {
  if (logsIntervalId.value) {
    clearInterval(logsIntervalId.value)
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
  background: rgba(0, 0, 0, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(40px) saturate(200%);
  -webkit-backdrop-filter: blur(40px) saturate(200%);
}

.alert-modal {
  background: color-mix(in srgb, var(--modal-bg) 75%, transparent);
  backdrop-filter: blur(60px) saturate(200%);
  -webkit-backdrop-filter: blur(60px) saturate(200%);
  border-radius: 24px;
  width: 100%;
  max-width: 800px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 
    0 30px 80px rgba(0, 0, 0, 0.4),
    0 0 0 1px rgba(255, 255, 255, 0.05),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.alert-modal-header {
  padding: 24px 32px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border-radius: 24px 24px 0 0;
}

.alert-modal-header .header-left {
  flex: 1;
}

.alert-modal-header .header-left h3 {
  font-size: 28px;
  font-weight: 800;
  margin: 8px 0 6px 0;
  letter-spacing: -0.02em;
  line-height: 1.2;
}

.alert-subtitle {
  font-size: 15px;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
  opacity: 0.8;
  font-weight: 400;
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

.header-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}

.alert-modal-share-btn {
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

.alert-modal-share-btn:hover {
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
  padding: 10px 18px;
  border-radius: 24px;
  font-size: 13px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  margin-bottom: 16px;
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  box-shadow: 
    0 4px 12px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
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
  justify-content: space-between;
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  border-bottom: 1px solid rgba(59, 130, 246, 0.2);
}

.alert-section-header .header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.streaming-toggle-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  border-radius: 20px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.streaming-toggle-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.streaming-toggle-btn.active {
  background: var(--success);
  color: white;
  border-color: var(--success);
}

.streaming-toggle-btn.active .toggle-indicator {
  background: white;
  animation: pulse 1.5s infinite;
}

.toggle-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-tertiary);
  transition: background 0.2s;
}

.toggle-label {
  font-weight: 600;
}

@keyframes pulse {
  0% { opacity: 1; }
  50% { opacity: 0.5; }
  100% { opacity: 1; }
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

.logs-header-actions {
  display: flex;
  gap: 8px;
}

.logs-controls {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  margin-bottom: 12px;
  flex-wrap: wrap;
  gap: 12px;
}

.logs-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  cursor: pointer;
}

.logs-toggle input[type="checkbox"] {
  margin: 0;
}

.logs-search {
  display: flex;
  align-items: center;
  gap: 6px;
}

.logs-search-input {
  background: var(--input-bg);
  color: var(--text-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 4px 8px;
  font-size: 12px;
  font-family: inherit;
  outline: none;
  transition: all 0.2s ease;
  flex: 1;
  min-width: 150px;
}

.logs-search-input:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
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

.state-select {
  background: var(--input-bg);
  color: var(--text-primary);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 14px;
  cursor: pointer;
  outline: none;
  transition: all 0.2s ease;
  font-family: inherit;
}

.state-select:hover {
  background: var(--surface-strong);
  border-color: rgba(224, 223, 240, 0.2);
}

.state-select:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

.comment-author-input {
  background: var(--input-bg);
  color: var(--text-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 14px;
  font-family: inherit;
  outline: none;
  transition: all 0.2s ease;
  margin-bottom: 8px;
  width: 100%;
  box-sizing: border-box;
}

.comment-author-input:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

.comment-textarea {
  background: var(--input-bg);
  color: var(--text-primary);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 8px 12px;
  font-size: 14px;
  font-family: inherit;
  outline: none;
  transition: all 0.2s ease;
  margin-bottom: 8px;
  width: 100%;
  box-sizing: border-box;
  resize: vertical;
  min-height: 80px;
}

.comment-textarea:focus {
  border-color: var(--border-active);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

/* === New Three-View Interface Styles === */
.context-bar {
  padding: 16px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: rgba(255, 255, 255, 0.03);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 500;
}

.context-bar-content {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
}

.context-item {
  padding: 8px 12px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
  transition: all 0.2s ease;
}

.context-item:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.15);
  transform: translateY(-1px);
}

.context-item strong {
  color: var(--text-primary);
  font-weight: 600;
}

.alert-tabs {
  display: flex;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  background: transparent;
  padding: 0 24px;
  backdrop-filter: blur(10px);
  -webkit-backdrop-filter: blur(10px);
}

.alert-tab {
  padding: 16px 20px;
  background: transparent;
  border: none;
  border-bottom: 3px solid transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s;
}

.alert-tab:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.06);
}

.alert-tab.active {
  color: white;
  border-bottom-color: transparent;
  background: var(--primary);
  box-shadow: 0 4px 12px rgba(138, 130, 224, 0.3);
  position: relative;
}

.alert-tab.active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 3px;
  background: rgba(255, 255, 255, 0.8);
  border-radius: 3px 3px 0 0;
}

.alert-tab-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

.tab-pane {
  display: flex;
  flex-direction: column;
  height: 100%;
  gap: 16px;
}

/* Logs View */
.logs-view-header {
  margin-bottom: 16px;
}

.logs-controls {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
}

.segmented-control {
  display: inline-flex;
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 3px;
  gap: 3px;
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 4px 12px rgba(0, 0, 0, 0.1);
}

.segment {
  padding: 10px 16px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  white-space: nowrap;
  border-radius: 10px;
  flex: 1;
  display: flex;
  justify-content: center;
  align-items: center;
  letter-spacing: -0.01em;
}

.segment:hover {
  background: rgba(255, 255, 255, 0.12);
  color: var(--text-primary);
  transform: translateY(-1px);
}

.segment.active {
  background: var(--primary);
  color: white;
  box-shadow: 
    0 4px 12px rgba(138, 130, 224, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
  font-weight: 600;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.2);
  transform: translateY(0);
}

.logs-search {
  display: flex;
  align-items: center;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 10px 14px;
  gap: 10px;
  flex: 1;
  max-width: 300px;
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  transition: all 0.2s ease;
}

.logs-search:focus-within {
  border-color: var(--primary);
  box-shadow: 0 0 0 2px rgba(138, 130, 224, 0.2);
  background: rgba(255, 255, 255, 0.12);
}

.logs-search-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 15px;
  outline: none;
  flex: 1;
  min-width: 0;
  font-weight: 500;
  letter-spacing: -0.01em;
}

.logs-search-input::placeholder {
  color: var(--text-tertiary);
  font-weight: 400;
}



.clear-search-btn {
  background: transparent;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.clear-search-btn:hover {
  color: var(--text-secondary);
}

.streaming-toggle-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.streaming-toggle-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.streaming-toggle-btn.active {
  background: var(--success);
  color: white;
  border-color: var(--success);
}

.streaming-toggle-btn.active .toggle-indicator {
  background: white;
  animation: pulse 1.5s infinite;
}

.fetch-logs-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--primary);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.fetch-logs-btn:hover {
  background: var(--primary-hover);
  border-color: var(--primary-hover);
}

.fetch-logs-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.logs-display-container {
  flex: 1;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  overflow: hidden;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(30px) saturate(180%);
  -webkit-backdrop-filter: blur(30px) saturate(180%);
  display: flex;
  flex-direction: column;
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 8px 40px rgba(0, 0, 0, 0.3);
}

.logs-display {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
}

.logs-display-wrapper {
  position: relative;
}

.logs-copy-btn {
  position: absolute;
  top: 12px;
  right: 12px;
  padding: 8px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.12);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  opacity: 0;
  transform: translateY(-4px);
  box-shadow: 
    0 4px 12px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.logs-display-wrapper:hover .logs-copy-btn {
  opacity: 1;
  transform: translateY(0);
}

.logs-copy-btn:hover {
  background: rgba(255, 255, 255, 0.18);
  border-color: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  transform: translateY(-2px);
  box-shadow: 
    0 6px 20px rgba(0, 0, 0, 0.3),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

.logs-content {
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Ubuntu Mono', monospace;
  font-size: 11px;
  line-height: 1.6;
  white-space: pre-wrap;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.logs-placeholder {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-tertiary);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.placeholder-icon {
  color: var(--text-tertiary);
  opacity: 0.5;
}

.placeholder-sub {
  font-size: 12px;
  opacity: 0.7;
}

/* Describe View */
.describe-header {
  margin-bottom: 16px;
}

.describe-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  position: relative;
}

.describe-action-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s;
}

.describe-action-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.describe-action-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.describe-menu-btn {
  padding: 8px 16px;
  border-radius: 10px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 4px 12px rgba(0, 0, 0, 0.1);
}

.describe-menu-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  transform: translateY(-1px);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    0 6px 20px rgba(0, 0, 0, 0.15);
}

.describe-menu-btn:disabled {
  opacity: 0.6;
  cursor: not-allowed;
  transform: none;
}

.describe-menu {
  position: absolute;
  top: 100%;
  right: 0;
  margin-top: 8px;
  min-width: 220px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(30, 30, 35, 0.95);
  backdrop-filter: blur(40px) saturate(180%);
  -webkit-backdrop-filter: blur(40px) saturate(180%);
  box-shadow: 
    0 20px 60px rgba(0, 0, 0, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  overflow: hidden;
  z-index: 100;
  display: flex;
  flex-direction: column;
  padding: 8px;
  gap: 4px;
}

.describe-menu-item {
  padding: 12px 16px;
  border-radius: 12px;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.2s;
  text-align: left;
}

.describe-menu-item:hover {
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-primary);
}

.describe-menu-item:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.describe-content {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.structured-describe {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.status-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 24px;
}

.status-card {
  padding: 24px;
  border-radius: 20px;
  border: 1px solid rgba(255, 255, 255, 0.08);
  background: color-mix(in srgb, var(--card-bg) 60%, transparent);
  backdrop-filter: blur(30px) saturate(180%);
  -webkit-backdrop-filter: blur(30px) saturate(180%);
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 
    0 12px 48px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
  transition: all 0.3s ease;
}

.status-card:hover {
  transform: translateY(-4px);
  box-shadow: 
    0 12px 40px rgba(0, 0, 0, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.12);
}

.status-card.running {
  border-color: var(--success);
  background: rgba(34, 197, 94, 0.05);
}

.status-card.pending {
  border-color: var(--warning);
  background: rgba(245, 158, 11, 0.05);
}

.status-card.failed {
  border-color: var(--error);
  background: rgba(239, 68, 68, 0.05);
}

.status-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-secondary);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
}

.status-card-header h4 {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
}

.status-card-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.status-value {
  font-size: 24px;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.01em;
  line-height: 1.2;
}

.status-detail,
.events-summary,
.volumes-list {
  font-size: 12px;
  color: var(--text-secondary);
}

.events-detail {
  font-size: 11px;
  color: var(--text-tertiary);
}

.volumes-list {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.volume-item {
  padding: 2px 6px;
  background: var(--hover);
  border-radius: 4px;
  font-size: 11px;
  color: var(--text-secondary);
}

.kv-grids {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.kv-card {
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  overflow: hidden;
  background: color-mix(in srgb, var(--card-bg) 70%, transparent);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  box-shadow: 
    0 8px 32px rgba(0, 0, 0, 0.15),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.kv-card-header {
  padding: 16px 20px;
  margin: 0;
  background: rgba(139, 92, 246, 0.08);
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.kv-card-content {
  padding: 20px;
}

.container-item,
.condition-item {
  padding: 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel-bg);
}

.container-header,
.condition-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.container-state,
.condition-status {
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
}

.container-state.running,
.condition-status.true {
  background: var(--success);
  color: white;
}

.container-state.waiting,
.container-state.pending,
.condition-status.false {
  background: var(--warning);
  color: white;
}

.container-state.terminated,
.container-state.failed {
  background: var(--error);
  color: white;
}

.container-details,
.condition-details {
  font-size: 12px;
  color: var(--text-secondary);
  display: grid;
  gap: 4px;
}

.labels-cloud {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.label-tag {
  padding: 4px 8px;
  background: var(--hover);
  border: 1px solid var(--border);
  border-radius: 4px;
  font-size: 11px;
  color: var(--text-secondary);
}

.raw-describe {
  flex: 1;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 16px;
  overflow: hidden;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(30px) saturate(180%);
  -webkit-backdrop-filter: blur(30px) saturate(180%);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 8px 40px rgba(0, 0, 0, 0.3);
}

.raw-content {
  font-family: 'SF Mono', 'Menlo', 'Monaco', 'Ubuntu Mono', monospace;
  font-size: 11px;
  line-height: 1.6;
  white-space: pre-wrap;
  padding: 20px;
  margin: 0;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.describe-placeholder {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-tertiary);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

/* AI Help View */
.ai-help-header {
  margin-bottom: 16px;
}

.ai-help-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.ai-help-action-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s;
}

.ai-help-action-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.ai-help-content {
  flex: 1;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.ai-help-init {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 40px 20px;
  text-align: center;
  gap: 24px;
}

.ai-help-prompt {
  max-width: 400px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 16px;
}

.prompt-icon {
  color: var(--text-tertiary);
  opacity: 0.5;
}

.quick-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  justify-content: center;
}

.quick-action-btn {
  padding: 16px 24px;
  border-radius: 16px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.2s;
  box-shadow: 
    0 6px 20px rgba(0, 0, 0, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.quick-action-btn:hover {
  background: rgba(255, 255, 255, 0.15);
  border-color: rgba(255, 255, 255, 0.2);
  color: var(--text-primary);
  transform: translateY(-2px);
  box-shadow: 
    0 10px 25px rgba(0, 0, 0, 0.15),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

.ai-analysis {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.aha-moment-card {
  padding: 24px;
  border-radius: 20px;
  border: 2px solid var(--primary);
  background: linear-gradient(135deg, rgba(138, 130, 224, 0.15), rgba(138, 130, 224, 0.05));
  backdrop-filter: blur(30px) saturate(180%);
  -webkit-backdrop-filter: blur(30px) saturate(180%);
  display: flex;
  flex-direction: column;
  gap: 16px;
  box-shadow: 
    0 10px 40px rgba(138, 130, 224, 0.2),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
}

.aha-moment-header {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--primary);
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.12em;
}

.aha-moment-content {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.aha-moment-link {
  font-size: 12px;
  color: var(--text-secondary);
}

.related-issues {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--card-bg);
}

.related-issues h5 {
  margin: 0 0 12px 0;
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--text-tertiary);
}

.issues-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.issue-item {
  padding: 8px 12px;
  background: var(--panel-bg);
  border-radius: 6px;
  font-size: 13px;
  color: var(--text-secondary);
}

.action-steps {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--card-bg);
}

.action-steps h5 {
  margin: 0 0 12px 0;
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--text-tertiary);
}

.action-buttons {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.action-btn {
  padding: 14px 24px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--text-secondary);
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.2s;
  box-shadow: 
    0 4px 12px rgba(0, 0, 0, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.05);
}

.action-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  border-color: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  transform: translateY(-2px);
  box-shadow: 
    0 8px 20px rgba(0, 0, 0, 0.15),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}

.action-btn.primary {
  background: var(--primary);
  color: white;
  border-color: var(--primary);
  box-shadow: 
    0 6px 20px rgba(138, 130, 224, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
  font-weight: 700;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.2);
}

.action-btn.primary:hover {
  background: var(--primary-hover);
  border-color: var(--primary-hover);
  transform: translateY(-2px);
  box-shadow: 
    0 10px 25px rgba(138, 130, 224, 0.5),
    inset 0 1px 0 rgba(255, 255, 255, 0.3);
}

.full-analysis {
  padding: 16px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--card-bg);
}

.full-analysis h5 {
  margin: 0 0 12px 0;
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.12em;
  color: var(--text-tertiary);
}

.analysis-content {
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-primary);
}

.analysis-content :deep(strong) {
  color: var(--text-primary);
  font-weight: 700;
}

.analysis-content :deep(em) {
  font-style: italic;
  color: var(--text-secondary);
}

/* Footer */
.alert-modal-footer {
  padding: 24px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--modal-bg);
}

.footer-left {
  display: flex;
  gap: 12px;
  align-items: center;
}

.footer-right {
  display: flex;
  gap: 12px;
}

.state-select {
  padding: 12px 20px;
  border-radius: 14px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.1);
  backdrop-filter: blur(20px) saturate(180%);
  -webkit-backdrop-filter: blur(20px) saturate(180%);
  color: var(--text-primary);
  font-size: 15px;
  font-weight: 500;
  cursor: pointer;
  outline: none;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 4px 12px rgba(0, 0, 0, 0.1);
  appearance: none;
  -webkit-appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' fill='%23ffffff' viewBox='0 0 256 256'%3E%3Cpath d='M213.66,101.66l-80,80a8,8,0,0,1-11.32,0l-80-80A8,8,0,0,1,53.66,90.34L128,164.69l74.34-74.35a8,8,0,0,1,11.32,11.32Z'%3E%3C/path%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 16px center;
  background-size: 16px;
  padding-right: 48px;
}

.state-select:hover {
  background: rgba(255, 255, 255, 0.14);
  border-color: rgba(255, 255, 255, 0.2);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    0 6px 20px rgba(0, 0, 0, 0.15);
  transform: translateY(-1px);
}

.state-select:focus {
  border-color: var(--primary);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.1),
    0 0 0 2px rgba(138, 130, 224, 0.3);
}

.small-btn {
  padding: 10px 20px;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.1);
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  font-weight: 400;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  backdrop-filter: blur(10px) saturate(180%);
  -webkit-backdrop-filter: blur(10px) saturate(180%);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.03),
    0 2px 8px rgba(0, 0, 0, 0.08);
}

.small-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  border-color: rgba(255, 255, 255, 0.15);
  color: var(--text-primary);
  transform: translateY(-1px);
  box-shadow: 
    inset 0 1px 0 rgba(255, 255, 255, 0.05),
    0 4px 12px rgba(0, 0, 0, 0.1);
}

.small-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
}

.small-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.investigate-btn {
  padding: 16px 32px;
  border-radius: 20px;
  border: none;
  background: linear-gradient(135deg, var(--primary), #9d94ff);
  color: white;
  font-size: 17px;
  font-weight: 700;
  cursor: pointer;
  display: flex;
  align-items: center;
  gap: 10px;
  transition: all 0.3s cubic-bezier(0.175, 0.885, 0.32, 1.275);
  box-shadow: 
    0 8px 30px rgba(138, 130, 224, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.3);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
  letter-spacing: -0.01em;
  position: relative;
  overflow: hidden;
}

.investigate-btn::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 50%;
  background: linear-gradient(rgba(255, 255, 255, 0.2), transparent);
  border-radius: 20px 20px 0 0;
  pointer-events: none;
}

.investigate-btn:hover {
  background: linear-gradient(135deg, var(--primary-hover), #a89fff);
  transform: translateY(-2px);
  box-shadow: 
    0 12px 40px rgba(138, 130, 224, 0.6),
    inset 0 1px 0 rgba(255, 255, 255, 0.4);
}

.investigate-btn:active {
  transform: translateY(0);
  box-shadow: 
    0 4px 20px rgba(138, 130, 224, 0.4),
    inset 0 1px 0 rgba(255, 255, 255, 0.2);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .alert-modal {
    max-width: 95vw;
    max-height: 95vh;
  }
  
  .status-cards {
    grid-template-columns: 1fr;
  }
  
  .logs-controls {
    flex-direction: column;
    align-items: stretch;
  }
  
  .logs-search {
    max-width: 100%;
  }
  
  .action-buttons {
    flex-direction: column;
  }
  
  .alert-modal-footer {
    flex-direction: column;
    gap: 16px;
    align-items: stretch;
  }
  
  .footer-left,
  .footer-right {
    width: 100%;
    justify-content: center;
  }
}
</style>