<script setup lang="ts">
import { computed } from "vue";
import type { AnalysisQuality } from "../../../types";

const props = defineProps<{
  quality?: AnalysisQuality;
}>();

const metrics = computed(() => {
  const quality = props.quality;
  if (!quality) return [];
  const values = [
    { label: "Page classification", value: boundedPercent(quality.classification_coverage_percent) },
    { label: "Classification confidence", value: boundedPercent(quality.average_classification_confidence * 100) },
    { label: "OCR reliability assessed", value: boundedPercent(quality.ocr_assessment_coverage_percent) },
    { label: "Average OCR confidence", value: boundedPercent(quality.average_ocr_confidence * 100) },
    { label: "Exact prompt matching", value: boundedPercent(quality.prompt_match_percent) },
    { label: "Structured analysis", value: boundedPercent(quality.analysis_coverage_percent) },
    { label: "Evidence-backed analysis", value: boundedPercent(quality.evidence_coverage_percent) },
  ];
  if (typeof quality.minimum_ocr_confidence === "number") {
    values.splice(4, 0, {
      label: "Lowest page OCR confidence",
      value: boundedPercent(quality.minimum_ocr_confidence * 100),
    });
  }
  return values;
});

const overallCoverage = computed(() => boundedPercent(props.quality?.overall_coverage_percent || 0));
const unclearPercent = computed(() => Math.max(0, Number(props.quality?.ocr_unclear_percent) || 0).toFixed(2));
const ocrReviewPages = computed(() => props.quality?.ocr_review_pages || []);

function boundedPercent(value: number) {
  return Math.round(Math.min(100, Math.max(0, Number(value) || 0)));
}
</script>

<template>
  <section v-if="quality" class="analysis-quality" :class="{ review: quality.requires_review }">
    <header class="quality-header">
      <div>
        <h4>Analysis coverage</h4>
        <p>Pipeline coverage and model confidence—not measured answer accuracy</p>
      </div>
      <strong>{{ overallCoverage }}%</strong>
    </header>

    <div class="quality-grid">
      <div v-for="metric in metrics" :key="metric.label" class="quality-metric">
        <span>{{ metric.label }}</span>
        <strong>{{ metric.value }}%</strong>
        <progress :value="metric.value" max="100">{{ metric.value }}%</progress>
      </div>
      <div class="quality-metric">
        <span>OCR unclear markers</span>
        <strong>{{ unclearPercent }}%</strong>
        <p>Share of extracted words marked unclear</p>
      </div>
    </div>

    <p v-if="ocrReviewPages.length" class="quality-review-pages">
      <strong>Verify OCR pages:</strong> {{ ocrReviewPages.join(", ") }}
    </p>

    <ul v-if="quality.warnings?.length" class="quality-warnings">
      <li v-for="warning in quality.warnings" :key="warning">{{ warning }}</li>
    </ul>
  </section>
</template>

<style scoped>
.analysis-quality {
  background: #0b1b18;
  border: 1px solid #245847;
  border-radius: 0.45rem;
  display: grid;
  gap: 0.65rem;
  padding: 0.75rem;
}

.analysis-quality.review {
  background: #21170f;
  border-color: #79512b;
}

.quality-header {
  align-items: center;
  display: flex;
  gap: 0.75rem;
  justify-content: space-between;
}

.quality-header h4,
.quality-header p,
.quality-metric p {
  margin: 0;
}

.quality-header h4 {
  color: #e5e7eb;
  font-size: 0.85rem;
}

.quality-header p,
.quality-metric p {
  color: #94a3b8;
  font-size: 0.68rem;
  margin-top: 0.12rem;
}

.quality-header > strong {
  color: #7dd3fc;
  font-size: 1.2rem;
}

.quality-review-pages {
  color: #fbbf24;
  font-size: 0.75rem;
  margin: 0;
}

.quality-grid {
  display: grid;
  gap: 0.45rem;
  grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
}

.quality-metric {
  background: rgb(2 6 23 / 42%);
  border-radius: 0.3rem;
  display: grid;
  gap: 0.2rem;
  padding: 0.48rem;
}

.quality-metric span {
  color: #a8b5c7;
  font-size: 0.68rem;
}

.quality-metric strong {
  color: #e2e8f0;
  font-size: 0.8rem;
}

.quality-metric progress {
  accent-color: #38bdf8;
  height: 0.35rem;
  width: 100%;
}

.quality-warnings {
  color: #fed7aa;
  display: grid;
  font-size: 0.72rem;
  gap: 0.25rem;
  line-height: 1.4;
  margin: 0;
  padding-left: 1rem;
}
</style>
