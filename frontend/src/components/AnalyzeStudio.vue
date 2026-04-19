<template>
  <div class="workspace-layout">
    <!-- Sidebar -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>UPSC Analyzer</h1>
        <div class="subtitle">Topper Answer Sheet Dashboard</div>
      </div>

      <div class="status-bar" v-if="status">
        <span class="status-chip pdfs">{{ status.total_pdfs }} PDFs</span>
        <span class="status-chip pages">{{ status.total_pages }} pages</span>
        <span class="status-chip classified">{{ status.classified_pages }} classified</span>
        <span class="status-chip errors" v-if="status.errors && Object.keys(status.errors).length">
          {{ Object.values(status.errors).reduce((a, b) => a + b, 0) }} errors
        </span>
      </div>

      <div class="pdf-list">
        <div
          v-for="pdf in pdfs"
          :key="pdf.id"
          :class="['pdf-item', { active: selectedPdf && selectedPdf.id === pdf.id }]"
          @click="selectPdf(pdf)"
        >
          <span class="icon">📄</span>
          <span class="filename" :title="pdf.filename">{{ pdf.filename }}</span>
          <div class="pdf-progress-dots" v-if="pdf.progress">
            <div 
              v-for="s in ['1','2','3','4','5']" 
              :key="s" 
              :class="['dot', pdf.progress[s]]"
              :title="'Step ' + s + ': ' + pdf.progress[s]"
            ></div>
          </div>
          <span class="page-count" v-if="pdf.page_count">({{ pdf.page_count }}p)</span>
          <button class="delete-btn-mini" @click.stop="handleDeletePdf(pdf)" title="Delete PDF" :disabled="pipelineRunning">🗑️</button>
        </div>
        <div v-if="!pdfs.length" class="pdf-item" style="opacity: 0.4; cursor: default;">
          No PDFs found
        </div>
      </div>

      <div class="sidebar-actions">
        <label class="btn btn-ghost btn-sm upload-btn" :class="{ disabled: pipelineRunning }">
          📤 Upload PDFs
          <input type="file" multiple accept=".pdf" @change="uploadPdfs" hidden :disabled="pipelineRunning" />
        </label>
        <button class="btn btn-ghost btn-sm" @click="retryErrorPages" :disabled="pipelineRunning">🔄 Retry Error Pages</button>
        <button class="btn btn-ghost btn-sm" @click="refreshAll">↻ Refresh</button>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <template v-if="selectedPdf">
        <div class="top-bar">
          <div class="top-bar-title">
            <h2>{{ selectedPdf.filename }}</h2>
            <button class="btn btn-ghost btn-sm btn-danger" @click="handleDeletePdf(selectedPdf)" :disabled="pipelineRunning">
              🗑️ Delete PDF
            </button>
          </div>
          <div class="pdf-status-strip" v-if="selectedPdf.progress">
            <div v-for="step in pipelineSteps" :key="step.id" :class="['status-step', selectedPdf.progress[step.id]]">
              <div class="step-dot"></div>
              <span>{{ step.name }}</span>
            </div>
          </div>
          <div class="tabs">
            <button :class="['tab', { active: activeTab === 'answers' }]" @click="activeTab = 'answers'">
              Questions ({{ answers.length }})
            </button>
            <button :class="['tab', { active: activeTab === 'aggregation' }]" @click="activeTab = 'aggregation'">
              Aggregation
            </button>
            <button :class="['tab', { active: activeTab === 'runner' }]" @click="activeTab = 'runner'" class="runner-btn">
              ▶ Runner
            </button>
          </div>
        </div>

        <div class="content-body">
          <!-- Answers Tab -->
          <div v-if="activeTab === 'answers'" class="answer-list">
            <div v-if="!answers.length" class="empty-state">
              <div class="icon">📝</div>
              <p>No answers segmented yet.<br>Run Step 4 (Segmentation) first.</p>
            </div>
            <div v-for="answer in answers" :key="answer.id" class="answer-card">
              <div class="answer-card-header" @click="toggleAnswer(answer.id)">
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
                    @click="openInspectorByPageNumber(pageNum)"
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
                <div v-else class="no-text" style="margin-top: 12px;">
                  No dimensions analyzed yet.
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
          <!-- Runner Tab -->
          <div v-if="activeTab === 'runner'" class="runner-tab">
            <div class="config-panel">
              <div class="panel-section">
                <h4>Core Parameters</h4>
                <div class="settings-grid">
                  <div class="form-group">
                    <label>Workers</label>
                    <input type="number" v-model.number="runConfig.workers" min="1" max="16" :disabled="pipelineRunning" />
                  </div>
                  <!-- DPI is now forced to high quality (300) -->
                  <div class="form-group span-full" style="grid-column: span 2;">
                    <label>LLM Model ID</label>
                    <input type="text" v-model="runConfig.llm_model" placeholder="Model for vision & reasoning" :disabled="pipelineRunning" />
                  </div>
                  <div class="form-group span-full" style="grid-column: span 2; display: flex; align-items: center; gap: 12px; margin-top: 8px;">
                    <label class="toggle-control" style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none;">
                      <input type="checkbox" :checked="runConfig.allow_reasoning" @change="runConfig.allow_reasoning = $event.target.checked" :disabled="pipelineRunning" style="width: 18px; height: 18px;" />
                      <span style="font-weight: 500; font-size: 0.9em;">Model Reasoning (Master Toggle)</span>
                    </label>
                    <div class="info-tip" title="Master switch for Deep Thinking. If OFF, reasoning is disabled for all steps. If ON, follows per-step settings below.">ⓘ</div>
                  </div>
                </div>
              </div>

              <div class="panel-section">
                <h4>
                  Pipeline Workflow
                  <button class="btn btn-ghost btn-sm btn-danger" @click="confirmResetStep(1)" :disabled="pipelineRunning" style="font-size: 10px; padding: 2px 8px;">
                    Reset All Data
                  </button>
                </h4>
                <div class="form-group">
                  <div class="radio-group" style="display: flex; gap: 16px; margin-bottom: 8px;">
                    <label style="display: flex; align-items: center; gap: 8px;" :style="{ cursor: pipelineRunning ? 'not-allowed' : 'pointer' }">
                      <input type="radio" v-model="runConfig.mode" value="all" :disabled="pipelineRunning" /> End-to-End
                    </label>
                    <label style="display: flex; align-items: center; gap: 8px;" :style="{ cursor: pipelineRunning ? 'not-allowed' : 'pointer' }">
                      <input type="radio" v-model="runConfig.mode" value="custom" :disabled="pipelineRunning" /> Custom Step Selection
                    </label>
                  </div>
                  
                  <div v-if="runConfig.mode === 'custom'" class="steps-list">
                    <div 
                      v-for="step in pipelineSteps" 
                      :key="step.id" 
                      class="step-row"
                      :class="[selectedPdf.progress ? selectedPdf.progress[step.id] : '', { disabled: pipelineRunning }]"
                      @click="!pipelineRunning && toggleStep(step.id)"
                    >
                      <input type="checkbox" :checked="runConfig.target_steps.includes(step.id)" @click.stop :disabled="pipelineRunning" />
                      <span class="step-name">{{ step.id }}: {{ step.fullname }}</span>
                      
                      <!-- Reasoning Toggle (Steps 2-7 only) -->
                      <div 
                        v-if="step.id > 1"
                        class="reasoning-toggle" 
                        :class="{ active: runConfig.stepReasoning[step.id], disabled: !runConfig.allow_reasoning || pipelineRunning }" 
                        @click.stop="toggleStepReasoning(step.id)"
                        :title="runConfig.stepReasoning[step.id] ? 'Reasoning ENABLED for this step' : 'Reasoning DISABLED for this step'"
                      >
                        🧠
                      </div>
                      
                      <button class="reset-step-btn" :class="{ disabled: pipelineRunning }" @click.stop="confirmResetStep(step.id)">↻ Reset</button>
                      <span class="step-badge" v-if="selectedPdf.progress">{{ selectedPdf.progress[step.id] }}</span>
                    </div>
                  </div>
                </div>
              </div>

              <button class="btn btn-primary" @click="startPipeline" :disabled="pipelineRunning" style="height: 48px; font-size: 14px; justify-content: center;">
                {{ pipelineRunning ? 'AI Pipeline Working...' : '▶ Start Execution' }}
              </button>
            </div>

            <div class="console-panel">
              <div class="console-header">
                <h3>Live Execution Logs</h3>
                <div class="console-controls">
                  <label class="autoscroll-toggle">
                    <input type="checkbox" v-model="autoscroll" /> Auto-scroll
                  </label>
                  <button class="clear-btn" @click="logs = []">🗑️ Clear</button>
                </div>
              </div>
              
              <div class="tasks-overlay" v-if="Object.keys(tasks).length > 0">
                <div v-for="task in tasks" :key="task.id" class="task-bar-container">
                  <div class="task-info">
                    <span>{{ task.description }}</span>
                    <span>{{ task.completed.toFixed(0) }} / {{ task.total }}</span>
                  </div>
                  <div class="progress-bar">
                    <div class="fill" :style="{ width: Math.min(100, Math.max(0, (task.completed / task.total) * 100)) + '%' }"></div>
                  </div>
                </div>
              </div>
              
              <div class="terminal" ref="terminal">
                <div v-for="(log, i) in parsedLogs" :key="i" :class="['log-line', log.level]">
                  <span class="log-icon">{{ log.icon }}</span>
                  <span class="log-text">{{ log.text }}</span>
                  <span v-if="log.page" class="log-page-tag">Page {{ log.page }}</span>
                </div>
                <div v-if="!logs.length" class="empty-terminal">
                  _ Waiting for pipeline execution...
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- No PDF Selected -->
      <div v-else class="empty-state">
        <div class="icon">📑</div>
        <p>Select a PDF from the sidebar to view analysis results.</p>
        <p style="font-size: 12px; color: var(--text-muted); margin-top: 4px;">
          Use the 📤 <strong>Upload PDFs</strong> button in the sidebar to add files.
        </p>
      </div>
    </main>

    <!-- Page Inspector -->
    <div v-if="inspectedPage" class="page-inspector-overlay" @click.self="closePageInspector">
      <div class="page-inspector">
        <div class="inspector-header">
          <div class="header-left">
            <h3>Page {{ inspectedPage.page_number }}</h3>
            <span :class="['classification-badge', badgeClass(inspectedPage.classification)]">
              {{ inspectedPage.classification || 'pending' }}
            </span>
          </div>
          <div class="header-center">
            <div class="page-pills">
              <button class="pill-btn" @click="runPageStep(2)" :disabled="pipelineRunning">Redo OCR</button>
              <button class="pill-btn" @click="runPageStep(3)" :disabled="pipelineRunning">Redo Classify</button>
              <button class="pill-btn" @click="runPageStep(4)" :disabled="pipelineRunning">Re-Segment</button>
              <button class="pill-btn" @click="runPageStep(5)" :disabled="pipelineRunning">Re-Analyze</button>
              <button class="pill-btn primary" @click="runPageStep(null)" title="Force all steps for this page" :disabled="pipelineRunning">Process All</button>
            </div>
          </div>
          <div class="header-right">
            <button class="btn btn-ghost btn-sm" @click="prevPage" :disabled="isFirstPage">← Prev</button>
            <button class="btn btn-ghost btn-sm" @click="nextPage" :disabled="isLastPage">Next →</button>
            <button class="btn btn-ghost btn-sm btn-icon" @click="closePageInspector">✕</button>
          </div>
        </div>
        
        <div class="inspector-body">
          <div class="inspector-image-pane">
            <img :src="getImageUrl(inspectedPage)" />
          </div>
          
          <div class="inspector-data-pane">
            <div class="inspector-tabs">
              <button :class="['itab', { active: inspectorTab === 'transcribe' }]" @click="inspectorTab = 'transcribe'">Transcription</button>
              <button :class="['itab', { active: inspectorTab === 'analysis' }]" @click="inspectorTab = 'analysis'">Analysis</button>
            </div>
            
            <div class="inspector-tab-content">
              <!-- Transcription Tab -->
              <div v-if="inspectorTab === 'transcribe'" class="transcription-view">
                <pre v-if="inspectedPage.transcription && !inspectedPage.transcription.startsWith('[TRANSCRIPTION_ERROR')">{{ inspectedPage.transcription }}</pre>
                <div v-else-if="inspectedPage.transcription && inspectedPage.transcription.startsWith('[TRANSCRIPTION_ERROR')" class="error-text">
                  {{ inspectedPage.transcription }}
                </div>
                <div v-else class="empty-state-text">No transcription available.</div>
              </div>
              
              <!-- Analysis Tab -->
              <div v-if="inspectorTab === 'analysis'" class="analysis-view">
                <div v-if="!getAnswersForPage(inspectedPage.id).length" class="empty-state-text">
                  No answer units identified on this page.
                </div>
                <div v-for="ans in getAnswersForPage(inspectedPage.id)" :key="ans.id" class="answer-unit-card">
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


  </div>
</template>

<script>
import {
  fetchStatus,
  fetchPdfs,
  fetchPages,
  fetchAnswers,
  fetchDimensions,
  fetchAggregations,
  resetPipeline,
  runPipeline,
  retryErrors,
  imageUrl,
  deletePdf,
} from '../api.js'

const API_BASE = 'http://127.0.0.1:8765/api'

export default {
  name: 'AnalyzeStudio',
  data() {
    return {
      status: null,
      pdfs: [],
      selectedPdf: null,
      pages: [],
      answers: [],
      answerDimensions: {},
      aggregations: [],
      activeTab: 'answers',
      expandedAnswers: new Set(),
      loading: false,
      
      // Runner State
      runConfig: {
        workers: 4,
        dpi: 300,
        llm_model: 'gemma-4-26b-a4b',
        allow_reasoning: true,
        mode: 'all',
        target_steps: [],
        stepReasoning: {
          2: false, 3: false, 4: false, 5: true, 6: true, 7: true
        }
      },
      pipelineSteps: [
        { id: 1, name: 'Images', fullname: 'PDF → Page Images' },
        { id: 2, name: 'OCR', fullname: 'OCR Transcription' },
        { id: 3, name: 'Classify', fullname: 'Page Classification' },
        { id: 4, name: 'Segment', fullname: 'Answer Segmentation' },
        { id: 5, name: 'Analyze', fullname: 'Dimension Analysis' },
        { id: 6, name: 'Aggregate', fullname: 'Cross-PDF Aggregation' },
        { id: 7, name: 'Report', fullname: 'Report Generation' }
      ],
      // Page Inspector
      inspectedPage: null,
      inspectorTab: 'transcribe',
      
      pipelineRunning: false,
      eventSource: null,
      logs: [],
      tasks: {},
      autoscroll: true,
    }
  },
  computed: {
    isLastPage() {
      if (!this.inspectedPage || !this.pages.length) return true
      return this.pages[this.pages.length - 1].id === this.inspectedPage.id
    },
    parsedLogs() {
      return this.logs.map(rawMsg => {
        let level = 'info';
        let icon = '⚙️';
        let text = rawMsg;
        let page = null;

        if (rawMsg.includes('[SUCCESS]')) {
          level = 'success';
          icon = '✅';
          text = rawMsg.replace('[SUCCESS]', '').trim();
        } else if (rawMsg.includes('[ERROR]')) {
          level = 'error';
          icon = '❌';
          text = rawMsg.replace('[ERROR]', '').trim();
        } else if (rawMsg.includes('[PAGE:')) {
          level = 'page';
          icon = '📄';
          const match = rawMsg.match(/\[PAGE:(\d+)\]/);
          if (match) page = match[1];
        } else if (rawMsg.includes('[SYSTEM]')) {
          level = 'system';
          icon = '🚀';
          text = rawMsg.replace('[SYSTEM]', '').trim();
        }

        return { level, icon, text, page, raw: rawMsg };
      });
    }
  },
  async mounted() {
    await this.refreshAll()
    // Auto-refresh status every 10s
    this._interval = setInterval(() => this.refreshStatus(), 10000)
    this.connectStream()
  },
  beforeUnmount() {
    clearInterval(this._interval)
    if (this.eventSource) this.eventSource.close()
  },
  methods: {
    async refreshAll() {
      await Promise.all([this.refreshStatus(), this.loadPdfs()])
      if (this.selectedPdf) {
        await this.loadPdfData(this.selectedPdf)
      }
    },
    async refreshStatus() {
      try {
        this.status = await fetchStatus()
      } catch (e) {
        console.warn('API not reachable:', e.message)
      }
    },
    async loadPdfs() {
      try {
        this.pdfs = await fetchPdfs()
      } catch (e) {
        this.pdfs = []
      }
    },
    async selectPdf(pdf) {
      this.selectedPdf = pdf
      this.activeTab = 'pages'
      await this.loadPdfData(pdf)
    },
    async loadPdfData(pdf) {
      if (!pdf.page_count) return; // Prevent loading data if PDF has not been analyzed yet.
      this.loading = true
      try {
        const [pages, answers, aggregations] = await Promise.all([
          fetchPages(pdf),
          fetchAnswers(pdf),
          fetchAggregations(),
        ])
        this.pages = pages
        this.answers = answers
        this.aggregations = aggregations

        // Load dimensions for all answers
        this.answerDimensions = {}
        for (const answer of answers) {
          const dims = await fetchDimensions(answer.id)
          this.answerDimensions[answer.id] = dims
        }
      } finally {
        this.loading = false
      }
    },
    async uploadPdfs(event) {
      const files = event.target.files
      if (!files.length) return

      const formData = new FormData()
      for (let i = 0; i < files.length; i++) {
        formData.append('files', files[i])
      }

      try {
        const res = await fetch(`${API_BASE}/analyze/upload`, {
          method: 'POST',
          body: formData
        })
        if (!res.ok) throw new Error("Upload failed")
        alert("Upload successful! Click the Runner tab and hit Start End-to-End Pipeline to process.")
        await this.refreshAll()
      } catch (err) {
        alert(err.message)
      } finally {
        // clear input
        event.target.value = ''
      }
    },
    async toggleAnswer(answerId) {
      if (this.expandedAnswers.has(answerId)) {
        this.expandedAnswers.delete(answerId)
      } else {
        this.expandedAnswers.add(answerId)
        if (!this.answerDimensions[answerId]) {
          this.answerDimensions[answerId] = await fetchDimensions(answerId)
        }
      }
      // Force reactivity
      this.expandedAnswers = new Set(this.expandedAnswers)
    },
    getImageUrl(page) {
      if (!this.selectedPdf) return '';
      return imageUrl(this.selectedPdf.filename, page.page_number)
    },
    openPageInspector(page) {
      this.inspectedPage = page
      this.inspectorTab = 'transcribe'
    },
    closePageInspector() {
      this.inspectedPage = null
    },
    nextPage() {
      const idx = this.pages.findIndex(p => p.id === this.inspectedPage.id)
      if (idx < this.pages.length - 1) this.inspectedPage = this.pages[idx + 1]
    },
    prevPage() {
      const idx = this.pages.findIndex(p => p.id === this.inspectedPage.id)
      if (idx > 0) this.inspectedPage = this.pages[idx - 1]
    },
    getPageNumbers(pageIdsStr) {
      if (!pageIdsStr) return []
      try {
        const ids = JSON.parse(pageIdsStr).map(id => parseInt(id))
        return ids.map(id => {
          const page = this.pages.find(p => p.id === id)
          return page ? page.page_number : null
        }).filter(n => n !== null)
      } catch(e) { return [] }
    },
    openInspectorByPageNumber(pageNum) {
      const page = this.pages.find(p => p.page_number === pageNum)
      if (page) this.openPageInspector(page)
    },
    getAnswersForPage(pageId) {
      return this.answers.filter(a => {
        try {
          const ids = JSON.parse(a.page_ids)
          return ids.includes(parseInt(pageId)) || ids.includes(String(pageId))
        } catch(e) { return false }
      })
    },
    badgeClass(classification) {
      if (!classification) return 'badge-pending'
      const map = {
        answer: 'badge-answer',
        continuation: 'badge-continuation',
        cover: 'badge-cover',
        evaluation: 'badge-evaluation',
        blank: 'badge-blank',
      }
      return map[classification] || 'badge-pending'
    },
    imageUrl: imageUrl,
    formatKey(key) {
      return key.replace(/_/g, ' ').replace(/\b\w/g, c => c.toUpperCase())
    },
    formatValue(value) {
      if (value === null || value === undefined) return '—'
      if (typeof value === 'boolean') return value ? '✓' : '✗'
      if (Array.isArray(value)) return value.length ? value.join(', ') : '—'
      if (typeof value === 'object') return JSON.stringify(value)
      return String(value)
    },
    async retryErrorPages() {
      try {
        const result = await retryErrors()
        alert(`Cleared ${result.cleared} error page(s). Re-run the pipeline to retry.`)
        await this.refreshAll()
      } catch (e) {
        alert('Failed: ' + e.message)
      }
    },
    async confirmResetStep(stepId) {
      if (this.pipelineRunning) return;
      const step = this.pipelineSteps.find(s => s.id === stepId) || { fullname: 'Pipeline' };
      const msg = stepId === 1 
        ? "⚠️ Are you sure you want to RESET THE ENTIRE PIPELINE?\n\nThis will delete ALL page images, transcriptions, classifications, and answer analysis. This action CANNOT be undone."
        : `⚠️ Reset from step "${step.id}: ${step.fullname}"?\n\nThis will also delete ALL data from subsequent steps (cascading). Continue?`;
      
      if (window.confirm(msg)) {
        await this.doReset(stepId);
      }
    },
    async doReset(stepId) {
      try {
        await resetPipeline(stepId)
        alert(`Reset from step ${stepId} successful.`)
        await this.refreshAll()
      } catch (e) {
        alert('Reset failed: ' + e.message)
      }
    },

    // Runner Logic
    async runPageStep(stepId) {
      if (!this.inspectedPage) return
      try {
        this.pipelineRunning = true
        this.logs = []
        this.tasks = {}
        
        const config = {
          ...this.runConfig,
          target_steps: stepId ? [stepId] : null,
          page_id: this.inspectedPage.id
        }
        await runPipeline(config)
        // Refresh page data after a short delay (or wait for SSE)
        setTimeout(() => this.loadPdfData(this.selectedPdf), 2000)
      } catch (err) {
        alert("Execution failed: " + err.message)
        this.pipelineRunning = false
      }
    },
    async handleDeletePdf(pdf) {
      if (!window.confirm(`Permanently delete "${pdf.filename}" and ALL its AI analysis results?`)) {
        return
      }
      try {
        await deletePdf(pdf.filename)
        if (this.selectedPdf && this.selectedPdf.filename === pdf.filename) {
          this.selectedPdf = null
        }
        await this.refreshAll()
      } catch (e) {
        alert('Deletion failed: ' + e.message)
      }
    },
    async startPipeline() {
      try {
        this.pipelineRunning = true
        this.logs = []
        this.tasks = {}
        
        // Prepare config clone
        const finalConfig = {
          workers: this.runConfig.workers,
          dpi: this.runConfig.dpi,
          llm_model: this.runConfig.llm_model,
          allow_reasoning: this.runConfig.allow_reasoning,
          target_steps: this.runConfig.mode === 'all' ? null : this.runConfig.target_steps,
          step_reasoning: this.runConfig.stepReasoning
        }
        
        await runPipeline(finalConfig)
      } catch(e) {
        alert("Could not start pipeline: " + e.message)
        this.pipelineRunning = false
      }
    },
    toggleStep(stepId) {
      const idx = this.runConfig.target_steps.indexOf(stepId)
      if (idx > -1) this.runConfig.target_steps.splice(idx, 1)
      else this.runConfig.target_steps.push(stepId)
      this.runConfig.target_steps.sort((a,b) => a - b)
    },
    toggleStepReasoning(stepId) {
      if (this.pipelineRunning || !this.runConfig.allow_reasoning) return;
      this.runConfig.stepReasoning[stepId] = !this.runConfig.stepReasoning[stepId];
    },
    connectStream() {
      import('../api.js').then(({ createStream }) => {
        this.eventSource = createStream()
        this.eventSource.onmessage = (e) => {
          const data = JSON.parse(e.data)
          
          if (data.type === 'ping') return;
          
          if (data.type === 'status') {
            if (data.status === 'error') {
              this.logs.push(`[SYSTEM ERROR] ${data.message}`)
              this.pipelineRunning = false
            }
            if (data.status === 'completed') {
              this.logs.push(`[SYSTEM] Pipeline execution completed successfully.`)
              this.pipelineRunning = false
              this.refreshAll()
              if (this.selectedPdf) {
                this.loadPdfData(this.selectedPdf)
              }
            }
          }
          else if (data.type === 'task_add') {
            this.tasks[data.task_id] = {
              id: data.task_id,
              description: data.description,
              total: data.total,
              completed: 0
            }
          }
          else if (data.type === 'task_progress') {
            if (this.tasks[data.task_id]) {
              this.tasks[data.task_id].completed = data.completed
              if (this.tasks[data.task_id].completed >= this.tasks[data.task_id].total) {
                // Remove task when 100% complete
                setTimeout(() => delete this.tasks[data.task_id], 1500)
              }
            }
          }
          else if (data.type === 'log') {
            this.logs.push(data.message)
            if (this.logs.length > 500) this.logs.shift()
            
            if (this.autoscroll) {
              this.$nextTick(() => {
                const term = this.$refs.terminal
                if (term) term.scrollTop = term.scrollHeight
              })
            }
          }
        }
      })
    }
  },
}
</script>
