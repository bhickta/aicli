import type { Job, Settings } from "../types";

export async function api<T>(path: string, options: RequestInit = {}): Promise<T> {
  const response = await fetch(path, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options,
  });
  if (!response.ok) {
    const payload = await response.json().catch(() => ({ error: response.statusText }));
    throw new Error(payload.error || response.statusText);
  }
  return response.json() as Promise<T>;
}

export function loadSettings(): Promise<Settings> {
  return api<Settings>("/api/settings");
}

export function parseJobOutput(output: string): unknown {
  if (!output) return null;
  try {
    return JSON.parse(output);
  } catch {
    return { output };
  }
}

export async function pollJob(jobID: string, onJob: (job: Job) => void): Promise<Job> {
  for (;;) {
    await sleep(900);
    const job = await api<Job>(`/api/jobs/${encodeURIComponent(jobID)}`);
    onJob(job);
    if (job.status === "completed" || job.status === "failed" || job.status === "cancelled") return job;
  }
}

export function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
