<template>
  <article class="deploy-card">
    <div class="deploy-head">
      <div>
        <span class="deploy-mode">{{ plan.mode }}</span>
        <h4>{{ plan.title }}</h4>
      </div>
      <strong class="deploy-namespace">{{ plan.namespace }}</strong>
    </div>
    <p class="deploy-summary">{{ plan.summary }}</p>

    <div class="detail-group" v-if="plan.commands?.length">
      <label>Commands</label>
      <CommandBlock
        v-for="command in plan.commands"
        :key="command"
        :command="command"
      />
    </div>

    <div class="detail-group" v-if="plan.validation?.length">
      <label>Validation</label>
      <CommandBlock
        v-for="check in plan.validation"
        :key="check"
        :command="check"
      />
    </div>

    <div class="detail-group" v-if="plan.artifacts?.length">
      <label>Artifacts</label>
      <CommandBlock
        v-for="artifact in plan.artifacts"
        :key="artifact"
        :command="artifact"
      />
    </div>
  </article>
</template>

<script setup lang="ts">
import CommandBlock from '../CommandBlock.vue'
import { data } from '../../../wailsjs/go/models'

interface Props {
  plan: data.AnomalyDeploymentPlan
}

defineProps<Props>()
</script>

<style scoped>
.deploy-card {
  padding: 20px;
  border-radius: 20px;
  border: 1px solid var(--border);
  background: var(--card-bg);
  transition: all 0.2s;
}

.deploy-card:hover {
  border-color: var(--border-active);
  background: var(--hover);
  transform: translateY(-2px);
}

.deploy-head {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
  gap: 12px;
}

.deploy-mode {
  display: block;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin-bottom: 4px;
}

.deploy-head h4 {
  font-size: 18px;
  font-weight: 600;
  margin: 0;
  color: var(--text-primary);
  line-height: 1.3;
}

.deploy-namespace {
  font-size: 14px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.deploy-summary {
  margin: 0 0 20px 0;
  font-size: 14px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.detail-group {
  margin-bottom: 16px;
}

.detail-group:last-child {
  margin-bottom: 0;
}

.detail-group label {
  display: block;
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
  margin-bottom: 8px;
  font-weight: 500;
}
</style>