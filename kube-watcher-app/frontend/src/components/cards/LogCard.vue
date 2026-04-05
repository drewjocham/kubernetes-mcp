<template>
  <li class="stack-card">
    <div class="stack-title">
      <strong>{{ log.source }}</strong>
      <span :class="['pill', log.level?.toLowerCase() ?? 'low']">{{ log.level }}</span>
    </div>
    <p>{{ log.message }}</p>
  </li>
</template>

<script setup lang="ts">
import { data } from '../../../wailsjs/go/models'

interface Props {
  log: data.LogLine
}

defineProps<Props>()
</script>

<style scoped>
.stack-card {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  transition: all 0.2s;
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

.pill.error,
.pill.critical {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
  border: 1px solid rgba(239, 68, 68, 0.2);
}

.pill.warning,
.pill.warn {
  background: rgba(245, 158, 11, 0.1);
  color: var(--warning);
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.pill.info {
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  border: 1px solid rgba(59, 130, 246, 0.2);
}

.pill.low,
.pill.success {
  background: rgba(34, 197, 94, 0.1);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.stack-card p {
  margin: 8px 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
  word-break: break-word;
}
</style>