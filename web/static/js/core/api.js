import { health } from "./elements.js";
import { state } from "./state.js";

export function api(path, options = {}) {
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

export async function loadSettings() {
  state.settings = await api("/api/settings");
}

export async function checkHealth() {
  try {
    const result = await api("/api/health");
    health.textContent = result.status;
  } catch {
    health.textContent = "offline";
  }
}
