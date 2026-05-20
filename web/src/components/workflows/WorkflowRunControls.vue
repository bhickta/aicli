<script setup lang="ts">
defineProps<{
  running: boolean;
  canCancel: boolean;
  cancelLabel: string;
}>();

const emit = defineEmits<{
  run: [];
  cancel: [];
}>();
</script>

<template>
  <div class="field">
    <button id="workflow-run" type="button" :disabled="running" @click="emit('run')">Run workflow</button>
    <button v-if="running && canCancel" type="button" @click="emit('cancel')">{{ cancelLabel }}</button>
  </div>
</template>
