<script setup lang="ts">
defineProps<{
  items: string[];
}>();
</script>

<template>
  <div class="zettel-badges">
    <span v-for="item in items" :key="item">{{ item }}</span>
  </div>
</template>

<style scoped>
.zettel-badges {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.zettel-badges span {
  padding: 5px 7px;
  border: 1px solid #343b46;
  border-radius: 999px;
  background: #202630;
  color: #9aa4b2;
  font-size: 12px;
}
</style>
