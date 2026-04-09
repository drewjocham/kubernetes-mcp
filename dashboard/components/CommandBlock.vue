<template>
  <div
    class="command-block"
    @click="runCommand"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <div class="command-content">
      <n-code
        :code="command"
        language="bash"
        class="command-code"
        :class="{ 'isRunning': isRunning }"
      />
      <div
        v-if="isRunning"
        class="running-spinner"
      >
        <svg
          width="16"
          height="16"
          viewBox="0 0 24 24"
          xmlns="http://www.w3.org/2000/svg"
        >
          <circle
            cx="12"
            cy="12"
            r="10"
            stroke="rgba(59, 130, 246, 0.3)"
            stroke-width="4"
            fill="none"
          />
          <circle
            cx="12"
            cy="12"
            r="10"
            stroke="#3b82f6"
            stroke-width="4"
            fill="none"
            stroke-linecap="round"
            stroke-dasharray="60"
            stroke-dashoffset="40"
          >
            <animateTransform
              attributeName="transform"
              type="rotate"
              from="0 12 12"
              to="360 12 12"
              dur="1s"
              repeatCount="indefinite"
            />
          </circle>
        </svg>
      </div>
    </div>
    <div
      class="copy-icon"
      @click.stop="copyCommand"
    >
      <n-icon
        size="16"
        :component="CopyIcon"
      />
      <transition name="fade">
        <span
          v-if="showCopied"
          class="copied-text"
        >copied</span>
      </transition>
    </div>
    <transition name="hover">
      <div
        v-if="isHovered && !isRunning"
        class="hover-text"
      >
        Run Command
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { NCode, NIcon } from 'naive-ui'
import { Copy16Regular as CopyIcon } from '@vicons/fluent'

interface Props {
  command: string
}

const props = defineProps<Props>()

const isHovered = ref(false)
const isRunning = ref(false)
const showCopied = ref(false)

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

const runCommand = () => {
  if (isRunning.value) return
  isRunning.value = true
  // Simulate running command
  console.log('Running command:', props.command)
  
  const startTime = Date.now()
  const MIN_RUN_TIME = 500 // ms
  
  // Here you could call an API to execute the command
  // For now, simulate execution with minimum runtime
  setTimeout(() => {
    const elapsed = Date.now() - startTime
    const remaining = Math.max(0, MIN_RUN_TIME - elapsed)
    setTimeout(() => {
      isRunning.value = false
    }, remaining)
  }, 100) // Simulate actual execution time (shorter, but with minimum visual feedback)
}

const handleMouseEnter = () => {
  isHovered.value = true
}

const handleMouseLeave = () => {
  isHovered.value = false
}
</script>

<style scoped>
.command-block {
  position: relative;
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease;
}

.command-content {
  transition: opacity 0.3s ease;
}

.command-code.isRunning {
  background: rgba(31, 41, 55, 0.8) !important;
  border-color: rgba(59, 130, 246, 0.6) !important;
  color: #9ca3af !important;
  position: relative;
}

.command-code.isRunning :deep(*) {
  color: #9ca3af !important;
  background: transparent !important;
  opacity: 0.8;
}

.command-code {
  background: rgba(31, 41, 55, 0.5);
  border: 1px solid rgba(75, 85, 99, 0.3);
  border-radius: 8px;
  padding: 16px;
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 14px;
  line-height: 1.5;
}

.copy-icon {
  position: absolute;
  top: 8px;
  right: 8px;
  opacity: 0.6;
  transition: opacity 0.2s ease;
  cursor: pointer;
  z-index: 2;
}

.copy-icon:hover {
  opacity: 1;
}

.copied-text {
  position: absolute;
  top: -20px;
  right: 0;
  background: #10b981;
  color: white;
  padding: 4px 8px;
  border-radius: 4px;
  font-size: 12px;
  font-weight: 500;
  animation: rise 2s ease-out forwards;
}

@keyframes rise {
  0% {
    opacity: 1;
    transform: translateY(0);
  }
  100% {
    opacity: 0;
    transform: translateY(-20px);
  }
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
  font-size: 14px;
  font-weight: 600;
  pointer-events: none;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  text-shadow: 0 0 8px rgba(59, 130, 246, 0.3);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.5s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

.hover-enter-active,
.hover-leave-active {
  transition: opacity 0.3s ease;
}

.hover-enter-from,
.hover-leave-to {
  opacity: 0;
}

.running-spinner {
  position: absolute;
  top: 50%;
  right: 40px;
  transform: translateY(-50%);
  z-index: 1;
}
</style>