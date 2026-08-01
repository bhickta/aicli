<script setup lang="ts">
import type { AnalysisImprovement } from "../../../types";

defineProps<{
  improvements?: AnalysisImprovement[];
}>();
</script>

<template>
  <section v-if="improvements?.length" class="improvement-plan">
    <h6>Highest-leverage improvements</h6>
    <ol>
      <li v-for="improvement in improvements" :key="`${improvement.priority}:${improvement.change}`">
        <span :class="`priority ${improvement.priority || 'medium'}`">{{ improvement.priority || "medium" }}</span>
        <div>
          <strong>{{ improvement.change }}</strong>
          <p v-if="improvement.example">Example: {{ improvement.example }}</p>
        </div>
      </li>
    </ol>
  </section>
</template>

<style scoped>
.improvement-plan {
  background: #17160d;
  border: 1px solid #5a5425;
  border-radius: 0.4rem;
  display: grid;
  gap: 0.5rem;
  padding: 0.65rem;
}

.improvement-plan h6,
.improvement-plan ol,
.improvement-plan p {
  margin: 0;
}

.improvement-plan h6 {
  color: #fef3c7;
  font-size: 0.78rem;
}

.improvement-plan ol {
  display: grid;
  gap: 0.45rem;
  list-style: none;
  padding: 0;
}

.improvement-plan li {
  align-items: start;
  display: grid;
  gap: 0.45rem;
  grid-template-columns: auto 1fr;
}

.improvement-plan li div {
  display: grid;
  gap: 0.2rem;
}

.improvement-plan strong {
  color: #e5e7eb;
  font-size: 0.78rem;
}

.improvement-plan p {
  color: #b6c2d1;
  font-size: 0.72rem;
  line-height: 1.4;
}

.priority {
  border: 1px solid #64748b;
  border-radius: 999px;
  color: #cbd5e1;
  font-size: 0.62rem;
  padding: 0.12rem 0.34rem;
  text-transform: uppercase;
}

.priority.high {
  border-color: #ef4444;
  color: #fecaca;
}

.priority.low {
  border-color: #22c55e;
  color: #bbf7d0;
}
</style>
