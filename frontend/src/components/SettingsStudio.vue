<template>
  <div class="studio-container">
    <div class="studio-header">
      <h2>Global System Settings</h2>
      <p>Configure backend API endpoints, LLM provider settings, and global concurrency behaviors here.</p>
    </div>

    <div class="studio-content">
      <div class="card">
        <h3 style="margin-bottom: 8px;">LLM Settings</h3>
        <p class="description">
          Connection details for LM Studio or other OpenAI-compatible local endpoints.
        </p>

        <div class="config-grid">
          <div class="form-group span-2">
            <label>LM Studio Base URL</label>
            <input type="text" v-model="settings.lm_studio_base_url" placeholder="http://localhost:1234/v1" />
          </div>

          <div class="form-group">
            <label>API Key</label>
            <input type="text" v-model="settings.lm_studio_api_key" placeholder="sk-..." />
          </div>

          <div class="form-group">
            <label>Preferred Base Model</label>
            <input type="text" v-model="settings.model_name" placeholder="local-model" />
          </div>
        </div>

        <div style="margin-top: 24px;">
          <button class="btn btn-primary" style="font-size: 14px; padding: 12px 24px;" @click="saveSettings">
            Save Configuration
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SettingsStudio',
  data() {
    return {
      settings: {
        lm_studio_base_url: 'http://localhost:1234/v1',
        lm_studio_api_key: 'sk-lm-UIfIMcJs:ga4Fhyit5WI6tz0FJTbR',
        model_name: 'local-model'
      }
    }
  },
  mounted() {
    this.loadSettings()
  },
  methods: {
    async loadSettings() {
      try {
        const res = await fetch('http://127.0.0.1:8765/api/settings')
        if (res.ok) {
          const data = await res.json()
          this.settings = data
        }
      } catch (err) {
        console.error("Failed to load settings:", err)
      }
    },
    async saveSettings() {
      try {
        const res = await fetch('http://127.0.0.1:8765/api/settings', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(this.settings)
        })
        if (res.ok) {
          alert("Settings successfully updated across the global system.")
        } else {
          alert("Failed to update settings.")
        }
      } catch (err) {
        alert("Error saving settings: " + err.message)
      }
    }
  }
}
</script>

<style scoped>
.studio-container {
  padding: 32px 48px;
  max-width: 1200px;
  margin: 0 auto;
}
.studio-header h2 { margin: 0 0 8px; font-size: 28px; font-weight: 600; }
.studio-header p { margin: 0; color: #a1a1aa; font-size: 15px; }
.studio-content { margin-top: 32px; display: flex; flex-direction: column; gap: 24px; }
.card { background: #18191c; border-radius: 12px; padding: 24px; border: 1px solid #27272a; box-shadow: 0 4px 6px rgba(0, 0, 0, 0.2); }
.description { color: #a1a1aa; font-size: 14px; margin-bottom: 24px; }
.config-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; }
.form-group { display: flex; flex-direction: column; gap: 8px; }
.form-group.span-2 { grid-column: span 2; }
label { font-size: 13px; font-weight: 600; color: #d4d4d8; }
input { background: #27272a; border: 1px solid #3f3f46; color: #fff; padding: 10px 12px; border-radius: 6px; font-size: 14px; font-family: inherit; }
input:focus { outline: none; border-color: #3b82f6; }
.btn { border: none; border-radius: 6px; font-weight: 500; cursor: pointer; transition: all 0.2s; }
.btn-primary { background: #3b82f6; color: white; display: inline-flex; align-items: center; justify-content: center; gap: 8px; }
.btn-primary:hover:not(:disabled) { background: #2563eb; }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
