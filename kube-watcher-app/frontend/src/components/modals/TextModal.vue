<template>
  <div class="text-modal-overlay" @click="emit('close')">
    <div class="text-modal" @click.stop>
      <div class="text-modal-header">
        <h3>{{ title }}</h3>
        <button class="text-modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
      <div class="text-modal-content">
        <pre v-if="isPre">{{ content }}</pre>
        <div v-else>{{ content }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  title: string
  content: string
  isPre?: boolean
}

defineProps<Props>()
const emit = defineEmits<{
  close: []
}>()
</script>

<style scoped>
.text-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
   background: var(--modal-overlay-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

.text-modal {
  background: var(--panel-bg, #ffffff);
  border-radius: 24px;
  width: 90%;
  max-width: 800px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--border);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
}

.text-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border);
}

.text-modal-header h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.text-modal-close-btn {
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

.text-modal-close-btn:hover {
  background: var(--hover);
}

.text-modal-content {
  padding: 24px;
  overflow: auto;
  flex: 1;
  background: var(--modal-content-bg, var(--panel-bg, #ffffff));
}

.text-modal-content pre {
  white-space: pre-wrap;
  font-family: 'Monaco', monospace;
  font-size: 13px;
  margin: 0;
  color: var(--text-primary, #000000);
  line-height: 1.5;
}

.text-modal-content div:not(pre) {
  color: var(--text-primary, #000000);
  line-height: 1.6;
  font-size: 14px;
}
</style>