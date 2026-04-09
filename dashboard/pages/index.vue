<template>
  <div class="page-wrap">
    <NGrid
      :cols="24"
      :x-gap="16"
      :y-gap="16"
    >
      <NGridItem :span="8">
        <NCard title="Kube-Watcher Workflow Config">
          <NForm label-placement="top">
            <NFormItem label="MCP diagnostics endpoint">
              <NInput
                v-model:value="config.mcpEndpoint"
                placeholder="https://mcp.internal/diagnostics"
              />
            </NFormItem>
            <NFormItem label="MCP API Key (optional)">
              <NInput
                v-model:value="config.mcpApiKey"
                type="password"
                show-password-on="click"
              />
            </NFormItem>
            <NFormItem label="MCP Tools API endpoint">
              <NInput
                v-model:value="config.toolsEndpoint"
                placeholder="http://localhost:8080"
              />
            </NFormItem>
            <NFormItem label="Agent RCA endpoint">
              <NInput
                v-model:value="config.agentEndpoint"
                placeholder="https://agent.internal/report"
              />
            </NFormItem>
            <NFormItem label="Agent API Key (optional)">
              <NInput
                v-model:value="config.agentApiKey"
                type="password"
                show-password-on="click"
              />
            </NFormItem>
            <NFormItem label="Auto-apply fixes">
              <NSwitch v-model:value="config.autoApplyFixes" />
            </NFormItem>
            <div class="action-buttons">
              <NButton
                type="primary"
                @click="saveConfig"
              >
                Save Configuration
              </NButton>
              <NButton
                class="demo-alert-btn"
                @click="sendDemoAlert"
              >
                Send Demo Alert
              </NButton>
            </div>
          </NForm>
        </NCard>
        <NCard
          title="Watcher action YAML"
          style="margin-top: 16px"
        >
          <CommandBlock
            :command="watcherYamlExample"
          />
        </NCard>
      </NGridItem>
      <NGridItem :span="16">
        <div class="alert-dashboard-header">
          <h1 class="dashboard-title">
            Alert Dashboard
          </h1>
          <div class="connection-status">
            <NTag :type="connected ? 'success' : 'warning'">
              {{ connected ? 'Live' : 'Reconnecting…' }}
            </NTag>
            <div
              class="status-indicator"
              :class="{ disconnected: !connected }"
            />
          </div>
        </div>

        <NCard :title="`Active Alerts (${alerts.length})`">
          <template #header-extra>
            <NSpace>
              <NButton
                size="small"
                tag="a"
                href="/clusters"
              >
                Clusters
              </NButton>
              <NButton
                size="small"
                tag="a"
                href="/tools"
              >
                Tools
              </NButton>
            </NSpace>
          </template>
          <NDataTable
            :columns="columns"
            :data="alerts"
            :row-key="(row: AlertRecord) => row.id"
            :pagination="{ pageSize: 8 }"
            :checked-row-keys="selectedRowKeys"
            :single-line="false"
            keyboard
            class="alerts-table"
            @update:checked-row-keys="handleRowSelect"
            @keydown="handleKeydown"
          />
        </NCard>
        <NCard
          v-if="selectedAlert"
          title="Selected Alert Details"
          class="alert-details-card"
          style="margin-top: 16px"
        >
          <div class="alert-details">
            <div><strong>ID:</strong> {{ selectedAlert.id }}</div>
            <div><strong>Rule:</strong> {{ selectedAlert.ruleName || 'N/A' }}</div>
            <div><strong>Reason:</strong> {{ selectedAlert.reason || 'N/A' }}</div>
            <div>
              <strong>Thinking Timeline:</strong>
              <ul>
                <li
                  v-for="step in selectedAlert.thinkingSteps"
                  :key="`${step.at}-${step.title}`"
                >
                  {{ step.at }} — {{ step.title }} — {{ step.details }}
                </li>
              </ul>
            </div>
            <div v-if="selectedAlert.report">
              <strong>Recommended Actions:</strong>
              <ul>
                <li
                  v-for="action in selectedAlert.report.recommendedActions"
                  :key="action"
                >
                  {{ action }}
                </li>
              </ul>
            </div>
          </div>
        </NCard>
      </NGridItem>
    </NGrid>

    <div class="navigation-hint">
      Use ↑↓ arrow keys to navigate alerts • Enter to view details
    </div>
  </div>
</template>

<script setup lang="ts">
import { h } from 'vue'
import {
  NButton,
  NCard,
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
import CommandBlock from '~/components/CommandBlock.vue'

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
const selectedRowKeys = ref<string[]>([])

function handleRowSelect(keys: (string | number)[]) {
  selectedRowKeys.value = keys.map(k => String(k))
  if (keys.length > 0) {
    selectedAlert.value = alerts.value.find(alert => alert.id === String(keys[0])) || null
  } else {
    selectedAlert.value = null
  }
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    const currentIndex = selectedRowKeys.value.length > 0
      ? alerts.value.findIndex(alert => alert.id === selectedRowKeys.value[0])
      : -1

    let newIndex
    if (event.key === 'ArrowDown') {
      newIndex = Math.min(currentIndex + 1, alerts.value.length - 1)
    } else {
      newIndex = Math.max(currentIndex - 1, 0)
    }

    if (newIndex >= 0 && newIndex < alerts.value.length) {
      const newId = alerts.value[newIndex].id
      selectedRowKeys.value = [newId]
      selectedAlert.value = alerts.value[newIndex]
    }
  }
}

const columns: DataTableColumns<AlertRecord> = [
  {
    title: 'Kind',
    key: 'kind',
    render: (row) => h('span', { class: selectedRowKeys.value.includes(row.id) ? 'selected-row-highlight' : '' }, row.kind)
  },
  {
    title: 'Cluster',
    key: 'cluster',
    render: (row) => h('span', { class: selectedRowKeys.value.includes(row.id) ? 'selected-row-highlight' : '' }, row.cluster)
  },
  {
    title: 'Namespace',
    key: 'namespace',
    render: (row) => h('span', { class: selectedRowKeys.value.includes(row.id) ? 'selected-row-highlight' : '' }, row.namespace)
  },
  {
    title: 'Pod',
    key: 'pod',
    render: (row) => h('span', { class: selectedRowKeys.value.includes(row.id) ? 'selected-row-highlight' : '' }, row.pod)
  },
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
            h('strong', { style: 'color: #8B5CF6;' }, row.report.rootCause),
            h('div', { style: 'margin-top: 6px; font-size: 12px; color: #94A3B8;' }, row.report.summary)
          ])
        : h('span', { style: 'color: #64748B; font-style: italic;' }, 'Pending')
  },
  {
    title: 'Thinking',
    key: 'thinking',
    render: (row) =>
      h(
        NSpace,
        { vertical: true, size: 2 },
        {
          default: () => row.thinkingSteps.map((step) => h('div', { style: 'font-size: 12px; color: #CBD5E1;' }, `• ${step.title}`))
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
          type: selectedRowKeys.value.includes(row.id) ? 'primary' : 'default',
          onClick: () => {
            selectedRowKeys.value = [row.id]
            selectedAlert.value = row
          }
        },
        { default: () => 'View' }
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
  padding: 40px;
  background: transparent;
  min-height: 100vh;
}

.alert-dashboard-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.dashboard-title {
  font-size: 28px;
  font-weight: 700;
  background: linear-gradient(135deg, #8B5CF6 0%, #F1F5F9 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  margin: 0;
}

.connection-status {
  display: flex;
  align-items: center;
  gap: 12px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10B981;
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.5);
  animation: pulse 2s infinite;
}

.status-indicator.disconnected {
  background: #EF4444;
  box-shadow: 0 0 8px rgba(239, 68, 68, 0.5);
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

.config-section {
  margin-bottom: 32px;
}

.workflow-config-card {
  margin-bottom: 24px;
}

.yaml-example-card {
  opacity: 0.8;
}

.alert-details-card {
  margin-top: 24px;
  border-left: 4px solid #8B5CF6;
}

.alert-details {
  background: linear-gradient(135deg, rgba(30, 41, 59, 0.6) 0%, rgba(51, 65, 85, 0.4) 100%);
  border-radius: 12px;
  padding: 20px;
  backdrop-filter: blur(10px);
  border: 1px solid rgba(139, 92, 246, 0.2);
}

.alert-details strong {
  color: #8B5CF6;
  font-weight: 600;
}

.alert-details ul {
  margin: 12px 0;
  padding-left: 24px;
}

.alert-details li {
  margin-bottom: 6px;
  color: #CBD5E1;
  line-height: 1.5;
}

.action-buttons {
  display: flex;
  gap: 12px;
  margin-top: 20px;
}

.demo-alert-btn {
  background: linear-gradient(135deg, #10B981 0%, #059669 100%);
  border: none;
}

.demo-alert-btn:hover {
  background: linear-gradient(135deg, #059669 0%, #047857 100%);
}

.selected-row-highlight {
  background: rgba(139, 92, 246, 0.1) !important;
  border-left: 3px solid #8B5CF6 !important;
}

.navigation-hint {
  position: fixed;
  bottom: 20px;
  right: 20px;
  background: rgba(15, 23, 42, 0.9);
  border: 1px solid rgba(139, 92, 246, 0.3);
  border-radius: 8px;
  padding: 8px 12px;
  font-size: 12px;
  color: #CBD5E1;
  backdrop-filter: blur(10px);
}
</style>
