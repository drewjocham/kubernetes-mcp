<template>
  <div class="sweep-controls">
    <div v-if="sweepInterval === 'custom'" class="custom-interval-input">
      <input v-model.number="localCustomValue" type="number" min="1" class="interval-number">
      <select v-model="localCustomUnit" class="interval-unit">
        <option value="m">min</option>
        <option value="h">hours</option>
        <option value="d">days</option>
      </select>
    </div>
    <select v-model="localInterval" class="sweep-select">
      <option value="manual">Manual</option>
      <option value="5m">5 min</option>
      <option value="30m">30 min</option>
      <option value="2h">2 hours</option>
      <option value="5h">5 hours</option>
      <option value="12h">12 hours</option>
      <option value="1d">1 day</option>
      <option value="custom">Custom</option>
    </select>
    <button class="primary-btn sweep-btn" @click="$emit('scan')" :disabled="isRunningSweep">
      {{ isRunningSweep ? 'Scanning…' : 'Scan Cluster' }}
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  sweepInterval: string
  customIntervalValue: number
  customIntervalUnit: string
  isRunningSweep: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'update:interval': [value: string]
  'update:customValue': [value: number]
  'update:customUnit': [value: string]
  'scan': []
}>()

const localInterval = computed({
  get: () => props.sweepInterval,
  set: (value) => emit('update:interval', value)
})

const localCustomValue = computed({
  get: () => props.customIntervalValue,
  set: (value) => emit('update:customValue', value)
})

const localCustomUnit = computed({
  get: () => props.customIntervalUnit,
  set: (value) => emit('update:customUnit', value)
})
</script>

<style scoped>
.sweep-controls {
  display: flex;
  align-items: center;
  gap: 8px;
}

.sweep-select {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 120px;
}

.sweep-select:hover {
  background: var(--surface-strong);
  border-color: rgba(224, 223, 240, 0.2);
}

.sweep-select:focus {
  border-color: var(--ink);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

.custom-interval-input {
  display: flex;
  align-items: center;
  gap: 8px;
}

.interval-number {
  width: 60px;
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
  transition: all 0.2s ease;
  text-align: center;
}

.interval-number:hover {
  background: var(--surface-strong);
  border-color: rgba(224, 223, 240, 0.2);
}

.interval-number:focus {
  border-color: var(--ink);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

.interval-unit {
  background: var(--surface);
  color: var(--text);
  border: 1px solid var(--border);
  padding: 8px 10px;
  border-radius: 8px;
  font-size: 13px;
  outline: none;
  cursor: pointer;
  transition: all 0.2s ease;
  min-width: 80px;
}

.interval-unit:hover {
  background: var(--surface-strong);
  border-color: rgba(224, 223, 240, 0.2);
}

.interval-unit:focus {
  border-color: var(--ink);
  box-shadow: 0 0 0 2px rgba(125, 116, 214, 0.2);
}

.primary-btn {
  padding: 10px 20px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.primary-btn:hover:not(:disabled) {
  background: var(--primary-hover);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.sweep-btn {
  font-size: 13px !important;
  letter-spacing: 0.02em;
  padding: 8px 16px !important;
  min-height: 36px;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", sans-serif;
  border-radius: 8px;
}
</style>