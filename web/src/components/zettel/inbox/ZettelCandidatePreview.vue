<script setup lang="ts">
import { computed, shallowRef } from "vue";
import type { InboxCandidatePreviewReport, InboxCandidateSource } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";

const props = defineProps<{
  report: InboxCandidatePreviewReport | null;
}>();

const selectedSourcePath = shallowRef("");

const sources = computed(() => props.report?.sources || []);
const selectedSource = computed<InboxCandidateSource | null>(() => {
  return sources.value.find((source) => source.source_path === selectedSourcePath.value) || sources.value[0] || null;
});
const reportBadges = computed(() => {
  if (!props.report) return [];
  return [
    `${props.report.selected_count} selected`,
    `${props.report.source_count} in inbox`,
    `${props.report.skipped_count} skipped`,
  ];
});

function selectSource(path: string) {
  selectedSourcePath.value = path;
}

function scoreLabel(score: number): string {
  return `${(score * 100).toFixed(1)}%`;
}
</script>

<template>
  <section v-if="report" class="candidate-preview">
    <header class="candidate-preview-header">
      <div>
        <h3>Embedding Candidate Preview</h3>
        <p class="muted">Read-only semantic matches from the current inbox selection.</p>
      </div>
      <ZettelBadges :items="reportBadges" />
    </header>

    <div class="candidate-preview-grid">
      <div class="candidate-source-list">
        <button
          v-for="source in sources"
          :key="source.source_path"
          type="button"
          class="candidate-source-row"
          :class="{ active: selectedSource?.source_path === source.source_path }"
          @click="selectSource(source.source_path)"
        >
          <strong>{{ source.source_path }}</strong>
          <span v-if="source.error">{{ source.error }}</span>
          <span v-else>{{ source.candidates.length }} candidates</span>
          <small v-if="source.candidates[0]">
            top: {{ source.candidates[0].path }} | {{ scoreLabel(source.candidates[0].similarity) }}
          </small>
        </button>
      </div>

      <article v-if="selectedSource" class="candidate-detail">
        <h4>{{ selectedSource.source_path }}</h4>
        <p v-if="selectedSource.error" class="status-line compact">{{ selectedSource.error }}</p>

        <details v-if="selectedSource.source_excerpt" class="source-excerpt">
          <summary>Source excerpt</summary>
          <pre>{{ selectedSource.source_excerpt }}</pre>
        </details>

        <div class="candidate-list">
          <article v-for="candidate in selectedSource.candidates" :key="candidate.path" class="candidate-card">
            <div class="candidate-card-header">
              <strong>{{ candidate.path }}</strong>
              <span>{{ scoreLabel(candidate.similarity) }}</span>
            </div>
            <pre v-if="candidate.excerpt">{{ candidate.excerpt }}</pre>
          </article>
        </div>
      </article>
    </div>
  </section>
</template>

<style scoped>
.candidate-preview {
  display: grid;
  gap: 12px;
}

.candidate-preview-header {
  margin: 0;
}

.candidate-preview-header p {
  margin: 4px 0 0;
}

.candidate-preview-grid {
  display: grid;
  grid-template-columns: minmax(260px, 0.8fr) minmax(0, 1.4fr);
  gap: 12px;
  min-width: 0;
}

.candidate-source-list,
.candidate-detail,
.candidate-list {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.candidate-source-row {
  display: grid;
  gap: 4px;
  text-align: left;
}

.candidate-source-row.active {
  border-color: #6ea8fe;
  background: #2d405e;
}

.candidate-source-row span,
.candidate-source-row small {
  color: #9aa4b2;
}

.candidate-detail {
  padding: 12px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.candidate-card {
  display: grid;
  gap: 8px;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #161b22;
}

.candidate-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.candidate-card-header span {
  flex: 0 0 auto;
  color: #9cc2ff;
  font-variant-numeric: tabular-nums;
}

.source-excerpt,
.candidate-card pre {
  min-width: 0;
}

@media (max-width: 820px) {
  .candidate-preview-grid {
    grid-template-columns: 1fr;
  }
}
</style>
