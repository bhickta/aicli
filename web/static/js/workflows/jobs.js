import { api } from "../core/api.js";
import { elapsedSeconds, formatDuration, sleep } from "../core/utils.js";
import { setMarkdownPreview } from "../views/previews.js";

export function setProgress(progressBar, percent) {
  if (!progressBar) return;
  progressBar.classList.remove("hidden");
  progressBar.firstElementChild.style.width = `${Math.max(0, Math.min(100, percent))}%`;
}

export function pollWorkflowJob(jobID, status, progressBar, output) {
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

export function parseJobOutput(output) {
  if (!output) return null;
  try {
    return JSON.parse(output);
  } catch {
    return { output };
  }
}

export function renderWorkflowJob(job, status, progressBar) {
  if (!job) return;
  const percent = Math.round((job.progress || 0) * 100);
  const elapsed = elapsedSeconds(job.created_at);
  const eta = job.eta_seconds ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
  const step = job.total_steps ? ` | ${job.current_step}/${job.total_steps}` : "";
  status.textContent = `${job.status}: ${job.stage || "working"}${step} | ${percent}% | elapsed ${formatDuration(elapsed)}${eta}`;
  setProgress(progressBar, percent);
}
