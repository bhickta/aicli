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
    provider_id: config.candidateProviderId,
    candidate_provider_id: config.candidateProviderId,
    merge_provider_id: config.mergeProviderId,
    validation_provider_id: config.validationProviderId,
    embedding_provider_id: config.embeddingProviderId,
    judge_model: config.candidateModel,
    candidate_model: config.candidateModel,
    merge_model: config.mergeModel,
    validation_model: config.validationModel,
    embedding_model: config.embeddingModel,
    candidate_limit: readNumberValue(config.candidateLimit, 12, 1),
    review_threshold: readNumberValue(config.reviewThreshold, 0.85, 0),
    validation_threshold: readNumberValue(config.validationThreshold, 0.98, 0),
  };
}
