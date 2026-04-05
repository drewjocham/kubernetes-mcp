import { ref, computed } from 'vue'
import {
  GetAIWorkspace,
  GetAlerts,
  GetHistory,
  GetRecommendations,
  GetServiceStatus,
  GetLogs,
  GetStatus,
  GetEndpoint,
  GetAnomstackAnomalies,
  GetCurrentContext,
  StartService,
  StopService,
} from '../../wailsjs/go/main/App'
import { data } from '../../wailsjs/go/models'

export function useWorkspace() {
  const workspace = ref<data.AIWorkspace | null>(null)
  const alerts = ref<data.AlertRecord[]>([])
  const history = ref<data.Incident[]>([])
  const recommendations = ref<data.Recommendation[]>([])
  const services = ref<data.ServiceStatus[]>([])
  const logs = ref<data.LogLine[]>([])
  const mcpStatus = ref<data.StatusResponse | null>(null)
  const mcpEndpoint = ref('')
  const currentContext = ref('unknown')
  const anomstackConnected = ref(false)
  const isLoading = ref(false)
  const isRefreshingServices = ref(false)
  const error = ref<string | null>(null)

  const summaryCards = computed(() => {
    if (!workspace.value) return []
    return [
      { label: 'Open alerts', value: workspace.value.summary.openAlerts, accent: 'rose' },
      { label: 'Critical now', value: workspace.value.summary.criticalAlerts, accent: 'amber' },
      { label: 'Arguskube actions ready', value: workspace.value.summary.automationReady, accent: 'teal' },
      { label: 'Deploy targets', value: workspace.value.summary.deployTargets, accent: 'ink' },
    ]
  })

  const topPriority = computed(() => workspace.value?.priorities?.[0] ?? null)

  const timeline = computed(() =>
    [...history.value]
      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
      .slice(0, 5),
  )

  const runtimeLogs = computed(() => logs.value.slice(0, 6))

  async function loadCurrentContext() {
    try {
      console.log('Loading current context...')
      const context = await GetCurrentContext()
      console.log('Current context:', context)
      currentContext.value = context
    } catch (err) {
      console.error('Failed to get current context:', err)
      currentContext.value = 'unknown'
      error.value = err instanceof Error ? err.message : String(err)
    }
  }

  async function loadWorkspace() {
    isLoading.value = true
    error.value = null
    console.log('Loading workspace data...')
    try {
      const results = await Promise.allSettled([
        GetAIWorkspace(),
        GetAlerts(),
        GetHistory(),
        GetRecommendations(),
        GetServiceStatus(),
        GetLogs(),
        GetStatus(),
        GetEndpoint(),
      ])
      
      // Helper to extract value or null
      const getValue = <T>(result: PromiseSettledResult<T>): T | null => 
        result.status === 'fulfilled' ? result.value : null
      
      const ws = getValue(results[0])
      const alertData = getValue(results[1]) || []
      const historyData = getValue(results[2]) || []
      const recommendationData = getValue(results[3]) || []
      const serviceData = getValue(results[4]) || []
      const logData = getValue(results[5]) || []
      const statusData = getValue(results[6])
      const endpointData = getValue(results[7]) || ''
      
      // Log any failures
      results.forEach((result, idx) => {
        if (result.status === 'rejected') {
          console.warn(`Promise ${idx} failed:`, result.reason)
        }
      })

      console.log('Workspace data loaded:', { 
        ws: !!ws, 
        wsSummary: ws?.summary,
        alerts: alertData.length, 
        history: historyData.length,
        recommendations: recommendationData.length,
        services: serviceData.length,
        logs: logData.length,
        status: !!statusData,
        endpoint: endpointData
      })
      
      // Set error if workspace failed to load
      if (!ws) {
        error.value = 'Failed to load workspace data. Check MCP server connection.'
        console.error('Workspace data load failed')
      }
      
      workspace.value = ws
      alerts.value = alertData // Start with MCP alerts only
      history.value = historyData
      recommendations.value = recommendationData
      services.value = serviceData
      logs.value = logData
      mcpStatus.value = statusData
      mcpEndpoint.value = endpointData
      
      // Try to load anomstack anomalies separately - don't let it fail the whole workspace
      try {
        const anomstackAlerts = await GetAnomstackAnomalies()
        console.log('Anomstack anomalies loaded:', anomstackAlerts.length)
        anomstackConnected.value = true
        // Combine MCP alerts with anomstack anomalies
        alerts.value = [...alertData, ...anomstackAlerts]
      } catch (error) {
        console.warn('Failed to load anomstack anomalies, continuing without them:', error)
        anomstackConnected.value = false
        // Keep only MCP alerts
        alerts.value = alertData
      }
    } catch (err) {
      console.error('Failed to load workspace:', err)
      anomstackConnected.value = false
      error.value = err instanceof Error ? err.message : String(err)
      // Don't throw - let UI handle error state
    } finally {
      isLoading.value = false
      console.log('Loading complete, isLoading:', isLoading.value)
    }
  }

  async function toggleService(service: data.ServiceStatus) {
    isRefreshingServices.value = true
    try {
      if (service.status === 'running') {
        await StopService(service.name)
      } else {
        await StartService(service.name)
      }
      // Refresh services list
      services.value = await GetServiceStatus()
    } catch (error) {
      console.error('Service action failed:', error)
      throw error
    } finally {
      isRefreshingServices.value = false
    }
  }

  return {
    workspace,
    alerts,
    history,
    recommendations,
    services,
    logs,
    mcpStatus,
    mcpEndpoint,
    currentContext,
    anomstackConnected,
    isLoading,
    isRefreshingServices,
    error,
    summaryCards,
    topPriority,
    timeline,
    runtimeLogs,
    loadCurrentContext,
    loadWorkspace,
    toggleService,
  }
}