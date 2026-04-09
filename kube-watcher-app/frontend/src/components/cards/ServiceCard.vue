<template>
  <button
    class="service-card"
    @click="emit('toggle')"
    :disabled="disabled"
  >
    <div class="stack-title">
      <strong>{{ service.name }}</strong>
      <span :class="['pill', service.status === 'running' ? 'low' : 'medium']">{{ service.status }}</span>
    </div>
    <small>{{ service.image || 'local image' }}</small>
  </button>
</template>

<script setup lang="ts">
import { data } from '../../../wailsjs/go/models'

interface Props {
  service: data.ServiceStatus
  disabled: boolean
}

defineProps<Props>()
const emit = defineEmits<{
  toggle: []
}>()
</script>

<style scoped>
.service-card {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  text-align: left;
  cursor: pointer;
  transition: all 0.2s;
}

.service-card:hover:not(:disabled) {
  border-color: var(--border-active);
  background: var(--hover);
  transform: translateY(-2px);
}

.service-card:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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

.pill.low {
  background: rgba(34, 197, 94, 0.1);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.pill.medium {
  background: rgba(245, 158, 11, 0.1);
  color: var(--warning);
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.service-card small {
  font-size: 12px;
  color: var(--text-tertiary);
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>