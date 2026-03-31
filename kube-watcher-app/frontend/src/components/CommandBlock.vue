<template>
  <div class="command-block" @click="handleCommandClick" @mouseenter="handleMouseEnter" @mouseleave="handleMouseLeave">
    <div class="command-content" :class="{ 'isRunning': isRunning, 'hasLogs': hasLogs }">
      <code>{{ command }}</code>
    </div>
    <div class="copy-icon" @click.stop="copyCommand">
      <svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
        <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
        <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
      </svg>
      <transition name="fade-rise">
        <span v-if="showCopied" class="copied-text">copied</span>
      </transition>
    </div>
    <transition name="hover">
      <div v-if="(isHovered || isRunning) && !showLogs" class="hover-text">Run Command</div>
    </transition>

    <!-- Log Layover Screen -->
    <transition name="modal">
      <div v-if="showLogs" class="log-layover" @click.stop>
        <div class="log-header">
          <div class="log-title">
            <span class="status-indicator" :class="{ 'pulse': isRunning }"></span>
            Logs for: <code>{{ command }}</code>
          </div>
          <div class="log-actions">
            <button class="close-btn" @click="showLogs = false">
              <svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                <line x1="18" y1="6" x2="6" y2="18"></line>
                <line x1="6" y1="6" x2="18" y2="18"></line>
              </svg>
            </button>
          </div>
        </div>
        <div class="log-body" ref="logBody">
          <pre v-if="logs">{{ logs }}</pre>
          <pre v-else-if="isRunning" class="empty-state">Executing command, please wait...</pre>
          <pre v-else class="empty-state">No logs available for this command yet.</pre>
        </div>
        <div class="log-footer">
          <span v-if="isRunning" class="executing-text">Command is running...</span>
          <span v-else class="last-run-text">Last run complete. Logs persisted.</span>
          <button class="re-run-btn" @click="runCommand" :disabled="isRunning">
            Re-run
          </button>
        </div>
      </div>
    </transition>
    <div v-if="showLogs" class="modal-overlay" @click.stop="showLogs = false"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue'
import { RunCommand } from '../../wailsjs/go/main/App'
import { logStore, setLogs, getLogs } from '../logStore'

interface Props {
  command: string
}

const props = defineProps<Props>()

const isHovered = ref(false)
const isRunning = ref(false)
const showCopied = ref(false)
const showLogs = ref(false)
const logBody = ref<HTMLElement | null>(null)

const logs = computed(() => getLogs(props.command))
const hasLogs = computed(() => !!logs.value)

const copyCommand = async () => {
  try {
    await navigator.clipboard.writeText(props.command)
    showCopied.value = true
    setTimeout(() => {
      showCopied.value = false
    }, 2000)
  } catch (err) {
    console.error('Failed to copy command:', err)
  }
}

const runCommand = async () => {
  if (isRunning.value) return
  isRunning.value = true
  showLogs.value = true
  
  // Clear previous logs on re-run (per requirement: prior logs are overridden)
  setLogs(props.command, '')
  
  try {
    const result = await RunCommand(props.command)
    setLogs(props.command, result || 'Command executed successfully with no output.')
  } catch (err) {
    setLogs(props.command, `Error: ${err}`)
  } finally {
    isRunning.value = false
  }
}

const handleCommandClick = () => {
  if (hasLogs.value) {
    showLogs.value = true
  } else {
    runCommand()
  }
}

const handleMouseEnter = () => {
  isHovered.value = true
}

const handleMouseLeave = () => {
  isHovered.value = false
}

// Auto-scroll logs to bottom
watch(() => logs.value, () => {
  if (showLogs.value) {
    nextTick(() => {
      if (logBody.value) {
        logBody.value.scrollTop = logBody.value.scrollHeight
      }
    })
  }
})
</script>

<style scoped>
.command-block {
  position: relative;
  cursor: pointer;
  transition: all 0.2s ease;
  margin-bottom: 8px;
  display: block;
}

.command-content {
  background: rgba(31, 41, 55, 0.5);
  border: 1px solid rgba(75, 85, 99, 0.3);
  border-radius: 8px;
  padding: 12px 16px;
  transition: all 0.3s ease;
}

.command-content code {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.5;
  color: #e5e7eb;
  word-break: break-all;
  background: transparent;
  padding: 0;
  border: none;
}

.command-content.isRunning {
  background: transparent !important;
  border-color: rgba(75, 85, 99, 0.6) !important;
}

.command-content.hasLogs {
  border-color: rgba(59, 130, 246, 0.4);
}

.command-content.isRunning code {
  color: transparent !important;
}

.copy-icon {
  position: absolute;
  top: 8px;
  right: 8px;
  opacity: 0.6;
  transition: opacity 0.2s ease;
  cursor: pointer;
  z-index: 2;
  color: #9ca3af;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
}

.copy-icon:hover {
  opacity: 1;
  color: #f3f4f6;
}

.copied-text {
  position: absolute;
  top: -24px;
  right: 0;
  background: #10b981;
  color: white;
  padding: 2px 8px;
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  pointer-events: none;
}

.hover-text {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  background: transparent;
  color: #3b82f6;
  padding: 8px 16px;
  border-radius: 4px;
  font-size: 13px;
  font-weight: 700;
  pointer-events: none;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-shadow: 0 0 8px rgba(59, 130, 246, 0.4);
  white-space: nowrap;
}

/* Log Layover Styles */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(4px);
  z-index: 100;
  cursor: default;
}

.log-layover {
  position: fixed;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 80vw;
  max-width: 900px;
  height: 70vh;
  background: #111827;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 12px;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  z-index: 101;
  overflow: hidden;
  cursor: default;
}

.log-header {
  padding: 16px 20px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(31, 41, 55, 0.5);
}

.log-title {
  color: #f3f4f6;
  font-size: 15px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 12px;
}

.log-title code {
  background: rgba(0, 0, 0, 0.3);
  padding: 2px 6px;
  border-radius: 4px;
  color: #9ca3af;
  font-size: 13px;
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: #10b981;
}

.status-indicator.pulse {
  background: #3b82f6;
  animation: pulse-blue 2s infinite;
}

@keyframes pulse-blue {
  0% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(59, 130, 246, 0.7); }
  70% { transform: scale(1); box-shadow: 0 0 0 10px rgba(59, 130, 246, 0); }
  100% { transform: scale(0.95); box-shadow: 0 0 0 0 rgba(59, 130, 246, 0); }
}

.close-btn {
  background: transparent;
  border: none;
  color: #9ca3af;
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  transition: all 0.2s ease;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: #f3f4f6;
}

.log-body {
  flex: 1;
  padding: 20px;
  overflow-y: auto;
  font-family: 'JetBrains Mono', 'Fira Code', 'Monaco', monospace;
  background: #000;
}

.log-body pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  color: #d1d5db;
  font-size: 13px;
  line-height: 1.6;
}

.empty-state {
  color: #4b5563;
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-style: italic;
}

.log-footer {
  padding: 12px 20px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: space-between;
  background: rgba(31, 41, 55, 0.5);
}

.last-run-text, .executing-text {
  font-size: 12px;
  color: #9ca3af;
}

.executing-text {
  color: #3b82f6;
}

.re-run-btn {
  background: #3b82f6;
  color: white;
  border: none;
  padding: 6px 14px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
}

.re-run-btn:hover:not(:disabled) {
  background: #2563eb;
}

.re-run-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* Transitions */
.modal-enter-active, .modal-leave-active {
  transition: all 0.3s ease;
}
.modal-enter-from, .modal-leave-to {
  opacity: 0;
  transform: translate(-50%, -45%);
}

.fade-rise-enter-active {
  animation: rise 2s ease-out forwards;
}
.fade-rise-leave-active {
  opacity: 0;
}

@keyframes rise {
  0% {
    opacity: 0;
    transform: translateY(10px);
  }
  15% {
    opacity: 1;
    transform: translateY(0);
  }
  85% {
    opacity: 1;
    transform: translateY(-10px);
  }
  100% {
    opacity: 0;
    transform: translateY(-20px);
  }
}

.hover-enter-active,
.hover-leave-active {
  transition: opacity 0.3s ease;
}

.hover-enter-from,
.hover-leave-to {
  opacity: 0;
}
</style>
