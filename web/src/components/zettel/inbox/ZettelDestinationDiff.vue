<script setup lang="ts">
import "../../../styles/zettel-destination-diff.css";
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
