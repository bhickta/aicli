import { shallowRef } from "vue";
import { api, parseJobOutput, pollJob } from "../lib/api";
import { elapsedSeconds, formatDuration, stringify } from "../lib/format";
import type { Job, WorkflowDefinition } from "../types";

export function useWorkflowRunner() {
  const status = shallowRef("Ready");
  const result = shallowRef("");
  const markdownPreview = shallowRef("");
  const sourcePreview = shallowRef("");
  const progress = shallowRef(0);
  const running = shallowRef(false);
  const currentJob = shallowRef<Job | null>(null);

  async function runWorkflow(workflow: WorkflowDefinition | undefined, inputValues: Record<string, unknown>) {
    if (!workflow) return;
    running.value = true;
    status.value = "Running workflow...";
    progress.value = 0;
    result.value = "";
    markdownPreview.value = "";
    try {
      const response = await api<{ job?: Job; result?: unknown }>(workflow.endpoint, {
        method: "POST",
        body: JSON.stringify(workflow.buildPayload(inputValues)),
      });
      if (response.job?.id && response.job.status === "running") {
        currentJob.value = response.job;
        await pollJob(response.job.id, renderWorkflowJob);
        return;
      }
      result.value = stringify(response.result || response);
      status.value = "Completed";
      progress.value = 100;
    } catch (error) {
      status.value = "Failed";
      result.value = error instanceof Error ? error.message : "Workflow failed";
    } finally {
      running.value = false;
      currentJob.value = null;
    }
  }

  async function cancelWorkflow() {
    if (!currentJob.value?.id) return;
    status.value = "Cancelling workflow...";
    try {
      const job = await api<Job>(`/api/jobs/${encodeURIComponent(currentJob.value.id)}/cancel`, {
        method: "POST",
      });
      renderWorkflowJob(job);
    } catch (error) {
      result.value = error instanceof Error ? error.message : "Cancel failed";
    }
  }

  function renderWorkflowJob(job: Job) {
    currentJob.value = job;
    const percent = Math.round((job.progress || 0) * 100);
    const eta = job.eta_seconds ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
    const step = job.total_steps ? ` | ${job.current_step}/${job.total_steps}` : "";
    status.value = `${job.status}: ${job.stage || "working"}${step} | ${percent}% | elapsed ${formatDuration(elapsedSeconds(job.created_at))}${eta}`;
    progress.value = percent;
    if (job.status === "completed") {
      const parsed = parseJobOutput(job.output);
      const maybeMarkdown = parsed && typeof parsed === "object" && "markdown" in parsed ? String((parsed as { markdown?: unknown }).markdown || "") : "";
      markdownPreview.value = maybeMarkdown;
      result.value = parsed ? stringify(parsed) : "";
      status.value = "Completed";
    }
    if (job.status === "failed") {
      result.value = job.error || "Workflow failed";
      status.value = "Failed";
    }
    if (job.status === "cancelled") {
      result.value = job.error || "Workflow cancelled";
      status.value = "Cancelled";
    }
  }

  return {
    status,
    result,
    markdownPreview,
    sourcePreview,
    progress,
    running,
    currentJob,
    runWorkflow,
    cancelWorkflow,
    renderWorkflowJob,
  };
}
