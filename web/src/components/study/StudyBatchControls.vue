<script setup lang="ts">
defineProps<{
  selectedCount: number;
  parallelism: number;
  forceRerun: boolean;
  running: boolean;
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
      <button type="button" :disabled="running" @click="emit('clear')">New Import / Run</button>
      <button type="button" :disabled="running || !selectedCount" @click="emit('runSelected')">
        {{ running ? "Running..." : "Run selected" }}
      </button>
    </div>
    <div class="study-batch-row">
      <label class="study-parallel-control">
        <span>Parallel</span>
        <input
          :value="parallelism"
          :disabled="running"
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
          :disabled="running"
          @change="emit('update:forceRerun', ($event.target as HTMLInputElement).checked)"
        />
        <span>Bypass cache</span>
      </label>
    </div>
  </section>
</template>
