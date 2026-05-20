import { shallowRef } from "vue";
import { api, parseJobOutput, pollJob } from "../lib/api";
import { stringify } from "../lib/format";
import { describeJobProgress } from "../lib/jobProgress";
import type { Job, ProgressMode, WorkflowDefinition } from "../types";

export function useWorkflowRunner() {
  const status = shallowRef("Ready");
  const result = shallowRef("");
  const markdownPreview = shallowRef("");
  const sourcePreview = shallowRef("");
  const progress = shallowRef(0);
  const progressMode = shallowRef<ProgressMode>("determinate");
  const progressVisible = shallowRef(false);
  const running = shallowRef(false);
  const currentJob = shallowRef<Job | null>(null);

  async function runWorkflow(workflow: WorkflowDefinition | undefined, inputValues: Record<string, unknown>) {
    if (!workflow) return;
    running.value = true;
    status.value = "Running workflow...";
    progress.value = 0;
    progressMode.value = "indeterminate";
    progressVisible.value = true;
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
      progressMode.value = "determinate";
      progressVisible.value = true;
    } catch (error) {
      status.value = "Failed";
      progressVisible.value = false;
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
    const presentation = describeJobProgress(job);
    status.value = presentation.text;
    progress.value = presentation.percent;
    progressMode.value = presentation.mode;
    progressVisible.value = presentation.visible;
    if (job.status === "completed") {
      const parsed = parseJobOutput(job.output);
      const maybeMarkdown = parsed && typeof parsed === "object" && "markdown" in parsed ? String((parsed as { markdown?: unknown }).markdown || "") : "";
      markdownPreview.value = maybeMarkdown;
      result.value = parsed ? stringify(parsed) : "";
      status.value = "Completed";
      progress.value = 100;
      progressMode.value = "determinate";
      progressVisible.value = true;
    }
    if (job.status === "failed") {
      result.value = job.error || "Workflow failed";
      status.value = "Failed";
      progressVisible.value = presentation.visible;
    }
    if (job.status === "cancelled") {
      result.value = job.error || "Workflow cancelled";
      status.value = "Cancelled";
      progressVisible.value = presentation.visible;
    }
  }

  return {
    status,
    result,
    markdownPreview,
    sourcePreview,
    progress,
    progressMode,
    progressVisible,
    running,
    currentJob,
    runWorkflow,
    cancelWorkflow,
    renderWorkflowJob,
  };
}
