<template>
  <div class="page-wrap">
    <NGrid :cols="24" :x-gap="16" :y-gap="16">
      <NGridItem :span="8">
        <NCard title="Kube-Watcher Workflow Config">
          <NForm label-placement="top">
            <NFormItem label="MCP diagnostics endpoint">
              <NInput v-model:value="config.mcpEndpoint" placeholder="https://mcp.internal/diagnostics" />
            </NFormItem>
            <NFormItem label="MCP API Key (optional)">
              <NInput v-model:value="config.mcpApiKey" type="password" show-password-on="click" />
            </NFormItem>
            <NFormItem label="MCP Tools API endpoint">
              <NInput v-model:value="config.toolsEndpoint" placeholder="http://localhost:8080" />
            </NFormItem>
            <NFormItem label="Agent RCA endpoint">
              <NInput v-model:value="config.agentEndpoint" placeholder="https://agent.internal/report" />
            </NFormItem>
            <NFormItem label="Agent API Key (optional)">
              <NInput v-model:value="config.agentApiKey" type="password" show-password-on="click" />
            </NFormItem>
            <NFormItem label="Auto-apply fixes">
              <NSwitch v-model:value="config.autoApplyFixes" />
            </NFormItem>
            <NSpace>
              <NButton type="primary" @click="saveConfig">Save workflow config</NButton>
              <NButton @click="sendDemoAlert">Send demo alert</NButton>
            </NSpace>
          </NForm>
        </NCard>
        <NCard title="Watcher action YAML" style="margin-top: 16px">
          <NCode :code="watcherYamlExample" language="yaml" />
        </NCard>
      </NGridItem>
      <NGridItem :span="16">
        <NCard :title="`Alert Dashboard (${alerts.length})`">
          <template #header-extra>
            <NSpace>
              <NButton size="small" tag="a" href="/clusters">Clusters</NButton>
              <NButton size="small" tag="a" href="/tools">Tools</NButton>
              <NTag :type="connected ? 'success' : 'warning'">
                {{ connected ? 'SSE Connected' : 'Reconnecting…' }}
              </NTag>
            </NSpace>
          </template>
          <NDataTable
            :columns="columns"
            :data="alerts"
            :row-key="(row: AlertRecord) => row.id"
            :pagination="{ pageSize: 8 }"
          />
        </NCard>
        <NCard v-if="selectedAlert" title="Selected Alert Details" style="margin-top: 16px">
          <NSpace vertical :size="12">
            <div><strong>ID:</strong> {{ selectedAlert.id }}</div>
            <div><strong>Rule:</strong> {{ selectedAlert.ruleName || 'N/A' }}</div>
            <div><strong>Reason:</strong> {{ selectedAlert.reason || 'N/A' }}</div>
            <div>
              <strong>Thinking Timeline:</strong>
              <ul>
                <li v-for="step in selectedAlert.thinkingSteps" :key="`${step.at}-${step.title}`">
                  {{ step.at }} — {{ step.title }} — {{ step.details }}
                </li>
              </ul>
            </div>
            <div v-if="selectedAlert.report">
              <strong>Recommended Actions:</strong>
              <ul>
                <li v-for="action in selectedAlert.report.recommendedActions" :key="action">{{ action }}</li>
              </ul>
            </div>
          </NSpace>
        </NCard>
      </NGridItem>
    </NGrid>
  </div>
</template>

<script setup lang="ts">
import { h } from 'vue'
import {
  NButton,
  NCard,
  NCode,
  NDataTable,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NSpace,
  NSwitch,
  NTag,
  useMessage
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import type { AlertIngestPayload, AlertRecord, WorkflowConfig } from '~/types/alerts'
import { useAlertStream } from '~/composables/useAlertStream'

const message = useMessage()

const { data: alertData, refresh: refreshAlerts } = await useFetch<{ alerts: AlertRecord[] }>('/api/alerts')
const { data: cfgData } = await useFetch<{ config: WorkflowConfig }>('/api/config')

const config = reactive<WorkflowConfig>({
  mcpEndpoint: cfgData.value?.config.mcpEndpoint ?? '',
  mcpApiKeyHeader: cfgData.value?.config.mcpApiKeyHeader ?? 'Authorization',
  mcpApiKey: cfgData.value?.config.mcpApiKey ?? '',
  agentEndpoint: cfgData.value?.config.agentEndpoint ?? '',
  agentApiKeyHeader: cfgData.value?.config.agentApiKeyHeader ?? 'Authorization',
  agentApiKey: cfgData.value?.config.agentApiKey ?? '',
  watchedErrors: cfgData.value?.config.watchedErrors ?? ['CrashLoopBackOff', 'OOMKilled', 'ImagePullBackOff'],
  autoApplyFixes: cfgData.value?.config.autoApplyFixes ?? false,
  toolsEndpoint: cfgData.value?.config.toolsEndpoint ?? ''
})

const { alerts, connected } = useAlertStream(alertData.value?.alerts ?? [])
watch(() => alertData.value?.alerts, (next) => {
  if (next) alerts.value = next
})

const selectedAlert = ref<AlertRecord | null>(null)

const columns: DataTableColumns<AlertRecord> = [
  { title: 'Kind', key: 'kind' },
  { title: 'Cluster', key: 'cluster' },
  { title: 'Namespace', key: 'namespace' },
  { title: 'Pod', key: 'pod' },
  {
    title: 'Status',
    key: 'status',
    render: (row) =>
      h(
        NTag,
        { type: row.status === 'failed' ? 'error' : row.status === 'report_ready' ? 'success' : 'warning' },
        { default: () => row.status }
      )
  },
  {
    title: 'RCA',
    key: 'report',
    render: (row) =>
      row.report
        ? h('div', [
            h('strong', row.report.rootCause),
            h('div', { style: 'margin-top: 6px; font-size: 12px; color: #9ca3af;' }, row.report.summary)
          ])
        : h('span', 'Pending')
  },
  {
    title: 'Thinking',
    key: 'thinking',
    render: (row) =>
      h(
        NSpace,
        { vertical: true, size: 2 },
        {
          default: () => row.thinkingSteps.map((step) => h('div', { style: 'font-size: 12px;' }, `• ${step.title}`))
        }
      )
  },
  {
    title: 'Action',
    key: 'action',
    render: (row) =>
      h(
        NButton,
        {
          size: 'small',
          onClick: () => {
            selectedAlert.value = row
          }
        },
        { default: () => 'Details' }
      )
  }
]

const watcherYamlExample = `actions:
  dashboard-webhook:
    type: webhook
    config:
      url: "http://localhost:3000/api/alerts/ingest"
    template: |
      {
        "kind": "CrashLoopBackOff",
        "cluster": "prod-eu",
        "namespace": "{{ .Event.Namespace }}",
        "pod": "{{ .Event.Name }}",
        "ruleName": "{{ .RuleName }}",
        "severity": "critical",
        "source": "kube-watcher"
      }`

async function saveConfig() {
  await $fetch('/api/config', {
    method: 'POST',
    body: config
  })
  message.success('Workflow config saved')
}

async function sendDemoAlert() {
  const demo: AlertIngestPayload = {
    kind: 'CrashLoopBackOff',
    cluster: 'demo-cluster',
    namespace: 'payments',
    pod: 'checkout-api-7ff98f4bcf-6jwkq',
    container: 'checkout-api',
    severity: 'critical',
    source: 'manual',
    reason: 'Container restart spike detected'
  }
  await $fetch('/api/alerts/ingest', { method: 'POST', body: demo })
  await refreshAlerts()
  message.success('Demo alert queued')
}
</script>

<style scoped>
.page-wrap {
  padding: 24px;
}
</style>
