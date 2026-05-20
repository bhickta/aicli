<script setup lang="ts">
import { computed, onMounted, shallowRef } from "vue";
import { api } from "../lib/api";
import { stringify } from "../lib/format";
import type { Job } from "../types";

const status = shallowRef("Loading jobs...");
const jobs = shallowRef<Job[]>([]);
const output = computed(() => stringify(jobs.value));

onMounted(loadJobs);

async function loadJobs() {
  status.value = "Loading jobs...";
  try {
    const payload = await api<{ jobs: Job[] }>("/api/jobs");
    jobs.value = payload.jobs || [];
    status.value = "Loaded";
  } catch (error) {
    jobs.value = [];
    status.value = "Failed to load jobs";
  }
}
</script>

<template>
  <div class="panel grid">
    <h2>Jobs</h2>
    <div class="field">
      <button type="button" @click="loadJobs">Refresh jobs</button>
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
