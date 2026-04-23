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
          <div class="answer-question" v-if="answer.question_text" style="font-size: 14px; color: var(--text-primary); font-weight: 500; margin-bottom: 16px; padding-bottom: 16px; border-bottom: 1px solid var(--border);">
            {{ answer.question_text }}
          </div>
          
          <div class="answer-layout" style="display: flex; gap: 24px; align-items: flex-start; flex-wrap: wrap;">
            <div class="answer-main-col" style="flex: 1; min-width: 300px;">
              <div class="answer-text" style="max-height: none; background: var(--bg-input); padding: 16px; border-radius: 8px; border: 1px solid var(--border); margin-bottom: 24px;">{{ answer.raw_text }}</div>
              
              <!-- Dimensions -->
              <div class="dimensions-grid" v-if="answerDimensions[answer.id]?.length" style="display: grid; grid-template-columns: repeat(auto-fill, minmax(300px, 1fr)); gap: 16px;">
                <div 
                  v-for="dim in answerDimensions[answer.id]" 
                  :key="dim.id"
                  class="dim-card"
                  style="background: var(--bg-card); border: 1px solid var(--border); border-radius: 8px; padding: 16px; box-shadow: 0 2px 8px rgba(0,0,0,0.1);"
                >
                  <h4 style="font-size: 13px; font-weight: 700; color: var(--accent); margin-bottom: 12px; border-bottom: 1px solid var(--border); padding-bottom: 8px;">{{ dim.dimension_name }}</h4>
                  <div class="dim-content">
                    <template v-if="typeof dim.result_json === 'object'">
                      <div
                        v-for="(value, key) in dim.result_json"
                        :key="key"
                      >
                        <div v-if="Array.isArray(value)" class="dim-list" style="margin-bottom: 12px;">
                          <strong style="text-transform: uppercase; font-size: 11px; color: var(--text-muted); letter-spacing: 0.05em;">{{ formatKey(String(key)) }}</strong>
                          <div v-for="(item, i) in value" :key="i" class="dim-list-item" style="background: rgba(0,0,0,0.1); padding: 8px 12px; margin-top: 6px; border-radius: 6px; border-left: 3px solid var(--accent);">
                            <template v-if="typeof item === 'object'">
                              <div v-for="(v, k) in item" :key="k" class="dim-field" style="border: none; padding: 3px 0; display: flex; justify-content: space-between;">
                                <span class="label" style="font-size: 11px; color: var(--text-muted);">{{ formatKey(String(k)) }}</span>
                                <span class="value" style="font-size: 12px; font-weight: 500; text-align: right; max-width: 60%;">{{ formatValue(v) }}</span>
                              </div>
                            </template>
                            <template v-else>
                              <div style="font-size: 12px; color: var(--text-primary);">{{ item }}</div>
                            </template>
                          </div>
                        </div>
                        <div v-else class="dim-field" style="display: flex; justify-content: space-between; padding: 6px 0; border-bottom: 1px solid var(--border);">
                          <span class="label" style="font-size: 11px; color: var(--text-muted);">{{ formatKey(String(key)) }}</span>
                          <span class="value" style="font-size: 12px; font-weight: 500; text-align: right; max-width: 60%;">{{ formatValue(value) }}</span>
                        </div>
                      </div>
                    </template>
                    <pre v-else style="font-size: 11px; white-space: pre-wrap;">{{ dim.result_json }}</pre>
                  </div>
                </div>
              </div>
            </div>

            <div class="answer-side-col" v-if="answer.page_ids" style="width: 260px; flex-shrink: 0; display: flex; flex-direction: column; gap: 16px; background: var(--bg-input); padding: 16px; border-radius: 8px; border: 1px solid var(--border);">
              <h4 style="font-size: 12px; font-weight: 700; color: var(--text-muted); text-transform: uppercase; margin: 0; letter-spacing: 0.05em; text-align: center;">Source Pages</h4>
              <div class="answer-page-strip" style="margin: 0; padding: 0; border: none; display: flex; flex-direction: column; gap: 16px; max-height: 800px; overflow-y: auto;">
                <div 
                  v-for="pageId in getPageNumbers(answer.page_ids)" 
                  :key="pageId"
                  class="answer-thumbnail"
                  style="width: 100%; border-radius: 6px; overflow: hidden; border: 1px solid var(--border); cursor: pointer; transition: transform 0.2s, box-shadow 0.2s;"
                  @click="$emit('open-inspector', getPhysicalPage(pageId))"
                >
                  <img v-if="getPhysicalPage(pageId)" :src="imageUrl(selectedPdf.filename, getPhysicalPage(pageId)!)" loading="lazy" style="width: 100%; display: block;" />
                  <span class="thumb-label" style="display: block; padding: 6px; text-align: center; font-size: 11px; font-weight: 600; background: rgba(0,0,0,0.5); color: #fff;">Page {{ getPhysicalPage(pageId) || '...' }}</span>
                </div>
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
