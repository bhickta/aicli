<script setup lang="ts">
import { useTaskSound } from "../../composables/useTaskSound";

const { available, enabled, setEnabled, testSound } = useTaskSound();
</script>

<template>
  <div v-if="available" class="workflow-sound-control" aria-label="Workflow sound alerts">
    <label>
      <input type="checkbox" :checked="enabled" @change="setEnabled(($event.target as HTMLInputElement).checked)" />
      Sound on finish
    </label>
    <button type="button" :disabled="!enabled" @click="testSound">Test</button>
  </div>
</template>

<style scoped>
.workflow-sound-control {
  align-items: center;
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
}

.workflow-sound-control label {
  align-items: center;
  color: #cbd5e1;
  display: inline-flex;
  gap: 0.45rem;
  font-size: 0.86rem;
}

.workflow-sound-control button {
  background: #111827;
  border: 1px solid #334155;
  border-radius: 0.42rem;
  color: #dbeafe;
  cursor: pointer;
  font: inherit;
  padding: 0.3rem 0.55rem;
}

.workflow-sound-control button:disabled {
  cursor: not-allowed;
  opacity: 0.5;
}
</style>
