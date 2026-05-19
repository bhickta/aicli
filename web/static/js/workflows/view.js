import { api } from "../core/api.js";
import { view } from "../core/elements.js";
import { state, WORKFLOW_PREFIX } from "../core/state.js";
import { escapeHtml, enableTabKeyboard, syncTabStates } from "../core/utils.js";
import { setupProviderModel } from "../providers/controls.js";
import { setMarkdownPreview } from "../views/previews.js";
import { pickSystemDirectory, openBrowser } from "./browser.js";
import { WORKFLOW_CATEGORIES } from "./definitions/index.js";
import { buildWorkflowPayload, renderWorkflowField } from "./form.js";
import { pollWorkflowJob, renderWorkflowJob, setProgress } from "./jobs.js";
import { renderDropZone, setupDropZone } from "./upload/dropzone.js";
import { activeWorkflowCategoryDefinitions, getActiveWorkflow, workflowBrowseId } from "./utils.js";

export function renderWorkflows(renderPage) {
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
  setupWorkflowCategoryTabs(renderPage);
  setupWorkflowSelector(renderPage);
  setupWorkflowFields(activeWorkflow);
  setupRunButton(activeWorkflow);
  setupDropZone(renderPage);
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

function syncWorkflowCategoryTabs() {
  const categoryTabs = document.querySelector(".workflow-category-tabs");
  if (!categoryTabs) return;
  syncTabStates(categoryTabs, (tab) => tab.dataset.workflowCategory === state.workflow.category);
}

function setupWorkflowCategoryTabs(renderPage) {
  document.querySelectorAll(".workflow-category-tabs [role='tab']").forEach((button) => {
    button.addEventListener("click", () => {
      selectWorkflowCategory(button.dataset.workflowCategory);
      renderPage();
    });
  });
  enableTabKeyboard(document.querySelector(".workflow-category-tabs"), (tab) => {
    selectWorkflowCategory(tab.dataset.workflowCategory);
    renderPage();
  });
}

function setupWorkflowSelector(renderPage) {
  const workflowSelect = document.querySelector("#workflow-selection");
  if (!workflowSelect) return;
  workflowSelect.addEventListener("change", () => {
    state.workflow.workflowId = workflowSelect.value;
    renderPage();
  });
}

function selectWorkflowCategory(category) {
  state.workflow.category = category;
  state.workflow.workflowId = activeWorkflowCategoryDefinitions(category)[0].id;
}

function setupWorkflowFields(activeWorkflow) {
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
}

function setupRunButton(activeWorkflow) {
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
}
