<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute, useRouter } from "vue-router";
import type { LocationQueryRaw } from "vue-router";
import type { StudyCopyDetail, StudyQuestionRecord } from "../../types";
import { filterStudyQuestions, questionFilterOptions, type StudyQuestionFilters as StudyQuestionFilterState } from "../../lib/studyMetadata";
import { uploadURLFromPath } from "../../lib/fileLinks";
import StudyAnalyticsPanel from "./StudyAnalyticsPanel.vue";
import StudyArchiveView from "../../views/StudyArchiveView.vue";
import StudyCopyMetadataPanel from "./StudyCopyMetadataPanel.vue";
import StudyQuestionCard from "./StudyQuestionCard.vue";
import StudyQuestionFilters from "./StudyQuestionFilters.vue";
import StudyQuestionNavigator from "./StudyQuestionNavigator.vue";

const props = defineProps<{ detail: StudyCopyDetail | null }>();
const emit = defineEmits<{ synced: [] }>();
const route = useRoute();
const router = useRouter();

const copiedId = ref<string | null>(null);
const copiedType = ref<"answer" | "qa" | null>(null);
const showAnalytics = ref(false);
const showDebug = ref(false);

const rawReviewId = computed(() => {
  const id = props.detail?.copy.id || "";
  return id.startsWith("copy-") ? id.slice(5) : id;
});
const sourcePDFURL = computed(() => uploadURLFromPath(props.detail?.copy.source_path || ""));
const filters = computed<StudyQuestionFilterState>(() => ({
  query: routeValue("q"),
  subject: routeValue("subject"),
  topic: routeValue("topic"),
  paper: routeValue("paper"),
  difficulty: routeValue("difficulty"),
}));
const filterOptions = computed(() => questionFilterOptions(props.detail?.questions || []));
const visibleQuestions = computed(() => filterStudyQuestions(props.detail?.questions || [], filters.value));
const activeQuestionId = computed(() => routeValue("question"));
const activeQuestion = computed(() =>
  visibleQuestions.value.find((question) => question.id === activeQuestionId.value) || visibleQuestions.value[0] || null,
);
const activeQuestionIndex = computed(() => visibleQuestions.value.findIndex((question) => question.id === activeQuestion.value?.id));
const canGoPrevious = computed(() => activeQuestionIndex.value > 0);
const canGoNext = computed(() => activeQuestionIndex.value >= 0 && activeQuestionIndex.value < visibleQuestions.value.length - 1);
const activeQuestionCountLabel = computed(() => {
  if (!activeQuestion.value || activeQuestionIndex.value < 0) return "No question selected";
  return `Question ${activeQuestionIndex.value + 1} of ${visibleQuestions.value.length}`;
});

function updateFilters(update: Partial<StudyQuestionFilterState>) {
  const query: LocationQueryRaw = { ...route.query };
  const keyMap: Record<keyof StudyQuestionFilterState, string> = {
    query: "q",
    subject: "subject",
    topic: "topic",
    paper: "paper",
    difficulty: "difficulty",
  };
  for (const [key, value] of Object.entries(update) as Array<[keyof StudyQuestionFilterState, string]>) {
    const routeKey = keyMap[key];
    if (value?.trim()) query[routeKey] = value.trim();
    else delete query[routeKey];
  }
  delete query.question;
  router.replace({ query });
}

function clearFilters() {
  updateFilters({ query: "", subject: "", topic: "", paper: "", difficulty: "" });
}

function copyAnswer(question: StudyQuestionRecord) {
  copyText(question.answer_text, question.id, "answer");
}

function copyQA(question: StudyQuestionRecord) {
  const label = question.label || `Q.${question.question_no}`;
  copyText(`${label}: ${question.prompt_text || ""}\n\nAnswer:\n${question.answer_text || ""}`, question.id, "qa");
}

function selectQuestion(id: string) {
  const query: LocationQueryRaw = { ...route.query, question: id };
  router.replace({ query });
}

function moveQuestion(delta: number) {
  const next = visibleQuestions.value[activeQuestionIndex.value + delta];
  if (next) selectQuestion(next.id);
}

function copyText(text: string, id: string, type: "answer" | "qa") {
  if (!text) return;
  navigator.clipboard.writeText(text).then(() => {
    copiedId.value = id;
    copiedType.value = type;
    setTimeout(() => {
      if (copiedId.value === id && copiedType.value === type) {
        copiedId.value = null;
        copiedType.value = null;
      }
    }, 2000);
  });
}

function getQuestionDimensions(questionId: string) {
  const dims: Record<string, string> = {};
  for (const analysis of props.detail?.analyses || []) {
    if (analysis.scope_type !== "question" || analysis.scope_id !== questionId) continue;
    try {
      const payload = JSON.parse(analysis.result_json);
      dims[analysis.dimension_key] = payload.analysis || analysis.result_json;
    } catch {
      dims[analysis.dimension_key] = analysis.result_json;
    }
  }
  return dims;
}

function routeValue(key: string): string {
  const value = route.query[key];
  return Array.isArray(value) ? String(value[0] || "") : String(value || "");
}
</script>

<template>
  <section class="study-card study-questions">
    <div v-if="!detail" class="study-empty">
      <p>Select a topper copy from the sidebar to view its metadata, questions, and answers.</p>
    </div>
    <template v-else>
      <header class="study-card-header">
        <div>
          <h2>{{ detail.copy.pdf_name || detail.copy.id }}</h2>
          <p>{{ detail.copy.source_path || "Question-wise answer text and source page mapping." }}</p>
        </div>
        <div class="study-question-actions">
          <a v-if="sourcePDFURL" class="study-btn-action secondary" :href="sourcePDFURL" target="_blank" rel="noopener">Open PDF</a>
          <span class="study-pill">{{ detail.copy.status || "pending" }}</span>
        </div>
      </header>

      <div class="study-kpis">
        <span><strong>{{ detail.pages.length }}</strong> pages</span>
        <span><strong>{{ detail.questions.length }}</strong> questions</span>
        <span><strong>{{ detail.copy.unclear_count }}</strong> unclear</span>
        <span><strong>{{ detail.analyses.length }}</strong> analyses</span>
      </div>

      <div class="study-panel-actions">
        <button type="button" class="study-btn-action secondary" @click="showAnalytics = !showAnalytics">Analytics</button>
        <button v-if="rawReviewId.startsWith('topper-')" type="button" class="study-btn-action secondary" @click="showDebug = !showDebug">
          Advanced Actions / Debug
        </button>
      </div>

      <StudyAnalyticsPanel v-if="showAnalytics" :detail="detail" class="study-embedded-panel" />
      <StudyArchiveView v-if="showDebug" :review-id="rawReviewId" class="study-embedded-panel" />
      <StudyCopyMetadataPanel :copy="detail.copy" @saved="emit('synced')" />

      <template v-if="detail.questions.length">
        <StudyQuestionFilters
          :filters="filters"
          :options="filterOptions"
          :total="detail.questions.length"
          :visible="visibleQuestions.length"
          @update="updateFilters"
          @clear="clearFilters"
        />
        <div v-if="activeQuestion" class="study-question-reader">
          <aside class="study-question-reader-rail">
            <div class="study-question-reader-summary">
              <strong>{{ activeQuestionCountLabel }}</strong>
              <span>{{ visibleQuestions.length }} matching questions</span>
            </div>
            <StudyQuestionNavigator
              :questions="visibleQuestions"
              :active-id="activeQuestion.id"
              @select="selectQuestion"
            />
          </aside>
          <div class="study-question-reader-main">
            <div class="study-question-reader-controls">
              <button type="button" class="study-btn-action secondary" :disabled="!canGoPrevious" @click="moveQuestion(-1)">Previous</button>
              <span>{{ activeQuestionCountLabel }}</span>
              <button type="button" class="study-btn-action secondary" :disabled="!canGoNext" @click="moveQuestion(1)">Next</button>
            </div>
            <StudyQuestionCard
              :question="activeQuestion"
              :dimensions="getQuestionDimensions(activeQuestion.id)"
              :copied-id="copiedId"
              :copied-type="copiedType"
              @copy-answer="copyAnswer"
              @copy-q-a="copyQA"
            />
          </div>
        </div>
        <div v-else class="study-empty"><p>No questions match these filters.</p></div>
      </template>
      <div v-else class="study-empty"><p>No questions split yet for this copy.</p></div>
    </template>
  </section>
</template>
