<script setup lang="ts">
import type { StudyQuestionRecord } from "../../types";

const props = defineProps<{
  question: StudyQuestionRecord;
  dimensions: Record<string, string>;
  copiedId: string | null;
  copiedType: "answer" | "qa" | null;
}>();

const emit = defineEmits<{
  copyAnswer: [question: StudyQuestionRecord];
  copyQA: [question: StudyQuestionRecord];
}>();

function renderMarkdown(md: string): string {
  if (!md) return "";
  let html = md.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  html = html.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/__(.*?)__/g, "<strong>$1</strong>");
  html = html.replace(/\*(.*?)\*/g, "<em>$1</em>");
  html = html.replace(/_(.*?)_/g, "<em>$1</em>");

  const resultLines: string[] = [];
  let inList = false;
  for (const line of html.split("\n")) {
    const trimmed = line.trim();
    if (trimmed.startsWith("### ")) {
      if (inList) resultLines.push("</ul>");
      inList = false;
      resultLines.push(`<h3>${trimmed.slice(4)}</h3>`);
      continue;
    }
    if (trimmed.startsWith("## ")) {
      if (inList) resultLines.push("</ul>");
      inList = false;
      resultLines.push(`<h2>${trimmed.slice(3)}</h2>`);
      continue;
    }
    if (trimmed.startsWith("# ")) {
      if (inList) resultLines.push("</ul>");
      inList = false;
      resultLines.push(`<h1>${trimmed.slice(2)}</h1>`);
      continue;
    }
    const listMatch = line.match(/^(\s*)([-*+])\s+(.*)$/);
    if (listMatch) {
      if (!inList) resultLines.push("<ul>");
      inList = true;
      resultLines.push(`<li>${listMatch[3]}</li>`);
      continue;
    }
    if (inList && trimmed === "") {
      resultLines.push("</ul>");
      inList = false;
      continue;
    }
    resultLines.push(trimmed === "" ? "<br/>" : inList ? line : `<p>${line}</p>`);
  }
  if (inList) resultLines.push("</ul>");
  return resultLines.join("\n").replace(/<p><\/p>/g, "").replace(/(<br\/>\s*){2,}/g, "<br/>");
}
</script>

<template>
  <article class="study-question">
    <div class="study-question-header">
      <div class="study-question-info">
        <h3>{{ question.label || `Q.${question.question_no}` }}</h3>
        <p class="study-question-prompt">{{ question.prompt_text || "Prompt not extracted yet." }}</p>
      </div>
      <div class="study-question-actions">
        <span class="study-pill">Pages {{ question.source_pages.join(", ") || "-" }}</span>
        <button type="button" class="study-btn-action" :disabled="!question.answer_text" @click="emit('copyAnswer', question)">
          {{ copiedId === question.id && copiedType === "answer" ? "Copied!" : "Copy Answer" }}
        </button>
        <button type="button" class="study-btn-action secondary" :disabled="!question.answer_text" @click="emit('copyQA', question)">
          {{ copiedId === question.id && copiedType === "qa" ? "Copied!" : "Copy Q&A" }}
        </button>
      </div>
    </div>
    <div v-if="question.answer_text" class="study-question-answer" v-html="renderMarkdown(question.answer_text)"></div>
    <div v-else class="study-question-answer empty">No answer text yet.</div>

    <div v-if="Object.keys(dimensions).length > 0" class="study-question-dimensions">
      <div v-for="(text, key) in dimensions" :key="key" class="study-question-dimension">
        <strong>{{ key }}</strong>
        <span>{{ text }}</span>
      </div>
    </div>
  </article>
</template>
