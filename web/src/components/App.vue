<script setup lang="ts">
import { computed, onMounted, onUnmounted } from "vue";
import { api, loadSettings } from "../lib/api";
import { appState } from "../stores/appState";
import type { SystemResources, ViewName } from "../types";
import SystemUsage from "./SystemUsage.vue";
import ChatView from "../views/ChatView.vue";
import JobsView from "../views/JobsView.vue";
import ProvidersView from "../views/ProvidersView.vue";
import SettingsView from "../views/SettingsView.vue";
import StudyView from "../views/StudyView.vue";
import ToolsView from "../views/ToolsView.vue";
import WorkflowsView from "../views/WorkflowsView.vue";
import ZettelView from "../views/ZettelView.vue";

const views: Array<{ id: ViewName; label: string; description: string; group: "Work" | "System" }> = [
  { id: "chat", label: "Chat", description: "Ask a configured model", group: "Work" },
  { id: "study-archive", label: "Study", description: "UPSC copies, recall, lectures", group: "Work" },
  { id: "workflows", label: "Workflows", description: "Documents, media, automation", group: "Work" },
  { id: "zettel", label: "Zettel", description: "Vault merge and review", group: "Work" },
  { id: "jobs", label: "Jobs", description: "Queue, progress, history", group: "System" },
  { id: "providers", label: "Providers", description: "Models and health", group: "System" },
  { id: "tools", label: "Tools", description: "Local binaries", group: "System" },
  { id: "settings", label: "Settings", description: "Defaults and paths", group: "System" },
];

const groupedViews = computed(() => {
  return [
    { label: "Work", views: views.filter((view) => view.group === "Work") },
    { label: "System", views: views.filter((view) => view.group === "System") },
  ];
});

const activeView = computed(() => views.find((view) => view.id === appState.view) || views[0]);

const activeComponent = computed(() => {
  switch (appState.view) {
    case "workflows":
      return WorkflowsView;
    case "study-archive":
      return StudyView;
    case "zettel":
      return ZettelView;
    case "jobs":
      return JobsView;
    case "providers":
      return ProvidersView;
    case "tools":
      return ToolsView;
    case "settings":
      return SettingsView;
    default:
      return ChatView;
  }
});

let resourceTimer = 0;

onMounted(async () => {
  await Promise.all([refreshSettings(), refreshHealth(), refreshResources()]);
  resourceTimer = window.setInterval(refreshResources, 2000);
});

onUnmounted(() => {
  window.clearInterval(resourceTimer);
});

async function refreshSettings() {
  appState.settings = await loadSettings();
}

async function refreshHealth() {
  try {
    const result = await api<{ status: string }>("/api/health");
    appState.health = result.status;
  } catch {
    appState.health = "offline";
  }
}

async function refreshResources() {
  try {
    appState.systemResources = await api<SystemResources>("/api/system/resources");
  } catch {
    appState.systemResources = null;
  }
}
</script>

<template>
  <aside class="app-sidebar">
    <div class="app-brand">
      <strong>AICLI</strong>
      <span>Local AI Control Center</span>
    </div>
    <nav aria-label="Primary navigation">
      <div id="primary-tabs" aria-label="Main views">
        <section v-for="group in groupedViews" :key="group.label" class="nav-group">
          <h2 class="nav-group-label">{{ group.label }}</h2>
          <button
            v-for="view in group.views"
            :id="`tab-${view.id}`"
            :key="view.id"
            type="button"
            :class="{ 'active-tab': appState.view === view.id }"
            :aria-current="appState.view === view.id ? 'page' : undefined"
            aria-controls="view"
            @click="appState.view = view.id"
          >
            <span class="nav-label">{{ view.label }}</span>
            <span class="nav-description">{{ view.description }}</span>
          </button>
        </section>
      </div>
    </nav>
  </aside>
  <section class="app-main">
    <header class="app-header">
      <div class="app-title">
        <h1>{{ activeView.label }}</h1>
        <p>{{ activeView.description }}</p>
      </div>
      <div class="header-status">
        <SystemUsage :resources="appState.systemResources" />
        <span id="health">{{ appState.health }}</span>
      </div>
    </header>
    <component :is="activeComponent" v-if="appState.settings" id="view" />
    <div v-else id="view" class="panel grid">
      <h2>Loading</h2>
      <p class="status-line">Loading settings...</p>
    </div>
  </section>
</template>
