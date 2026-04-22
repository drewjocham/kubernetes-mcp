<template>
  <section class="live-alerts-panel">
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

      <div class="table-container">
        <table class="argus-table">
          <thead>
            <tr>
              <th class="col-status">Status</th>
              <th class="col-name">Name</th>
              <th class="col-source">Source</th>
              <th class="col-time">Timestamp</th>
              <th class="col-actions">Actions</th>
            </tr>
          </thead>
          <tbody>
              <tr
                v-for="(alert, index) in filteredAlerts.slice(0, 10)"
                :key="alert?.id || index"
                :class="['table-row', getAlertClass(alert)]"
                @click="handleAlertClick(alert)"
              >
               <td class="col-status" @click.stop="startEditState(alert, $event)">
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
              <td class="col-name">
                <div class="alert-name">{{ alert.name }}</div>
                <div class="alert-message-truncated">{{ alert.message }}</div>
              </td>
              <td class="col-source">{{ alert.namespace || 'cluster-wide' }}</td>
              <td class="col-time">{{ formatWhen(alert.receivedAt) }}</td>
              <td class="col-actions">
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
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { data } from '../../../wailsjs/go/models'
import { PhLink, PhBrain, PhMagnifyingGlass, PhCheck, PhCheckCircle } from '@phosphor-icons/vue'

interface Props {
  alerts: data.AlertRecord[]
  anomstackConnected: boolean
  signozConnected: boolean
  isConnectingAnomstack: boolean
  isInvestigating: boolean
  formatWhen: (value: unknown) => string
  selectedSeverity?: string
  selectedState?: string
}

const props = withDefaults(defineProps<Props>(), {
  alerts: () => [],
  selectedSeverity: 'all',
  selectedState: 'all'
})

const severityOptions = [
  { value: 'all', label: 'All', icon: '🌐' },
  { value: 'critical', label: 'Critical', icon: '🔥' },
  { value: 'error', label: 'Error', icon: '❌' },
  { value: 'warning', label: 'Warning', icon: '⚠️' },
  { value: 'info', label: 'Info', icon: 'ℹ️' },
  { value: 'low', label: 'Low', icon: '📉' }
]

const stateOptions = [
  { value: 'all', label: 'All States', icon: '🌐' },
  { value: 'new', label: 'New', icon: '🆕' },
  { value: 'acknowledged', label: 'Acknowledged', icon: '✅' },
  { value: 'silenced', label: 'Silenced', icon: '🔇' },
  { value: 'being_investigated', label: 'Being Investigated', icon: '🔍' },
  { value: 'false_positive', label: 'False Positive', icon: '👻' },
  { value: 'deleted', label: 'Deleted', icon: '🗑️' }
]

const editingStateId = ref<string | null>(null)

const filteredAlerts = computed(() => {
  let filtered = (props.alerts ?? []).filter(Boolean)
  
  if (props.selectedSeverity !== 'all') {
    filtered = filtered.filter(alert => alert.severity === props.selectedSeverity)
  }
  
  if (props.selectedState !== 'all') {
    filtered = filtered.filter(alert => alert.state === props.selectedState)
  }
  
  return filtered
})

function getAlertClass(alert: data.AlertRecord): string {
  if (!alert) return ''
  // Heuristic: if anomalyScore exists and is > 0, it's an anomaly (blue)
  // Otherwise, it's a user-set alert (purple)
  const isAnomaly = alert.anomalyScore != null && alert.anomalyScore > 0
  return isAnomaly ? 'alert-type-anomaly' : 'alert-type-user'
}

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

function handleAlertClick(alert: data.AlertRecord) {
  emit('alert-clicked', alert)
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
</script>

<style scoped>
.live-alerts-panel {
  width: 100%;
}

.panel {
  background: var(--panel-bg);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  padding: 28px;
  box-shadow: var(--shadow);
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  overflow: hidden;
  margin-bottom: 24px;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.panel-head h3 {
  margin: 4px 0 0 0;
  font-size: 20px;
  font-weight: 600;
}

.meta-label {
  color: var(--text-muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0;
}

.panel-actions {
  display: flex;
  gap: 8px;
}

.select-input {
  background: var(--input-bg);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text);
  padding: 8px 12px;
  font-size: 14px;
  min-width: 120px;
}

.global-action-bar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 24px;
  padding-bottom: 16px;
  border-bottom: 1px solid var(--border);
}

.glass-btn {
  background: var(--surface-muted);
  border: 1px solid var(--border);
  border-radius: var(--radius-sm);
  color: var(--text);
  padding: 10px 16px;
  font-size: 14px;
  display: flex;
  align-items: center;
  gap: 8px;
  transition: all 0.2s ease;
  min-width: 140px;
  justify-content: center;
}

.glass-btn:hover:not(:disabled) {
  background: var(--surface);
  border-color: var(--border-active);
}

.glass-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.connect-btn {
  background: rgba(var(--primary-rgb), 0.1);
  border-color: rgba(var(--primary-rgb), 0.3);
}

.investigate-btn {
  background: rgba(var(--success-rgb), 0.1);
  border-color: rgba(var(--success-rgb), 0.3);
}

.btn-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text-muted);
  font-size: 14px;
}

.status-dot {
  display: inline-block;
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

.signoz-status {
  display: flex;
  align-items: center;
}

.table-container {
  overflow-x: auto;
  overflow-y: auto;
  max-width: 100%;
  max-height: 400px;
}

.argus-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  min-width: 700px;
}

.argus-table th {
  text-align: left;
  padding: 12px 16px;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 14px;
  border-bottom: 1px solid var(--border);
}

.argus-table tr {
  border-bottom: 1px solid var(--border);
}

.argus-table tr:nth-child(even) {
  background: var(--surface-muted);
}

.argus-table tr:hover {
  background: var(--hover);
}

.argus-table td {
  padding: 16px;
  font-size: 14px;
}



.table-row {
  cursor: pointer;
}

.table-row:hover .row-actions {
  opacity: 1;
}

.alert-name {
  font-weight: 500;
  margin-bottom: 4px;
}

.alert-message-truncated {
  color: var(--text-muted);
  font-size: 13px;
  line-height: 1.4;
  max-width: 100%;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.table-container {
  overflow-x: auto;
  overflow-y: auto;
  max-width: 100%;
  max-height: 400px;
}

.argus-table {
  table-layout: fixed;
  min-width: 500px;
}

.col-status {
  width: 100px;
  text-align: center;
}

.col-name {
  width: 40%;
  min-width: 150px;
}

.col-source {
  width: 20%;
  min-width: 80px;
}

.col-time {
  width: 20%;
  min-width: 80px;
}

.col-actions {
  width: 110px;
  text-align: center;
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

.row-actions {
  display: flex;
  gap: 4px;
  opacity: 0.7;
  transition: opacity 0.2s ease;
}

.row-action-btn {
  background: var(--surface-muted);
  border: 1px solid var(--border);
  border-radius: 6px;
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text);
  transition: all 0.2s ease;
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

 .alert-type-anomaly td:first-child {
   border-left: 4px solid var(--info);
   padding-left: 12px;
 }
 .alert-type-anomaly {
   background: linear-gradient(90deg, rgba(var(--info-rgb), 0.05) 0%, transparent 100%);
 }

 .alert-type-user td:first-child {
   border-left: 4px solid var(--primary);
   padding-left: 12px;
 }
 .alert-type-user {
   background: linear-gradient(90deg, rgba(var(--primary-rgb), 0.05) 0%, transparent 100%);
 }

 @media (max-width: 1024px) {
  .panel {
    padding: 16px;
  }
  
  .argus-table {
    min-width: 450px;
  }
  
  .col-name {
    min-width: 150px;
  }
  
  .col-source, .col-time {
    min-width: 70px;
  }
}

@media (max-width: 768px) {
  .panel-head {
    flex-direction: column;
    gap: 12px;
    align-items: stretch;
  }
  
  .panel-actions {
    justify-content: flex-start;
  }
  
  .global-action-bar {
    flex-wrap: wrap;
    gap: 8px;
  }
  
  .glass-btn {
    min-width: 120px;
  }
  
  .argus-table {
    min-width: 400px;
  }
  
  .col-name {
    min-width: 120px;
  }
  
  .col-source, .col-time {
    min-width: 60px;
  }
  
  .col-actions {
    width: 100px;
  }
  
  .row-action-btn {
    width: 28px;
    height: 28px;
  }
}
</style>