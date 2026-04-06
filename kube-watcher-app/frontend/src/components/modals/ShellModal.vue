<template>
  <div class="shell-modal-overlay" @click="emit('close')">
    <div class="shell-modal" @click.stop>
      <div class="shell-modal-header">
        <h3>Terminal: {{ podTitle }}</h3>
        <button class="shell-modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="shell-modal-content">
        <!-- Container selector for multi-container pods -->
        <div v-if="pod && pod.containers && pod.containers.length > 1" class="container-selector">
          <label for="container-select">Container:</label>
          <select id="container-select" v-model="selectedContainer">
            <option v-for="container in pod.containers" :key="container.name" :value="container.name">
              {{ container.name }}
            </option>
          </select>
        </div>

        <!-- Terminal output -->
        <div class="terminal-output" ref="terminalOutput">
          <div v-for="(line, index) in terminalLines" :key="index" class="terminal-line">
            <span v-if="line.type === 'command'" class="prompt">$ </span>
            <span v-if="line.type === 'command'" class="command-text">{{ line.text }}</span>
            <pre v-if="line.type === 'output'" class="output-text">{{ line.text }}</pre>
            <div v-if="line.type === 'error'" class="error-text">Error: {{ line.text }}</div>
          </div>
          <div v-if="isExecuting" class="executing-indicator">
            <span class="prompt">$ </span>
            <span class="command-text">{{ currentCommand }}</span>
            <span class="blinking-cursor">█</span>
          </div>
        </div>

        <!-- Command input -->
        <div class="command-input-wrapper">
          <div class="command-input-area">
            <span class="prompt">$ </span>
            <input
              type="text"
              v-model="currentCommand"
              @keyup.enter="executeCommand"
              @keyup.up="handleVerticalArrow(-1)"
              @keyup.down="handleVerticalArrow(1)"
              @keydown.tab.prevent="handleTab"
              @keydown.esc="showSuggestions = false"
              @input="handleInput"
              :disabled="isExecuting"
              placeholder="Type command and press Enter..."
              ref="commandInput"
              class="command-input"
            />
            <button class="execute-btn" @click="executeCommand" :disabled="isExecuting || !currentCommand.trim()">
              Execute
            </button>
          </div>

          <!-- Command suggestions -->
          <div v-if="showSuggestions && filteredSuggestions.length > 0" class="suggestions-dropdown">
            <div
              v-for="(suggestion, index) in filteredSuggestions"
              :key="suggestion"
              :class="['suggestion-item', { 'selected': index === suggestionIndex }]"
              @click="selectSuggestion(suggestion)"
              @mouseenter="suggestionIndex = index"
            >
              {{ suggestion }}
            </div>
          </div>
        </div>

        <div class="terminal-help">
          <small>Press ↑/↓ to navigate command history • Type 'exit' to close</small>
        </div>
      </div>

      <div class="shell-modal-footer">
        <button class="clear-btn" @click="clearTerminal">Clear Terminal</button>
        <button class="close-btn" @click="emit('close')">Close Terminal</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue'
import { data } from '../../../wailsjs/go/models'
import { ExecPodCommand } from '../../../wailsjs/go/main/App'

interface Props {
  pod: data.PodInfo | null
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
}>()

const podTitle = computed(() => {
  if (!props.pod) return ''
  return `${props.pod.namespace}/${props.pod.name}`
})

const selectedContainer = ref('')
const currentCommand = ref('')
const terminalLines = ref<Array<{type: 'command' | 'output' | 'error', text: string}>>([])
const commandHistory = ref<string[]>([])
const historyIndex = ref(-1)
const isExecuting = ref(false)
const terminalOutput = ref<HTMLElement | null>(null)
const commandInput = ref<HTMLInputElement | null>(null)

// Auto-complete suggestions
const suggestions = ref<string[]>([])
const suggestionIndex = ref(-1)
const showSuggestions = ref(false)

const commonCommands = [
  'ls', 'cd', 'pwd', 'cat', 'echo', 'grep', 'awk', 'sed', 'find',
  'ps', 'top', 'vi', 'nano', 'curl', 'wget', 'tar', 'gzip', 'kill',
  'ping', 'netstat', 'ifconfig', 'df', 'du', 'mount', 'uname',
  'whoami', 'id', 'env', 'export', 'source', 'sh', 'bash',
  'python', 'python3', 'node', 'npm', 'java', 'docker', 'kubectl', 'helm'
]

// Initialize selected container
watch(() => props.pod, (newPod) => {
  if (newPod && newPod.containers && newPod.containers.length > 0) {
    selectedContainer.value = newPod.containers[0].name
  }
}, { immediate: true })

// Auto-complete suggestions
const filteredSuggestions = computed(() => {
  const input = currentCommand.value.trim()
  if (!input) return []
  
  // Get current word being typed (last word in command)
  const words = input.split(' ')
  const currentWord = words[words.length - 1]
  
  // Combine common commands and command history
  const allSuggestions = [...new Set([...commonCommands, ...commandHistory.value])]
  
  // Filter suggestions that start with current word
  return allSuggestions.filter(cmd => cmd.startsWith(currentWord))
})

// Auto-scroll terminal output
watch(terminalLines, () => {
  nextTick(() => {
    if (terminalOutput.value) {
      terminalOutput.value.scrollTop = terminalOutput.value.scrollHeight
    }
  })
}, { deep: true })

// Focus input when modal opens
onMounted(() => {
  nextTick(() => {
    if (commandInput.value) {
      commandInput.value.focus()
    }
  })
})

async function executeCommand() {
  const command = currentCommand.value.trim()
  if (!command || !props.pod || isExecuting.value) return

  // Handle exit command
  if (command.toLowerCase() === 'exit') {
    emit('close')
    return
  }

  // Add command to history and terminal
  commandHistory.value.push(command)
  historyIndex.value = -1
  terminalLines.value.push({ type: 'command', text: command })
  
  const cmdToExecute = command
  currentCommand.value = ''
  isExecuting.value = true

  try {
    console.log('Executing command in pod:', props.pod.namespace, props.pod.name, selectedContainer.value, cmdToExecute)
    const output = await ExecPodCommand(props.pod.namespace, props.pod.name, selectedContainer.value, cmdToExecute)
    terminalLines.value.push({ type: 'output', text: output })
  } catch (err) {
    console.error('Failed to execute command:', err)
    const errMsg = err instanceof Error ? err.message : String(err)
    terminalLines.value.push({ type: 'error', text: errMsg })
  } finally {
    isExecuting.value = false
    nextTick(() => {
      if (commandInput.value) {
        commandInput.value.focus()
      }
    })
  }
}

function navigateHistory(direction: number) {
  if (commandHistory.value.length === 0) return
  
  let newIndex = historyIndex.value + direction
  if (newIndex < -1) newIndex = -1
  if (newIndex >= commandHistory.value.length) newIndex = commandHistory.value.length - 1
  
  historyIndex.value = newIndex
  if (newIndex === -1) {
    currentCommand.value = ''
  } else {
    currentCommand.value = commandHistory.value[commandHistory.value.length - 1 - newIndex]
  }
}

function clearTerminal() {
  terminalLines.value = []
}

function handleTab() {
  if (filteredSuggestions.value.length === 0) {
    // No suggestions, do nothing
    return
  }

  // Show suggestions if hidden
  if (!showSuggestions.value) {
    showSuggestions.value = true
  }

  // Get current word being typed
  const input = currentCommand.value.trim()
  const words = input.split(' ')
  const currentWord = words[words.length - 1]

  // Cycle suggestion index
  let newIndex = suggestionIndex.value + 1
  if (newIndex >= filteredSuggestions.value.length) {
    newIndex = 0
  }
  suggestionIndex.value = newIndex

  const suggestion = filteredSuggestions.value[newIndex]
  
  // Replace the current word with the suggestion
  words[words.length - 1] = suggestion
  currentCommand.value = words.join(' ')
  
  // Move cursor to end
  nextTick(() => {
    if (commandInput.value) {
      commandInput.value.focus()
      commandInput.value.setSelectionRange(
        currentCommand.value.length,
        currentCommand.value.length
      )
    }
  })
}

function handleInput() {
  const input = currentCommand.value.trim()
  if (input) {
    showSuggestions.value = true
    suggestionIndex.value = -1
  } else {
    showSuggestions.value = false
  }
}

function selectSuggestion(suggestion: string) {
  const input = currentCommand.value.trim()
  const words = input.split(' ')
  words[words.length - 1] = suggestion
  currentCommand.value = words.join(' ')
  showSuggestions.value = false
  suggestionIndex.value = -1
  
  // Focus back on input
  nextTick(() => {
    if (commandInput.value) {
      commandInput.value.focus()
    }
  })
}

function handleVerticalArrow(direction: number) {
  if (showSuggestions.value && filteredSuggestions.value.length > 0) {
    // Navigate suggestions
    let newIndex = suggestionIndex.value + direction
    if (newIndex < 0) newIndex = filteredSuggestions.value.length - 1
    if (newIndex >= filteredSuggestions.value.length) newIndex = 0
    suggestionIndex.value = newIndex
    showSuggestions.value = true
  } else {
    // Navigate command history
    navigateHistory(direction)
  }
}
</script>

<style scoped>
.shell-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
   background: rgba(0, 0, 0, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

.shell-modal {
  background: var(--panel-bg, #1e293b);
  border-radius: 24px;
  width: 90%;
  max-width: 900px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--border);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.shell-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border);
}

.shell-modal-header h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}

.shell-modal-close-btn {
  background: transparent;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  border-radius: 6px;
}

.shell-modal-close-btn:hover {
  background: var(--hover);
}

.shell-modal-content {
  padding: 24px;
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 16px;
  overflow: hidden;
}

.container-selector {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: var(--hover);
  border-radius: 12px;
  border: 1px solid var(--border);
}

.container-selector label {
  font-weight: 600;
  color: var(--text-secondary);
}

.container-selector select {
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  flex: 1;
}

.terminal-output {
  flex: 1;
  background: #000;
  border-radius: 12px;
  padding: 20px;
  overflow-y: auto;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  line-height: 1.5;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.terminal-line {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.prompt {
  color: #10b981;
  font-weight: bold;
  margin-right: 8px;
  user-select: none;
}

.command-text {
  color: #ffffff;
  font-weight: 500;
}

.output-text {
  color: #d1d5db;
  margin: 4px 0 8px 20px;
  white-space: pre-wrap;
  word-break: break-all;
  font-family: inherit;
}

.error-text {
  color: #ef4444;
  margin: 4px 0 8px 20px;
  white-space: pre-wrap;
}

.executing-indicator {
  display: flex;
  align-items: center;
}

.blinking-cursor {
  color: #10b981;
  animation: blink 1s infinite;
  margin-left: 2px;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.command-input-area {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  background: var(--hover);
  border-radius: 12px;
  border: 1px solid var(--border);
}

.command-input {
  flex: 1;
  padding: 8px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
}

.command-input:focus {
  outline: none;
  border-color: #10b981;
}

.execute-btn {
  padding: 8px 16px;
  border-radius: 8px;
  border: none;
  background: #10b981;
  color: white;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.2s;
}

.execute-btn:hover:not(:disabled) {
  background: #0da271;
}

.execute-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.terminal-help {
  text-align: center;
  color: var(--text-secondary);
  font-size: 12px;
  padding: 8px;
}

.command-input-wrapper {
  position: relative;
}

.suggestions-dropdown {
  position: absolute;
  bottom: calc(100% + 8px);
  left: 16px;
  right: 16px;
  background: var(--panel-bg);
  border: 1px solid var(--border);
  border-radius: 8px;
  max-height: 200px;
  overflow-y: auto;
  z-index: 10;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.2);
}

.suggestion-item {
  padding: 8px 12px;
  cursor: pointer;
  color: var(--text-primary);
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
  border-bottom: 1px solid var(--border);
}

.suggestion-item:last-child {
  border-bottom: none;
}

.suggestion-item:hover,
.suggestion-item.selected {
  background: var(--hover);
  color: #10b981;
}

.shell-modal-footer {
  padding: 20px 24px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.clear-btn, .close-btn {
  padding: 10px 20px;
  border-radius: 8px;
  border: none;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.clear-btn {
  background: transparent;
  color: var(--text-secondary);
  border: 1px solid var(--border);
}

.clear-btn:hover {
  background: var(--hover);
}

.close-btn {
  background: var(--primary);
  color: white;
}

.close-btn:hover {
  background: var(--primary-hover);
}
</style>