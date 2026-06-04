<script setup lang="ts">
import { computed, reactive, shallowRef, watch } from "vue";
import type { TopperCopyPage, TopperCopyQuestion, TopperCopyReview } from "../../types";

type TopperRerunAction = "ocr" | "questions" | "report" | "all";

const props = defineProps<{
  review: TopperCopyReview;
  editable?: boolean;
  busy?: boolean;
}>();

const emit = defineEmits<{
  "update:review": [review: TopperCopyReview];
  rerunPage: [action: TopperRerunAction, pageNumber: number];
}>();

const activeTab = shallowRef<"pages" | "questions" | "report">("pages");
const activePageNumber = shallowRef(props.review.pages[0]?.number || 1);
const activeQuestionID = shallowRef(props.review.questions[0]?.id || "");
const zoom = shallowRef(1);
const fullscreen = shallowRef(false);
const pageEdits = reactive<Record<number, { text: string; verified: boolean }>>({});
const questionEdits = reactive<Record<string, { answer: string; title: string; status: string }>>({});
const reportEdit = shallowRef(props.review.report);

watch(
  () => props.review,
  (review, previousReview) => {
    const previousPageNumber = activePageNumber.value;
    const previousQuestionID = activeQuestionID.value;
    const reviewChanged = review.review_id !== previousReview?.review_id;
    for (const page of review.pages) {
      pageEdits[page.number] = { text: page.text, verified: page.verified };
    }
    for (const question of review.questions) {
      questionEdits[question.id] = {
        answer: question.answer_markdown,
        title: question.title || "",
        status: question.status,
      };
    }
    reportEdit.value = review.report;
    activePageNumber.value = reviewChanged
      ? review.pages[0]?.number || 1
      : review.pages.some((page) => page.number === previousPageNumber)
        ? previousPageNumber
        : review.pages[0]?.number || 1;
    activeQuestionID.value = reviewChanged
      ? review.questions[0]?.id || ""
      : review.questions.some((question) => question.id === previousQuestionID)
        ? previousQuestionID
        : review.questions[0]?.id || "";
  },
  { immediate: true },
);

const activePage = computed<TopperCopyPage | undefined>(() => {
  return props.review.pages.find((page) => page.number === activePageNumber.value) || props.review.pages[0];
});
const activePageEdit = computed(() => {
  if (!activePage.value) return null;
  return pageEdits[activePage.value.number] || { text: activePage.value.text, verified: activePage.value.verified };
});
const activeQuestion = computed<TopperCopyQuestion | undefined>(() => {
  return props.review.questions.find((question) => question.id === activeQuestionID.value) || props.review.questions[0];
});
const activeQuestionEdit = computed(() => {
  if (!activeQuestion.value) return null;
  return questionEdits[activeQuestion.value.id] || {
    answer: activeQuestion.value.answer_markdown,
    title: activeQuestion.value.title || "",
    status: activeQuestion.value.status,
  };
});
const verifiedCount = computed(() => props.review.pages.filter((page) => pageEdits[page.number]?.verified).length);
const unclearCount = computed(() => props.review.pages.reduce((total, page) => total + page.unclear_count, 0));
const sourcePageLabels = computed(() => activeQuestion.value?.source_pages.map((page) => `P${page}`).join(", ") || "");
const activePageHasImage = computed(() => Boolean(activePage.value?.image_url || activePage.value?.path));

function selectPage(pageNumber: number) {
  activeTab.value = "pages";
  activePageNumber.value = pageNumber;
}

function selectQuestion(question: TopperCopyQuestion) {
  activeTab.value = "questions";
  activeQuestionID.value = question.id;
  if (question.source_pages[0]) activePageNumber.value = question.source_pages[0];
}

function setZoom(nextZoom: number) {
  zoom.value = Math.min(2, Math.max(0.6, nextZoom));
}

function updateActivePageText(value: string) {
  if (!activePage.value) return;
  pageEdits[activePage.value.number] = {
    text: value,
    verified: pageEdits[activePage.value.number]?.verified || false,
  };
  emitReview();
}

function toggleActivePageVerified() {
  if (!activePage.value) return;
  pageEdits[activePage.value.number] = {
    text: activePageEdit.value?.text || activePage.value.text,
    verified: !activePageEdit.value?.verified,
  };
  emitReview();
}

function updateActiveQuestionAnswer(value: string) {
	if (!activeQuestion.value) return;
	questionEdits[activeQuestion.value.id] = {
		answer: value,
		title: activeQuestionEdit.value?.title || activeQuestion.value.title || "",
		status: activeQuestionEdit.value?.status || activeQuestion.value.status,
	};
	emitReview();
}

function updateActiveQuestionTitle(value: string) {
	if (!activeQuestion.value) return;
	questionEdits[activeQuestion.value.id] = {
		answer: activeQuestionEdit.value?.answer || activeQuestion.value.answer_markdown,
		title: value,
		status: activeQuestionEdit.value?.status || activeQuestion.value.status,
	};
	emitReview();
}

function updateActiveQuestionStatus(value: string) {
	if (!activeQuestion.value) return;
	questionEdits[activeQuestion.value.id] = {
		answer: activeQuestionEdit.value?.answer || activeQuestion.value.answer_markdown,
		title: activeQuestionEdit.value?.title || activeQuestion.value.title || "",
		status: value,
	};
	emitReview();
}

function updateReport(value: string) {
  reportEdit.value = value;
  emitReview();
}

function rerunActivePage(action: TopperRerunAction) {
  if (!activePage.value) return;
  emit("rerunPage", action, activePage.value.number);
}

function emitReview() {
  if (!props.editable) return;
  emit("update:review", {
    ...props.review,
    pages: props.review.pages.map((page) => ({
      ...page,
      text: pageEdits[page.number]?.text ?? page.text,
      verified: pageEdits[page.number]?.verified ?? page.verified,
    })),
    questions: props.review.questions.map((question) => ({
      ...question,
      title: questionEdits[question.id]?.title ?? question.title,
      status: questionEdits[question.id]?.status ?? question.status,
      answer_markdown: questionEdits[question.id]?.answer ?? question.answer_markdown,
    })),
    report: reportEdit.value,
  });
}
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
      <button type="button" :class="{ active: activeTab === 'questions' }" @click="activeTab = 'questions'">Question-wise</button>
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
          <h4>OCR text</h4>
          <textarea :readonly="!editable" :value="activePageEdit?.text || ''" @input="updateActivePageText(($event.target as HTMLTextAreaElement).value)" />
        </section>
      </main>
    </div>

    <div v-else-if="activeTab === 'questions'" class="topper-review-grid">
      <aside class="topper-review-list" aria-label="Questions">
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
      <main class="topper-question-workspace">
        <section class="topper-question-panel">
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
    </div>

    <section v-else class="topper-report-panel">
      <h4>Final analysis</h4>
      <textarea v-if="editable" class="topper-report-editor" :value="reportEdit" @input="updateReport(($event.target as HTMLTextAreaElement).value)" />
      <pre v-else>{{ review.report }}</pre>
    </section>
  </section>
</template>

<style scoped>
.topper-review {
  border: 1px solid #253247;
  border-radius: 0.5rem;
  background: #0d121b;
  display: grid;
  gap: 0.75rem;
  min-width: 0;
  padding: 0.85rem;
}

.topper-review.fullscreen {
  position: fixed;
  inset: 1rem;
  z-index: 50;
  overflow: auto;
}

.topper-review-header,
.topper-review-actions,
.topper-review-tabs,
.topper-page-toolbar,
.topper-question-panel header {
  align-items: center;
  display: flex;
  gap: 0.6rem;
  justify-content: space-between;
}

.topper-review-header h3,
.topper-question-panel h4,
.topper-report-panel h4,
.topper-ocr-panel h4 {
  margin: 0;
}

.topper-review-header p,
.topper-question-panel p {
  color: #94a3b8;
  margin: 0.2rem 0 0;
}

.topper-review-actions span,
.topper-question-panel header span {
  background: #172033;
  border: 1px solid #2e3c54;
  border-radius: 999px;
  color: #cbd5e1;
  font-size: 0.78rem;
  padding: 0.25rem 0.55rem;
}

.topper-review button {
  background: #111827;
  border: 1px solid #334155;
  border-radius: 0.42rem;
  color: #dbeafe;
  cursor: pointer;
  font: inherit;
  padding: 0.42rem 0.65rem;
}

.topper-review button:hover,
.topper-review button.active {
  border-color: #60a5fa;
}

.topper-review-tabs {
  justify-content: flex-start;
}

.topper-review-tabs button.active {
  background: #1e3a5f;
}

.topper-review-grid {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: minmax(12rem, 18rem) minmax(0, 1fr);
  min-height: min(42rem, calc(100vh - 15rem));
}

.topper-review-list {
  background: transparent;
  border-right: 1px solid #253247;
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  max-height: 70vh;
  overflow: auto;
  padding: 0 0.75rem 0 0;
}

.topper-review-list button {
  align-items: flex-start;
  display: grid;
  gap: 0.25rem;
  text-align: left;
}

.topper-review-list span {
  color: #94a3b8;
  font-size: 0.78rem;
}

.topper-page-workspace,
.topper-question-workspace {
  display: grid;
  gap: 0.75rem;
  grid-template-columns: minmax(0, 1.1fr) minmax(22rem, 0.9fr);
  min-width: 0;
}

.topper-page-image,
.topper-ocr-panel,
.topper-question-panel,
.topper-report-panel {
  background: #0a0f18;
  border: 1px solid #253247;
  border-radius: 0.45rem;
  min-width: 0;
  padding: 0.7rem;
}

.topper-image-scroll {
  align-items: flex-start;
  background: #020617;
  border-radius: 0.35rem;
  display: flex;
  justify-content: center;
  margin-top: 0.6rem;
  max-height: 68vh;
  min-height: 32rem;
  overflow: auto;
  padding: 1rem;
}

.topper-image-scroll img {
  max-width: 100%;
  transform-origin: top center;
}

.topper-ocr-panel textarea,
.topper-question-editor,
.topper-report-editor {
  background: #020617;
  border: 1px solid #253247;
  border-radius: 0.35rem;
  color: #e5e7eb;
  font: 0.9rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  min-height: 32rem;
  resize: vertical;
  width: 100%;
}

.topper-question-editor,
.topper-report-editor {
  margin-top: 0.8rem;
}

.topper-question-meta {
  display: grid;
  gap: 0.6rem;
  grid-template-columns: minmax(0, 1fr) minmax(8rem, 12rem);
  margin-top: 0.75rem;
}

.topper-question-meta label {
  color: #94a3b8;
  display: grid;
  gap: 0.3rem;
  font-size: 0.78rem;
}

.topper-question-panel pre,
.topper-report-panel pre {
  color: #e5e7eb;
  font: 0.9rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  margin: 0.8rem 0 0;
  max-height: 68vh;
  overflow: auto;
  white-space: pre-wrap;
}

.topper-page-image.compact .topper-image-scroll {
  min-height: 24rem;
}

@media (max-width: 900px) {
  .topper-review-grid,
  .topper-page-workspace,
  .topper-question-workspace {
    grid-template-columns: 1fr;
  }

  .topper-review-list {
    border-right: 0;
    border-bottom: 1px solid #253247;
    flex-direction: row;
    max-height: none;
    overflow-x: auto;
    padding: 0 0 0.75rem;
  }
}
</style>
