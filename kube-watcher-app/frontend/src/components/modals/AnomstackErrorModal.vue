<template>
  <div v-if="show" class="anomstack-error-modal-overlay" @click="closeModal">
    <div class="anomstack-error-modal" @click.stop>
      <div class="anomstack-error-modal-header">
        <h3>Connection Failed</h3>
         <button class="anomstack-error-modal-close-btn" @click="closeModal">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="anomstack-error-modal-content">
        <div class="error-summary">
          <div class="error-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 2C6.48 2 2 6.48 2 12C2 17.52 6.48 22 12 22C17.52 22 22 17.52 22 12C22 6.48 17.52 2 12 2ZM13 17H11V15H13V17ZM13 13H11V7H13V13Z" fill="#ef4444"/>
            </svg>
          </div>
          <div class="error-details">
            <h4>Unable to connect to Anomalies service</h4>
            <p class="error-message">{{ error }}</p>
          </div>
        </div>

        <div class="recommendations-section">
          <h4>Recommended Actions</h4>
          <ul class="recommendations-list">
            <li v-for="(recommendation, index) in recommendations" :key="index" class="recommendation-item">
              <span class="recommendation-number">{{ index + 1 }}</span>
              <span class="recommendation-text">{{ recommendation }}</span>
            </li>
          </ul>
        </div>

        <div class="error-modal-actions">
           <button class="ghost-btn" @click="closeModal">Cancel</button>
          <button class="primary-btn" @click="emit('retry')">Try Again</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
interface Props {
  show: boolean
  error: string
  recommendations: string[]
}

defineProps<Props>()
const emit = defineEmits<{
  'update:show': [value: boolean]
  close: []
  retry: []
}>()

function closeModal() {
  emit('update:show', false)
  emit('close')
}
</script>

<style scoped>
.anomstack-error-modal-overlay {
  position: fixed;
  inset: 0;
   background: rgba(0, 0, 0, 0.85);
  z-index: 999;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  pointer-events: auto;
}

.anomstack-error-modal {
  background: var(--surface-strong);
  border: 1px solid var(--border);
  border-radius: 24px;
  box-shadow: var(--shadow);
  max-width: 500px;
  width: 100%;
  max-height: 80vh;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.anomstack-error-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid var(--border);
}

.anomstack-error-modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.anomstack-error-modal-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  transition: color 0.2s;
}

.anomstack-error-modal-close-btn:hover {
  color: var(--text);
  background: rgba(255, 255, 255, 0.1);
}

.anomstack-error-modal-content {
  padding: 24px;
}

.error-summary {
  display: flex;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 24px;
  padding: 20px;
  background: rgba(239, 68, 68, 0.1);
  border: 1px solid rgba(239, 68, 68, 0.2);
  border-radius: 12px;
}

.error-icon {
  flex-shrink: 0;
}

.error-details h4 {
  margin: 0 0 8px 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}

.error-message {
  margin: 0;
  color: var(--text-muted);
  font-size: 14px;
  line-height: 1.4;
}

.recommendations-section h4 {
  margin: 0 0 16px 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}

.recommendations-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.recommendation-item {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 8px;
}

.recommendation-number {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: var(--accent);
  color: var(--text-primary);
  border-radius: 50%;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
}

.recommendation-text {
  color: var(--text);
  font-size: 14px;
  line-height: 1.4;
  margin: 0;
}

.error-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  margin-top: 24px;
  padding-top: 20px;
  border-top: 1px solid var(--border);
}

.ghost-btn {
  padding: 10px 20px;
  border-radius: 12px;
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

.primary-btn:hover {
  background: var(--primary-hover);
}
</style>