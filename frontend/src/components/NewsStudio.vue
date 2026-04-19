<template>
  <div class="workspace-layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>News Hub</h1>
        <div class="subtitle">AI Current Affairs Processing</div>
      </div>

      <div class="sidebar-actions" style="margin-top: 16px;">
        <button class="btn btn-primary" style="width: 100%; border-radius: 6px; padding: 10px;" @click="activeTab = 'process'">
          📰 God-Mode RAG
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left; margin-top: 8px;" @click="activeTab = 'dedupe'">
          ✨ Excel Deduplicator
        </button>
      </div>
    </aside>

    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        <!-- Process Tab -->
        <div v-if="activeTab === 'process'" class="card">
          <h3 style="margin-bottom: 8px;">Parse & Deduplicate JSON Feed</h3>
          <p class="description">
            Parses a raw JSON news feed, appends it into an existing master Excel database, and natively deduplicates the entire dataset using RAG + Local LLM.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Input JSON File (Absolute Path)</label>
              <input type="text" v-model="processConfig.json_path" placeholder="/home/bhickta/News/december.json" />
            </div>
            
            <div class="form-group span-2">
              <label>Output Master Excel (Absolute Path)</label>
              <input type="text" v-model="processConfig.output" placeholder="/home/bhickta/News/master_news.xlsx" />
            </div>

            <div class="form-group">
              <label>Cosine Threshold</label>
              <input type="number" step="0.05" min="0.5" max="1.0" v-model.number="processConfig.threshold" />
            </div>
            
            <div class="form-group">
              <label>LLM Merge Workers</label>
              <input type="number" v-model.number="processConfig.workers" min="1" max="24" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startProcess" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Pipeline Running...' : '▶ Start End-to-End Build' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="processConfig.no_cache" /> Disable Cache
            </label>
          </div>
        </div>

        <!-- Dedupe Tab -->
        <div v-if="activeTab === 'dedupe'" class="card">
          <h3 style="margin-bottom: 8px;">Excel AI Deduplicator</h3>
          <p class="description">
            Takes an existing Excel file and uses local LLM embeddings to find and merge duplicate rows.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target Excel File (Absolute Path)</label>
              <input type="text" v-model="dedupeConfig.file_path" placeholder="/home/bhickta/News/master_news.xlsx" />
            </div>
            
            <div class="form-group">
              <label>Output Excel (Optional)</label>
              <input type="text" v-model="dedupeConfig.output" placeholder="Defaults to _deduped.xlsx" />
            </div>

            <div class="form-group">
              <label>Cosine Threshold</label>
              <input type="number" step="0.05" min="0.5" max="1.0" v-model.number="dedupeConfig.threshold" />
            </div>
            
            <div class="form-group">
              <label>LLM Merge Workers</label>
              <input type="number" v-model.number="dedupeConfig.workers" min="1" max="24" />
            </div>
          </div>

          <div style="margin-top: 24px;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startDedupe" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Start AI Deduplication' }}
            </button>
          </div>
        </div>

        <!-- Terminal Output -->
        <div class="console-panel" style="margin-top: 24px;" v-if="logs.length > 0 || pipelineRunning">
          <div class="console-header">
            <h3>Live Execution Logs</h3>
            <span v-if="pipelineRunning" class="pulse"></span>
          </div>
          
          <div class="tasks-overlay" v-if="Object.keys(tasks).length > 0">
            <div v-for="task in tasks" :key="task.id" class="task-bar-container">
              <div class="task-info">
                <span>{{ task.description }}</span>
              </div>
            </div>
          </div>
          
          <div class="terminal" ref="terminal" style="height: 400px;">
            <div v-for="(log, i) in logs" :key="i" class="log-line">
              <span v-if="log.includes('[SUCCESS]')" style="color: var(--success);">{{ log }}</span>
              <span v-else-if="log.includes('[ERROR]')" style="color: var(--danger);">{{ log }}</span>
              <span v-else>{{ log }}</span>
            </div>
          </div>
        </div>

      </div>
    </main>
  </div>
</template>

<script>
export default {
  name: 'NewsStudio',
  data() {
    return {
      activeTab: 'process',
      pipelineRunning: false,
      eventSource: null,
      logs: [],
      tasks: {},
      processConfig: {
        json_path: '',
        output: '',
        threshold: 0.82,
        workers: 8,
        no_cache: false
      },
      dedupeConfig: {
        file_path: '',
        output: '',
        threshold: 0.82,
        workers: 10
      }
    }
  },
  computed: {
    tabTitle() {
      if (this.activeTab === 'process') return 'God-Mode RAG Pipeline'
      return 'Excel AI Deduplicator'
    }
  },
  beforeUnmount() {
    if (this.eventSource) this.eventSource.close()
  },
  methods: {
    async startProcess() {
      if (!this.processConfig.json_path) {
        alert("Please provide the path to the JSON file.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/news/process', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.processConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startDedupe() {
      if (!this.dedupeConfig.file_path) {
        alert("Please provide the path to the Excel file.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/news/dedupe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.dedupeConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    connectStream() {
      if (this.eventSource) this.eventSource.close()

      this.eventSource = new EventSource('http://127.0.0.1:8765/api/news/stream')
      this.eventSource.onmessage = (e) => {
        const data = JSON.parse(e.data)
        
        if (data.type === 'status') {
          if (data.status === 'error') {
            this.logs.push(`[SYSTEM ERROR] ${data.message}`)
            this.pipelineRunning = false
          }
          if (data.status === 'completed') {
            this.logs.push(`[SYSTEM] News Pipeline execution completed successfully.`)
            this.pipelineRunning = false
          }
        }
        else if (data.type === 'task_add') {
          this.tasks[data.task_id] = { id: data.task_id, description: data.description, total: data.total, completed: 0 }
        }
        else if (data.type === 'task_progress') {
          if (this.tasks[data.task_id]) {
            this.tasks[data.task_id].completed = data.completed
            if (this.tasks[data.task_id].completed >= this.tasks[data.task_id].total) {
              setTimeout(() => delete this.tasks[data.task_id], 1500)
            }
          }
        }
        else if (data.type === 'log') {
          this.logs.push(data.message)
          this.$nextTick(() => { if (this.$refs.terminal) this.$refs.terminal.scrollTop = this.$refs.terminal.scrollHeight })
        }
      }
    }
  }
}
</script>

<style scoped>
.description {
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.5;
  margin-bottom: 24px;
}
.card {
  background: var(--bg-card);
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  padding: 24px;
}
.config-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}
.span-2 {
  grid-column: span 2;
}
</style>
