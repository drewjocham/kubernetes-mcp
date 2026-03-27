import { EventEmitter } from 'node:events'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import type { AlertIngestPayload, AlertRecord, AlertThinkingStep, RCAReport, WorkflowConfig } from '~/types/alerts'

const STORE_KEY = '__kube_watcher_dashboard_store__'

type DashboardStore = {
  alerts: AlertRecord[]
  config: WorkflowConfig
  bus: EventEmitter
}

type PersistedDashboardStore = {
  alerts: AlertRecord[]
  config: WorkflowConfig
}

function storeFilePath(): string {
  const baseDir = process.env.KW_DASHBOARD_DATA_DIR || join(process.cwd(), '.kube-watcher', 'dashboard')
  return join(baseDir, 'alerts-store.json')
}

function loadPersistedStore(): PersistedDashboardStore | null {
  const filePath = storeFilePath()
  if (!existsSync(filePath)) return null
  try {
    const raw = readFileSync(filePath, 'utf8')
    if (!raw.trim()) return null
    const parsed = JSON.parse(raw) as Partial<PersistedDashboardStore>
    return {
      alerts: Array.isArray(parsed.alerts) ? parsed.alerts : [],
      config: parsed.config ? { ...defaultConfig(), ...parsed.config } : defaultConfig()
    }
  } catch {
    return null
  }
}

function persistStore(snapshot: PersistedDashboardStore) {
  const filePath = storeFilePath()
  const dir = dirname(filePath)
  mkdirSync(dir, { recursive: true })
  writeFileSync(filePath, JSON.stringify(snapshot, null, 2), { mode: 0o600 })
}

function defaultConfig(): WorkflowConfig {
  return {
    mcpEndpoint: '',
    mcpApiKeyHeader: 'Authorization',
    mcpApiKey: '',
    agentEndpoint: '',
    agentApiKeyHeader: 'Authorization',
    agentApiKey: '',
    watchedErrors: ['CrashLoopBackOff', 'OOMKilled', 'ImagePullBackOff'],
    autoApplyFixes: false,
    toolsEndpoint: 'http://localhost:8080/v1'
  }
}

function getStore(): DashboardStore {
  const globalStore = globalThis as typeof globalThis & Record<string, DashboardStore | undefined>
  if (!globalStore[STORE_KEY]) {
    const persisted = loadPersistedStore()
    globalStore[STORE_KEY] = {
      alerts: persisted?.alerts ?? [],
      config: persisted?.config ?? defaultConfig(),
      bus: new EventEmitter()
    }
  }
  return globalStore[STORE_KEY]!
}

function notify() {
  const store = getStore()
  persistStore({
    alerts: store.alerts,
    config: store.config
  })
  store.bus.emit('alert-updated', store.alerts)
}

function normalizePayload(payload: AlertIngestPayload): AlertIngestPayload {
  return {
    ...payload,
    source: payload.source ?? 'kube-watcher',
    severity: payload.severity ?? 'critical',
    firstSeenAt: payload.firstSeenAt ?? new Date().toISOString()
  }
}

export function listAlerts(): AlertRecord[] {
  return [...getStore().alerts].sort((a, b) => b.createdAt.localeCompare(a.createdAt))
}

export function getAlert(id: string): AlertRecord | undefined {
  return getStore().alerts.find((alert) => alert.id === id)
}

export function createAlert(payload: AlertIngestPayload): AlertRecord {
  const store = getStore()
  const now = new Date().toISOString()
  const normalized = normalizePayload(payload)
  const alert: AlertRecord = {
    ...normalized,
    id: crypto.randomUUID(),
    status: 'detected',
    createdAt: now,
    updatedAt: now,
    thinkingSteps: []
  }
  store.alerts.unshift(alert)
  notify()
  return alert
}

export function updateAlert(id: string, mutator: (current: AlertRecord) => AlertRecord): AlertRecord | undefined {
  const store = getStore()
  const idx = store.alerts.findIndex((alert) => alert.id === id)
  if (idx < 0) return undefined
  const current = store.alerts[idx]
  const updated = {
    ...mutator(current),
    updatedAt: new Date().toISOString()
  }
  store.alerts[idx] = updated
  notify()
  return updated
}

export function pushThinkingStep(id: string, step: AlertThinkingStep): AlertRecord | undefined {
  return updateAlert(id, (current) => ({
    ...current,
    thinkingSteps: [...current.thinkingSteps, step]
  }))
}

export function setThinking(id: string): AlertRecord | undefined {
  return updateAlert(id, (current) => ({ ...current, status: 'thinking' }))
}

export function setReport(id: string, report: RCAReport): AlertRecord | undefined {
  return updateAlert(id, (current) => ({
    ...current,
    status: 'report_ready',
    report
  }))
}

export function setFailed(id: string, error: string): AlertRecord | undefined {
  return updateAlert(id, (current) => ({
    ...current,
    status: 'failed',
    error
  }))
}

export function getWorkflowConfig(): WorkflowConfig {
  return getStore().config
}

export function setWorkflowConfig(next: Partial<WorkflowConfig>): WorkflowConfig {
  const store = getStore()
  store.config = {
    ...store.config,
    ...next
  }
  notify()
  return store.config
}

export function onStoreUpdates(handler: (alerts: AlertRecord[]) => void): () => void {
  const store = getStore()
  store.bus.on('alert-updated', handler)
  return () => store.bus.off('alert-updated', handler)
}

export function __resetDashboardStoreForTests() {
  const store = getStore()
  store.alerts = []
  store.config = defaultConfig()
  store.bus.removeAllListeners()
}
