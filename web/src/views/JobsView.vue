<script setup lang="ts">
import { computed, onMounted, onUnmounted, shallowRef } from "vue";
import { api } from "../lib/api";
import { stringify } from "../lib/format";
import { describeJobProgress, progressBarWidth } from "../lib/jobProgress";
import type { Job } from "../types";

const status = shallowRef("Loading jobs...");
const jobs = shallowRef<Job[]>([]);
let refreshTimer: number | undefined;
const output = computed(() => stringify(jobs.value));
const runningJobs = computed(() => jobs.value.filter((job) => job.status === "running"));
const runningJobRows = computed(() =>
  runningJobs.value.map((job) => {
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
    };
  }),
);

onMounted(() => {
  void loadJobs();
  refreshTimer = window.setInterval(() => void loadJobs({ silent: true }), 2_000);
});

onUnmounted(() => {
  if (refreshTimer !== undefined) window.clearInterval(refreshTimer);
});

async function loadJobs(options: { silent?: boolean } = {}) {
  if (!options.silent) status.value = "Loading jobs...";
  try {
    const payload = await api<{ jobs: Job[] }>("/api/jobs");
    jobs.value = payload.jobs || [];
    if (!options.silent) status.value = "Loaded";
  } catch (error) {
    if (!options.silent) {
      jobs.value = [];
      status.value = "Failed to load jobs";
    }
  }
}

async function cancelJob(job: Job) {
  status.value = `Cancelling ${job.id}...`;
  try {
    const cancelled = await api<Job>(`/api/jobs/${encodeURIComponent(job.id)}/cancel`, { method: "POST" });
    jobs.value = jobs.value.map((item) => item.id === cancelled.id ? cancelled : item);
    status.value = "Cancelled";
  } catch (error) {
    status.value = error instanceof Error ? error.message : "Cancel failed";
  }
}
</script>

<template>
  <div class="panel grid">
    <h2>Jobs</h2>
    <div class="field">
      <button type="button" @click="loadJobs()">Refresh jobs</button>
    </div>
    <div v-if="runningJobRows.length" class="field">
      <h3>Running jobs</h3>
      <div class="job-actions">
        <article v-for="{ job, progress, progressClass, progressStyle } in runningJobRows" :key="job.id" class="job-action-row">
          <div>
            <strong>{{ job.type }}</strong>
            <p class="muted">{{ job.input || job.id }}</p>
            <p class="status-line compact">{{ progress.text }}</p>
            <div class="progress" :class="progressClass">
              <div :style="progressStyle" />
            </div>
          </div>
          <button type="button" @click="cancelJob(job)">
            {{ job.type === "whatsapp-scheduled-message" ? "Cancel schedule" : "Cancel job" }}
          </button>
        </article>
      </div>
    </div>
    <div class="field">
      <h3>Status</h3>
      <p id="jobs-status" class="status-line" role="status" aria-live="polite">{{ status }}</p>
    </div>
    <div class="field">
      <h3>Job list</h3>
      <pre id="jobs-output" role="status" aria-live="polite">{{ output }}</pre>
    </div>
  </div>
</template>

<style scoped>
.job-actions {
  display: grid;
  gap: 8px;
}

.job-action-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  gap: 12px;
  align-items: center;
  padding: 10px;
  border: 1px solid #343b46;
  border-radius: 6px;
  background: #161b22;
}

.job-action-row p {
  margin: 4px 0 0;
}
</style>
