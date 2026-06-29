<script setup lang="ts">
import "../styles/study-archive.css";
import { onMounted, watch } from "vue";
import ProviderModelControl from "../components/ProviderModelControl.vue";
import TopperCopyReviewResult from "../components/workflows/TopperCopyReviewResult.vue";
import { useStudyArchive, type TopperRerunAction } from "../composables/useStudyArchive";
import type { TopperReviewRecord } from "../types";

const props = defineProps<{ reviewId?: string }>();
const archive = useStudyArchive();

onMounted(() => {
  if (props.reviewId) {
    void archive.openReview({ id: props.reviewId } as TopperReviewRecord);
  } else {
    void archive.loadReviews();
  }
});

watch(() => props.reviewId, (newId) => {
  if (newId) {
    void archive.openReview({ id: newId } as TopperReviewRecord);
  }
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
  <div class="study-archive" :style="reviewId ? 'height: auto; border: none; background: transparent; padding: 0;' : ''">
    <header v-if="!reviewId" class="study-archive-toolbar">
      <div class="study-archive-heading">
        <h3>Topper Answer Copies</h3>
        <p class="muted">{{ archive.summary.value }}</p>
      </div>
      <div class="study-archive-search">
        <input v-model="archive.query.value" type="search" placeholder="Search OCR, questions, report, PDF..." @keydown.enter="archive.loadReviews()">
        <button type="button" @click="archive.loadReviews()">Search</button>
      </div>
    </header>

    <div class="study-archive-layout" :style="reviewId ? 'grid-template-columns: 1fr;' : ''">
      <aside v-if="!reviewId" class="study-review-list" aria-label="Saved topper copy reviews">
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

      <main class="study-review-main" :style="reviewId ? 'padding: 0; background: transparent; border: none;' : ''">
        <section v-if="archive.selectedReview.value" :class="['study-review-controls', reviewId ? 'embedded' : '']">
          <div class="study-review-control-row">
            <div class="study-step-models">
              <div class="field">
                <label>Gemini-Lite Model</label>
                <ProviderModelControl
                  :provider-id="archive.providerModel.provider_id"
                  :model="archive.providerModel.model"
                  @change="Object.assign(archive.providerModel, $event)"
                />
              </div>
            </div>
          </div>

          <div class="study-review-action-row">
            <div class="action-cards-grid">
              <div class="action-card">
                <div class="action-info">
                  <h4>Open PDF</h4>
                  <p>Open the original uploaded copy in a new tab.</p>
                </div>
                <a v-if="archive.sourcePDFURL.value" class="archive-action-link" :href="archive.sourcePDFURL.value" target="_blank" rel="noopener">Open</a>
                <button v-else type="button" disabled>Unavailable</button>
              </div>

              <div class="action-card">
                <div class="action-info">
                  <h4>Save Edits</h4>
                  <p>Save any manual edits made to the text.</p>
                </div>
                <button type="button" :disabled="archive.running.value" @click="archive.saveReview()">Save</button>
              </div>

              <div class="action-card primary-card">
                <div class="action-info">
                  <h4>Rerun Analytics</h4>
                  <p>Fast! Only extracts dimensions (Intro/Fact) for existing questions.</p>
                </div>
                <button type="button" class="primary" :disabled="archive.running.value" @click="rerun('analytics')">Run</button>
              </div>

              <div class="action-card">
                <div class="action-info">
                  <h4>Rerun Report</h4>
                  <p>Generates the final overarching summary from existing questions.</p>
                </div>
                <button type="button" :disabled="archive.running.value" @click="rerun('report')">Run</button>
              </div>

              <div class="action-card">
                <div class="action-info">
                  <h4>Rerun All</h4>
                  <p>{{ archive.isPDFDirectReview.value ? "Runs full Gemini-Lite PDF extraction from the original PDF." : "Runs the full OCR and extraction pipeline from scratch." }}</p>
                </div>
                <button type="button" :disabled="archive.running.value || !archive.canRerunAll.value" @click="rerun('all')">Run</button>
              </div>
            </div>
            
            <div class="archive-delete" v-if="!reviewId">
              <label class="checkbox">
                <input v-model="archive.unloadModels.value" type="checkbox"> Unload models
              </label>
              <label class="checkbox">
                <input v-model="archive.deletePDF.value" type="checkbox"> Delete PDF
              </label>
              <button type="button" class="danger-button" :disabled="archive.running.value" @click="archive.deleteReview()">Delete Copy</button>
            </div>
          </div>
        </section>

        <TopperCopyReviewResult
          v-if="!reviewId && archive.selectedReview.value"
          :review="archive.selectedReview.value"
          editable
          :busy="archive.running.value"
          @update:review="archive.updateSelectedReview"
          @rerun-page="(action, pageNumber) => rerun(action, [pageNumber])"
        />
        <p v-else-if="!reviewId" class="empty-state">Select a review to inspect OCR, questions, and report.</p>
      </main>
    </div>
  </div>
</template>
