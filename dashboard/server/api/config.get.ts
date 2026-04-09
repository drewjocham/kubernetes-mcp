import { getWorkflowConfig } from '~/server/utils/alert-store'

export default defineEventHandler(() => ({ config: getWorkflowConfig() }))
