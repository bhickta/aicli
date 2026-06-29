<script setup lang="ts">
import { computed } from "vue";
import { questionMetadata, questionMetadataChips } from "../../lib/studyMetadata";
import type { StudyQuestionRecord } from "../../types";
import StudyMetadataChips from "./StudyMetadataChips.vue";

const props = defineProps<{
  questions: StudyQuestionRecord[];
  activeId: string;
}>();

const emit = defineEmits<{
  select: [id: string];
}>();

const rows = computed(() => props.questions.map((question, index) => {
  const meta = questionMetadata(question);
  return {
    question,
    index: index + 1,
    label: question.label || `Q.${question.question_no || index + 1}`,
    title: question.prompt_text || meta?.topic || "Untitled question",
    topic: meta?.topic || meta?.subject || "",
    chips: questionMetadataChips(question).slice(0, 3),
  };
}));
</script>

<template>
  <nav class="study-question-nav" aria-label="Questions">
    <button
      v-for="row in rows"
      :key="row.question.id"
      type="button"
      class="study-question-nav-item"
      :class="{ active: row.question.id === activeId }"
      @click="emit('select', row.question.id)"
    >
      <span class="study-question-nav-index">{{ row.index }}</span>
      <span class="study-question-nav-main">
        <strong>{{ row.label }}</strong>
        <span>{{ row.title }}</span>
        <small v-if="row.topic">{{ row.topic }}</small>
        <StudyMetadataChips :chips="row.chips" />
      </span>
    </button>
  </nav>
</template>
