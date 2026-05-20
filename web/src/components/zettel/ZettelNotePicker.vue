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
    <div class="selected-note">
      <div>
        <span>Active note</span>
        <strong>{{ activePath || "Choose from loaded notes" }}</strong>
      </div>
      <button type="button" :disabled="busy" @click="emit('load')">Load notes</button>
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

.selected-note {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 10px;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.selected-note div {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.selected-note span {
  color: #9aa4b2;
  font-size: 12px;
}

.selected-note strong {
  overflow-wrap: anywhere;
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

@media (max-width: 640px) {
  .selected-note {
    grid-template-columns: 1fr;
  }
}
</style>
