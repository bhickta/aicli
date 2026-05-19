const WORKFLOW_CATEGORIES = ["Study", "Documents", "Images", "Audio", "Video", "News"];

const WORKFLOW_DEFINITIONS = [
  {
    id: "recall",
    category: "Study",
    label: "Recall",
    endpoint: "/api/workflows/recall/run",
    fields: [
      { type: "providerModel" },
      { type: "textarea", id: "notes", label: "Notes", rows: 12, placeholder: "Paste UPSC notes..." },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      notes: values.notes,
    }),
  },
  {
    id: "video-generate",
    category: "Video",
    label: "Video notes/tags/course",
    endpoint: "/api/workflows/video/generate",
    fields: [
      { type: "providerModel" },
      { type: "text", id: "title", label: "Video title", value: "", placeholder: "Untitled video" },
      { type: "textarea", id: "transcript", label: "Transcript", rows: 14, placeholder: "Paste or generate transcript text..." },
      {
        type: "select",
        id: "mode",
        label: "Output mode",
        default: "notes",
        options: [
          { value: "notes", label: "Notes" },
          { value: "tags", label: "Tags" },
          { value: "course", label: "Course" },
        ],
      },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      title: values.title,
      transcript: values.transcript,
      mode: values.mode || "notes",
    }),
  },
  {
    id: "ocr-run",
    category: "Documents",
    label: "OCR: ZIP images to Markdown",
    endpoint: "/api/workflows/ocr/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input ZIP file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
  {
    id: "ocr-pdf",
    category: "Documents",
    label: "OCR: PDF to Markdown",
    endpoint: "/api/workflows/ocr/pdf",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input PDF file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
  {
    id: "analyze",
    category: "Documents",
    label: "Analyze: PDF report",
    endpoint: "/api/workflows/analyze/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input PDF file" },
      { type: "number", id: "render_workers", label: "Render workers", min: 1, max: 64, default: 2 },
      { type: "number", id: "workers", label: "OCR workers", min: 1, max: 64, default: 1 },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      render_workers: values.render_workers,
      workers: values.workers,
    }),
  },
  {
    id: "image-run",
    category: "Images",
    label: "Analyze image",
    endpoint: "/api/workflows/image/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input image" },
      {
        type: "select",
        id: "mode",
        label: "Mode",
        default: "",
        options: [
          { value: "", label: "Default (safe rename)" },
          { value: "rename", label: "Rename" },
          { value: "junk", label: "Junk classification" },
          { value: "digitize", label: "Digitize text" },
        ],
      },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      mode: values.mode || "",
    }),
  },
  {
    id: "image-rename",
    category: "Images",
    label: "Safe image rename",
    endpoint: "/api/workflows/image/rename",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input image" },
      { type: "checkbox", id: "apply", label: "Apply rename", checked: false },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      apply: Boolean(values.apply),
    }),
  },
  {
    id: "image-prune-refs",
    category: "Images",
    label: "Prune stale refs",
    endpoint: "/api/workflows/image/prune-refs",
    fields: [
      { type: "path", id: "markdown_path", label: "Markdown file" },
      { type: "path", id: "asset_dir", label: "Asset directory" },
      { type: "checkbox", id: "apply", label: "Apply filesystem changes", checked: false },
    ],
    buildPayload: (values) => ({
      markdown_path: values.markdown_path,
      asset_dir: values.asset_dir,
      apply: Boolean(values.apply),
    }),
  },
  {
    id: "audio-transcribe",
    category: "Audio",
    label: "Transcribe",
    endpoint: "/api/workflows/audio/transcribe",
    fields: [
      { type: "path", id: "path", label: "Input audio file" },
      { type: "text", id: "model", label: "Whisper model path (optional)", value: "", placeholder: "Leave blank to use whisper-cli default" },
    ],
    buildPayload: (values) => ({
      model: values.model,
      path: values.path,
    }),
  },
  {
    id: "audio-analyze",
    category: "Audio",
    label: "Analyze tracks",
    endpoint: "/api/workflows/audio/analyze",
    fields: [
      { type: "providerModel" },
      { type: "textarea", id: "transcript", label: "Transcript", rows: 10, placeholder: "Paste transcript(s), separated by blank line delimiter `---`" },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      track_text: values.transcript ? values.transcript.split("\n---\n").map((text) => text.trim()).filter(Boolean) : [],
      track_names: [],
    }),
  },
  {
    id: "video-info",
    category: "Video",
    label: "Info",
    endpoint: "/api/workflows/video/info",
    fields: [{ type: "path", id: "path", label: "Input video file" }],
    buildPayload: (values) => ({ path: values.path }),
  },
  {
    id: "video-course",
    category: "Video",
    label: "Course folder",
    endpoint: "/api/workflows/video/course",
    fields: [
      { type: "path", id: "path", label: "Course source folder", picker: "directory" },
      { type: "path", id: "output_dir", label: "Course output folder (optional)", picker: "directory" },
      {
        type: "select",
        id: "preset",
        label: "Compression preset",
        default: "slideshow",
        options: [
          { value: "slideshow", label: "Slideshow" },
          { value: "ultralight", label: "Ultralight" },
          { value: "light", label: "Light" },
          { value: "balanced", label: "Balanced" },
        ],
      },
      { type: "number", id: "resolution", label: "Resolution (0 keeps original for slideshow)", min: 0, max: 1080, default: 0 },
      { type: "number", id: "crf", label: "NVENC CQ quality (optional)", min: 0, max: 51, default: 0 },
      { type: "text", id: "fps", label: "Frame rate", value: "1/2", placeholder: "1/2" },
      { type: "number", id: "max_merge_hours", label: "Max hours per course part (0 = single file)", min: 0, max: 24, default: 0 },
      { type: "checkbox", id: "fast_skip", label: "Fast keyframe skip", checked: true },
    ],
    buildPayload: (values) => ({
      path: values.path,
      output_dir: values.output_dir,
      preset: values.preset || "slideshow",
      resolution: values.resolution,
      crf: values.crf,
      fps: values.fps || "1/2",
      max_merge_hours: values.max_merge_hours,
      fast_skip: Boolean(values.fast_skip),
    }),
  },
  {
    id: "video-compress",
    category: "Video",
    label: "Compress",
    endpoint: "/api/workflows/video/compress",
    fields: [
      { type: "path", id: "path", label: "Input video file" },
      { type: "path", id: "output", label: "Output video file (optional)" },
      {
        type: "select",
        id: "preset",
        label: "Compression preset",
        default: "light",
        options: [
          { value: "ultralight", label: "Ultralight" },
          { value: "light", label: "Light" },
          { value: "balanced", label: "Balanced" },
          { value: "slideshow", label: "Slideshow" },
        ],
      },
      { type: "number", id: "resolution", label: "Resolution", min: 0, max: 1080, default: 240 },
      { type: "number", id: "crf", label: "NVENC CQ quality (optional)", min: 0, max: 51, default: 0 },
      { type: "text", id: "fps", label: "Frame rate override (optional)", value: "", placeholder: "15" },
      { type: "checkbox", id: "fast_skip", label: "Fast keyframe skip", checked: false },
    ],
    buildPayload: (values) => ({
      path: values.path,
      output: values.output,
      crf: values.crf,
      preset: values.preset || "light",
      resolution: values.resolution,
      fps: values.fps,
      fast_skip: Boolean(values.fast_skip),
    }),
  },
  {
    id: "video-backup",
    category: "Video",
    label: "Metadata backup",
    endpoint: "/api/workflows/video/metadata/backup",
    fields: [
      { type: "path", id: "path", label: "Input video file" },
      { type: "path", id: "sidecar", label: "Sidecar file (optional)" },
    ],
    buildPayload: (values) => ({
      path: values.path,
      sidecar: values.sidecar,
      output: "",
    }),
  },
  {
    id: "video-restore",
    category: "Video",
    label: "Metadata restore",
    endpoint: "/api/workflows/video/metadata/restore",
    fields: [
      { type: "path", id: "path", label: "Input video file" },
      { type: "path", id: "sidecar", label: "Sidecar file" },
      { type: "path", id: "output", label: "Output video file (optional)" },
    ],
    buildPayload: (values) => ({
      path: values.path,
      sidecar: values.sidecar,
      output: values.output,
    }),
  },
  {
    id: "news",
    category: "News",
    label: "Analyze news feed",
    endpoint: "/api/workflows/news/run",
    fields: [
      { type: "providerModel" },
      { type: "path", id: "path", label: "Input JSON/XLSX file" },
      { type: "path", id: "output_path", label: "Output XLSX (optional)" },
      { type: "checkbox", id: "use_llm", label: "Use LLM summary", checked: true },
    ],
    buildPayload: (values) => ({
      provider_id: values.provider_id,
      model: values.model,
      path: values.path,
      output_path: values.output_path,
      use_llm: Boolean(values.model) && Boolean(values.use_llm),
    }),
  },
];

const state = {
  view: "chat",
  settings: null,
  models: {},
  browser: { target: null, path: "" },
  workflow: {
    category: "Study",
    workflowId: "recall",
    pathValues: {},
  },
};

const view = document.querySelector("#view");
const health = document.querySelector("#health");
const primaryTabs = document.querySelector("#primary-tabs");

const WORKFLOW_PREFIX = "wf";

function escapeHtml(value) {
  const text = String(value ?? "");
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function workflowControlId(id) {
  return `${WORKFLOW_PREFIX}-${id}`;
}

function workflowDisplayId(id) {
  return `${WORKFLOW_PREFIX}-${id}-value`;
}

function workflowBrowseId(id) {
  return `${WORKFLOW_PREFIX}-${id}-browse`;
}

function getProviders() {
  return state.settings?.providers || [];
}

function renderProviderOptions(selectedId = "") {
  const providers = getProviders();
  if (!providers.length) {
    return `<option value="">No providers configured</option>`;
  }
  return providers
    .map((provider) => {
      const id = escapeHtml(provider.id);
      const name = escapeHtml(provider.name || provider.id);
      const selected = provider.id === selectedId ? " selected" : "";
      return `<option value="${id}"${selected}>${name}</option>`;
    })
    .join("");
}

function defaultModelOption() {
  return state.settings?.default_model || "";
}

function defaultProviderId() {
  return state.settings?.default_provider || state.settings?.providers?.[0]?.id || "";
}

function activeWorkflowCategoryDefinitions(category) {
  return WORKFLOW_DEFINITIONS.filter((workflow) => workflow.category === category);
}

function getWorkflowById(workflowId) {
  return WORKFLOW_DEFINITIONS.find((workflow) => workflow.id === workflowId) || null;
}

function getActiveWorkflow() {
  const byCategory = activeWorkflowCategoryDefinitions(state.workflow.category);
  if (!byCategory.length) {
    state.workflow.category = WORKFLOW_CATEGORIES[0];
    state.workflow.workflowId = WORKFLOW_DEFINITIONS[0].id;
    return WORKFLOW_DEFINITIONS[0];
  }
  const match = getWorkflowById(state.workflow.workflowId);
  if (match && byCategory.some((workflow) => workflow.id === match.id)) {
    return match;
  }
  state.workflow.workflowId = byCategory[0].id;
  return byCategory[0];
}

function enableTabKeyboard(container, onActivate) {
  if (!container) return;
  container.addEventListener("keydown", (event) => {
    const tabs = Array.from(container.querySelectorAll('[role="tab"]'));
    const currentIndex = tabs.indexOf(document.activeElement);
    if (currentIndex === -1) return;
    let nextIndex = -1;
    if (event.key === "ArrowRight") {
      nextIndex = (currentIndex + 1) % tabs.length;
    } else if (event.key === "ArrowLeft") {
      nextIndex = (currentIndex - 1 + tabs.length) % tabs.length;
    } else if (event.key === "Home") {
      nextIndex = 0;
    } else if (event.key === "End") {
      nextIndex = tabs.length - 1;
    }
    if (nextIndex === -1) return;
    event.preventDefault();
    tabs[nextIndex].focus();
    if (onActivate) onActivate(tabs[nextIndex]);
  });
}

function syncTabStates(container, isActive) {
  const tabs = container.querySelectorAll('[role="tab"]');
  tabs.forEach((tab) => {
    const active = isActive(tab);
    tab.setAttribute("aria-selected", active ? "true" : "false");
    tab.tabIndex = active ? 0 : -1;
    tab.classList.toggle("active-tab", active);
  });
}

function syncPrimaryTabs() {
  if (!primaryTabs) return;
  syncTabStates(primaryTabs, (tab) => tab.dataset.view === state.view);
}

function syncWorkflowCategoryTabs() {
  const categoryTabs = document.querySelector(".workflow-category-tabs");
  if (!categoryTabs) return;
  syncTabStates(categoryTabs, (tab) => tab.dataset.workflowCategory === state.workflow.category);
}

function readNumberValue(value, fallback = 0, min = Number.NEGATIVE_INFINITY) {
  const parsed = Number.parseInt(value, 10);
  if (Number.isNaN(parsed)) return fallback;
  return parsed < min ? fallback : parsed;
}

function api(path, options = {}) {
  return fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  }).then(async (response) => {
    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(payload.error || response.statusText);
    }
    return response.json();
  });
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

function renderProviderModelRow(prefix, options = {}) {
  const providers = getProviders();
  const providerId = `${prefix}-provider`;
  const modelId = `${prefix}-model`;
  const refreshId = `${prefix}-refresh-models`;
  const defaultProvider = defaultProviderId();
  const modelDefault = defaultModelOption();
  const providerOptions = renderProviderOptions(defaultProvider);
  return `
    <div class="field-row">
      <div class="field">
        <label for="${providerId}">Provider</label>
        <select id="${providerId}">${providerOptions}</select>
      </div>
      <div class="field">
        <label for="${modelId}">Model</label>
        <select id="${modelId}">
          <option value="${escapeHtml(modelDefault)}">${escapeHtml(modelDefault || "Load models...")}</option>
        </select>
      </div>
      ${options.includeRefresh !== false ? `<div class="field"><button type="button" id="${refreshId}">Refresh models</button></div>` : ""}
    </div>
  `;
}

function setupProviderModel(prefix) {
  const providerSelect = document.querySelector(`#${prefix}-provider`);
  const modelSelect = document.querySelector(`#${prefix}-model`);
  const refreshButton = document.querySelector(`#${prefix}-refresh-models`);
  if (!providerSelect || !modelSelect) return;
  if (!providerSelect.value && providerSelect.options.length > 0) {
    providerSelect.value = providerSelect.options[0].value;
  }
  providerSelect.addEventListener("change", () => populateModelSelect(providerSelect.value, `#${modelSelect.id}`));
  if (refreshButton) {
    refreshButton.addEventListener("click", () => populateModelSelect(providerSelect.value, `#${modelSelect.id}`, true));
  }
  populateModelSelect(providerSelect.value, `#${modelSelect.id}`);
}

function getProviderModelValues(prefix) {
  const providerSelect = document.querySelector(`#${prefix}-provider`);
  const modelSelect = document.querySelector(`#${prefix}-model`);
  return {
    provider_id: providerSelect?.value || "",
    model: modelSelect?.value || "",
  };
}

function renderWorkflowField(field) {
  if (field.type === "providerModel") {
    return renderProviderModelRow(WORKFLOW_PREFIX);
  }
  if (field.type === "text") {
    const id = workflowControlId(field.id);
    const value = field.value || "";
    return `
      <div class="field">
        <label for="${id}">${field.label}</label>
        <input id="${id}" type="text" value="${escapeHtml(value)}" placeholder="${escapeHtml(field.placeholder || "")}">
      </div>
    `;
  }
  if (field.type === "textarea") {
    const id = workflowControlId(field.id);
    return `
      <div class="field">
        <label for="${id}">${field.label}</label>
        <textarea id="${id}" rows="${field.rows || 6}" placeholder="${escapeHtml(field.placeholder || "")}"></textarea>
      </div>
    `;
  }
  if (field.type === "select") {
    const id = workflowControlId(field.id);
    const options = field.options || [];
    return `
      <div class="field">
        <label for="${id}">${field.label}</label>
        <select id="${id}">
          ${options
            .map((option) => {
              const selected = option.value === (field.default ?? "") ? " selected" : "";
              return `<option value="${escapeHtml(option.value)}"${selected}>${escapeHtml(option.label)}</option>`;
            })
            .join("")}
        </select>
      </div>
    `;
  }
  if (field.type === "number") {
    const id = workflowControlId(field.id);
    const min = Number.isFinite(field.min) ? field.min : "";
    const max = Number.isFinite(field.max) ? field.max : "";
    const value = Number.isFinite(field.default) ? field.default : "";
    return `
      <div class="field">
        <label for="${id}">${field.label}</label>
        <input id="${id}" type="number" min="${min}" max="${max}" value="${escapeHtml(value)}">
      </div>
    `;
  }
  if (field.type === "checkbox") {
    const id = workflowControlId(field.id);
    const checked = field.checked ? " checked" : "";
    return `
      <div class="field">
        <label class="checkbox">
          <input id="${id}" type="checkbox" ${checked}>
          <span>${field.label}</span>
        </label>
      </div>
    `;
  }
  if (field.type === "path") {
    const browseId = workflowBrowseId(field.id);
    const valueId = workflowDisplayId(field.id);
    const currentPath = state.workflow.pathValues[field.id] || "";
    const chooseLabel = field.picker === "directory" ? `Browse ${field.label}` : `Choose ${field.label}`;
    return `
      <div class="field">
        <label for="${valueId}">${field.label}</label>
        <div class="path-control">
          <output id="${valueId}" data-path="${escapeHtml(currentPath)}">${escapeHtml(currentPath || `No ${field.label.toLowerCase()} selected`)}</output>
          <button type="button" id="${browseId}" data-workflow-target="${escapeHtml(field.id)}">${escapeHtml(chooseLabel)}</button>
        </div>
      </div>
    `;
  }
  return "";
}

function renderWorkflowCategoryTabs() {
  return `
    <div class="tabs workflow-category-tabs" role="tablist" aria-label="Workflow categories">
      ${WORKFLOW_CATEGORIES.map((category) => `
        <button type="button" role="tab" class="workflow-tab" data-workflow-category="${category}" aria-selected="false">
          ${escapeHtml(category)}
        </button>
      `).join("")}
    </div>
  `;
}

function renderWorkflowSelect(activeWorkflow) {
  const items = activeWorkflowCategoryDefinitions(state.workflow.category);
  return `
    <div class="field">
      <label for="workflow-selection">Workflow</label>
      <select id="workflow-selection">
        ${items.map((workflow) => `<option value="${escapeHtml(workflow.id)}"${workflow.id === activeWorkflow.id ? " selected" : ""}>${escapeHtml(workflow.label)}</option>`).join("")}
      </select>
    </div>
  `;
}

function collectWorkflowInputValues(definition) {
  const values = {};
  definition.fields.forEach((field) => {
    if (field.type === "providerModel") {
      Object.assign(values, getProviderModelValues(WORKFLOW_PREFIX));
      return;
    }
    const id = workflowControlId(field.id);
    const element = document.querySelector(`#${id}`);
    if (!element) return;
    if (field.type === "checkbox") {
      values[field.id] = element.checked;
      return;
    }
    if (field.type === "path") {
      const display = document.querySelector(`#${workflowDisplayId(field.id)}`);
      values[field.id] = display?.dataset.path || "";
      return;
    }
    if (field.type === "number") {
      values[field.id] = readNumberValue(element.value, field.default || 0, field.min || 0);
      return;
    }
    values[field.id] = element.value || "";
  });
  return values;
}

function setWorkflowPathValue(target, pathValue) {
  state.workflow.pathValues[target] = pathValue || "";
  const display = document.querySelector(`#${workflowDisplayId(target)}`);
  if (!display) return;
  display.dataset.path = pathValue;
  display.textContent = pathValue || "No path selected";
}

function getWorkflowPathValue(target) {
  const display = document.querySelector(`#${workflowDisplayId(target)}`);
  return display?.dataset.path || "";
}

function buildWorkflowPayload(definition) {
  const inputValues = collectWorkflowInputValues(definition);
  if (definition.buildPayload) {
    return definition.buildPayload(inputValues);
  }
  return inputValues;
}

async function render() {
  if (!state.settings) {
    await loadSettings();
  }
  syncPrimaryTabs();
  if (state.view === "providers") return renderProviders();
  if (state.view === "tools") return renderTools();
  if (state.view === "jobs") return renderJobs();
  if (state.view === "settings") return renderSettings();
  if (state.view === "workflows") return renderWorkflows();
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

function renderWorkflows() {
  const activeWorkflow = getActiveWorkflow();
  view.innerHTML = `
    <div class="panel grid">
      <h2>Workflows</h2>
      ${renderWorkflowCategoryTabs()}
      ${renderWorkflowSelect(activeWorkflow)}
      ${renderDropZone()}
      <p class="muted">Workflow controls are shown only for the selected workflow. Recall is included in the <strong>Study</strong> workflow set.</p>
      <div id="workflow-fields" class="grid">
        ${activeWorkflow.fields.map(renderWorkflowField).join("")}
      </div>
      <div class="field">
        <button id="workflow-run" type="button">Run workflow</button>
      </div>
      <div class="field">
        <h3>Status</h3>
        <p id="workflow-status" class="status-line" role="status" aria-live="polite">Ready</p>
      </div>
      <div id="workflow-progress" class="progress hidden">
        <div></div>
      </div>
      <div class="field">
        <h3>Result</h3>
        <pre id="workflow-result" role="status" aria-live="polite"></pre>
      </div>
      <div id="review-pane" class="review-pane hidden">
        <iframe id="source-preview" title="Source file preview"></iframe>
        <textarea id="markdown-preview" readonly placeholder="Markdown preview appears here"></textarea>
      </div>
      <div id="file-browser" class="browser hidden" aria-live="polite"></div>
    </div>
  `;
  syncWorkflowCategoryTabs();

  document.querySelectorAll(".workflow-category-tabs [role='tab']").forEach((button) => {
    button.addEventListener("click", () => {
      state.workflow.category = button.dataset.workflowCategory;
      state.workflow.workflowId = activeWorkflowCategoryDefinitions(state.workflow.category)[0].id;
      render();
    });
  });
  enableTabKeyboard(document.querySelector(".workflow-category-tabs"), (tab) => {
    const nextCategory = tab.dataset.workflowCategory;
    state.workflow.category = nextCategory;
    state.workflow.workflowId = activeWorkflowCategoryDefinitions(nextCategory)[0].id;
    render();
  });
  const workflowSelect = document.querySelector("#workflow-selection");
  if (workflowSelect) {
    workflowSelect.addEventListener("change", () => {
      state.workflow.workflowId = workflowSelect.value;
      render();
    });
  }
  if (activeWorkflow.fields.some((field) => field.type === "providerModel")) {
    setupProviderModel(WORKFLOW_PREFIX);
  }
  activeWorkflow.fields.forEach((field) => {
    if (field.type !== "path") return;
    const pickButton = document.querySelector(`#${workflowBrowseId(field.id)}`);
    if (!pickButton) return;
    pickButton.addEventListener("click", () => {
      if (field.picker === "directory") {
        pickSystemDirectory(field.id, field.label);
        return;
      }
      openBrowser(field.id, field.label);
    });
  });
  const runButton = document.querySelector("#workflow-run");
  const result = document.querySelector("#workflow-result");
  const progressBar = document.querySelector("#workflow-progress");
  runButton.addEventListener("click", async () => {
    const status = document.querySelector("#workflow-status");
    runButton.disabled = true;
    status.textContent = "Running workflow...";
    setProgress(progressBar, 0);
    result.textContent = "";
    setMarkdownPreview("");
    try {
      const payload = buildWorkflowPayload(activeWorkflow);
      const response = await api(activeWorkflow.endpoint, {
        method: "POST",
        body: JSON.stringify(payload),
      });
      if (response.job?.id && response.job.status === "running") {
        await pollWorkflowJob(response.job.id, status, progressBar, result);
      return;
      }
      renderWorkflowJob(response.job, status, progressBar);
      setMarkdownPreview(response.result?.markdown || "");
      result.textContent = JSON.stringify(response.result || response, null, 2);
      status.textContent = "Completed";
    } catch (error) {
      status.textContent = "Failed";
      result.textContent = error.message;
    } finally {
      runButton.disabled = false;
    }
  });
  setupDropZone();
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

function setProgress(progressBar, percent) {
  if (!progressBar) return;
  progressBar.classList.remove("hidden");
  progressBar.firstElementChild.style.width = `${Math.max(0, Math.min(100, percent))}%`;
}

function pollWorkflowJob(jobID, status, progressBar, output) {
  return (async () => {
    while (true) {
      await sleep(900);
      const job = await api(`/api/jobs/${encodeURIComponent(jobID)}`);
      renderWorkflowJob(job, status, progressBar);
      if (job.status === "completed") {
        const result = parseJobOutput(job.output);
        setMarkdownPreview(result?.markdown || "");
        output.textContent = result ? JSON.stringify(result, null, 2) : "";
        status.textContent = "Completed";
        return;
      }
      if (job.status === "failed") {
        output.textContent = job.error || "Workflow failed";
        status.textContent = "Failed";
        return;
      }
    }
  })();
}

function parseJobOutput(output) {
  if (!output) return null;
  try {
    return JSON.parse(output);
  } catch {
    return { output };
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

async function uploadWorkflowFile(file) {
  return uploadWorkflowFiles([{ file, relativePath: file.name }]);
}

async function uploadWorkflowFiles(entries) {
  const status = document.querySelector("#workflow-status");
  const output = document.querySelector("#workflow-result");
  const dropZone = document.querySelector("#drop-zone");
  const isFolderDrop = entries.some((entry) => (entry.relativePath || "").includes("/"));
  if (isFolderDrop) {
    state.workflow.category = "Video";
    state.workflow.workflowId = "video-course";
    if (state.view === "workflows") {
      render();
    }
    const currentStatus = document.querySelector("#workflow-status");
    const currentOutput = document.querySelector("#workflow-result");
    if (currentStatus) currentStatus.textContent = "Folder not uploaded";
    if (currentOutput) {
      currentOutput.textContent = "For in-place processing, click Choose Course source folder, navigate to the folder, then use current directory.";
    }
    return;
  }
  const uploadLabel = entries.length === 1 ? entries[0].file.name : `${entries.length} files`;
  if (status) status.textContent = `Uploading ${uploadLabel}...`;
  if (output) output.textContent = "";
  if (dropZone) dropZone.classList.add("uploading");
  try {
    const form = new FormData();
    entries.forEach((entry) => {
      const relativePath = entry.relativePath || entry.file.name;
      const fieldName = relativePath.includes("/") ? `file:${relativePath}` : "file";
      form.append(fieldName, entry.file, entry.file.name);
    });
    const response = await fetch("/api/fs/upload", { method: "POST", body: form });
    if (!response.ok) {
      const payload = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(payload.error || response.statusText);
    }
    const result = await response.json();
    const uploadedFiles = result.files || [];
    const uploaded = uploadedFiles[0];
    if (!uploaded) throw new Error("upload finished without a stored file");
    const hasCourseVideos = uploadedFiles.filter((file) => isVideoFileName(file.name)).length > 0;
    if (hasCourseVideos && uploadedFiles.length > 1 && result.root) {
      state.workflow.category = "Video";
      state.workflow.workflowId = "video-course";
      if (state.view === "workflows") {
        render();
      }
      setWorkflowPathValue("path", result.root);
      const currentStatus = document.querySelector("#workflow-status");
      if (currentStatus) currentStatus.textContent = `Ready: uploaded folder with ${uploadedFiles.length} files`;
      return;
    }
    autoSelectWorkflow(uploaded.name || entries[0].file.name);
    const workflow = getActiveWorkflow();
    const primaryPathField = workflow.fields.find((field) => field.type === "path");
    if (primaryPathField) {
      setWorkflowPathValue(primaryPathField.id, uploaded.path);
      setSourcePreview(uploaded.url);
    }
    const currentStatus = document.querySelector("#workflow-status");
    if (currentStatus) {
      currentStatus.textContent = `Ready: ${uploaded.name || entries[0].file.name}`;
    }
  } catch (error) {
    const currentStatus = document.querySelector("#workflow-status");
    const currentOutput = document.querySelector("#workflow-result");
    if (currentStatus) currentStatus.textContent = "Upload failed";
    if (currentOutput) currentOutput.textContent = error.message;
  } finally {
    const currentDropZone = document.querySelector("#drop-zone") || dropZone;
    if (currentDropZone) currentDropZone.classList.remove("uploading");
  }
}

function autoSelectWorkflow(fileName) {
  const lower = fileName.toLowerCase();
  if (lower.endsWith(".pdf")) {
    state.workflow.workflowId = "ocr-pdf";
    state.workflow.category = "Documents";
  } else if (lower.endsWith(".zip")) {
    state.workflow.workflowId = "ocr-run";
    state.workflow.category = "Documents";
  } else if (/\.(png|jpe?g|webp|gif|bmp|tiff?)$/.test(lower)) {
    state.workflow.workflowId = "image-run";
    state.workflow.category = "Images";
  } else if (/\.(mp3|wav|m4a|flac|ogg|opus)$/.test(lower)) {
    state.workflow.workflowId = "audio-transcribe";
    state.workflow.category = "Audio";
  } else if (/\.(mp4|mov|mkv|webm|avi|m4v)$/.test(lower)) {
    state.workflow.workflowId = "video-info";
    state.workflow.category = "Video";
  }
  if (state.view === "workflows") {
    render();
  }
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
    const entries = await collectDroppedFiles(event.dataTransfer);
    if (!entries.length) return;
    await uploadWorkflowFiles(entries);
  });
}

function renderDropZone() {
  return `
    <div id="drop-zone" class="drop-zone" tabindex="0">
      <strong>Drop a file to auto-select workflow</strong>
      <span>Single files upload to app storage. For in-place video course folders, use Choose Course source folder.</span>
    </div>
  `;
}

async function collectDroppedFiles(dataTransfer) {
  const items = Array.from(dataTransfer?.items || []);
  const entries = [];
  const readers = items
    .map((item) => (typeof item.webkitGetAsEntry === "function" ? item.webkitGetAsEntry() : null))
    .filter(Boolean);
  if (readers.length) {
    for (const entry of readers) {
      await collectEntryFiles(entry, "", entries);
    }
  }
  if (!entries.length) {
    Array.from(dataTransfer?.files || []).forEach((file) => {
      entries.push({ file, relativePath: file.webkitRelativePath || file.name });
    });
  }
  return entries;
}

function collectEntryFiles(entry, prefix, out) {
  return new Promise((resolve, reject) => {
    if (entry.isFile) {
      entry.file(
        (file) => {
          out.push({ file, relativePath: `${prefix}${file.name}` });
          resolve();
        },
        reject,
      );
      return;
    }
    if (!entry.isDirectory) {
      resolve();
      return;
    }
    const reader = entry.createReader();
    const directoryPrefix = `${prefix}${entry.name}/`;
    const readBatch = () => {
      reader.readEntries(
        async (children) => {
          if (!children.length) {
            resolve();
            return;
          }
          try {
            for (const child of children) {
              await collectEntryFiles(child, directoryPrefix, out);
            }
            readBatch();
          } catch (error) {
            reject(error);
          }
        },
        reject,
      );
    };
    readBatch();
  });
}

function isVideoFileName(name) {
  return /\.(mp4|mov|mkv|webm|avi|m4v)$/i.test(name || "");
}

async function openBrowser(target, targetLabel, path = "") {
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
        <strong>Choose ${escapeHtml(targetLabel || target)}</strong>
        <div>
          <button id="use-current-directory" type="button">Use current directory</button>
          <button id="close-browser" type="button">Close</button>
        </div>
      </div>
      <code>${escapeHtml(result.path)}</code>
      <div class="browser-list">
        ${(result.entries || []).map((entry) => `
          <button class="browser-entry ${entry.is_dir ? "dir" : "file"}" data-path="${escapeHtml(entry.path)}" data-dir="${entry.is_dir}">
            ${entry.is_dir ? "▸" : "•"} ${escapeHtml(entry.name)}
          </button>
        `).join("")}
      </div>
    `;
    const closeButton = document.querySelector("#close-browser");
    if (closeButton) {
      closeButton.addEventListener("click", () => browser.classList.add("hidden"));
    }
    const useCurrentDirectory = document.querySelector("#use-current-directory");
    if (useCurrentDirectory) {
      useCurrentDirectory.addEventListener("click", () => selectBrowserPath(target, result.path));
    }
    browser.querySelectorAll(".browser-entry").forEach((button) => {
      button.addEventListener("click", () => {
        const selectedPath = button.dataset.path;
        const isDir = button.dataset.dir === "true";
        if (isDir) {
          openBrowser(target, targetLabel, selectedPath);
          return;
        }
        selectBrowserPath(target, selectedPath);
      });
      button.addEventListener("dblclick", () => selectBrowserPath(target, button.dataset.path));
    });
  } catch (error) {
    browser.innerHTML = `<pre>${escapeHtml(error.message)}</pre>`;
  }
}

async function pickSystemDirectory(target, targetLabel) {
  const status = document.querySelector("#workflow-status");
  const output = document.querySelector("#workflow-result");
  const currentPath = state.workflow.pathValues[target] || state.browser.path || "";
  if (status) status.textContent = `Opening system folder picker for ${targetLabel}...`;
  try {
    const query = currentPath ? `?path=${encodeURIComponent(currentPath)}` : "";
    const result = await api(`/api/fs/pick-directory${query}`);
    setWorkflowPathValue(target, result.path);
    state.browser.path = result.path;
    if (status) status.textContent = `Selected ${targetLabel}`;
    if (output && output.textContent.includes("For in-place processing")) {
      output.textContent = "";
    }
  } catch (error) {
    if (status) status.textContent = "Folder picker failed";
    if (output) output.textContent = error.message;
  }
}

function selectBrowserPath(target, selectedPath, hideBrowser = true) {
  setWorkflowPathValue(target, selectedPath);
  const browser = document.querySelector("#file-browser");
  if (hideBrowser && browser) {
    browser.classList.add("hidden");
  }
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

function handlePrimaryTab(event) {
  const button = event.target.closest('[role="tab"]');
  if (!button || !button.dataset.view) return;
  event.preventDefault();
  state.view = button.dataset.view;
  render();
}

function initNav() {
  if (!primaryTabs) return;
  primaryTabs.addEventListener("click", handlePrimaryTab);
  enableTabKeyboard(primaryTabs, (tab) => {
    state.view = tab.dataset.view;
    render();
  });
}

function populateModelSelect(providerID, selector, force = false) {
  const select = document.querySelector(selector);
  if (!select) return;
  if (!providerID) {
    select.innerHTML = `<option value="">Select a provider first</option>`;
    return;
  }
  if (!state.models[providerID] || force) {
    select.innerHTML = `<option>Loading models...</option>`;
    const load = async () => {
      const result = await api(`/api/providers/${encodeURIComponent(providerID)}/models`);
      state.models[providerID] = result.models || [];
      if (!state.models[providerID].length) {
        select.innerHTML = `<option value="">No models returned</option>`;
        return;
      }
      select.innerHTML = state.models[providerID]
        .map((m) => `<option value="${escapeHtml(m.id)}">${escapeHtml(m.name || m.id)}</option>`)
        .join("");
    };
    load().catch((error) => {
      select.innerHTML = `<option value="">${escapeHtml(error.message)}</option>`;
    });
    return;
  }
  if (!state.models[providerID].length) {
    select.innerHTML = `<option value="">No models returned</option>`;
    return;
  }
  select.innerHTML = state.models[providerID]
    .map((m) => `<option value="${escapeHtml(m.id)}">${escapeHtml(m.name || m.id)}</option>`)
    .join("");
}

initNav();
checkHealth();
render();
