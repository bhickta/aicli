<script setup lang="ts">
import { ref, watch } from 'vue';
import { useAnalyzeStatus } from '../composables/useAnalyzeStatus';
import { useAnalyzePipeline } from '../composables/useAnalyzePipeline';
import { usePageInspector } from '../composables/usePageInspector';
import { 
  fetchPages, 
  fetchAnswers, 
  fetchAggregations, 
  fetchDimensions, 
  retryErrors, 
  resetPipeline,
  deletePdf,
  runPipeline 
} from '../api.js';

import AnalyzeSidebar from './AnalyzeStudio/AnalyzeSidebar.vue';
import AnalyzeBanner from './AnalyzeStudio/AnalyzeBanner.vue';
import AnswerList from './AnalyzeStudio/AnswerList.vue';
import PipelineRunner from './AnalyzeStudio/PipelineRunner.vue';
import PageInspector from './AnalyzeStudio/PageInspector.vue';

// 1. Status & PDF List logic
const { status, pdfs, refreshAll, refreshStatus, loadPdfs } = useAnalyzeStatus();

// 2. State
const selectedPdf = ref<any>(null);
const pages = ref<any[]>([]);
const answers = ref<any[]>([]);
const answerDimensions = ref<Record<number, any[]>>({});
const aggregations = ref<any[]>([]);
const activeTab = ref('answers');
const expandedAnswers = ref(new Set<number>());
const loading = ref(false);

// 3. Composables
const { 
  pipelineRunning, 
  parsedLogs, 
  tasks, 
  autoscroll, 
  startPipeline, 
  logs 
} = useAnalyzePipeline(() => {
  refreshAll();
  if (selectedPdf.value) loadPdfData(selectedPdf.value);
});

const { 
  inspectedPage, 
  inspectorTab, 
  isFirstPage, 
  isLastPage, 
  openPageInspector, 
  closePageInspector, 
  nextPage, 
  prevPage 
} = usePageInspector(pages.value);

// 4. Methods
async function selectPdf(pdf: any) {
  selectedPdf.value = pdf;
  activeTab.value = 'answers';
  await loadPdfData(pdf);
}

async function loadPdfData(pdf: any) {
  if (!pdf.page_count) return;
  loading.value = true;
  try {
    const [p, a, agg] = await Promise.all([
      fetchPages(pdf),
      fetchAnswers(pdf),
      fetchAggregations(),
    ]);
    pages.value = p;
    answers.value = a;
    aggregations.value = agg;

    // Load dimensions for expanded
    for (const answerId of expandedAnswers.value) {
      if (!answerDimensions.value[answerId]) {
        answerDimensions.value[answerId] = await fetchDimensions(answerId);
      }
    }
  } finally {
    loading.value = false;
  }
}

async function toggleAnswer(answerId: number) {
  if (expandedAnswers.value.has(answerId)) {
    expandedAnswers.value.delete(answerId);
  } else {
    expandedAnswers.value.add(answerId);
    if (!answerDimensions.value[answerId]) {
      answerDimensions.value[answerId] = await fetchDimensions(answerId);
    }
  }
  expandedAnswers.value = new Set(expandedAnswers.value);
}

async function uploadPdfs(event: any) {
  const files = event.target.files;
  if (!files.length) return;
  const formData = new FormData();
  for (let i = 0; i < files.length; i++) formData.append('files', files[i]);

  try {
    const res = await fetch('http://localhost:8765/api/analyze/upload', {
      method: 'POST',
      body: formData
    });
    if (!res.ok) throw new Error("Upload failed");
    alert("Upload successful! Go to Runner tab to process.");
    await refreshAll();
  } catch (err: any) {
    alert(err.message);
  } finally {
    event.target.value = '';
  }
}

async function handleRetryErrors() {
  try {
    const result = await retryErrors();
    alert(`Cleared ${result.cleared} errors. Re-run pipeline to retry.`);
    await refreshAll();
  } catch (e: any) {
    alert('Failed: ' + e.message);
  }
}

async function handleResetStep(stepId: number) {
  if (pipelineRunning.value) return;
  if (window.confirm(`⚠️ Reset from step ${stepId}? This will delete subsequent data.`)) {
    try {
      await resetPipeline(stepId);
      alert('Reset successful.');
      await refreshAll();
      if (selectedPdf.value) loadPdfData(selectedPdf.value);
    } catch (e: any) {
      alert('Reset failed: ' + e.message);
    }
  }
}

async function handleDeletePdf(pdf: any) {
  if (!window.confirm(`Permanently delete "${pdf.filename}"?`)) return;
  try {
    await deletePdf(pdf.filename);
    if (selectedPdf.value && selectedPdf.value.filename === pdf.filename) {
      selectedPdf.value = null;
    }
    await refreshAll();
  } catch (e: any) {
    alert('Deletion failed: ' + e.message);
  }
}

async function runPageStep(stepId: number | null) {
  if (!inspectedPage.value) return;
  try {
    pipelineRunning.value = true;
    logs.value = [];
    tasks.value = {};
    const config = {
      target_steps: stepId ? [stepId] : null,
      page_id: inspectedPage.value.id
    };
    await runPipeline(config);
    setTimeout(() => { if (selectedPdf.value) loadPdfData(selectedPdf.value); }, 2000);
  } catch (err: any) {
    alert("Execution failed: " + err.message);
    pipelineRunning.value = false;
  }
}

function openInspectorByPageNum(pageNum: number) {
  const page = pages.value.find(p => p.page_number === pageNum);
  if (page) openPageInspector(page);
}

// Watchers
watch(pages, (newPages) => {
  // Update the pages list in the inspector composable if needed
  // This is handled by reactivity since we pass pages.value or the ref itself if we refactored the composable to take a ref
});
</script>

<template>
  <div class="workspace-layout">
    <AnalyzeSidebar 
      :pdfs="pdfs"
      :selected-pdf="selectedPdf"
      :status="status"
      :pipeline-running="pipelineRunning"
      @select-pdf="selectPdf"
      @delete-pdf="handleDeletePdf"
      @upload-pdfs="uploadPdfs"
      @retry-errors="handleRetryErrors"
      @refresh="refreshAll"
    />

    <main class="main-content">
      <template v-if="selectedPdf">
        <AnalyzeBanner 
          :selected-pdf="selectedPdf"
          v-model:active-tab="activeTab"
          :pipeline-running="pipelineRunning"
          :answers-count="answers.length"
          @delete-pdf="handleDeletePdf"
        />

        <AnswerList 
          v-if="activeTab !== 'runner'"
          :active-tab="activeTab"
          :answers="answers"
          :expanded-answers="expandedAnswers"
          :answer-dimensions="answerDimensions"
          :aggregations="aggregations"
          :selected-pdf="selectedPdf"
          @toggle-answer="toggleAnswer"
          @open-inspector="openInspectorByPageNum"
        />

        <PipelineRunner 
          v-else
          :pipeline-running="pipelineRunning"
          :selected-pdf="selectedPdf"
          :parsed-logs="parsedLogs"
          :tasks="tasks"
          v-model:autoscroll="autoscroll"
          @clear-logs="logs = []"
          @start-pipeline="startPipeline"
          @reset-step="handleResetStep"
        />
      </template>

      <div v-else class="empty-state">
        <div class="icon">📑</div>
        <p>Select a PDF from the sidebar to view analysis results.</p>
      </div>
    </main>

    <PageInspector 
      v-if="inspectedPage"
      :page="inspectedPage"
      :pages="pages"
      v-model:tab="inspectorTab"
      :is-first-page="isFirstPage"
      :is-last-page="isLastPage"
      :pipeline-running="pipelineRunning"
      :answers="answers"
      :answer-dimensions="answerDimensions"
      @close="closePageInspector"
      @prev="prevPage"
      @next="nextPage"
      @run-page-step="runPageStep"
    />
  </div>
</template>
