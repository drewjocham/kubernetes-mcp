import { getWorkflowConfig } from '~/server/utils/alert-store'
import { assertAllowedToolsBaseUrl } from '~/server/utils/tools-endpoint'

export default defineEventHandler(async (event) => {
  const config = getWorkflowConfig()
  if (!config.toolsEndpoint) {
    throw createError({ statusCode: 400, message: 'Tools endpoint not configured' })
  }
  const tool = event.context.params?.tool
  if (!tool) {
    throw createError({ statusCode: 400, message: 'Tool name required' })
  }

  let baseUrl: string
  try {
    baseUrl = assertAllowedToolsBaseUrl(config.toolsEndpoint)
  } catch (e) {
    const message = e instanceof Error ? e.message : 'Invalid tools endpoint'
    throw createError({ statusCode: 400, message })
  }
  const body = await readBody(event)
  try {
    return await $fetch(`${baseUrl}/tools/${tool}`, {
      method: 'POST',
      body,
      timeout: 30000
    })
  } catch (err: unknown) {
    const statusCode = typeof err === 'object' && err !== null && 'response' in err
      ? (err as { response?: { status?: number } }).response?.status
      : undefined
    const responseMessage = typeof err === 'object' && err !== null && 'response' in err
      ? (err as { response?: { _data?: { error?: string } } }).response?._data?.error
      : undefined
    const fallbackMessage = err instanceof Error ? err.message : 'Tool execution failed'
    throw createError({
      statusCode: statusCode || 502,
      message: responseMessage || fallbackMessage
    })
  }
})