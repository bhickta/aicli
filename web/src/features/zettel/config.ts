import type { SelectOption, ZettelConfig, ZettelMode } from "./types";

export const candidateLimitOptions = [6, 12, 20, 30, 50];
export const thresholdOptions: SelectOption<number>[] = [
  { value: 0.75, label: "Broad" },
  { value: 0.85, label: "Balanced" },
  { value: 0.9, label: "Strict" },
  { value: 0.98, label: "Lossless" },
];
export const validationThresholdOptions: SelectOption<number>[] = [
  { value: 0.9, label: "Strict" },
  { value: 0.98, label: "Lossless" },
  { value: 1, label: "Exact" },
];
export const promptOptions: SelectOption<string>[] = [
  { value: "example_prompts.md", label: "Extreme shorthand" },
  { value: "builtin", label: "Built-in fallback" },
];

export function createZettelConfig(): ZettelConfig {
  const legacyProviderId = localStorage.getItem("aicli.zettel.providerId") || "lms";
  const legacyJudgeModel = localStorage.getItem("aicli.zettel.judgeModel") || "deepseek-reasoner";
  return {
    vaultPath: localStorage.getItem("aicli.zettel.vaultPath") || "",
    activePath: localStorage.getItem("aicli.zettel.activePath") || "",
    rootFolder: localStorage.getItem("aicli.zettel.rootFolder") || "zettelkasten",
    inboxFolder: localStorage.getItem("aicli.zettel.inboxFolder") || "inbox-to-merge",
    inboxLimit: Number(localStorage.getItem("aicli.zettel.inboxLimit") || 0),
    dataFolder: localStorage.getItem("aicli.zettel.dataFolder") || ".aicli-zettel-merge",
    shorthandPromptPath: localStorage.getItem("aicli.zettel.shorthandPromptPath") || "example_prompts.md",
    providerId: legacyProviderId,
    candidateProviderId: localStorage.getItem("aicli.zettel.candidateProviderId") || legacyProviderId,
    mergeProviderId: localStorage.getItem("aicli.zettel.mergeProviderId") || legacyProviderId,
    validationProviderId: localStorage.getItem("aicli.zettel.validationProviderId") || legacyProviderId,
    embeddingProviderId: localStorage.getItem("aicli.zettel.embeddingProviderId") || "lms",
    judgeModel: legacyJudgeModel,
    candidateModel: localStorage.getItem("aicli.zettel.candidateModel") || legacyJudgeModel,
    mergeModel: localStorage.getItem("aicli.zettel.mergeModel") || "deepseek-reasoner",
    validationModel: localStorage.getItem("aicli.zettel.validationModel") || legacyJudgeModel,
    embeddingModel: localStorage.getItem("aicli.zettel.embeddingModel") || "text-embedding-nomic-embed-text-v1.5",
    candidateLimit: Number(localStorage.getItem("aicli.zettel.candidateLimit") || 12),
    reviewThreshold: Number(localStorage.getItem("aicli.zettel.reviewThreshold") || 0.85),
    validationThreshold: Number(localStorage.getItem("aicli.zettel.validationThreshold") || 0.98),
  };
}

export function readZettelMode(): ZettelMode {
  const storedMode = localStorage.getItem("aicli.zettel.mode");
  return storedMode === "manual" || storedMode === "settings" ? storedMode : "inbox";
}

export function persistZettelConfig(config: ZettelConfig) {
  localStorage.setItem("aicli.zettel.vaultPath", config.vaultPath);
  localStorage.setItem("aicli.zettel.activePath", config.activePath);
  localStorage.setItem("aicli.zettel.rootFolder", config.rootFolder);
  localStorage.setItem("aicli.zettel.inboxFolder", config.inboxFolder);
  localStorage.setItem("aicli.zettel.inboxLimit", String(config.inboxLimit));
  localStorage.setItem("aicli.zettel.dataFolder", config.dataFolder);
  localStorage.setItem("aicli.zettel.shorthandPromptPath", config.shorthandPromptPath);
  localStorage.setItem("aicli.zettel.providerId", config.providerId);
  localStorage.setItem("aicli.zettel.candidateProviderId", config.candidateProviderId);
  localStorage.setItem("aicli.zettel.mergeProviderId", config.mergeProviderId);
  localStorage.setItem("aicli.zettel.validationProviderId", config.validationProviderId);
  localStorage.setItem("aicli.zettel.embeddingProviderId", config.embeddingProviderId);
  localStorage.setItem("aicli.zettel.judgeModel", config.judgeModel);
  localStorage.setItem("aicli.zettel.candidateModel", config.candidateModel);
  localStorage.setItem("aicli.zettel.mergeModel", config.mergeModel);
  localStorage.setItem("aicli.zettel.validationModel", config.validationModel);
  localStorage.setItem("aicli.zettel.embeddingModel", config.embeddingModel);
  localStorage.setItem("aicli.zettel.candidateLimit", String(config.candidateLimit));
  localStorage.setItem("aicli.zettel.reviewThreshold", String(config.reviewThreshold));
  localStorage.setItem("aicli.zettel.validationThreshold", String(config.validationThreshold));
}

export function persistZettelMode(mode: ZettelMode) {
  localStorage.setItem("aicli.zettel.mode", mode);
}
