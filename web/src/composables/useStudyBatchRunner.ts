import { shallowRef } from "vue";
import { api, pollJob, sleep } from "../lib/api";
import type { Job, StudyBatchItemRecord, StudyBatchRecord, StudyBatchResponse } from "../types";
import { useToasts } from "./useToasts";

export function useStudyBatchRunner(onFinished: () => Promise<void>) {
  const toasts = useToasts();
  const batchParallelism = shallowRef(2);
  const forceRerun = shallowRef(false);
  const running = shallowRef(false);
  const runStatus = shallowRef("");
  const activeBatch = shallowRef<StudyBatchRecord | null>(null);
  const batchItems = shallowRef<StudyBatchItemRecord[]>([]);

  async function startBatch(copyIds: string[], stage = "all") {
    if (!copyIds.length) return;
    running.value = true;
    runStatus.value = `Starting ${copyIds.length} copy analysis...`;
    try {
      const payload = await api<StudyBatchResponse>("/api/study/batches", {
        method: "POST",
        body: JSON.stringify({
          copy_ids: copyIds,
          stage,
          parallelism: batchParallelism.value,
          force_ocr: forceRerun.value,
        }),
      });
      await trackBatch(payload, `${payload.batch.total} copy(s), ${batchParallelism.value} parallel`);
    } catch (error) {
      handleRunError(error);
    } finally {
      running.value = false;
    }
  }

  async function runCopy(copyId: string) {
    running.value = true;
    runStatus.value = "Starting PDF analysis...";
    try {
      const payload = await api<StudyBatchResponse>(`/api/study/copies/${encodeURIComponent(copyId)}/run`, {
        method: "POST",
        body: JSON.stringify({ force_ocr: forceRerun.value }),
      });
      await trackBatch(payload, "One copy analysis started");
    } catch (error) {
      handleRunError(error);
    } finally {
      running.value = false;
    }
  }

  async function trackBatch(payload: StudyBatchResponse, message: string) {
    activeBatch.value = payload.batch;
    batchItems.value = payload.items || [];
    toasts.info("Analysis started", message);
    if (!payload.job?.id) return;
    runStatus.value = "Analysis running...";
    await Promise.all([pollBatch(payload.batch.id), pollJob(payload.job.id, updateJobStatus)]);
    await pollBatch(payload.batch.id);
    runStatus.value = activeBatch.value?.status === "completed" ? "Analysis completed" : `Analysis ${activeBatch.value?.status}`;
    await onFinished();
  }

  async function pollBatch(batchId: string) {
    for (;;) {
      const payload = await api<{ batch: StudyBatchRecord; items: StudyBatchItemRecord[] }>(
        `/api/study/batches/${encodeURIComponent(batchId)}`,
      );
      activeBatch.value = payload.batch;
      batchItems.value = payload.items || [];
      if (["completed", "failed", "partial_failed", "cancelled"].includes(payload.batch.status)) return;
      await sleep(900);
    }
  }

  function updateJobStatus(job: Job) {
    const total = job.total_units || activeBatch.value?.total || 0;
    runStatus.value = `${job.stage} (${job.completed_units || 0}/${total})`;
  }

  function handleRunError(error: unknown) {
    const message = error instanceof Error ? error.message : "Analysis failed to start";
    runStatus.value = message;
    toasts.error("Analysis failed", message);
  }

  return { batchParallelism, forceRerun, running, runStatus, activeBatch, batchItems, startBatch, runCopy };
}
