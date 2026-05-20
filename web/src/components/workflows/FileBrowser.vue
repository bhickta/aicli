<script setup lang="ts">
import { onMounted, shallowRef, watch } from "vue";
import { api } from "../../lib/api";
import { appState } from "../../stores/appState";
import type { BrowserEntry, WorkflowField } from "../../types";

const props = defineProps<{
  field: WorkflowField | null;
}>();

const emit = defineEmits<{
  select: [id: string, path: string];
  close: [];
}>();

const currentPath = shallowRef("");
const entries = shallowRef<BrowserEntry[]>([]);
const status = shallowRef("Loading files...");

onMounted(() => void load(""));
watch(() => props.field, () => {
  if (props.field) void load(appState.browserPath);
});

async function load(path: string) {
  status.value = "Loading files...";
  const query = path ? `?path=${encodeURIComponent(path)}` : "";
  try {
    const result = await api<{ path: string; entries: BrowserEntry[] }>(`/api/fs/list${query}`);
    currentPath.value = result.path;
    appState.browserPath = result.path;
    entries.value = result.entries || [];
    status.value = "";
  } catch (error) {
    entries.value = [];
    status.value = error instanceof Error ? error.message : "File browser failed";
  }
}

function select(path: string) {
  if (!props.field?.id) return;
  emit("select", props.field.id, path);
}
</script>

<template>
  <div v-if="field" id="file-browser" class="browser" aria-live="polite">
    <div class="browser-header">
      <strong>Choose {{ field.label || field.id }}</strong>
      <div>
        <button type="button" @click="select(currentPath)">Use current directory</button>
        <button type="button" @click="emit('close')">Close</button>
      </div>
    </div>
    <code>{{ currentPath }}</code>
    <p v-if="status" class="muted">{{ status }}</p>
    <div v-else class="browser-list">
      <button
        v-for="entry in entries"
        :key="entry.path"
        type="button"
        class="browser-entry"
        :class="{ dir: entry.is_dir, file: !entry.is_dir }"
        @click="entry.is_dir ? load(entry.path) : select(entry.path)"
        @dblclick="select(entry.path)"
      >
        {{ entry.is_dir ? ">" : "-" }} {{ entry.name }}
      </button>
    </div>
  </div>
</template>
