import { listAlerts } from '~/server/utils/alert-store'

export default defineEventHandler(() => {
  return {
    alerts: listAlerts()
  }
})
