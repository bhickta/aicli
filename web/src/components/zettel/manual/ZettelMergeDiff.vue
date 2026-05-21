<script setup lang="ts">
import { computed } from "vue";

interface Insertion {
  after_line: number;
  markdown: string;
  reason?: string;
}

interface DiffRow {
  key: string;
  kind: "context" | "added" | "gap";
  oldLine?: number;
  newLine?: number;
  marker: string;
  text: string;
  reason?: string;
}

const props = defineProps<{
  originalMarkdown: string;
  finalMarkdown: string;
  insertions: Insertion[];
}>();

const normalizedInsertions = computed(() =>
  [...(props.insertions || [])]
    .filter((insertion) => insertion.markdown.trim())
    .sort((a, b) => a.after_line - b.after_line)
);
const rows = computed(() => buildInsertionDiff(props.originalMarkdown, normalizedInsertions.value));
const addedLineCount = computed(() => rows.value.filter((row) => row.kind === "added").length);
const hasDiff = computed(() => normalizedInsertions.value.length > 0);
const finalLineCount = computed(() => splitLines(props.finalMarkdown).length);

function buildInsertionDiff(originalMarkdown: string, insertions: Insertion[]) {
  const originalLines = splitLines(originalMarkdown);
  if (!insertions.length) return [];

  const out: DiffRow[] = [];
  let oldLine = 1;
  let newLine = 1;
  const contextRadius = 3;

  insertions.forEach((insertion, insertionIndex) => {
    const afterLine = clamp(Math.trunc(insertion.after_line || 0), 0, originalLines.length);
    const contextStart = Math.max(oldLine, afterLine - contextRadius + 1);
    if (contextStart > oldLine) {
      const skipped = contextStart - oldLine;
      out.push(gapRow(`gap-before-${insertionIndex}`, skipped));
      oldLine += skipped;
      newLine += skipped;
    }

    while (oldLine <= afterLine) {
      out.push(contextRow(oldLine, newLine, originalLines[oldLine - 1]));
      oldLine++;
      newLine++;
    }

    splitLines(insertion.markdown).forEach((line, lineIndex) => {
      out.push({
        key: `add-${insertionIndex}-${lineIndex}`,
        kind: "added",
        newLine,
        marker: "+",
        text: line,
        reason: lineIndex === 0 ? insertion.reason || `Inserted after line ${afterLine}` : "",
      });
      newLine++;
    });

    const contextEnd = Math.min(originalLines.length, afterLine + contextRadius);
    while (oldLine <= contextEnd) {
      out.push(contextRow(oldLine, newLine, originalLines[oldLine - 1]));
      oldLine++;
      newLine++;
    }
  });

  if (oldLine <= originalLines.length) {
    out.push(gapRow("gap-tail", originalLines.length - oldLine + 1));
  }
  return out;
}

function contextRow(oldLine: number, newLine: number, text: string): DiffRow {
  return {
    key: `ctx-${oldLine}-${newLine}`,
    kind: "context",
    oldLine,
    newLine,
    marker: " ",
    text,
  };
}

function gapRow(key: string, count: number): DiffRow {
  return {
    key,
    kind: "gap",
    marker: "",
    text: count === 1 ? "1 unchanged line hidden" : `${count} unchanged lines hidden`,
  };
}

function splitLines(value: string) {
  const normalized = String(value || "").replace(/\r\n/g, "\n").replace(/\n$/, "");
  if (!normalized) return [];
  return normalized.split("\n");
}

function clamp(value: number, low: number, high: number) {
  if (value < low) return low;
  if (value > high) return high;
  return value;
}
</script>

<template>
  <section class="merge-diff" aria-label="Merge diff">
    <header class="merge-diff-header">
      <div>
        <h4>Inline Diff</h4>
        <p>{{ hasDiff ? `+${addedLineCount} line(s) across ${normalizedInsertions.length} insertion(s)` : "No insertion operations returned" }}</p>
      </div>
      <span>{{ finalLineCount }} final line(s)</span>
    </header>

    <div v-if="hasDiff" class="merge-diff-table">
      <div v-for="row in rows" :key="row.key" class="merge-diff-row" :class="`merge-diff-row-${row.kind}`">
        <template v-if="row.kind === 'gap'">
          <span class="gap-text">{{ row.text }}</span>
        </template>
        <template v-else>
          <span class="line-number">{{ row.oldLine || "" }}</span>
          <span class="line-number">{{ row.newLine || "" }}</span>
          <span class="line-marker">{{ row.marker }}</span>
          <code>{{ row.text || " " }}</code>
          <small v-if="row.reason">{{ row.reason }}</small>
        </template>
      </div>
    </div>

    <pre v-else class="merge-diff-empty">{{ finalMarkdown || "No merge preview available." }}</pre>
  </section>
</template>

<style scoped>
.merge-diff {
  display: grid;
  gap: 10px;
  min-width: 0;
}

.merge-diff-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 10px;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #10141b;
}

.merge-diff-header h4,
.merge-diff-header p {
  margin: 0;
}

.merge-diff-header p,
.merge-diff-header span {
  color: #9aa4b2;
  font-size: 12px;
}

.merge-diff-table {
  display: grid;
  max-height: 640px;
  overflow: auto;
  border: 1px solid #2b313b;
  border-radius: 6px;
  background: #0d1015;
}

.merge-diff-row {
  display: grid;
  grid-template-columns: 46px 46px 24px minmax(0, 1fr);
  align-items: start;
  min-width: 0;
  border-bottom: 1px solid #202630;
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 13px;
}

.merge-diff-row:last-child {
  border-bottom: 0;
}

.line-number,
.line-marker {
  min-height: 26px;
  padding: 5px 6px;
  color: #7d8795;
  text-align: right;
  user-select: none;
}

.line-marker {
  text-align: center;
}

.merge-diff-row code {
  min-height: 26px;
  padding: 5px 8px;
  border: 0;
  border-radius: 0;
  background: transparent;
  white-space: pre-wrap;
}

.merge-diff-row small {
  grid-column: 4;
  padding: 0 8px 6px;
  color: #7fb1ff;
}

.merge-diff-row-added {
  background: #12301f;
}

.merge-diff-row-added .line-marker,
.merge-diff-row-added code {
  color: #b7f5c8;
}

.merge-diff-row-gap {
  grid-template-columns: 1fr;
  padding: 6px 10px;
  color: #8792a2;
  background: #111821;
  font-family: ui-sans-serif, system-ui, sans-serif;
  font-size: 12px;
}

.merge-diff-empty {
  min-height: 180px;
}

@media (max-width: 760px) {
  .merge-diff-header {
    flex-direction: column;
  }

  .merge-diff-row {
    grid-template-columns: 36px 36px 22px minmax(0, 1fr);
    font-size: 12px;
  }
}
</style>
