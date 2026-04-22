import { ref, computed, onUnmounted } from 'vue'
import {
  GetAIWorkspace,
  GetAlerts,
  GetHistory,
  GetRecommendations,
  GetStatus,
  GetEndpoint,
  GetAnomstackAnomalies,
  GetCurrentContext,
  LogError,
} from '../../wailsjs/go/main/App'
import { data } from '../../wailsjs/go/models'

export function useWorkspace() {
  const workspace = ref<data.AIWorkspace | null>(null)
  const alerts = ref<data.AlertRecord[]>([])
  const history = ref<data.Incident[]>([])
  const recommendations = ref<data.Recommendation[]>([])
  const logs = ref<any[]>([])
  const mcpStatus = ref<data.StatusResponse | null>(null)
  const mcpEndpoint = ref('')
  const currentContext = ref('unknown')
  const anomstackConnected = ref(false)
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const services = ref<any[]>([])

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
      LogError(`loadCurrentContext error: ${error.value}`).catch(() => {})
    }
  }



  async function loadWorkspace() {
    isLoading.value = true
    error.value = null
    // Debug logging
    const debug = (window as any).debugLog = (window as any).debugLog || []
    debug.push({ time: Date.now(), action: 'loadWorkspace.start' })
    console.log('Loading workspace data...')
    try {
      console.log('Calling GetAIWorkspace...')
      const results = await Promise.allSettled([
        GetAIWorkspace(),
        GetAlerts(),
        GetHistory(),
        GetRecommendations(),
        GetStatus(),
        GetEndpoint(),
      ])
      debug.push({ time: Date.now(), action: 'promise.allSettled', results })
      console.log('Promise.allSettled completed, results count:', results.length)
      
      // Helper to extract value or null
      const getValue = <T>(result: PromiseSettledResult<T>, idx: number): T | null => {
        if (result.status === 'fulfilled') {
          console.log(`Promise ${idx} fulfilled`, result.value)
          LogError(`Promise ${idx} fulfilled`).catch(() => {})
          return result.value
        } else {
          console.error(`Promise ${idx} rejected:`, result.reason)
          // Also log to window for inspection
          ;(window as any).lastPromiseRejection = { idx, reason: result.reason }
          // Send to backend log
          LogError(`Promise ${idx} rejected: ${result.reason}`).catch(() => {})
          return null
        }
      }
      
      const ws = getValue(results[0], 0)
      const alertData = getValue(results[1], 1) || []
      const historyData = getValue(results[2], 2) || []
      const recommendationData = getValue(results[3], 3) || []
       const statusData = getValue(results[4], 4)
       const endpointData = getValue(results[5], 5) || ''
       
       LogError(`statusData: ${statusData ? 'present' : 'null'}, endpointData: ${endpointData}`).catch(() => {})
       
       // Service data is empty (Docker Compose support removed)
      
      // Log any failures
      results.forEach((result, idx) => {
        if (result.status === 'rejected') {
          console.error(`Promise ${idx} failed:`, result.reason)
          ;(window as any).lastRejection = { idx, reason: result.reason }
        }
      })

       console.log('Workspace data loaded:', { 
        ws: !!ws, 
        wsSummary: ws?.summary,
        alerts: alertData.length, 
        history: historyData.length,
        recommendations: recommendationData.length,
        services: 0,
        logs: 0,
        status: !!statusData,
        endpoint: endpointData
       })
       // Debug log to backend
       LogError(`ws truthy: ${!!ws}, ws type: ${typeof ws}, ws keys: ${ws ? Object.keys(ws).join(',') : 'null'}`).catch(() => {})
       if (ws) {
         LogError(`Workspace load SUCCESS: ws=${!!ws}, alerts=${alertData.length}, history=${historyData.length}, recommendations=${recommendationData.length}`).catch(() => {})
       }
      
       // Set error if workspace failed to load
      if (!ws) {
        error.value = 'Failed to load workspace data. Check MCP server connection.'
        console.error('Workspace data load failed', results)
        // Log to backend for debugging
        LogError(`Workspace load failed: ${JSON.stringify(results.map(r => ({status: r.status, reason: r.status === 'rejected' ? r.reason : 'fulfilled'})))}`).catch(() => {})
      }
      
       workspace.value = ws
      alerts.value = alertData // Start with MCP alerts only
      history.value = historyData
      recommendations.value = recommendationData
      services.value = []
      logs.value = []
      mcpStatus.value = statusData
      mcpEndpoint.value = endpointData
      
      // Debug: log error state
      console.log('Workspace load completed, error:', error.value)
      if (error.value) {
        LogError(`Unexpected error after load: ${error.value}`).catch(() => {})
      }
      
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
      // Log to backend for debugging
      LogError(`Workspace load catch error: ${err instanceof Error ? err.message : String(err)}`).catch(() => {})
      // Don't throw - let UI handle error state
    } finally {
      isLoading.value = false
      console.log('Loading complete, isLoading:', isLoading.value)
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
    error,
    summaryCards,
    topPriority,
    timeline,
    runtimeLogs,
    loadCurrentContext,
    loadWorkspace,
  }
}