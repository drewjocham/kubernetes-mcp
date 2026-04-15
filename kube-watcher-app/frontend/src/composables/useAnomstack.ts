import { ref } from 'vue'
import { GetAnomstackAnomalies } from '../../wailsjs/go/main/App'

export function useAnomstack() {
  const isConnectingAnomstack = ref(false)
  const isInvestigating = ref(false)
  const showAnomstackErrorModal = ref(false)
  const anomstackError = ref('')
  const anomstackRecommendations = ref<string[]>([])

  async function connectAnomstack() {
    console.log('Manual connect to anomstack...')
    isConnectingAnomstack.value = true
    anomstackError.value = ''
    anomstackRecommendations.value = []

    try {
      const anomalies = await GetAnomstackAnomalies()
      console.log('Manual connect successful, anomalies:', anomalies)
      return anomalies
    } catch (error: any) {
      console.warn('Anomstack connection failed (ignored):', error)
      const errorMessage = error?.message || error?.toString() || 'Unknown error'
      anomstackError.value = errorMessage
      anomstackRecommendations.value = generateAnomstackRecommendations(errorMessage)
      // Don't show modal, return empty array
      return []
    } finally {
      isConnectingAnomstack.value = false
    }
  }

  function generateAnomstackRecommendations(errorMessage: string): string[] {
    const recommendations: string[] = []

    if (errorMessage.includes('kubectl proxy is running')) {
      recommendations.push('Start kubectl proxy to access cluster services: kubectl proxy --port=8001')
      recommendations.push('Keep kubectl proxy running in a separate terminal')
      recommendations.push('Ensure you have kubectl configured and connected to the cluster')
    } else if (errorMessage.includes('connection refused') || errorMessage.includes('ECONNREFUSED')) {
      recommendations.push('Start kubectl proxy: kubectl proxy --port=8001')
      recommendations.push('Ensure kubectl proxy is running on port 8001')
      recommendations.push('Check if kubectl proxy process is still running')
    } else if (errorMessage.includes('timeout') || errorMessage.includes('ETIMEDOUT')) {
      recommendations.push('Check if kubectl proxy is running: kubectl proxy --port=8001')
      recommendations.push('Verify kubectl proxy is accessible on localhost:8001')
      recommendations.push('Ensure anomstack service is healthy: kubectl get pods -n kw-anomaly')
    } else if (errorMessage.includes('Service') && errorMessage.includes('not found')) {
      recommendations.push('Deploy anomstack service: Apply the anomstack deployment YAML')
      recommendations.push('Check namespace exists: kubectl get ns kw-anomaly')
      recommendations.push('Verify service name matches: kubectl get svc -n kw-anomaly')
    } else if (errorMessage.includes('403') || errorMessage.includes('Forbidden')) {
      recommendations.push('Check RBAC permissions for accessing anomstack service')
      recommendations.push('Verify service account has necessary cluster roles')
      recommendations.push('Ensure kubectl proxy has proper authentication')
    } else {
      recommendations.push('Start kubectl proxy: kubectl proxy --port=8001')
      recommendations.push('Check anomstack pod logs: kubectl logs -n kw-anomaly -l app=anomstack')
      recommendations.push('Verify anomstack configuration and environment variables')
      recommendations.push('Ensure Prometheus is running and accessible')
      recommendations.push('Test anomstack service from within cluster: kubectl run test-pod --image=busybox --rm -i --restart=Never -- wget http://anomstack.kw-anomaly.svc.cluster.local:8080/health')
    }

    return recommendations
  }

  return {
    isConnectingAnomstack,
    isInvestigating,
    showAnomstackErrorModal,
    anomstackError,
    anomstackRecommendations,
    connectAnomstack,
    generateAnomstackRecommendations,
  }
}