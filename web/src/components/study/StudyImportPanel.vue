<script setup lang="ts">
defineProps<{
  importPath: string;
  importFolder: string;
}>();

const emit = defineEmits<{
  "update:importPath": [value: string];
  "update:importFolder": [value: string];
  import: [];
}>();
</script>

<template>
  <section class="study-card study-import">
    <div>
      <h2>Import PDFs</h2>
      <p>Track answer-copy PDFs first, then run OCR, question split, and analysis in focused stages.</p>
    </div>
    <label>
      PDF paths
      <textarea
        :value="importPath"
        rows="4"
        placeholder="/home/.../copy.pdf"
        @input="emit('update:importPath', ($event.target as HTMLTextAreaElement).value)"
      ></textarea>
    </label>
    <label>
      Folder path
      <input
        :value="importFolder"
        type="text"
        placeholder="/home/.../answer-copies"
        @input="emit('update:importFolder', ($event.target as HTMLInputElement).value)"
      />
    </label>
    <button class="primary" type="button" @click="emit('import')">Import</button>
  </section>
</template>
