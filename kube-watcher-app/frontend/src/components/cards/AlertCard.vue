<template>
  <li class="stack-card" @click="handleClick">
    <div class="stack-title">
      <strong>{{ alert.name }}</strong>
      <span :class="['pill', alert.severity]">{{ alert.severity }}</span>
    </div>
    <p>{{ alert.message }}</p>
    <small>{{ alert.namespace || 'cluster-wide' }} · {{ formatWhen(alert.receivedAt) }}</small>
  </li>
</template>

<script setup lang="ts">
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
</script>

<style scoped>
.stack-card {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  transition: all 0.2s;
  cursor: pointer;
}

.stack-card:hover {
  border-color: var(--border-active);
  background: var(--hover);
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

.stack-card p {
  margin: 8px 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.stack-card small {
  font-size: 12px;
  color: var(--text-tertiary);
  display: block;
}
</style>