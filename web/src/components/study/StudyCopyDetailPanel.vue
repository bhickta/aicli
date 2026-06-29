<script setup lang="ts">
import type { StudyCopyDetail } from "../../types";

defineProps<{ detail: StudyCopyDetail | null }>();
</script>

<template>
  <section class="study-card study-detail">
    <div v-if="!detail" class="study-empty">Select a copy to inspect pages, OCR, and questions.</div>
    <template v-else>
      <header class="study-card-header">
        <div>
          <h2>{{ detail.copy.pdf_name || detail.copy.id }}</h2>
          <p>{{ detail.copy.source_path }}</p>
        </div>
        <span class="study-pill">{{ detail.copy.status || "pending" }}</span>
      </header>

      <div class="study-kpis">
        <span><strong>{{ detail.pages.length }}</strong> pages</span>
        <span><strong>{{ detail.questions.length }}</strong> questions</span>
        <span><strong>{{ detail.copy.unclear_count }}</strong> unclear</span>
        <span><strong>{{ detail.analyses.length }}</strong> analyses</span>
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

      <div class="study-pages-strip">
        <article v-for="page in detail.pages" :key="page.page_number" class="study-page-chip">
          <strong>Page {{ page.page_number }}</strong>
          <span>{{ page.status || "pending" }}</span>
          <small>{{ page.unclear_count }} unclear</small>
        </article>
      </div>
    </template>
  </section>
</template>
