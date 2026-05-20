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
import ToolsView from "../views/ToolsView.vue";
import WorkflowsView from "../views/WorkflowsView.vue";
import ZettelView from "../views/ZettelView.vue";

const views: Array<{ id: ViewName; label: string }> = [
  { id: "chat", label: "Chat" },
  { id: "workflows", label: "Workflows" },
  { id: "zettel", label: "Zettel" },
  { id: "jobs", label: "Jobs" },
  { id: "providers", label: "Providers" },
  { id: "tools", label: "Tools" },
  { id: "settings", label: "Settings" },
];

const activeComponent = computed(() => {
  switch (appState.view) {
    case "workflows":
      return WorkflowsView;
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
  <aside>
    <strong>AICLI</strong>
    <nav aria-label="Primary navigation">
      <div id="primary-tabs" role="tablist" aria-label="Main views">
        <button
          v-for="view in views"
          :id="`tab-${view.id}`"
          :key="view.id"
          type="button"
          role="tab"
          :class="{ 'active-tab': appState.view === view.id }"
          :aria-selected="appState.view === view.id"
          aria-controls="view"
          @click="appState.view = view.id"
        >
          {{ view.label }}
        </button>
      </div>
    </nav>
  </aside>
  <section>
    <header>
      <h1>Local AI Control Center</h1>
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
