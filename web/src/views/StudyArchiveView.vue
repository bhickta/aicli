<script setup lang="ts">
import "../styles/study-archive.css";
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
            <div class="study-step-models">
              <div class="field">
                <label>OCR model</label>
                <ProviderModelControl
                  :provider-id="archive.ocrProviderModel.provider_id"
                  :model="archive.ocrProviderModel.model"
                  @change="Object.assign(archive.ocrProviderModel, $event)"
                />
              </div>
              <div class="field">
                <label>Question split model</label>
                <ProviderModelControl
                  :provider-id="archive.questionProviderModel.provider_id"
                  :model="archive.questionProviderModel.model"
                  @change="Object.assign(archive.questionProviderModel, $event)"
                />
              </div>
              <div class="field">
                <label>Report model</label>
                <ProviderModelControl
                  :provider-id="archive.reportProviderModel.provider_id"
                  :model="archive.reportProviderModel.model"
                  @change="Object.assign(archive.reportProviderModel, $event)"
                />
              </div>
            </div>
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
                <input v-model="archive.unloadModels.value" type="checkbox">
                Unload local models after run
              </label>
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
