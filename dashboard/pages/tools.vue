<template>
  <div class="page-wrap">
    <NGrid
      :cols="24"
      :x-gap="16"
      :y-gap="16"
    >
      <NGridItem :span="10">
        <NCard title="MCP Tools">
          <NSpace
            vertical
            :size="12"
          >
            <NAlert
              v-if="!toolsEndpoint"
              title="Configuration"
              type="info"
            >
              Configure the MCP Tools API endpoint on the <NuxtLink to="/">
                dashboard
              </NuxtLink>.
            </NAlert>
            <NInput
              v-if="toolsEndpoint"
              v-model:value="search"
              placeholder="Search tools by name/description"
              clearable
            />
            <NSpace v-if="toolsEndpoint">
              <NButton
                :loading="loadingTools"
                @click="loadTools"
              >
                Refresh
              </NButton>
              <NButton
                :disabled="!selectedTool"
                @click="resetSelection"
              >
                Clear Selection
              </NButton>
            </NSpace>
            <NAlert
              v-if="error"
              title="Error"
              type="error"
            >
              {{ error }}
            </NAlert>
            <NDataTable
              v-if="toolsEndpoint"
              :columns="toolColumns"
              :data="filteredTools"
              :row-key="(row: Tool) => row.name"
              size="small"
              max-height="560"
            />
          </NSpace>
        </NCard>
      </NGridItem>

      <NGridItem :span="14">
        <NCard title="Tool Runner">
          <NSpace
            vertical
            :size="12"
          >
            <NText depth="3">
              Featured operations
            </NText>
            <NSpace>
              <NButton
                v-for="name in featuredToolNames"
                :key="name"
                size="small"
                @click="selectToolByName(name)"
              >
                {{ name === 'run_watch_dog' ? 'Run Synapse Sweep' : name }}
              </NButton>
            </NSpace>

            <div class="synapse-sweep-section">
              <NSpace
                align="center"
                justify="space-between"
                style="width: 100%"
              >
                <NSpace align="center">
                  <NButton
                    size="small"
                    type="primary"
                    @click="selectToolByName('run_watch_dog')"
                  >
                    Run Synapse Sweep
                  </NButton>
                  <NSelect
                    v-model:value="watchDogRounds"
                    :options="roundOptions"
                    size="small"
                    placeholder="Synapse Sweep Intervals"
                    style="width: 200px"
                  />
                </NSpace>
                <NSpace align="center" style="margin-left: auto;">
                  <span class="switch-label">Switch model, API key, or provider config</span>
                  <NSwitch
                    v-model:value="switchConfig"
                    size="small"
                  />
                </NSpace>
              </NSpace>
            </div>

            <NAlert
              v-if="!selectedTool"
              type="info"
            >
              Select a tool from the table to render its parameter form.
            </NAlert>

            <template v-else>
              <NText strong>
                {{ selectedTool.name }}
              </NText>
              <NText depth="3">
                {{ selectedTool.description }}
              </NText>

              <NForm label-placement="top">
                <NFormItem
                  v-for="param in selectedTool.parameters || []"
                  :key="param.name"
                  :label="param.required ? `${param.name} *` : param.name"
                >
                  <template #default>
                    <div class="param-input-wrap">
                      <NInput
                        v-if="param.type === 'string'"
                        :value="getStringParam(param.name)"
                        :placeholder="param.description || 'string'"
                        @update:value="setParamValue(param.name, $event)"
                      />
                      <NInputNumber
                        v-else-if="param.type === 'number'"
                        :value="getNumberParam(param.name)"
                        clearable
                        style="width: 100%"
                        @update:value="setParamValue(param.name, $event)"
                      />
                      <NSwitch
                        v-else-if="param.type === 'boolean'"
                        :value="getBooleanParam(param.name)"
                        @update:value="setParamValue(param.name, $event)"
                      />
                      <NInput
                        v-else
                        :value="getStringParam(param.name)"
                        :placeholder="param.type"
                        @update:value="setParamValue(param.name, $event)"
                      />
                      <NText
                        depth="3"
                        class="param-help"
                      >
                        {{ param.description || `${param.type} parameter` }}
                        <span v-if="param.default !== undefined"> · default: {{ param.default }}</span>
                      </NText>
                    </div>
                  </template>
                </NFormItem>
              </NForm>

              <NAlert
                v-if="validationErrors.length > 0"
                type="warning"
              >
                <ul class="validation-list">
                  <li
                    v-for="message in validationErrors"
                    :key="message"
                  >
                    {{ message }}
                  </li>
                </ul>
              </NAlert>

              <NSpace>
                <NButton
                  type="primary"
                  :loading="executing"
                  @click="executeTool"
                >
                  Execute
                </NButton>
                <NButton @click="resetParamValues">
                  Reset Values
                </NButton>
              </NSpace>
            </template>
          </NSpace>
        </NCard>

        <NCard
          v-if="result"
          title="Execution Result"
          style="margin-top: 16px"
        >
          <CommandBlock
            :command="JSON.stringify(result, null, 2)"
          />
        </NCard>
      </NGridItem>
    </NGrid>
  </div>
</template>

<script setup lang="ts">
import { h, ref, computed, onMounted } from 'vue'
import {
  NAlert,
  NButton,
  NCard,
  NDataTable,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NSpace,
  NSelect,
  NSwitch,
  NText
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'
import CommandBlock from '~/components/CommandBlock.vue'

interface ToolParameter {
  name: string
  type: string
  description?: string
  required?: boolean
  default?: unknown
}

function setParamValue(name: string, value: unknown) {
  paramValues.value[name] = value
}

function getStringParam(name: string) {
  const value = paramValues.value[name]
  if (typeof value === 'string') return value
  if (value === undefined || value === null) return null
  return String(value)
}

function getNumberParam(name: string) {
  const value = paramValues.value[name]
  if (typeof value === 'number') return value
  if (typeof value === 'string' && value.trim() !== '') {
    const parsed = Number(value)
    return Number.isNaN(parsed) ? null : parsed
  }
  return null
}

function getBooleanParam(name: string) {
  const value = paramValues.value[name]
  if (typeof value === 'boolean') return value
  return false
}

interface Tool {
  name: string
  description: string
  parameters?: ToolParameter[]
}

const featuredToolNames = [
  'get_node_status',
  'get_pod_resources',
  'list_namespaces',
  'get_pod_logs',
  'run_watch_dog'
]

const roundOptions = [
  { label: '1 minute', value: '1min' },
  { label: '5 minutes', value: '5min' },
  { label: '10 minutes', value: '10min' },
  { label: '30 minutes', value: '30min' },
  { label: '1 hour', value: '1h' },
  { label: 'Manual', value: 'manual' }
]

const toolsEndpoint = ref('')
const tools = ref<Tool[]>([])
const loadingTools = ref(false)
const error = ref('')
const selectedToolName = ref('')
const paramValues = ref<Record<string, unknown>>({})
const executing = ref(false)
const result = ref<unknown>(null)
const search = ref('')
const watchDogRounds = ref('')
const switchConfig = ref(false)
let watchDogInterval: any = null

watch(watchDogRounds, (newVal) => {
  if (watchDogInterval) {
    clearInterval(watchDogInterval)
    watchDogInterval = null
  }

  if (newVal === 'manual') return

  const msMap: Record<string, number> = {
    '1min': 60 * 1000,
    '5min': 5 * 60 * 1000,
    '10min': 10 * 60 * 1000,
    '30min': 30 * 60 * 1000,
    '1h': 60 * 60 * 1000
  }

  const ms = msMap[newVal]
  if (ms) {
    watchDogInterval = setInterval(() => {
      console.log('[DEBUG_LOG] Auto-running Synapse Sweep (analyze_cluster)')
      const tool = tools.value.find(t => t.name === 'analyze_cluster')
      if (tool) {
        // We can either just select it or actually execute it if we had a silent execute method.
        // For now, let's just select it and notify or we can call executeTool directly if parameters are met.
        // To be safe and simple, we'll just trigger the same logic as the button if it's already selected.
        if (selectedToolName.value === 'analyze_cluster' && validationErrors.value.length === 0) {
          executeTool()
        } else {
           // If not selected or has errors, we might want to just skip or log.
           console.warn('[DEBUG_LOG] Cannot auto-run Synapse Sweep: not selected or has validation errors')
        }
      }
    }, ms)
  }
})

onBeforeUnmount(() => {
  if (watchDogInterval) {
    clearInterval(watchDogInterval)
  }
})

const selectedTool = computed(() => tools.value.find((tool) => tool.name === selectedToolName.value) || null)
const filteredTools = computed(() => {
  const query = search.value.trim().toLowerCase()
  if (!query) return tools.value
  return tools.value.filter((tool) => {
    const inName = tool.name.toLowerCase().includes(query)
    const inDescription = tool.description.toLowerCase().includes(query)
    return inName || inDescription
  })
})

const validationErrors = computed(() => {
  if (!selectedTool.value) return []
  const issues: string[] = []
  for (const param of selectedTool.value.parameters || []) {
    if (!param.required) continue
    const value = paramValues.value[param.name]
    if (value === undefined || value === null || value === '') {
      issues.push(`Required parameter missing: ${param.name}`)
    }
  }
  return issues
})

const toolColumns: DataTableColumns<Tool> = [
  { title: 'Name', key: 'name' },
  { title: 'Description', key: 'description' },
  {
    title: 'Run',
    key: 'actions',
    render: (row) =>
      h(
        NButton,
        { size: 'small', onClick: () => selectTool(row) },
        { default: () => 'Select' }
      )
  }
]

const { data: configData } = await useFetch<{ config: { toolsEndpoint?: string } }>('/api/config')
toolsEndpoint.value = configData.value?.config.toolsEndpoint || ''

function normalizeParameterValue(param: ToolParameter, raw: unknown) {
  if (raw === undefined || raw === null || raw === '') return raw
  if (param.type === 'number') {
    return Number(raw)
  }
  if (param.type === 'boolean') {
    return Boolean(raw)
  }
  return raw
}

function buildPayload() {
  if (!selectedTool.value) return {}
  const payload: Record<string, unknown> = {}
  for (const param of selectedTool.value.parameters || []) {
    const raw = paramValues.value[param.name]
    const value = normalizeParameterValue(param, raw)
    if (value === undefined || value === null || value === '') continue
    payload[param.name] = value
  }
  return payload
}

function selectTool(tool: Tool) {
  selectedToolName.value = tool.name
  result.value = null
  resetParamValues()
}

function selectToolByName(name: string) {
  const actualName = name === 'run_watch_dog' ? 'analyze_cluster' : name
  const tool = tools.value.find((entry) => entry.name === actualName)
  if (tool) {
    selectTool(tool)
  }
}

function resetSelection() {
  selectedToolName.value = ''
  result.value = null
  paramValues.value = {}
}

function resetParamValues() {
  paramValues.value = {}
  if (!selectedTool.value?.parameters) return
  for (const param of selectedTool.value.parameters) {
    if (param.default !== undefined) {
      paramValues.value[param.name] = param.default
    }
  }
}

async function loadTools() {
  if (!toolsEndpoint.value) return
  loadingTools.value = true
  error.value = ''
  try {
    const data = await $fetch<{ tools: Tool[] }>('/api/tools')
    tools.value = data.tools || []

    if (selectedToolName.value && !tools.value.some((tool) => tool.name === selectedToolName.value)) {
      resetSelection()
    }
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Failed to load tools'
    const apiMessage = typeof err === 'object' && err !== null && 'data' in err
      ? (err as { data?: { message?: string } }).data?.message
      : undefined
    error.value = apiMessage || message
  } finally {
    loadingTools.value = false
  }
}

async function executeTool() {
  if (!selectedTool.value) return
  if (validationErrors.value.length > 0) return

  executing.value = true
  error.value = ''
  try {
    const payload = buildPayload()
    result.value = await $fetch(`/api/tools/${selectedTool.value.name}`, {
      method: 'POST',
      body: payload
    })
  } catch (err: unknown) {
    const message = err instanceof Error ? err.message : 'Tool execution failed'
    const apiMessage = typeof err === 'object' && err !== null && 'data' in err
      ? (err as { data?: { message?: string } }).data?.message
      : undefined
    error.value = apiMessage || message
  } finally {
    executing.value = false
  }
}

onMounted(async () => {
  if (!toolsEndpoint.value) return
  await loadTools()
  selectToolByName('run_watch_dog')
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

.n-button--primary {
  background: linear-gradient(135deg, #8B5CF6 0%, #7C3AED 100%);
  border: none;
  box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
}

.n-button--primary:hover {
  background: linear-gradient(135deg, #7C3AED 0%, #6D28D9 100%);
  box-shadow: 0 6px 16px rgba(139, 92, 246, 0.4);
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

.n-alert {
  border-radius: 8px;
}

.param-input-wrap {
  width: 100%;
}

.param-help {
  display: block;
  margin-top: 6px;
  color: #D1D5DB;
}

.validation-list {
  margin: 0;
  padding-left: 18px;
  color: #EF4444;
}

.n-text {
  color: #F3F4F6;
}

.n-text--3 {
  color: #D1D5DB;
}

.synapse-sweep-section {
  margin-top: 16px;
  padding: 12px;
  background: rgba(31, 41, 55, 0.3);
  border-radius: 8px;
  border: 1px solid rgba(75, 85, 99, 0.3);
}

.switch-label {
  font-size: 12px;
  color: #D1D5DB;
  margin-left: 8px;
}
</style>
