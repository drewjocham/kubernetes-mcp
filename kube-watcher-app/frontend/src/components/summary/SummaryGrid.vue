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
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 16px;
  margin-bottom: 32px;
}

.summary-card {
  padding: 20px;
  border-radius: 16px;
  background: var(--panel-bg);
  border: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.summary-card span {
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-secondary);
}

.summary-card strong {
  font-size: 24px;
  font-weight: 700;
  color: var(--text-primary);
}

.summary-card.rose { border-left: 4px solid var(--accent-rose); }
.summary-card.amber { border-left: 4px solid var(--accent-amber); }
.summary-card.teal { border-left: 4px solid var(--accent-teal); }
.summary-card.ink { border-left: 4px solid var(--accent-ink); }
</style>