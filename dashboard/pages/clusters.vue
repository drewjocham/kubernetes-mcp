<template>
  <div class="page-wrap">
    <div class="page-header">
      <h1 class="page-title">
        Cluster Management
      </h1>
      <p class="page-subtitle">
        Monitor and manage your Kubernetes clusters
      </p>
    </div>

    <NCard
      title="Managed Clusters"
      class="cluster-card"
    >
      <template #header-extra>
        <div class="refresh-section">
          <NSpace>
            <NButton
              size="small"
              @click="refresh"
            >
              Refresh Clusters
            </NButton>
            <NButton
              size="small"
              tag="a"
              href="/"
            >
              Alerts
            </NButton>
            <NButton
              size="small"
              tag="a"
              href="/tools"
            >
              Tools
            </NButton>
          </NSpace>
        </div>
      </template>
      
      <NDataTable
        :columns="columns"
        :data="clusters"
        :row-key="(row: ClusterRecord) => row.id"
        :loading="loading"
        class="cluster-table"
      />

      <div class="deployment-section">
        <NText
          type="info"
          strong
          style="font-size: 16px; margin-bottom: 16px;"
        >
          Deployment Instructions
        </NText>
        <NSpace
          vertical
          :size="16"
        >
          <div>
            <NText
              type="info"
              style="margin-bottom: 8px;"
            >
              Clusters are automatically discovered from alert sources. To deploy a watcher to a cluster:
            </NText>
            <CommandBlock :command="deployCommand" />
          </div>
          <div>
            <NText
              type="info"
              style="margin-bottom: 8px;"
            >
              To manually register a cluster:
            </NText>
            <CommandBlock :command="registerCommand" />
          </div>
        </NSpace>
      </div>
    </NCard>
  </div>
</template>

<script setup lang="ts">
import { NButton, NCard, NDataTable, NTag, NText, NSpace } from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

interface ClusterRecord {
  id: string
  name: string
  type: 'kubernetes' | 'docker'
  status: 'healthy' | 'unhealthy' | 'unknown'
  lastHeartbeat: string
  createdAt: string
  updatedAt: string
  config: {
    dashboardWebhook?: string
    prometheusEndpoint?: string
    kubeconfigPath?: string
    namespace?: string
    target?: string
  }
}

const clusters = ref<ClusterRecord[]>([])
const loading = ref(false)

const deployCommand = './bin/watcher-deploy --action deploy --target kube --app-type watcher --cluster-name "my-cluster" --dashboard-webhook "http://localhost:3000/api/alerts/ingest"'

const registerCommand = `curl -X POST http://localhost:3000/api/clusters -H "Content-Type: application/json" -d '{
  "name": "my-cluster",
  "type": "kubernetes",
  "config": {
    "dashboardWebhook": "http://localhost:3000/api/alerts/ingest",
    "prometheusEndpoint": "http://prometheus-server.monitoring.svc:9090"
  }
}'`

const columns: DataTableColumns<ClusterRecord> = [
  {
    title: 'Name',
    key: 'name',
    render(row) {
      return h('div', [
        h('strong', row.name),
        h('div', { style: 'font-size: 12px; color: #9ca3af;' }, row.id.slice(0, 8))
      ])
    }
  },
  {
    title: 'Type',
    key: 'type',
    render(row) {
      return h(NTag, { type: row.type === 'kubernetes' ? 'info' : 'default' }, {
        default: () => row.type
      })
    }
  },
  {
    title: 'Status',
    key: 'status',
    render(row) {
      const typeMap = {
        healthy: 'success',
        unhealthy: 'error',
        unknown: 'warning'
      } as const
      return h(NTag, { type: typeMap[row.status] }, {
        default: () => row.status
      })
    }
  },
  {
    title: 'Last Heartbeat',
    key: 'lastHeartbeat',
    render(row) {
      const date = new Date(row.lastHeartbeat)
      const now = new Date()
      const diffMs = now.getTime() - date.getTime()
      const diffMins = Math.floor(diffMs / 60000)
      
      let text = date.toLocaleTimeString()
      if (diffMins > 5) {
        text += ` (${diffMins}m ago)`
      }
      
      return h('div', [
        h('div', text),
        h('div', { style: 'font-size: 12px; color: #9ca3af;' }, 
          diffMins < 5 ? 'Active' : 'Stale'
        )
      ])
    }
  },
  {
    title: 'Config',
    key: 'config',
    render(row) {
      const configItems = []
      if (row.config.target) configItems.push(`target: ${row.config.target}`)
      if (row.config.namespace) configItems.push(`ns: ${row.config.namespace}`)
      if (row.config.dashboardWebhook) configItems.push('webhook: ✓')
      if (row.config.prometheusEndpoint) configItems.push('prometheus: ✓')
      if (row.config.kubeconfigPath) configItems.push('kubeconfig: ✓')
      
      return h('div', [
        h('div', { style: 'font-size: 12px;' }, configItems.join(', ')),
        h('div', { style: 'font-size: 11px; color: #9ca3af; margin-top: 4px;' }, 
          `created: ${new Date(row.createdAt).toLocaleDateString()}`
        )
      ])
    }
  }
]

async function refresh() {
  loading.value = true
  try {
    const data = await $fetch('/api/clusters')
    clusters.value = data.clusters
  } catch (err) {
    console.error('Failed to fetch clusters:', err)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  refresh()
})
</script>

<style scoped>
.page-wrap {
  max-width: 1400px;
  margin: 0 auto;
  padding: 32px;
  background: linear-gradient(135deg, #0D0D0F 0%, #1A1A1A 100%);
  min-height: 100vh;
}

.n-card {
  backdrop-filter: blur(10px);
  background: rgba(26, 26, 26, 0.95);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.n-card__header {
  font-weight: 600;
  font-size: 18px;
  color: #F3F4F6;
}

.n-button {
  transition: all 0.15s ease;
  font-weight: 500;
}

.n-data-table {
  background: transparent;
}

.n-data-table th {
  background: rgba(31, 41, 55, 0.8);
  font-weight: 600;
  color: #F3F4F6;
}

.n-data-table td {
  border-bottom: 1px solid rgba(75, 85, 99, 0.3);
  color: #E5E7EB;
}

.n-tag {
  font-size: 12px;
  font-weight: 500;
}

.n-text--info {
  color: #D1D5DB;
}
</style>
