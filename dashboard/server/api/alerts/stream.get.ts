import { onStoreUpdates } from '~/server/utils/alert-store'

function sendSSE(res: import('node:http').ServerResponse, eventName: string, data: unknown) {
  res.write(`event: ${eventName}\n`)
  res.write(`data: ${JSON.stringify(data)}\n\n`)
}

export default defineEventHandler((event) => {
  const res = event.node.res
  res.setHeader('Content-Type', 'text/event-stream')
  res.setHeader('Cache-Control', 'no-cache, no-transform')
  res.setHeader('Connection', 'keep-alive')
  res.flushHeaders?.()

  sendSSE(res, 'ready', { ok: true })

  const unsubscribe = onStoreUpdates((alerts) => {
    sendSSE(res, 'alerts', { alerts })
  })

  const keepAlive = setInterval(() => {
    sendSSE(res, 'heartbeat', { at: new Date().toISOString() })
  }, 15000)

  event.node.req.on('close', () => {
    clearInterval(keepAlive)
    unsubscribe()
    res.end()
  })
})
