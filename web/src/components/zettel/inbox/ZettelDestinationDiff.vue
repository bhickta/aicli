<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef, watch } from "vue";
import type { InboxDestinationDiff } from "../../../types";
import { buildDiffRows } from "../../../features/zettel/diff";

const props = defineProps<{
  diffs: InboxDestinationDiff[];
  sourcePath?: string;
  sourceContent?: string;
  processedPath?: string;
}>();

const selectedPath = shallowRef("");
const isExpanded = shallowRef(false);
let previousBodyOverflow = "";
const selectedDiff = computed(() => {
  const diffs = props.diffs || [];
  return diffs.find((diff) => diff.path === selectedPath.value) || diffs[0] || null;
});
const rows = computed(() => selectedDiff.value ? buildDiffRows(selectedDiff.value) : []);
const selectedFileName = computed(() => selectedDiff.value ? fileName(selectedDiff.value.path) : "");
const selectedParentPath = computed(() => selectedDiff.value ? parentPath(selectedDiff.value.path) : "");
const sourceFileName = computed(() => props.sourcePath ? fileName(props.sourcePath) : "Source note");
const sourceParentPath = computed(() => props.sourcePath ? parentPath(props.sourcePath) : "");
const stats = computed(() => {
  let added = 0;
  let removed = 0;
  for (const row of rows.value) {
    if (row.afterText && row.type !== "same") added++;
    if (row.beforeText && row.type !== "same") removed++;
  }
  return { added, removed };
});

watch(
  () => props.diffs,
  (diffs) => {
    if (!diffs.length) {
      selectedPath.value = "";
      return;
    }
    if (!diffs.some((diff) => diff.path === selectedPath.value)) {
      selectedPath.value = diffs[0].path;
    }
  },
  { immediate: true },
);

watch(isExpanded, (expanded) => {
  if (expanded) {
    previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return;
  }
  document.body.style.overflow = previousBodyOverflow;
});

onMounted(() => {
  window.addEventListener("keydown", closeOnEscape);
});

onUnmounted(() => {
  window.removeEventListener("keydown", closeOnEscape);
  document.body.style.overflow = previousBodyOverflow;
});

function selectDiff(path: string) {
  selectedPath.value = path;
}

function toggleExpanded() {
  isExpanded.value = !isExpanded.value;
}

function closeOnEscape(event: KeyboardEvent) {
  if (event.key === "Escape") isExpanded.value = false;
}

function fileName(path: string): string {
  return path.split("/").pop() || path;
}

function parentPath(path: string): string {
  const parts = path.split("/");
  parts.pop();
  return parts.join("/");
}
</script>

<template>
  <section
    v-if="diffs.length"
    class="destination-diff"
    :class="{ 'is-expanded': isExpanded }"
    :aria-modal="isExpanded ? 'true' : undefined"
  >
    <aside class="merge-map" aria-label="Source and merge destinations">
      <div class="merge-map-header">
        <strong>Source merge</strong>
        <span>{{ diffs.length }} destination{{ diffs.length === 1 ? "" : "s" }}</span>
      </div>

      <article class="source-note">
        <span class="source-label">Source</span>
        <strong>{{ sourceFileName }}</strong>
        <small v-if="sourceParentPath">{{ sourceParentPath }}</small>
        <small v-if="processedPath" class="processed-path">processed -> {{ processedPath }}</small>
        <details class="source-content" open>
          <summary>Source content</summary>
          <pre>{{ sourceContent || "Source content is available for new merge runs only." }}</pre>
        </details>
      </article>

      <div class="merge-arrow">Merged into</div>

      <div class="destination-files" aria-label="Changed destination files">
        <div class="destination-files-header">
          <strong>Changed destinations</strong>
          <span>{{ diffs.length }}</span>
        </div>
        <button
          v-for="diff in diffs"
          :key="diff.path"
          type="button"
          class="destination-file"
          :class="{ active: selectedDiff?.path === diff.path }"
          @click="selectDiff(diff.path)"
        >
          <span class="file-state">{{ diff.created ? "A" : "M" }}</span>
          <span class="file-copy">
            <strong>{{ fileName(diff.path) }}</strong>
            <small>{{ parentPath(diff.path) }}</small>
          </span>
        </button>
      </div>
    </aside>

    <article v-if="selectedDiff" class="diff-editor" aria-label="Destination diff">
      <header class="diff-editor-header">
        <div class="diff-title">
          <strong>{{ selectedFileName }}</strong>
          <small>{{ selectedParentPath }}</small>
        </div>
        <div class="diff-header-actions">
          <div class="diff-stats">
            <span class="added">+{{ stats.added }}</span>
            <span class="removed">-{{ stats.removed }}</span>
          </div>
          <button
            type="button"
            class="fullscreen-button"
            :aria-pressed="isExpanded"
            @click="toggleExpanded"
          >
            {{ isExpanded ? "Exit full screen" : "Full screen" }}
          </button>
        </div>
      </header>

      <div class="diff-columns">
        <div class="diff-pane">
          <div class="diff-pane-title">Before</div>
          <div
            v-for="row in rows"
            :key="`${row.id}-before`"
            class="diff-line"
            :class="[`is-${row.type}`, { empty: !row.beforeText }]"
          >
            <span class="diff-gutter">{{ row.beforeNumber || "" }}</span>
            <code class="diff-code">{{ row.beforeText }}</code>
          </div>
        </div>
        <div class="diff-pane">
          <div class="diff-pane-title">After</div>
          <div
            v-for="row in rows"
            :key="`${row.id}-after`"
            class="diff-line"
            :class="[`is-${row.type}`, { empty: !row.afterText }]"
          >
            <span class="diff-gutter">{{ row.afterNumber || "" }}</span>
            <code class="diff-code">{{ row.afterText }}</code>
          </div>
        </div>
      </div>
    </article>
  </section>

  <p v-else class="muted">No changed destination diff was returned for this source.</p>
</template>

<style scoped>
.destination-diff {
  position: relative;
  display: grid;
  grid-template-columns: minmax(280px, 360px) minmax(0, 1fr);
  min-height: 420px;
  overflow: hidden;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #0d1117;
}

.destination-diff.is-expanded {
  position: fixed;
  inset: 12px;
  z-index: 1000;
  grid-template-columns: minmax(320px, 430px) minmax(0, 1fr);
  width: auto;
  height: calc(100vh - 24px);
  min-height: 0;
  box-shadow: 0 18px 80px rgba(0, 0, 0, 0.6);
}

.merge-map {
  display: grid;
  grid-template-rows: auto auto auto minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  border-right: 1px solid #2b313b;
  background: #11161f;
}

.merge-map-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 38px;
  padding: 8px 10px;
  border-bottom: 1px solid #2b313b;
}

.destination-files-header,
.diff-editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  min-height: 38px;
  margin: 0;
  padding: 8px 10px;
  border-bottom: 1px solid #2b313b;
}

.merge-map-header span {
  color: #9aa4b2;
  font-size: 12px;
}

.source-note {
  display: grid;
  gap: 5px;
  padding: 10px;
  border-bottom: 1px solid #2b313b;
}

.source-label {
  color: #7ee787;
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.source-note strong,
.source-note small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.processed-path {
  color: #9aa4b2;
}

.source-content {
  margin-top: 4px;
  min-width: 0;
}

.source-content summary {
  cursor: pointer;
  color: #d6deea;
  font-size: 12px;
}

.source-content pre {
  max-height: 180px;
  min-height: 92px;
  margin: 8px 0 0;
  padding: 8px;
  overflow: auto;
  border: 1px solid #2b313b;
  border-radius: 4px;
  background: #0d1117;
  color: #d6deea;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.45;
  white-space: pre-wrap;
}

.merge-arrow {
  padding: 7px 10px;
  border-bottom: 1px solid #2b313b;
  color: #9aa4b2;
  font-size: 12px;
  font-weight: 700;
  text-transform: uppercase;
}

.destination-files {
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.destination-files-header span {
  color: #9aa4b2;
  font-size: 12px;
}

.destination-file {
  display: grid;
  grid-template-columns: 22px minmax(0, 1fr);
  gap: 8px;
  width: 100%;
  padding: 9px 10px;
  border: 0;
  border-bottom: 1px solid #202632;
  border-radius: 0;
  background: transparent;
  color: #d6deea;
  text-align: left;
}

.destination-file:hover,
.destination-file.active {
  background: #1f3554;
}

.file-state {
  color: #6ea8fe;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  font-weight: 700;
}

.file-copy {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.file-copy strong,
.diff-title strong {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.file-copy small,
.diff-title small {
  overflow: hidden;
  color: #9aa4b2;
  font-size: 12px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.diff-editor {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
}

.diff-title {
  display: grid;
  gap: 2px;
  min-width: 0;
}

.diff-header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  flex: 0 0 auto;
}

.diff-stats {
  display: flex;
  gap: 8px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
}

.diff-stats .added {
  color: #7ee787;
}

.diff-stats .removed {
  color: #ff7b72;
}

.fullscreen-button {
  min-height: 30px;
  padding: 5px 10px;
  border-color: #343b46;
  background: #1f252e;
  color: #d6deea;
  white-space: nowrap;
}

.fullscreen-button:hover {
  border-color: #6ea8fe;
  background: #2d405e;
}

.diff-columns {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  min-height: 0;
  overflow: auto;
}

.diff-pane {
  min-width: 0;
  border-right: 1px solid #2b313b;
}

.diff-pane:last-child {
  border-right: 0;
}

.diff-pane-title {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 7px 10px;
  border-bottom: 1px solid #2b313b;
  background: #161b22;
  color: #9aa4b2;
  font-size: 12px;
  font-weight: 700;
}

.diff-line {
  display: grid;
  grid-template-columns: 46px minmax(0, 1fr);
  min-height: 22px;
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 12px;
  line-height: 1.45;
}

.diff-gutter {
  padding: 2px 8px;
  border-right: 1px solid #222936;
  color: #6e7681;
  text-align: right;
  user-select: none;
}

.diff-code {
  min-height: 22px;
  padding: 2px 8px;
  overflow-wrap: anywhere;
  border-radius: 0;
  background: transparent;
  color: #d6deea;
  white-space: pre-wrap;
}

.diff-line.is-deleted .diff-gutter,
.diff-line.is-changed:not(.empty) .diff-gutter {
  background: rgba(248, 81, 73, 0.12);
}

.diff-line.is-deleted .diff-code,
.diff-line.is-changed:not(.empty) .diff-code {
  background: rgba(248, 81, 73, 0.18);
}

.diff-pane:last-child .diff-line.is-inserted .diff-gutter,
.diff-pane:last-child .diff-line.is-changed:not(.empty) .diff-gutter {
  background: rgba(46, 160, 67, 0.12);
}

.diff-pane:last-child .diff-line.is-inserted .diff-code,
.diff-pane:last-child .diff-line.is-changed:not(.empty) .diff-code {
  background: rgba(46, 160, 67, 0.18);
}

.diff-line.empty .diff-code {
  background-image: repeating-linear-gradient(
    -45deg,
    rgba(110, 118, 129, 0.18) 0,
    rgba(110, 118, 129, 0.18) 1px,
    transparent 1px,
    transparent 6px
  );
}

@media (max-width: 980px) {
  .destination-diff,
  .diff-columns {
    grid-template-columns: 1fr;
  }

  .destination-diff.is-expanded {
    inset: 6px;
    height: calc(100vh - 12px);
    grid-template-columns: 1fr;
  }

  .merge-map {
    max-height: 320px;
    border-right: 0;
    border-bottom: 1px solid #2b313b;
  }

  .source-content pre {
    max-height: 120px;
  }
}
</style>
