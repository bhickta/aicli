<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { api } from "../../lib/api";
import { copyMetadata, copyMetadataChips } from "../../lib/studyMetadata";
import type { StudyCopyRecord } from "../../types";
import { useToasts } from "../../composables/useToasts";
import StudyMetadataChips from "./StudyMetadataChips.vue";

const props = defineProps<{
  copy: StudyCopyRecord;
}>();

const emit = defineEmits<{
  saved: [];
}>();

const toasts = useToasts();
const editing = ref(false);
const saving = ref(false);
const form = ref(copyForm(props.copy));
const metadata = computed(() => copyMetadata(props.copy));
const chips = computed(() => copyMetadataChips(props.copy));
const rows = computed(() => [
  ["Topper", metadata.value?.topper_name || props.copy.candidate_name],
  ["Rank", metadata.value?.rank],
  ["Institute", metadata.value?.coaching_institute],
  ["Series", metadata.value?.test_series],
  ["Language", metadata.value?.language],
]);
const visibleRows = computed(() => rows.value.filter(([, value]) => value));

watch(
  () => props.copy,
  (copy) => {
    form.value = copyForm(copy);
    editing.value = false;
  },
);

async function save() {
  saving.value = true;
  try {
    await api(`/api/study/copies/${encodeURIComponent(props.copy.id)}`, {
      method: "PUT",
      body: JSON.stringify({ ...props.copy, ...form.value }),
    });
    editing.value = false;
    emit("saved");
    toasts.success("Metadata saved", "Copy metadata has been updated.");
  } catch (error) {
    toasts.error("Save failed", error instanceof Error ? error.message : String(error));
  } finally {
    saving.value = false;
  }
}

function copyForm(copy: StudyCopyRecord) {
  return {
    pdf_name: copy.pdf_name || "",
    candidate_name: copy.candidate_name || "",
    paper: copy.paper || "",
    test_code: copy.test_code || "",
    copy_date: copy.copy_date || "",
    roll_no: copy.roll_no || "",
  };
}
</script>

<template>
  <section class="study-copy-metadata-panel">
    <div class="study-copy-metadata-head">
      <StudyMetadataChips :chips="chips" />
      <button v-if="!editing" type="button" class="study-btn-action" @click="editing = true">Edit Details</button>
      <button v-else type="button" class="study-btn-action primary" :disabled="saving" @click="save">
        {{ saving ? "Saving..." : "Save Details" }}
      </button>
    </div>

    <div class="study-meta-grid">
      <label>
        PDF / Copy Name
        <input v-model="form.pdf_name" :readonly="!editing" />
      </label>
      <label>
        Candidate
        <input v-model="form.candidate_name" :readonly="!editing" />
      </label>
      <label>
        Paper
        <input v-model="form.paper" :readonly="!editing" />
      </label>
      <label>
        Test code
        <input v-model="form.test_code" :readonly="!editing" />
      </label>
      <label>
        Date
        <input v-model="form.copy_date" :readonly="!editing" />
      </label>
      <label>
        Roll no.
        <input v-model="form.roll_no" :readonly="!editing" />
      </label>
    </div>

    <dl v-if="visibleRows.length" class="study-copy-metadata-facts">
      <div v-for="[label, value] in visibleRows" :key="label">
        <dt>{{ label }}</dt>
        <dd>{{ value }}</dd>
      </div>
    </dl>
  </section>
</template>
