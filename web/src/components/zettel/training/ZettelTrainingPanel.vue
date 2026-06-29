<script setup lang="ts">
import "../../../styles/zettel-training-panel.css";
import { computed } from "vue";
import type { TrainingExportReport } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";
import ZettelSection from "../common/ZettelSection.vue";

const props = defineProps<{
  busy: boolean;
  canRun: boolean;
  strict: boolean;
  report: TrainingExportReport | null;
}>();

const emit = defineEmits<{
  run: [];
  updateStrict: [value: boolean];
}>();

const badges = computed(() => {
  if (!props.report) return [];
  return [
    `${props.report.train_count} train`,
    `${props.report.eval_count} eval`,
    `${props.report.exported_count} clean`,
    `${props.report.skipped_count} skipped`,
    `${props.report.duplicate_count} duplicates`,
  ];
});

const skippedReasons = computed(() => {
  const reasons = props.report?.skipped_by_reason || {};
  return Object.entries(reasons)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([reason, count]) => ({ reason, count }));
});

const exportPaths = computed(() => {
  if (!props.report) return [];
  return [
    { label: "Train JSONL", path: props.report.train_path },
    { label: "Eval JSONL", path: props.report.eval_path },
    { label: "ShareGPT train", path: props.report.sharegpt_train_path },
    { label: "ShareGPT eval", path: props.report.sharegpt_eval_path },
    { label: "Manifest", path: props.report.manifest_path },
  ].filter((item): item is { label: string; path: string } => Boolean(item.path));
});

const qualityRows = computed(() => {
  const quality = props.report?.quality;
  if (!quality) return [];
  return [
    ["Prompt variants", quality.system_prompt_variants],
    ["Primary prompt examples", quality.primary_system_prompt_count],
    ["Semantic candidate examples", quality.examples_with_semantic_candidates],
    ["No candidate examples", quality.examples_without_semantic_candidates],
    ["Code fence outputs", quality.examples_with_code_fences],
    ["Duplicate frontmatter", quality.examples_with_duplicate_frontmatter],
    ["Bad note boundaries", quality.examples_with_bad_note_boundaries],
    ["Status/JSON outputs", quality.examples_with_status_or_json_output],
    ["Final notes", quality.total_final_notes],
    ["Max notes/example", quality.max_final_notes_per_example],
    ["Avg prompt chars", quality.average_user_chars],
    ["Avg answer chars", quality.average_assistant_chars],
  ];
});

function emitStrictChange(event: Event) {
  emit("updateStrict", (event.target as HTMLInputElement).checked);
}
</script>

<template>
  <ZettelSection
    title="Training data"
    description="Export accepted merge calls as clean chat JSONL for local fine-tuning."
  >
    <template #actions>
      <label class="strict-toggle">
        <input
          type="checkbox"
          :checked="strict"
          :disabled="busy"
          @change="emitStrictChange"
        />
        Strict
      </label>
      <button type="button" class="mod-cta" :disabled="!canRun" @click="emit('run')">
        Export Dataset
      </button>
    </template>

    <section v-if="report" class="training-report">
      <header class="training-report-header">
        <div>
          <h3>Clean Merge Dataset</h3>
          <p>{{ report.run_id }}</p>
        </div>
        <ZettelBadges :items="badges" />
      </header>

      <div v-if="qualityRows.length" class="quality-grid" aria-label="Training export quality">
        <div v-for="[label, value] in qualityRows" :key="label">
          <span>{{ label }}</span>
          <strong>{{ value }}</strong>
        </div>
      </div>

      <div class="training-paths" aria-label="Training export files">
        <div v-for="item in exportPaths" :key="item.label">
          <span>{{ item.label }}</span>
          <code>{{ item.path }}</code>
        </div>
      </div>

      <details v-if="skippedReasons.length" class="training-skips">
        <summary>Skipped reasons</summary>
        <dl>
          <template v-for="item in skippedReasons" :key="item.reason">
            <dt>{{ item.reason }}</dt>
            <dd>{{ item.count }}</dd>
          </template>
        </dl>
      </details>
    </section>

    <p v-else class="muted">Clean dataset exports appear here after a run.</p>
  </ZettelSection>
</template>
