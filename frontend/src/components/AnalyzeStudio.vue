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
          {{ pdf.filename }} ({{ pdf.page_count }}p)
        </div>
        <div v-if="!pdfs.length" class="pdf-item" style="opacity: 0.4; cursor: default;">
          No PDFs found
        </div>
      </div>

      <div class="sidebar-actions">
        <button class="btn btn-ghost btn-sm" @click="retryErrorPages">🔄 Retry Error Pages</button>
        <button class="btn btn-ghost btn-sm" @click="showResetModal = true">⚠️ Reset Pipeline</button>
        <button class="btn btn-ghost btn-sm" @click="refreshAll">↻ Refresh</button>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <template v-if="selectedPdf">
        <div class="top-bar">
          <h2>{{ selectedPdf.filename }}</h2>
          <div class="tabs">
            <button :class="['tab', { active: activeTab === 'pages' }]" @click="activeTab = 'pages'">
              Pages ({{ pages.length }})
            </button>
            <button :class="['tab', { active: activeTab === 'answers' }]" @click="activeTab = 'answers'">
              Answers ({{ answers.length }})
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
          <!-- Pages Tab -->
          <div v-if="activeTab === 'pages'" class="page-grid">
            <div v-for="page in pages" :key="page.id" class="page-card">
              <div class="page-card-header">
                <span class="page-number">Page {{ page.page_number }}</span>
                <span :class="['classification-badge', badgeClass(page.classification)]">
                  {{ page.classification || 'pending' }}
                </span>
              </div>
              <div class="page-card-body">
                <div class="page-image-col">
                  <img
                    :src="getImageUrl(page)"
                    :alt="'Page ' + page.page_number"
                    @click="openLightbox(page)"
                    loading="lazy"
                  />
                </div>
                <div class="page-text-col">
                  <pre v-if="page.transcription && !page.transcription.startsWith('[TRANSCRIPTION_ERROR')">{{ page.transcription }}</pre>
                  <div v-else-if="page.transcription && page.transcription.startsWith('[TRANSCRIPTION_ERROR')" class="no-text" style="color: var(--danger);">
                    ✖ {{ page.transcription }}
                  </div>
                  <div v-else class="no-text">Not yet transcribed</div>
                </div>
              </div>
            </div>
          </div>

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
              <h3>Pipeline Configuration</h3>
              <div class="form-group">
                <label>Workers</label>
                <input type="number" v-model.number="runConfig.workers" min="1" max="16" />
              </div>
              <div class="form-group">
                <label>DPI</label>
                <input type="number" v-model.number="runConfig.dpi" min="50" max="600" />
              </div>
              <div class="form-group">
                <label>LLM Model ID</label>
                <input type="text" v-model="runConfig.llm_model" placeholder="e.g. gemma-4-26b-a4b" />
              </div>
              <button class="btn btn-primary" @click="startPipeline" :disabled="pipelineRunning">
                {{ pipelineRunning ? 'Pipeline Running...' : '▶ Start End-to-End Pipeline' }}
              </button>
            </div>

            <div class="console-panel">
              <div class="console-header">
                <h3>Live Execution Logs</h3>
                <span v-if="pipelineRunning" class="pulse"></span>
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
                <div v-for="(log, i) in logs" :key="i" class="log-line">{{ log }}</div>
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
          Start the API: <code style="color: var(--accent);">aicli analyze view -d ./data/toppers_2022/</code>
        </p>
      </div>
    </main>

    <!-- Lightbox -->
    <div v-if="lightboxImage" class="lightbox-overlay" @click="lightboxImage = null">
      <img :src="lightboxImage" />
    </div>

    <!-- Reset Modal -->
    <div v-if="showResetModal" class="lightbox-overlay" @click.self="showResetModal = false">
      <div style="background: var(--bg-card); border: 1px solid var(--border); border-radius: var(--radius-lg); padding: 24px; max-width: 400px; width: 90%;" @click.stop>
        <h3 style="margin-bottom: 16px; font-size: 15px;">Reset Pipeline</h3>
        <p style="font-size: 12px; color: var(--text-secondary); margin-bottom: 16px;">
          Select a step to reset from. All data from that step onwards will be cleared.
        </p>
        <select v-model="resetStep" style="width: 100%; padding: 8px; background: var(--bg-input); color: var(--text-primary); border: 1px solid var(--border); border-radius: var(--radius); font-family: inherit; font-size: 13px; margin-bottom: 16px;">
          <option :value="2">Step 2: OCR Transcription</option>
          <option :value="3">Step 3: Classification</option>
          <option :value="4">Step 4: Segmentation</option>
          <option :value="5">Step 5: Dimensions</option>
          <option :value="6">Step 6: Aggregation</option>
        </select>
        <div style="display: flex; gap: 8px; justify-content: flex-end;">
          <button class="btn btn-ghost btn-sm" @click="showResetModal = false">Cancel</button>
          <button class="btn btn-danger btn-sm" @click="doReset">Reset</button>
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
  retryErrors,
  imageUrl,
} from '../api.js'

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
      activeTab: 'pages',
      expandedAnswers: new Set(),
      lightboxImage: null,
      showResetModal: false,
      resetStep: 3,
      loading: false,
      
      // Runner State
      runConfig: {
        workers: 4,
        dpi: 200,
        llm_model: 'gemma-4-26b-a4b',
      },
      pipelineRunning: false,
      eventSource: null,
      logs: [],
      tasks: {},
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
    openLightbox(page) {
      this.lightboxImage = this.getImageUrl(page)
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
    async doReset() {
      try {
        await resetPipeline(this.resetStep)
        this.showResetModal = false
        alert(`Reset from step ${this.resetStep} successful.`)
        await this.refreshAll()
      } catch (e) {
        alert('Reset failed: ' + e.message)
      }
    },

    // Runner Logic
    async startPipeline() {
      try {
        this.pipelineRunning = true
        this.logs = []
        this.tasks = {}
        await runPipeline(this.runConfig)
      } catch(e) {
        alert("Could not start pipeline: " + e.message)
        this.pipelineRunning = false
      }
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
            this.$nextTick(() => {
              if (this.$refs.terminal) {
                this.$refs.terminal.scrollTop = this.$refs.terminal.scrollHeight
              }
            })
          }
        }
      })
    }
  },
}
</script>
