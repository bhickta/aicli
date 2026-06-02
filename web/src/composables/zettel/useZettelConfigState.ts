import { computed, reactive, shallowRef, watch, type Ref } from "vue";
import {
  candidateLimitOptions,
  createZettelConfig,
  embeddingBatchSizeOptions,
  embeddingWorkerOptions,
  inboxWorkerOptions,
  metadataWorkerOptions,
  persistZettelConfig,
  persistZettelMode,
  promptOptions,
  readZettelMode,
} from "../../features/zettel/config";
import type {
  ZettelConfig,
  ZettelMode,
  ZettelProviderSettingsPatch,
} from "../../features/zettel/types";

export function useZettelConfigState(busy: Ref<boolean>, onConfigChanged: () => void) {
  const config = reactive<ZettelConfig>(createZettelConfig());
  const mode = shallowRef<ZettelMode>(readZettelMode());

  const hasVault = computed(() => Boolean(config.vaultPath.trim()));
  const canRunInboxMerge = computed(() => hasVault.value && !busy.value);
  const canRunMetadata = computed(() => hasVault.value && !busy.value);
  const canExportTraining = computed(() => hasVault.value && !busy.value);
  const canUseVaultFolders = computed(() => hasVault.value && !busy.value);

  watch(config, () => {
    persistZettelConfig(config);
    onConfigChanged();
  });

  watch(mode, () => {
    persistZettelMode(mode.value);
  });

  function updateProviderSettings(value: ZettelProviderSettingsPatch) {
    Object.assign(config, value);
  }

  function updateConfig(field: keyof ZettelConfig, value: ZettelConfig[keyof ZettelConfig]) {
    Object.assign(config, { [field]: value });
  }

  return {
    config,
    mode,
    candidateLimitOptions,
    promptOptions,
    embeddingBatchSizeOptions,
    embeddingWorkerOptions,
    inboxWorkerOptions,
    metadataWorkerOptions,
    canRunInboxMerge,
    canRunMetadata,
    canExportTraining,
    canUseVaultFolders,
    updateProviderSettings,
    updateConfig,
  };
}
