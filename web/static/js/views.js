import { api, loadSettings } from "./api.js";
import { view } from "./elements.js";
import { getProviderModelValues, getProviders, renderProviderModelRow, setupProviderModel } from "./provider-controls.js";
import { state } from "./state.js";
import { escapeHtml } from "./utils.js";
import { renderWorkflows } from "./workflows-view.js";

export async function render() {
  if (!state.settings) {
    await loadSettings();
  }
  if (state.view === "providers") return renderProviders();
  if (state.view === "tools") return renderTools();
  if (state.view === "jobs") return renderJobs();
  if (state.view === "settings") return renderSettings();
  if (state.view === "workflows") return renderWorkflows(render);
  return renderChat();
}

function renderChat() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Chat</h2>
      ${renderProviderModelRow("chat")}
      <div class="field">
        <label for="chat-prompt">Prompt</label>
        <textarea id="chat-prompt" rows="8" placeholder="Ask a local model..."></textarea>
      </div>
      <div class="field">
        <button id="chat-send" type="button">Send</button>
      </div>
      <div class="field">
        <h3>Status</h3>
        <p id="chat-status" class="status-line" role="status" aria-live="polite">Ready</p>
      </div>
      <div class="field">
        <h3>Answer</h3>
        <pre id="chat-answer" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  setupProviderModel("chat");
  document.querySelector("#chat-send").addEventListener("click", async () => {
    const prompt = document.querySelector("#chat-prompt");
    const status = document.querySelector("#chat-status");
    const answer = document.querySelector("#chat-answer");
    const { provider_id, model } = getProviderModelValues("chat");
    status.textContent = "Thinking...";
    answer.textContent = "";
    try {
      const result = await api("/api/chat", {
        method: "POST",
        body: JSON.stringify({
          provider_id,
          model,
          messages: [{ role: "user", content: prompt.value }],
          temperature: 0.1,
        }),
      });
      status.textContent = "Done";
      answer.textContent = result.content;
    } catch (error) {
      status.textContent = "Failed";
      answer.textContent = error.message;
    }
  });
}

function renderProviders() {
  view.innerHTML = `
    <div class="panel grid">
      <h2>Providers</h2>
      <div class="field">
        <label for="provider-list">Provider</label>
        <select id="provider-list"></select>
      </div>
      <div class="field">
        <button id="load-models" type="button">Load models</button>
      </div>
      <div class="field">
        <h3>Status</h3>
        <p id="providers-status" class="status-line" role="status" aria-live="polite">Loading providers...</p>
      </div>
      <div class="field">
        <h3>Model response</h3>
        <pre id="providers-models" role="status" aria-live="polite"></pre>
      </div>
      <div class="field">
        <h3>Configured providers</h3>
        <pre id="provider-config" role="status" aria-live="polite"></pre>
      </div>
    </div>
  `;
  renderProvidersData();
}

async function renderProvidersData() {
  const providerStatus = document.querySelector("#providers-status");
  const providerSelect = document.querySelector("#provider-list");
  const modelOutput = document.querySelector("#providers-models");
  const configOutput = document.querySelector("#provider-config");
  providerSelect.innerHTML = getProviders().map((provider) => `<option value="${escapeHtml(provider.id)}">${escapeHtml(provider.name || provider.id)}</option>`).join("");
  providerStatus.textContent = "Select a provider to load models.";
  configOutput.textContent = JSON.stringify(state.settings.providers, null, 2);
  const load = async () => {
    const providerID = providerSelect.value;
    providerStatus.textContent = `Loading models for ${providerID || "provider"}...`;
    modelOutput.textContent = "";
    try {
      const payload = await api(`/api/providers/${encodeURIComponent(providerID)}/models`);
      modelOutput.textContent = JSON.stringify(payload.models, null, 2);
      providerStatus.textContent = "Models loaded";
    } catch (error) {
      modelOutput.textContent = error.message;
      providerStatus.textContent = "Failed to load models";
    }
  };
  providerSelect.addEventListener("change", load);
  document.querySelector("#load-models").addEventListener("click", load);
  load();
}

function renderJobs() {
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

function renderTools() {
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

function renderSettings() {
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
  document.querySelector("#settings-save").addEventListener("click", async () => {
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
  });
}
