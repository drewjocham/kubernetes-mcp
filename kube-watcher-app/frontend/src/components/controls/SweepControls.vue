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
  background: #111;
  color: #888;
  border: 1px solid #333;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}

.custom-interval-input {
  display: flex;
  align-items: center;
  gap: 4px;
}

.interval-number {
  width: 50px;
  background: #111;
  color: #fff;
  border: 1px solid #333;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
}

.interval-unit {
  background: #111;
  color: #888;
  border: 1px solid #333;
  padding: 4px 4px;
  border-radius: 4px;
  font-size: 13px;
  outline: none;
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
  font-size: 10px !important;
  letter-spacing: 0.02em;
  padding: 4px 10px !important;
  min-height: 26px;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "SF Pro Display", "Helvetica Neue", sans-serif;
}
</style>