<template>
  <div class="page-wrap">
    <NGrid :cols="24" :x-gap="16" :y-gap="16">
      <NGridItem :span="10">
        <NCard title="MCP Tools">
          <NSpace vertical :size="12">
            <NAlert title="Configuration" type="info" v-if="!toolsEndpoint">
              Configure the MCP Tools API endpoint on the <NuxtLink to="/">dashboard</NuxtLink>.
            </NAlert>
            <NInput
              v-if="toolsEndpoint"
              v-model:value="search"
              placeholder="Search tools by name/description"
              clearable
            />
            <NSpace v-if="toolsEndpoint">
              <NButton @click="loadTools" :loading="loadingTools">Refresh</NButton>
              <NButton @click="resetSelection" :disabled="!selectedTool">Clear Selection</NButton>
            </NSpace>
            <NAlert title="Error" type="error" v-if="error">
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
          <NSpace vertical :size="12">
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
                {{ name }}
              </NButton>
            </NSpace>

            <NAlert type="info" v-if="!selectedTool">
              Select a tool from the table to render its parameter form.
            </NAlert>

            <template v-else>
              <NText strong>{{ selectedTool.name }}</NText>
              <NText depth="3">{{ selectedTool.description }}</NText>

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
                        @update:value="setParamValue(param.name, $event)"
                        :placeholder="param.description || 'string'"
                      />
                      <NInputNumber
                        v-else-if="param.type === 'number'"
                        :value="getNumberParam(param.name)"
                        @update:value="setParamValue(param.name, $event)"
                        clearable
                        style="width: 100%"
                      />
                      <NSwitch
                        v-else-if="param.type === 'boolean'"
                        :value="getBooleanParam(param.name)"
                        @update:value="setParamValue(param.name, $event)"
                      />
                      <NInput
                        v-else
                        :value="getStringParam(param.name)"
                        @update:value="setParamValue(param.name, $event)"
                        :placeholder="param.type"
                      />
                      <NText depth="3" class="param-help">
                        {{ param.description || `${param.type} parameter` }}
                        <span v-if="param.default !== undefined"> · default: {{ param.default }}</span>
                      </NText>
                    </div>
                  </template>
                </NFormItem>
              </NForm>

              <NAlert type="warning" v-if="validationErrors.length > 0">
                <ul class="validation-list">
                  <li v-for="message in validationErrors" :key="message">{{ message }}</li>
                </ul>
              </NAlert>

              <NSpace>
                <NButton type="primary" @click="executeTool" :loading="executing">Execute</NButton>
                <NButton @click="resetParamValues">Reset Values</NButton>
              </NSpace>
            </template>
          </NSpace>
        </NCard>

        <NCard title="Execution Result" style="margin-top: 16px" v-if="result">
          <NCode :code="JSON.stringify(result, null, 2)" language="json" />
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
  NCode,
  NDataTable,
  NForm,
  NFormItem,
  NGrid,
  NGridItem,
  NInput,
  NInputNumber,
  NSpace,
  NSwitch,
  NText
} from 'naive-ui'
import type { DataTableColumns } from 'naive-ui'

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
  'analyze_cluster'
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
  const tool = tools.value.find((entry) => entry.name === name)
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
  selectToolByName('analyze_cluster')
})
</script>

<style scoped>
.page-wrap {
  padding: 24px;
}

.param-input-wrap {
  width: 100%;
}

.param-help {
  display: block;
  margin-top: 6px;
}

.validation-list {
  margin: 0;
  padding-left: 18px;
}
</style>
