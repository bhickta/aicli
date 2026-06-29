<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef, watch } from "vue";
import { useRoute } from "vue-router";
import PageHeader from "../components/layout/PageHeader.vue";
import PageTabs from "../components/layout/PageTabs.vue";
import { useConfirm } from "../composables/useConfirm";
import { useToasts } from "../composables/useToasts";
import { api } from "../lib/api";
import { describeJobProgress, progressBarWidth } from "../lib/jobProgress";
import type { Job } from "../types";

type JobFilter = "recent" | "running" | "failed" | "completed" | "cancelled" | "all";

const filterOptions: Array<{ id: JobFilter; label: string }> = [
  { id: "recent", label: "Recent" },
  { id: "running", label: "Running" },
  { id: "failed", label: "Failed" },
  { id: "completed", label: "Completed" },
  { id: "cancelled", label: "Cancelled" },
  { id: "all", label: "All" },
];

const status = shallowRef("Loading jobs...");
const jobs = shallowRef<Job[]>([]);
const route = useRoute();
const toasts = useToasts();
const { confirm } = useConfirm();
const activeFilter = computed<JobFilter>(() => normalizeJobFilter(String(route.params.filter || "recent")));
let refreshTimer: number | undefined;

const jobRows = computed(() =>
  jobs.value.map((job) => {
    const progress = describeJobProgress(job);
    return {
      job,
      progress,
      progressClass: {
        hidden: !progress.visible,
        indeterminate: progress.mode === "indeterminate",
      },
      progressStyle: {
        width: progressBarWidth(progress.mode, progress.percent),
      },
      statusClass: `job-status ${job.status || "unknown"}`,
    };
  }),
);

const loadedSummary = computed(() => {
  const counts = jobs.value.reduce<Record<string, number>>((acc, job) => {
    acc[job.status] = (acc[job.status] || 0) + 1;
    return acc;
  }, {});
  return [
    `${jobs.value.length} loaded`,
    `${counts.running || 0} running`,
    `${counts.failed || 0} failed`,
  ].join(" | ");
});
const filterTabs = computed(() => filterOptions.map((option) => ({
  label: option.label,
  to: { name: "jobs", params: { filter: option.id } },
  active: activeFilter.value === option.id,
})));

onMounted(() => {
  void loadJobs();
  refreshTimer = window.setInterval(() => void loadJobs({ silent: true }), 2_000);
});

onUnmounted(() => {
  if (refreshTimer !== undefined) window.clearInterval(refreshTimer);
});

watch(activeFilter, () => {
  void loadJobs({ silent: false });
});

async function loadJobs(options: { silent?: boolean } = {}) {
  if (!options.silent) status.value = "Loading jobs...";
  try {
    const limit = activeFilter.value === "all" ? 200 : 20;
    const payload = await api<{ jobs: Job[] }>(`/api/jobs?status=${encodeURIComponent(activeFilter.value)}&limit=${limit}`);
    jobs.value = payload.jobs || [];
    if (!options.silent) status.value = "Loaded";
  } catch (error) {
    if (!options.silent) {
      jobs.value = [];
      status.value = error instanceof Error ? error.message : "Failed to load jobs";
    }
  }
}

async function clearFinishedJobs() {
  const ok = await confirm({
    title: "Clear finished jobs?",
    message: "Completed, failed, and cancelled job records will be removed from the list.",
    confirmLabel: "Clear finished",
    danger: true,
  });
  if (!ok) return;
  status.value = "Clearing finished jobs...";
  try {
    const payload = await api<{ deleted: number }>("/api/jobs?scope=finished", { method: "DELETE" });
    status.value = `Cleared ${payload.deleted} job(s)`;
    toasts.success("Finished jobs cleared", `${payload.deleted} job(s) removed.`);
    await loadJobs({ silent: true });
  } catch (error) {
    status.value = error instanceof Error ? error.message : "Clear failed";
    toasts.error("Clear failed", status.value);
  }
}

async function cancelJob(job: Job) {
  const ok = await confirm({
    title: "Cancel job?",
    message: `Cancel ${job.type || job.id}? Running work may stop before producing output.`,
    confirmLabel: job.type === "whatsapp-scheduled-message" ? "Cancel schedule" : "Cancel job",
    danger: true,
  });
  if (!ok) return;
  status.value = `Cancelling ${job.id}...`;
  try {
    const cancelled = await api<Job>(`/api/jobs/${encodeURIComponent(job.id)}/cancel`, { method: "POST" });
    jobs.value = jobs.value.map((item) => item.id === cancelled.id ? cancelled : item);
    status.value = "Cancelled";
    toasts.info("Job cancelled", cancelled.type || cancelled.id);
  } catch (error) {
    status.value = error instanceof Error ? error.message : "Cancel failed";
    toasts.error("Cancel failed", status.value);
  }
}

function jobTime(job: Job) {
  const value = job.finished_at || job.updated_at || job.created_at;
  if (!value) return "";
  const date = new Date(value);
  if (!Number.isFinite(date.getTime())) return "";
  return date.toLocaleString();
}

function shortText(value: string, limit = 320) {
  const text = String(value || "").trim();
  if (text.length <= limit) return text;
  return `${text.slice(0, limit)}...`;
}

function normalizeJobFilter(value: string): JobFilter {
  return filterOptions.some((option) => option.id === value) ? value as JobFilter : "recent";
}
</script>

<template>
  <div class="panel jobs-view">
    <PageHeader title="Jobs" :description="loadedSummary">
      <template #actions>
        <button type="button" @click="loadJobs()">Refresh</button>
        <button type="button" @click="clearFinishedJobs">Clear finished</button>
      </template>
    </PageHeader>

    <PageTabs :tabs="filterTabs" label="Job filters" />

    <p class="status-line" role="status" aria-live="polite">{{ status }}</p>

    <div v-if="jobRows.length" class="job-list">
      <article v-for="{ job, progress, progressClass, progressStyle, statusClass } in jobRows" :key="job.id" class="job-row">
        <div class="job-main">
          <div class="job-title-row">
            <strong>{{ job.type }}</strong>
            <span :class="statusClass">{{ job.status }}</span>
          </div>
          <p class="job-input">{{ job.input || job.id }}</p>
          <p class="muted">{{ job.stage || job.id }} | {{ jobTime(job) }}</p>
          <div class="progress" :class="progressClass">
            <div :style="progressStyle" />
          </div>
          <p v-if="progress.visible" class="status-line compact">{{ progress.text }}</p>
          <p v-if="job.error" class="job-error">{{ shortText(job.error) }}</p>
          <details v-if="job.output" class="job-details">
            <summary>Output</summary>
            <pre>{{ shortText(job.output, 1200) }}</pre>
          </details>
        </div>
        <button v-if="job.status === 'running'" type="button" @click="cancelJob(job)">
          {{ job.type === "whatsapp-scheduled-message" ? "Cancel schedule" : "Cancel job" }}
        </button>
      </article>
    </div>

    <p v-else class="empty-state">No jobs</p>
  </div>
</template>

<style scoped>
.jobs-view {
  display: grid;
  gap: 14px;
}

.jobs-header {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: start;
}

.jobs-header h2,
.jobs-header p {
  margin: 0;
}

.jobs-actions {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
  justify-content: end;
}

.job-filter-tabs {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.job-filter-tabs button {
  padding: 6px 10px;
  border-color: #343b46;
  background: #161b22;
}

.job-filter-tabs button.active {
  border-color: #69a1ff;
  background: #1f3350;
}

.job-list {
  display: grid;
  gap: 8px;
}

.job-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 10px;
  border: 1px solid #343b46;
  border-radius: 6px;
  background: #11161d;
}

.job-main {
  min-width: 0;
}

.job-title-row {
  display: flex;
  gap: 8px;
  align-items: center;
  justify-content: space-between;
}

.job-input,
.job-row p {
  margin: 4px 0 0;
}

.job-input {
  overflow-wrap: anywhere;
}

.job-status {
  padding: 2px 7px;
  border: 1px solid #343b46;
  border-radius: 999px;
  color: #c8d1dc;
  background: #1b222c;
  font-size: 12px;
}

.job-status.running {
  color: #d9ecff;
  border-color: #4f83c2;
  background: #16304f;
}

.job-status.completed {
  color: #d7f8dd;
  border-color: #44875a;
  background: #183322;
}

.job-status.failed {
  color: #ffdede;
  border-color: #9a4d4d;
  background: #3b1b1f;
}

.job-status.cancelled {
  color: #f0dfbd;
  border-color: #8d7040;
  background: #352818;
}

.job-error {
  color: #ffb7b7;
  overflow-wrap: anywhere;
}

.job-details {
  margin-top: 8px;
}

.job-details pre {
  max-height: 220px;
  overflow: auto;
  margin: 8px 0 0;
  padding: 8px;
  border: 1px solid #2b3440;
  border-radius: 6px;
  background: #0c1117;
  white-space: pre-wrap;
}

.empty-state {
  margin: 0;
  padding: 14px;
  border: 1px dashed #343b46;
  border-radius: 6px;
  color: #9aa7b6;
}

@media (max-width: 720px) {
  .jobs-header,
  .job-row {
    grid-template-columns: 1fr;
  }

  .jobs-actions {
    justify-content: start;
  }
}
</style>
