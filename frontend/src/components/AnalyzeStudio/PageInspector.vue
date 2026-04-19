<script setup lang="ts">
import { computed } from 'vue';
import { analyzeApi } from '../../api/AnalyzeApiClient';
import { formatKey, formatValue } from '../../utils/format.utils';

const props = defineProps<{
  page: any;
  pages: any[];
  inspectorTab: string;
  isFirstPage: boolean;
  isLastPage: boolean;
  pipelineRunning: boolean;
  answers: any[];
  answerDimensions: Record<number, any[]>;
}>();

const emit = defineEmits<{
  'close': [];
  'prev': [];
  'next': [];
  'update:tab': [tab: string];
  'run-page-step': [stepId: number | null];
}>();

function getImageUrl(page: any) {
  return analyzeApi.imageUrl(page.pdf_file, page.page_number);
}

function getAnswersForPage(pageId: number) {
  return props.answers.filter(a => {
    try {
      const ids = JSON.parse(a.page_ids);
      return ids.includes(pageId) || ids.includes(String(pageId));
    } catch (e) { return false; }
  });
}

function badgeClass(classification: string) {
  if (!classification) return 'badge-pending';
  const map: Record<string, string> = {
    answer: 'badge-answer',
    continuation: 'badge-continuation',
    cover: 'badge-cover',
    evaluation: 'badge-evaluation',
    blank: 'badge-blank',
  };
  return map[classification] || 'badge-pending';
}
</script>

<template>
  <div class="page-inspector-overlay" @click.self="$emit('close')">
    <div class="page-inspector">
      <div class="inspector-header">
        <div class="header-left">
          <h3>Page {{ page.page_number }}</h3>
          <span :class="['classification-badge', badgeClass(page.classification)]">
            {{ page.classification || 'pending' }}
          </span>
        </div>
        <div class="header-center">
          <div class="page-pills">
            <button class="pill-btn" @click="$emit('run-page-step', 2)" :disabled="pipelineRunning">Redo OCR</button>
            <button class="pill-btn" @click="$emit('run-page-step', 3)" :disabled="pipelineRunning">Redo Classify</button>
            <button class="pill-btn" @click="$emit('run-page-step', 4)" :disabled="pipelineRunning">Re-Segment</button>
            <button class="pill-btn" @click="$emit('run-page-step', 5)" :disabled="pipelineRunning">Re-Analyze</button>
            <button class="pill-btn primary" @click="$emit('run-page-step', null)" title="Force all steps" :disabled="pipelineRunning">Process All</button>
          </div>
        </div>
        <div class="header-right">
          <button class="btn btn-ghost btn-sm" @click="$emit('prev')" :disabled="isFirstPage">← Prev</button>
          <button class="btn btn-ghost btn-sm" @click="$emit('next')" :disabled="isLastPage">Next →</button>
          <button class="btn btn-ghost btn-sm btn-icon" @click="$emit('close')">✕</button>
        </div>
      </div>
      
      <div class="inspector-body">
        <div class="inspector-image-pane">
          <img :src="getImageUrl(page)" />
        </div>
        
        <div class="inspector-data-pane">
          <div class="inspector-tabs">
            <button :class="['itab', { active: inspectorTab === 'transcribe' }]" @click="$emit('update:tab', 'transcribe')">Transcription</button>
            <button :class="['itab', { active: inspectorTab === 'analysis' }]" @click="$emit('update:tab', 'analysis')">Analysis</button>
          </div>
          
          <div class="inspector-tab-content">
            <!-- Transcription Tab -->
            <div v-if="inspectorTab === 'transcribe'" class="transcription-view">
              <pre v-if="page.transcription && !page.transcription.startsWith('[TRANSCRIPTION')">{{ page.transcription }}</pre>
              <div v-else-if="page.transcription && page.transcription.startsWith('[TRANSCRIPTION')" class="error-text">
                {{ page.transcription }}
              </div>
              <div v-else class="empty-state-text">No transcription available.</div>
            </div>
            
            <!-- Analysis Tab -->
            <div v-if="inspectorTab === 'analysis'" class="analysis-view">
              <div v-if="!getAnswersForPage(page.id).length" class="empty-state-text">
                No answer units identified on this page.
              </div>
              <div v-for="ans in getAnswersForPage(page.id)" :key="ans.id" class="answer-unit-card">
                <div class="unit-header">
                  <span class="unit-qnum">{{ ans.question_number || 'Unnumbered' }}</span>
                  <span class="unit-range">Pages {{ JSON.parse(ans.page_ids).join(', ') }}</span>
                </div>
                <div class="unit-text highlight">{{ ans.question_text || 'Segmentation result...' }}</div>
                
                <div class="unit-dimensions" v-if="answerDimensions[ans.id]">
                  <div v-for="(result, name) in answerDimensions[ans.id]" :key="name" class="dim-row">
                    <span class="dim-name">{{ formatKey(name) }}</span>
                    <span class="dim-value">{{ formatValue(result) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
