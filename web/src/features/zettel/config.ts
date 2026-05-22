import type { SelectOption, ZettelConfig, ZettelMode } from "./types";

export const candidateLimitOptions = [6, 12, 20, 30, 50];
export const promptOptions: SelectOption<string>[] = [
  { value: "example_prompts.md", label: "Extreme shorthand" },
  { value: "builtin", label: "Built-in fallback" },
];
export const embeddingBatchSizeOptions = [64, 128, 256, 512];
export const embeddingWorkerOptions = [1, 2, 3, 4, 6, 8];

export function createZettelConfig(): ZettelConfig {
  const legacyProviderId = localStorage.getItem("aicli.zettel.providerId") || "lms";
  const storedAdoptUnmatched = localStorage.getItem("aicli.zettel.adoptUnmatchedInbox");
  return {
    vaultPath: localStorage.getItem("aicli.zettel.vaultPath") || "",
    rootFolder: localStorage.getItem("aicli.zettel.rootFolder") || "zettelkasten",
    inboxFolder: localStorage.getItem("aicli.zettel.inboxFolder") || "inbox-to-merge",
    inboxLimit: Number(localStorage.getItem("aicli.zettel.inboxLimit") || 0),
    adoptUnmatchedInbox: storedAdoptUnmatched === null ? true : storedAdoptUnmatched === "true",
    dataFolder: localStorage.getItem("aicli.zettel.dataFolder") || ".aicli-zettel-merge",
    shorthandPromptPath: localStorage.getItem("aicli.zettel.shorthandPromptPath") || "example_prompts.md",
    mergeProviderId: localStorage.getItem("aicli.zettel.mergeProviderId") || legacyProviderId,
    embeddingProviderId: localStorage.getItem("aicli.zettel.embeddingProviderId") || "lms",
    mergeModel: localStorage.getItem("aicli.zettel.mergeModel") || "deepseek-reasoner",
    embeddingModel: localStorage.getItem("aicli.zettel.embeddingModel") || "text-embedding-nomic-embed-text-v1.5",
    embeddingBatchSize: Number(localStorage.getItem("aicli.zettel.embeddingBatchSize") || 128),
    embeddingWorkers: Number(localStorage.getItem("aicli.zettel.embeddingWorkers") || 4),
    candidateLimit: Number(localStorage.getItem("aicli.zettel.candidateLimit") || 12),
  };
}

export function readZettelMode(): ZettelMode {
  const storedMode = localStorage.getItem("aicli.zettel.mode");
  return storedMode === "settings" ? storedMode : "inbox";
}

export function persistZettelConfig(config: ZettelConfig) {
  localStorage.setItem("aicli.zettel.vaultPath", config.vaultPath);
  localStorage.setItem("aicli.zettel.rootFolder", config.rootFolder);
  localStorage.setItem("aicli.zettel.inboxFolder", config.inboxFolder);
  localStorage.setItem("aicli.zettel.inboxLimit", String(config.inboxLimit));
  localStorage.setItem("aicli.zettel.adoptUnmatchedInbox", String(config.adoptUnmatchedInbox));
  localStorage.setItem("aicli.zettel.dataFolder", config.dataFolder);
  localStorage.setItem("aicli.zettel.shorthandPromptPath", config.shorthandPromptPath);
  localStorage.setItem("aicli.zettel.providerId", config.mergeProviderId);
  localStorage.setItem("aicli.zettel.mergeProviderId", config.mergeProviderId);
  localStorage.setItem("aicli.zettel.embeddingProviderId", config.embeddingProviderId);
  localStorage.setItem("aicli.zettel.mergeModel", config.mergeModel);
  localStorage.setItem("aicli.zettel.embeddingModel", config.embeddingModel);
  localStorage.setItem("aicli.zettel.embeddingBatchSize", String(config.embeddingBatchSize));
  localStorage.setItem("aicli.zettel.embeddingWorkers", String(config.embeddingWorkers));
  localStorage.setItem("aicli.zettel.candidateLimit", String(config.candidateLimit));
}

export function persistZettelMode(mode: ZettelMode) {
  localStorage.setItem("aicli.zettel.mode", mode);
}
