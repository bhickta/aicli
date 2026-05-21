<script setup lang="ts">
defineProps<{
  title: string;
  description?: string;
}>();
</script>

<template>
  <section class="zettel-section">
    <header class="zettel-section-header">
      <div>
        <h3>{{ title }}</h3>
        <p v-if="description" class="muted">{{ description }}</p>
      </div>
      <div v-if="$slots.actions" class="zettel-section-actions">
        <slot name="actions" />
      </div>
    </header>
    <slot />
  </section>
</template>

<style scoped>
.zettel-section {
  display: grid;
  gap: 12px;
  padding: 14px;
  overflow: visible;
  border: 1px solid #242b35;
  border-radius: 8px;
  background: #141922;
}

.zettel-section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.zettel-section-header p {
  margin: 6px 0 0;
}

.zettel-section-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.zettel-section-actions :deep(button) {
  text-align: center;
}

@media (max-width: 760px) {
  .zettel-section-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
