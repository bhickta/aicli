<script setup lang="ts">
import { computed } from "vue";
import type { TrainingExportReport } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";
import ZettelSection from "../common/ZettelSection.vue";

const props = defineProps<{
  busy: boolean;
  canRun: boolean;
  report: TrainingExportReport | null;
}>();

const emit = defineEmits<{
  run: [];
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
</script>

<template>
  <ZettelSection
    title="Training data"
    description="Export accepted merge calls as clean chat JSONL for local fine-tuning."
  >
    <template #actions>
      <button type="button" class="mod-cta" :disabled="!canRun" @click="emit('run')">
        Export Clean Dataset
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

      <div class="training-paths" aria-label="Training export files">
        <div>
          <span>Train JSONL</span>
          <code>{{ report.train_path }}</code>
        </div>
        <div>
          <span>Eval JSONL</span>
          <code>{{ report.eval_path }}</code>
        </div>
        <div>
          <span>Manifest</span>
          <code>{{ report.manifest_path }}</code>
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
