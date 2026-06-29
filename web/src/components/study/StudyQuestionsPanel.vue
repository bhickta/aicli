<script setup lang="ts">
import { ref } from "vue";
import type { StudyQuestionRecord } from "../../types";

defineProps<{ questions: StudyQuestionRecord[] }>();

const copiedId = ref<string | null>(null);
const copiedType = ref<"answer" | "qa" | null>(null);

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
</script>

<template>
  <section class="study-card study-questions">
    <header class="study-card-header">
      <div>
        <h2>Questions</h2>
        <p>Question-wise answer text and source page mapping.</p>
      </div>
    </header>
    <article v-for="question in questions" :key="question.id" class="study-question">
      <div class="study-question-header">
        <div class="study-question-info">
          <h3>{{ question.label || `Q.${question.question_no}` }}</h3>
          <p class="study-question-prompt">{{ question.prompt_text || "Prompt not extracted yet." }}</p>
        </div>
        <div class="study-question-actions">
          <span class="study-pill">Pages {{ question.source_pages.join(", ") || "-" }}</span>
          <button
            type="button"
            class="study-btn-action"
            :disabled="!question.answer_text"
            @click="copyText(question.answer_text, question.id, 'answer')"
          >
            <svg v-if="copiedId === question.id && copiedType === 'answer'" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
            {{ copiedId === question.id && copiedType === 'answer' ? 'Copied!' : 'Copy Answer' }}
          </button>
          <button
            type="button"
            class="study-btn-action secondary"
            :disabled="!question.answer_text"
            @click="copyQA(question)"
          >
            <svg v-if="copiedId === question.id && copiedType === 'qa'" xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path></svg>
            {{ copiedId === question.id && copiedType === 'qa' ? 'Copied!' : 'Copy Q&A' }}
          </button>
        </div>
      </div>
      <pre>{{ question.answer_text || "No answer text yet." }}</pre>
    </article>
    <div v-if="!questions.length" class="study-empty">No question split saved yet.</div>
  </section>
</template>
