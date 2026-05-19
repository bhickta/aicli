import { api } from "../core/api.js";
import { state, WORKFLOW_PREFIX } from "../core/state.js";
import { escapeHtml } from "../core/utils.js";

export function getProviders() {
  return state.settings?.providers || [];
}

export function renderProviderOptions(selectedId = "") {
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

export function defaultModelOption() {
  return state.settings?.default_model || "";
}

export function defaultProviderId() {
  return state.settings?.default_provider || state.settings?.providers?.[0]?.id || "";
}

export function renderProviderModelRow(prefix = WORKFLOW_PREFIX, options = {}) {
  const providerId = `${prefix}-provider`;
  const modelId = `${prefix}-model`;
  const refreshId = `${prefix}-refresh-models`;
  const providerOptions = renderProviderOptions(defaultProviderId());
  const modelDefault = defaultModelOption();
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

export function setupProviderModel(prefix = WORKFLOW_PREFIX) {
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

export function getProviderModelValues(prefix = WORKFLOW_PREFIX) {
  const providerSelect = document.querySelector(`#${prefix}-provider`);
  const modelSelect = document.querySelector(`#${prefix}-model`);
  return {
    provider_id: providerSelect?.value || "",
    model: modelSelect?.value || "",
  };
}

export function populateModelSelect(providerID, selector, force = false) {
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
        .map((model) => `<option value="${escapeHtml(model.id)}">${escapeHtml(model.name || model.id)}</option>`)
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
    .map((model) => `<option value="${escapeHtml(model.id)}">${escapeHtml(model.name || model.id)}</option>`)
    .join("");
}
