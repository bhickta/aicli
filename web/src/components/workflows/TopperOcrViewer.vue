<script setup lang="ts">
import { computed, shallowRef } from "vue";

type OCRBlock =
  | { type: "heading"; text: string }
  | { type: "paragraph"; text: string }
  | { type: "list"; items: string[] }
  | { type: "table"; rows: string[][] }
  | { type: "diagram"; text: string };

const props = defineProps<{
  text: string;
  editable?: boolean;
}>();

const emit = defineEmits<{
  update: [value: string];
}>();

const mode = shallowRef<"rendered" | "raw">("rendered");
const blocks = computed(() => parseOCRBlocks(props.text || ""));

function parseOCRBlocks(text: string): OCRBlock[] {
  const out: OCRBlock[] = [];
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  let paragraph: string[] = [];
  let list: string[] = [];
  let table: string[][] = [];
  let diagram: string[] = [];
  let inMath = false;

  function flushParagraph() {
    if (!paragraph.length) return;
    out.push({ type: "paragraph", text: cleanInline(paragraph.join(" ")) });
    paragraph = [];
  }

  function flushList() {
    if (!list.length) return;
    out.push({ type: "list", items: list.map(cleanInline) });
    list = [];
  }

  function flushTable() {
    if (!table.length) return;
    out.push({ type: "table", rows: table });
    table = [];
  }

  function flushDiagram() {
    const text = diagram.map(cleanDiagramLine).filter(Boolean).join("\n").trim();
    if (text) out.push({ type: "diagram", text });
    diagram = [];
  }

  function flushAll() {
    flushParagraph();
    flushList();
    flushTable();
    flushDiagram();
  }

  for (const rawLine of lines) {
    const line = rawLine.trimEnd();
    const trimmed = line.trim();
    if (trimmed.startsWith("$$")) {
      flushAll();
      inMath = !inMath;
      const remainder = trimmed.replaceAll("$$", "").trim();
      if (remainder) diagram.push(remainder);
      continue;
    }
    if (inMath) {
      diagram.push(trimmed);
      continue;
    }
    if (!trimmed) {
      flushAll();
      continue;
    }
    if (isDiagramLine(trimmed)) {
      flushParagraph();
      flushList();
      flushTable();
      diagram.push(trimmed);
      continue;
    }
    flushDiagram();
    if (trimmed.startsWith("|") && trimmed.endsWith("|")) {
      flushParagraph();
      flushList();
      const cells = trimmed.split("|").slice(1, -1).map((cell) => cleanInline(cell.trim()));
      if (!cells.every((cell) => /^:?-{2,}:?$/.test(cell))) table.push(cells);
      continue;
    }
    flushTable();
    const listMatch = trimmed.match(/^([-*•]|\d+[.)])\s+(.*)$/);
    if (listMatch) {
      flushParagraph();
      list.push(listMatch[2] || "");
      continue;
    }
    flushList();
    if (/^#{1,4}\s+/.test(trimmed) || /^[A-Z][A-Z\s()/.-]{4,}$/.test(trimmed)) {
      flushParagraph();
      out.push({ type: "heading", text: cleanInline(trimmed.replace(/^#{1,4}\s+/, "")) });
      continue;
    }
    paragraph.push(trimmed);
  }
  flushAll();
  return out;
}

function isDiagramLine(line: string) {
  return /(\$\\?(nearrow|rightarrow|swarrow|searrow|uparrow|downarrow|leftarrow)\$|[↗→↙↘↑↓←]|\\begin\{array\}|\\text\{)/.test(line);
}

function cleanDiagramLine(line: string) {
  return cleanInline(line)
    .replace(/\\begin\{array\}\{[^}]*\}/g, "")
    .replace(/\\end\{array\}/g, "")
    .replace(/\\\\/g, "\n")
    .replace(/\n{2,}/g, "\n")
    .trim();
}

function cleanInline(text: string) {
  return text
    .replace(/\$+/g, "")
    .replace(/\\text\{([^}]*)\}/g, "$1")
    .replace(/\\square/g, "□")
    .replace(/\\nearrow/g, "↗")
    .replace(/\\rightarrow/g, "→")
    .replace(/\\swarrow/g, "↙")
    .replace(/\\searrow/g, "↘")
    .replace(/\\uparrow/g, "↑")
    .replace(/\\downarrow/g, "↓")
    .replace(/\\leftarrow/g, "←")
    .replace(/\*\*([^*]+)\*\*/g, "$1")
    .trim();
}
</script>

<template>
  <section class="ocr-viewer">
    <div class="ocr-viewer-toolbar">
      <h4>OCR text</h4>
      <div class="ocr-viewer-tabs" role="group" aria-label="OCR display mode">
        <button type="button" :class="{ active: mode === 'rendered' }" @click="mode = 'rendered'">Rendered</button>
        <button type="button" :class="{ active: mode === 'raw' }" @click="mode = 'raw'">Raw</button>
      </div>
    </div>

    <div v-if="mode === 'rendered'" class="ocr-rendered">
      <template v-for="(block, index) in blocks" :key="index">
        <h5 v-if="block.type === 'heading'">{{ block.text }}</h5>
        <p v-else-if="block.type === 'paragraph'">{{ block.text }}</p>
        <ul v-else-if="block.type === 'list'">
          <li v-for="(item, itemIndex) in block.items" :key="itemIndex">{{ item }}</li>
        </ul>
        <table v-else-if="block.type === 'table'">
          <tbody>
            <tr v-for="(row, rowIndex) in block.rows" :key="rowIndex">
              <td v-for="(cell, cellIndex) in row" :key="cellIndex">{{ cell }}</td>
            </tr>
          </tbody>
        </table>
        <pre v-else>{{ block.text }}</pre>
      </template>
      <p v-if="!blocks.length" class="muted">No OCR text for this page.</p>
    </div>

    <textarea
      v-else
      :readonly="!editable"
      :value="text"
      @input="emit('update', ($event.target as HTMLTextAreaElement).value)"
    />
  </section>
</template>

<style scoped>
.ocr-viewer {
  display: grid;
  gap: 0.45rem;
  min-width: 0;
}

.ocr-viewer-toolbar,
.ocr-viewer-tabs {
  align-items: center;
  display: flex;
  gap: 0.5rem;
  justify-content: space-between;
}

.ocr-viewer h4 {
  font-size: 0.9rem;
  line-height: 1.2;
  margin: 0;
}

.ocr-viewer-tabs {
  background: #0f1724;
  border: 1px solid #253247;
  border-radius: 0.35rem;
  padding: 0.12rem;
}

.ocr-viewer-tabs button {
  border: 0;
  border-radius: 0.25rem;
  background: transparent;
  padding: 0.22rem 0.5rem;
}

.ocr-viewer-tabs button.active {
  background: #1e3a5f;
}

.ocr-rendered {
  background: #020617;
  border: 1px solid #253247;
  border-radius: 0.35rem;
  color: #e5e7eb;
  min-height: 29rem;
  overflow: auto;
  padding: 0.7rem;
}

.ocr-rendered h5 {
  color: #bfdbfe;
  font-size: 0.92rem;
  margin: 0.8rem 0 0.45rem;
}

.ocr-rendered h5:first-child {
  margin-top: 0;
}

.ocr-rendered p,
.ocr-rendered ul {
  margin: 0 0 0.75rem;
}

.ocr-rendered li {
  margin: 0.18rem 0;
}

.ocr-rendered pre {
  background: #07111f;
  border: 1px solid #1e3656;
  color: #dbeafe;
  font: 0.92rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  margin: 0 0 0.75rem;
  overflow: auto;
  padding: 0.75rem;
  white-space: pre-wrap;
}

.ocr-rendered table {
  border-collapse: collapse;
  margin: 0 0 0.75rem;
  width: 100%;
}

.ocr-rendered td {
  border: 1px solid #253247;
  padding: 0.35rem 0.45rem;
  vertical-align: top;
}

.ocr-viewer textarea {
  background: #020617;
  border: 1px solid #253247;
  border-radius: 0.35rem;
  color: #e5e7eb;
  font: 0.9rem/1.55 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  min-height: 29rem;
  resize: vertical;
  width: 100%;
}
</style>
