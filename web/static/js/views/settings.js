import { api } from "../core/api.js";
import { view } from "../core/elements.js";
import { state } from "../core/state.js";
import { escapeHtml } from "../core/utils.js";

export function renderSettings() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Settings</h2>
      <div class="field">
        <label for="settings-editor">Configuration JSON</label>
        <textarea id="settings-editor" rows="20">${escapeHtml(JSON.stringify(state.settings, null, 2))}</textarea>
      </div>
      <div class="field">
        <button id="settings-save" type="button">Save</button>
      </div>
      <div class="field">
        <h3>Status</h3>
        <p id="settings-status" class="status-line" role="status" aria-live="polite">Ready</p>
      </div>
      <div class="field">
        <h3>Save result</h3>
        <pre id="settings-result" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  document.querySelector("#settings-save").addEventListener("click", saveSettings);
}

async function saveSettings() {
  const status = document.querySelector("#settings-status");
  const output = document.querySelector("#settings-result");
  const editor = document.querySelector("#settings-editor");
  status.textContent = "Saving...";
  output.textContent = "";
  try {
    state.settings = await api("/api/settings", {
      method: "PUT",
      body: editor.value,
    });
    output.textContent = "saved";
    status.textContent = "Saved";
  } catch (error) {
    output.textContent = error.message;
    status.textContent = "Failed";
  }
}
