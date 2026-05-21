import { computed, shallowRef } from "vue";
import { describeJobProgress, progressBarWidth } from "../lib/jobProgress";
import { runZettelJob } from "../features/zettel/jobs";
import type { Job, ProgressMode } from "../types";

export function useZettelRunner() {
  const status = shallowRef("Ready");
  const result = shallowRef("");
  const progress = shallowRef(0);
  const progressMode = shallowRef<ProgressMode>("determinate");
  const progressVisible = shallowRef(false);
  const busy = shallowRef(false);

  const rawResultSummary = computed(() => result.value ? "Raw result" : "");
  const progressClass = computed(() => ({
    hidden: !progressVisible.value,
    indeterminate: progressMode.value === "indeterminate",
  }));
  const progressStyle = computed(() => ({
    width: progressBarWidth(progressMode.value, progress.value),
  }));

  async function runWorkflow(label: string, endpoint: string, payload: Record<string, unknown>, onDone: (output: unknown) => void) {
    await run(label, async () => {
      onDone(await runZettelJob(endpoint, payload, renderJob));
    });
  }

  async function run(label: string, task: () => Promise<void>) {
    busy.value = true;
    status.value = `${label}...`;
    result.value = "";
    progress.value = 0;
    progressMode.value = "indeterminate";
    progressVisible.value = true;
    try {
      await task();
    } catch (error) {
      status.value = `${label} failed`;
      result.value = error instanceof Error ? error.message : "operation failed";
      if (progress.value <= 0) progressVisible.value = false;
    } finally {
      busy.value = false;
    }
  }

  function renderJob(job: Job) {
    const presentation = describeJobProgress(job);
    status.value = presentation.text;
    progress.value = presentation.percent;
    progressMode.value = presentation.mode;
    progressVisible.value = presentation.visible;
  }

  return {
    status,
    result,
    busy,
    rawResultSummary,
    progressClass,
    progressStyle,
    run,
    runWorkflow,
  };
}
