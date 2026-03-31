<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import CommandBlock from './components/CommandBlock.vue'
import {
  GetAIWorkspace,
  GetAlerts,
  GetHistory,
  GetLogs,
  GetRecommendations,
  GetServiceStatus,
  GetStatus,
  GetEndpoint,
  AskAI,
  RunSynapseSweep,
  StartService,
  StopService,
} from '../wailsjs/go/main/App'
import { data } from '../wailsjs/go/models'

type TabId = 'arguskube' | 'anomalies' | 'deploy' | 'runtime'

type ChatMessage = {
  role: 'assistant' | 'user'
  content: string
  tone?: string
}

type ScanHealth = 'good' | 'warning' | 'unhealthy'

type ScanCell = {
  title: string
  summary: string
  health: ScanHealth
  detail: string
}

const tabs: { id: TabId; label: string; eyebrow: string }[] = [
  { id: 'arguskube', label: 'Arguskube Sentinels', eyebrow: 'First class' },
  { id: 'anomalies', label: 'Anomalies', eyebrow: 'Live context' },
  { id: 'deploy', label: 'Deploy', eyebrow: 'Local to cluster' },
  { id: 'runtime', label: 'Runtime', eyebrow: 'Services + scans' },
]

const activeTab = ref<TabId>('arguskube')
const workspace = ref<data.AIWorkspace | null>(null)
const alerts = ref<data.AlertRecord[]>([])
const history = ref<data.Incident[]>([])
const recommendations = ref<data.Recommendation[]>([])
const services = ref<data.ServiceStatus[]>([])
const logs = ref<data.LogLine[]>([])
const synapseSweepOutput = ref('')
const mcpStatus = ref<data.StatusResponse | null>(null)
const mcpEndpoint = ref('')
const clusterLabels = ref<Record<string, string>>({})
const isLoading = ref(true)
const isThinking = ref(false)
const isArguskubeExpanded = ref(false)
const isTopPriorityExpanded = ref(false)
const isPriorityQueueExpanded = ref(false)
const isDeploymentTracksExpanded = ref(true)
const isRunningSweep = ref(false)
const isRefreshingServices = ref(false)
const sweepInterval = ref('manual')
const customIntervalValue = ref(5)
const customIntervalUnit = ref('m')
const nextSweepTime = ref<number | null>(null)
const countdownText = ref('')
let sweepTimer: any = null
let countdownTimer: any = null

const updateCountdown = () => {
  if (!nextSweepTime.value) {
    countdownText.value = ''
    return
  }

  const now = Date.now()
  const diff = nextSweepTime.value - now

  if (diff <= 0) {
    countdownText.value = 'Arguskube Sentinels are online now'
    return
  }

  const seconds = Math.floor(diff / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  let timeStr = ''
  if (days > 0) {
    timeStr = `${days}d ${hours % 24}h`
  } else if (hours > 0) {
    timeStr = `${hours}h ${minutes % 60}m`
  } else if (minutes > 0) {
    timeStr = `${minutes}m ${seconds % 60}s`
  } else {
    timeStr = `${seconds}s`
  }

  countdownText.value = `Arguskube Sentinels are online in ${timeStr}`
}

watch(sweepInterval, (newVal: string) => {
  if (sweepTimer) {
    clearInterval(sweepTimer)
    sweepTimer = null
  }
  if (countdownTimer) {
    clearInterval(countdownTimer)
    countdownTimer = null
  }
  nextSweepTime.value = null
  countdownText.value = ''

  if (newVal === 'manual') return

  let ms = 0
  if (newVal === 'custom') {
    const unitMsMap: Record<string, number> = {
      'm': 60 * 1000,
      'h': 60 * 60 * 1000,
      'd': 24 * 60 * 60 * 1000,
    }
    ms = (customIntervalValue.value || 0) * (unitMsMap[customIntervalUnit.value] || 60000)
  } else {
    const msMap: Record<string, number> = {
      '5m': 5 * 60 * 1000,
      '30m': 30 * 60 * 1000,
      '2h': 2 * 60 * 60 * 1000,
      '5h': 5 * 60 * 60 * 1000,
      '12h': 12 * 60 * 60 * 1000,
      '1d': 24 * 60 * 60 * 1000,
    }
    ms = msMap[newVal]
  }

  if (ms > 0) {
    nextSweepTime.value = Date.now() + ms
    updateCountdown()
    countdownTimer = setInterval(updateCountdown, 1000)

    sweepTimer = setInterval(() => {
      if (!isRunningSweep.value) {
        runSynapseSweep()
        nextSweepTime.value = Date.now() + ms
      }
    }, ms)
  }
})

watch([customIntervalValue, customIntervalUnit], () => {
  if (sweepInterval.value === 'custom') {
    // Trigger the sweepInterval watcher
    const val = sweepInterval.value
    sweepInterval.value = 'manual'
    nextTick(() => {
      sweepInterval.value = val
    })
  }
})

const draftPrompt = ref('')
const chatMessages = ref<ChatMessage[]>([
  {
    role: 'assistant',
    tone: 'calm',
    content: 'Arguskube Sentinels are ready to investigate the cluster. Load the workspace, then ask for rollout prep, incident triage, or anomaly validation.',
  },
])

const summaryCards = computed(() => {
  if (!workspace.value) return []
  return [
    { label: 'Open alerts', value: workspace.value.summary.openAlerts, accent: 'rose' },
    { label: 'Critical now', value: workspace.value.summary.criticalAlerts, accent: 'amber' },
    { label: 'Arguskube actions ready', value: workspace.value.summary.automationReady, accent: 'teal' },
    { label: 'Deploy targets', value: workspace.value.summary.deployTargets, accent: 'ink' },
  ]
})

const connectivityStatus = computed(() => {
  if (!mcpStatus.value) return 'red'
  if (mcpStatus.value.status === 'ok') return 'green'
  return 'yellow'
})

const activeClusterDisplay = computed(() => {
  const cluster = mcpStatus.value?.cluster || 'unknown'
  return clusterLabels.value[cluster] || cluster
})

const topPriority = computed(() => workspace.value?.priorities?.[0] ?? null)

const statusText = computed(() => {
  const running = services.value.filter((s) => s.status === 'running').length
  return `${running}/${services.value.length || 0} services online`
})

const timeline = computed(() =>
  [...history.value]
    .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
    .slice(0, 5),
)

const runtimeLogs = computed(() => logs.value.slice(0, 6))

const synapseSweepGrid = computed(() => buildScanGrid(synapseSweepOutput.value))

onMounted(async () => {
  const savedLabels = localStorage.getItem('cluster-labels')
  if (savedLabels) {
    try {
      clusterLabels.value = JSON.parse(savedLabels)
    } catch (e) {
      console.error('Failed to parse cluster labels', e)
    }
  }
  await loadWorkspace()
})

async function loadWorkspace() {
  isLoading.value = true
  try {
    const [ws, alertData, historyData, recommendationData, serviceData, logData, statusData, endpointData] = await Promise.all([
      GetAIWorkspace(),
      GetAlerts(),
      GetHistory(),
      GetRecommendations(),
      GetServiceStatus(),
      GetLogs(),
      GetStatus(),
      GetEndpoint(),
    ])
    workspace.value = ws
    alerts.value = alertData
    history.value = historyData
    recommendations.value = recommendationData
    services.value = serviceData
    logs.value = logData
    mcpStatus.value = statusData
    mcpEndpoint.value = endpointData

    if (ws.summary.headline) {
      chatMessages.value = [
        {
          role: 'assistant',
          tone: 'calm',
          content: `${ws.summary.headline} ${ws.summary.subheadline}`,
        },
      ]
    }
  } catch (error) {
    chatMessages.value.push({
      role: 'assistant',
      tone: 'warn',
      content: `Workspace load failed: ${formatError(error)}`,
    })
  } finally {
    isLoading.value = false
  }
}

async function sendPrompt(input: string | Event = draftPrompt.value) {
  const prompt = typeof input === 'string' ? input : draftPrompt.value
  const trimmed = prompt.trim()
  if (!trimmed) return

  chatMessages.value.push({ role: 'user', content: trimmed })
  draftPrompt.value = ''
  isThinking.value = true

  const lines: string[] = []
  if (topPriority.value) lines.push(`Priority: ${topPriority.value.title} — ${topPriority.value.detail}`)
  if (workspace.value) {
    lines.push(
      `Workspace: ${workspace.value.summary.openAlerts} alerts, ${workspace.value.summary.criticalAlerts} critical, ${workspace.value.summary.deployTargets} deployment tracks.`,
    )
  }
  if (activeTab.value === 'deploy') {
    const preferredPlan = workspace.value?.deploymentPlans?.[1] ?? workspace.value?.deploymentPlans?.[0]
    if (preferredPlan) lines.push(`Recommended path: ${preferredPlan.title}. First command: ${preferredPlan.commands[0]}`)
  }
  if (activeTab.value === 'runtime' && synapseSweepOutput.value) {
    lines.push('Synapse Sweep scan is available in the runtime panel for deeper cluster detail.')
  }

  try {
    const response = await AskAI(trimmed, lines.join('\n'))
    chatMessages.value.push({ role: 'assistant', tone: 'calm', content: response })
  } catch (error) {
    chatMessages.value.push({
      role: 'assistant',
      tone: 'warn',
      content: `Arguskube encountered an error: ${formatError(error)}`,
    })
  } finally {
    isThinking.value = false
  }
}

function usePlaybook(playbook: data.AIPlaybook) {
  activeTab.value = playbook.target === 'Deployment' ? 'deploy' : 'arguskube'
  draftPrompt.value = playbook.prompt
  void sendPrompt(playbook.prompt)
}

async function toggleService(service: data.ServiceStatus) {
  isRefreshingServices.value = true
  try {
    if (service.status === 'running') {
      await StopService(service.name)
    } else {
      await StartService(service.name)
    }
    services.value = await GetServiceStatus()
  } catch (error) {
    chatMessages.value.push({
      role: 'assistant',
      tone: 'warn',
      content: `Service action failed for ${service.name}: ${formatError(error)}`,
    })
  } finally {
    isRefreshingServices.value = false
  }
}

function createClusterLabel() {
  const cluster = mcpStatus.value?.cluster
  if (!cluster) return
  const currentLabel = clusterLabels.value[cluster] || ''
  const newLabel = window.prompt(`Enter a label for cluster context "${cluster}":`, currentLabel)
  if (newLabel !== null) {
    if (newLabel.trim()) {
      clusterLabels.value[cluster] = newLabel.trim()
    } else {
      delete clusterLabels.value[cluster]
    }
    localStorage.setItem('cluster-labels', JSON.stringify(clusterLabels.value))
  }
}

async function runSynapseSweep() {
  isRunningSweep.value = true
  synapseSweepOutput.value = ''
  try {
    synapseSweepOutput.value = await RunSynapseSweep()
    activeTab.value = 'runtime'
  } catch (error) {
    synapseSweepOutput.value = formatError(error)
  } finally {
    isRunningSweep.value = false
  }
}

function formatWhen(value: unknown) {
  if (!value) return 'Just now'
  const date = new Date(String(value))
  if (Number.isNaN(date.getTime())) return String(value)
  return new Intl.DateTimeFormat(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function formatError(error: unknown) {
  if (error instanceof Error) return error.message
  return String(error)
}

function buildScanGrid(raw: string) {
  const trimmed = raw.trim()
  if (!trimmed) {
    return { cells: [] as ScanCell[], headline: '', fallback: '' }
  }

  const parsed = tryParseSynapseSweep(trimmed)
  if (!parsed) {
    return { cells: [] as ScanCell[], headline: '', fallback: raw }
  }

  const source = readRecord(parsed)
  const report =
    Object.keys(readRecord(source.synapse_sweep)).length > 0
      ? readRecord(source.synapse_sweep)
      : Object.keys(readRecord(source.popeye)).length > 0
        ? readRecord(source.popeye)
        : source
  const sanitizers = Array.isArray(report.sanitizers) ? report.sanitizers : []
  if (sanitizers.length === 0) {
    return { cells: [] as ScanCell[], headline: '', fallback: raw }
  }

  const cells = sanitizers
    .map((entry, index) => buildScanCell(entry, index))
    .filter((entry): entry is ScanCell => entry !== null)
    .sort((left, right) => healthRank(left.health) - healthRank(right.health))

  if (cells.length === 0) {
    return { cells: [] as ScanCell[], headline: '', fallback: raw }
  }

  const score = readRecord(report.score)
  const grade = readString(score.grade) || readString(score.letter)
  const totalIssues = cells.filter((cell) => cell.health !== 'good').length
  const headline = grade
    ? `Grade ${grade} · ${totalIssues} areas need attention`
    : `${totalIssues} areas need attention`

  return { cells, headline, fallback: '' }
}

function tryParseSynapseSweep(raw: string) {
  try {
    return JSON.parse(raw)
  } catch {
    const start = raw.indexOf('{')
    const end = raw.lastIndexOf('}')
    if (start === -1 || end === -1 || end <= start) return null
    try {
      return JSON.parse(raw.slice(start, end + 1))
    } catch {
      return null
    }
  }
}

function buildScanCell(entry: unknown, index: number): ScanCell | null {
  const record = readRecord(entry)
  const tally = readRecord(record.tally)
  const name = readString(record.sanitizer) || readString(record.name) || `Check ${index + 1}`
  const errors = readNumber(tally.error) + readNumber(tally.errors)
  const warnings = readNumber(tally.warning) + readNumber(tally.warnings) + readNumber(tally.warn)
  const passing = readNumber(tally.ok) + readNumber(tally.pass) + readNumber(tally.passed)
  const infos = readNumber(tally.info)
  const detail = buildScanDetail(name, tally, record.issues)

  if (!detail && errors === 0 && warnings === 0 && passing === 0 && infos === 0) {
    return null
  }

  return {
    title: shortenLabel(name),
    summary:
      errors > 0
        ? `${errors} errors`
        : warnings > 0
          ? `${warnings} warnings`
          : `${Math.max(passing, infos, 1)} healthy checks`,
    health: errors > 0 ? 'unhealthy' : warnings > 0 ? 'warning' : 'good',
    detail: detail || `${name}\nNo detailed findings were reported.`,
  }
}

function buildScanDetail(name: string, tally: Record<string, unknown>, issues: unknown) {
  const parts = [name]
  const counters = [
    readNumber(tally.error) + readNumber(tally.errors) > 0
      ? `${readNumber(tally.error) + readNumber(tally.errors)} errors`
      : '',
    readNumber(tally.warning) + readNumber(tally.warnings) + readNumber(tally.warn) > 0
      ? `${readNumber(tally.warning) + readNumber(tally.warnings) + readNumber(tally.warn)} warnings`
      : '',
    readNumber(tally.ok) + readNumber(tally.pass) + readNumber(tally.passed) > 0
      ? `${readNumber(tally.ok) + readNumber(tally.pass) + readNumber(tally.passed)} passing`
      : '',
  ].filter(Boolean)

  if (counters.length > 0) {
    parts.push(counters.join(' · '))
  }

  const findings = flattenIssues(issues).slice(0, 3)
  if (findings.length > 0) {
    parts.push(findings.join('\n'))
  }

  return parts.join('\n')
}

function flattenIssues(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.flatMap((entry) => flattenIssues(entry))
  }

  const record = readRecord(value)
  const message = [readString(record.message), readString(record.msg), readString(record.title)]
    .filter(Boolean)
    .join(' — ')
  if (message) {
    return [message]
  }

  return Object.values(record).flatMap((entry) => flattenIssues(entry))
}

function readRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

function readString(value: unknown) {
  return typeof value === 'string' ? value.trim() : ''
}

function readNumber(value: unknown) {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : 0
  }
  return 0
}

function healthRank(value: ScanHealth) {
  if (value === 'unhealthy') return 0
  if (value === 'warning') return 1
  return 2
}

function shortenLabel(value: string) {
  return value.length > 28 ? `${value.slice(0, 25)}...` : value
}
</script>

<template>
  <div class="shell">
    <aside class="sidebar">
      <div class="brand">
        <div class="brand-mark">AC</div>
        <div>
          <h1>Argus Console</h1>
        </div>
      </div>

      <nav class="nav">
        <button
          v-for="tab in tabs"
          :key="tab.id"
          :class="['nav-link', { active: activeTab === tab.id }]"
          @click="activeTab = tab.id"
        >
          <span class="nav-eyebrow">{{ tab.eyebrow }}</span>
          <strong>{{ tab.label }}</strong>
        </button>
      </nav>

      <div class="sidebar-foot">
        <p class="meta-label">Runtime posture</p>
        <strong>{{ statusText }}</strong>
        <button class="ghost-btn" @click="loadWorkspace">Refresh workspace</button>
      </div>
    </aside>

    <main class="main-panel">
      <header class="hero">
        <div>
          <h2>{{ workspace?.summary.headline ?? 'Loading AI workspace…' }}</h2>
          <p>{{ workspace?.summary.subheadline ?? 'Pulling alerts, history, and recommendations.' }}</p>
        </div>

        <div class="connectivity-info">
          <div class="mcp-details">
            <div class="mcp-endpoint">{{ mcpEndpoint }}</div>
            <div class="mcp-cluster" @click="createClusterLabel">
              {{ activeClusterDisplay }}
            </div>
          </div>
          <div :class="['status-light', connectivityStatus]" :title="`Status: ${connectivityStatus}`"></div>
        </div>

        <div class="header-actions">
          <div class="sweep-controls">
            <div v-if="sweepInterval === 'custom'" class="custom-interval-input">
              <input v-model.number="customIntervalValue" type="number" min="1" class="interval-number">
              <select v-model="customIntervalUnit" class="interval-unit">
                <option value="m">min</option>
                <option value="h">hours</option>
                <option value="d">days</option>
              </select>
            </div>
            <select v-model="sweepInterval" class="sweep-select">
              <option value="manual">Manual</option>
              <option value="5m">5 min</option>
              <option value="30m">30 min</option>
              <option value="2h">2 hours</option>
              <option value="5h">5 hours</option>
              <option value="12h">12 hours</option>
              <option value="1d">1 day</option>
              <option value="custom">Custom</option>
            </select>
            <button class="primary-btn sweep-btn" @click="runSynapseSweep" :disabled="isRunningSweep">
              {{ isRunningSweep ? 'Sending…' : 'Send Sentinels' }}
            </button>
          </div>
        </div>

        <div v-if="countdownText" class="sweep-countdown-bar">
          {{ countdownText }}
        </div>
      </header>

      <section class="summary-grid">
        <article v-for="card in summaryCards" :key="card.label" :class="['summary-card', card.accent]">
          <span>{{ card.label }}</span>
          <strong>{{ card.value }}</strong>
        </article>
      </section>

      <section class="workspace">
        <div class="workspace-main">
          <div v-if="isLoading" class="loading-state">
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
            <section v-if="activeTab === 'arguskube'" class="panel-grid">
              <article :class="['panel', 'feature-panel', 'scrollable-panel', { expanded: isTopPriorityExpanded }]">
                <div class="panel-head sticky-head">
                  <div>
                    <p class="meta-label">Top priority</p>
                    <h3>{{ topPriority?.title ?? 'No urgent drift detected' }}</h3>
                    <button v-if="topPriority && isTopPriorityExpanded" class="ghost-btn mini-btn" @click="draftPrompt = topPriority.detail">
                      Open incident context
                    </button>
                  </div>
                  <div class="head-actions">
                    <span :class="['pill', topPriority?.severity ?? 'low']">{{ topPriority?.severity ?? 'low' }}</span>
                    <button class="ghost-btn expand-toggle" @click="isTopPriorityExpanded = !isTopPriorityExpanded">
                      {{ isTopPriorityExpanded ? '−' : '+' }}
                    </button>
                  </div>
                </div>

                <div v-show="isTopPriorityExpanded" class="panel-scroll-content">
                  <p class="feature-copy">
                    {{ topPriority?.detail ?? 'Arguskube Sentinels' }}
                  </p>

                  <div class="playbook-section">
                    <p class="meta-label playbook-label">Watchers Recommended Command Playbook</p>
                    <div class="playbook-grid mini">
                      <button
                        v-for="playbook in workspace?.playbooks"
                        :key="playbook.title"
                        class="playbook-card"
                        @click="usePlaybook(playbook)"
                      >
                        <span>{{ playbook.target }}</span>
                        <strong>{{ playbook.title }}</strong>
                        <p>{{ playbook.description }}</p>
                        <code>{{ playbook.commands[0] }}</code>
                      </button>
                    </div>
                  </div>
                </div>
              </article>

              <article :class="['panel', { expanded: isPriorityQueueExpanded }]">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Priority queue</p>
                    <h3>What Arguskube should tackle next</h3>
                  </div>
                  <button class="ghost-btn expand-toggle" @click="isPriorityQueueExpanded = !isPriorityQueueExpanded">
                    {{ isPriorityQueueExpanded ? '−' : '+' }}
                  </button>
                </div>
                <ul v-show="isPriorityQueueExpanded" class="stack-list">
                  <li v-for="priority in workspace?.priorities" :key="priority.title" class="stack-card">
                    <div class="stack-title">
                      <strong>{{ priority.title }}</strong>
                      <span :class="['pill', priority.severity]">{{ priority.severity }}</span>
                    </div>
                    <p>{{ priority.detail }}</p>
                  </li>
                </ul>
              </article>
            </section>

            <section v-else-if="activeTab === 'anomalies'" class="panel-grid">
              <article class="panel">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Live alerts</p>
                    <h3>Current anomaly candidates</h3>
                  </div>
                </div>
                <ul class="stack-list">
                  <li v-for="alert in alerts.slice(0, 6)" :key="alert.id" class="stack-card">
                    <div class="stack-title">
                      <strong>{{ alert.name }}</strong>
                      <span :class="['pill', alert.severity]">{{ alert.severity }}</span>
                    </div>
                    <p>{{ alert.message }}</p>
                    <small>{{ alert.namespace || 'cluster-wide' }} · {{ formatWhen(alert.receivedAt) }}</small>
                  </li>
                </ul>
              </article>

              <article class="panel">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Recommendations</p>
                    <h3>AI-assisted remediation</h3>
                  </div>
                </div>
                <ul class="stack-list">
                  <li v-for="recommendation in recommendations.slice(0, 5)" :key="recommendation.id" class="stack-card">
                    <div class="stack-title">
                      <strong>{{ recommendation.title }}</strong>
                      <span :class="['pill', recommendation.severity]">{{ recommendation.severity }}</span>
                    </div>
                    <p>{{ recommendation.summary }}</p>
                    <code v-if="recommendation.steps?.length">{{ recommendation.steps[0] }}</code>
                  </li>
                </ul>
              </article>

              <article class="panel span-wide">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Recent incident history</p>
                    <h3>Context the AI can cite back immediately</h3>
                  </div>
                </div>
                <div class="timeline">
                  <div v-for="incident in timeline" :key="incident.id" class="timeline-row">
                    <span class="timeline-time">{{ formatWhen(incident.timestamp) }}</span>
                    <div>
                      <strong>{{ incident.kind }} · {{ incident.name }}</strong>
                      <p>{{ incident.message }}</p>
                    </div>
                  </div>
                </div>
              </article>
            </section>

            <section v-else-if="activeTab === 'deploy'" class="panel-grid">
              <article :class="['panel', 'span-wide', { expanded: isDeploymentTracksExpanded }]">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Deployment tracks</p>
                    <h3>Local compose, minikube, and cluster-ready anomaly paths</h3>
                  </div>
                  <div class="head-actions">
                    <button
                      class="ghost-btn expand-toggle"
                      @click="isDeploymentTracksExpanded = !isDeploymentTracksExpanded"
                    >
                      {{ isDeploymentTracksExpanded ? '−' : '+' }}
                    </button>
                  </div>
                </div>
                <div v-show="isDeploymentTracksExpanded" class="deploy-grid">
                  <article v-for="plan in workspace?.deploymentPlans" :key="plan.profile" class="deploy-card">
                    <div class="deploy-head">
                      <div>
                        <span>{{ plan.mode }}</span>
                        <h4>{{ plan.title }}</h4>
                      </div>
                      <strong>{{ plan.namespace }}</strong>
                    </div>
                    <p>{{ plan.summary }}</p>

                    <div class="detail-group">
                      <label>Commands</label>
                      <CommandBlock v-for="command in plan.commands" :key="command" :command="command" />
                    </div>

                    <div class="detail-group">
                      <label>Validation</label>
                      <CommandBlock v-for="check in plan.validation" :key="check" :command="check" />
                    </div>

                    <div class="detail-group">
                      <label>Artifacts</label>
                      <CommandBlock v-for="artifact in plan.artifacts" :key="artifact" :command="artifact" />
                    </div>
                  </article>
                </div>
              </article>
            </section>

            <section v-else class="panel-grid">
              <article class="panel">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Services</p>
                    <h3>Platform runtime</h3>
                  </div>
                </div>
                <div class="service-grid">
                  <button
                    v-for="service in services"
                    :key="service.name"
                    class="service-card"
                    @click="toggleService(service)"
                    :disabled="isRefreshingServices"
                  >
                    <div class="stack-title">
                      <strong>{{ service.name }}</strong>
                      <span :class="['pill', service.status === 'running' ? 'low' : 'medium']">{{ service.status }}</span>
                    </div>
                    <small>{{ service.image || 'local image' }}</small>
                  </button>
                </div>
              </article>

              <article class="panel">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Logs</p>
                    <h3>Recent runtime events</h3>
                  </div>
                </div>
                <ul class="stack-list">
                  <li v-for="entry in runtimeLogs" :key="`${entry.timestamp}-${entry.message}`" class="stack-card">
                    <div class="stack-title">
                      <strong>{{ entry.source }}</strong>
                      <span :class="['pill', entry.level?.toLowerCase() ?? 'low']">{{ entry.level }}</span>
                    </div>
                    <p>{{ entry.message }}</p>
                  </li>
                </ul>
              </article>

              <article class="panel span-wide">
                <div class="panel-head">
                  <div>
                    <p class="meta-label">Synapse Sweep output</p>
                    <h3>Cluster scan payload</h3>
                  </div>
                </div>
                <div v-if="synapseSweepGrid.cells.length" class="scan-grid-wrap">
                  <p class="scan-grid-headline">{{ synapseSweepGrid.headline }}</p>
                  <div class="scan-grid">
                    <article
                      v-for="cell in synapseSweepGrid.cells"
                      :key="`${cell.title}-${cell.summary}`"
                      :class="['scan-cell', cell.health]"
                    >
                      <div class="scan-cell-copy">
                        <strong>{{ cell.title }}</strong>
                        <p>{{ cell.summary }}</p>
                      </div>
                      <span class="scan-tooltip">{{ cell.detail }}</span>
                    </article>
                  </div>
                </div>
                <CommandBlock v-if="synapseSweepOutput" :command="synapseSweepOutput" />
                <pre v-else class="terminal-output">Run a scan to populate cluster diagnostics here.</pre>
              </article>
            </section>
          </template>
        </div>

        <aside :class="['arguskube-panel', { expanded: isArguskubeExpanded }]">
      <div class="panel-head">
        <div>
          <p class="meta-label">Arguskube</p>
          <h3>Arguskube is always on deck</h3>
        </div>
        <button class="ghost-btn expand-toggle" @click="isArguskubeExpanded = !isArguskubeExpanded">
          {{ isArguskubeExpanded ? '−' : '+' }}
        </button>
      </div>

          <div v-show="isArguskubeExpanded" class="prompt-stack">
            <template v-if="workspace && workspace.playbooks && workspace.playbooks.length">
              <button
                v-for="playbook in workspace.playbooks"
                :key="playbook.title"
                class="prompt-chip"
                @click="usePlaybook(playbook)"
              >
                {{ playbook.title }}
              </button>
            </template>
            <template v-else>
              <button class="prompt-chip" @click="draftPrompt = 'Summarize the blast radius and safest next remediation step.'">
                Summarize blast radius
              </button>
              <button class="prompt-chip" @click="draftPrompt = 'Prepare the minikube anomaly rollout and list the first validation checks.'">
                Prep minikube rollout
              </button>
              <button class="prompt-chip" @click="draftPrompt = 'Turn recent incidents into an operator handoff note.'">
                Create handoff note
              </button>
            </template>
          </div>

          <div class="chat-thread">
            <template v-if="isArguskubeExpanded">
              <article
                v-for="message in chatMessages"
                :key="`${message.role}-${message.content}`"
                :class="['chat-bubble', message.role]"
              >
                <span>{{ message.role === 'assistant' ? 'AI' : 'You' }}</span>
                <p>{{ message.content }}</p>
              </article>
            </template>
            <template v-else-if="chatMessages.length">
              <article
                class="chat-bubble assistant"
              >
                <span>AI</span>
                <p>{{ chatMessages[chatMessages.length - 1].content }}</p>
              </article>
            </template>

            <article v-if="isThinking" class="chat-bubble assistant thinking-bubble">
              <div class="arguskube-thinking mini">
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <path d="M12 2C6.477 2 2 6.477 2 12C2 17.523 6.477 22 12 22C17.523 22 22 17.523 22 12C22 6.477 17.523 2 12 2Z" stroke="currentColor" stroke-width="1" stroke-opacity="0.2"/>
                  <circle cx="9" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
                  <circle cx="15" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
                </svg>
              </div>
              <span>Arguskube is thinking…</span>
            </article>
          </div>

          <div class="composer">
            <textarea
              v-model="draftPrompt"
              rows="4"
              placeholder="Ask the Arguskube for rollout help, incident triage, or anomaly validation."
            />
            <button class="primary-btn" @click="sendPrompt">Send to Arguskube</button>
          </div>
        </aside>
      </section>
    </main>
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

.sweep-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sweep-select {
  background: #111;
  color: #888;
  border: 1px solid #333;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}

.custom-interval-input {
  display: flex;
  align-items: center;
  gap: 4px;
}

.interval-number {
  width: 50px;
  background: #111;
  color: #fff;
  border: 1px solid #333;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}

.interval-unit {
  background: #111;
  color: #888;
  border: 1px solid #333;
  padding: 4px 4px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
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

</style>
