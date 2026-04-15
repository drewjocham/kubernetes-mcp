<template>
  <section class="summary-grid">
    <article v-for="card in cards" :key="card.label" :class="['summary-card', card.accent]">
      <span>{{ card.label }}</span>
      <strong>{{ card.value }}</strong>
    </article>
  </section>
</template>

<script setup lang="ts">
interface SummaryCard {
  label: string
  value: number | string
  accent: string
}

interface Props {
  cards: SummaryCard[]
}

defineProps<Props>()
</script>

<style scoped>
.summary-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  margin: 0 auto 32px auto;
  max-width: 800px;
}

.summary-card {
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: center;
  text-align: center;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--border);
  border-radius: 8px;
}

.summary-card span {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
}

.summary-card strong {
  font-size: 1.5rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}
</style>