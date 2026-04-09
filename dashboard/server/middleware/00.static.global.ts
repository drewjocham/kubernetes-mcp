export default defineEventHandler((event) => {
  console.log(`[00.static] ${event.method} ${event.path} ${event.node.req.url}`)
})