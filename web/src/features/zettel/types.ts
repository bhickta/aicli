export type ZettelMode = "inbox" | "metadata" | "settings";

export type ZettelFolderField = "rootFolder" | "inboxFolder" | "metadataFolder" | "dataFolder";

export interface ZettelConfig {
  vaultPath: string;
  rootFolder: string;
  inboxFolder: string;
  inboxLimit: number;
  inboxWorkers: number;
  inboxRandom: boolean;
  metadataFolder: string;
  metadataLimit: number;
  metadataWorkers: number;
  metadataOverwrite: boolean;
  dataFolder: string;
  shorthandPromptPath: string;
  mergeProviderId: string;
  embeddingProviderId: string;
  mergeModel: string;
  embeddingModel: string;
  embeddingBatchSize: number;
  embeddingWorkers: number;
  candidateLimit: number;
}

export type ZettelProviderSettingsPatch = Partial<Pick<
  ZettelConfig,
  "mergeProviderId" |
  "embeddingProviderId" |
  "mergeModel" |
  "embeddingModel"
>>;

export interface SelectOption<T extends string | number = string | number> {
  value: T;
  label: string;
}
