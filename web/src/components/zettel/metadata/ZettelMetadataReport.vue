<script setup lang="ts">
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

<style scoped>
.metadata-report {
  display: grid;
  gap: 12px;
}

.metadata-report-header {
  margin: 0;
}

.metadata-report-badges {
  display: grid;
  gap: 6px;
  justify-items: end;
}

.metadata-workbench {
  display: grid;
  grid-template-columns: minmax(240px, 360px) minmax(0, 1fr);
  min-height: 520px;
  min-width: 0;
  overflow: hidden;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #0d1117;
}

.metadata-note-list,
.metadata-detail {
  display: grid;
  min-width: 0;
}

.metadata-note-list {
  align-content: start;
  overflow: auto;
  border-right: 1px solid #2b313b;
  background: #11161f;
}

.note-list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  min-height: 38px;
  padding: 8px 10px;
  border-bottom: 1px solid #2b313b;
}

.note-list-header span {
  color: #9aa4b2;
  font-size: 12px;
}

.note-row {
  display: grid;
  grid-template-columns: 74px minmax(0, 1fr);
  gap: 8px;
  width: 100%;
  padding: 10px;
  border: 0;
  border-bottom: 1px solid #202632;
  border-radius: 0;
  background: transparent;
  color: #d6deea;
  text-align: left;
}

.note-row:hover,
.note-row.active {
  background: #1f3554;
}

.note-state {
  align-self: start;
  padding-top: 2px;
  color: #9aa4b2;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.note-state.is-processed {
  color: #7ee787;
}

.note-state.is-failed {
  color: #ff7b72;
}

.note-state.is-skipped {
  color: #f2cc60;
}

.note-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.note-copy strong,
.note-copy small,
.note-copy em {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.note-copy small,
.note-copy em {
  color: #9aa4b2;
}

.note-copy em {
  font-size: 12px;
  font-style: normal;
}

.metadata-detail {
  align-content: start;
  gap: 12px;
  padding: 12px;
  overflow: auto;
  background: #10141b;
}

.note-summary {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin: 0;
  padding: 0;
}

.note-summary strong,
.metadata-field strong,
.metadata-field p {
  overflow-wrap: anywhere;
}

.note-reason {
  margin: 0;
  color: #f2cc60;
}

.metadata-card {
  display: grid;
  gap: 10px;
  padding: 12px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #161b22;
}

.metadata-field {
  display: grid;
  gap: 4px;
}

.metadata-field span {
  color: #9aa4b2;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.metadata-field p,
.metadata-field ol {
  margin: 0;
}

.metadata-field ol {
  padding-left: 20px;
}

.diff-panel {
  overflow: hidden;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #0d1117;
}

.diff-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 10px;
  border-bottom: 1px solid #2b313b;
}

.diff-header span {
  overflow: hidden;
  color: #9aa4b2;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.diff-panel pre {
  max-height: 520px;
  margin: 0;
  overflow: auto;
  font-size: 12px;
  line-height: 1.5;
}

.diff-panel code {
  display: grid;
}

.diff-panel span {
  min-height: 18px;
  padding: 0 10px;
  white-space: pre-wrap;
}

.diff-panel .is-header {
  color: #9aa4b2;
  background: #11161f;
}

.diff-panel .is-added {
  color: #b9f6ca;
  background: rgba(46, 160, 67, 0.22);
}

.diff-panel .is-removed {
  color: #ffd7d5;
  background: rgba(248, 81, 73, 0.18);
}

.diff-panel .is-context {
  color: #d6deea;
}

@media (max-width: 820px) {
  .metadata-workbench {
    grid-template-columns: 1fr;
  }

  .metadata-note-list {
    max-height: 260px;
    border-right: 0;
    border-bottom: 1px solid #2b313b;
  }

  .note-summary {
    display: grid;
  }
}
</style>
