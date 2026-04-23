<script setup lang="ts">

import { analyzeApi } from '../../api/AnalyzeApiClient';
import { formatKey, formatValue } from '../../utils/format.utils';

const props = defineProps<{
  activeTab: string;
  answers: any[];
  expandedAnswers: Set<number>;
  answerDimensions: Record<number, any[]>;
  aggregations: any[];
  selectedPdf: any;
  pages: any[];
}>();

const emit = defineEmits<{
  'toggle-answer': [answerId: number];
  'open-inspector': [pageNum: number];
}>();

function getPageNumbers(pageIdsStr: string) {
  if (!pageIdsStr) return [];
  try {
    return JSON.parse(pageIdsStr).map((id: any) => parseInt(id));
  } catch (e) { return []; }
}

function getPhysicalPage(dbId: number) {
  if (!props.pages || props.pages.length === 0) return null;
  const page = props.pages.find(p => p.id == dbId);
  return page ? page.page_number : null;
}

function imageUrl(pdfFile: string, pageNum: number) {
  return analyzeApi.imageUrl(pdfFile, pageNum);
}
</script>

<template>
  <div class="content-body">
    <!-- Answers Tab -->
    <div v-if="activeTab === 'answers'" class="answer-list">
      <div v-if="!answers.length" class="empty-state">
        <div class="icon">📝</div>
        <p>No answers segmented yet.<br>Run Step 4 (Segmentation) first.</p>
      </div>

      <!-- Metadata Header -->
      <div v-if="answers.length && (answers[0].candidate_name || answers[0].upsc_id || answers[0].test_code)" class="candidate-meta-bar">
        <div class="meta-item" v-if="answers[0].candidate_name">
          <span class="label">Candidate</span>
          <span class="value">{{ answers[0].candidate_name }}</span>
        </div>
        <div class="meta-item" v-if="answers[0].test_code">
          <span class="label">Test Code</span>
          <span class="value">{{ answers[0].test_code }}</span>
        </div>
        <div class="meta-item" v-if="answers[0].upsc_id">
          <span class="label">UPSC ID</span>
          <span class="value">{{ answers[0].upsc_id }}</span>
        </div>
      </div>

      <div v-for="answer in answers" :key="answer.id" class="answer-card">
        <div class="answer-card-header" @click="$emit('toggle-answer', answer.id)">
          <span class="answer-q-num">{{ answer.question_number || 'Q?' }}</span>
          <span class="answer-directive">{{ answer.question_directive || '' }}</span>
        </div>
        <div class="answer-card-body" v-if="expandedAnswers.has(answer.id)">
          <div class="answer-question" v-if="answer.question_text">
            {{ answer.question_text }}
          </div>
          <div class="answer-text">{{ answer.raw_text }}</div>

          <div class="answer-page-strip" v-if="answer.page_ids">
            <div 
              v-for="pageId in getPageNumbers(answer.page_ids)" 
              :key="pageId"
              class="answer-thumbnail"
              @click="$emit('open-inspector', getPhysicalPage(pageId))"
            >
              <img v-if="getPhysicalPage(pageId)" :src="imageUrl(selectedPdf.filename, getPhysicalPage(pageId)!)" loading="lazy" />
              <span class="thumb-label">Page {{ getPhysicalPage(pageId) || '...' }}</span>
            </div>
          </div>

          <!-- Dimensions -->
          <div class="dimensions-grid" v-if="answerDimensions[answer.id]?.length">
            <div
              v-for="dim in answerDimensions[answer.id]"
              :key="dim.dimension_name"
              class="dim-card"
            >
              <h4>{{ dim.dimension_name }}</h4>
              <div class="dim-content">
                <template v-if="typeof dim.result_json === 'object'">
                  <div
                    v-for="(value, key) in dim.result_json"
                    :key="key"
                    class="dim-field"
                  >
                    <span class="label">{{ formatKey(String(key)) }}</span>
                    <span class="value">{{ formatValue(value) }}</span>
                  </div>
                </template>
                <pre v-else>{{ dim.result_json }}</pre>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Aggregation Tab -->
    <div v-if="activeTab === 'aggregation'">
      <div v-if="!aggregations.length" class="empty-state">
        <div class="icon">📊</div>
        <p>No aggregation data yet.<br>Run Step 6 (Aggregation) first.</p>
      </div>
      <div v-for="agg in aggregations" :key="agg.dimension_name" class="agg-section">
        <h3>{{ agg.dimension_name }} ({{ agg.answer_count }} answers)</h3>
        <template v-if="typeof agg.aggregation_json === 'object' && (agg.aggregation_json.insights || agg.aggregation_json.patterns)">
          <div
            v-for="insight in (agg.aggregation_json.insights || agg.aggregation_json.patterns)"
            :key="insight.insight_name || insight.pattern_name"
            class="pattern-card"
          >
            <h4>{{ insight.insight_name || insight.pattern_name }}</h4>
            <div class="desc">{{ insight.description }}</div>
            
            <div v-if="insight.examples && insight.examples.length" style="margin-top: 8px; font-size: 13px; color: var(--text-secondary);">
              <strong>Examples:</strong>
              <ul style="margin: 4px 0 0 20px; padding: 0;">
                <li v-for="(ex, i) in insight.examples" :key="i" style="margin-bottom: 4px;">
                  <em>"{{ ex.exact_text }}"</em> <span v-if="ex.why_it_works || ex.what_makes_it_work"> - {{ ex.why_it_works || ex.what_makes_it_work }}</span>
                </li>
              </ul>
            </div>

            <div class="meta" style="margin-top: 8px;" v-if="insight.frequency || insight.reusable_template">
              <span v-if="insight.frequency">Frequency: {{ insight.frequency }} <span v-if="insight.percentage">({{ insight.percentage }}%)</span></span>
              <span v-if="insight.frequency && insight.reusable_template"> · </span>
              <span v-if="insight.reusable_template">Template: {{ insight.reusable_template }}</span>
            </div>
          </div>
        </template>
        <pre v-else style="font-size: 11px; color: var(--text-secondary);">{{ JSON.stringify(agg.aggregation_json, null, 2) }}</pre>
      </div>
    </div>
  </div>
</template>
