import { checkHealth } from "./api.js";
import { primaryTabs } from "./elements.js";
import { state } from "./state.js";
import { enableTabKeyboard, syncTabStates } from "./utils.js";
import { render } from "../views/index.js";

export function init() {
  initNav();
  checkHealth();
  render();
}

function initNav() {
  if (!primaryTabs) return;
  primaryTabs.addEventListener("click", handlePrimaryTab);
  enableTabKeyboard(primaryTabs, (tab) => {
    state.view = tab.dataset.view;
    syncPrimaryTabs();
    render();
  });
  syncPrimaryTabs();
}

function handlePrimaryTab(event) {
  const button = event.target.closest('[role="tab"]');
  if (!button || !button.dataset.view) return;
  event.preventDefault();
  state.view = button.dataset.view;
  syncPrimaryTabs();
  render();
}

function syncPrimaryTabs() {
  if (!primaryTabs) return;
  syncTabStates(primaryTabs, (tab) => tab.dataset.view === state.view);
}
