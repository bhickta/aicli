import { api } from "../api.js";
import { view } from "../elements.js";

export function renderTools() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Tools</h2>
      <div class="field">
        <h3>Status</h3>
        <p id="tools-status" class="status-line" role="status" aria-live="polite">Loading tools...</p>
      </div>
      <div class="field">
        <h3>Tool metadata</h3>
        <pre id="tools-output" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  loadTools();
}

async function loadTools() {
  const status = document.querySelector("#tools-status");
  const output = document.querySelector("#tools-output");
  status.textContent = "Loading tools...";
  try {
    const payload = await api("/api/tools");
    output.textContent = JSON.stringify(payload.tools, null, 2);
    status.textContent = "Loaded";
  } catch (error) {
    output.textContent = error.message;
    status.textContent = "Failed to load tools";
  }
}
