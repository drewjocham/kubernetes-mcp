import type { AlertIngestPayload } from '~/types/alerts'
import { createAlert } from '~/server/utils/alert-store'
import { runWorkflow } from '~/server/utils/workflow'
import { createOrUpdateCluster } from '~/server/utils/cluster-store'

export default defineEventHandler(async (event) => {
  const payload = await readBody<Partial<AlertIngestPayload>>(event)
  // Transform if needed
  const transformed: AlertIngestPayload = {
    kind: payload.kind || 'Unknown',
    cluster: payload.cluster || 'unknown',
    namespace: payload.namespace || '',
    pod: payload.pod || '',
    container: payload.container,
    ruleName: payload.ruleName,
    severity: payload.severity || 'critical',
    reason: payload.reason,
    firstSeenAt: payload.firstSeenAt || new Date().toISOString(),
    source: payload.source || 'kube-watcher',
    diagnostics: payload.diagnostics
  }
  
  // Auto-create/update cluster from alert
  if (transformed.cluster && transformed.cluster !== 'unknown') {
    createOrUpdateCluster(
      transformed.cluster,
      'kubernetes', // default type, could be inferred from source
      {
        dashboardWebhook: `http://localhost:3000/api/alerts/ingest`
      }
    )
  }
  
  const alert = createAlert(transformed)
  runWorkflow(alert.id)
  return {
    ok: true,
    alert
  }
})
