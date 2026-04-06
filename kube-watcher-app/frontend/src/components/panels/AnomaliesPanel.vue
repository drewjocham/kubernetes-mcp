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
            <button
              v-if="!anomstackConnected"
              class="primary-btn connect-btn"
              @click="emit('connect-anomstack')"
              :disabled="isConnectingAnomstack"
            >
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
              class="primary-btn investigate-btn"
              @click="emit('investigate-anomalies')"
              :disabled="isInvestigating"
            >
              {{ isInvestigating ? 'Investigating...' : 'Investigate with AI' }}
            </button>
          </div>
        </div>
        <div class="filter-bar">
          <button
            v-for="severity in ['all', 'critical', 'error', 'warning', 'info', 'low']"
            :key="severity"
            :class="['filter-pill', { active: selectedSeverity === severity }]"
            @click="selectedSeverity = severity"
          >
            {{ severity }}
          </button>
        </div>
        <div class="filter-bar">
          <button
            v-for="opt in stateOptions"
            :key="opt.value"
            :class="['filter-pill', { active: selectedState === opt.value }]"
            @click="selectedState = opt.value"
          >
            {{ opt.label }}
          </button>
        </div>
        <ul class="stack-list">
          <AlertCard
            v-for="alert in filteredAlerts.slice(0, 6)"
            :key="alert.id"
            :alert="alert"
            :format-when="formatWhen"
            @click="handleAlertClick(alert)"
          />
        </ul>
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
      <article class="panel span-wide">
        <div class="panel-head">
          <div>
            <p class="meta-label">Recent incident history</p>
            <h3>Context the AI can cite back immediately</h3>
          </div>
        </div>
        <div class="timeline">
          <IncidentTimelineItem
            v-for="incident in timeline"
            :key="incident.id"
            :incident="incident"
            :format-when="formatWhen"
          />
        </div>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import AlertCard from '../cards/AlertCard.vue'
import RecommendationCard from '../cards/RecommendationCard.vue'
import IncidentTimelineItem from '../cards/IncidentTimelineItem.vue'
import { data } from '../../../wailsjs/go/models'

interface Props {
  alerts: data.AlertRecord[]
  recommendations: data.Recommendation[]
  timeline: data.Incident[]
  anomstackConnected: boolean
  signozConnected: boolean
  isConnectingAnomstack: boolean
  isInvestigating: boolean
  formatWhen: (value: unknown) => string
}

const props = defineProps<Props>()
const selectedSeverity = ref<string>('all')
const selectedState = ref<string>('all')

const stateOptions = [
  { value: 'all', label: 'All States' },
  { value: 'new', label: 'New' },
  { value: 'acknowledged', label: 'Acknowledged' },
  { value: 'silenced', label: 'Silenced' },
  { value: 'being_investigated', label: 'Being Investigated' },
  { value: 'false_positive', label: 'False Positive' },
  { value: 'deleted', label: 'Deleted' }
]

const filteredAlerts = computed(() => {
  let filtered = props.alerts
  
  if (selectedSeverity.value !== 'all') {
    filtered = filtered.filter(alert => alert.severity === selectedSeverity.value)
  }
  
  if (selectedState.value !== 'all') {
    filtered = filtered.filter(alert => alert.state === selectedState.value)
  }
  
  return filtered
})

console.log('AnomaliesPanel props:', props)
onMounted(() => console.log('AnomaliesPanel mounted'))
const emit = defineEmits<{
  'connect-anomstack': []
  'investigate-anomalies': []
  'alert-clicked': [alert: data.AlertRecord]
}>()

function handleAlertClick(alert: data.AlertRecord) {
  emit('alert-clicked', alert)
}
</script>

<style scoped>
.anomalies-panel {
  width: 100%;
}

.panel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.panel {
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
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

.filter-bar {
  display: flex;
  gap: 8px;
  margin-bottom: 16px;
  flex-wrap: wrap;
}

.filter-pill {
  padding: 6px 12px;
  border-radius: 20px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  text-transform: capitalize;
}

.filter-pill:hover {
  border-color: var(--border-active);
  background: var(--hover);
}

.filter-pill.active {
  border-color: var(--primary);
  background: var(--primary);
  color: white;
}

.stack-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
}

.timeline {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.primary-btn {
  padding: 8px 16px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
  white-space: nowrap;
}

.primary-btn:hover:not(:disabled) {
  background: var(--primary-hover);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.connect-btn {
  background: var(--info);
}

.connect-btn:hover:not(:disabled) {
  background: var(--info-hover);
}

.investigate-btn {
  background: var(--success);
}

.investigate-btn:hover:not(:disabled) {
  background: var(--success-hover);
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
}
</style>