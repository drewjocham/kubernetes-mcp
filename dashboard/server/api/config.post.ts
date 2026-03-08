import type { WorkflowConfig } from '~/types/alerts'
import { setWorkflowConfig } from '~/server/utils/alert-store'

export default defineEventHandler(async (event) => {
  const payload = await readBody<Partial<WorkflowConfig>>(event)
  const config = setWorkflowConfig(payload)
  return { ok: true, config }
})
