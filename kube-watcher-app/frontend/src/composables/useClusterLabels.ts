import { ref } from 'vue'

interface ClusterInfo {
  label: string
  type: 'real' | 'local'
}

export function useClusterLabels() {
  const clusterLabels = ref<Record<string, ClusterInfo>>({})

  function loadClusterLabels() {
    const savedLabels = localStorage.getItem('cluster-labels')
    if (savedLabels) {
      try {
        const parsed = JSON.parse(savedLabels)
        // Migration: if value is string, convert to ClusterInfo
        const migrated: Record<string, ClusterInfo> = {}
        for (const [cluster, value] of Object.entries(parsed)) {
          if (typeof value === 'string') {
            migrated[cluster] = { label: value, type: 'real' }
          } else if (value && typeof value === 'object' && 'label' in value && 'type' in value) {
            migrated[cluster] = value as ClusterInfo
          } else {
            // fallback
            migrated[cluster] = { label: String(value), type: 'real' }
          }
        }
        clusterLabels.value = migrated
      } catch (e) {
        console.error('Failed to parse cluster labels', e)
      }
    }
  }

  function saveClusterLabels() {
    localStorage.setItem('cluster-labels', JSON.stringify(clusterLabels.value))
  }

  function createClusterLabel(cluster: string, currentInfo?: ClusterInfo) {
    const currentLabel = currentInfo?.label || ''
    const currentType = currentInfo?.type || 'real'
    const newLabel = window.prompt(`Enter a label for cluster context "${cluster}":`, currentLabel)
    if (newLabel === null) return
    let newType = currentType
    const typeInput = window.prompt(`Select cluster type for "${cluster}":\nEnter "real" for real cluster, "local" for local cluster.`, currentType)
    if (typeInput === null) return
    if (typeInput === 'real' || typeInput === 'local') {
      newType = typeInput
    } else {
      // default to real if invalid input
      newType = 'real'
    }
    if (newLabel.trim()) {
      clusterLabels.value[cluster] = { label: newLabel.trim(), type: newType }
    } else {
      delete clusterLabels.value[cluster]
    }
    saveClusterLabels()
  }

  return {
    clusterLabels,
    loadClusterLabels,
    saveClusterLabels,
    createClusterLabel,
  }
}