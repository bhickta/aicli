<script setup lang="ts">
import { computed } from "vue";
import type { StudyAnalysisRecord, StudyCopyDetail } from "../../types";

const props = defineProps<{ detail: StudyCopyDetail | null }>();

const report = computed(() => {
  const item = props.detail?.analyses.find((analysis) => analysis.dimension_key === "report");
  if (!item) return "";
  try {
    const payload = JSON.parse(item.result_json) as { report?: string };
    return payload.report || item.result_json;
  } catch {
    return item.result_json;
  }
});

function label(analysis: StudyAnalysisRecord) {
  return `${analysis.scope_type || "copy"} / ${analysis.dimension_key || "analysis"}`;
}
</script>

<template>
  <section class="study-card study-analytics">
    <header class="study-card-header">
      <div>
        <h2>Analytics</h2>
        <p>Copy, question, and dimension-wise review outputs.</p>
      </div>
    </header>
    <pre v-if="report" class="study-report">{{ report }}</pre>
    <div v-else class="study-empty">No report saved yet.</div>
    <div class="study-analysis-list">
      <span v-for="analysis in detail?.analyses || []" :key="analysis.id" class="study-pill">
        {{ label(analysis) }}
      </span>
    </div>
  </section>
</template>
