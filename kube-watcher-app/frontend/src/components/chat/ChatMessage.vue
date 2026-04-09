<template>
  <article :class="['chat-bubble', message.role, { preview, [message.tone || '']: message.tone }]">
    <span class="chat-role">{{ message.role === 'assistant' ? 'AI' : 'You' }}</span>
    <div v-if="message.role === 'assistant'" ref="markdownRef" class="markdown-content" v-html="renderMarkdown(message.content)"></div>
    <p v-else>{{ message.content }}</p>
  </article>
</template>

<script setup lang="ts">
import { onMounted, ref, nextTick, watch } from 'vue'

interface ChatMessage {
  role: 'assistant' | 'user'
  content: string
  tone?: string
}

interface Props {
  message: ChatMessage
  renderMarkdown: (text: string) => string
  preview?: boolean
}

const props = defineProps<Props>()
const markdownRef = ref<HTMLElement | null>(null)

const addCopyButtons = () => {
  if (!markdownRef.value || props.preview) return
  
  // Find all pre elements inside the markdown content
  const preElements = markdownRef.value.querySelectorAll('pre')
  preElements.forEach((pre) => {
    // Check if already has copy button
    if (pre.parentElement?.classList.contains('code-block-wrapper')) {
      return
    }
    
    // Create wrapper div
    const wrapper = document.createElement('div')
    wrapper.className = 'code-block-wrapper'
    
    // Create copy button
    const copyButton = document.createElement('button')
    copyButton.className = 'copy-code-button'
    copyButton.textContent = 'Copy'
    copyButton.title = 'Copy to clipboard'
    
    copyButton.addEventListener('click', async () => {
      const code = pre.querySelector('code')?.textContent || pre.textContent || ''
      try {
        await navigator.clipboard.writeText(code)
        copyButton.textContent = 'Copied!'
        copyButton.classList.add('copied')
        setTimeout(() => {
          copyButton.textContent = 'Copy'
          copyButton.classList.remove('copied')
        }, 2000)
      } catch (err) {
        console.error('Failed to copy:', err)
        copyButton.textContent = 'Failed'
        setTimeout(() => {
          copyButton.textContent = 'Copy'
        }, 2000)
      }
    })
    
    // Wrap the pre element
    pre.parentNode?.insertBefore(wrapper, pre)
    wrapper.appendChild(pre)
    wrapper.appendChild(copyButton)
  })
}

onMounted(() => {
  nextTick(() => {
    addCopyButtons()
  })
})

watch(() => props.message.content, () => {
  nextTick(() => {
    addCopyButtons()
  })
})
</script>

<style scoped>
.chat-bubble {
  padding: 16px;
  border-radius: 16px;
  position: relative;
}

.chat-bubble.assistant {
  background: var(--chat-ai-bg);
  border: 1px solid var(--chat-ai-border);
}

.chat-bubble.user {
  background: var(--chat-user-bg);
  border: 1px solid var(--chat-user-border);
  margin-left: 24px;
}

.chat-bubble.preview {
  opacity: 0.8;
  max-height: 100px;
  overflow: hidden;
  position: relative;
}

.chat-bubble.preview::after {
  content: '';
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  height: 40px;
  background: linear-gradient(to bottom, transparent, var(--chat-ai-bg));
}

.chat-bubble.warn {
  border-color: var(--warning);
  background: rgba(245, 158, 11, 0.1);
}

.chat-bubble.calm {
  border-color: var(--success);
  background: rgba(34, 197, 94, 0.1);
}

.chat-role {
  display: block;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin-bottom: 8px;
  font-weight: 500;
}

.markdown-content {
  font-size: 14px;
  line-height: 1.6;
}

.markdown-content :deep(h1),
.markdown-content :deep(h2),
.markdown-content :deep(h3) {
  margin: 1em 0 0.5em 0;
  font-weight: 600;
  color: var(--text-primary);
}

.markdown-content :deep(h1) {
  font-size: 1.5em;
}

.markdown-content :deep(h2) {
  font-size: 1.3em;
}

.markdown-content :deep(h3) {
  font-size: 1.1em;
}

.markdown-content :deep(p) {
  margin: 0.5em 0;
}

.markdown-content :deep(ul),
.markdown-content :deep(ol) {
  margin: 0.5em 0;
  padding-left: 1.5em;
}

.markdown-content :deep(li) {
  margin: 0.25em 0;
}

.markdown-content :deep(code) {
  font-family: 'SF Mono', monospace;
  font-size: 0.9em;
  padding: 2px 4px;
  border-radius: 4px;
  background: var(--code-bg);
  color: var(--code-text);
}

.markdown-content :deep(pre) {
  margin: 1em 0;
  padding: 12px;
  border-radius: 8px;
  background: var(--code-bg);
  overflow-x: auto;
}

.markdown-content :deep(pre code) {
  background: transparent;
  padding: 0;
}

.markdown-content :deep(blockquote) {
  margin: 1em 0;
  padding-left: 1em;
  border-left: 3px solid var(--border);
  color: var(--text-secondary);
}

.markdown-content :deep(table) {
  width: 100%;
  border-collapse: collapse;
  margin: 1em 0;
}

.markdown-content :deep(th),
.markdown-content :deep(td) {
  padding: 8px 12px;
  border: 1px solid var(--border);
  text-align: left;
}

.markdown-content :deep(th) {
  background: var(--hover);
  font-weight: 600;
}

.markdown-content :deep(strong) {
  font-weight: 600;
  color: var(--text-primary);
}

.markdown-content :deep(em) {
  font-style: italic;
}

.markdown-content :deep(a) {
  color: var(--primary);
  text-decoration: none;
}

.markdown-content :deep(a:hover) {
  text-decoration: underline;
}

.markdown-content :deep(.code-block-wrapper) {
  position: relative;
  margin: 1em 0;
}

.markdown-content :deep(.copy-code-button) {
  position: absolute;
  top: 8px;
  right: 8px;
  padding: 4px 8px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  opacity: 0;
  z-index: 10;
}

.markdown-content :deep(.code-block-wrapper:hover .copy-code-button) {
  opacity: 1;
}

.markdown-content :deep(.copy-code-button:hover) {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.markdown-content :deep(.copy-code-button.copied) {
  background: var(--success);
  border-color: var(--success);
  color: white;
  opacity: 1;
}
</style>