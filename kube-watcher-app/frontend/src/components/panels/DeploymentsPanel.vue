<template>
  <section class="deployments-panel">
    <article :class="['panel', 'span-wide', { expanded: isExpanded }]">
      <div class="panel-head">
        <div>
          <p class="meta-label">Deployment tracks</p>
          <h3>Local compose, minikube, and cluster-ready anomaly paths</h3>
        </div>
        <div class="head-actions">
          <button
            class="ghost-btn expand-toggle"
            @click="emit('toggle-expand')"
          >
            {{ isExpanded ? '−' : '+' }}
          </button>
        </div>
      </div>
      <div v-show="isExpanded" class="deploy-grid">
        <DeploymentCard
          v-for="plan in deploymentPlans"
          :key="plan.profile"
          :plan="plan"
        />
      </div>
    </article>
  </section>
</template>

<script setup lang="ts">
import DeploymentCard from '../cards/DeploymentCard.vue'
import { data } from '../../../wailsjs/go/models'

interface Props {
  isExpanded: boolean
  deploymentPlans?: data.AnomalyDeploymentPlan[]
}

const props = defineProps<Props>()
console.log('DeploymentsPanel props:', props)
import { onMounted } from 'vue'
onMounted(() => console.log('DeploymentsPanel mounted'))
const emit = defineEmits<{
  'toggle-expand': []
}>()
</script>

<style scoped>
.deployments-panel {
  width: 100%;
}

.panel {
  padding: 22px;
  border-radius: 24px;
  border: 1px solid var(--border);
  background: var(--panel-bg);
}

.panel.span-wide {
  width: 100%;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 20px;
}

.head-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.deploy-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(300px, 1fr));
  gap: 20px;
}

.meta-label {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin: 0 0 4px 0;
}

h3 {
  font-size: 20px;
  font-weight: 600;
  margin: 0;
  line-height: 1.3;
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

@media (max-width: 768px) {
  .deploy-grid {
    grid-template-columns: 1fr;
  }
  
  .panel-head {
    flex-direction: column;
    gap: 12px;
  }
  
  .head-actions {
    align-self: flex-end;
  }
}
</style>