<script setup lang="ts">
import { ref, watch, computed } from "vue";
import type { StudyCopyDetail, StudyQuestionRecord } from "../../types";
import StudyAnalyticsPanel from "./StudyAnalyticsPanel.vue";
import StudyArchiveView from "../../views/StudyArchiveView.vue";
import StudyQuestionCard from "./StudyQuestionCard.vue";
import { api } from "../../lib/api";
import { useToasts } from "../../composables/useToasts";

const props = defineProps<{ detail: StudyCopyDetail | null }>();
const toasts = useToasts();

const rawReviewId = computed(() => {
  const id = props.detail?.copy.id;
  if (!id) return "";
  return id.startsWith("copy-") ? id.slice(5) : id;
});

const copiedId = ref<string | null>(null);
const copiedType = ref<"answer" | "qa" | null>(null);
const showAnalytics = ref(false);
const showDebug = ref(false);

const isEditingMetadata = ref(false);
const metadataForm = ref({
  pdf_name: "",
  candidate_name: "",
  paper: "",
  test_code: "",
  roll_no: "",
});

watch(
  () => props.detail?.copy,
  (newCopy) => {
    if (newCopy) {
      metadataForm.value = {
        pdf_name: newCopy.pdf_name || "",
        candidate_name: newCopy.candidate_name || "",
        paper: newCopy.paper || "",
        test_code: newCopy.test_code || "",
        roll_no: newCopy.roll_no || "",
      };
      isEditingMetadata.value = false;
    }
  },
  { immediate: true }
);

async function saveMetadata() {
  if (!props.detail?.copy) return;
  try {
    const payload = await api<{ copy: any }>(`/api/study/copies/${encodeURIComponent(props.detail.copy.id)}`, {
      method: "PUT",
      body: JSON.stringify({
        pdf_name: metadataForm.value.pdf_name,
        candidate_name: metadataForm.value.candidate_name,
        paper: metadataForm.value.paper,
        test_code: metadataForm.value.test_code,
        roll_no: metadataForm.value.roll_no,
      })
    });
    Object.assign(props.detail.copy, payload.copy);
    isEditingMetadata.value = false;
    toasts.success("Metadata Saved", "Successfully renamed copy details.");
  } catch (e) {
    toasts.error("Failed to save", e instanceof Error ? e.message : String(e));
  }
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
  }).catch(err => {
    console.error("Failed to copy text: ", err);
  });
}

function copyQA(q: StudyQuestionRecord) {
  const label = q.label || `Q.${q.question_no}`;
  const prompt = q.prompt_text || "";
  const answer = q.answer_text || "";
  
  const textToCopy = `${label}: ${prompt}\n\nAnswer:\n${answer}`;
  copyText(textToCopy, q.id, "qa");
}

function getQuestionDimensions(questionId: string) {
  if (!props.detail?.analyses) return {};
  const dims: Record<string, string> = {};
  
  for (const analysis of props.detail.analyses) {
    if (analysis.scope_type === "question" && analysis.scope_id === questionId) {
      try {
        const payload = JSON.parse(analysis.result_json);
        dims[analysis.dimension_key] = payload.analysis || analysis.result_json;
      } catch (e) {
        dims[analysis.dimension_key] = analysis.result_json;
      }
    }
  }
  return dims;
}
</script>

<template>
  <section class="study-card study-questions">
    <div v-if="!detail" class="study-empty">
      <svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="empty-icon"><path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"></path><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"></path></svg>
      <p>Select a topper copy from the sidebar to view its metadata, questions, and answers.</p>
    </div>
    <template v-else>
      <header class="study-card-header">
        <div>
          <h2>{{ detail.copy.pdf_name || detail.copy.id }}</h2>
          <p>{{ detail.copy.source_path || "Question-wise answer text and source page mapping." }}</p>
        </div>
        <span class="study-pill">{{ detail.copy.status || "pending" }}</span>
      </header>
      
      <div class="study-kpis">
        <span><strong>{{ detail.pages.length }}</strong> pages</span>
        <span><strong>{{ detail.questions.length }}</strong> questions</span>
        <span><strong>{{ detail.copy.unclear_count }}</strong> unclear</span>
        <span><strong>{{ detail.analyses.length }}</strong> analyses</span>
      </div>

      <div style="margin-top: 16px; margin-bottom: 24px; display: flex; gap: 12px;">
        <button type="button" class="study-btn-action secondary" @click="showAnalytics = !showAnalytics">
          <svg v-if="showAnalytics" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"></polyline></svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"></polyline></svg>
          Analytics
        </button>
        <button v-if="rawReviewId.startsWith('topper-')" type="button" class="study-btn-action secondary" @click="showDebug = !showDebug">
          <svg v-if="showDebug" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="18 15 12 9 6 15"></polyline></svg>
          <svg v-else xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="6 9 12 15 18 9"></polyline></svg>
          Advanced Actions / Debug
        </button>
      </div>

      <div v-if="showAnalytics" style="margin-bottom: 24px; border: 1px solid rgba(255, 255, 255, 0.05); border-radius: 8px; overflow: hidden;">
        <StudyAnalyticsPanel :detail="detail" />
      </div>

      <div v-if="showDebug" style="margin-bottom: 24px; border: 1px solid rgba(255, 255, 255, 0.05); border-radius: 8px; overflow: hidden; background: rgba(0,0,0,0.2);">
        <StudyArchiveView :review-id="rawReviewId" />
      </div>

      <div class="study-meta-grid" style="position: relative;">
        <div style="position: absolute; right: 0; top: -35px;">
          <button v-if="!isEditingMetadata" type="button" class="study-btn-action" @click="isEditingMetadata = true">
             <svg xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"></path><path d="M16.5 3.5a2.121 2.121 0 0 1 3 3L7 19l-4 1 1-4L16.5 3.5z"></path></svg>
             Edit Details
          </button>
          <button v-else type="button" class="study-btn-action primary" @click="saveMetadata">
             Save Details
          </button>
        </div>

        <label>
          PDF / Copy Name
          <input v-model="metadataForm.pdf_name" :readonly="!isEditingMetadata" :style="isEditingMetadata ? 'border-bottom: 1px solid var(--accent); border-radius: 0; background: rgba(255,255,255,0.05); padding-left: 4px;' : 'border: none; background: transparent; padding-left: 0;'" />
        </label>
        <label>
          Candidate
          <input v-model="metadataForm.candidate_name" :readonly="!isEditingMetadata" :style="isEditingMetadata ? 'border-bottom: 1px solid var(--accent); border-radius: 0; background: rgba(255,255,255,0.05); padding-left: 4px;' : 'border: none; background: transparent; padding-left: 0;'" />
        </label>
        <label>
          Paper
          <input v-model="metadataForm.paper" :readonly="!isEditingMetadata" :style="isEditingMetadata ? 'border-bottom: 1px solid var(--accent); border-radius: 0; background: rgba(255,255,255,0.05); padding-left: 4px;' : 'border: none; background: transparent; padding-left: 0;'" />
        </label>
        <label>
          Test code
          <input v-model="metadataForm.test_code" :readonly="!isEditingMetadata" :style="isEditingMetadata ? 'border-bottom: 1px solid var(--accent); border-radius: 0; background: rgba(255,255,255,0.05); padding-left: 4px;' : 'border: none; background: transparent; padding-left: 0;'" />
        </label>
        <label>
          Roll no.
          <input v-model="metadataForm.roll_no" :readonly="!isEditingMetadata" :style="isEditingMetadata ? 'border-bottom: 1px solid var(--accent); border-radius: 0; background: rgba(255,255,255,0.05); padding-left: 4px;' : 'border: none; background: transparent; padding-left: 0;'" />
        </label>
      </div>

      <div class="study-questions-list" v-if="detail.questions.length">
        <StudyQuestionCard
          v-for="question in detail.questions"
          :key="question.id"
          :question="question"
          :dimensions="getQuestionDimensions(question.id)"
          :copied-id="copiedId"
          :copied-type="copiedType"
          @copy-answer="copyText(question.answer_text, question.id, 'answer')"
          @copy-q-a="copyQA"
        />
      </div>
      <div v-else class="study-empty">
        <p>No questions split yet for this copy.</p>
      </div>
    </template>
  </section>
</template>
