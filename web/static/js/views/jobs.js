import { api } from "../api.js";
import { view } from "../elements.js";

export function renderJobs() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Jobs</h2>
      <div class="field">
        <h3>Status</h3>
        <p id="jobs-status" class="status-line" role="status" aria-live="polite">Loading jobs...</p>
      </div>
      <div class="field">
        <h3>Job list</h3>
        <pre id="jobs-output" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  loadJobs();
}

async function loadJobs() {
  const status = document.querySelector("#jobs-status");
  const output = document.querySelector("#jobs-output");
  status.textContent = "Loading jobs...";
  try {
    const payload = await api("/api/jobs");
    output.textContent = JSON.stringify(payload.jobs, null, 2);
    status.textContent = "Loaded";
  } catch (error) {
    output.textContent = error.message;
    status.textContent = "Failed to load jobs";
  }
}
