import { api } from "../core/api.js";
import { state } from "../core/state.js";
import { escapeHtml } from "../core/utils.js";
import { setWorkflowPathValue } from "./form.js";

export async function openBrowser(target, targetLabel, path = "") {
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
            ${entry.is_dir ? ">" : "-"} ${escapeHtml(entry.name)}
          </button>
        `).join("")}
      </div>
    `;
    document.querySelector("#close-browser")?.addEventListener("click", () => browser.classList.add("hidden"));
    document.querySelector("#use-current-directory")?.addEventListener("click", () => selectBrowserPath(target, result.path));
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

export async function pickSystemDirectory(target, targetLabel) {
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

export function selectBrowserPath(target, selectedPath, hideBrowser = true) {
  setWorkflowPathValue(target, selectedPath);
  const browser = document.querySelector("#file-browser");
  if (hideBrowser && browser) {
    browser.classList.add("hidden");
  }
}
