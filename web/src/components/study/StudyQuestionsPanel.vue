<script setup lang="ts">
import { computed, ref } from "vue";
import type { StudyCopyDetail, StudyQuestionRecord } from "../../types";
import StudyAnalyticsPanel from "./StudyAnalyticsPanel.vue";
import StudyArchiveView from "../../views/StudyArchiveView.vue";

const props = defineProps<{ detail: StudyCopyDetail | null }>();

const rawReviewId = computed(() => {
  const id = props.detail?.copy.id;
  if (!id) return "";
  return id.startsWith("copy-") ? id.slice(5) : id;
});

const copiedId = ref<string | null>(null);
const copiedType = ref<"answer" | "qa" | null>(null);
const showAnalytics = ref(false);
const showDebug = ref(false);

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

function renderMarkdown(md: string): string {
  if (!md) return "";
  
  // Escape HTML to prevent XSS
  let html = md
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

  // Bold (**text** or __text__)
  html = html.replace(/\*\*(.*?)\*\*/g, "<strong>$1</strong>");
  html = html.replace(/__(.*?)__/g, "<strong>$1</strong>");

  // Italic (*text* or _text_)
  html = html.replace(/\*(.*?)\*/g, "<em>$1</em>");
  html = html.replace(/_(.*?)_/g, "<em>$1</em>");

  // Split into lines to parse block elements
  const lines = html.split("\n");
  const resultLines: string[] = [];
  let inList = false;

  for (let line of lines) {
    const trimmed = line.trim();

    // Headers
    if (trimmed.startsWith("### ")) {
      if (inList) { resultLines.push("</ul>"); inList = false; }
      resultLines.push(`<h3>${trimmed.slice(4)}</h3>`);
      continue;
    }
    if (trimmed.startsWith("## ")) {
      if (inList) { resultLines.push("</ul>"); inList = false; }
      resultLines.push(`<h2>${trimmed.slice(3)}</h2>`);
      continue;
    }
    if (trimmed.startsWith("# ")) {
      if (inList) { resultLines.push("</ul>"); inList = false; }
      resultLines.push(`<h1>${trimmed.slice(2)}</h1>`);
      continue;
    }

    // Bullet lists
    const listMatch = line.match(/^(\s*)([-*+])\s+(.*)$/);
    if (listMatch) {
      if (!inList) {
        resultLines.push("<ul>");
        inList = true;
      }
      resultLines.push(`<li>${listMatch[3]}</li>`);
      continue;
    }

    // If we were in a list and this line is not a list item, close the list
    if (inList && trimmed === "") {
      resultLines.push("</ul>");
      inList = false;
      continue;
    }

    // Regular paragraphs
    if (trimmed === "") {
      resultLines.push("<br/>");
    } else {
      if (inList) {
        resultLines.push(line);
      } else {
        resultLines.push(`<p>${line}</p>`);
      }
    }
  }

  if (inList) {
    resultLines.push("</ul>");
  }

  return resultLines.join("\n").replace(/<p><\/p>/g, "").replace(/(<br\/>\s*){2,}/g, "<br/>");
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
        <button type="button" class="study-btn-action secondary" @click="showDebug = !showDebug">
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

      <div class="study-meta-grid">
        <label>
          Candidate
          <input :value="detail.copy.candidate_name" readonly />
        </label>
        <label>
          Paper
          <input :value="detail.copy.paper" readonly />
        </label>
        <label>
          Test code
          <input :value="detail.copy.test_code" readonly />
        </label>
        <label>
          Roll no.
          <input :value="detail.copy.roll_no" readonly />
        </label>
      </div>

      <div class="study-questions-list" v-if="detail.questions.length">
        <article v-for="question in detail.questions" :key="question.id" class="study-question">
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
        <div v-if="question.answer_text" class="study-question-answer" v-html="renderMarkdown(question.answer_text)"></div>
        <div v-else class="study-question-answer empty">No answer text yet.</div>
      </article>
      </div>
      <div v-else class="study-empty">
        <p>No questions split yet for this copy.</p>
      </div>
    </template>
  </section>
</template>
