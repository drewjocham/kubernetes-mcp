/**
 * In-memory cluster registry for the Nitro dev server.
 * Data is lost on process restart and is not shared across instances — use an external store in production.
 */
import { EventEmitter } from 'node:events'

export interface ClusterRecord {
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

const STORE_KEY = '__kube_watcher_cluster_store__'

type ClusterStore = {
  clusters: ClusterRecord[]
  bus: EventEmitter
}

function getStore(): ClusterStore {
  const globalStore = globalThis as typeof globalThis & Record<string, ClusterStore | undefined>
  if (!globalStore[STORE_KEY]) {
    globalStore[STORE_KEY] = {
      clusters: [],
      bus: new EventEmitter()
    }
  }
  return globalStore[STORE_KEY]!
}

function notify() {
  const store = getStore()
  store.bus.emit('cluster-updated', store.clusters)
}

export function listClusters(): ClusterRecord[] {
  const clusters = [...getStore().clusters]
  return clusters.map(computeClusterStatus)
    .sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
}

function computeClusterStatus(cluster: ClusterRecord): ClusterRecord {
  const now = new Date()
  const lastHeartbeat = new Date(cluster.lastHeartbeat)
  const diffMs = now.getTime() - lastHeartbeat.getTime()
  const diffMins = Math.floor(diffMs / 60000)
  
  const status: ClusterRecord['status'] =
    diffMins < 5 ? 'healthy' : diffMins < 30 ? 'unhealthy' : 'unknown'
  
  return {
    ...cluster,
    status
  }
}

export function getCluster(id: string): ClusterRecord | undefined {
  const cluster = getStore().clusters.find((cluster) => cluster.id === id)
  return cluster ? computeClusterStatus(cluster) : undefined
}

export function getClusterByName(name: string): ClusterRecord | undefined {
  const cluster = getStore().clusters.find((cluster) => cluster.name === name)
  return cluster ? computeClusterStatus(cluster) : undefined
}

export function createOrUpdateCluster(
  name: string,
  type: 'kubernetes' | 'docker',
  config: ClusterRecord['config']
): ClusterRecord {
  const store = getStore()
  const now = new Date().toISOString()
  
  // Check if cluster exists by name
  const existing = store.clusters.find((c) => c.name === name)
  
  if (existing) {
    // Update existing
    const updated = {
      ...existing,
      type,
      config: { ...existing.config, ...config },
      updatedAt: now,
      lastHeartbeat: now
    }
    const idx = store.clusters.findIndex((c) => c.id === existing.id)
    store.clusters[idx] = updated
    notify()
    return updated
  }
  
  // Create new
  const cluster: ClusterRecord = {
    id: crypto.randomUUID(),
    name,
    type,
    status: 'unknown',
    lastHeartbeat: now,
    createdAt: now,
    updatedAt: now,
    config
  }
  store.clusters.unshift(cluster)
  notify()
  return cluster
}

export function updateClusterHeartbeat(name: string): ClusterRecord | undefined {
  const store = getStore()
  const idx = store.clusters.findIndex((c) => c.name === name)
  if (idx < 0) {
    // Cluster doesn't exist yet, create a default one
    return createOrUpdateCluster(name, 'kubernetes', {})
  }
  
  const now = new Date().toISOString()
  const cluster = store.clusters[idx]
  const updated: ClusterRecord = {
    ...cluster,
    status: 'healthy',
    lastHeartbeat: now,
    updatedAt: now
  }
  store.clusters[idx] = updated
  notify()
  return updated
}

export function updateClusterStatus(name: string, status: ClusterRecord['status']): ClusterRecord | undefined {
  const store = getStore()
  const idx = store.clusters.findIndex((c) => c.name === name)
  if (idx < 0) return undefined
  
  const now = new Date().toISOString()
  const cluster = store.clusters[idx]
  const updated = {
    ...cluster,
    status,
    updatedAt: now
  }
  store.clusters[idx] = updated
  notify()
  return updated
}

export function deleteCluster(id: string): boolean {
  const store = getStore()
  const idx = store.clusters.findIndex((c) => c.id === id)
  if (idx < 0) return false
  
  store.clusters.splice(idx, 1)
  notify()
  return true
}

export function onClusterUpdates(handler: (clusters: ClusterRecord[]) => void): () => void {
  const store = getStore()
  store.bus.on('cluster-updated', handler)
  return () => store.bus.off('cluster-updated', handler)
}

export function __resetClusterStoreForTests() {
  const store = getStore()
  store.clusters = []
  store.bus.removeAllListeners()
}