<template>
  <section class="anomalies-panel">
    <div class="panel-grid">
      <!-- Live Alerts Panel -->
      <article class="panel">
        <div class="panel-head">
          <div>
            <p class="meta-label">Live alerts</p>
            <h3>Current anomaly candidates</h3>
          </div>
          <div class="panel-actions">
            <select 
              :value="selectedSeverity" 
              @change="emit('severity-change', ($event.target as HTMLSelectElement).value)"
              class="select-input"
            >
              <option v-for="option in severityOptions" :value="option.value">
                {{ option.icon }} {{ option.label }}
              </option>
            </select>
          </div>
        </div>
        <div class="global-action-bar">
          <button
            v-if="!anomstackConnected"
            class="glass-btn connect-btn"
            @click="emit('connect-anomstack')"
            :disabled="isConnectingAnomstack"
          >
            <PhLink size="16" weight="bold" class="btn-icon" />
            {{ isConnectingAnomstack ? 'Connecting...' : 'Connect Anomalies' }}
          </button>
          <div v-if="anomstackConnected" class="connection-status">
            <span class="status-dot green"></span>
            <span class="status-text">Connected</span>
          </div>
          <div v-if="signozConnected" class="signoz-status">
            <span class="status-dot blue"></span>
          </div>
          <button
            v-if="alerts.length > 0"
            class="glass-btn investigate-btn"
            @click="emit('investigate-anomalies')"
            :disabled="isInvestigating"
          >
            <PhBrain size="16" weight="bold" class="btn-icon" />
            {{ isInvestigating ? 'Investigating...' : 'Investigate with AI' }}
          </button>
        </div>

        <table class="argus-table">
          <thead>
            <tr>
              <th>Status</th>
              <th>Name</th>
              <th>Source</th>
              <th>Timestamp</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="alert in filteredAlerts.slice(0, 10)"
              :key="alert.id"
              class="table-row"
              @click="handleAlertClick(alert)"
            >
              <td @click.stop="startEditState(alert, $event)">
                <template v-if="editingStateId === alert.id">
                  <select 
                    class="state-select"
                    :value="alert.state || 'new'"
                    @change="changeState(alert, ($event.target as HTMLSelectElement).value)"
                    @blur="cancelEditState"
                    @keydown.enter="changeState(alert, ($event.target as HTMLSelectElement).value)"
                    @keydown.esc="cancelEditState"
                    autofocus
                  >
                    <option v-for="option in stateOptions" :value="option.value">
                      {{ option.icon }} {{ option.label }}
                    </option>
                  </select>
                </template>
                <template v-else>
                  <span class="state-badge" :class="alert.state || 'new'">
                    {{ alert.state || 'new' }}
                  </span>
                </template>
              </td>
              <td>
                <div class="alert-name">{{ alert.name }}</div>
                <div class="alert-message-truncated">{{ alert.message }}</div>
              </td>
              <td>{{ alert.namespace || 'cluster-wide' }}</td>
              <td>{{ formatWhen(alert.receivedAt) }}</td>
              <td>
                <div class="row-actions">
                   <button class="row-action-btn" title="Investigate" @click.stop="handleInvestigateRow(alert)">
                     <PhMagnifyingGlass size="12" weight="bold" class="action-icon" />
                   </button>
                   <button class="row-action-btn" title="Dismiss" @click.stop="handleDismissRow(alert)">
                     <PhCheck size="12" weight="bold" class="action-icon" />
                   </button>
                   <button class="row-action-btn" title="Resolve" @click.stop="handleResolveRow(alert)">
                     <PhCheckCircle size="12" weight="bold" class="action-icon" />
                   </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </article>

      <!-- Recommendations Panel -->
      <article class="panel">
        <div class="panel-head">
          <div>
            <p class="meta-label">Recommendations</p>
            <h3>AI-assisted remediation</h3>
          </div>
        </div>
        <ul class="stack-list">
          <RecommendationCard
            v-for="recommendation in recommendations.slice(0, 5)"
            :key="recommendation.id"
            :recommendation="recommendation"
          />
        </ul>
      </article>

      <!-- Incident History Panel -->
      <IncidentHistoryTabs
        :timeline="timeline"
        :format-when="formatWhen"
        :selected-severity="selectedSeverity"
        :selected-state="selectedState"
        :selected-time-range="selectedTimeRange"
        @clear-filters="handleClearFilters"
      />
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AlertCard from '../cards/AlertCard.vue'
import RecommendationCard from '../cards/RecommendationCard.vue'
import IncidentTimelineItem from '../cards/IncidentTimelineItem.vue'
import IncidentHistoryTabs from './IncidentHistoryTabs.vue'
import { data } from '../../../wailsjs/go/models'
import { PhLink, PhBrain, PhMagnifyingGlass, PhGlobe, PhNewspaper, PhCheckCircle, PhSpeakerSlash, PhGhost, PhTrash, PhFire, PhX, PhWarning, PhInfo, PhChartLineDown } from '@phosphor-icons/vue'

interface Props {
  alerts: data.AlertRecord[]
  recommendations: data.Recommendation[]
  timeline: data.Incident[]
  anomstackConnected: boolean
  signozConnected: boolean
  isConnectingAnomstack: boolean
  isInvestigating: boolean
  formatWhen: (value: unknown) => string
  selectedSeverity?: string
  selectedState?: string
  selectedTimeRange?: string
}

const props = withDefaults(defineProps<Props>(), {
  selectedSeverity: 'all',
  selectedState: 'all',
  selectedTimeRange: '24h'
})

const stateOptions = [
  { value: 'all', label: 'All States', icon: '🌐' },
  { value: 'new', label: 'New', icon: '🆕' },
  { value: 'acknowledged', label: 'Acknowledged', icon: '✅' },
  { value: 'silenced', label: 'Silenced', icon: '🔇' },
  { value: 'being_investigated', label: 'Being Investigated', icon: '🔍' },
  { value: 'false_positive', label: 'False Positive', icon: '👻' },
  { value: 'deleted', label: 'Deleted', icon: '🗑️' }
]

const severityOptions = [
  { value: 'all', label: 'All', icon: '🌐' },
  { value: 'critical', label: 'Critical', icon: '🔥' },
  { value: 'error', label: 'Error', icon: '❌' },
  { value: 'warning', label: 'Warning', icon: '⚠️' },
  { value: 'info', label: 'Info', icon: 'ℹ️' },
  { value: 'low', label: 'Low', icon: '📉' }
]

const filteredAlerts = computed(() => {
  let filtered = props.alerts
  
  if (props.selectedSeverity !== 'all') {
    filtered = filtered.filter(alert => alert.severity === props.selectedSeverity)
  }
  
  if (props.selectedState !== 'all') {
    filtered = filtered.filter(alert => alert.state === props.selectedState)
  }
  
  return filtered
})

console.log('AnomaliesPanel props:', props)
onMounted(() => console.log('AnomaliesPanel mounted'))
const emit = defineEmits<{
  'connect-anomstack': []
  'investigate-anomalies': []
  'alert-clicked': [alert: data.AlertRecord]
  'clear-filters': []
  'severity-change': [value: string]
  'investigate-row': [alert: data.AlertRecord]
  'dismiss-row': [alert: data.AlertRecord]
  'resolve-row': [alert: data.AlertRecord]
  'state-change': [alert: data.AlertRecord, newState: string]
}>()

const editingStateId = ref<string | null>(null)

function handleAlertClick(alert: data.AlertRecord) {
  emit('alert-clicked', alert)
}

function startEditState(alert: data.AlertRecord, event: Event) {
  event.stopPropagation()
  editingStateId.value = alert.id
}

function cancelEditState() {
  editingStateId.value = null
}

function changeState(alert: data.AlertRecord, newState: string) {
  editingStateId.value = null
  emit('state-change', alert, newState)
}

function handleClearFilters() {
  emit('clear-filters')
}

function handleInvestigateRow(alert: data.AlertRecord) {
  emit('investigate-row', alert)
}

function handleDismissRow(alert: data.AlertRecord) {
  emit('dismiss-row', alert)
}

function handleResolveRow(alert: data.AlertRecord) {
  emit('resolve-row', alert)
}
</script>

<style scoped>
.anomalies-panel {
  width: 100%;
}

.panel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  min-width: 0;
}

.panel {
  padding: 24px;
  background: var(--glass-card-surface);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: var(--glass-border-rim);
  border-radius: var(--radius-lg);
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

.panel-actions {
  display: flex;
  gap: 12px;
  align-items: center;
  flex-wrap: wrap;
}

.global-action-bar {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: flex-end;
  margin-bottom: 20px;
}

.connection-status,
.signoz-status {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.green {
  background: var(--success);
}

.status-dot.blue {
  background: var(--info);
}

.status-text {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}



.filter-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 11px;
  align-items: center;
}

.filter-chip {
  padding: 11px 20px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

.filter-chip:hover {
  color: var(--text);
}

.filter-chip.active {
  color: var(--primary);
  font-weight: 600;
}

.filter-icon {
  font-size: 12px;
  line-height: 1;
}

.filter-text {
  flex: 1;
  text-align: left;
}

.stack-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.timeline {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.primary-btn {
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

.primary-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.2);
  border-color: var(--border-active);
  transform: translateY(-1px);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.connect-btn {
  background: rgba(var(--primary-rgb), 0.1);
  border-color: rgba(var(--primary-rgb), 0.3);
}

.connect-btn:hover:not(:disabled) {
  background: rgba(var(--primary-rgb), 0.2);
  border-color: rgba(var(--primary-rgb), 0.5);
}

.investigate-btn {
  background: var(--accent);
  color: white;
  border: none;
  box-shadow: 0 4px 12px rgba(var(--teal-rgb), 0.2);
}

.investigate-btn:hover:not(:disabled) {
  background: var(--accent);
  filter: brightness(1.1);
  box-shadow: 0 6px 16px rgba(var(--teal-rgb), 0.3);
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

@media (max-width: 768px) {
  .filter-section {
    flex-direction: column;
  }
}

@media (max-width: 1024px) {
  .panel-grid {
    grid-template-columns: 1fr;
  }
  
  .panel-head {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
  
  .panel-actions {
    justify-content: flex-start;
  }
  
  .filter-chips {
    gap: 6px;
  }
}

@media (max-width: 640px) {
  .filter-section {
    gap: 16px;
  }
  
  .filter-chips {
    gap: 4px;
  }
  
  .filter-chip {
    padding: 6px 10px;
    font-size: 10px;
  }
  
  .filter-icon {
    font-size: 10px;
  }
}

.glass-btn {
  padding: 8px 16px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--surface-muted);
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
  min-width: 140px;
  justify-content: center;
}

.glass-btn:hover:not(:disabled) {
  background: var(--surface);
  border-color: var(--border-active);
  transform: translateY(-1px);
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.connect-btn {
  background: rgba(var(--primary-rgb), 0.1);
  border-color: rgba(var(--primary-rgb), 0.3);
}

.connect-btn:hover:not(:disabled) {
  background: rgba(var(--primary-rgb), 0.2);
  border-color: rgba(var(--primary-rgb), 0.5);
}

.investigate-btn {
  background: var(--accent);
  color: white;
  border: none;
  box-shadow: 0 4px 12px rgba(var(--teal-rgb), 0.2);
}

.investigate-btn:hover:not(:disabled) {
  background: var(--accent);
  filter: brightness(1.1);
  box-shadow: 0 6px 16px rgba(var(--teal-rgb), 0.3);
}

.btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

.argus-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 10px;
}

.argus-table th {
  text-align: left;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  padding: 12px 16px;
  border-bottom: 1px solid rgba(0,0,0,0.05);
}

.argus-table tr {
  transition: background 0.2s ease;
}

.argus-table tr:nth-child(even) {
  background: rgba(0, 0, 0, 0.02);
}

.argus-table tr:hover {
  background: rgba(0, 0, 0, 0.03);
  cursor: pointer;
}

.argus-table td {
  padding: 14px 16px;
  font-size: 13px;
  border-bottom: 1px solid rgba(0,0,0,0.02);
}

.status-dot {
  height: 8px;
  width: 8px;
  border-radius: 50%;
  display: inline-block;
  margin-right: 8px;
}

.status-dot.critical,
.status-dot.error {
  background: var(--danger);
}

.status-dot.warning {
  background: var(--warning);
}

.status-dot.info,
.status-dot.low {
  background: var(--info);
}

.alert-name {
  font-weight: 600;
  color: var(--text-primary);
}

.alert-message-truncated {
  font-size: 12px;
  color: var(--text-secondary);
  margin-top: 4px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 200px;
}

.row-actions {
  display: flex;
  gap: 8px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.table-row:hover .row-actions {
  opacity: 1;
}

.row-action-btn {
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: var(--surface-muted);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.row-action-btn:hover {
  background: var(--surface);
  border-color: var(--border-active);
}

.action-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

.argus-table td:last-child {
  width: 1%;
  white-space: nowrap;
}

.row-actions {
  flex-wrap: nowrap;
  justify-content: flex-end;
}

.state-badge {
  padding: 4px 10px;
  border-radius: 12px;
  font-size: 11px;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  background: var(--surface-muted);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
}

.state-badge:hover {
  background: var(--surface);
  color: var(--text-primary);
}

.state-select {
  padding: 4px 8px;
  border-radius: 8px;
  border: 1px solid var(--border-active);
  background: var(--surface);
  color: var(--text-primary);
  font-size: 11px;
  font-weight: 500;
  cursor: pointer;
  outline: none;
  min-width: 120px;
}
</style>