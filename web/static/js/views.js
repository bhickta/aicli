import { loadSettings } from "./api.js";
import { state } from "./state.js";
import { renderChat } from "./views/chat.js";
import { renderJobs } from "./views/jobs.js";
import { renderProviders } from "./views/providers.js";
import { renderSettings } from "./views/settings.js";
import { renderTools } from "./views/tools.js";
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
