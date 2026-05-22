import { readNumberValue } from "../../lib/format";
import type { ZettelConfig } from "./types";

export function buildZettelPayload(config: ZettelConfig) {
  return {
    vault_path: config.vaultPath,
    root_folder: config.rootFolder,
    inbox_folder: config.inboxFolder,
    inbox_limit: readNumberValue(config.inboxLimit, 0, 0),
    adopt_unmatched_inbox: config.adoptUnmatchedInbox,
    data_folder: config.dataFolder,
    shorthand_prompt_path: config.shorthandPromptPath,
    provider_id: config.mergeProviderId,
    merge_provider_id: config.mergeProviderId,
    embedding_provider_id: config.embeddingProviderId,
    merge_model: config.mergeModel,
    embedding_model: config.embeddingModel,
    embedding_batch_size: readNumberValue(config.embeddingBatchSize, 128, 1),
    embedding_workers: readNumberValue(config.embeddingWorkers, 4, 1),
    candidate_limit: readNumberValue(config.candidateLimit, 12, 1),
  };
}
