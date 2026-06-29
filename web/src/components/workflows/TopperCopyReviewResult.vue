<script setup lang="ts">
import "../../styles/topper-copy-review.css";
import { useTopperCopyReview, type TopperRerunAction } from "../../composables/useTopperCopyReview";
import type { TopperCopyReview } from "../../types";
import TopperOcrViewer from "./TopperOcrViewer.vue";

const props = defineProps<{
  review: TopperCopyReview;
  editable?: boolean;
  busy?: boolean;
}>();

const emit = defineEmits<{
  "update:review": [review: TopperCopyReview];
  rerunPage: [action: TopperRerunAction, pageNumber: number];
}>();

const {
  activeTab,
  zoom,
  fullscreen,
  pageEdits,
  reportEdit,
  activePage,
  activePageEdit,
  activeQuestion,
  activeQuestionEdit,
  verifiedCount,
  unclearCount,
  sourcePageLabels,
  activePageHasImage,
  hasQuestionBlocks,
  questionTabLabel,
  selectPage,
  selectQuestion,
  setZoom,
  updateActivePageText,
  toggleActivePageVerified,
  updateActiveQuestionAnswer,
  updateActiveQuestionTitle,
  updateActiveQuestionStatus,
  updateReport,
  rerunActivePage,
} = useTopperCopyReview(
  props,
  (review) => emit("update:review", review),
  (action, pageNumber) => emit("rerunPage", action, pageNumber),
);
</script>

<template>
  <section class="topper-review" :class="{ fullscreen }">
    <header class="topper-review-header">
      <div>
        <h3>Topper Copy Review</h3>
        <p>{{ review.pdf_name }} · {{ review.pages.length }} page(s) · {{ review.questions.length }} question block(s)</p>
      </div>
      <div class="topper-review-actions">
        <span>{{ verifiedCount }}/{{ review.pages.length }} verified</span>
        <span>{{ unclearCount }} unclear</span>
        <button type="button" @click="fullscreen = !fullscreen">{{ fullscreen ? "Exit full screen" : "Full screen" }}</button>
      </div>
    </header>

    <nav class="topper-review-tabs" aria-label="Topper review views">
      <button type="button" :class="{ active: activeTab === 'pages' }" @click="activeTab = 'pages'">Page OCR</button>
      <button type="button" :class="{ active: activeTab === 'questions' }" @click="activeTab = 'questions'">{{ questionTabLabel }}</button>
      <button type="button" :class="{ active: activeTab === 'report' }" @click="activeTab = 'report'">Report</button>
    </nav>

    <div v-if="activeTab === 'pages'" class="topper-review-grid">
      <aside class="topper-review-list" aria-label="Pages">
        <button
          v-for="page in review.pages"
          :key="page.number"
          type="button"
          :class="{ active: activePage?.number === page.number }"
          @click="selectPage(page.number)"
        >
          <strong>Page {{ page.number }}</strong>
          <span>{{ page.unclear_count }} unclear · {{ pageEdits[page.number]?.verified ? "verified" : "unchecked" }}</span>
        </button>
      </aside>

      <main class="topper-page-workspace">
        <section class="topper-page-image">
          <div class="topper-page-toolbar">
            <span>{{ activePage?.name || "Page" }}</span>
            <div>
              <button type="button" :disabled="busy" @click="toggleActivePageVerified">{{ activePageEdit?.verified ? "Unverify" : "Mark verified" }}</button>
              <button v-if="editable" type="button" :disabled="busy || !activePageHasImage" @click="rerunActivePage('ocr')">OCR page</button>
              <button v-if="editable" type="button" :disabled="busy" @click="rerunActivePage('questions')">Split page</button>
              <button v-if="editable" type="button" :disabled="busy || !activePageHasImage" @click="rerunActivePage('all')">Rerun page</button>
              <button type="button" @click="setZoom(zoom - 0.1)">-</button>
              <button type="button" @click="setZoom(1)">Fit</button>
              <button type="button" @click="setZoom(zoom + 0.1)">+</button>
            </div>
          </div>
          <div class="topper-image-scroll">
            <img v-if="activePage?.image_url" :src="activePage.image_url" :alt="`Page ${activePage.number}`" :style="{ transform: `scale(${zoom})` }" />
            <p v-else>No page image saved for this OCR-only record. Question-wise split can still use the OCR text.</p>
          </div>
        </section>
        <section class="topper-ocr-panel">
          <TopperOcrViewer :text="activePageEdit?.text || ''" :editable="editable" @update="updateActivePageText" />
        </section>
      </main>
    </div>

    <div v-else-if="activeTab === 'questions'" class="topper-review-grid">
      <aside class="topper-review-list" aria-label="Questions">
        <p v-if="!review.questions.length" class="topper-empty-state">No question blocks saved.</p>
        <button
          v-for="question in review.questions"
          :key="question.id"
          type="button"
          :class="{ active: activeQuestion?.id === question.id }"
          @click="selectQuestion(question)"
        >
          <strong>{{ question.label }}</strong>
          <span>{{ question.status }} · pages {{ question.source_pages.join(", ") }}</span>
        </button>
      </aside>
      <main v-if="activeQuestion" class="topper-question-workspace">
        <section class="topper-question-panel">
          <p v-if="!hasQuestionBlocks" class="topper-warning">
            Question-wise split has not produced answer blocks yet. These are page-level OCR fallback blocks; rerun questions after choosing a capable model.
          </p>
          <header>
            <div>
              <h4>{{ activeQuestion?.label || "Question" }}</h4>
              <p>{{ activeQuestion?.title || "Detected answer block" }} · {{ sourcePageLabels }}</p>
            </div>
            <span>{{ activeQuestion?.status }}</span>
          </header>
          <div v-if="editable" class="topper-question-meta">
            <label>
              Title
              <input :value="activeQuestionEdit?.title || ''" @input="updateActiveQuestionTitle(($event.target as HTMLInputElement).value)">
            </label>
            <label>
              Status
              <input :value="activeQuestionEdit?.status || ''" @input="updateActiveQuestionStatus(($event.target as HTMLInputElement).value)">
            </label>
          </div>
          <textarea
            v-if="editable"
            class="topper-question-editor"
            :value="activeQuestionEdit?.answer || ''"
            @input="updateActiveQuestionAnswer(($event.target as HTMLTextAreaElement).value)"
          />
          <pre v-else>{{ activeQuestion?.answer_markdown || "" }}</pre>
        </section>
        <section class="topper-page-image compact">
          <div class="topper-page-toolbar">
            <span>Source page {{ activePage?.number }}</span>
            <button type="button" @click="activeTab = 'pages'">Open page OCR</button>
          </div>
          <div class="topper-image-scroll">
            <img v-if="activePage?.image_url" :src="activePage.image_url" :alt="`Page ${activePage.number}`" />
          </div>
        </section>
      </main>
      <main v-else class="topper-question-panel">
        <p class="topper-empty-state">No question answer block is available. Rerun question-wise split for this review.</p>
      </main>
    </div>

    <section v-else class="topper-report-panel">
      <h4>Final analysis</h4>
      <textarea v-if="editable" class="topper-report-editor" :value="reportEdit" @input="updateReport(($event.target as HTMLTextAreaElement).value)" />
      <pre v-else>{{ review.report }}</pre>
    </section>
  </section>
</template>
