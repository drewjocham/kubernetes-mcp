import { createError } from 'h3'
import { getAlert } from '~/server/utils/alert-store'

export default defineEventHandler((event) => {
  const id = getRouterParam(event, 'id')
  const alert = id ? getAlert(id) : undefined
  if (!alert) {
    throw createError({ statusCode: 404, statusMessage: 'Alert not found' })
  }
  return { alert }
})
