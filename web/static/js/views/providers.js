import { api } from "../core/api.js";
import { view } from "../core/elements.js";
import { state } from "../core/state.js";
import { escapeHtml } from "../core/utils.js";
import { getProviders } from "../providers/controls.js";

export function renderProviders() {
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
