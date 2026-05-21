import { api, parseJobOutput, pollJob } from "../../lib/api";
import type { Job } from "../../types";

export async function runZettelJob(
  endpoint: string,
  payload: Record<string, unknown>,
  onProgress: (job: Job) => void,
) {
  const response = await api<{ job?: Job }>(endpoint, {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!response.job?.id) throw new Error("workflow did not return a job");
  await pollJob(response.job.id, onProgress);
  const finalJob = await api<Job>(`/api/jobs/${encodeURIComponent(response.job.id)}`);
  if (finalJob.status === "failed") throw new Error(finalJob.error || "workflow failed");
  return parseJobOutput(finalJob.output);
}
