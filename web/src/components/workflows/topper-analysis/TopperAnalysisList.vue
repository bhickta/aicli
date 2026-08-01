<script setup lang="ts">
defineProps<{
  title: string;
  values?: string[];
  tone?: "neutral" | "warning" | "positive";
}>();
</script>

<template>
  <section v-if="values?.length" class="analysis-list" :class="tone || 'neutral'">
    <h6>{{ title }}</h6>
    <ul>
      <li v-for="value in values" :key="value">{{ value }}</li>
    </ul>
  </section>
</template>

<style scoped>
.analysis-list {
  background: #0d1726;
  border: 1px solid #2b405d;
  border-radius: 0.4rem;
  padding: 0.6rem;
}

.analysis-list.warning {
  background: #20140f;
  border-color: #663a29;
}

.analysis-list.positive {
  background: #0b1b18;
  border-color: #245847;
}

.analysis-list h6,
.analysis-list ul {
  margin: 0;
}

.analysis-list h6 {
  color: #e2e8f0;
  font-size: 0.76rem;
  margin-bottom: 0.35rem;
}

.analysis-list ul {
  color: #b9c7d8;
  display: grid;
  font-size: 0.73rem;
  gap: 0.25rem;
  line-height: 1.4;
  padding-left: 1rem;
}
</style>
