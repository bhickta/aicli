<script setup lang="ts">
import type { ZettelCandidate, ZettelProposal } from "../../../features/zettel/types";
import ZettelBadges from "../common/ZettelBadges.vue";
import ZettelMergeDiff from "./ZettelMergeDiff.vue";
import ZettelNotePicker from "./ZettelNotePicker.vue";
import ZettelSection from "../common/ZettelSection.vue";

defineProps<{
  activePath: string;
  notes: string[];
  notesStatus: string;
  busy: boolean;
  candidates: ZettelCandidate[];
  selectedPaths: string[];
  proposal: ZettelProposal | null;
  proposalQuality: string;
  canSuggest: boolean;
  canPreview: boolean;
  canApply: boolean;
}>();

const emit = defineEmits<{
  updateActivePath: [path: string];
  loadNotes: [];
  suggest: [];
  preview: [];
  apply: [];
  toggleCandidate: [path: string, checked: boolean];
}>();

function formatScore(value: unknown) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return "n/a";
  return parsed.toFixed(2);
}

function formatRanges(ranges: ZettelCandidate["source_line_ranges"]) {
  if (!ranges.length) return "none";
  return ranges
    .map((range) => range.start_line === range.end_line ? String(range.start_line) : `${range.start_line}-${range.end_line}`)
    .join(", ");
}

function candidateBadges(candidate: ZettelCandidate) {
  return [
    `sim ${formatScore(candidate.similarity)}`,
    `conf ${formatScore(candidate.confidence)}`,
    candidate.relationship,
    candidate.risk,
  ].filter(Boolean);
}

function toggleCandidate(candidate: ZettelCandidate, event: Event) {
  const target = event.target;
  emit("toggleCandidate", candidate.path, target instanceof HTMLInputElement && target.checked);
}
</script>

<template>
  <ZettelSection title="Manual review" description="Select one existing note, inspect candidates, preview the diff, then apply.">
    <template #actions>
      <button type="button" :disabled="!canSuggest" @click="emit('suggest')">Suggest</button>
      <button type="button" :disabled="!canPreview" @click="emit('preview')">Preview</button>
      <button type="button" class="mod-cta" :disabled="!canApply" @click="emit('apply')">Apply</button>
    </template>

    <ZettelNotePicker
      :active-path="activePath"
      :notes="notes"
      :status="notesStatus"
      :busy="busy"
      @update-active-path="emit('updateActivePath', $event)"
      @load="emit('loadNotes')"
    />

    <div class="zettel-review-grid">
      <div class="zettel-candidates">
        <h3>Candidates</h3>
        <p v-if="!candidates.length" class="muted">Run Suggest after selecting a vault and active note.</p>
        <article v-for="candidate in candidates" :key="candidate.path" class="zettel-card">
          <label class="zettel-card-header">
            <input
              type="checkbox"
              :checked="selectedPaths.includes(candidate.path)"
              @change="toggleCandidate(candidate, $event)"
            >
            <span>{{ candidate.path }}</span>
          </label>
          <ZettelBadges :items="candidateBadges(candidate)" />
          <p class="muted">{{ candidate.reason || "No reason returned." }}</p>
          <p class="muted">Lines: {{ formatRanges(candidate.source_line_ranges) }}</p>
          <pre>{{ candidate.extracted_markdown }}</pre>
        </article>
      </div>

      <div class="zettel-preview">
        <h3>Merge Preview</h3>
        <p v-if="proposal" class="status-line">{{ proposalQuality }}</p>
        <template v-if="proposal">
          <ZettelMergeDiff
            :original-markdown="proposal.active_markdown || ''"
            :final-markdown="proposal.final_markdown"
            :insertions="proposal.merge_plan?.insertions || []"
          />
          <details class="zettel-final-markdown">
            <summary>Final markdown</summary>
            <textarea :value="proposal.final_markdown" readonly />
          </details>
        </template>
        <p v-else class="muted">Select candidates and run Preview. Nothing is written until Apply succeeds.</p>
      </div>
    </div>
  </ZettelSection>
</template>

<style scoped>
.zettel-review-grid {
  display: grid;
  grid-template-columns: minmax(320px, 1fr) minmax(320px, 1fr);
  gap: 12px;
}

.zettel-candidates,
.zettel-preview {
  display: grid;
  align-content: start;
  gap: 10px;
  min-width: 0;
}

.zettel-card {
  display: grid;
  gap: 8px;
  padding: 12px;
  border: 1px solid #2b313b;
  border-radius: 8px;
  background: #10141b;
}

.zettel-card-header {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: 8px;
  overflow-wrap: anywhere;
  font-weight: 650;
}

.zettel-card pre {
  max-height: 220px;
  overflow: auto;
  white-space: pre-wrap;
}

.zettel-preview textarea {
  box-sizing: border-box;
  width: 100%;
  min-height: 520px;
  resize: vertical;
  font-family: ui-sans-serif, system-ui, sans-serif;
}

.zettel-final-markdown {
  display: grid;
  gap: 8px;
}

.zettel-final-markdown summary {
  cursor: pointer;
  color: #d6deea;
}

.mod-cta {
  border-color: #6ea8fe;
  background: #2d405e;
}

@media (max-width: 760px) {
  .zettel-review-grid {
    grid-template-columns: 1fr;
  }
}
</style>
