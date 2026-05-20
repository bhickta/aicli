<script setup lang="ts">
import { shallowRef, watch } from "vue";
import { api } from "../lib/api";
import { appState } from "../stores/appState";
import type { Settings } from "../types";

const editor = shallowRef("");
const status = shallowRef("Ready");
const result = shallowRef("");
const busy = shallowRef(false);

watch(() => appState.settings, (settings) => {
  editor.value = JSON.stringify(settings, null, 2);
}, { immediate: true });

async function saveSettings() {
  busy.value = true;
  status.value = "Saving...";
  result.value = "";
  try {
    appState.settings = await api<Settings>("/api/settings", {
      method: "PUT",
      body: editor.value,
    });
    result.value = "saved";
    status.value = "Saved";
  } catch (error) {
    result.value = error instanceof Error ? error.message : "Save failed";
    status.value = "Failed";
  } finally {
    busy.value = false;
  }
}
</script>

<template>
  <div class="panel grid">
    <h2>Settings</h2>
    <div class="field">
      <label for="settings-editor">Configuration JSON</label>
      <textarea id="settings-editor" v-model="editor" rows="20" />
    </div>
    <div class="field">
      <button id="settings-save" type="button" :disabled="busy" @click="saveSettings">Save</button>
    </div>
    <div class="field">
      <h3>Status</h3>
      <p id="settings-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="field">
      <h3>Save result</h3>
      <pre id="settings-result" role="status" aria-live="polite">{{ result }}</pre>
    </div>
  </div>
</template>
