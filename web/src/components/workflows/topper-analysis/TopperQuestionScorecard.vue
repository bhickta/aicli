<script setup lang="ts">
import { computed } from "vue";
import type { QuestionScorecard } from "../../../types";

const props = defineProps<{
  scorecard?: QuestionScorecard;
}>();

const metrics = computed(() => {
  const scorecard = props.scorecard;
  if (!scorecard) return [];
  return [
    { label: "Demand", score: normalizedScore(scorecard.demand_fulfilment) },
    { label: "Structure", score: normalizedScore(scorecard.structure) },
    { label: "Depth", score: normalizedScore(scorecard.content_depth) },
    { label: "Evidence", score: normalizedScore(scorecard.evidence) },
    { label: "Dimensions", score: normalizedScore(scorecard.multidimensionality) },
    { label: "Presentation", score: normalizedScore(scorecard.presentation) },
    { label: "Conclusion", score: normalizedScore(scorecard.conclusion) },
  ];
});

const overallPercent = computed(() => Math.min(100, Math.max(0, Number(props.scorecard?.overall_percent) || 0)));

function normalizedScore(value: number) {
  return Math.min(5, Math.max(0, Number(value) || 0));
}
</script>

<template>
  <section v-if="scorecard" class="scorecard">
    <header class="scorecard-header">
      <div>
        <h6>Analytical scorecard</h6>
        <p>Learning rubric, not predicted UPSC marks</p>
      </div>
      <div class="scorecard-summary">
        <strong>{{ overallPercent }}%</strong>
        <span v-if="scorecard.estimated_band">{{ scorecard.estimated_band }}</span>
        <span v-if="scorecard.confidence">{{ scorecard.confidence }} confidence</span>
      </div>
    </header>

    <div class="scorecard-grid">
      <div v-for="metric in metrics" :key="metric.label" class="scorecard-metric">
        <span>{{ metric.label }}</span>
        <strong>{{ metric.score }}/5</strong>
        <progress :value="metric.score" max="5">{{ metric.score }}/5</progress>
      </div>
    </div>

    <p v-if="scorecard.rationale" class="scorecard-rationale">{{ scorecard.rationale }}</p>
  </section>
</template>

<style scoped>
.scorecard {
  background: #0a1322;
  border: 1px solid #264568;
  border-radius: 0.4rem;
  display: grid;
  gap: 0.65rem;
  padding: 0.7rem;
}

.scorecard-header,
.scorecard-summary,
.scorecard-metric {
  align-items: center;
  display: flex;
}

.scorecard-header {
  gap: 0.65rem;
  justify-content: space-between;
}

.scorecard-header h6,
.scorecard-header p,
.scorecard-rationale {
  margin: 0;
}

.scorecard-header h6 {
  color: #e5e7eb;
  font-size: 0.82rem;
}

.scorecard-header p {
  color: #94a3b8;
  font-size: 0.7rem;
  margin-top: 0.12rem;
}

.scorecard-summary {
  flex-wrap: wrap;
  gap: 0.35rem;
  justify-content: flex-end;
}

.scorecard-summary strong {
  color: #7dd3fc;
  font-size: 1rem;
}

.scorecard-summary span {
  background: #17233a;
  border: 1px solid #355175;
  border-radius: 999px;
  color: #cbd5e1;
  font-size: 0.68rem;
  padding: 0.15rem 0.4rem;
  text-transform: capitalize;
}

.scorecard-grid {
  display: grid;
  gap: 0.45rem;
  grid-template-columns: repeat(auto-fit, minmax(8rem, 1fr));
}

.scorecard-metric {
  background: #0f1a2b;
  border-radius: 0.3rem;
  display: grid;
  gap: 0.2rem;
  padding: 0.45rem;
}

.scorecard-metric span {
  color: #94a3b8;
  font-size: 0.68rem;
}

.scorecard-metric strong {
  color: #e2e8f0;
  font-size: 0.78rem;
}

.scorecard-metric progress {
  accent-color: #38bdf8;
  height: 0.35rem;
  width: 100%;
}

.scorecard-rationale {
  color: #cbd5e1;
  font-size: 0.78rem;
  line-height: 1.45;
}
</style>
