<template>
  <li :class="['stack-card', podExistsBorder]" @click="handleClick">
    <div class="stack-title">
      <strong>{{ alert.name }}</strong>
      <div class="alert-badges">
         <span :class="['status-dot', alert.severity]"></span>
         <span class="severity-text">{{ alert.severity }}</span>
         <span v-if="stateLabel" class="state-indicator" :style="{ backgroundColor: stateColor }"></span>
         <div class="card-actions">
           <button class="card-action-btn" title="Investigate" @click.stop="handleInvestigate">
             <span class="action-icon">🔍</span>
           </button>
           <button class="card-action-btn" title="Dismiss" @click.stop="handleDismiss">
             <span class="action-icon">✓</span>
           </button>
           <button class="card-action-btn" title="Resolve" @click.stop="handleResolve">
             <span class="action-icon">✔</span>
           </button>
         </div>
       </div>
    </div>
    <div class="alert-info-icon" title="Message">i</div>
    <p class="alert-message">{{ alert.message }}</p>
    <small>{{ alert.namespace || 'cluster-wide' }} · {{ formatWhen(alert.receivedAt) }}</small>
  </li>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { data } from '../../../wailsjs/go/models'

interface Props {
  alert: data.AlertRecord
  formatWhen: (value: unknown) => string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  click: [alert: data.AlertRecord]
  investigate: [alert: data.AlertRecord]
  dismiss: [alert: data.AlertRecord]
  resolve: [alert: data.AlertRecord]
}>()

const handleClick = () => {
  emit('click', props.alert)
}

const handleInvestigate = () => {
  emit('investigate', props.alert)
}

const handleDismiss = () => {
  emit('dismiss', props.alert)
}

const handleResolve = () => {
  emit('resolve', props.alert)
}

const podExistsBorder = computed(() => {
  if (props.alert.podExists === true) return 'border-pod-exists'
  if (props.alert.podExists === false) return 'border-pod-gone'
  return ''
})

const stateLabel = computed(() => {
  const state = props.alert.state
  if (!state) return ''
  const labels: Record<string, string> = {
    new: 'New',
    acknowledged: 'Acknowledged',
    silenced: 'Silenced',
    being_investigated: 'Investigating',
    false_positive: 'False Positive',
    deleted: 'Deleted'
  }
  return labels[state] || state
})

const stateColor = computed(() => {
  const state = props.alert.state
  if (!state) return ''
  const colors: Record<string, string> = {
    new: 'var(--info)',
    acknowledged: 'var(--success)',
    silenced: 'var(--warning)',
    being_investigated: 'var(--primary)',
    false_positive: 'var(--error)',
    deleted: 'var(--text-tertiary)'
  }
  return colors[state] || 'var(--border)'
})
</script>

<style scoped>
.stack-card {
  padding: 22px;
  transition: all 0.2s;
  cursor: pointer;
  position: relative;
  border-bottom: 1px solid var(--border);
}

.stack-card:hover {
  background: rgba(255, 255, 255, 0.05);
}

.border-pod-exists {
  border-color: rgba(59, 130, 246, 0.6); /* blue */
}

.border-pod-gone {
  border-color: rgba(245, 158, 11, 0.6); /* yellow */
}

.stack-title {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 8px;
  gap: 8px;
}

.stack-title strong {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  flex: 1;
}

.status-dot {
  height: 8px;
  width: 8px;
  border-radius: 50%;
  display: inline-block;
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

.severity-text {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin-left: 4px;
}

.state-indicator {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-left: 8px;
}

.alert-badges {
  display: flex;
  gap: 8px;
  align-items: center;
  position: relative;
}

.card-actions {
  display: flex;
  gap: 4px;
  opacity: 0;
  transition: opacity 0.2s ease;
}

.stack-card:hover .card-actions {
  opacity: 1;
}

.card-action-btn {
  padding: 2px 6px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: rgba(255, 255, 255, 0.1);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}

.card-action-btn:hover {
  background: rgba(255, 255, 255, 0.2);
  border-color: var(--border-active);
}

.action-icon {
  font-size: 10px;
}



.alert-message {
  margin: 8px 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
  opacity: 0;
  transition: opacity 0.2s;
}

.alert-info-icon {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: var(--text-secondary);
  color: var(--card-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 12px;
  font-weight: bold;
  cursor: help;
  transition: background 0.2s;
  position: absolute;
  top: 16px;
  right: 16px;
  z-index: 1;
}

.alert-info-icon:hover {
  background: var(--text-primary);
}

.alert-info-icon:hover ~ .alert-message,
.alert-message:hover {
  opacity: 1;
}



.stack-card small {
  font-size: 12px;
  color: var(--text-tertiary);
  display: block;
}
</style>