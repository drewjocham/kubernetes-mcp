<template>
  <header class="header">
    <div class="header-content">
      <div class="header-left">
        <h1 class="header-title">{{ title }}</h1>
        <p class="header-subtitle" v-if="subtitle">{{ subtitle }}</p>
      </div>
      
      <div class="header-actions">
        <slot name="actions">
          <!-- Default actions slot -->
        </slot>
      </div>
    </div>
    
    <div class="header-meta" v-if="hasMeta">
      <slot name="meta">
        <!-- Meta information slot -->
      </slot>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  title: string
  subtitle?: string
}

const props = defineProps<Props>()
const slots = defineSlots()

const hasMeta = computed(() => !!slots.meta)
</script>

<style scoped>
.header {
  background: var(--header-bg);
  border-bottom: 1px solid var(--border);
  padding: 20px 24px;
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 16px;
}

.header-left {
  flex: 1;
}

.header-title {
  font-size: 28px;
  font-weight: 600;
  margin: 0 0 4px 0;
  line-height: 1.2;
}

.header-subtitle {
  color: var(--text-secondary);
  font-size: 16px;
  margin: 0;
  line-height: 1.4;
}

.header-actions {
  display: flex;
  gap: 12px;
  align-items: center;
}

.header-meta {
  display: flex;
  gap: 24px;
  flex-wrap: wrap;
}

@media (max-width: 768px) {
  .header-content {
    flex-direction: column;
    gap: 16px;
  }
  
  .header-actions {
    width: 100%;
    justify-content: flex-start;
  }
}
</style>