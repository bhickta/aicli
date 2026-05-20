<script setup lang="ts">
defineProps<{
  categories: string[];
  activeCategory: string;
}>();

const emit = defineEmits<{
  select: [category: string];
}>();
</script>

<template>
  <div class="tabs workflow-category-tabs" role="tablist" aria-label="Workflow categories">
    <button
      v-for="category in categories"
      :key="category"
      type="button"
      role="tab"
      class="workflow-tab"
      :class="{ 'active-tab': activeCategory === category }"
      :aria-selected="activeCategory === category"
      @click="emit('select', category)"
    >
      {{ category }}
    </button>
  </div>
</template>
