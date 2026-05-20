<script setup lang="ts">
import { computed, shallowRef } from "vue";

const props = defineProps<{
  activePath: string;
  notes: string[];
  status: string;
  busy: boolean;
}>();

const emit = defineEmits<{
  updateActivePath: [path: string];
  load: [];
}>();

const query = shallowRef("");

const filteredNotes = computed(() => {
  const terms = query.value.toLowerCase().split(/\s+/).filter(Boolean);
  if (!terms.length) return props.notes.slice(0, 80);
  return props.notes
    .filter((note) => {
      const value = note.toLowerCase();
      return terms.every((term) => value.includes(term));
    })
    .slice(0, 80);
});

function updateActivePath(path: string) {
  emit("updateActivePath", path);
}
</script>

<template>
  <div class="zettel-note-picker">
    <div class="field">
      <label for="zettel-active">Active note path</label>
      <div class="path-control">
        <input
          id="zettel-active"
          :value="activePath"
          type="text"
          placeholder="zettelkasten/.../Note.md"
          @input="updateActivePath(($event.target as HTMLInputElement).value)"
        >
        <button type="button" :disabled="busy" @click="emit('load')">Load notes</button>
      </div>
    </div>

    <div class="note-picker-panel">
      <div class="field">
        <label for="zettel-note-filter">Select note</label>
        <input id="zettel-note-filter" v-model="query" type="search" placeholder="Filter notes">
      </div>
      <p class="muted">{{ status || `${notes.length} note(s) available` }}</p>
      <div v-if="filteredNotes.length" class="note-picker-list" role="listbox" aria-label="Zettelkasten notes">
        <button
          v-for="note in filteredNotes"
          :key="note"
          type="button"
          class="note-picker-item"
          :class="{ selected: note === activePath }"
          @click="updateActivePath(note)"
        >
          {{ note }}
        </button>
      </div>
      <p v-else class="muted">No matching notes loaded.</p>
    </div>
  </div>
</template>

<style scoped>
.zettel-note-picker {
  display: grid;
  gap: 10px;
}

.note-picker-panel {
  display: grid;
  gap: 8px;
}

.note-picker-list {
  display: grid;
  gap: 4px;
  max-height: 220px;
  overflow: auto;
  padding: 2px;
}

.note-picker-item {
  justify-content: flex-start;
  overflow-wrap: anywhere;
  text-align: left;
}

.note-picker-item.selected {
  border-color: #6ea8fe;
  color: #eceff4;
}
</style>
