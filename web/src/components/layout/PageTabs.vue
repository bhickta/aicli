<script setup lang="ts">
import { RouterLink, type RouteLocationRaw } from "vue-router";

defineProps<{
  tabs: Array<{
    label: string;
    to: RouteLocationRaw;
    active?: boolean;
  }>;
  label: string;
}>();
</script>

<template>
  <nav class="page-tabs" :aria-label="label">
    <RouterLink
      v-for="tab in tabs"
      :key="tab.label"
      :to="tab.to"
      class="page-tab"
      :class="{ active: tab.active }"
    >
      {{ tab.label }}
    </RouterLink>
  </nav>
</template>

<style scoped>
.page-tabs {
  background: #0a0f18;
  border: 1px solid #2b3440;
  border-radius: 6px;
  display: flex;
  gap: 2px;
  min-width: 0;
  overflow-x: auto;
  padding: 3px;
}

.page-tab {
  border-radius: 4px;
  color: #dbeafe;
  min-height: 2rem;
  min-width: 7.5rem;
  padding: 5px 10px;
  text-align: center;
  text-decoration: none;
  white-space: nowrap;
}

.page-tab:hover,
.page-tab.active,
.page-tab.router-link-active {
  background: #17304f;
  color: #ffffff;
}
</style>
