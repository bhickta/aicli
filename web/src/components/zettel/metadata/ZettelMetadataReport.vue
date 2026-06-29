<script setup lang="ts">
import "../../../styles/zettel-metadata-report.css";
import { computed, shallowRef } from "vue";
import type { MetadataReport } from "../../../types";
import ZettelBadges from "../common/ZettelBadges.vue";

const props = defineProps<{
  report: MetadataReport | null;
}>();

const selectedPath = shallowRef("");
const rows = computed(() => {
  if (!props.report) return [];
  return [
    ...(props.report.processed || []),
    ...(props.report.skipped || []),
    ...(props.report.failed || []),
  ];
});
const selected = computed(() => rows.value.find((row) => row.path === selectedPath.value) || rows.value[0] || null);
const reportBadges = computed(() => {
  if (!props.report) return [];
  const skippedItems = props.report.skipped?.length || 0;
  return [
    `${props.report.selected_count ?? rows.value.length} selected`,
    `${props.report.source_count ?? 0} notes found`,
    `${props.report.skipped_count ?? 0} not selected`,
    `${props.report.processed_count} updated`,
    `${skippedItems} skipped`,
    `${props.report.failed_count} failed`,
  ];
});
const apiBadges = computed(() => {
  if (!props.report?.api_calls) return [];
  return [
    `${props.report.api_calls.chat} chat`,
    `${props.report.api_calls.total} calls`,
  ];
});
const diffLines = computed(() => (selected.value?.diff?.diff || "").split("\n").filter(Boolean));

function selectRow(path: string) {
  selectedPath.value = path;
}

function fileName(path: string): string {
  return path.split("/").pop() || path;
}

function parentPath(path: string): string {
  const parts = path.split("/");
  parts.pop();
  return parts.join("/");
}

function diffLineClass(line: string) {
  if (line.startsWith("+++") || line.startsWith("---")) return "is-header";
  if (line.startsWith("+")) return "is-added";
  if (line.startsWith("-")) return "is-removed";
  return "is-context";
}
</script>

<template>
  <section v-if="report" class="metadata-report">
    <header class="metadata-report-header">
      <div>
        <h3>Metadata Audit</h3>
        <p class="muted">{{ report.run_id }}</p>
      </div>
      <div class="metadata-report-badges">
        <ZettelBadges :items="reportBadges" />
        <ZettelBadges v-if="apiBadges.length" :items="apiBadges" />
      </div>
    </header>

    <div class="metadata-workbench">
      <aside class="metadata-note-list" aria-label="Metadata notes">
        <div class="note-list-header">
          <strong>Notes</strong>
          <span>{{ rows.length }}</span>
        </div>
        <button
          v-for="row in rows"
          :key="row.path"
          type="button"
          class="note-row"
          :class="{ active: selected?.path === row.path }"
          @click="selectRow(row.path)"
        >
          <span class="note-state" :class="`is-${row.status}`">{{ row.status }}</span>
          <span class="note-copy">
            <strong>{{ fileName(row.path) }}</strong>
            <small>{{ parentPath(row.path) }}</small>
            <em v-if="row.title">{{ row.title }}</em>
            <em v-else-if="row.reason">{{ row.reason }}</em>
          </span>
        </button>
      </aside>

      <article v-if="selected" class="metadata-detail">
        <header class="note-summary">
          <div>
            <strong>{{ selected.path }}</strong>
            <p class="status-line compact">{{ selected.status }}</p>
          </div>
          <ZettelBadges :items="[selected.diff ? 'changed' : 'unchanged']" />
        </header>
        <p v-if="selected.reason" class="note-reason">{{ selected.reason }}</p>

        <section v-if="selected.title || selected.summary_keywords || selected.recall_questions?.length" class="metadata-card">
          <div v-if="selected.title" class="metadata-field">
            <span>Title</span>
            <strong>{{ selected.title }}</strong>
          </div>
          <div v-if="selected.summary_keywords" class="metadata-field">
            <span>Summary keywords</span>
            <p>{{ selected.summary_keywords }}</p>
          </div>
          <div v-if="selected.recall_questions?.length" class="metadata-field">
            <span>Recall prompts</span>
            <ol>
              <li v-for="question in selected.recall_questions" :key="question">{{ question }}</li>
            </ol>
          </div>
        </section>

        <section v-if="diffLines.length" class="diff-panel" aria-label="Metadata diff">
          <div class="diff-header">
            <strong>Diff</strong>
            <span>{{ selected.path }}</span>
          </div>
          <pre><code><span
            v-for="(line, index) in diffLines"
            :key="`${index}-${line}`"
            :class="diffLineClass(line)"
          >{{ line }}
</span></code></pre>
        </section>
      </article>
    </div>
  </section>
</template>
