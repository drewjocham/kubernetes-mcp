export default defineNitroPlugin((nitroApp) => {
  console.log('[Nitro Plugin] Debug plugin loaded')
  console.log('Nitro app keys:', Object.keys(nitroApp))
  console.log('Router:', nitroApp.router)
  nitroApp.router.use('/plugin-debug', defineEventHandler((event) => {
    console.log(`[Debug Route] ${event.method} ${event.path}`)
    return { ok: true }
  }), 'get')
})