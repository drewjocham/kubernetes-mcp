<template>
  <div v-if="show" class="error-modal-overlay" @click="closeModal">
    <div class="error-modal" @click.stop>
      <div class="error-modal-header">
        <h3>{{ title }}</h3>
        <button class="error-modal-close-btn" @click="closeModal">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="error-modal-content">
        <div class="error-summary">
          <div class="error-icon">
            <svg width="48" height="48" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M12 8V12M12 16H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="#ef4444" stroke-width="2" stroke-linecap="round"/>
            </svg>
          </div>
          <div class="error-details">
            <h4>{{ summary }}</h4>
            <p class="error-message">{{ error }}</p>
          </div>
        </div>

        <div v-if="recommendations.length > 0" class="recommendations-section">
          <h4>Recommended Actions</h4>
          <ul class="recommendations-list">
            <li v-for="(recommendation, index) in recommendations" :key="index" class="recommendation-item">
              <span class="recommendation-number">{{ index + 1 }}</span>
              <span class="recommendation-text">{{ recommendation }}</span>
            </li>
          </ul>
        </div>

        <div class="error-modal-actions">
          <button class="ghost-btn" @click="closeModal">Dismiss</button>
          <button v-if="showInvestigate" class="primary-btn" @click="investigate">
            Investigate with AI
          </button>
          <button v-if="showRetry" class="primary-btn" @click="retry">
            Try Again
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { AskAI } from '../../../wailsjs/go/main/App'

interface Props {
  show: boolean
  title?: string
  summary: string
  error: string
  context?: any
  showRetry?: boolean
  showInvestigate?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  title: 'Error',
  showRetry: false,
  showInvestigate: true,
  context: null,
})

const emit = defineEmits<{
  'update:show': [value: boolean]
  close: []
  retry: []
  investigate: [context: any]
}>()

const recommendations = ref<string[]>([])
const isLoading = ref(false)

function closeModal() {
  emit('update:show', false)
  emit('close')
}

async function retry() {
  emit('retry')
  closeModal()
}

async function investigate() {
  isLoading.value = true
  try {
    // Send error context to AI for analysis
    const aiContext = JSON.stringify({
      error: props.error,
      summary: props.summary,
      context: props.context,
      timestamp: new Date().toISOString()
    })
    
    const aiResponse = await AskAI(
      `I encountered this error: "${props.error}". Please analyze what might have gone wrong and suggest specific troubleshooting steps.`,
      aiContext
    )
    
    // Parse AI response for recommendations (simple approach)
    const lines = aiResponse.split('\n').filter((line: string) => 
      line.trim().length > 0 && 
      (line.includes('•') || line.includes('-') || line.includes('1.') || line.includes('2.') || line.includes('3.'))
    )
    
    if (lines.length > 0) {
      recommendations.value = lines.slice(0, 3).map((line: string) => line.replace(/^[•\-]\s*|\d+\.\s*/g, '').trim())
    } else {
      // Fallback recommendations
      recommendations.value = [
        'Check your network connection',
        'Verify the input data is valid',
        'Try refreshing the page'
      ]
    }
    
    emit('investigate', { error: props.error, context: props.context, aiResponse })
  } catch (err) {
    console.error('Failed to get AI analysis:', err)
    recommendations.value = [
      'Failed to get AI analysis. Please check your internet connection.',
      'Try the operation again.',
      'Contact support if the issue persists.'
    ]
  } finally {
    isLoading.value = false
  }
}

// Auto-investigate on open if showInvestigate is true
onMounted(() => {
  if (props.show && props.showInvestigate) {
    investigate()
  }
})
</script>

<style scoped>
.error-modal-overlay {
  position: fixed;
  inset: 0;
   background: rgba(0, 0, 0, 0.85);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  pointer-events: auto;
}

.error-modal {
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

.error-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24px;
  border-bottom: 1px solid var(--border);
}

.error-modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
}

.error-modal-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 6px;
  transition: color 0.2s;
}

.error-modal-close-btn:hover {
  color: var(--text);
  background: rgba(255, 255, 255, 0.1);
}

.error-modal-content {
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

.primary-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>