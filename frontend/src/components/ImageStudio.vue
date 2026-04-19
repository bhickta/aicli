<template>
  <div class="workspace-layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>Image Studio</h1>
        <div class="subtitle">AI Vision Processing</div>
      </div>

      <div class="sidebar-actions" style="margin-top: 16px;">
        <button class="btn btn-primary" style="width: 100%; border-radius: 6px; padding: 10px;" @click="activeTab = 'rename'">
          🪄 AI Renamer
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left; margin-top: 8px;" @click="activeTab = 'clean'">
          🧹 Junk Cleaner
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left;" @click="activeTab = 'digitize'">
          📝 Markdown Digitize
        </button>
      </div>
    </aside>

    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        <!-- Rename Tab -->
        <div v-if="activeTab === 'rename'" class="card">
          <h3 style="margin-bottom: 8px;">Auto Semantic Rename</h3>
          <p class="description">
            Uses Vision Models to scan a directory of images and intelligently rename them based on what they depict.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target Directory (Absolute Path)</label>
              <input type="text" v-model="renameConfig.target_path" placeholder="/home/bhickta/Screenshots/" />
            </div>
            
            <div class="form-group">
              <label>LM Vision Workers</label>
              <input type="number" v-model.number="renameConfig.workers" min="1" max="16" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startRename" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Start Rename Pipeline' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="renameConfig.auto_rename" /> Auto Apply
            </label>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="renameConfig.sync_refs" /> Sync MD/JSON Links
            </label>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="renameConfig.trash_junk" /> Trash Logos
            </label>
          </div>
        </div>

        <!-- Clean Tab -->
        <div v-if="activeTab === 'clean'" class="card">
          <h3 style="margin-bottom: 8px;">Junk Screenshot Sweeper</h3>
          <p class="description">
            Scans a directory using AI. Throws cosmetic icons, logos, or useless graphics into a .trash folder.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target Directory (Absolute Path)</label>
              <input type="text" v-model="cleanConfig.target_path" placeholder="/home/bhickta/Screenshots/" />
            </div>
            
            <div class="form-group">
              <label>LM Vision Workers</label>
              <input type="number" v-model.number="cleanConfig.workers" min="1" max="16" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startClean" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Start Junk Cleaner' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="cleanConfig.auto_trash" /> Auto Trash
            </label>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="cleanConfig.strict" /> Strict Mode (Aggressive)
            </label>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="cleanConfig.sync_refs" /> Sync References
            </label>
          </div>
        </div>

        <!-- Digitize Tab -->
        <div v-if="activeTab === 'digitize'" class="card">
          <h3 style="margin-bottom: 8px;">Markdown Digitize</h3>
          <p class="description">
            Extracts pure-text images into markdown data and replaces file references in your notes natively.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target Directory (Absolute Path)</label>
              <input type="text" v-model="digitizeConfig.target_path" placeholder="/home/bhickta/Screenshots/" />
            </div>
            
            <div class="form-group">
              <label>LM Vision Workers</label>
              <input type="number" v-model.number="digitizeConfig.workers" min="1" max="16" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startDigitize" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Digitize Text Images' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="digitizeConfig.auto_replace" /> Auto Convert & Delete
            </label>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="digitizeConfig.sync_refs" /> Inject into Markdown
            </label>
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
  name: 'ImageStudio',
  data() {
    return {
      activeTab: 'rename',
      pipelineRunning: false,
      eventSource: null,
      logs: [],
      tasks: {},
      renameConfig: {
        target_path: '',
        auto_rename: true,
        workers: 4,
        sync_refs: false,
        trash_junk: true
      },
      cleanConfig: {
        target_path: '',
        auto_trash: true,
        strict: false,
        sync_refs: false,
        workers: 4
      },
      digitizeConfig: {
        target_path: '',
        auto_replace: true,
        sync_refs: true,
        workers: 2
      }
    }
  },
  computed: {
    tabTitle() {
      if (this.activeTab === 'rename') return 'AI Vision Renamer'
      if (this.activeTab === 'clean') return 'Junk Screenshot Sweeper'
      return 'Markdown Digitizer'
    }
  },
  beforeUnmount() {
    if (this.eventSource) this.eventSource.close()
  },
  methods: {
    async startRename() {
      if (!this.renameConfig.target_path) {
        alert("Please provide the absolute path to the screenshot directory.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/image/rename', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.renameConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startClean() {
      if (!this.cleanConfig.target_path) {
        alert("Please provide the absolute path to the screenshot directory.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/image/clean', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.cleanConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startDigitize() {
      if (!this.digitizeConfig.target_path) {
        alert("Please provide the absolute path to the screenshot directory.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/image/digitize', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.digitizeConfig)
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

      this.eventSource = new EventSource('http://127.0.0.1:8765/api/image/stream')
      this.eventSource.onmessage = (e) => {
        const data = JSON.parse(e.data)
        
        if (data.type === 'status') {
          if (data.status === 'error') {
            this.logs.push(`[SYSTEM ERROR] ${data.message}`)
            this.pipelineRunning = false
          }
          if (data.status === 'completed') {
            this.logs.push(`[SYSTEM] Image Pipeline execution completed successfully.`)
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
  grid-column: span 1;
}
</style>
