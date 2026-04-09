import { ref } from 'vue'

export function useClusterLabels() {
  const clusterLabels = ref<Record<string, string>>({})

  function loadClusterLabels() {
    const savedLabels = localStorage.getItem('cluster-labels')
    if (savedLabels) {
      try {
        clusterLabels.value = JSON.parse(savedLabels)
      } catch (e) {
        console.error('Failed to parse cluster labels', e)
      }
    }
  }

  function saveClusterLabels() {
    localStorage.setItem('cluster-labels', JSON.stringify(clusterLabels.value))
  }

  function createClusterLabel(cluster: string, currentLabel?: string) {
    const newLabel = window.prompt(`Enter a label for cluster context "${cluster}":`, currentLabel || '')
    if (newLabel !== null) {
      if (newLabel.trim()) {
        clusterLabels.value[cluster] = newLabel.trim()
      } else {
        delete clusterLabels.value[cluster]
      }
      saveClusterLabels()
    }
  }

  return {
    clusterLabels,
    loadClusterLabels,
    saveClusterLabels,
    createClusterLabel,
  }
}