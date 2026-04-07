<template>
  <aside :class="['arguskube-panel', { expanded: isExpanded }]">
    <div class="panel-head">
      <div>
        <p class="meta-label">Arguskube</p>
      </div>
      <button class="ghost-btn expand-toggle" @click="emit('toggle-expand')">
        {{ isExpanded ? '−' : '+' }}
      </button>
    </div>

    <div v-show="isExpanded" class="prompt-stack">
      <template v-if="playbooks && playbooks.length">
        <button
          v-for="playbook in playbooks"
          :key="playbook.title"
          class="prompt-chip"
          @click="emit('use-playbook', playbook)"
        >
          {{ playbook.title }}
        </button>
      </template>
      <template v-else>
        <button class="prompt-chip" @click="emit('quick-prompt', 'Summarize the blast radius and safest next remediation step.')">
          Summarize blast radius
        </button>
        <button class="prompt-chip" @click="emit('quick-prompt', 'Prepare the minikube anomaly rollout and list the first validation checks.')">
          Prep minikube rollout
        </button>
        <button class="prompt-chip" @click="emit('quick-prompt', 'Turn recent incidents into an operator handoff note.')">
          Create handoff note
        </button>
      </template>
    </div>

    <div class="chat-thread">
      <template v-if="isExpanded">
        <ChatMessage
          v-for="message in messages"
          :key="`${message.role}-${message.content}`"
          :message="message"
          :render-markdown="renderMarkdown"
        />
      </template>
      <template v-else-if="messages.length">
        <ChatMessage
          :message="messages[messages.length - 1]"
          :render-markdown="renderMarkdown"
          preview
        />
      </template>

      <div v-if="isThinking" class="thinking-indicator">
        <div class="arguskube-thinking mini">
          <svg width="24" height="24" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M12 2C6.477 2 2 6.477 2 12C2 17.523 6.477 22 12 22C17.523 22 22 17.523 22 12C22 6.477 17.523 2 12 2Z" stroke="currentColor" stroke-width="1" stroke-opacity="0.2"/>
            <circle cx="9" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
            <circle cx="15" cy="11" r="1.5" fill="currentColor" class="eye-blink"/>
          </svg>
        </div>
        <span>Arguskube is thinking…</span>
      </div>
    </div>

    <div class="composer">
      <textarea
        v-model="draftPrompt"
        rows="4"
        placeholder="Ask the Arguskube for rollout help, incident triage, or anomaly validation."
        @keydown.enter.exact.prevent="sendPrompt"
      />
      <button class="primary-btn" @click="sendPrompt" :disabled="!draftPrompt.trim()">
        Send to Arguskube
      </button>
    </div>
  </aside>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import ChatMessage from '../chat/ChatMessage.vue'
import { data } from '../../../wailsjs/go/models'

interface ChatMessage {
  role: 'assistant' | 'user'
  content: string
  tone?: string
}

interface Props {
  isExpanded: boolean
  messages: ChatMessage[]
  playbooks?: data.AIPlaybook[]
  isThinking: boolean
  renderMarkdown: (text: string) => string
}

const props = defineProps<Props>()
const emit = defineEmits<{
  'toggle-expand': []
  'use-playbook': [playbook: data.AIPlaybook]
  'quick-prompt': [prompt: string]
  'send-prompt': [prompt: string]
}>()

const draftPrompt = ref('')

function sendPrompt() {
  const trimmed = draftPrompt.value.trim()
  if (!trimmed) return
  
  emit('send-prompt', trimmed)
  draftPrompt.value = ''
}
</script>

<style scoped>
.arguskube-panel {
  padding: 22px;
  display: grid;
  grid-template-rows: auto 1fr auto;
  gap: 20px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  max-height: calc(100vh - 200px);
}

.arguskube-panel:not(.expanded) {
  align-content: end;
}

.arguskube-panel:not(.expanded) .chat-thread {
  min-height: auto;
  max-height: 120px;
  overflow: hidden;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.prompt-stack {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.prompt-chip {
  padding: 8px 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--hover);
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.prompt-chip:hover {
  background: var(--active);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.chat-thread {
  display: flex;
  flex-direction: column;
  gap: 16px;
  overflow-y: auto;
  min-height: 200px;
  max-height: 400px;
}

.thinking-indicator {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 16px;
  border-radius: 16px;
  background: var(--hover);
  color: var(--text-secondary);
  font-size: 14px;
}

.arguskube-thinking.mini {
  animation: float 2s ease-in-out infinite;
}

@keyframes float {
  0%, 100% { transform: translateY(0); }
  50% { transform: translateY(-4px); }
}

.composer {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.composer textarea {
  width: 100%;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-family: inherit;
  font-size: 14px;
  resize: vertical;
  min-height: 80px;
  transition: border-color 0.2s;
}

.composer textarea:focus {
  outline: none;
  border-color: var(--border-active);
}

.composer textarea::placeholder {
  color: var(--text-tertiary);
}

.primary-btn {
  padding: 10px 20px;
  border-radius: 12px;
  border: none;
  background: var(--primary);
  color: white;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: background 0.2s;
}

.primary-btn:hover:not(:disabled) {
  background: var(--primary-hover);
}

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ghost-btn {
  padding: 6px 12px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  font-size: 14px;
  cursor: pointer;
  transition: all 0.2s;
}

.ghost-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.expand-toggle {
  width: 32px;
  height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
}
</style>