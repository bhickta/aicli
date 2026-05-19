import { loadSettings } from "../core/api.js";
import { state } from "../core/state.js";
import { renderWorkflows } from "../workflows/view.js";
import { renderChat } from "./chat.js";
import { renderJobs } from "./jobs.js";
import { renderProviders } from "./providers.js";
import { renderSettings } from "./settings.js";
import { renderTools } from "./tools.js";

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
