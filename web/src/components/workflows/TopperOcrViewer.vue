<script setup lang="ts">
import "../../styles/topper-ocr-viewer.css";
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
