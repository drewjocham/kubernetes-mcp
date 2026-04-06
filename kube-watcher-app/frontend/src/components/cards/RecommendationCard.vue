<template>
  <li class="stack-card">
    <div class="stack-title">
      <strong>{{ recommendation.title }}</strong>
      <span :class="['pill', recommendation.severity]">{{ recommendation.severity }}</span>
    </div>
    <div class="recommendation-info-icon" title="Summary">i</div>
    <p class="recommendation-summary">{{ recommendation.summary }}</p>
    <code v-if="recommendation.steps?.length">{{ recommendation.steps[0] }}</code>
  </li>
</template>

<script setup lang="ts">
import { data } from '../../../wailsjs/go/models'

interface Props {
  recommendation: data.Recommendation
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
  position: relative;
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

.recommendation-summary {
  margin: 8px 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
  opacity: 0;
  transition: opacity 0.2s;
}

.stack-card code {
  display: block;
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  padding: 8px;
  border-radius: 8px;
  background: var(--code-bg);
  color: var(--code-text);
  margin-top: 8px;
  overflow-x: auto;
  white-space: pre-wrap;
  word-break: break-all;
}

.recommendation-info-icon {
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

.recommendation-info-icon:hover {
  background: var(--text-primary);
}

.recommendation-info-icon:hover ~ .recommendation-summary,
.recommendation-summary:hover {
  opacity: 1;
}
</style>