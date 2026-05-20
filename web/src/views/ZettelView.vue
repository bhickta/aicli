<script setup lang="ts">
import { computed, onMounted, reactive, shallowRef, watch } from "vue";
import ZettelMergeDiff from "../components/zettel/ZettelMergeDiff.vue";
import ZettelNotePicker from "../components/zettel/ZettelNotePicker.vue";
import ZettelProviderSettings from "../components/zettel/ZettelProviderSettings.vue";
import { api, parseJobOutput, pollJob } from "../lib/api";
import { elapsedSeconds, formatDuration, readNumberValue, stringify } from "../lib/format";
import type { Job } from "../types";

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

const legacyProviderId = localStorage.getItem("aicli.zettel.providerId") || "lms";
const legacyJudgeModel = localStorage.getItem("aicli.zettel.judgeModel") || "deepseek-reasoner";

const config = reactive({
  vaultPath: localStorage.getItem("aicli.zettel.vaultPath") || "",
  activePath: localStorage.getItem("aicli.zettel.activePath") || "",
  rootFolder: localStorage.getItem("aicli.zettel.rootFolder") || "zettelkasten",
  dataFolder: localStorage.getItem("aicli.zettel.dataFolder") || ".aicli-zettel-merge",
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
const status = shallowRef("Ready");
const notesStatus = shallowRef("Load notes after selecting a vault.");
const result = shallowRef("");
const progress = shallowRef(0);
const busy = shallowRef(false);
const rollbackJobID = shallowRef("");

const selectedCandidates = computed(() => {
  const selected = new Set(selectedPaths.value);
  return candidates.value.filter((candidate) => selected.has(candidate.path));
});

const canPreview = computed(() => selectedCandidates.value.length > 0 && !busy.value);
const canApply = computed(() => Boolean(proposal.value) && !busy.value);
const proposalQuality = computed(() => {
  if (!proposal.value) return "";
  const coverage = formatScore(proposal.value.coverage?.score);
  const judge = formatScore(proposal.value.judge?.score);
  return `coverage ${coverage} | judge ${judge} | ${proposal.value.judge?.verdict || "unknown"}`;
});

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
  localStorage.setItem("aicli.zettel.dataFolder", config.dataFolder);
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
    data_folder: config.dataFolder,
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
  try {
    await task();
  } catch (error) {
    status.value = `${label} failed`;
    result.value = error instanceof Error ? error.message : "operation failed";
  } finally {
    busy.value = false;
  }
}

function renderJob(job: Job) {
  const percent = Math.round((job.progress || 0) * 100);
  const eta = job.eta_seconds ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
  const step = job.total_steps ? ` | ${job.current_step}/${job.total_steps}` : "";
  status.value = `${job.status}: ${job.stage || "working"}${step} | ${percent}% | elapsed ${formatDuration(elapsedSeconds(job.created_at))}${eta}`;
  progress.value = percent;
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
</script>

<template>
  <div class="panel grid zettel-panel">
    <h2>Zettelkasten Merge</h2>

    <div class="zettel-grid">
      <div class="grid">
        <div class="field">
          <label for="zettel-vault">Vault folder</label>
          <div class="path-control">
            <input id="zettel-vault" v-model="config.vaultPath" type="text" placeholder="/home/bhickta/development/upsc">
            <button type="button" :disabled="busy" @click="pickVault">Browse vault</button>
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
        <div class="field-row">
          <div class="field">
            <label for="zettel-root">Zettelkasten folder</label>
            <input id="zettel-root" v-model="config.rootFolder" type="text">
          </div>
          <div class="field">
            <label for="zettel-data">Data folder</label>
            <input id="zettel-data" v-model="config.dataFolder" type="text">
          </div>
        </div>
      </div>

      <div class="grid">
        <ZettelProviderSettings :settings="config" @update="updateProviderSettings" />
        <div class="field-row">
          <div class="field">
            <label for="zettel-limit">Candidate limit</label>
            <input id="zettel-limit" v-model.number="config.candidateLimit" type="number" min="1" max="50">
          </div>
          <div class="field">
            <label for="zettel-review">Review threshold</label>
            <input id="zettel-review" v-model.number="config.reviewThreshold" type="number" min="0" max="1" step="0.01">
          </div>
          <div class="field">
            <label for="zettel-validation">Validation threshold</label>
            <input id="zettel-validation" v-model.number="config.validationThreshold" type="number" min="0" max="1" step="0.01">
          </div>
        </div>
      </div>
    </div>

    <div class="zettel-actions">
      <button type="button" :disabled="busy" @click="buildIndex">Build Index</button>
      <button type="button" :disabled="busy" @click="suggest">Suggest</button>
      <button type="button" :disabled="!canPreview" @click="previewMerge">Preview Merge</button>
      <button type="button" class="mod-cta" :disabled="!canApply" @click="applyMerge">Apply</button>
      <button type="button" :disabled="busy" @click="rollback">Rollback</button>
    </div>

    <div class="field">
      <h3>Status</h3>
      <p class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="progress" :class="{ hidden: progress <= 0 }">
      <div :style="{ width: `${Math.max(0, Math.min(100, progress))}%` }" />
    </div>

    <div class="zettel-review-grid">
      <div class="zettel-candidates">
        <h3>Candidates</h3>
        <p v-if="!candidates.length" class="muted">Run Suggest after selecting a vault and active note.</p>
        <article v-for="candidate in candidates" :key="candidate.path" class="zettel-card">
          <label class="zettel-card-header">
            <input type="checkbox" :checked="selectedPaths.includes(candidate.path)" @change="toggleCandidate(candidate.path, ($event.target as HTMLInputElement).checked)">
            <span>{{ candidate.path }}</span>
          </label>
          <div class="zettel-badges">
            <span>sim {{ formatScore(candidate.similarity) }}</span>
            <span>conf {{ formatScore(candidate.confidence) }}</span>
            <span>{{ candidate.relationship }}</span>
            <span>{{ candidate.risk }}</span>
          </div>
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
        <p v-else class="muted">Select candidates and run Preview Merge. Nothing is written until Apply succeeds.</p>
      </div>
    </div>

    <div class="field">
      <h3>Raw result</h3>
      <pre role="status" aria-live="polite">{{ result }}</pre>
    </div>
  </div>
</template>
