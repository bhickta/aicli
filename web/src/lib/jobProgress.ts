import type { Job, ProgressMode } from "../types";
import { elapsedSeconds, formatDuration } from "./format";

export interface JobProgressPresentation {
  mode: ProgressMode;
  percent: number;
  visible: boolean;
  text: string;
}

const progressModes = new Set<ProgressMode>(["determinate", "indeterminate", "timed"]);

export function describeJobProgress(job: Job): JobProgressPresentation {
  const mode = normalizeProgressMode(job);
  const percent = job.status === "completed" ? 100 : clampPercent((job.progress || 0) * 100);
  const stage = job.stage || "working";
  const elapsed = `elapsed ${formatDuration(elapsedSeconds(job.created_at))}`;
  const label = capitalize(job.status || "running");

  if (job.status === "completed") {
    return { mode: "determinate", percent, visible: true, text: `Completed: ${stage} | 100% | ${elapsed}` };
  }
  if (job.status === "failed" || job.status === "cancelled") {
    return {
      mode,
      percent,
      visible: percent > 0,
      text: `${label}: ${stage} | ${elapsed}`,
    };
  }
  if (mode === "indeterminate") {
    return {
      mode,
      percent: 0,
      visible: true,
      text: `${label}: ${stage} | working | ${elapsed}`,
    };
  }
  if (mode === "timed") {
    const remaining = remainingSeconds(job);
    const eta = remaining > 0 ? ` | remaining ${formatDuration(remaining)}` : "";
    return {
      mode,
      percent,
      visible: true,
      text: `${label}: ${stage} | ${percent}% | ${elapsed}${eta}`,
    };
  }

  const units = formatUnits(job);
  const eta = job.eta_seconds > 0 ? ` | ETA ${formatDuration(job.eta_seconds)}` : "";
  return {
    mode,
    percent,
    visible: true,
    text: `${label}: ${stage}${units} | ${percent}% | ${elapsed}${eta}`,
  };
}

export function progressBarWidth(mode: ProgressMode, percent: number): string {
  if (mode === "indeterminate") return "34%";
  return `${clampPercent(percent)}%`;
}

function normalizeProgressMode(job: Job): ProgressMode {
  const mode = job.progress_mode;
  if (mode && progressModes.has(mode)) return mode;
  if ((job.total_units || 0) > 0 || (job.total_steps || 0) > 0) return "determinate";
  if (job.status === "running") return "indeterminate";
  return "determinate";
}

function formatUnits(job: Job): string {
  const total = job.total_units || job.total_steps || 0;
  if (total <= 0) return "";
  const completed = job.total_units ? job.completed_units || 0 : job.current_step || 0;
  if (job.unit_label === "video second") {
    return ` | ${formatDuration(completed)}/${formatDuration(total)} video processed`;
  }
  const label = pluralize(job.unit_label || (job.total_units ? "unit" : "step"), total);
  return ` | ${Math.max(0, completed)}/${total} ${label}`;
}

function remainingSeconds(job: Job): number {
  if (job.eta_seconds > 0) return job.eta_seconds;
  if (!job.progress_ends_at) return 0;
  const endsAt = Date.parse(job.progress_ends_at);
  if (!Number.isFinite(endsAt)) return 0;
  return Math.max(0, Math.round((endsAt - Date.now()) / 1000));
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0;
  return Math.max(0, Math.min(100, Math.round(value)));
}

function pluralize(label: string, total: number): string {
  if (total === 1 || label.endsWith("s")) return label;
  return `${label}s`;
}

function capitalize(value: string): string {
  if (!value) return "";
  return `${value.slice(0, 1).toUpperCase()}${value.slice(1)}`;
}
