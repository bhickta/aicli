<script setup lang="ts">
import { computed } from "vue";
import type { QuestionDimensions } from "../../types";
import TopperAnalysisList from "./topper-analysis/TopperAnalysisList.vue";
import TopperAnalysisPoints from "./topper-analysis/TopperAnalysisPoints.vue";
import TopperImprovementPlan from "./topper-analysis/TopperImprovementPlan.vue";
import TopperQuestionScorecard from "./topper-analysis/TopperQuestionScorecard.vue";

const props = defineProps<{
  dimensions?: QuestionDimensions;
}>();

interface AnalysisRow {
  label: string;
  value: string;
}

const diagnosticRows = computed<AnalysisRow[]>(() => compactRows([
  { label: "Demand alignment", value: props.dimensions?.demand_alignment || "" },
  { label: "Body structure", value: props.dimensions?.body_structure || "" },
  { label: "Content depth", value: props.dimensions?.content_depth || "" },
  { label: "Dimensions covered", value: props.dimensions?.multidimensionality || "" },
  { label: "Presentation", value: props.dimensions?.presentation || "" },
]));

const foundationRows = computed<AnalysisRow[]>(() => compactRows([
  { label: "Introduction", value: props.dimensions?.introduction || "" },
  { label: "Conclusion", value: props.dimensions?.outro || "" },
  { label: "Transitions", value: props.dimensions?.transition || "" },
  { label: "Diagram", value: props.dimensions?.diagram || "" },
  { label: "Facts", value: props.dimensions?.fact || "" },
  { label: "Fact usage", value: props.dimensions?.fact_usage || "" },
  { label: "Other observations", value: props.dimensions?.custom || "" },
]));

const hasAnalysis = computed(() => {
  const dimensions = props.dimensions;
  if (!dimensions) return false;
  return diagnosticRows.value.length > 0
    || foundationRows.value.length > 0
    || Boolean(dimensions.scorecard)
    || Boolean(dimensions.strengths?.length)
    || Boolean(dimensions.gaps?.length)
    || Boolean(dimensions.improvements?.length)
    || Boolean(dimensions.missing_dimensions?.length)
    || Boolean(dimensions.examiner_signals?.length)
    || Boolean(dimensions.reusable_techniques?.length);
});

function compactRows(rows: AnalysisRow[]) {
  return rows.filter((row) => row.value.trim() !== "");
}
</script>

<template>
  <section v-if="hasAnalysis" class="topper-dimensions">
    <header class="analysis-header">
      <div>
        <h5>Question analysis</h5>
        <p>Evidence-based learning diagnosis from the extracted answer</p>
      </div>
    </header>

    <TopperQuestionScorecard :scorecard="dimensions?.scorecard" />

    <section v-if="diagnosticRows.length" class="analysis-details">
      <h6>Demand and execution</h6>
      <dl>
        <div v-for="row in diagnosticRows" :key="row.label">
          <dt>{{ row.label }}</dt>
          <dd>{{ row.value }}</dd>
        </div>
      </dl>
    </section>

    <div class="analysis-points-grid">
      <TopperAnalysisPoints title="Evidence-backed strengths" tone="strength" :points="dimensions?.strengths" />
      <TopperAnalysisPoints title="Gaps and risks" tone="gap" :points="dimensions?.gaps" />
    </div>

    <TopperImprovementPlan :improvements="dimensions?.improvements" />

    <div class="analysis-list-grid">
      <TopperAnalysisList title="Reusable techniques" tone="positive" :values="dimensions?.reusable_techniques" />
      <TopperAnalysisList title="Missing demand-relevant dimensions" tone="warning" :values="dimensions?.missing_dimensions" />
      <TopperAnalysisList title="Examiner-friendly signals" :values="dimensions?.examiner_signals" />
    </div>

    <section v-if="foundationRows.length" class="analysis-details compact">
      <h6>Structural detail</h6>
      <dl>
        <div v-for="row in foundationRows" :key="row.label">
          <dt>{{ row.label }}</dt>
          <dd>{{ row.value }}</dd>
        </div>
      </dl>
    </section>
  </section>
</template>

<style scoped>
.analysis-header h5,
.analysis-header p,
.analysis-details h6 {
  margin: 0;
}

.analysis-header p {
  color: #94a3b8;
  font-size: 0.7rem;
  margin-top: 0.12rem;
}

.analysis-details {
  background: #0c1421;
  border: 1px solid #293a51;
  border-radius: 0.4rem;
  display: grid;
  gap: 0.45rem;
  padding: 0.65rem;
}

.analysis-details h6 {
  color: #e2e8f0;
  font-size: 0.78rem;
}

.analysis-details dl {
  display: grid;
  gap: 0.5rem;
  margin: 0;
}

.analysis-details.compact dl {
  grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr));
}

.analysis-details dl div {
  display: grid;
  gap: 0.12rem;
}

.analysis-details dt {
  color: #93c5fd;
  font-size: 0.68rem;
  font-weight: 700;
  text-transform: uppercase;
}

.analysis-details dd {
  color: #cbd5e1;
  font-size: 0.78rem;
  line-height: 1.45;
  margin: 0;
}

.analysis-points-grid,
.analysis-list-grid {
  display: grid;
  gap: 0.55rem;
  grid-template-columns: repeat(auto-fit, minmax(15rem, 1fr));
}
</style>
