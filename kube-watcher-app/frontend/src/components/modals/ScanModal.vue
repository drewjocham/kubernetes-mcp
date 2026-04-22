<template>
  <div v-if="cell" class="scan-modal-overlay" @click="emit('close')">
    <div class="scan-modal" @click.stop v-if="cell">
      <div class="scan-modal-header">
        <h3>{{ cell.title }}</h3>
        <button class="scan-modal-close-btn" @click="emit('close')">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
            <path d="M18 6L6 18M6 6L18 18" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
          </svg>
        </button>
      </div>

      <div class="scan-modal-content">
        <div class="scan-modal-summary">
          <span :class="['pill', cell.health]">{{ cell.health }}</span>
          <p>{{ cell.summary }}</p>
          <div v-if="cell.service" class="scan-service-info">
            <strong>Service:</strong> {{ cell.service }}
          </div>
        </div>

        <div class="scan-modal-issues" v-if="cell && hasIssues(cell.detail)">
          <div v-if="getErrors(cell.detail).length > 0" class="scan-issue-section">
            <h4 class="scan-issue-header error-header">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 8V12M12 16H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              Errors
            </h4>
            <ul class="scan-issue-list">
              <li v-for="(error, index) in getErrors(cell.detail)" :key="error.id + '-' + index">
                <div class="issue-message">
                  <strong>[POP-{{ error.id }}]</strong> {{ error.message }}
                </div>
                <div v-if="error.context" class="issue-context">
                  {{ error.context }}
                </div>
              </li>
            </ul>
          </div>

          <div v-if="getWarnings(cell.detail).length > 0" class="scan-issue-section">
            <h4 class="scan-issue-header warning-header">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 9V11M12 15H12.01M5.07183 19H18.9282C20.4678 19 21.4301 17.3333 20.6603 16L13.7321 4C12.9623 2.66667 11.0377 2.66667 10.2679 4L3.33975 16C2.56995 17.3333 3.53223 19 5.07183 19Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              Warnings
            </h4>
            <ul class="scan-issue-list">
              <li v-for="(warning, index) in getWarnings(cell.detail)" :key="warning.id + '-' + index">
                <div class="issue-message">
                  <strong>[POP-{{ warning.id }}]</strong> {{ warning.message }}
                </div>
                <div v-if="warning.context" class="issue-context">
                  {{ warning.context }}
                </div>
              </li>
            </ul>
          </div>

          <div v-if="getInfo(cell.detail).length > 0" class="scan-issue-section">
            <h4 class="scan-issue-header info-header">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M12 16V12M12 8H12.01M22 12C22 17.5228 17.5228 22 12 22C6.47715 22 2 17.5228 2 12C2 6.47715 6.47715 2 12 2C17.5228 2 22 6.47715 22 12Z" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              Information
            </h4>
            <ul class="scan-issue-list">
              <li v-for="(info, index) in getInfo(cell.detail)" :key="info.id + '-' + index">
                <div class="issue-message">
                  <strong>[POP-{{ info.id }}]</strong> {{ info.message }}
                </div>
                <div v-if="info.context" class="issue-context">
                  {{ info.context }}
                </div>
              </li>
            </ul>
          </div>
        </div>
        
        <div v-if="cell && !hasIssues(cell.detail)" class="scan-modal-details">
          <h4 class="scan-details-header">Scan Details</h4>
          <pre class="scan-modal-details-text">{{ cell.detail }}</pre>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ScanCell } from '../panels/types'

interface Props {
  cell: ScanCell | null
}

defineProps<Props>()
const emit = defineEmits<{
  close: []
}>()

interface Issue {
  id: string
  message: string
  context: string
  level: number // 3: error, 2: warning, 1: info
}

function parseIssues(detail: string, level: number): Issue[] {
  const issues: Issue[] = []
  const lines = detail.split('\n')
  
  // Parse popeye issues - look for lines with [POP-XXXX] pattern
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim()
    if (!line || line.length < 5) continue
    
    // Match popeye issue pattern: [POP-XXXX] message
    const popMatch = line.match(/\[POP-(\d+)\]\s*(.+)/)
    if (popMatch) {
      const issueId = popMatch[1]
      let message = popMatch[2].trim()
      
      // Clean up the message
      message = message.replace(/^\[.*?\]\s*/, '')
      
      // Determine level from message content
      let issueLevel = 2 // default warning
      const lowerMsg = message.toLowerCase()
      
      if (lowerMsg.includes('error') || lowerMsg.includes('critical') || 
          lowerMsg.includes('failed') || lowerMsg.includes('unhappy') || 
          lowerMsg.includes('crash') || lowerMsg.includes('not found')) {
        issueLevel = 3 // error
      } else if (lowerMsg.includes('warning') || lowerMsg.includes('deprecated') || 
                 lowerMsg.includes('no resources') || lowerMsg.includes('not secured') ||
                 lowerMsg.includes('no limits') || lowerMsg.includes('no probes')) {
        issueLevel = 2 // warning
      } else if (lowerMsg.includes('info') || lowerMsg.includes('used?') || 
                 lowerMsg.includes('unable to locate') || lowerMsg.includes('consider')) {
        issueLevel = 1 // info
      }
      
      if (issueLevel === level) {
        // Try to get context from next line if it contains service/pod info
        let context = ''
        if (i + 1 < lines.length) {
          const nextLine = lines[i + 1].trim()
          if (nextLine && nextLine.length > 5 && !nextLine.match(/\[POP-\d+\]/)) {
            context = nextLine
          }
        }
        
        issues.push({
          id: issueId,
          message: message,
          context: context || 'Kubernetes resource configuration issue',
          level: issueLevel
        })
      }
    }
  }
  
  // If no structured issues found, try to extract from the raw detail
  if (issues.length === 0 && (detail.includes('errors') || detail.includes('warnings'))) {
    const errorMatches = detail.match(/(\d+)\s+errors?/gi)
    const warningMatches = detail.match(/(\d+)\s+warnings?/gi)
    
    if ((level === 3 && errorMatches) || (level === 2 && warningMatches)) {
      issues.push({
        id: 'general',
        message: level === 3 ? 'Multiple configuration errors detected' : 'Multiple configuration warnings detected',
        context: 'Check resource limits, network policies, and pod configurations',
        level: level
      })
    }
  }

  return issues
}

function getErrors(detail: string): Issue[] {
  return parseIssues(detail, 3)
}

function getWarnings(detail: string): Issue[] {
  return parseIssues(detail, 2)
}

function getInfo(detail: string): Issue[] {
  return parseIssues(detail, 1)
}

function hasIssues(detail: string): boolean {
  return getErrors(detail).length > 0 || getWarnings(detail).length > 0 || getInfo(detail).length > 0
}
</script>

<style scoped>
.scan-modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
   background: var(--modal-overlay-bg);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  padding: 20px;
  backdrop-filter: blur(2px);
}

.scan-modal {
  background: var(--modal-bg);
  border-radius: 24px;
  width: 100%;
  max-width: 800px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
  border: 1px solid var(--border);
}

.scan-modal-header {
  padding: 24px;
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.scan-modal-header h3 {
  font-size: 24px;
  font-weight: 600;
  margin: 0;
}

.scan-modal-close-btn {
  padding: 8px;
  border-radius: 8px;
  border: 1px solid var(--border);
  background: transparent;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s;
  display: flex;
  align-items: center;
  justify-content: center;
}

.scan-modal-close-btn:hover {
  background: var(--hover);
  border-color: var(--border-active);
  color: var(--text-primary);
}

.scan-modal-content {
  padding: 24px;
  overflow-y: auto;
  flex: 1;
}

.scan-modal-summary {
  margin-bottom: 32px;
}

.pill {
  display: inline-block;
  padding: 6px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 16px;
}

.pill.good {
  background: rgba(34, 197, 94, 0.1);
  color: var(--success);
  border: 1px solid rgba(34, 197, 94, 0.2);
}

.pill.warning {
  background: rgba(245, 158, 11, 0.1);
  color: var(--warning);
  border: 1px solid rgba(245, 158, 11, 0.2);
}

.pill.unhealthy {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
  border: 1px solid rgba(239, 68, 68, 0.2);
}

.scan-modal-summary p {
  font-size: 16px;
  line-height: 1.6;
  color: var(--text-secondary);
  margin: 16px 0;
}

.scan-service-info {
  font-size: 14px;
  color: var(--text-tertiary);
}

.scan-service-info strong {
  color: var(--text-secondary);
  font-weight: 500;
}

.scan-modal-issues {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.scan-issue-section {
  border-radius: 16px;
  border: 1px solid var(--border);
  overflow: hidden;
}

.scan-issue-header {
  padding: 16px;
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  display: flex;
  align-items: center;
  gap: 8px;
}

.error-header {
  background: rgba(239, 68, 68, 0.1);
  color: var(--error);
  border-bottom: 1px solid rgba(239, 68, 68, 0.2);
}

.warning-header {
  background: rgba(245, 158, 11, 0.1);
  color: var(--warning);
  border-bottom: 1px solid rgba(245, 158, 11, 0.2);
}

.info-header {
  background: rgba(59, 130, 246, 0.1);
  color: var(--info);
  border-bottom: 1px solid rgba(59, 130, 246, 0.2);
}

.scan-issue-list {
  list-style: none;
  padding: 16px;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.scan-issue-list li {
  padding: 12px;
  border-radius: 8px;
  background: var(--hover);
  font-size: 14px;
  line-height: 1.5;
  color: var(--text-secondary);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.scan-issue-list li:before {
  content: none;
}



.issue-message {
  line-height: 1.4;
}

.issue-message strong {
  color: var(--text-primary);
  font-weight: 600;
}

.issue-context {
  font-size: 13px;
  color: var(--text-tertiary);
  font-style: italic;
  margin-top: 2px;
  padding-left: 8px;
  border-left: 2px solid var(--border);
}

.scan-modal-details {
  margin-top: 24px;
  padding: 20px;
  border-radius: 16px;
  border: 1px solid var(--border);
  background: var(--hover);
}

.scan-details-header {
  font-size: 18px;
  font-weight: 600;
  margin: 0 0 16px 0;
  color: var(--text-secondary);
}

.scan-modal-details-text {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary);
  white-space: pre-wrap;
  word-break: break-all;
  background: rgba(0, 0, 0, 0.2);
  padding: 16px;
  border-radius: 8px;
  overflow-x: auto;
}
</style>