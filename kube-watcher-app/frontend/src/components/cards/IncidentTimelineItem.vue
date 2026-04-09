<template>
  <div class="timeline-row">
    <span class="timeline-time">{{ formatWhen(incident.timestamp) }}</span>
    <div>
      <strong>{{ incident.kind }} · {{ incident.name }}</strong>
      <p>{{ incident.message }}</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { data } from '../../../wailsjs/go/models'

interface Props {
  incident: data.Incident
  formatWhen: (value: unknown) => string
}

defineProps<Props>()
</script>

<style scoped>
.timeline-row {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 16px;
  padding: 12px 0;
  border-bottom: 1px solid var(--border);
}

.timeline-row:last-child {
  border-bottom: none;
}

.timeline-time {
  font-size: 14px;
  color: var(--text-tertiary);
  font-family: 'SF Mono', monospace;
}

.timeline-row strong {
  display: block;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.timeline-row p {
  margin: 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
}

@media (max-width: 768px) {
  .timeline-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
  
  .timeline-time {
    font-size: 12px;
  }
}
</style>