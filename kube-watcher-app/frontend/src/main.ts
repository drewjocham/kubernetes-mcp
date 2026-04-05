import {createApp} from 'vue'
import App from './App.vue'
import './style.css';

console.log('Starting Vue app...')

// Check if Wails runtime is available
if (!(window as any).go?.main?.App) {
  console.warn('Wails runtime not detected. Running in browser-only mode.')
  const appEl = document.getElementById('app')
  if (appEl) {
    appEl.innerHTML = `
      <div style="padding: 20px; color: orange; font-family: monospace;">
        <h2>Wails Runtime Not Detected</h2>
        <p>Running in browser-only mode. Backend functions will not work.</p>
        <p>If this is unexpected, ensure the Wails dev server is running.</p>
        <p>Check console for errors.</p>
      </div>
    `
  }
}

const app = createApp(App)

app.config.errorHandler = (err, instance, info) => {
  console.error('Vue error:', err, info)
  // Display error in DOM
  const appEl = document.getElementById('app')
  if (appEl) {
    appEl.innerHTML = `
      <div style="padding: 20px; color: red; font-family: monospace;">
        <h2>Vue Error</h2>
        <pre>${err instanceof Error ? err.stack : String(err)}</pre>
        <p>Info: ${info}</p>
      </div>
    `
  }
}

try {
  app.mount('#app')
  console.log('Vue app mounted successfully')
} catch (err) {
  console.error('Failed to mount Vue app:', err)
  const appEl = document.getElementById('app')
  if (appEl) {
    appEl.innerHTML = `
      <div style="padding: 20px; color: red; font-family: monospace;">
        <h2>Failed to mount Vue app</h2>
        <pre>${err instanceof Error ? err.stack : String(err)}</pre>
      </div>
    `
  }
}
