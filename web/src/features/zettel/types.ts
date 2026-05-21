export type ZettelMode = "inbox" | "manual" | "settings";

export interface ZettelLineRange {
  start_line: number;
  end_line: number;
  reason?: string;
}

export interface ZettelCandidate {
  path: string;
  similarity: number;
  confidence: number;
  relationship: string;
  risk: string;
  reason: string;
  source_line_ranges: ZettelLineRange[];
  extracted_markdown: string;
}

export interface ZettelProposal {
  id: string;
  active_markdown?: string;
  final_markdown: string;
  merge_plan?: { insertions?: Array<{ after_line: number; markdown: string; reason?: string }> };
  coverage?: { score?: number };
  judge?: { verdict?: string; score?: number; notes?: string };
}

export type ZettelFolderField = "rootFolder" | "inboxFolder" | "dataFolder";

export interface ZettelConfig {
  vaultPath: string;
  activePath: string;
  rootFolder: string;
  inboxFolder: string;
  inboxLimit: number;
  dataFolder: string;
  shorthandPromptPath: string;
  providerId: string;
  candidateProviderId: string;
  mergeProviderId: string;
  validationProviderId: string;
  embeddingProviderId: string;
  judgeModel: string;
  candidateModel: string;
  mergeModel: string;
  validationModel: string;
  embeddingModel: string;
  candidateLimit: number;
  reviewThreshold: number;
  validationThreshold: number;
}

export type ZettelProviderSettingsPatch = Partial<Pick<
  ZettelConfig,
  "candidateProviderId" |
  "mergeProviderId" |
  "validationProviderId" |
  "embeddingProviderId" |
  "candidateModel" |
  "mergeModel" |
  "validationModel" |
  "embeddingModel"
>>;

export interface SelectOption<T extends string | number = string | number> {
  value: T;
  label: string;
}
