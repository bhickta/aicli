const state = {
  view: "chat",
  settings: null,
  models: {},
};

const view = document.querySelector("#view");
const health = document.querySelector("#health");

document.querySelectorAll("aside button").forEach((button) => {
  button.addEventListener("click", () => {
    state.view = button.dataset.view;
    render();
  });
});

async function api(path, options = {}) {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json();
}

async function loadSettings() {
  state.settings = await api("/api/settings");
}

async function checkHealth() {
  try {
    const result = await api("/api/health");
    health.textContent = result.status;
  } catch {
    health.textContent = "offline";
  }
}

async function render() {
  if (!state.settings) {
    await loadSettings();
  }
  if (state.view === "providers") return renderProviders();
  if (state.view === "tools") return renderTools();
  if (state.view === "recall") return renderRecall();
  if (state.view === "workflows") return renderWorkflows();
  if (state.view === "jobs") return renderJobs();
  if (state.view === "settings") return renderSettings();
  return renderChat();
}

function renderChat() {
  const providers = state.settings.providers || [];
  view.innerHTML = `
    <div class="panel grid">
      <div class="row">
        <select id="provider">${providers.map((p) => `<option value="${p.id}">${p.name || p.id}</option>`).join("")}</select>
        <select id="model"><option value="${state.settings.default_model || ""}">${state.settings.default_model || "Load models..."}</option></select>
        <button id="refresh-models">Refresh models</button>
      </div>
      <textarea id="prompt" placeholder="Ask a local model..."></textarea>
      <button id="send">Send</button>
      <pre id="answer"></pre>
    </div>
  `;
  const providerSelect = document.querySelector("#provider");
  providerSelect.addEventListener("change", () => populateModelSelect(providerSelect.value, "#model"));
  document.querySelector("#refresh-models").addEventListener("click", () => populateModelSelect(providerSelect.value, "#model", true));
  populateModelSelect(providerSelect.value, "#model");
  document.querySelector("#send").addEventListener("click", async () => {
    const answer = document.querySelector("#answer");
    answer.textContent = "thinking...";
    try {
      const result = await api("/api/chat", {
        method: "POST",
        body: JSON.stringify({
          provider_id: document.querySelector("#provider").value,
          model: document.querySelector("#model").value,
          messages: [{ role: "user", content: document.querySelector("#prompt").value }],
          temperature: 0.1,
        }),
      });
      answer.textContent = result.content;
    } catch (error) {
      answer.textContent = error.message;
    }
  });
}

function renderRecall() {
  const providers = state.settings.providers || [];
  view.innerHTML = `
    <div class="panel grid">
      <div class="row">
        <select id="provider">${providers.map((p) => `<option value="${p.id}">${p.name || p.id}</option>`).join("")}</select>
        <select id="model"><option value="${state.settings.default_model || ""}">${state.settings.default_model || "Load models..."}</option></select>
        <button id="refresh-models">Refresh models</button>
      </div>
      <textarea id="notes" placeholder="Paste UPSC notes..."></textarea>
      <button id="generate">Generate recall triggers</button>
      <pre id="triggers"></pre>
    </div>
  `;
  const providerSelect = document.querySelector("#provider");
  providerSelect.addEventListener("change", () => populateModelSelect(providerSelect.value, "#model"));
  document.querySelector("#refresh-models").addEventListener("click", () => populateModelSelect(providerSelect.value, "#model", true));
  populateModelSelect(providerSelect.value, "#model");
  document.querySelector("#generate").addEventListener("click", async () => {
    const output = document.querySelector("#triggers");
    output.textContent = "generating...";
    try {
      const result = await api("/api/workflows/recall/run", {
        method: "POST",
        body: JSON.stringify({
          provider_id: document.querySelector("#provider").value,
          model: document.querySelector("#model").value,
          notes: document.querySelector("#notes").value,
        }),
      });
      output.textContent = result.result.triggers;
    } catch (error) {
      output.textContent = error.message;
    }
  });
}

function renderWorkflows() {
  const providers = state.settings.providers || [];
  view.innerHTML = `
    <div class="panel grid">
      <p class="muted">Run local workflows against files on this machine. Provider/model fields are used by LLM and vision workflows.</p>
      <div class="row">
        <select id="workflow">
          <option value="/api/workflows/image/run">Image: rename/junk/digitize</option>
          <option value="/api/workflows/image/rename">Image: apply safe rename</option>
          <option value="/api/workflows/image/prune-refs">Image: prune stale refs</option>
          <option value="/api/workflows/ocr/run">OCR: ZIP images to Markdown</option>
          <option value="/api/workflows/analyze/run">Analyze: PDF to report</option>
          <option value="/api/workflows/news/run">News: dedupe/merge JSON/XLSX</option>
          <option value="/api/workflows/video/info">Video: info</option>
          <option value="/api/workflows/video/compress">Video: compress</option>
          <option value="/api/workflows/video/metadata/backup">Video: metadata backup</option>
          <option value="/api/workflows/video/metadata/restore">Video: metadata restore</option>
          <option value="/api/workflows/video/generate">Video: notes/tags/course</option>
          <option value="/api/workflows/audio/transcribe">Audio: transcribe</option>
          <option value="/api/workflows/audio/analyze">Audio: analyze/playlists</option>
        </select>
        <select id="provider">${providers.map((p) => `<option value="${p.id}">${p.name || p.id}</option>`).join("")}</select>
        <input id="model" placeholder="model name" value="${state.settings.default_model || ""}" />
      </div>
      <div class="row">
        <input id="path" placeholder="file path" />
        <input id="mode" placeholder="mode, e.g. rename | junk | digitize | notes | tags | course" />
        <input id="output" placeholder="output path, sidecar, markdown path, or asset dir" />
      </div>
      <textarea id="text" placeholder="transcript, notes, or JSON array input for advanced workflows"></textarea>
      <label><input id="apply" type="checkbox" /> apply filesystem changes</label>
      <button id="run">Run</button>
      <pre id="workflow-result"></pre>
    </div>
  `;
  document.querySelector("#run").addEventListener("click", async () => {
    const endpoint = document.querySelector("#workflow").value;
    const output = document.querySelector("#workflow-result");
    const payload = {
      provider_id: document.querySelector("#provider").value,
      model: document.querySelector("#model").value,
      path: document.querySelector("#path").value,
      mode: document.querySelector("#mode").value,
      output: document.querySelector("#output").value,
      output_path: document.querySelector("#output").value,
      sidecar: document.querySelector("#output").value,
      markdown_path: document.querySelector("#path").value,
      asset_dir: document.querySelector("#output").value,
      transcript: document.querySelector("#text").value,
      notes: document.querySelector("#text").value,
      track_text: document.querySelector("#text").value ? document.querySelector("#text").value.split("\\n---\\n") : [],
      apply: document.querySelector("#apply").checked,
      use_llm: Boolean(document.querySelector("#model").value),
    };
    output.textContent = "running...";
    try {
      const result = await api(endpoint, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      output.textContent = JSON.stringify(result, null, 2);
    } catch (error) {
      output.textContent = error.message;
    }
  });
}

async function renderProviders() {
  const payload = await api("/api/providers");
  view.innerHTML = `
    <div class="panel grid">
      <div class="row">
        <select id="provider-list">${payload.providers.map((p) => `<option value="${p.id}">${p.name || p.id}</option>`).join("")}</select>
        <button id="load-models">Load models</button>
      </div>
      <pre id="provider-config">${JSON.stringify(payload.providers, null, 2)}</pre>
      <pre id="provider-models"></pre>
    </div>
  `;
  const load = async () => {
    const id = document.querySelector("#provider-list").value;
    const output = document.querySelector("#provider-models");
    output.textContent = "loading...";
    try {
      const result = await api(`/api/providers/${encodeURIComponent(id)}/models`);
      output.textContent = JSON.stringify(result.models, null, 2);
    } catch (error) {
      output.textContent = error.message;
    }
  };
  document.querySelector("#provider-list").addEventListener("change", load);
  document.querySelector("#load-models").addEventListener("click", load);
  load();
}

async function populateModelSelect(providerID, selector, force = false) {
  const select = document.querySelector(selector);
  if (!select || !providerID) return;
  if (!state.models[providerID] || force) {
    select.innerHTML = `<option>Loading models...</option>`;
    try {
      const result = await api(`/api/providers/${encodeURIComponent(providerID)}/models`);
      state.models[providerID] = result.models || [];
    } catch (error) {
      select.innerHTML = `<option value="">${error.message}</option>`;
      return;
    }
  }
  const models = state.models[providerID];
  if (!models.length) {
    select.innerHTML = `<option value="">No models returned</option>`;
    return;
  }
  select.innerHTML = models.map((m) => `<option value="${m.id}">${m.name || m.id}</option>`).join("");
}

async function renderTools() {
  const payload = await api("/api/tools");
  view.innerHTML = `<div class="panel"><pre>${JSON.stringify(payload.tools, null, 2)}</pre></div>`;
}

async function renderJobs() {
  const payload = await api("/api/jobs");
  view.innerHTML = `<div class="panel"><pre>${JSON.stringify(payload.jobs, null, 2)}</pre></div>`;
}

function renderSettings() {
  view.innerHTML = `
    <div class="panel grid">
      <textarea id="settings">${JSON.stringify(state.settings, null, 2)}</textarea>
      <button id="save">Save</button>
      <pre id="save-result"></pre>
    </div>
  `;
  document.querySelector("#save").addEventListener("click", async () => {
    const output = document.querySelector("#save-result");
    try {
      state.settings = await api("/api/settings", {
        method: "PUT",
        body: document.querySelector("#settings").value,
      });
      output.textContent = "saved";
    } catch (error) {
      output.textContent = error.message;
    }
  });
}

checkHealth();
render();
