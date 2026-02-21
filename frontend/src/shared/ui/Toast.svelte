<script lang="ts" module>
  import { getContext, setContext } from "svelte";
  import { generateUUID } from "$shared/utils/uuid";
  import { toast as toastStore } from "$shared/stores/toast.svelte";

  interface Toast {
    id: string;
    type: "info" | "success" | "warning" | "error";
    message: string;
    duration: number;
  }

  interface ToastContext {
    toasts: Toast[];
    show: (message: string, type?: Toast["type"], duration?: number) => void;
    info: (message: string, duration?: number) => void;
    success: (message: string, duration?: number) => void;
    warning: (message: string, duration?: number) => void;
    error: (message: string, duration?: number) => void;
    remove: (id: string) => void;
  }

  const TOAST_CONTEXT = Symbol("toast");

  // 全局 toast store，用于在组件外部调用
  let globalToastContext: ToastContext | null = null;

  export function createToastContext(): ToastContext {
    let toasts = $state<Toast[]>([]);

    function show(message: string, type: Toast["type"] = "info", duration = 3000) {
      const id = generateUUID();
      toasts = [...toasts, { id, type, message, duration }];

      if (duration > 0) {
        setTimeout(() => remove(id), duration);
      }
    }

    function remove(id: string) {
      toasts = toasts.filter((t) => t.id !== id);
    }

    const context: ToastContext = {
      get toasts() {
        return toasts;
      },
      show,
      info: (message, duration) => show(message, "info", duration),
      success: (message, duration) => show(message, "success", duration),
      warning: (message, duration) => show(message, "warning", duration),
      error: (message, duration) => show(message, "error", duration),
      remove,
    };

    setContext(TOAST_CONTEXT, context);
    // 保存到全局变量
    globalToastContext = context;
    // 处理在 context 初始化前调用的 toast
    processPendingToasts();
    return context;
  }

  export function useToast(): ToastContext {
    const context = getContext<ToastContext>(TOAST_CONTEXT);
    if (!context) {
      throw new Error("useToast must be used within a ToastProvider");
    }
    return context;
  }

  // 待处理的 toast 队列（在 context 初始化前调用的）
  let pendingToasts: Array<{
    message: string;
    type: "success" | "error" | "warning" | "info";
    duration: number;
  }> = [];

  // 全局 showToast 函数，可以在任何地方调用
  // 委托到全局 ToastStore（系统 B），确保即使 <Toast /> 未挂载也能显示
  export function showToast(
    message: string,
    type: "success" | "error" | "warning" | "info" = "info",
    duration = 3000,
  ) {
    if (globalToastContext) {
      globalToastContext.show(message, type, duration);
    } else {
      // 回退到全局 ToastStore（由 Desktop 中的 ToastContainer 渲染）
      toastStore.add({ type, title: message, duration });
    }
  }

  // 处理待处理的 toast（由 createToastContext 调用）
  function processPendingToasts() {
    if (globalToastContext && pendingToasts.length > 0) {
      for (const toast of pendingToasts) {
        globalToastContext.show(toast.message, toast.type, toast.duration);
      }
      pendingToasts = [];
    }
  }
</script>

<script lang="ts">
  import Icon from "@iconify/svelte";
  import type { Snippet } from "svelte";
  import { t } from "$lib/i18n";

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  const context = createToastContext();

  const icons = {
    info: "mdi:information",
    success: "mdi:check-circle",
    warning: "mdi:alert",
    error: "mdi:alert-circle",
  };
</script>

{#if children}
  {@render children()}
{/if}

<div class="toast-container">
  {#each context.toasts as toast (toast.id)}
    <div class="toast toast-{toast.type}">
      <span class="toast-icon">
        <Icon icon={icons[toast.type]} />
      </span>
      <span class="toast-message">{toast.message}</span>
      <button class="toast-close" onclick={() => context.remove(toast.id)} aria-label={$t('common.close')}>
        <Icon icon="mdi:close" />
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    top: 20px;
    right: 20px;
    z-index: 10001;
    display: flex;
    flex-direction: column;
    gap: 10px;
    pointer-events: none;
  }

  .toast {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 12px 16px;
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.15);
    animation: slideIn 0.3s ease-out;
    pointer-events: auto;
    max-width: 360px;
  }

  .toast-info {
    background: #e7f3ff;
    border: 1px solid #91caff;
    color: #0958d9;
  }

  .toast-success {
    background: #f6ffed;
    border: 1px solid #b7eb8f;
    color: #389e0d;
  }

  .toast-warning {
    background: #fffbe6;
    border: 1px solid #ffe58f;
    color: #d48806;
  }

  .toast-error {
    background: #fff2f0;
    border: 1px solid #ffccc7;
    color: #cf1322;
  }

  .toast-icon {
    display: flex;
    font-size: 20px;
    flex-shrink: 0;
  }

  .toast-message {
    flex: 1;
    font-size: 14px;
    line-height: 1.4;
  }

  .toast-close {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 20px;
    height: 20px;
    background: none;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    opacity: 0.6;
    flex-shrink: 0;

    &:hover {
      opacity: 1;
    }

    :global(svg) {
      font-size: 16px;
    }
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateX(20px);
    }
    to {
      opacity: 1;
      transform: translateX(0);
    }
  }
</style>
