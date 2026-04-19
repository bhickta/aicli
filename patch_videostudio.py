import re
from pathlib import Path

fpath = Path("frontend/src/components/VideoStudio.vue")
content = fpath.read_text()

# Add to Sidebar
content = content.replace(
'''        <button class="btn btn-ghost" style="width: 100%; text-align: left;" @click="activeTab = 'tag'">
          🏷️ AI Tagging
        </button>''', 
'''        <button class="btn btn-ghost" style="width: 100%; text-align: left;" @click="activeTab = 'tag'">
          🏷️ AI Tagging
        </button>
        <button class="btn btn-ghost" style="width: 100%; text-align: left; margin-top: 8px;" @click="activeTab = 'notes'">
          📝 Study Notes
        </button>''')

# Add Tab Target
notes_tab = '''
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
'''
content = content.replace('<!-- AI Tagging Tab -->', notes_tab + '\n        <!-- AI Tagging Tab -->')

# Add to Config
content = content.replace('''tagConfig: {
        target_path: '',
        whisper_model: 'base',
        write: false,
        transcribe_only: false,
        full_cc: false,
        retranscribe: false,
        workers: 2,
      }''', '''tagConfig: {
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
      }''')

# Add to TabTitle
content = content.replace("if (this.activeTab === 'compress') return 'NVENC Compression'", "if (this.activeTab === 'compress') return 'NVENC Compression'\n      if (this.activeTab === 'notes') return 'Markdown Study Notes'")

# Add Method
method = '''    async startNotes() {
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
    connectStream()'''

content = content.replace("connectStream()", method)

fpath.write_text(content)
print("VideoStudio UI Component Patched.")
