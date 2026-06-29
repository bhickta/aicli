<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef } from "vue";
import { RouterView, useRoute } from "vue-router";
import { api, loadSettings } from "../../lib/api";
import { appState } from "../../stores/appState";
import type { SystemResources } from "../../types";
import SystemUsage from "../SystemUsage.vue";
import ConfirmDialog from "../ui/ConfirmDialog.vue";
import ToastHost from "../ui/ToastHost.vue";
import AppSidebar from "./AppSidebar.vue";

const route = useRoute();
const settingsError = shallowRef("");
const activeRouteMeta = computed(() => ({
  label: String(route.meta.label || "AICLI"),
  description: String(route.meta.description || "Local AI Control Center"),
}));

let resourceTimer = 0;

onMounted(async () => {
  await Promise.all([refreshSettings(), refreshHealth(), refreshResources()]);
  resourceTimer = window.setInterval(refreshResources, 2000);
});

onUnmounted(() => {
  window.clearInterval(resourceTimer);
});

async function refreshSettings() {
  settingsError.value = "";
  try {
    appState.settings = await loadSettings();
  } catch (error) {
    appState.settings = null;
    settingsError.value = error instanceof Error ? error.message : "Failed to load settings";
  }
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
  <AppSidebar />
  <section class="app-main">
    <header class="app-header">
      <div class="app-title">
        <h1>{{ activeRouteMeta.label }}</h1>
        <p>{{ activeRouteMeta.description }}</p>
      </div>
      <div class="header-status">
        <SystemUsage :resources="appState.systemResources" />
        <span id="health">{{ appState.health }}</span>
      </div>
    </header>
    <RouterView v-if="appState.settings" id="view" />
    <div v-else-if="settingsError" id="view" class="panel grid">
      <h2>Settings unavailable</h2>
      <p class="status-line">{{ settingsError }}</p>
      <p class="muted">Start the AICLI backend on http://127.0.0.1:8765 or use the embedded Go server UI.</p>
      <button type="button" @click="refreshSettings">Retry</button>
    </div>
    <div v-else id="view" class="panel grid">
      <h2>Loading</h2>
      <p class="status-line">Loading settings...</p>
    </div>
  </section>
  <ToastHost />
  <ConfirmDialog />
</template>
