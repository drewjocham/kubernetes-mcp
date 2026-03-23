<template>
  <div class="page-wrap">
    <NCard title="Managed Clusters">
      <template #header-extra>
        <NSpace>
          <NButton size="small" @click="refresh">Refresh</NButton>
          <NButton size="small" tag="a" href="/">Alerts</NButton>
          <NButton size="small" tag="a" href="/tools">Tools</NButton>
        </NSpace>
      </template>
      
      <NDataTable
        :columns="columns"
        :data="clusters"
        :row-key="(row: ClusterRecord) => row.id"
        :loading="loading"
      />
      
      <NSpace vertical style="margin-top: 24px">
        <NText type="info">
          Clusters are automatically discovered from alert sources. To deploy a watcher to a cluster, use:
        </NText>
        <NCode :code="deployCommand" language="bash" />
        <NText type="info" style="margin-top: 16px">
          To manually register a cluster:
        </NText>
        <NCode :code="registerCommand" language="bash" />
      </NSpace>
    </NCard>
  </div>
</template>

<script setup lang="ts">
import { NButton, NCard, NDataTable, NTag, NText, NCode, NSpace } from 'naive-ui'
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
    kubeconfigPath?: string
    namespace?: string
    target?: string
  }
}

const clusters = ref<ClusterRecord[]>([])
const loading = ref(false)

const deployCommand = `./bin/watcher-deploy \\
  --action deploy \\
  --target kube \\
  --app-type watcher \\
  --cluster-name "my-cluster" \\
  --dashboard-webhook "http://localhost:3000/api/alerts/ingest"`

const registerCommand = `curl -X POST http://localhost:3000/api/clusters \\
  -H "Content-Type: application/json" \\
  -d '{
    "name": "my-cluster",
    "type": "kubernetes",
    "config": {
      "dashboardWebhook": "http://localhost:3000/api/alerts/ingest"
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
  max-width: 1200px;
  margin: 0 auto;
  padding: 24px;
}
</style>