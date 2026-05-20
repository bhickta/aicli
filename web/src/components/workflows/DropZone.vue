<script setup lang="ts">
import { shallowRef } from "vue";
import { collectDroppedFiles, type DropEntry } from "../../workflows/drop";

const emit = defineEmits<{
  upload: [entries: DropEntry[]];
}>();

const dragging = shallowRef(false);
const uploading = shallowRef(false);

function stop(event: DragEvent) {
  event.preventDefault();
  event.stopPropagation();
}

async function drop(event: DragEvent) {
  stop(event);
  dragging.value = false;
  uploading.value = true;
  try {
    const entries = await collectDroppedFiles(event.dataTransfer);
    if (entries.length) emit("upload", entries);
  } finally {
    uploading.value = false;
  }
}
</script>

<template>
  <div
    id="drop-zone"
    class="drop-zone"
    :class="{ dragover: dragging, uploading }"
    tabindex="0"
    @dragenter="stop($event); dragging = true"
    @dragover="stop($event); dragging = true"
    @dragleave="stop($event); dragging = false"
    @drop="drop"
  >
    <strong>Drop a file to auto-select workflow</strong>
    <span>Single files upload to app storage. For in-place video course folders, use Choose Course source folder.</span>
  </div>
</template>
