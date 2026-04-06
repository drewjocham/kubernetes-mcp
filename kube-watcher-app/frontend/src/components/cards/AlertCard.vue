<template>
  <li :class="['stack-card', podExistsBorder]" @click="handleClick">
    <div class="stack-title">
      <strong>{{ alert.name }}</strong>
      <div class="alert-badges">
        <span :class="['pill', alert.severity]">{{ alert.severity }}</span>
        <span v-if="stateLabel" class="state-badge" :style="{ backgroundColor: stateColor }">
          {{ stateLabel }}
        </span>
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
}>()

const handleClick = () => {
  emit('click', props.alert)
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
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  transition: all 0.2s;
  cursor: pointer;
  position: relative;
}

.stack-card:hover {
  border-color: var(--border-active);
  background: var(--hover);
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

.pill {
  padding: 4px 8px;
  border-radius: 20px;
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  white-space: nowrap;
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

.alert-badges {
  display: flex;
  gap: 6px;
  align-items: center;
}

.state-badge {
  padding: 2px 6px;
  border-radius: 10px;
  font-size: 10px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: white;
  white-space: nowrap;
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