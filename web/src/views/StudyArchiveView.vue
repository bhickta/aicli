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
  <div class="study-archive panel">
    <header class="study-archive-header">
      <div>
        <h2>Study</h2>
        <p class="muted">{{ archive.summary.value }}</p>
      </div>
      <div class="study-archive-search">
        <input v-model="archive.query.value" type="search" placeholder="Search OCR, questions, report, PDF..." @keydown.enter="archive.loadReviews()">
        <button type="button" @click="archive.loadReviews()">Search</button>
      </div>
    </header>

    <nav class="study-subtabs" aria-label="Study sections">
      <button type="button" class="active">Topper answer copies</button>
    </nav>

    <p class="status-line" role="status" aria-live="polite">{{ archive.status.value }}</p>

    <div class="study-archive-layout">
      <aside class="study-review-list" aria-label="Saved topper copy reviews">
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
          <ProviderModelControl
            :provider-id="archive.providerModel.provider_id"
            :model="archive.providerModel.model"
            @change="Object.assign(archive.providerModel, $event)"
          />
          <div class="field-row">
            <div class="field">
              <label>OCR workers</label>
              <input v-model.number="archive.ocrWorkers.value" type="number" min="0">
            </div>
            <div class="field">
              <label>Question workers</label>
              <input v-model.number="archive.questionWorkers.value" type="number" min="0">
            </div>
            <div class="field archive-actions">
              <button type="button" :disabled="archive.running.value" @click="archive.saveReview()">Save edits</button>
              <button type="button" :disabled="archive.running.value" @click="rerun('questions')">Rerun questions</button>
              <button type="button" :disabled="archive.running.value" @click="rerun('report')">Rerun report</button>
              <button type="button" :disabled="archive.running.value || !archive.canRerunOCR.value" @click="rerun('all')">Rerun all</button>
            </div>
            <div class="field archive-delete">
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
  gap: 14px;
  min-height: calc(100vh - 7.5rem);
}

.study-archive-header,
.study-archive-search,
.study-archive-layout,
.study-review-controls,
.archive-actions,
.archive-delete,
.study-subtabs {
  display: flex;
  gap: 10px;
}

.study-archive-header {
  align-items: start;
  justify-content: space-between;
}

.study-archive-header h2,
.study-archive-header p {
  margin: 0;
}

.study-archive-search {
  min-width: min(34rem, 100%);
}

.study-archive-search input {
  flex: 1;
}

.study-archive-layout {
  align-items: stretch;
  display: grid;
  grid-template-columns: minmax(18rem, 22rem) minmax(0, 1fr);
}

.study-review-list {
  background: transparent;
  border-right: 1px solid #2b3440;
  display: grid;
  gap: 8px;
  max-height: calc(100vh - 15rem);
  overflow: auto;
  padding: 0 10px 0 0;
}

.study-review-list button {
  background: #11161d;
  border: 1px solid #343b46;
  border-radius: 6px;
  color: #dce7f5;
  cursor: pointer;
  display: grid;
  gap: 4px;
  padding: 10px;
  text-align: left;
}

.study-review-list button.active {
  border-color: #69a1ff;
  background: #17304f;
}

.study-review-list span {
  color: #9aa7b6;
  font-size: 12px;
}

.study-review-main {
  display: grid;
  gap: 12px;
  min-width: 0;
}

.study-review-controls {
  background: #0d121b;
  border: 1px solid #253247;
  border-radius: 6px;
  display: grid;
  padding: 10px;
}

.archive-actions {
  align-items: end;
  flex-wrap: wrap;
}

.archive-delete {
  align-items: end;
  flex-wrap: wrap;
}

.study-subtabs {
  border-bottom: 1px solid #2b3440;
  padding-bottom: 8px;
}

.study-subtabs button.active {
  border-color: #69a1ff;
  background: #17304f;
}

.danger-button {
  border-color: #7f1d1d;
  background: #3b1115;
  color: #fecaca;
}

@media (max-width: 920px) {
  .study-archive-header,
  .study-archive-layout {
    display: grid;
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
