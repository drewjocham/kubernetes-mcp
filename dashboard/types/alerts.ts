export type AlertKind = 'CrashLoopBackOff' | 'OOMKilled' | 'ImagePullBackOff' | 'Unknown'
export type AlertStatus = 'detected' | 'thinking' | 'report_ready' | 'failed'

export interface AlertIngestPayload {
  kind: AlertKind
  cluster: string
  namespace: string
  pod: string
  container?: string
  ruleName?: string
  severity?: 'warning' | 'critical'
  reason?: string
  firstSeenAt?: string
  source?: 'kube-watcher' | 'manual'
  diagnostics?: {
    lastLogLines?: string[]
    describeOutput?: string
    clusterEvents?: string[]
    yaml?: string
    aiHelp?: string
    podUID?: string
  }
}

export interface AlertThinkingStep {
  at: string
  title: string
  details: string
}

export interface RCAReport {
  generatedAt: string
  summary: string
  rootCause: string
  recommendedActions: string[]
  confidence: 'low' | 'medium' | 'high'
}

export interface AlertRecord extends AlertIngestPayload {
  id: string
  status: AlertStatus
  createdAt: string
  updatedAt: string
  thinkingSteps: AlertThinkingStep[]
  report?: RCAReport
  error?: string
  podExists?: boolean
  cachedAt?: string
}

export interface WorkflowConfig {
  mcpEndpoint: string
  mcpApiKeyHeader: string
  mcpApiKey: string
  agentEndpoint: string
  agentApiKeyHeader: string
  agentApiKey: string
  watchedErrors: AlertKind[]
  autoApplyFixes: boolean
  toolsEndpoint: string
}
