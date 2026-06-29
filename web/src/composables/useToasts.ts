import { readonly, shallowRef } from "vue";

export type ToastKind = "success" | "error" | "info";

export interface ToastMessage {
  id: number;
  kind: ToastKind;
  title: string;
  message?: string;
}

const toasts = shallowRef<ToastMessage[]>([]);
let nextToastID = 1;

export function useToasts() {
  function showToast(kind: ToastKind, title: string, message = "") {
    const toast = { id: nextToastID++, kind, title, message };
    toasts.value = [...toasts.value, toast];
    window.setTimeout(() => dismissToast(toast.id), kind === "error" ? 7000 : 4200);
  }

  function dismissToast(id: number) {
    toasts.value = toasts.value.filter((toast) => toast.id !== id);
  }

  return {
    toasts: readonly(toasts),
    success: (title: string, message = "") => showToast("success", title, message),
    error: (title: string, message = "") => showToast("error", title, message),
    info: (title: string, message = "") => showToast("info", title, message),
    dismissToast,
  };
}
