<script setup lang="ts">
defineProps<{
  kind?: "info" | "success" | "error";
}>();
</script>

<template>
  <p class="inline-alert" :class="kind || 'info'">
    <slot />
  </p>
</template>

<style scoped>
.inline-alert {
  border: 1px solid #334155;
  border-left-width: 3px;
  border-radius: 6px;
  margin: 0;
  padding: 0.6rem 0.7rem;
}

.inline-alert.info {
  background: #0f1724;
  border-left-color: #60a5fa;
  color: #dbeafe;
}

.inline-alert.success {
  background: #0f1f17;
  border-left-color: #4ade80;
  color: #dcfce7;
}

.inline-alert.error {
  background: #2a1115;
  border-left-color: #f87171;
  color: #fee2e2;
}
</style>
