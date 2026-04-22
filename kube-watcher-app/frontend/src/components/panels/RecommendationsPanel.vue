<template>
  <section class="recommendations-panel">
    <article class="panel">
      <div class="panel-head">
        <div>
          <p class="meta-label">Recommendations</p>
          <h3>AI-assisted remediation</h3>
        </div>
      </div>
      <ul class="stack-list">
        <RecommendationCard
          v-for="recommendation in recommendations.slice(0, 10)"
          :key="recommendation.id"
          :recommendation="recommendation"
        />
      </ul>
    </article>
  </section>
</template>

<script setup lang="ts">
import { data } from '../../../wailsjs/go/models'
import RecommendationCard from '../cards/RecommendationCard.vue'

interface Props {
  recommendations: data.Recommendation[]
}

defineProps<Props>()
</script>

<style scoped>
.recommendations-panel {
  width: 100%;
}

.panel {
  background: var(--panel-bg);
  border-radius: var(--radius-lg);
  border: 1px solid var(--border);
  padding: 24px;
  box-shadow: var(--shadow);
  width: 100%;
}

.panel-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24px;
}

.panel-head h3 {
  margin: 4px 0 0 0;
  font-size: 20px;
  font-weight: 600;
}

.meta-label {
  color: var(--text-muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin: 0;
}

.stack-list {
  list-style: none;
  padding: 0;
  margin: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}
</style>