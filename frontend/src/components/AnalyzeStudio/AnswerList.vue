<script setup lang="ts">
import { ref } from 'vue';

const props = defineProps<{
  activeTab: string;
  answers: any[];
  expandedAnswers: Set<number>;
  answerDimensions: Record<number, any[]>;
  aggregations: any[];
  selectedPdf: any;
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

function imageUrl(pdfFile: string, pageNum: number) {
  const paddedPage = String(pageNum).padStart(4, '0');
  const pdfName = pdfFile.replace(/\.pdf$/i, '');
  return `http://localhost:8765/api/analyze/images/${encodeURIComponent(pdfName)}/page_${paddedPage}.png`;
}

function formatKey(key: string) {
  return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}

function formatValue(value: any) {
  if (value === null || value === undefined) return '—';
  if (typeof value === 'boolean') return value ? '✓' : '✗';
  if (Array.isArray(value)) return value.length ? value.join(', ') : '—';
  if (typeof value === 'object') return JSON.stringify(value);
  return String(value);
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
              v-for="pageNum in getPageNumbers(answer.page_ids)" 
              :key="pageNum"
              class="answer-thumbnail"
              @click="$emit('open-inspector', pageNum)"
            >
              <img :src="imageUrl(selectedPdf.filename, pageNum)" loading="lazy" />
              <span class="thumb-label">Page {{ pageNum }}</span>
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
                    <span class="label">{{ formatKey(key) }}</span>
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
        <template v-if="typeof agg.aggregation_json === 'object' && agg.aggregation_json.patterns">
          <div
            v-for="pattern in agg.aggregation_json.patterns"
            :key="pattern.pattern_name"
            class="pattern-card"
          >
            <h4>{{ pattern.pattern_name }}</h4>
            <div class="desc">{{ pattern.description }}</div>
            <div class="meta">
              Frequency: {{ pattern.frequency }} ({{ pattern.percentage }}%)
              · Template: {{ pattern.reusable_template }}
            </div>
          </div>
        </template>
        <pre v-else style="font-size: 11px; color: var(--text-secondary);">{{ JSON.stringify(agg.aggregation_json, null, 2) }}</pre>
      </div>
    </div>
  </div>
</template>
