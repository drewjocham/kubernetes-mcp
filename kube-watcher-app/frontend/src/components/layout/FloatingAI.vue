<template>
  <div class="floating-ai">
    <!-- Popup -->
    <div v-if="showPopup" class="ai-popup">
      <div class="popup-header">
        <div class="popup-title">
          <span class="popup-icon">🤖</span>
          <h3>AI Assistant</h3>
        </div>
        <button class="popup-close" @click="showPopup = false">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
      
      <div class="popup-content">
        <!-- Chat Messages -->
        <div class="chat-messages">
          <div v-for="(message, index) in displayMessages" :key="index" :class="['message', message.role]">
            <div class="message-avatar">
              <span v-if="message.role === 'assistant'">🤖</span>
              <span v-else>👤</span>
            </div>
            <div class="message-content">
              <div class="message-text">{{ message.content }}</div>
              <div class="message-time">{{ formatTime(message.timestamp) }}</div>
            </div>
          </div>
        </div>
        
        <!-- Input Area -->
        <div class="chat-input">
          <textarea
            v-model="input"
            placeholder="Ask the AI assistant..."
            @keydown.enter.exact.prevent="sendMessage"
            rows="2"
            class="chat-textarea"
          />
          <button @click="sendMessage" :disabled="!input.trim()" class="send-btn">
            Send
          </button>
        </div>
      </div>
    </div>
    
    <!-- Floating Button -->
    <button class="floating-btn" @click="togglePopup" :title="showPopup ? 'Close AI' : 'Open AI Assistant'">
      <span class="mascot">👁️</span>
    </button>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { PhRobot, PhUser, PhEye } from '@phosphor-icons/vue'

interface ChatMessage {
  role: 'assistant' | 'user'
  content: string
  timestamp?: Date
  tone?: string
}

interface ChatMessageWithTimestamp extends ChatMessage {
  timestamp: Date
}

interface Props {
  messages?: ChatMessage[]
  sendPrompt?: (prompt: string) => Promise<void> | void
}

const props = withDefaults(defineProps<Props>(), {
  messages: undefined,
  sendPrompt: undefined
})

const showPopup = ref(false)
const input = ref('')
const internalMessages = ref<ChatMessageWithTimestamp[]>([
  {
    role: 'assistant',
    content: 'Hello! I\'m your AI assistant. How can I help you with your Kubernetes monitoring today?',
    timestamp: new Date()
  }
])

const displayMessages = computed<ChatMessageWithTimestamp[]>(() => {
  if (props.messages) {
    // Convert chatMessages format to our format (add timestamp if missing)
    return props.messages.map(msg => ({
      role: msg.role,
      content: msg.content,
      timestamp: 'timestamp' in msg ? msg.timestamp as Date : new Date()
    })) as ChatMessageWithTimestamp[]
  }
  return internalMessages.value
})

function togglePopup() {
  showPopup.value = !showPopup.value
}

async function sendMessage() {
  const text = input.value.trim()
  if (!text) return
  
  if (props.sendPrompt) {
    // Use external sendPrompt function
    await props.sendPrompt(text)
  } else {
    // Fallback to internal simulation
    internalMessages.value.push({
      role: 'user',
      content: text,
      timestamp: new Date()
    })
    input.value = ''
    
    setTimeout(() => {
      internalMessages.value.push({
        role: 'assistant',
        content: `I received your message: "${text}". This is a simulated response. In a real implementation, this would connect to an AI API.`,
        timestamp: new Date()
      })
    }, 1000)
    return
  }
  
  input.value = ''
}

function formatTime(date: Date) {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
</script>

<style scoped>
.floating-ai {
  position: fixed;
  bottom: 24px;
  right: 24px;
  z-index: 1000;
}

.floating-btn {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: linear-gradient(145deg, var(--primary), var(--accent));
  border: none;
  color: white;
  font-size: 24px;
  cursor: pointer;
  box-shadow: 0 4px 20px rgba(var(--primary-rgb, 138, 130, 224), 0.4);
  transition: transform 0.2s, box-shadow 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.floating-btn:hover {
  transform: scale(1.05);
  box-shadow: 0 6px 24px rgba(var(--primary-rgb, 138, 130, 224), 0.6);
}

.ai-popup {
  position: absolute;
  bottom: 70px;
  right: 0;
  width: 380px;
  height: 500px;
  background: var(--modal-bg);
  border: 1px solid var(--border);
  border-radius: 16px;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.popup-header {
  padding: 16px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: var(--surface-strong);
}

.popup-title {
  display: flex;
  align-items: center;
  gap: 10px;
}

.popup-icon {
  font-size: 20px;
}

.popup-title h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.popup-close {
  padding: 4px;
  border-radius: 6px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
}

.popup-close:hover {
  background: var(--hover);
  border-color: var(--border-active);
}

.popup-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.chat-messages {
  flex: 1;
  overflow-y: auto;
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.message {
  display: flex;
  gap: 10px;
}

.message.user {
  flex-direction: row-reverse;
}

.message-avatar {
  width: 32px;
  height: 32px;
  border-radius: 50%;
  background: rgba(59, 130, 246, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 16px;
  flex-shrink: 0;
}

.message.user .message-avatar {
  background: rgba(138, 130, 224, 0.1);
}

.message-content {
  max-width: 70%;
  padding: 10px 14px;
  border-radius: 12px;
  background: var(--card-bg);
  border: 1px solid var(--border);
}

.message.user .message-content {
  background: rgba(138, 130, 224, 0.15);
  border-color: rgba(138, 130, 224, 0.3);
}

.message-text {
  font-size: 14px;
  line-height: 1.5;
  margin-bottom: 4px;
}

.message-time {
  font-size: 11px;
  color: var(--text-tertiary);
  text-align: right;
}

.chat-input {
  padding: 16px;
  border-top: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.chat-textarea {
  padding: 12px;
  border-radius: 12px;
  border: 1px solid var(--border);
  background: var(--input-bg);
  color: var(--text-primary);
  font-size: 14px;
  resize: none;
  outline: none;
  transition: border-color 0.2s;
}

.chat-textarea:focus {
  border-color: var(--primary);
}

.send-btn {
  align-self: flex-end;
  padding: 8px 16px;
  border-radius: 999px;
  border: none;
  background: linear-gradient(145deg, var(--primary), var(--accent));
  color: white;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.send-btn:hover:not(:disabled) {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(var(--primary-rgb, 138, 130, 224), 0.4);
}

.send-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>