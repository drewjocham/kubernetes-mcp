import type { AlertIngestPayload } from '~/types/alerts'
import { createAlert } from '~/server/utils/alert-store'
import { runWorkflow } from '~/server/utils/workflow'

export default defineEventHandler(async (event) => {
  const payload = await readBody<AlertIngestPayload>(event)
  const alert = createAlert(payload)
  runWorkflow(alert.id)
  return {
    ok: true,
    alert
  }
})
