<template>
  <section class="runtime-panel">
    <div class="panel-grid">
      <!-- Services Panel -->
      <article class="panel">
        <div class="panel-head">
          <div>
            <p class="meta-label">Services</p>
            <h3>Platform runtime</h3>
          </div>
        </div>
        <div class="service-grid">
          <ServiceCard
            v-for="service in services"
            :key="service.name"
            :service="service"
            :disabled="isRefreshingServices"
            @toggle="emit('toggle-service', service)"
          />
        </div>
      </article>

      <!-- Logs Panel -->
      <article class="panel">
        <div class="panel-head">
          <div>
            <p class="meta-label">Logs</p>
            <h3>Recent runtime events</h3>
          </div>
        </div>
        <ul class="stack-list">
          <LogCard
            v-for="entry in logs"
            :key="`${entry.timestamp}-${entry.message}`"
            :log="entry"
          />
        </ul>
      </article>

      <!-- Synapse Sweep Panel -->
      <article class="panel span-wide">
        <div class="panel-head">
          <div>
            <p class="meta-label">Synapse Sweep output</p>
            <h3>Cluster scan payload</h3>
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
import ServiceCard from '../cards/ServiceCard.vue'
import LogCard from '../cards/LogCard.vue'
import ScanCell from '../cards/ScanCell.vue'
import { data } from '../../../wailsjs/go/models'
import type { ScanCell as ScanCellType } from './types'

interface Props {
  services: data.ServiceStatus[]
  logs: data.LogLine[]
  scanGrid: {
    cells: ScanCellType[]
    headline: string
    fallback: string
  }
  isRefreshingServices: boolean
}

const props = defineProps<Props>()
console.log('RuntimePanel props:', props)
import { onMounted } from 'vue'
onMounted(() => console.log('RuntimePanel mounted'))
const emit = defineEmits<{
  'toggle-service': [service: data.ServiceStatus]
  'open-scan-modal': [cell: ScanCellType]
}>()
</script>

<style scoped>
.runtime-panel {
  width: 100%;
}

.panel-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

.panel {
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
}

.panel.span-wide {
  grid-column: 1 / -1;
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
}

h3 {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  line-height: 1.3;
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