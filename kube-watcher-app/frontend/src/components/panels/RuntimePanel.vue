<template>
  <section class="runtime-panel">
    <div class="panel-grid">
      <!-- Synapse Sweep Panel -->
      <article class="panel">
        <div class="panel-head">
          <div>
            <p class="meta-label">Synapse Sweep output</p>
            <h3>Cluster diagnostics</h3>
          </div>
        </div>
        <div v-if="scanGrid.cells.length" class="scan-grid-wrap">
          <p class="scan-grid-headline">{{ scanGrid.headline }}</p>
          <div class="scan-grid">
            <ScanCell
              v-for="cell in scanGrid.cells"
              :key="`${cell.title}-${cell.summary}`"
              :cell="cell"
              @click="emit('open-scan-modal', cell)"
            />
          </div>
        </div>
        <pre v-else class="terminal-output">Run a scan to populate cluster diagnostics here.</pre>
      </article>
    </div>
  </section>
</template>

<script setup lang="ts">

import ScanCell from '../cards/ScanCell.vue'
import { data } from '../../../wailsjs/go/models'
import type { ScanCell as ScanCellType } from './types'

interface Props {
  scanGrid: {
    cells: ScanCellType[]
    headline: string
    fallback: string
  }
}

const props = defineProps<Props>()
console.log('RuntimePanel props:', props)
import { onMounted } from 'vue'
onMounted(() => console.log('RuntimePanel mounted'))
const emit = defineEmits<{
  'open-scan-modal': [cell: ScanCellType]
}>()
</script>

<style scoped>
.runtime-panel {
  width: 100%;
}

.panel-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
}

.panel {
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
}



.panel-head {
  margin-bottom: 20px;
}

.service-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  gap: 12px;
}

.stack-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.scan-grid-wrap {
  margin-top: 16px;
}

.scan-grid-headline {
  font-size: 16px;
  font-weight: 500;
  color: var(--text-primary);
  margin: 0 0 16px 0;
}

.scan-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
  gap: 12px;
}

.terminal-output {
  font-family: 'SF Mono', monospace;
  font-size: 12px;
  padding: 16px;
  border-radius: 12px;
  background: var(--code-bg);
  color: var(--code-text);
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  line-height: 1.5;
}

.meta-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin: 0 0 4px 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

h3 {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.panel-head-actions {
  margin-left: auto;
}

.streaming-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 12px;
  border-radius: 20px;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.2s;
}

.streaming-toggle:hover {
  background: var(--surface-strong);
}

.streaming-toggle.streaming-on {
  border-color: var(--good);
  background: rgba(63, 191, 127, 0.1);
}

.streaming-toggle-label {
  font-weight: 500;
}

.streaming-toggle-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-muted);
  transition: background 0.2s;
}

.streaming-toggle.streaming-on .streaming-toggle-dot {
  background: var(--good);
}

@media (max-width: 1024px) {
  .panel-grid {
    grid-template-columns: 1fr;
  }
  
  .service-grid {
    grid-template-columns: repeat(auto-fill, minmax(150px, 1fr));
  }
  
  .scan-grid {
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  }
}
</style>