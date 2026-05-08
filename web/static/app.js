const state = {
  view: "chat",
  settings: null,
  models: {},
  browser: {
    target: null,
    path: "",
  },
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
          <option value="/api/workflows/ocr/pdf">OCR: PDF to Markdown</option>
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
        <select id="model"><option value="${state.settings.default_model || ""}">${state.settings.default_model || "Load models..."}</option></select>
        <button id="refresh-models">Refresh models</button>
      </div>
      <div class="row">
        <select id="mode">
          <option value="">Default mode</option>
          <option value="rename">rename</option>
          <option value="junk">junk</option>
          <option value="digitize">digitize</option>
          <option value="notes">notes</option>
          <option value="tags">tags</option>
          <option value="course">course</option>
        </select>
        <label><input id="apply" type="checkbox" /> apply filesystem changes</label>
      </div>
      <div class="row">
        <button id="pick-path">Choose input</button>
        <button id="pick-output">Choose output / sidecar / asset dir</button>
      </div>
      <div class="row">
        <label>PDF render workers<input id="render-workers" type="number" min="1" max="64" value="2" /></label>
        <label>OCR workers<input id="ocr-workers" type="number" min="1" max="64" value="1" /></label>
      </div>
      <div id="drop-zone" class="drop-zone" tabindex="0">
        <strong>Drop a PDF here</strong>
        <span>PDFs auto-select OCR. ZIPs, images, audio, and video files are accepted too.</span>
      </div>
      <div class="row">
        <code id="path-display">No input selected</code>
        <code id="output-display">No output selected</code>
      </div>
      <textarea id="text" placeholder="transcript, notes, or JSON array input for advanced workflows"></textarea>
      <button id="run">Run</button>
      <div id="workflow-status" class="status-line">Ready</div>
      <div id="workflow-progress" class="progress hidden">
        <div></div>
      </div>
      <div id="review-pane" class="review-pane hidden">
        <iframe id="source-preview" title="Source PDF preview"></iframe>
        <textarea id="markdown-preview" readonly placeholder="Markdown result appears here"></textarea>
      </div>
      <pre id="workflow-result"></pre>
      <div id="file-browser" class="browser hidden"></div>
    </div>
  `;
  const providerSelect = document.querySelector("#provider");
  providerSelect.addEventListener("change", () => populateModelSelect(providerSelect.value, "#model"));
  document.querySelector("#refresh-models").addEventListener("click", () => populateModelSelect(providerSelect.value, "#model", true));
  populateModelSelect(providerSelect.value, "#model");
  document.querySelector("#pick-path").addEventListener("click", () => openBrowser("path"));
  document.querySelector("#pick-output").addEventListener("click", () => openBrowser("output"));
  setupDropZone();
  document.querySelector("#run").addEventListener("click", async () => {
    const endpoint = document.querySelector("#workflow").value;
    const output = document.querySelector("#workflow-result");
    const runButton = document.querySelector("#run");
    const status = document.querySelector("#workflow-status");
    const progressBar = document.querySelector("#workflow-progress");
    const payload = {
      provider_id: document.querySelector("#provider").value,
      model: document.querySelector("#model").value,
      path: document.querySelector("#path-display").dataset.path || "",
      mode: document.querySelector("#mode").value,
      output: document.querySelector("#output-display").dataset.path || "",
      output_path: document.querySelector("#output-display").dataset.path || "",
      sidecar: document.querySelector("#output-display").dataset.path || "",
      markdown_path: document.querySelector("#path-display").dataset.path || "",
      asset_dir: document.querySelector("#output-display").dataset.path || "",
      transcript: document.querySelector("#text").value,
      notes: document.querySelector("#text").value,
      track_text: document.querySelector("#text").value ? document.querySelector("#text").value.split("\\n---\\n") : [],
      render_workers: numberValue("#render-workers"),
      workers: numberValue("#ocr-workers"),
      apply: document.querySelector("#apply").checked,
      use_llm: Boolean(document.querySelector("#model").value),
    };
    runButton.disabled = true;
    status.textContent = "Running workflow...";
    setProgress(progressBar, 0);
    setMarkdownPreview("");
    output.textContent = "";
    try {
      const result = await api(endpoint, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      if (result.job?.id && result.job.status === "running") {
        await pollWorkflowJob(result.job.id, status, progressBar, output);
      } else {
        renderWorkflowJob(result.job, status, progressBar);
        setMarkdownPreview(result.result?.markdown || "");
        output.textContent = JSON.stringify(result.result || result, null, 2);
      }
    } catch (error) {
      status.textContent = "Failed";
      output.textContent = error.message;
    } finally {
      runButton.disabled = false;
    }
  });
}

function numberValue(selector) {
  const value = Number.parseInt(document.querySelector(selector)?.value || "", 10);
  return Number.isFinite(value) && value > 0 ? value : 0;
}

async function pollWorkflowJob(jobID, status, progressBar, output) {
  while (true) {
    await sleep(900);
    const job = await api(`/api/jobs/${encodeURIComponent(jobID)}`);
    renderWorkflowJob(job, status, progressBar);
    if (job.status === "completed") {
      const result = parseJobOutput(job.output);
      setMarkdownPreview(result?.markdown || "");
      output.textContent = result ? JSON.stringify(result, null, 2) : "";
      return;
    }
    if (job.status === "failed") {
      output.textContent = job.error || "Workflow failed";
      return;
    }
  }
}

function parseJobOutput(output) {
  if (!output) return null;
  try {
    return JSON.parse(output);
  } catch {
    return { output };
  }
}

function setSourcePreview(url) {
  const pane = document.querySelector("#review-pane");
  const frame = document.querySelector("#source-preview");
  if (!pane || !frame) return;
  if (!url) {
    frame.removeAttribute("src");
    return;
  }
  frame.src = url;
  pane.classList.remove("hidden");
}

function setMarkdownPreview(markdown) {
  const pane = document.querySelector("#review-pane");
  const preview = document.querySelector("#markdown-preview");
  if (!pane || !preview) return;
  preview.value = markdown || "";
  if (markdown || document.querySelector("#source-preview")?.getAttribute("src")) {
    pane.classList.remove("hidden");
  }
}

function renderWorkflowJob(job, status, progressBar) {
  if (!job) return;
  const percent = Math.round((job.progress || 0) * 100);
  const elapsed = elapsedSeconds(job.created_at);
  const eta = job.eta_seconds ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
  const step = job.total_steps ? ` | ${job.current_step}/${job.total_steps}` : "";
  status.textContent = `${job.status}: ${job.stage || "working"}${step} | ${percent}% | elapsed ${formatDuration(elapsed)}${eta}`;
  setProgress(progressBar, percent);
}

function setProgress(progressBar, percent) {
  if (!progressBar) return;
  progressBar.classList.remove("hidden");
  progressBar.firstElementChild.style.width = `${Math.max(0, Math.min(100, percent))}%`;
}

function elapsedSeconds(createdAt) {
  const started = Date.parse(createdAt);
  if (Number.isNaN(started)) return 0;
  return Math.max(0, Math.round((Date.now() - started) / 1000));
}

function formatDuration(seconds) {
  if (!seconds) return "0s";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (!mins) return `${secs}s`;
  const hours = Math.floor(mins / 60);
  if (!hours) return `${mins}m ${secs}s`;
  return `${hours}h ${mins % 60}m`;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function setupDropZone() {
  const dropZone = document.querySelector("#drop-zone");
  if (!dropZone) return;
  const stop = (event) => {
    event.preventDefault();
    event.stopPropagation();
  };
  ["dragenter", "dragover"].forEach((eventName) => {
    dropZone.addEventListener(eventName, (event) => {
      stop(event);
      dropZone.classList.add("dragover");
    });
  });
  ["dragleave", "drop"].forEach((eventName) => {
    dropZone.addEventListener(eventName, (event) => {
      stop(event);
      dropZone.classList.remove("dragover");
    });
  });
  dropZone.addEventListener("drop", async (event) => {
    const files = Array.from(event.dataTransfer.files || []);
    if (!files.length) return;
    await uploadWorkflowFile(files[0]);
  });
}

async function uploadWorkflowFile(file) {
  const status = document.querySelector("#workflow-status");
  const output = document.querySelector("#workflow-result");
  const dropZone = document.querySelector("#drop-zone");
  const form = new FormData();
  form.append("file", file);
  status.textContent = `Uploading ${file.name}...`;
  output.textContent = "";
  dropZone.classList.add("uploading");
  try {
    const response = await fetch("/api/fs/upload", { method: "POST", body: form });
    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(payload.error || response.statusText);
    }
    const result = await response.json();
    const uploaded = (result.files || [])[0];
    if (!uploaded) throw new Error("upload finished without a stored file");
    selectBrowserPath("path", uploaded.path, false);
    setSourcePreview(uploaded.url);
    autoSelectWorkflow(uploaded.name || file.name);
    status.textContent = `Ready: ${uploaded.name || file.name}`;
  } catch (error) {
    status.textContent = "Upload failed";
    output.textContent = error.message;
  } finally {
    dropZone.classList.remove("uploading");
  }
}

function autoSelectWorkflow(name) {
  const lower = name.toLowerCase();
  const workflow = document.querySelector("#workflow");
  if (!workflow) return;
  if (lower.endsWith(".pdf")) workflow.value = "/api/workflows/ocr/pdf";
  else if (lower.endsWith(".zip")) workflow.value = "/api/workflows/ocr/run";
  else if (/\.(png|jpe?g|webp|gif|bmp|tiff?)$/.test(lower)) workflow.value = "/api/workflows/image/run";
  else if (/\.(mp3|wav|m4a|flac|ogg|opus)$/.test(lower)) workflow.value = "/api/workflows/audio/transcribe";
  else if (/\.(mp4|mov|mkv|webm|avi)$/.test(lower)) workflow.value = "/api/workflows/video/info";
}

async function openBrowser(target, path = "") {
  state.browser.target = target;
  state.browser.path = path || state.browser.path;
  const browser = document.querySelector("#file-browser");
  browser.classList.remove("hidden");
  browser.innerHTML = `<p class="muted">Loading files...</p>`;
  try {
    const query = state.browser.path ? `?path=${encodeURIComponent(state.browser.path)}` : "";
    const result = await api(`/api/fs/list${query}`);
    state.browser.path = result.path;
    browser.innerHTML = `
      <div class="browser-header">
        <strong>${target === "path" ? "Choose input" : "Choose output / sidecar / asset dir"}</strong>
        ${target === "output" ? `<button id="use-current-directory">Use current directory</button>` : ""}
        <button id="close-browser">Close</button>
      </div>
      <code>${result.path}</code>
      <div class="browser-list">
        ${(result.entries || []).map((entry) => `
          <button class="browser-entry ${entry.is_dir ? "dir" : "file"}" data-path="${entry.path}" data-dir="${entry.is_dir}">
            ${entry.is_dir ? "▸" : "•"} ${entry.name}
          </button>
        `).join("")}
      </div>
    `;
    document.querySelector("#close-browser").addEventListener("click", () => browser.classList.add("hidden"));
    const useCurrentDirectory = document.querySelector("#use-current-directory");
    if (useCurrentDirectory) {
      useCurrentDirectory.addEventListener("click", () => selectBrowserPath(target, result.path));
    }
    browser.querySelectorAll(".browser-entry").forEach((button) => {
      button.addEventListener("click", () => {
        const selectedPath = button.dataset.path;
        const isDir = button.dataset.dir === "true";
        if (isDir) {
          openBrowser(target, selectedPath);
          return;
        }
        selectBrowserPath(target, selectedPath);
      });
      button.addEventListener("dblclick", () => selectBrowserPath(target, button.dataset.path));
    });
  } catch (error) {
    browser.innerHTML = `<pre>${error.message}</pre>`;
  }
}

function selectBrowserPath(target, selectedPath, hideBrowser = true) {
  const display = document.querySelector(target === "path" ? "#path-display" : "#output-display");
  display.dataset.path = selectedPath;
  display.textContent = selectedPath;
  const browser = document.querySelector("#file-browser");
  if (hideBrowser && browser) browser.classList.add("hidden");
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
