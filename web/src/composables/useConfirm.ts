import { readonly, shallowRef } from "vue";

export interface ConfirmRequest {
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  danger?: boolean;
}

interface ActiveConfirm extends ConfirmRequest {
  resolve: (value: boolean) => void;
}

const activeConfirm = shallowRef<ActiveConfirm | null>(null);

export function useConfirm() {
  function confirm(request: ConfirmRequest) {
    return new Promise<boolean>((resolve) => {
      activeConfirm.value = { ...request, resolve };
    });
  }

  function resolveConfirm(value: boolean) {
    activeConfirm.value?.resolve(value);
    activeConfirm.value = null;
  }

  return {
    activeConfirm: readonly(activeConfirm),
    confirm,
    resolveConfirm,
  };
}
