<template>
  <div class="workspace-layout">
    <!-- Sidebar for Video Studio -->
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>Video Studio</h1>
        <div class="subtitle">Course Processing & Tagging</div>
      </div>

      <div class="sidebar-actions" style="margin-top: 16px;">
        <button class="btn btn-primary" style="width: 100%; border-radius: 6px; padding: 10px;" @click="activeTab = 'course'">
          🎓 God-Mode Course
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left; margin-top: 8px;" @click="activeTab = 'compress'">
          ⚡ GPU Compress
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left;" @click="activeTab = 'tag'">
          🏷️ AI Tagging
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left; margin-top: 8px;" @click="activeTab = 'notes'">
          📝 Study Notes
        </button>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="main-content">
      <div class="top-bar">
        <h2>{{ tabTitle }}</h2>
      </div>

      <div class="content-body" style="padding: 24px; max-width: 900px;">
        
        <!-- Course Builder Tab -->
        <div v-if="activeTab === 'course'" class="card">
          <h3 style="margin-bottom: 8px;">End-to-End Course Pipeline</h3>
          <p class="description">
            Turns a folder of raw videos into a single merged course pack. Extracts text, uses Whisper for subtitles, generates slideshows, and merges everything into a master video container with embedded CC.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target Directory (Absolute Path)</label>
              <input type="text" v-model="courseConfig.target_dir" placeholder="/home/bhickta/Videos/RawCourse" />
            </div>
            
            <div class="form-group">
              <label>Whisper Model (Phase 1)</label>
              <input type="text" v-model="courseConfig.whisper_model" />
            </div>
            
            <div class="form-group">
              <label>LLM Tagging Model (Phase 2)</label>
              <input type="text" v-model="courseConfig.llm_model" />
            </div>

            <div class="form-group">
              <label>Whisper Workers</label>
              <input type="number" v-model.number="courseConfig.w1" min="1" max="4" />
            </div>
            <div class="form-group">
              <label>Tag Workers</label>
              <input type="number" v-model.number="courseConfig.w2" min="1" max="24" />
            </div>
            <div class="form-group">
              <label>NVENC Workers</label>
              <input type="number" v-model.number="courseConfig.w3" min="1" max="24" />
            </div>
          </div>

          <div style="margin-top: 24px;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startCoursePipeline" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Pipeline Running...' : '▶ Start Compilation' }}
            </button>
          </div>
        </div>

        <!-- GPU Compress Tab -->
        <div v-if="activeTab === 'compress'" class="card">
          <h3 style="margin-bottom: 8px;">GPU Video Compressor</h3>
          <p class="description">
            Compress files blazingly fast by fully saturating an NVENC capable GPU.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target File/Directory (Absolute Path)</label>
              <input type="text" v-model="compressConfig.target_path" placeholder="/home/bhickta/Videos/Raw/" />
            </div>
            
            <div class="form-group">
              <label>Preset</label>
              <select v-model="compressConfig.preset">
                <option value="ultralight">ultralight</option>
                <option value="light">light</option>
                <option value="balanced">balanced</option>
                <option value="slideshow">slideshow</option>
              </select>
            </div>

            <div class="form-group">
              <label>Resolution (p)</label>
              <input type="number" v-model.number="compressConfig.resolution" min="144" max="1080" />
            </div>
            
            <div class="form-group">
              <label>CRF (Optional Quality Override, 0-51)</label>
              <input type="number" v-model.number="compressConfig.crf" min="0" max="51" placeholder="Auto" />
            </div>
            
            <div class="form-group">
              <label>Number of Workers</label>
              <input type="number" v-model.number="compressConfig.workers" min="1" max="24" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startCompress" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Start GPU Compress' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="compressConfig.overwrite" /> Overwrite Original
            </label>
          </div>
        </div>

        <!-- AI Tagging Tab -->
        <div v-if="activeTab === 'tag'" class="card">
          <h3 style="margin-bottom: 8px;">AI Video Tagger</h3>
          <p class="description">
            Transcribe, summarize, and intelligently tag lecture videos using Whisper and LLMs.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target File/Directory (Absolute Path)</label>
              <input type="text" v-model="tagConfig.target_path" placeholder="/home/bhickta/Videos/Raw/" />
            </div>
            
            <div class="form-group">
              <label>Whisper Model</label>
              <input type="text" v-model="tagConfig.whisper_model" />
            </div>

            <div class="form-group">
               <label>Workers</label>
               <input type="number" v-model.number="tagConfig.workers" min="1" max="24" />
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; flex-wrap: wrap; gap: 16px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startTag" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Start AI Tagging' }}
            </button>
            
            <div style="display: flex; flex-direction: column; gap: 6px;">
              <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
                <input type="checkbox" v-model="tagConfig.write" /> Write Mode (Rename Files)
              </label>
              <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
                <input type="checkbox" v-model="tagConfig.transcribe_only" /> Transcribe Only
              </label>
            </div>
            <div style="display: flex; flex-direction: column; gap: 6px;">
              <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
                <input type="checkbox" v-model="tagConfig.full_cc" /> Generate Subtitles (.srt)
              </label>
              <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
                <input type="checkbox" v-model="tagConfig.retranscribe" /> Force Retranscribe
              </label>
            </div>
          </div>
        </div>

        <!-- Study Notes Tab -->
        <div v-if="activeTab === 'notes'" class="card">
          <h3 style="margin-bottom: 8px;">Markdown Study Notes</h3>
          <p class="description">
            Generate ultra-dense exam-ready study notes from videos with sidecar .srt files or embedded subtitle streams using local LLM inference.
          </p>

          <div class="config-grid">
            <div class="form-group span-2">
              <label>Target File/Directory (Absolute Path)</label>
              <input type="text" v-model="notesConfig.target_path" placeholder="/home/bhickta/Videos/Raw/" />
            </div>
            
            <div class="form-group">
              <label>Note Style</label>
              <select v-model="notesConfig.style">
                <option value="bullet">bullet (ultra-dense)</option>
                <option value="clean">clean (removes fluff)</option>
              </select>
            </div>
          </div>

          <div style="margin-top: 24px; display: flex; gap: 12px; align-items: center;">
            <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="startNotes" :disabled="pipelineRunning">
              {{ pipelineRunning ? 'Running...' : '▶ Generate Notes' }}
            </button>
            <label style="display: flex; align-items: center; gap: 6px; font-size: 13px;">
              <input type="checkbox" v-model="notesConfig.overwrite" /> Overwrite Existing
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
  name: 'VideoStudio',
  data() {
    return {
      activeTab: 'course',
      pipelineRunning: false,
      eventSource: null,
      logs: [],
      tasks: {},
      courseConfig: {
        target_dir: '',
        whisper_model: 'large-v3',
        cleanup: 'keep',
        w1: 2,
        w2: 12,
        w3: 12,
        llm_model: 'gemma-4-26b-a4b'
      },
      compressConfig: {
        target_path: '',
        preset: 'light',
        resolution: 240,
        overwrite: false,
        workers: 4,
        crf: null,
      },
      tagConfig: {
        target_path: '',
        whisper_model: 'base',
        write: false,
        transcribe_only: false,
        full_cc: false,
        retranscribe: false,
        workers: 2,
      },
      notesConfig: {
        target_path: '',
        style: 'bullet',
        overwrite: false,
      }
    }
  },
  computed: {
    tabTitle() {
      if (this.activeTab === 'course') return 'Course Compilation'
      if (this.activeTab === 'compress') return 'NVENC Compression'
      if (this.activeTab === 'notes') return 'Markdown Study Notes'
      return 'Video Tagging'
    }
  },
  beforeUnmount() {
    if (this.eventSource) this.eventSource.close()
  },
  methods: {
    async startCoursePipeline() {
      if (!this.courseConfig.target_dir) {
        alert("Please provide an absolute path to the target directory.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/video/course/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.courseConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startCompress() {
      if (!this.compressConfig.target_path) {
        alert("Please provide an absolute path.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/video/compress/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.compressConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startTag() {
      if (!this.tagConfig.target_path) {
        alert("Please provide an absolute path.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/video/tag/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.tagConfig)
        })
        
        if (!res.ok) throw new Error(await res.text())
        this.connectStream()
      } catch (err) {
        alert("Error starting pipeline: " + err.message)
        this.pipelineRunning = false
      }
    },
    async startNotes() {
      if (!this.notesConfig.target_path) {
        alert("Please provide an absolute path.")
        return
      }

      this.logs = []
      this.tasks = {}
      this.pipelineRunning = true

      try {
        const res = await fetch('http://127.0.0.1:8765/api/video/notes/run', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.notesConfig)
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

      this.eventSource = new EventSource('http://127.0.0.1:8765/api/video/course/stream')
      this.eventSource.onmessage = (e) => {
        const data = JSON.parse(e.data)
        
        if (data.type === 'status') {
          if (data.status === 'error') {
            this.logs.push(`[SYSTEM ERROR] ${data.message}`)
            this.pipelineRunning = false
          }
          if (data.status === 'completed') {
            this.logs.push(`[SYSTEM] Video Pipeline execution completed successfully.`)
            this.pipelineRunning = false
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
      this.eventSource.onerror = () => {
        this.eventSource.close()
        // Optionally reconnect or mark complete
        if (this.pipelineRunning) {
          this.pipelineRunning = false
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
  grid-template-columns: 1fr 1fr 1fr;
  gap: 16px;
}
.span-2 {
  grid-column: span 3;
}
</style>
