<script setup lang="ts">
import type { StudyQuestionRecord } from "../../types";

defineProps<{ questions: StudyQuestionRecord[] }>();
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
      <div>
        <h3>{{ question.label || `Q.${question.question_no}` }}</h3>
        <p>{{ question.prompt_text || "Prompt not extracted yet." }}</p>
      </div>
      <span class="study-pill">Pages {{ question.source_pages.join(", ") || "-" }}</span>
      <pre>{{ question.answer_text || "No answer text yet." }}</pre>
    </article>
    <div v-if="!questions.length" class="study-empty">No question split saved yet.</div>
  </section>
</template>
