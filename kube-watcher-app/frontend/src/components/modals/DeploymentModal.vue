<template>
  <div class="deployment-modal-overlay" @click="emit('close')">
    <div class="deployment-modal" @click.stop>
      <div class="deployment-modal-header">
        <h3>{{ title }}</h3>
        <button class="deployment-modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>
      <div class="deployment-modal-content">
        <div v-if="deploying" class="deploying-status">
          <div class="spinner"></div>
          <p>Deploying {{ profile }}...</p>
        </div>
        <div v-else-if="deployError" class="deploy-error">
          <h4>Deployment Failed</h4>
          <pre>{{ deployError }}</pre>
        </div>
        <div v-else-if="deployResult" class="deploy-success">
          <h4>Deployment Successful</h4>
          <pre>{{ deployResult }}</pre>
        </div>
        <div v-else class="deploy-preview">
          <h4>Deployment Preview</h4>
          <p>Click "Start Deployment" to begin deploying {{ profile }}.</p>
          <div v-if="plan" class="plan-details">
            <div class="summary-section">
              <div class="summary-item">
                <span class="summary-label">Commands:</span>
                <span class="summary-value">{{ plan.commands?.length || 0 }}</span>
              </div>
              <div v-if="plan.validation?.length" class="summary-item">
                <span class="summary-label">Validation Steps:</span>
                <span class="summary-value">{{ plan.validation.length }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Services:</span>
                <span class="summary-value">{{ plan.services?.join(', ') || 'None' }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Mode:</span>
                <span class="summary-value">{{ plan.mode }}</span>
              </div>
              <div class="summary-item">
                <span class="summary-label">Namespace:</span>
                <span class="summary-value">{{ plan.namespace }}</span>
              </div>
              <button class="view-commands-btn" @click="showCommands = !showCommands">
                {{ showCommands ? 'Hide Commands' : 'View Commands' }}
              </button>
            </div>
            <div v-if="showCommands">
              <div class="detail-section">
                <h5>Commands</h5>
                <CommandBlock
                  v-for="command in plan.commands"
                  :key="command"
                  :command="command"
                />
              </div>
              <div v-if="plan.validation?.length" class="detail-section">
                <h5>Validation</h5>
                <CommandBlock
                  v-for="check in plan.validation"
                  :key="check"
                  :command="check"
                />
              </div>
            </div>
          </div>
        </div>
        <div class="deployment-modal-footer">
          <button v-if="!deploying && !deployResult" class="primary-btn" @click="startDeployment">
            Start Deployment
          </button>
          <button class="ghost-btn" @click="emit('close')">
            {{ deploying ? 'Cancel' : 'Close' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import CommandBlock from '../CommandBlock.vue'
import { data } from '../../../wailsjs/go/models'
import { DeployAnomstack } from '../../../wailsjs/go/main/App'

interface Props {
  profile: string
  title: string
  plan?: data.AnomalyDeploymentPlan
}

const props = defineProps<Props>()
const emit = defineEmits<{
  close: []
  'deployment-started': [profile: string]
  'deployment-completed': [profile: string, result: string]
}>()

const deploying = ref(false)
const deployResult = ref('')
const deployError = ref('')
const showCommands = ref(false)

async function startDeployment() {
  deploying.value = true
  deployResult.value = ''
  deployError.value = ''
  emit('deployment-started', props.profile)
  try {
    const result = await DeployAnomstack(props.profile)
    deployResult.value = result
    emit('deployment-completed', props.profile, result)
  } catch (err) {
    deployError.value = err instanceof Error ? err.message : String(err)
  } finally {
    deploying.value = false
  }
}
</script>

<style scoped>
.deployment-modal-overlay {
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
}

.deployment-modal {
  background: var(--panel-bg);
  border-radius: 24px;
  width: 90%;
  max-width: 700px;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border: 1px solid var(--border);
}

.deployment-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 20px 24px;
  border-bottom: 1px solid var(--border);
}

.deployment-modal-header h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.deployment-modal-close-btn {
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

.deployment-modal-close-btn:hover {
  background: var(--hover);
}

.deployment-modal-content {
  padding: 24px;
  overflow: auto;
  flex: 1;
}

.deploying-status {
  text-align: center;
  padding: 40px 0;
}

.spinner {
  border: 3px solid var(--border);
  border-top: 3px solid var(--primary);
  border-radius: 50%;
  width: 40px;
  height: 40px;
  animation: spin 1s linear infinite;
  margin: 0 auto 20px;
}

@keyframes spin {
  0% { transform: rotate(0deg); }
  100% { transform: rotate(360deg); }
}

.deploy-error {
  background: #ef444420;
  border: 1px solid #ef4444;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 20px;
}

.deploy-error h4 {
  margin: 0 0 8px 0;
  color: #ef4444;
}

.deploy-error pre {
  white-space: pre-wrap;
  font-family: 'Monaco', monospace;
  font-size: 12px;
  margin: 0;
}

.deploy-success {
  background: #10b98120;
  border: 1px solid #10b981;
  border-radius: 12px;
  padding: 16px;
  margin-bottom: 20px;
}

.deploy-success h4 {
  margin: 0 0 8px 0;
  color: #10b981;
}

.deploy-success pre {
  white-space: pre-wrap;
  font-family: 'Monaco', monospace;
  font-size: 12px;
  margin: 0;
}

.deploy-preview h4 {
  margin: 0 0 12px 0;
  font-size: 18px;
}

.plan-details {
  margin-top: 20px;
}

.detail-section {
  margin-bottom: 20px;
}

.detail-section h5 {
  font-size: 14px;
  margin: 0 0 8px 0;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.deployment-modal-footer {
  display: flex;
  gap: 12px;
  justify-content: flex-end;
  padding-top: 20px;
  border-top: 1px solid var(--border);
  margin-top: 20px;
}

 .summary-section {
   background: rgba(31, 41, 55, 0.3);
   border-radius: 12px;
   padding: 16px;
   margin-bottom: 20px;
   border: 1px solid var(--border);
 }

 .summary-item {
   display: flex;
   justify-content: space-between;
   margin-bottom: 8px;
   font-size: 14px;
 }

 .summary-item:last-child {
   margin-bottom: 0;
 }

 .summary-label {
   color: var(--text-secondary);
   font-weight: 500;
 }

 .summary-value {
   color: var(--text);
   font-weight: 600;
 }

 .view-commands-btn {
   margin-top: 16px;
   padding: 8px 16px;
   border-radius: 6px;
   border: 1px solid var(--border);
   background: transparent;
   color: var(--text-secondary);
   cursor: pointer;
   font-size: 13px;
   font-weight: 500;
   transition: all 0.2s ease;
 }

 .view-commands-btn:hover {
   background: var(--hover);
   color: var(--text);
 }

 .primary-btn {
   padding: 10px 20px;
   border-radius: 8px;
   background: var(--primary);
   color: white;
   border: none;
   font-weight: 500;
   cursor: pointer;
 }

.ghost-btn {
  padding: 10px 20px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
}
</style>