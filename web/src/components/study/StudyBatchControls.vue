<script setup lang="ts">
defineProps<{
  selectedCount: number;
  parallelism: number;
  forceRerun: boolean;
}>();

const emit = defineEmits<{
  "update:parallelism": [value: number];
  "update:forceRerun": [value: boolean];
  runSelected: [];
  clear: [];
}>();
</script>

<template>
  <section class="study-batch-controls" aria-label="Batch analysis controls">
    <div class="study-batch-row">
      <button type="button" @click="emit('clear')">New Import / Run</button>
      <button type="button" :disabled="!selectedCount" @click="emit('runSelected')">Run selected</button>
    </div>
    <div class="study-batch-row">
      <label class="study-parallel-control">
        <span>Parallel</span>
        <input
          :value="parallelism"
          type="number"
          min="1"
          max="5"
          @input="emit('update:parallelism', Number(($event.target as HTMLInputElement).value || 1))"
        />
      </label>
      <label class="study-rerun-control">
        <input
          type="checkbox"
          :checked="forceRerun"
          @change="emit('update:forceRerun', ($event.target as HTMLInputElement).checked)"
        />
        <span>Rerun saved analysis</span>
      </label>
    </div>
  </section>
</template>
