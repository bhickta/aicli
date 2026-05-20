<script setup lang="ts">
import { computed, onMounted } from "vue";
import { api, loadSettings } from "../lib/api";
import { appState } from "../stores/appState";
import type { ViewName } from "../types";
import ChatView from "../views/ChatView.vue";
import JobsView from "../views/JobsView.vue";
import ProvidersView from "../views/ProvidersView.vue";
import SettingsView from "../views/SettingsView.vue";
import ToolsView from "../views/ToolsView.vue";
import WorkflowsView from "../views/WorkflowsView.vue";

const views: Array<{ id: ViewName; label: string }> = [
  { id: "chat", label: "Chat" },
  { id: "workflows", label: "Workflows" },
  { id: "jobs", label: "Jobs" },
  { id: "providers", label: "Providers" },
  { id: "tools", label: "Tools" },
  { id: "settings", label: "Settings" },
];

const activeComponent = computed(() => {
  switch (appState.view) {
    case "workflows":
      return WorkflowsView;
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

onMounted(async () => {
  await Promise.all([refreshSettings(), refreshHealth()]);
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
      <span id="health">{{ appState.health }}</span>
    </header>
    <component :is="activeComponent" v-if="appState.settings" id="view" />
    <div v-else id="view" class="panel grid">
      <h2>Loading</h2>
      <p class="status-line">Loading settings...</p>
    </div>
  </section>
</template>
