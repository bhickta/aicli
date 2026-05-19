export function escapeHtml(value) {
  const text = String(value ?? "");
  return text
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

export function enableTabKeyboard(container, onActivate) {
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

export function syncTabStates(container, isActive) {
  const tabs = container.querySelectorAll('[role="tab"]');
  tabs.forEach((tab) => {
    const active = isActive(tab);
    tab.setAttribute("aria-selected", active ? "true" : "false");
    tab.tabIndex = active ? 0 : -1;
    tab.classList.toggle("active-tab", active);
  });
}

export function readNumberValue(value, fallback = 0, min = Number.NEGATIVE_INFINITY) {
  const parsed = Number.parseInt(value, 10);
  if (Number.isNaN(parsed)) return fallback;
  return parsed < min ? fallback : parsed;
}

export function elapsedSeconds(createdAt) {
  const started = Date.parse(createdAt);
  if (Number.isNaN(started)) return 0;
  return Math.max(0, Math.round((Date.now() - started) / 1000));
}

export function formatDuration(seconds) {
  if (!seconds) return "0s";
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  if (!mins) return `${secs}s`;
  const hours = Math.floor(mins / 60);
  if (!hours) return `${mins}m ${secs}s`;
  return `${hours}h ${mins % 60}m`;
}

export function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}
