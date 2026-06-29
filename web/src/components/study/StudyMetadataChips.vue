<script setup lang="ts">
defineProps<{
  chips: string[];
}>();
</script>

<template>
  <div v-if="chips.length" class="study-metadata-chips">
    <span v-for="chip in chips" :key="chip" class="study-metadata-chip">{{ chip }}</span>
  </div>
</template>
