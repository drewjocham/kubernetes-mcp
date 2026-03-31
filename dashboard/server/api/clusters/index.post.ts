import { createOrUpdateCluster } from '~/server/utils/cluster-store'

interface ClusterRegistrationPayload {
  name: string
  type: 'kubernetes' | 'docker'
  config?: {
    dashboardWebhook?: string
    prometheusEndpoint?: string
    kubeconfigPath?: string
    namespace?: string
    target?: string
  }
}

export default defineEventHandler(async (event) => {
  const body = await readBody<ClusterRegistrationPayload>(event)
  
  if (!body.name) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Cluster name is required'
    })
  }
  
  const cluster = createOrUpdateCluster(
    body.name,
    body.type || 'kubernetes',
    body.config || {}
  )
  
  return {
    ok: true,
    cluster
  }
})