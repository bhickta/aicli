<script setup lang="ts">
import { onMounted } from "vue";
import ProviderModelControl from "../components/ProviderModelControl.vue";
import TopperCopyReviewResult from "../components/workflows/TopperCopyReviewResult.vue";
import { useStudyArchive, type TopperRerunAction } from "../composables/useStudyArchive";
import type { TopperReviewRecord } from "../types";

const archive = useStudyArchive();

onMounted(() => {
  void archive.loadReviews();
});

function recordTime(record: TopperReviewRecord) {
  const value = record.updated_at || record.created_at;
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  return date.toLocaleString();
}

function rerun(action: TopperRerunAction, pageNumbers: number[] = []) {
  void archive.rerunReview(action, pageNumbers);
}
</script>

<template>
  <div class="study-archive">
    <header class="study-archive-toolbar">
      <div class="study-archive-heading">
        <h3>Topper Answer Copies</h3>
        <p class="muted">{{ archive.summary.value }}</p>
      </div>
      <div class="study-archive-search">
        <input v-model="archive.query.value" type="search" placeholder="Search OCR, questions, report, PDF..." @keydown.enter="archive.loadReviews()">
        <button type="button" @click="archive.loadReviews()">Search</button>
      </div>
    </header>

    <div class="study-archive-layout">
      <aside class="study-review-list" aria-label="Saved topper copy reviews">
        <p class="study-archive-status" role="status" aria-live="polite">{{ archive.status.value }}</p>
        <button
          v-for="record in archive.reviews.value"
          :key="record.id"
          type="button"
          :class="{ active: archive.selectedRecord.value?.id === record.id }"
          @click="archive.openReview(record)"
        >
          <strong>{{ record.pdf_name || record.id }}</strong>
          <span>{{ record.page_count }} pages · {{ record.question_count }} questions · {{ record.unclear_count }} unclear</span>
          <span>{{ recordTime(record) }}</span>
        </button>
        <p v-if="!archive.reviews.value.length" class="empty-state">No saved topper reviews yet.</p>
      </aside>

      <main class="study-review-main">
        <section v-if="archive.selectedReview.value" class="study-review-controls">
          <div class="study-review-control-row">
            <ProviderModelControl
              class="study-provider-control"
              :provider-id="archive.providerModel.provider_id"
              :model="archive.providerModel.model"
              @change="Object.assign(archive.providerModel, $event)"
            />
            <div class="study-worker-controls">
              <label>
                OCR
                <input v-model.number="archive.ocrWorkers.value" type="number" min="0">
              </label>
              <label>
                Questions
                <input v-model.number="archive.questionWorkers.value" type="number" min="0">
              </label>
            </div>
          </div>
          <div class="study-review-action-row">
            <div class="archive-actions">
              <button type="button" :disabled="archive.running.value" @click="archive.saveReview()">Save edits</button>
              <button type="button" :disabled="archive.running.value" @click="rerun('questions')">Rerun questions</button>
              <button type="button" :disabled="archive.running.value" @click="rerun('report')">Rerun report</button>
              <button type="button" :disabled="archive.running.value || !archive.canRerunOCR.value" @click="rerun('all')">Rerun all</button>
            </div>
            <div class="archive-delete">
              <label class="checkbox">
                <input v-model="archive.deletePDF.value" type="checkbox">
                Delete uploaded PDF too
              </label>
              <button type="button" class="danger-button" :disabled="archive.running.value" @click="archive.deleteReview()">Delete copy + assets</button>
            </div>
          </div>
        </section>

        <TopperCopyReviewResult
          v-if="archive.selectedReview.value"
          :review="archive.selectedReview.value"
          editable
          :busy="archive.running.value"
          @update:review="archive.updateSelectedReview"
          @rerun-page="(action, pageNumber) => rerun(action, [pageNumber])"
        />
        <p v-else class="empty-state">Select a review to inspect OCR, questions, and report.</p>
      </main>
    </div>
  </div>
</template>

<style scoped>
.study-archive {
  display: grid;
  gap: 8px;
  padding: 0;
  border: 0;
  background: transparent;
}

.study-archive-toolbar,
.study-archive-search,
.study-archive-layout,
.study-review-controls,
.study-review-control-row,
.study-review-action-row,
.study-worker-controls,
.archive-actions,
.archive-delete {
  display: flex;
  gap: 8px;
}

.study-archive-toolbar {
  align-items: center;
  justify-content: space-between;
  min-width: 0;
}

.study-archive-heading {
  min-width: 0;
}

.study-archive-heading h3,
.study-archive-heading p {
  margin: 0;
}

.study-archive-heading h3 {
  font-size: 15px;
  line-height: 1.2;
}

.study-archive-search {
  flex: 0 1 32rem;
  min-width: min(22rem, 100%);
}

.study-archive-search input {
  flex: 1;
  min-width: 0;
}

.study-archive-search button {
  text-align: center;
  white-space: nowrap;
}

.study-archive-layout {
  align-items: stretch;
  display: grid;
  gap: 10px;
  grid-template-columns: minmax(16rem, 19rem) minmax(0, 1fr);
  min-width: 0;
}

.study-review-list {
  background: transparent;
  border-right: 1px solid #2b3440;
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: calc(100vh - 11.5rem);
  overflow: auto;
  padding: 0 8px 0 0;
}

.study-archive-status {
  background: #0d121b;
  border-left: 2px solid #69a1ff;
  color: #b8c3d2;
  font-size: 12px;
  margin: 0 0 2px;
  padding: 6px 8px;
}

.study-review-list button {
  background: #11161d;
  border: 1px solid #343b46;
  border-radius: 6px;
  color: #dce7f5;
  cursor: pointer;
  display: grid;
  gap: 3px;
  padding: 8px;
  text-align: left;
}

.study-review-list strong {
  font-size: 13px;
  line-height: 1.25;
  overflow-wrap: anywhere;
}

.study-review-list button.active {
  border-color: #69a1ff;
  background: #17304f;
}

.study-review-list span {
  color: #9aa7b6;
  font-size: 11px;
}

.study-review-main {
  display: grid;
  gap: 8px;
  min-width: 0;
}

.study-review-controls {
  background: #0d121b;
  border: 1px solid #253247;
  border-radius: 7px;
  display: grid;
  gap: 8px;
  padding: 8px;
}

.study-review-control-row,
.study-review-action-row {
  align-items: end;
  justify-content: space-between;
  min-width: 0;
}

.study-provider-control {
  flex: 1 1 34rem;
  min-width: 0;
}

.study-worker-controls {
  align-items: end;
  flex: 0 0 auto;
}

.study-worker-controls label {
  color: #94a3b8;
  display: grid;
  font-size: 12px;
  gap: 4px;
  width: 6.5rem;
}

.study-worker-controls input {
  min-width: 0;
  width: 100%;
}

.archive-actions {
  align-items: end;
  flex-wrap: wrap;
  min-width: 0;
}

.archive-delete {
  align-items: end;
  flex-wrap: wrap;
  justify-content: flex-end;
}

.danger-button {
  border-color: #7f1d1d;
  background: #3b1115;
  color: #fecaca;
}

@media (max-width: 920px) {
  .study-archive-toolbar,
  .study-archive-layout,
  .study-review-control-row,
  .study-review-action-row {
    display: grid;
    grid-template-columns: 1fr;
  }

  .study-worker-controls {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .study-review-list {
    border-right: 0;
    border-bottom: 1px solid #2b3440;
    display: flex;
    flex: none;
    max-height: none;
    overflow-x: auto;
    padding: 0 0 10px;
  }

  .study-review-list button {
    min-width: 16rem;
  }
}
</style>
