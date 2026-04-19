<template>
  <div id="app" class="god-mode-app">
    <nav class="global-sidebar">
      <div class="logo">🚀</div>
      <div class="nav-links">
        <button 
          :class="['nav-btn', { active: workspace === 'analyze' }]" 
          @click="workspace = 'analyze'"
          title="PDF Analyzer"
        >
          📄
        </button>
        <button 
          :class="['nav-btn', { active: workspace === 'video' }]" 
          @click="workspace = 'video'"
          title="Video Studio"
        >
          🎬
        </button>
        <button 
          :class="['nav-btn', { active: workspace === 'news' }]" 
          @click="workspace = 'news'"
          title="News Hub"
        >
          📰
        </button>
        <button 
          :class="['nav-btn', { active: workspace === 'image' }]" 
          @click="workspace = 'image'"
          title="Image Tools"
        >
          🖼️
        </button>
        <!-- Settings Tab -->
        <button 
          :class="['nav-btn', { active: workspace === 'settings' }]" 
          @click="workspace = 'settings'"
          title="Settings"
        >
          ⚙️
        </button>
      </div>
      <div class="nav-footer">
        <div class="status-indicator" :title="healthStatus" :class="{ ok: healthStatus === 'connected' }"></div>
      </div>
    </nav>
    
    <div class="workspace-container">
      <AnalyzeStudio v-show="workspace === 'analyze'" />
      <VideoStudio v-show="workspace === 'video'" />
      <NewsStudio v-show="workspace === 'news'" />
      <ImageStudio v-show="workspace === 'image'" />
      <SettingsStudio v-show="workspace === 'settings'" />
    </div>
  </div>
</template>

<script>
import AnalyzeStudio from './components/AnalyzeStudio.vue'
import VideoStudio from './components/VideoStudio.vue'
import NewsStudio from './components/NewsStudio.vue'
import ImageStudio from './components/ImageStudio.vue'
import SettingsStudio from './components/SettingsStudio.vue'

export default {
  name: 'App',
  components: {
    AnalyzeStudio,
    VideoStudio,
    NewsStudio,
    ImageStudio,
    SettingsStudio
  },
  data() {
    return {
      workspace: 'analyze',
      healthStatus: 'disconnected'
    }
  },
  mounted() {
    this.checkHealth()
    setInterval(this.checkHealth, 30000)
  },
  methods: {
    async checkHealth() {
      try {
        const res = await fetch('http://127.0.0.1:8765/api/health')
        const data = await res.json()
        if (data.status === 'ok') {
          this.healthStatus = 'connected'
        }
      } catch (e) {
        this.healthStatus = 'disconnected'
      }
    }
  }
}
</script>

<style>
/* Reset some layout constraints in styles if needed */
.god-mode-app {
  display: flex;
  height: 100vh;
  width: 100vw;
  overflow: hidden;
  background-color: var(--bg-surface);
}

.global-sidebar {
  width: 60px;
  background-color: var(--bg-card);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 0;
  z-index: 100;
}

.global-sidebar .logo {
  font-size: 24px;
  margin-bottom: 32px;
}

.nav-links {
  display: flex;
  flex-direction: column;
  gap: 16px;
  flex-grow: 1;
}

.nav-btn {
  background: transparent;
  border: none;
  font-size: 20px;
  width:  ৪০px;
  height: 40px;
  border-radius: 8px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  opacity: 0.6;
}

.nav-btn:hover {
  opacity: 0.9;
  background: rgba(255, 255, 255, 0.05);
}

.nav-btn.active {
  opacity: 1;
  background: rgba(144, 150, 255, 0.15);
  box-shadow: inset 2px 0 0 var(--accent);
}

.status-indicator {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--danger);
}
.status-indicator.ok {
  background: var(--success);
}

.workspace-container {
  flex-grow: 1;
  height: 100%;
  overflow: hidden;
  position: relative;
}

/* Update the AnalyzeStudio wrapper to fill space */
.workspace-layout {
  display: flex;
  height: 100%;
  width: 100%;
}
</style>
