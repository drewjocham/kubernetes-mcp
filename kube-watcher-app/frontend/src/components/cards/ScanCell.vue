<template>
  <article
    :class="['scan-cell', cell.health]"
    @click="emit('click')"
  >
    <div class="scan-cell-copy">
      <strong>{{ cell.title }}</strong>
      <p class="scan-cell-summary">{{ cell.summary }}</p>
    </div>
    <div class="scan-cell-info-icon" title="Summary">i</div>
    <div class="scan-cell-click-hint">Click for details</div>
  </article>
</template>

<script setup lang="ts">
import type { ScanCell } from '../panels/types'

interface Props {
  cell: ScanCell
}

defineProps<Props>()
const emit = defineEmits<{
  click: []
}>()
</script>

<style scoped>
.scan-cell {
  padding: 16px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  cursor: pointer;
  transition: all 0.2s;
  position: relative;
  overflow: hidden;
}

.scan-cell:hover {
  border-color: var(--border-active);
  background: var(--hover);
  transform: translateY(-2px);
}

.scan-cell.good {
  border-left: 4px solid var(--success);
}

.scan-cell.warning {
  border-left: 4px solid var(--warning);
}

.scan-cell.unhealthy {
  border-left: 4px solid var(--error);
}

.scan-cell-copy strong {
  display: block;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 4px;
}

.scan-cell-summary {
  margin: 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
  opacity: 0;
  transition: opacity 0.2s;
}

.scan-cell-info-icon {
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

.scan-cell-info-icon:hover {
  background: var(--text-primary);
}

.scan-cell-info-icon:hover ~ .scan-cell-summary,
.scan-cell-summary:hover {
  opacity: 1;
}

.scan-cell-click-hint {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 8px;
  background: rgba(0, 0, 0, 0.05);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-tertiary);
  text-align: center;
  opacity: 0;
  transition: opacity 0.2s;
}

.scan-cell:hover .scan-cell-click-hint {
  opacity: 1;
}
</style>