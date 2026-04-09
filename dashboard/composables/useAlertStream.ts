import type { AlertRecord } from '~/types/alerts'

export function useAlertStream(initial: AlertRecord[] = []) {
  const alerts = ref<AlertRecord[]>(initial)
  const connected = ref(false)
  let source: EventSource | null = null

  function connect() {
    if (!import.meta.client) return
    source = new EventSource('/api/alerts/stream')
    source.addEventListener('ready', () => {
      connected.value = true
    })
    source.addEventListener('alerts', (evt) => {
      const payload = JSON.parse((evt as MessageEvent).data) as { alerts: AlertRecord[] }
      alerts.value = payload.alerts
    })
    source.onerror = () => {
      connected.value = false
    }
  }

  function close() {
    source?.close()
    source = null
    connected.value = false
  }

  onMounted(connect)
  onBeforeUnmount(close)

  return {
    alerts,
    connected
  }
}
