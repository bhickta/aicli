<script setup lang="ts">
import type { AnalysisPoint } from "../../../types";

defineProps<{
  title: string;
  tone: "strength" | "gap";
  points?: AnalysisPoint[];
}>();
</script>

<template>
  <section v-if="points?.length" class="analysis-points" :class="tone">
    <h6>{{ title }}</h6>
    <article v-for="point in points" :key="`${point.point}:${point.evidence || ''}`">
      <strong>{{ point.point }}</strong>
      <blockquote v-if="point.evidence">Evidence: {{ point.evidence }}</blockquote>
      <p v-if="point.why_it_matters">Why it matters: {{ point.why_it_matters }}</p>
    </article>
  </section>
</template>

<style scoped>
.analysis-points {
  border: 1px solid #334155;
  border-radius: 0.4rem;
  display: grid;
  gap: 0.45rem;
  padding: 0.65rem;
}

.analysis-points.strength {
  background: #0b1c19;
  border-color: #1f5c4b;
}

.analysis-points.gap {
  background: #21130f;
  border-color: #6b382b;
}

.analysis-points h6,
.analysis-points p,
.analysis-points blockquote {
  margin: 0;
}

.analysis-points h6 {
  color: #e5e7eb;
  font-size: 0.78rem;
}

.analysis-points article {
  background: rgb(2 6 23 / 35%);
  border-radius: 0.3rem;
  display: grid;
  gap: 0.25rem;
  padding: 0.5rem;
}

.analysis-points strong {
  color: #e2e8f0;
  font-size: 0.78rem;
}

.analysis-points blockquote,
.analysis-points p {
  color: #aebdd0;
  font-size: 0.73rem;
  line-height: 1.4;
}

.analysis-points blockquote {
  border-left: 2px solid #64748b;
  padding-left: 0.45rem;
}
</style>
