<script setup lang="ts">
import { computed, onMounted, shallowRef } from "vue";
import PageHeader from "../components/layout/PageHeader.vue";
import { useToasts } from "../composables/useToasts";
import { api } from "../lib/api";
import { stringify } from "../lib/format";

interface ToolStatus {
  name: string;
  command: string;
  available: boolean;
  version: string;
  error: string;
}

const status = shallowRef("Loading tools...");
const tools = shallowRef<ToolStatus[]>([]);
const output = computed(() => stringify(tools.value));
const toasts = useToasts();

onMounted(loadTools);

async function loadTools() {
  status.value = "Loading tools...";
  try {
    const payload = await api<{ tools: ToolStatus[] }>("/api/tools");
    tools.value = payload.tools || [];
    status.value = "Loaded";
    toasts.success("Tools loaded", `${tools.value.length} tool(s) checked.`);
  } catch (error) {
    tools.value = [];
    status.value = "Failed to load tools";
    toasts.error("Tool check failed", status.value);
  }
}
</script>

<template>
  <div class="panel grid">
    <PageHeader title="Tools" description="Check local binary availability and versions.">
      <template #actions>
        <button type="button" @click="loadTools">Refresh tools</button>
      </template>
    </PageHeader>
    <div class="field">
      <h3>Status</h3>
      <p id="tools-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="field">
      <h3>Tool metadata</h3>
      <pre id="tools-output" role="status" aria-live="polite">{{ output }}</pre>
    </div>
  </div>
</template>
