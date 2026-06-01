<script setup lang="ts">
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

<style scoped>
.mod-cta {
  border-color: #6ea8fe;
  background: #2d405e;
}

.strict-toggle {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  color: #c9d1d9;
  font-size: 13px;
  font-weight: 700;
}

.training-report {
  display: grid;
  gap: 14px;
  padding: 14px;
  border: 1px solid #2b313b;
  border-radius: 8px;
  background: #10141b;
}

.training-report-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.training-report-header h3 {
  margin: 0;
}

.training-report-header p {
  margin: 5px 0 0;
  color: #9aa4b2;
  font-size: 13px;
}

.training-paths {
  display: grid;
  gap: 8px;
}

.quality-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
  gap: 8px;
}

.quality-grid div {
  display: grid;
  gap: 4px;
  padding: 9px;
  border: 1px solid #2d3440;
  border-radius: 6px;
  background: #0d1117;
}

.quality-grid span {
  color: #9aa4b2;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.quality-grid strong {
  color: #e6edf3;
  font-size: 16px;
  font-variant-numeric: tabular-nums;
}

.training-paths div {
  display: grid;
  gap: 5px;
}

.training-paths span,
.training-skips summary {
  color: #9aa4b2;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.training-paths code {
  overflow-wrap: anywhere;
  padding: 8px;
  border: 1px solid #343b46;
  border-radius: 6px;
  background: #0d1117;
  color: #d8dee9;
}

.training-skips {
  border-top: 1px solid #252b34;
  padding-top: 10px;
}

.training-skips dl {
  display: grid;
  grid-template-columns: minmax(160px, 1fr) auto;
  gap: 6px 12px;
  margin: 10px 0 0;
}

.training-skips dt,
.training-skips dd {
  margin: 0;
}

.training-skips dd {
  color: #d8dee9;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 760px) {
  .training-report-header {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
