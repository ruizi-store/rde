<script lang="ts" module>
  import { getContext, setContext } from "svelte";

  interface ConfirmOptions {
    title?: string;
    message: string;
    confirmText?: string;
    cancelText?: string;
    type?: "info" | "warning" | "danger";
  }

  interface ConfirmContext {
    show: (options: ConfirmOptions) => Promise<boolean>;
  }

  const CONFIRM_CONTEXT = Symbol("confirm");

  export function setConfirmContext(context: ConfirmContext) {
    setContext(CONFIRM_CONTEXT, context);
  }

  export function useConfirm(): ConfirmContext {
    const context = getContext<ConfirmContext>(CONFIRM_CONTEXT);
    if (!context) {
      // 降级为原生 confirm
      return {
        show: async (options) => window.confirm(options.message),
      };
    }
    return context;
  }
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  // 状态
  let open = $state(false);
  let options = $state<ConfirmOptions>({
    title: $t('common.confirm'),
    message: "",
    confirmText: $t('common.ok'),
    cancelText: $t('common.cancel'),
    type: "info",
  });
  let resolvePromise: ((value: boolean) => void) | null = null;

  // 显示确认框
  function show(opts: ConfirmOptions): Promise<boolean> {
    options = {
      title: opts.title || $t('common.confirm'),
      message: opts.message,
      confirmText: opts.confirmText || $t('common.ok'),
      cancelText: opts.cancelText || $t('common.cancel'),
      type: opts.type || "info",
    };
    open = true;

    return new Promise((resolve) => {
      resolvePromise = resolve;
    });
  }

  // 确认
  function handleConfirm() {
    open = false;
    resolvePromise?.(true);
    resolvePromise = null;
  }

  // 取消
  function handleCancel() {
    open = false;
    resolvePromise?.(false);
    resolvePromise = null;
  }

  // 键盘事件
  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      handleCancel();
    } else if (e.key === "Enter") {
      handleConfirm();
    }
  }

  // 图标
  const icons = {
    info: "mdi:information",
    warning: "mdi:alert",
    danger: "mdi:alert-circle",
  };

  // 注册 context
  setConfirmContext({ show });

  $effect(() => {
    if (open) {
      document.addEventListener("keydown", handleKeydown);
    }
    return () => {
      document.removeEventListener("keydown", handleKeydown);
    };
  });
</script>

{#if children}
  {@render children()}
{/if}

{#if open}
  <div class="confirm-overlay" onclick={handleCancel} role="dialog" aria-modal="true">
    <div class="confirm-modal {options.type}" onclick={(e) => e.stopPropagation()}>
      <div class="confirm-icon">
        <Icon icon={icons[options.type || "info"]} />
      </div>

      <div class="confirm-content">
        <h3 class="confirm-title">{options.title}</h3>
        <p class="confirm-message">{options.message}</p>
      </div>

      <div class="confirm-actions">
        <button class="btn btn-cancel" onclick={handleCancel}>
          {options.cancelText}
        </button>
        <button class="btn btn-confirm {options.type}" onclick={handleConfirm}>
          {options.confirmText}
        </button>
      </div>
    </div>
  </div>
{/if}

<style>
  .confirm-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10001;
    animation: fadeIn 0.15s ease-out;
  }

  .confirm-modal {
    width: 400px;
    max-width: 90vw;
    background: var(--bg-card, white);
    border-radius: 12px;
    padding: 24px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
    animation: slideIn 0.2s ease-out;
    text-align: center;
  }

  .confirm-icon {
    width: 56px;
    height: 56px;
    margin: 0 auto 16px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;

    :global(svg) {
      width: 32px;
      height: 32px;
    }

    .info & {
      background: #dbeafe;
      color: #2563eb;
    }

    .warning & {
      background: #fef3c7;
      color: #d97706;
    }

    .danger & {
      background: #fee2e2;
      color: #dc2626;
    }
  }

  .confirm-content {
    margin-bottom: 24px;
  }

  .confirm-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary, #1e293b);
    margin: 0 0 8px 0;
  }

  .confirm-message {
    font-size: 14px;
    color: var(--text-secondary, #64748b);
    margin: 0;
    line-height: 1.5;
    white-space: pre-line;
  }

  .confirm-actions {
    display: flex;
    gap: 12px;
    justify-content: center;
  }

  .btn {
    padding: 10px 24px;
    border: none;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: all 0.15s;

    &.btn-cancel {
      background: var(--bg-hover, #f1f5f9);
      color: var(--text-primary, #1e293b);

      &:hover {
        background: var(--border-color, #e2e8f0);
      }
    }

    &.btn-confirm {
      color: white;

      &.info {
        background: #3b82f6;

        &:hover {
          background: #2563eb;
        }
      }

      &.warning {
        background: #f59e0b;

        &:hover {
          background: #d97706;
        }
      }

      &.danger {
        background: #ef4444;

        &:hover {
          background: #dc2626;
        }
      }
    }
  }

  @keyframes fadeIn {
    from {
      opacity: 0;
    }
    to {
      opacity: 1;
    }
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: scale(0.95) translateY(-10px);
    }
    to {
      opacity: 1;
      transform: scale(1) translateY(0);
    }
  }
</style>
