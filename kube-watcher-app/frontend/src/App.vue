<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
console.log('App.vue setup starting...')
import Sidebar from './components/layout/Sidebar.vue'
import Header from './components/layout/Header.vue'
import ArguskubePanel from './components/panels/ArguskubePanel.vue'
import AnomaliesPanel from './components/panels/AnomaliesPanel.vue'
import WatcherPanel from './components/panels/WatcherPanel.vue'
import SummaryGrid from './components/summary/SummaryGrid.vue'
import SweepControls from './components/controls/SweepControls.vue'
import SettingsModal from './components/modals/SettingsModal.vue'
import ScanModal from './components/modals/ScanModal.vue'
import AlertDetailModal from './components/modals/AlertDetailModal.vue'
import AnomstackErrorModal from './components/modals/AnomstackErrorModal.vue'
import InvestigationModal from './components/modals/InvestigationModal.vue'
import { useWorkspace } from './composables/useWorkspace'
import { useChat } from './composables/useChat'
import { useSynapseSweep } from './composables/useSynapseSweep'
import { useSettings } from './composables/useSettings'
import { useAnomstack } from './composables/useAnomstack'
import { useClusterLabels } from './composables/useClusterLabels'
import { renderMarkdown, formatWhen } from './utils'
import { data } from '../wailsjs/go/models'
import type { ScanCell } from './components/panels/types'

type TabId = 'arguskube' | 'anomalies' | 'watcher'

const tabs: { id: TabId; label: string; eyebrow: string }[] = [
  { id: 'arguskube', label: 'Arguskube', eyebrow: '' },
  { id: 'anomalies', label: 'Anomalies', eyebrow: '' },
  { id: 'watcher', label: 'Watcher', eyebrow: '' },
]

const activeTab = ref<TabId>('anomalies')

// Composables
const workspace: ReturnType<typeof useWorkspace> = useWorkspace()
const chat: ReturnType<typeof useChat> = useChat()
const sweep: ReturnType<typeof useSynapseSweep> = useSynapseSweep()
const settings: ReturnType<typeof useSettings> = useSettings()
const anomstack: ReturnType<typeof useAnomstack> = useAnomstack()
const clusterLabels: ReturnType<typeof useClusterLabels> = useClusterLabels()

const workspaceSummary = computed(() => workspace.workspace.value?.summary)

const showContent = computed(() => !workspace.isLoading.value && !workspace.error.value)

// Local UI state
const isArguskubeExpanded = ref(false)
const isTopPriorityExpanded = ref(false)
const isPriorityQueueExpanded = ref(false)
const showScanModal = ref(false)
const selectedScanCell = ref<ScanCell | null>(null)
const showAlertDetailModal = ref(false)
const selectedAlert = ref<data.AlertRecord | null>(null)
const showInvestigationModal = ref(false)
const loadError = ref<string | null>(null)
const hasAutoRetried = ref(false)
const sidebarCollapsed = ref(false)

// Computed
const connectivityStatus = computed(() => {
  if (!workspace.mcpStatus.value) return 'red'
  if (workspace.mcpStatus.value.status === 'ok') return 'green'
  return 'yellow'
})

const activeClusterDisplay = computed(() => {
  return clusterLabels.clusterLabels.value[workspace.currentContext.value]?.label || workspace.currentContext.value
})

const clusterLabel = computed(() => clusterLabels.clusterLabels.value[workspace.currentContext.value]?.label || '')



const scanGrid = computed(() => sweep.synapseSweepGrid.value as { cells: ScanCell[], headline: string, fallback: string })

const errorMessage = computed(() => workspace.error.value)

// Event handlers
function handleTabChange(tab: string) {
  console.log('Tab changed to:', tab, 'current activeTab:', activeTab.value, 'isLoading:', workspace.isLoading.value)
  console.trace('handleTabChange stack trace')
  activeTab.value = tab as TabId
}

function handleEditClusterLabel() {
  const cluster = workspace.mcpStatus.value?.cluster
  if (!cluster) return
  const currentInfo = clusterLabels.clusterLabels.value[cluster]
  clusterLabels.createClusterLabel(cluster, currentInfo)
}

function handleOpenSettings() {
  settings.showSettings.value = true
}

function handleScan() {
  sweep.runSynapseSweep()
}



function handleOpenScanModal(cell: ScanCell) {
  selectedScanCell.value = cell
  showScanModal.value = true
}

function handleCloseScanModal() {
  showScanModal.value = false
  selectedScanCell.value = null
}

function handleOpenAlertDetailModal(alert: data.AlertRecord) {
  selectedAlert.value = alert
  showAlertDetailModal.value = true
}

function handleCloseAlertDetailModal() {
  showAlertDetailModal.value = false
  selectedAlert.value = null
}

function handleCloseInvestigationModal() {
  showInvestigationModal.value = false
}

function handleToggleSidebarCollapse() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

function handleRefreshContext() {
  workspace.loadWorkspace()
}

function handleInvestigateAlert(alert: data.AlertRecord) {
  chat.sendPrompt(`Please investigate this alert: ${alert.name} (${alert.severity} severity).`, JSON.stringify(alert))
}

function handleUsePlaybook(playbook: any) {
  chat.usePlaybook(playbook, (tab) => {
    // Tab switching disabled after removal of deploy tab
  })
}

function handleQuickPrompt(prompt: string) {
  chat.sendPrompt(prompt)
}

function handleSendPrompt(prompt: string) {
  chat.sendPrompt(prompt)
}

function handleConnectAnomstack() {
  anomstack.connectAnomstack().then(() => {
    workspace.loadWorkspace()
  })
}

function handleInvestigateAnomalies() {
  showInvestigationModal.value = true
}

// Error handling
function retryWorkspace() {
  workspace.error.value = null
  workspace.loadWorkspace()
}

// Lifecycle
onMounted(async () => {
  console.log('App mounted, loading workspace...')
  try {
    clusterLabels.loadClusterLabels()
    settings.loadSettings()
    await workspace.loadCurrentContext()
    console.log('Current context loaded:', workspace.currentContext.value)
    await workspace.loadWorkspace()
    console.log('Workspace loaded, isLoading:', workspace.isLoading.value)
    // Auto-run synapse sweep after 2 seconds
    setTimeout(() => {
      sweep.runSynapseSweep()
    }, 2000)
  } catch (err) {
    console.error('Error during app initialization:', err)
    // Error is already set in workspace.error
  }
})

// Auto-retry workspace load on error
watch(() => workspace.error.value, (newError) => {
  if (newError && !hasAutoRetried.value) {
    console.log('Workspace error detected, retrying in 5 seconds...')
    hasAutoRetried.value = true
    setTimeout(() => {
      if (workspace.error.value) {
        console.log('Auto-retrying workspace load...')
        retryWorkspace()
      }
    }, 5000)
  }
})
</script>

<template>
  <div :class="['shell', { 'sidebar-collapsed': sidebarCollapsed }]">
    <Sidebar
      :activeTab="activeTab"
      :tabs="tabs"
      :currentContext="workspace.currentContext.value"
      :clusterLabel="clusterLabel"
      :endpoint="workspace.mcpEndpoint.value"
      :connectivityStatus="connectivityStatus"
      :collapsed="sidebarCollapsed"
      @tab-change="handleTabChange"
      @edit-cluster-label="handleEditClusterLabel"
      @open-settings="handleOpenSettings"
      @toggle-collapse="handleToggleSidebarCollapse"
    />

    <main class="main-panel">
      <!-- Debug panel commented out to avoid rendering issues -->
      <!-- <div class="debug-tab" style="position: fixed; top: 10px; left: 300px; z-index: 9999; background: rgba(0,0,0,0.8); color: white; padding: 5px; font-size: 12px; max-width: 600px;">
        Active tab: {{ activeTab }} | Loading: {{ workspace.isLoading.value }} | Error: {{ workspace.error.value }} | Context: {{ workspace.currentContext.value }} | Workspace: {{ !!workspace.workspace.value }} | Alerts: {{ workspace.alerts.value.length }} | Services: {{ workspace.services.value.length }}
      </div> -->
      <Header :title="workspaceSummary?.headline ?? 'Loading AI workspace…'" :subtitle="workspaceSummary?.subheadline ?? 'Pulling alerts, history, and recommendations.'">
        <template #actions>
          <SweepControls
            :sweepInterval="sweep.sweepInterval.value"
            @update:sweepInterval="sweep.sweepInterval.value = $event"
            :customIntervalValue="sweep.customIntervalValue.value"
            @update:customIntervalValue="sweep.customIntervalValue.value = $event"
            :customIntervalUnit="sweep.customIntervalUnit.value"
            @update:customIntervalUnit="sweep.customIntervalUnit.value = $event"
            :is-running-sweep="sweep.isRunningSweep.value"
            @scan="handleScan"
          />
        </template>
        <template #meta>
          <div class="connectivity-info">
            <div class="mcp-details">
              <div class="mcp-endpoint">{{ workspace.mcpEndpoint.value }}</div>
              <div class="mcp-cluster" @click="handleEditClusterLabel">
                {{ activeClusterDisplay }}
              </div>
            </div>
            <div :class="['status-light', connectivityStatus]" :title="`Status: ${connectivityStatus}`"></div>
          </div>
          <div v-if="sweep.countdownText" class="sweep-countdown-bar">
            {{ sweep.countdownText }}
          </div>
        </template>
      </Header>

      <SummaryGrid :cards="workspace.summaryCards.value" />

      <section class="workspace">
        <div class="workspace-main">
          <div v-if="errorMessage" class="loading-state">
            <div class="arguskube-thinking">
              <svg width="64" height="64" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 2C6.477 2 2 6.477 2 12C2 17.523 6.477 22 12 22C17.523 22 22 17.523 22 12C22 6.477 17.523 2 12 2Z" stroke="currentColor" stroke-width="1" stroke-opacity="0.2"/>
                <circle cx="9" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
                <circle cx="15" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
              </svg>
            </div>
            <p>Failed to load workspace: {{ errorMessage }}</p>
            <button class="primary-btn" @click="retryWorkspace">Retry</button>
          </div>
          <div v-else-if="workspace.isLoading.value" class="loading-state">
            <div class="arguskube-thinking">
              <svg width="64" height="64" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 2C6.477 2 2 6.477 2 12C2 17.523 6.477 22 12 22C17.523 22 22 17.523 22 12C22 6.477 17.523 2 12 2Z" stroke="currentColor" stroke-width="1" stroke-opacity="0.2"/>
                <path d="M12 4C7.582 4 4 7.582 4 12C4 16.418 7.582 20 12 20C16.418 20 20 16.418 20 12C20 7.582 16.418 4 12 4Z" fill="currentColor" fill-opacity="0.05"/>
                <circle cx="9" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
                <circle cx="15" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
                <path d="M8 16C8 16 9.5 17.5 12 17.5C14.5 17.5 16 16 16 16" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
                <path d="M11.5 7C11.5 7 12 6.5 12 6C12 5.5 11.5 5 11.5 5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
                <path d="M12.5 7.5C12.5 7.5 13.5 7 13.5 6C13.5 5 12.5 4.5 12.5 4.5" stroke="currentColor" stroke-width="1" stroke-linecap="round"/>
              </svg>
            </div>
            <p>Collecting the control plane state…</p>
          </div>

            <template v-else>
              <div v-if="activeTab === 'arguskube'" key="arguskube">
                <ArguskubePanel
                  :is-expanded="isArguskubeExpanded"
                  :messages="chat.chatMessages.value"
                  :playbooks="workspace.workspace.value?.playbooks"
                  :is-thinking="chat.isThinking.value"
                  :render-markdown="renderMarkdown"
                  @toggle-expand="isArguskubeExpanded = !isArguskubeExpanded"
                  @use-playbook="handleUsePlaybook"
                  @quick-prompt="handleQuickPrompt"
                  @send-prompt="handleSendPrompt"
                />
              </div>
              <div v-else-if="activeTab === 'anomalies'" key="anomalies">
                <AnomaliesPanel
                  :alerts="workspace.alerts.value"
                  :recommendations="workspace.recommendations.value"
                  :timeline="workspace.timeline.value"
                  :anomstack-connected="workspace.anomstackConnected.value"
                  :signoz-connected="false"
                  :is-connecting-anomstack="anomstack.isConnectingAnomstack.value"
                  :is-investigating="chat.isThinking.value"
                  :format-when="formatWhen"
                  @connect-anomstack="handleConnectAnomstack"
                  @investigate-anomalies="handleInvestigateAnomalies"
                  @alert-clicked="handleOpenAlertDetailModal"
                />
              </div>

              <div v-else-if="activeTab === 'watcher'" key="watcher">
                 <WatcherPanel />
              </div>
              <div v-else style="background: orange; color: white; padding: 20px;">
                UNKNOWN TAB: {{ activeTab }}
              </div>
            </template>
        </div>


      </section>
    </main>

    <!-- Modals -->
    <SettingsModal
      v-model:show="settings.showSettings.value"
      :settings="settings.settings.value"
      @save="settings.saveSettings"
    />
    <ScanModal
      :cell="selectedScanCell"
      @close="handleCloseScanModal"
    />
    <AlertDetailModal
      :alert="selectedAlert"
      :format-when="formatWhen"
      @close="handleCloseAlertDetailModal"
      @investigate="handleInvestigateAlert"
    />
    <AnomstackErrorModal
      v-model:show="anomstack.showAnomstackErrorModal.value"
      :error="anomstack.anomstackError.value"
      :recommendations="anomstack.anomstackRecommendations.value"
      @retry="handleConnectAnomstack"
    />
    <InvestigationModal
      :show="showInvestigationModal"
      :context="{
        alerts: workspace.alerts.value,
        recommendations: workspace.recommendations.value,
        timeline: workspace.timeline.value,
      }"
      @close="handleCloseInvestigationModal"
      @refresh-context="handleRefreshContext"
    />
  </div>
</template>

<style>
.connectivity-info {
  display: flex;
  align-items: center;
  gap: 12px;
  background: rgba(255, 255, 255, 0.03);
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid rgba(255, 255, 255, 0.05);
}

.mcp-details {
  text-align: right;
}

.mcp-endpoint {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.4);
  font-family: monospace;
}

.mcp-cluster {
  font-size: 12px;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.8);
  cursor: pointer;
  transition: color 0.2s;
}

.mcp-cluster:hover {
  color: #fff;
  text-decoration: underline;
}

.status-light {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  box-shadow: 0 0 8px currentColor;
}

.status-light.green {
  background-color: #10b981;
  color: rgba(16, 185, 129, 0.4);
}

.status-light.yellow {
  background-color: #f59e0b;
  color: rgba(245, 158, 11, 0.4);
}

.status-light.red {
  background-color: #ef4444;
  color: rgba(239, 68, 68, 0.4);
}

.sweep-countdown-bar {
  position: absolute;
  bottom: -24px;
  left: 50%;
  transform: translateX(-50%);
  font-size: 10px;
  color: #10b981;
  letter-spacing: 0.02em;
  opacity: 0.8;
  white-space: nowrap;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", sans-serif;
}

.sweep-btn {
  font-size: 10px !important;
  letter-spacing: 0.02em;
  padding: 4px 10px !important;
  min-height: 26px;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", sans-serif;
}



.arguskube-thinking {
  margin-bottom: 24px;
  color: rgba(255, 255, 255, 0.6);
  animation: float 3s ease-in-out infinite;
}

.eye-blink {
  animation: blink 4s infinite;
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-10px); }
}

@keyframes blink {
  0%, 90%, 100% { opacity: 1; }
  95% { opacity: 0; }
}

.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: rgba(255, 255, 255, 0.5);
}
.arguskube-thinking.mini {
  margin-bottom: 0;
  animation: float 2s ease-in-out infinite;
  display: inline-block;
  vertical-align: middle;
  margin-right: 8px;
}

.thinking-bubble {
  font-style: italic;
  opacity: 0.8;
  display: flex;
  align-items: center;
}

.hero {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 48px;
  position: relative;
}
.feature-panel {
  background:
      radial-gradient(circle at top right, rgba(125, 116, 214, 0.16), transparent 34%),
      rgba(38, 37, 49, 0.95);
}

.scrollable-panel {
  display: flex;
  flex-direction: column;
  height: 500px;
  overflow: hidden;
}

.sticky-head {
  position: sticky;
  top: 0;
  background: rgba(38, 37, 49, 0.98);
  z-index: 10;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
  margin-bottom: 16px;
}

.panel-scroll-content {
  overflow-y: auto;
  flex: 1;
  padding-right: 4px;
}

.panel-scroll-content::-webkit-scrollbar {
  width: 4px;
}

.panel-scroll-content::-webkit-scrollbar-thumb {
  background: rgba(255, 255, 255, 0.1);
  border-radius: 2px;
}

.playbook-section {
  margin-top: 20px;
  padding-top: 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.playbook-label {
  margin-bottom: 12px;
}

.playbook-grid.mini {
  grid-template-columns: 1fr;
  gap: 12px;
}

.playbook-grid.mini .playbook-card {
  padding: 12px;
}

.playbook-grid.mini .playbook-card strong {
  font-size: 13px;
}

.playbook-grid.mini .playbook-card p {
  font-size: 11px;
}
.mini-btn {
  font-size: 11px;
  padding: 4px 8px;
  margin-top: 8px;
}
.expand-toggle {
  padding: 0;
  width: 24px;
  height: 24px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  line-height: 1;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: var(--text-muted);
}

.arguskube-panel:not(.expanded) {
  align-content: end;
}

.arguskube-panel:not(.expanded) .chat-thread {
  min-height: auto;
  max-height: 120px;
}

.panel:not(.expanded).scrollable-panel {
  height: auto;
  min-height: 0;
}

.panel:not(.expanded) .panel-head {
  border-bottom: none;
  margin-bottom: 0;
  padding-bottom: 0;
}

.head-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.settings-btn {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.refresh-btn {
  padding: 8px;
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
}

.tab-status-light {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  margin-left: 8px;
  box-shadow: 0 0 6px currentColor;
  flex-shrink: 0;
}

.tab-status-light.green {
  background-color: #10b981;
  color: rgba(16, 185, 129, 0.4);
}

.tab-status-light.red {
  background-color: #ef4444;
  color: rgba(239, 68, 68, 0.4);
}

.panel-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.connect-btn {
  font-size: 12px !important;
  padding: 6px 12px !important;
  min-height: 28px;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status-dot.green {
  background-color: #10b981;
  box-shadow: 0 0 4px rgba(16, 185, 129, 0.6);
}

.status-dot.blue {
  background-color: #3b82f6;
  box-shadow: 0 0 4px rgba(59, 130, 246, 0.6);
}



/* Scan Modal Styles */


/* Markdown rendering styles */
.markdown-content {
  line-height: 1.6;
  font-size: 15px;
  color: var(--text);
  white-space: pre-line;
}

.markdown-content h1,
.markdown-content h2,
.markdown-content h3,
.markdown-content h4,
.markdown-content h5,
.markdown-content h6 {
  margin-top: 1.5em;
  margin-bottom: 0.75em;
  color: var(--text-primary);
}

.markdown-content h1 { font-size: 1.4em; font-weight: 700; }
.markdown-content h2 { font-size: 1.25em; font-weight: 600; }
.markdown-content h3 { font-size: 1.1em; font-weight: 600; }

.markdown-content p {
  margin-bottom: 1em;
}

.markdown-content strong {
  font-weight: 600;
}

.markdown-content em {
  font-style: italic;
}

.markdown-content ul,
.markdown-content ol {
  padding-left: 2em;
  margin-bottom: 1em;
}

.markdown-content li {
  margin-bottom: 0.5em;
}

.markdown-content code {
  background: rgba(255, 255, 255, 0.1);
  color: #e0e0e0;
  padding: 0.2em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
  font-family: 'SF Mono', monospace;
}

.markdown-content pre {
  background: #0c0b11dd;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  padding: 1.25rem;
  overflow-x: auto;
  margin: 1em 0;
  position: relative;
}

.markdown-content pre code {
  background: none;
  border-radius: 0;
  padding: 0;
  font-size: 14px;
}

.markdown-content blockquote {
  border-left: 4px solid #3b82f6;
  padding-left: 1.25em;
  margin: 1.5em 0;
  font-style: italic;
  color: rgba(255, 255, 255, 0.85);
  background: rgba(59, 130, 246, 0.1);
  padding: 1rem 1.25rem;
  border-radius: 0 8px 8px 0;
}

.markdown-content table {
  border-collapse: collapse;
  margin: 1em 0;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  overflow: hidden;
}

.markdown-content th,
.markdown-content td {
  padding: 0.75em 1em;
  text-align: left;
  border-bottom: 1px solid rgba(255, 255, 255, 0.1);
}

.markdown-content th {
  background: rgba(255, 255, 255, 0.08);
  font-weight: 600;
}

 .preview-markdown {
   max-height: 120px;
   overflow: hidden;
   mask-image: linear-gradient(to bottom, white 80%, transparent);
   -webkit-mask-image: linear-gradient(to bottom, white 80%, transparent);
 }

  .workspace-main {
    min-height: 500px;
    position: relative;
  }

  .test-panel {
    padding: 2rem;
    border: 2px solid green;
    border-radius: 12px;
    background: rgba(0, 255, 0, 0.1);
    margin: 1rem 0;
  }

</style>
