export default defineNuxtConfig({
  devtools: { enabled: true },
  build: {
    transpile: ['naive-ui', 'vueuc', '@css-render/vue3-ssr', '@juggle/resize-observer']
  },
  vite: {
    optimizeDeps: {
      include: ['naive-ui', 'vueuc', 'date-fns-tz/formatInTimeZone']
    }
  },
  css: ['~/assets/main.css'],
  typescript: {
    strict: true,
    typeCheck: true
  }
})
