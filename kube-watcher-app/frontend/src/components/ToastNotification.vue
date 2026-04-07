<template>
  <Transition name="toast">
    <div v-if="show" :class="['toast', type]" @click="close">
      <div class="toast-icon">
        <svg v-if="type === 'success'" width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M9 12L11 14L15 10M21 12C21 16.9706 16.9706 21 12 21C7.02944 21 3 16.9706 3 12C3 7.02944 7.02944 3 12 3C16.9706 3 21 7.02944 21 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
        <svg v-if="type === 'error'" width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M12 8V12M12 16H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
        <svg v-if="type === 'info'" width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M13 16H12V12H11M12 8H12.01M21 12C21 16.9706 16.9706 21 12 21C7.02944 21 3 16.9706 3 12C3 7.02944 7.02944 3 12 3C16.9706 3 21 7.02944 21 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </div>
      <div class="toast-content">
        <div class="toast-title">{{ title }}</div>
        <div class="toast-message">{{ message }}</div>
      </div>
      <button class="toast-close" @click.stop="close">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
          <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
        </svg>
      </button>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

interface Props {
  title?: string
  message: string
  type?: 'success' | 'error' | 'info'
  duration?: number
  show: boolean
}

const props = withDefaults(defineProps<Props>(), {
  type: 'info',
  duration: 3000,
  title: '',
})

const emit = defineEmits<{
  close: []
}>()

const show = ref(props.show)
let timeoutId: number | null = null

function close() {
  show.value = false
  if (timeoutId) {
    clearTimeout(timeoutId)
    timeoutId = null
  }
  setTimeout(() => {
    emit('close')
  }, 300) // Wait for transition
}

onMounted(() => {
  if (props.duration > 0) {
    timeoutId = setTimeout(() => {
      close()
    }, props.duration)
  }
})
</script>

<style scoped>
.toast {
  position: fixed;
  top: 20px;
  right: 20px;
  min-width: 300px;
  max-width: 400px;
  background: var(--surface-strong);
  border: 1px solid var(--border);
  border-radius: 12px;
  padding: 16px;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
  z-index: 9999;
  animation: rise 0.5s ease-out forwards;
  cursor: pointer;
  transition: transform 0.2s;
}

.toast:hover {
  transform: translateY(-2px);
}

.toast.success {
  border-left: 4px solid var(--success);
}

.toast.error {
  border-left: 4px solid var(--error);
}

.toast.info {
  border-left: 4px solid var(--info);
}

.toast-icon {
  flex-shrink: 0;
  padding-top: 2px;
}

.toast.success .toast-icon {
  color: var(--success);
}

.toast.error .toast-icon {
  color: var(--error);
}

.toast.info .toast-icon {
  color: var(--info);
}

.toast-content {
  flex: 1;
  min-width: 0;
}

.toast-title {
  font-weight: 600;
  font-size: 14px;
  margin-bottom: 4px;
  color: var(--text-primary);
}

.toast-message {
  font-size: 13px;
  line-height: 1.4;
  color: var(--text-secondary);
}

.toast-close {
  flex-shrink: 0;
  background: transparent;
  border: none;
  color: var(--text-tertiary);
  cursor: pointer;
  padding: 2px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.toast-close:hover {
  background: var(--hover);
  color: var(--text-primary);
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.3s ease;
}

.toast-enter-from {
  opacity: 0;
  transform: translateX(100%);
}

.toast-leave-to {
  opacity: 0;
  transform: translateX(100%);
}

@keyframes rise {
  0% {
    transform: translateY(100px);
    opacity: 0;
  }
  100% {
    transform: translateY(0);
    opacity: 1;
  }
}
</style>