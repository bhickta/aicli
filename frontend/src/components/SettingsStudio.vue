<template>
  <div class="studio-container">
    <div class="studio-header">
      <h2>Global System Settings</h2>
      <p>Configure backend API endpoints, LLM provider settings, and global concurrency behaviors here.</p>
    </div>

    <div class="studio-content">
      <div class="config-card">
        <h3 style="margin-bottom: 8px;">LLM Settings</h3>
        <p class="description">
          Connection details for LM Studio or other OpenAI-compatible local endpoints.
        </p>

        <div class="config-grid">
          <div class="form-group span-full">
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

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { API_BASE } from '../constants/api.constants'

const settings = ref({
  lm_studio_base_url: 'http://localhost:1234/v1',
  lm_studio_api_key: 'sk-lm-UIfIMcJs:ga4Fhyit5WI6tz0FJTbR',
  model_name: 'local-model'
})

async function loadSettings() {
  try {
    const res = await fetch(`${API_BASE}/api/settings`)
    if (res.ok) {
      settings.value = await res.json()
    }
  } catch (err) {
    console.error('Failed to load settings:', err)
  }
}

async function saveSettings() {
  try {
    const res = await fetch(`${API_BASE}/api/settings`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings.value)
    })
    if (res.ok) {
      alert('Settings successfully updated across the global system.')
    } else {
      alert('Failed to update settings.')
    }
  } catch (err: any) {
    alert('Error saving settings: ' + err.message)
  }
}

onMounted(loadSettings)
</script>

<style scoped>
.studio-container {
  padding: 32px 48px;
  max-width: 1200px;
  margin: 0 auto;
}
.studio-header h2 { margin: 0 0 8px; font-size: 28px; font-weight: 600; }
.studio-header p { margin: 0; color: var(--text-secondary); font-size: 15px; }
.studio-content { margin-top: 32px; display: flex; flex-direction: column; gap: 24px; }
</style>
