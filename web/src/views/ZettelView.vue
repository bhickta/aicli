<script setup lang="ts">
import { computed, onMounted, reactive, shallowRef, watch } from "vue";
import ZettelBadges from "../components/zettel/ZettelBadges.vue";
import ZettelFolderChooser from "../components/zettel/ZettelFolderChooser.vue";
import ZettelInboxReport from "../components/zettel/ZettelInboxReport.vue";
import ZettelMergeDiff from "../components/zettel/ZettelMergeDiff.vue";
import ZettelNotePicker from "../components/zettel/ZettelNotePicker.vue";
import ZettelProviderSettings from "../components/zettel/ZettelProviderSettings.vue";
import ZettelRunSizeControl from "../components/zettel/ZettelRunSizeControl.vue";
import ZettelWorkflowTabs from "../components/zettel/ZettelWorkflowTabs.vue";
import { api, parseJobOutput, pollJob } from "../lib/api";
import { readNumberValue, stringify } from "../lib/format";
import { describeJobProgress, progressBarWidth } from "../lib/jobProgress";
import type { InboxMergeReport, Job, ProgressMode } from "../types";
import type { ZettelMode } from "../components/zettel/types";

interface LineRange {
  start_line: number;
  end_line: number;
  reason?: string;
}

interface Candidate {
  path: string;
  similarity: number;
  confidence: number;
  relationship: string;
  risk: string;
  reason: string;
  source_line_ranges: LineRange[];
  extracted_markdown: string;
}

interface Proposal {
  id: string;
  active_markdown?: string;
  final_markdown: string;
  merge_plan?: { insertions?: Array<{ after_line: number; markdown: string; reason?: string }> };
  coverage?: { score?: number };
  judge?: { verdict?: string; score?: number; notes?: string };
}

type ZettelFolderField = "rootFolder" | "inboxFolder" | "dataFolder";

const legacyProviderId = localStorage.getItem("aicli.zettel.providerId") || "lms";
const legacyJudgeModel = localStorage.getItem("aicli.zettel.judgeModel") || "deepseek-reasoner";
const storedMode = localStorage.getItem("aicli.zettel.mode");

const config = reactive({
  vaultPath: localStorage.getItem("aicli.zettel.vaultPath") || "",
  activePath: localStorage.getItem("aicli.zettel.activePath") || "",
  rootFolder: localStorage.getItem("aicli.zettel.rootFolder") || "zettelkasten",
  inboxFolder: localStorage.getItem("aicli.zettel.inboxFolder") || "inbox-to-merge",
  inboxLimit: Number(localStorage.getItem("aicli.zettel.inboxLimit") || 0),
  dataFolder: localStorage.getItem("aicli.zettel.dataFolder") || ".aicli-zettel-merge",
  shorthandPromptPath: localStorage.getItem("aicli.zettel.shorthandPromptPath") || "example_prompts.md",
  providerId: legacyProviderId,
  candidateProviderId: localStorage.getItem("aicli.zettel.candidateProviderId") || legacyProviderId,
  mergeProviderId: localStorage.getItem("aicli.zettel.mergeProviderId") || legacyProviderId,
  validationProviderId: localStorage.getItem("aicli.zettel.validationProviderId") || legacyProviderId,
  embeddingProviderId: localStorage.getItem("aicli.zettel.embeddingProviderId") || "lms",
  judgeModel: legacyJudgeModel,
  candidateModel: localStorage.getItem("aicli.zettel.candidateModel") || legacyJudgeModel,
  mergeModel: localStorage.getItem("aicli.zettel.mergeModel") || "deepseek-reasoner",
  validationModel: localStorage.getItem("aicli.zettel.validationModel") || legacyJudgeModel,
  embeddingModel: localStorage.getItem("aicli.zettel.embeddingModel") || "text-embedding-nomic-embed-text-v1.5",
  candidateLimit: Number(localStorage.getItem("aicli.zettel.candidateLimit") || 12),
  reviewThreshold: Number(localStorage.getItem("aicli.zettel.reviewThreshold") || 0.85),
  validationThreshold: Number(localStorage.getItem("aicli.zettel.validationThreshold") || 0.98),
});

const candidates = shallowRef<Candidate[]>([]);
const notes = shallowRef<string[]>([]);
const selectedPaths = shallowRef<string[]>([]);
const proposal = shallowRef<Proposal | null>(null);
const inboxReport = shallowRef<InboxMergeReport | null>(null);
const status = shallowRef("Ready");
const notesStatus = shallowRef("Load notes after selecting a vault.");
const result = shallowRef("");
const progress = shallowRef(0);
const progressMode = shallowRef<ProgressMode>("determinate");
const progressVisible = shallowRef(false);
const busy = shallowRef(false);
const rollbackJobID = shallowRef("");
const mode = shallowRef<ZettelMode>(storedMode === "manual" || storedMode === "settings" ? storedMode : "inbox");

const candidateLimitOptions = [6, 12, 20, 30, 50];
const thresholdOptions = [
  { value: 0.75, label: "Broad" },
  { value: 0.85, label: "Balanced" },
  { value: 0.9, label: "Strict" },
  { value: 0.98, label: "Lossless" },
];
const validationThresholdOptions = [
  { value: 0.9, label: "Strict" },
  { value: 0.98, label: "Lossless" },
  { value: 1, label: "Exact" },
];
const promptOptions = [
  { value: "example_prompts.md", label: "Extreme shorthand" },
  { value: "builtin", label: "Built-in fallback" },
];

const selectedCandidates = computed(() => {
  const selected = new Set(selectedPaths.value);
  return candidates.value.filter((candidate) => selected.has(candidate.path));
});

const canSuggest = computed(() => Boolean(config.vaultPath.trim() && config.activePath.trim()) && !busy.value);
const canPreview = computed(() => selectedCandidates.value.length > 0 && !busy.value);
const canApply = computed(() => Boolean(proposal.value) && !busy.value);
const canRunInboxMerge = computed(() => Boolean(config.vaultPath.trim()) && !busy.value);
const canUseVaultFolders = computed(() => Boolean(config.vaultPath.trim()) && !busy.value);
const rawResultSummary = computed(() => result.value ? "Raw result" : "");
const proposalQuality = computed(() => {
  if (!proposal.value) return "";
  const coverage = formatScore(proposal.value.coverage?.score);
  const judge = formatScore(proposal.value.judge?.score);
  return `coverage ${coverage} | judge ${judge} | ${proposal.value.judge?.verdict || "unknown"}`;
});
const progressClass = computed(() => ({
  hidden: !progressVisible.value,
  indeterminate: progressMode.value === "indeterminate",
}));
const progressStyle = computed(() => ({
  width: progressBarWidth(progressMode.value, progress.value),
}));

onMounted(() => {
  if (!config.vaultPath) return;
  void refreshNotes().catch((error) => {
    notesStatus.value = error instanceof Error ? error.message : "Unable to load notes";
  });
});

watch(config, () => {
  localStorage.setItem("aicli.zettel.vaultPath", config.vaultPath);
  localStorage.setItem("aicli.zettel.activePath", config.activePath);
  localStorage.setItem("aicli.zettel.rootFolder", config.rootFolder);
  localStorage.setItem("aicli.zettel.inboxFolder", config.inboxFolder);
  localStorage.setItem("aicli.zettel.inboxLimit", String(config.inboxLimit));
  localStorage.setItem("aicli.zettel.dataFolder", config.dataFolder);
  localStorage.setItem("aicli.zettel.shorthandPromptPath", config.shorthandPromptPath);
  localStorage.setItem("aicli.zettel.providerId", config.providerId);
  localStorage.setItem("aicli.zettel.candidateProviderId", config.candidateProviderId);
  localStorage.setItem("aicli.zettel.mergeProviderId", config.mergeProviderId);
  localStorage.setItem("aicli.zettel.validationProviderId", config.validationProviderId);
  localStorage.setItem("aicli.zettel.embeddingProviderId", config.embeddingProviderId);
  localStorage.setItem("aicli.zettel.judgeModel", config.judgeModel);
  localStorage.setItem("aicli.zettel.candidateModel", config.candidateModel);
  localStorage.setItem("aicli.zettel.mergeModel", config.mergeModel);
  localStorage.setItem("aicli.zettel.validationModel", config.validationModel);
  localStorage.setItem("aicli.zettel.embeddingModel", config.embeddingModel);
  localStorage.setItem("aicli.zettel.candidateLimit", String(config.candidateLimit));
  localStorage.setItem("aicli.zettel.reviewThreshold", String(config.reviewThreshold));
  localStorage.setItem("aicli.zettel.validationThreshold", String(config.validationThreshold));
});

watch(mode, () => {
  localStorage.setItem("aicli.zettel.mode", mode.value);
});

function updateProviderSettings(value: Partial<Pick<
  typeof config,
  "candidateProviderId" |
  "mergeProviderId" |
  "validationProviderId" |
  "embeddingProviderId" |
  "candidateModel" |
  "mergeModel" |
  "validationModel" |
  "embeddingModel"
>>) {
  Object.assign(config, value);
  config.providerId = config.candidateProviderId;
  config.judgeModel = config.candidateModel;
}

function basePayload() {
  return {
    vault_path: config.vaultPath,
    root_folder: config.rootFolder,
    inbox_folder: config.inboxFolder,
    inbox_limit: readNumberValue(config.inboxLimit, 0, 0),
    data_folder: config.dataFolder,
    shorthand_prompt_path: config.shorthandPromptPath,
    provider_id: config.candidateProviderId,
    candidate_provider_id: config.candidateProviderId,
    merge_provider_id: config.mergeProviderId,
    validation_provider_id: config.validationProviderId,
    embedding_provider_id: config.embeddingProviderId,
    judge_model: config.candidateModel,
    candidate_model: config.candidateModel,
    merge_model: config.mergeModel,
    validation_model: config.validationModel,
    embedding_model: config.embeddingModel,
    candidate_limit: readNumberValue(config.candidateLimit, 12, 1),
    review_threshold: readNumberValue(config.reviewThreshold, 0.85, 0),
    validation_threshold: readNumberValue(config.validationThreshold, 0.98, 0),
  };
}

async function pickVault() {
  await run("Opening vault picker", async () => {
    const query = config.vaultPath ? `?path=${encodeURIComponent(config.vaultPath)}` : "";
    const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
    config.vaultPath = picked.path;
    status.value = "Vault selected";
    result.value = picked.path;
    await refreshNotes();
  });
}

async function pickZettelFolder(field: ZettelFolderField, label: string) {
  await run(`Choosing ${label}`, async () => {
    if (!config.vaultPath.trim()) throw new Error("Select a vault first");
    const startPath = folderPickerStartPath(field);
    const query = startPath ? `?path=${encodeURIComponent(startPath)}` : "";
    const picked = await api<{ path: string }>(`/api/fs/pick-directory${query}`);
    const relative = relativeToVault(picked.path);
    if (!relative) throw new Error(`${label} must be inside the selected vault`);
    config[field] = relative;
    status.value = `${label} selected`;
    result.value = `${label}: ${relative}`;
  });
}

async function loadNotes() {
  await run("Loading notes", refreshNotes);
}

async function refreshNotes() {
  notesStatus.value = "Loading notes...";
  const output = await api<{ notes?: string[]; count?: number }>("/api/workflows/zettel/notes", {
    method: "POST",
    body: JSON.stringify(basePayload()),
  });
  notes.value = output.notes || [];
  notesStatus.value = `${output.count ?? notes.value.length} note(s) loaded`;
  if (config.activePath && !notes.value.includes(config.activePath)) {
    notesStatus.value += " | active note is outside the loaded list";
  }
}

async function buildIndex() {
  await runWorkflow("Building zettel index", "/api/workflows/zettel/index", basePayload(), (output) => {
    status.value = "Embedding index is ready";
    result.value = stringify(output);
  });
}

async function suggest() {
  await runWorkflow("Finding merge candidates", "/api/workflows/zettel/suggest", {
    ...basePayload(),
    active_path: config.activePath,
  }, (output) => {
    const response = output as { candidates?: Candidate[] };
    candidates.value = response.candidates || [];
    selectedPaths.value = [];
    proposal.value = null;
    status.value = candidates.value.length ? `${candidates.value.length} mergeable candidate(s) found` : "No mergeable candidates found";
    result.value = stringify(output);
  });
}

async function previewMerge() {
  const selections = selectedCandidates.value.map((candidate) => ({
    path: candidate.path,
    source_line_ranges: candidate.source_line_ranges,
  }));
  await runWorkflow("Preparing merge preview", "/api/workflows/zettel/propose", {
    ...basePayload(),
    active_path: config.activePath,
    selections,
  }, (output) => {
    const response = output as { proposal?: Proposal };
    proposal.value = response.proposal || null;
    status.value = proposal.value ? "Merge preview ready" : "Merge preview returned no proposal";
    result.value = stringify(output);
  });
}

async function applyMerge() {
  if (!proposal.value) return;
  await runWorkflow("Applying approved merge", "/api/workflows/zettel/apply", {
    ...basePayload(),
    proposal: proposal.value,
  }, (output) => {
    const response = output as { job_id?: string };
    rollbackJobID.value = response.job_id || proposal.value?.id || "";
    status.value = `Applied merge ${rollbackJobID.value}`;
    result.value = stringify(output);
  });
}

async function rollback() {
  await runWorkflow("Rolling back zettel merge", "/api/workflows/zettel/rollback", {
    ...basePayload(),
    job_id: rollbackJobID.value,
  }, (output) => {
    status.value = "Rollback completed";
    result.value = stringify(output);
  });
}

async function runInboxMerge() {
  await runWorkflow("Running inbox merge", "/api/workflows/zettel/inbox-merge", basePayload(), (output) => {
    const response = output as InboxMergeReport;
    inboxReport.value = response;
    rollbackJobID.value = response.run_id || "";
    status.value = `Inbox merge completed: ${response.processed_count} processed, ${response.pending_count} pending, ${response.failed_count} failed`;
    result.value = stringify(output);
  });
}

async function runWorkflow(label: string, endpoint: string, payload: Record<string, unknown>, onDone: (output: unknown) => void) {
  await run(label, async () => {
    const response = await api<{ job?: Job }>(endpoint, {
      method: "POST",
      body: JSON.stringify(payload),
    });
    if (!response.job?.id) throw new Error("workflow did not return a job");
    await pollJob(response.job.id, renderJob);
    const finalJob = await api<Job>(`/api/jobs/${encodeURIComponent(response.job.id)}`);
    if (finalJob.status === "failed") throw new Error(finalJob.error || "workflow failed");
    onDone(parseJobOutput(finalJob.output));
  });
}

async function run(label: string, task: () => Promise<void>) {
  busy.value = true;
  status.value = `${label}...`;
  result.value = "";
  progress.value = 0;
  progressMode.value = "indeterminate";
  progressVisible.value = true;
  try {
    await task();
  } catch (error) {
    status.value = `${label} failed`;
    result.value = error instanceof Error ? error.message : "operation failed";
    if (progress.value <= 0) progressVisible.value = false;
  } finally {
    busy.value = false;
  }
}

function renderJob(job: Job) {
  const presentation = describeJobProgress(job);
  status.value = presentation.text;
  progress.value = presentation.percent;
  progressMode.value = presentation.mode;
  progressVisible.value = presentation.visible;
}

function toggleCandidate(path: string, checked: boolean) {
  const next = new Set(selectedPaths.value);
  if (checked) next.add(path);
  else next.delete(path);
  selectedPaths.value = Array.from(next);
  proposal.value = null;
}

function formatScore(value: unknown) {
  const parsed = Number(value);
  if (!Number.isFinite(parsed)) return "n/a";
  return parsed.toFixed(2);
}

function formatRanges(ranges: LineRange[]) {
  if (!ranges.length) return "none";
  return ranges.map((range) => range.start_line === range.end_line ? String(range.start_line) : `${range.start_line}-${range.end_line}`).join(", ");
}

function candidateBadges(candidate: Candidate) {
  return [
    `sim ${formatScore(candidate.similarity)}`,
    `conf ${formatScore(candidate.confidence)}`,
    candidate.relationship,
    candidate.risk,
  ].filter(Boolean);
}

function normalizePath(value: string) {
  return value.trim().replace(/\\/g, "/").replace(/\/+$/, "");
}

function folderPickerStartPath(field: ZettelFolderField) {
  const vaultPath = normalizePath(config.vaultPath);
  const current = normalizePath(String(config[field] || ""));
  if (!vaultPath) return "";
  if (!current) return vaultPath;
  if (current.startsWith("/")) return current;
  return `${vaultPath}/${current.replace(/^\/+/, "")}`;
}

function relativeToVault(path: string) {
  const vaultPath = normalizePath(config.vaultPath);
  const pickedPath = normalizePath(path);
  if (!vaultPath || !pickedPath || pickedPath === vaultPath) return "";
  if (!pickedPath.startsWith(`${vaultPath}/`)) return "";
  return pickedPath.slice(vaultPath.length + 1);
}
</script>

<template>
  <div class="panel grid zettel-panel">
    <div class="zettel-title-row">
      <div>
        <h2>Zettelkasten</h2>
        <p class="muted">Inbox merge by default. Manual review and provider setup are separate.</p>
      </div>
      <ZettelWorkflowTabs v-model="mode" />
    </div>

    <section class="zettel-section">
      <div class="zettel-section-header">
        <div>
          <h3>Vault</h3>
          <p class="muted">{{ config.vaultPath || "No vault selected" }}</p>
        </div>
        <button type="button" :disabled="busy" @click="pickVault">Browse vault</button>
      </div>
    </section>

    <section v-if="mode === 'inbox'" class="zettel-section">
      <div class="zettel-section-header">
        <div>
          <h3>Inbox merge</h3>
          <p class="muted">Choose source and destination folders, then run the lossless audit workflow.</p>
        </div>
        <button type="button" class="mod-cta" :disabled="!canRunInboxMerge" @click="runInboxMerge">Run Inbox Merge</button>
      </div>

      <div class="zettel-folder-grid">
        <ZettelFolderChooser
          label="Source inbox"
          :value="config.inboxFolder"
          description="New atomic notes waiting to be merged"
          :disabled="!canUseVaultFolders"
          @choose="pickZettelFolder('inboxFolder', 'source inbox')"
        />
        <ZettelFolderChooser
          label="Destination notes"
          :value="config.rootFolder"
          description="Existing zettelkasten tree that receives merged claims"
          :disabled="!canUseVaultFolders"
          @choose="pickZettelFolder('rootFolder', 'destination notes')"
        />
      </div>

      <ZettelRunSizeControl v-model="config.inboxLimit" :disabled="busy" />

      <div class="zettel-inline-actions">
        <button type="button" :disabled="busy || !config.vaultPath" @click="buildIndex">Build Index</button>
      </div>

      <ZettelInboxReport :report="inboxReport" />
      <p v-if="!inboxReport" class="muted">Run report, changed files, pending notes, and rollback id appear here.</p>
    </section>

    <section v-if="mode === 'manual'" class="zettel-section">
      <div class="zettel-section-header">
        <div>
          <h3>Manual review</h3>
          <p class="muted">Select one existing note, inspect candidates, preview the diff, then apply.</p>
        </div>
        <div class="zettel-inline-actions">
          <button type="button" :disabled="!canSuggest" @click="suggest">Suggest</button>
          <button type="button" :disabled="!canPreview" @click="previewMerge">Preview</button>
          <button type="button" class="mod-cta" :disabled="!canApply" @click="applyMerge">Apply</button>
        </div>
      </div>

      <ZettelNotePicker
        :active-path="config.activePath"
        :notes="notes"
        :status="notesStatus"
        :busy="busy"
        @update-active-path="config.activePath = $event"
        @load="loadNotes"
      />

      <div class="zettel-review-grid">
        <div class="zettel-candidates">
          <h3>Candidates</h3>
          <p v-if="!candidates.length" class="muted">Run Suggest after selecting a vault and active note.</p>
          <article v-for="candidate in candidates" :key="candidate.path" class="zettel-card">
            <label class="zettel-card-header">
              <input type="checkbox" :checked="selectedPaths.includes(candidate.path)" @change="toggleCandidate(candidate.path, ($event.target as HTMLInputElement).checked)">
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
    </section>

    <section v-if="mode === 'settings'" class="zettel-section">
      <div class="zettel-section-header">
        <div>
          <h3>Settings</h3>
          <p class="muted">All choices are saved locally and reused on the next run.</p>
        </div>
      </div>

      <div class="zettel-settings-grid">
        <ZettelProviderSettings :settings="config" @update="updateProviderSettings" />
        <div class="grid">
          <ZettelFolderChooser
            label="Index and audit data"
            :value="config.dataFolder"
            description="Cache, embeddings, reports, and rollback snapshots"
            :disabled="!canUseVaultFolders"
            @choose="pickZettelFolder('dataFolder', 'index and audit data')"
          />
          <div class="field-row">
            <div class="field">
              <label for="zettel-limit">Candidate limit</label>
              <select id="zettel-limit" v-model.number="config.candidateLimit">
                <option v-for="value in candidateLimitOptions" :key="value" :value="value">{{ value }} notes</option>
              </select>
            </div>
            <div class="field">
              <label for="zettel-review">Review threshold</label>
              <select id="zettel-review" v-model.number="config.reviewThreshold">
                <option v-for="option in thresholdOptions" :key="option.value" :value="option.value">
                  {{ option.label }} ({{ option.value }})
                </option>
              </select>
            </div>
            <div class="field">
              <label for="zettel-validation">Validation threshold</label>
              <select id="zettel-validation" v-model.number="config.validationThreshold">
                <option v-for="option in validationThresholdOptions" :key="option.value" :value="option.value">
                  {{ option.label }} ({{ option.value }})
                </option>
              </select>
            </div>
            <div class="field">
              <label for="zettel-shorthand-prompt">Prompt mode</label>
              <select id="zettel-shorthand-prompt" v-model="config.shorthandPromptPath">
                <option v-for="option in promptOptions" :key="option.value" :value="option.value">{{ option.label }}</option>
              </select>
            </div>
          </div>
        </div>
      </div>
    </section>

    <section class="zettel-section">
      <div class="zettel-section-header">
        <div>
          <h3>Status</h3>
          <p class="status-line" role="status" aria-live="polite">{{ status }}</p>
        </div>
        <button type="button" :disabled="busy" @click="rollback">Rollback</button>
      </div>
      <div class="progress" :class="progressClass">
        <div :style="progressStyle" />
      </div>
      <details v-if="rawResultSummary" class="zettel-raw-result">
        <summary>{{ rawResultSummary }}</summary>
        <pre role="status" aria-live="polite">{{ result }}</pre>
      </details>
    </section>
  </div>
</template>

<style scoped>
.zettel-panel {
  max-width: 1280px;
}

.zettel-title-row,
.zettel-section-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
}

.zettel-title-row p,
.zettel-section-header p {
  margin: 6px 0 0;
}

.zettel-section {
  display: grid;
  gap: 12px;
  padding: 14px;
  overflow: visible;
  border: 1px solid #242b35;
  border-radius: 8px;
  background: #141922;
}

.zettel-section-header button,
.zettel-inline-actions button,
.zettel-folder-grid button {
  text-align: center;
}

.zettel-folder-grid,
.zettel-settings-grid,
.zettel-review-grid {
  display: grid;
  gap: 12px;
}

.zettel-folder-grid,
.zettel-settings-grid {
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
}

.zettel-review-grid {
  grid-template-columns: minmax(320px, 1fr) minmax(320px, 1fr);
}

.zettel-inline-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.zettel-inline-actions .mod-cta,
.zettel-section-header .mod-cta {
  border-color: #6ea8fe;
  background: #2d405e;
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
  font-weight: 650;
  overflow-wrap: anywhere;
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

.zettel-final-markdown,
.zettel-raw-result {
  display: grid;
  gap: 8px;
}

.zettel-final-markdown summary,
.zettel-raw-result summary {
  cursor: pointer;
  color: #d6deea;
}

@media (max-width: 760px) {
  .zettel-title-row,
  .zettel-section-header {
    align-items: stretch;
    flex-direction: column;
  }

  .zettel-review-grid {
    grid-template-columns: 1fr;
  }
}
</style>
