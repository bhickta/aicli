<script setup lang="ts">
defineProps<{
  label: string;
  value: string;
  description?: string;
  disabled?: boolean;
}>();

const emit = defineEmits<{
  choose: [];
}>();
</script>

<template>
  <article class="zettel-folder-chooser">
    <div class="zettel-folder-copy">
      <span>{{ label }}</span>
      <strong>{{ value || "Not selected" }}</strong>
      <small v-if="description">{{ description }}</small>
    </div>
    <button type="button" :disabled="disabled" @click="emit('choose')">Choose</button>
  </article>
</template>

<style scoped>
.zettel-folder-chooser {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  min-width: 0;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.zettel-folder-copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.zettel-folder-copy span,
.zettel-folder-copy small {
  color: #9aa4b2;
  font-size: 12px;
}

.zettel-folder-copy strong {
  overflow-wrap: anywhere;
}

.zettel-folder-chooser button {
  white-space: nowrap;
}

@media (max-width: 640px) {
  .zettel-folder-chooser {
    grid-template-columns: 1fr;
  }
}
</style>
