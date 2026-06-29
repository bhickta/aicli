import { defineStore } from "pinia";
import { api, pollJob, sleep } from "../lib/api";
import type { Job, StudyBatchItemRecord, StudyBatchRecord, StudyBatchResponse, StudyCopyDetail, StudyCopyRecord } from "../types";

const terminalBatchStatuses = ["completed", "failed", "partial_failed", "cancelled"];
const terminalItemStatuses = ["ready", "failed"];

interface StudyState {
  status: string;
  query: string;
  copies: StudyCopyRecord[];
  selected: StudyCopyDetail | null;
  loading: boolean;
  selectedIds: string[];
  importPath: string;
  importFolder: string;
  batchParallelism: number;
  forceRerun: boolean;
  running: boolean;
  runStatus: string;
  activeBatch: StudyBatchRecord | null;
  batchItems: StudyBatchItemRecord[];
  refreshedItemKeys: string[];
}

export const useStudyStore = defineStore("study", {
  state: (): StudyState => ({
    status: "Loading study copies...",
    query: "",
    copies: [],
    selected: null,
    loading: false,
    selectedIds: [],
    importPath: "",
    importFolder: "",
    batchParallelism: 2,
    forceRerun: false,
    running: false,
    runStatus: "",
    activeBatch: null,
    batchItems: [],
    refreshedItemKeys: [],
  }),

  getters: {
    summary(state) {
      const pages = state.copies.reduce((total, copy) => total + copy.page_count, 0);
      const questions = state.copies.reduce((total, copy) => total + copy.question_count, 0);
      return `${state.copies.length} copies, ${pages} pages, ${questions} questions`;
    },
    visibleStatus(state) {
      return state.runStatus || state.status;
    },
    activeCopyId(state) {
      return state.selected?.copy.id || "";
    },
  },

  actions: {
    async loadCopies() {
      this.loading = true;
      this.status = "Loading study copies...";
      try {
        const payload = await api<{ copies: StudyCopyRecord[] }>(
          `/api/study/copies?query=${encodeURIComponent(this.query)}&limit=200`,
        );
        this.copies = payload.copies || [];
        this.status = "Loaded";
        this.applyBatchItems(this.batchItems);
        await this.loadRecentProblemBatch();
      } catch (error) {
        this.status = errorMessage(error, "Load failed");
      } finally {
        this.loading = false;
      }
    },

    async openCopy(id: string) {
      this.status = "Opening copy...";
      this.selected = await api<StudyCopyDetail>(`/api/study/copies/${encodeURIComponent(id)}`);
      this.status = "Copy loaded";
      this.applyBatchItems(this.batchItems);
    },

    clearSelectedCopy() {
      this.selected = null;
    },

    toggleCopy(id: string) {
      this.selectedIds = this.selectedIds.includes(id)
        ? this.selectedIds.filter((item) => item !== id)
        : [...this.selectedIds, id];
    },

    async importCopies() {
      const paths = this.importPath.split(/\n|,/).map((path) => path.trim()).filter(Boolean);
      if (!paths.length && !this.importFolder.trim()) return;
      this.status = "Importing PDFs...";
      await api<{ copies: StudyCopyRecord[] }>("/api/study/imports", {
        method: "POST",
        body: JSON.stringify({ paths, folder_path: this.importFolder.trim(), recursive: true }),
      });
      this.importPath = "";
      this.importFolder = "";
      await this.loadCopies();
    },

    async startBatch(stage = "all") {
      if (!this.selectedIds.length) return;
      await this.startRun("/api/study/batches", {
        copy_ids: this.selectedIds,
        stage,
        parallelism: this.batchParallelism,
        force_ocr: this.forceRerun,
      }, `Starting ${this.selectedIds.length} copy analysis...`);
    },

    async runCopy(copyId: string) {
      await this.startRun(`/api/study/copies/${encodeURIComponent(copyId)}/run`, {
        force_ocr: this.forceRerun,
      }, "Starting PDF analysis...");
    },

    async startRun(path: string, body: Record<string, unknown>, initialStatus: string) {
      this.running = true;
      this.runStatus = initialStatus;
      try {
        const payload = await api<StudyBatchResponse>(path, {
          method: "POST",
          body: JSON.stringify(body),
        });
        await this.trackBatch(payload);
      } catch (error) {
        this.runStatus = errorMessage(error, "Analysis failed to start");
      } finally {
        this.running = false;
      }
    },

    async trackBatch(payload: StudyBatchResponse) {
      this.activeBatch = payload.batch;
      this.batchItems = payload.items || [];
      this.applyBatchItems(this.batchItems);
      if (!payload.job?.id) return;
      this.runStatus = "Analysis running...";
      await Promise.all([this.pollBatch(payload.batch.id), pollJob(payload.job.id, (job) => this.updateJobStatus(job))]);
      await this.pollBatch(payload.batch.id);
      const failedItem = this.batchItems.find((item) => item.error);
      this.runStatus = failedItem?.error || (this.activeBatch?.status === "completed" ? "Analysis completed" : `Analysis ${this.activeBatch?.status}`);
      await this.refreshSettledSelection(this.batchItems);
    },

    async pollBatch(batchId: string) {
      for (;;) {
        const payload = await api<{ batch: StudyBatchRecord; items: StudyBatchItemRecord[] }>(
          `/api/study/batches/${encodeURIComponent(batchId)}`,
        );
        this.activeBatch = payload.batch;
        this.batchItems = payload.items || [];
        this.applyBatchItems(this.batchItems);
        void this.refreshSettledSelection(this.batchItems);
        if (terminalBatchStatuses.includes(payload.batch.status)) return;
        await sleep(900);
      }
    },

    async loadRecentProblemBatch() {
      if (this.activeBatch || this.running) return;
      const payload = await api<{ batches: StudyBatchRecord[] }>("/api/study/batches?limit=10");
      const batch = payload.batches.find((item) => ["running", "failed", "partial_failed"].includes(item.status));
      if (!batch) return;
      const detail = await api<{ batch: StudyBatchRecord; items: StudyBatchItemRecord[] }>(
        `/api/study/batches/${encodeURIComponent(batch.id)}`,
      );
      this.activeBatch = detail.batch;
      this.batchItems = detail.items || [];
      const failedItem = this.batchItems.find((item) => item.error);
      this.runStatus = failedItem?.error || `Analysis ${detail.batch.status}`;
      this.applyBatchItems(this.batchItems);
      if (detail.batch.status === "running") void this.pollBatch(detail.batch.id);
    },

    updateJobStatus(job: Job) {
      const total = job.total_units || this.activeBatch?.total || 0;
      this.runStatus = `${job.stage} (${job.completed_units || 0}/${total})`;
    },

    applyBatchItems(items: StudyBatchItemRecord[]) {
      if (!items.length) return;
      const byCopy = new Map(items.map((item) => [item.copy_id, item]));
      this.copies = this.copies.map((copy) => {
        const item = byCopy.get(copy.id);
        return item ? copyWithRunState(copy, item) : copy;
      });
      if (!this.selected) return;
      const item = byCopy.get(this.selected.copy.id);
      if (item) this.selected = { ...this.selected, copy: copyWithRunState(this.selected.copy, item) };
    },

    async refreshSettledSelection(items: StudyBatchItemRecord[]) {
      const selectedID = this.selected?.copy.id;
      if (!selectedID) return;
      const item = items.find((candidate) => candidate.copy_id === selectedID);
      if (!item || !terminalItemStatuses.includes(item.status)) return;
      const key = `${item.batch_id}:${item.copy_id}:${item.status}`;
      if (this.refreshedItemKeys.includes(key)) return;
      this.refreshedItemKeys = [...this.refreshedItemKeys, key];
      await this.openCopy(selectedID);
      await this.loadCopies();
    },
  },
});

function copyWithRunState(copy: StudyCopyRecord, item: StudyBatchItemRecord): StudyCopyRecord {
  if (item.status === "ready") return { ...copy, status: "ready", last_error: "" };
  if (item.status === "failed") return { ...copy, status: "failed", last_error: item.error || "Analysis failed" };
  return { ...copy, status: "running", analysis_status: "running", last_error: "" };
}

function errorMessage(error: unknown, fallback: string) {
  return error instanceof Error ? error.message : fallback;
}
