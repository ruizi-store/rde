<script lang="ts" module>
  import { getContext, setContext } from "svelte";
  import type { SudoAction } from "$shared/services/sudo";

  export interface SudoConfirmOptions {
    action: SudoAction;
    args?: string[];
    command: string;
  }

  export interface SudoExecuteResult {
    success: boolean;
    output?: string;
    error?: string;
    exit_code: number;
    duration_ms: number;
  }

  export interface SudoConfirmContext {
    confirm: (options: SudoConfirmOptions) => Promise<boolean>;
    execute: (
      actionId: string,
      args?: string[],
      skipConfirm?: boolean
    ) => Promise<SudoExecuteResult | null>;
  }

  const SUDO_CONTEXT = Symbol("sudo-confirm");

  export function setSudoContext(context: SudoConfirmContext) {
    setContext(SUDO_CONTEXT, context);
  }

  export function useSudo(): SudoConfirmContext {
    const context = getContext<SudoConfirmContext>(SUDO_CONTEXT);
    if (!context) {
      throw new Error("SudoConfirmModal not found in context");
    }
    return context;
  }
</script>

<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";
  import { sudoService } from "$shared/services/sudo";
  import { t } from "$lib/i18n";

  interface Props {
    children?: Snippet;
  }

  let { children }: Props = $props();

  // 状态
  let open = $state(false);
  let loading = $state(false);
  let options = $state<SudoConfirmOptions | null>(null);
  let resolvePromise: ((value: boolean) => void) | null = null;

  // 显示确认框
  function showConfirm(opts: SudoConfirmOptions): Promise<boolean> {
    options = opts;
    open = true;
    loading = false;

    return new Promise((resolve) => {
      resolvePromise = resolve;
    });
  }

  // 执行操作（带确认）
  async function executeWithConfirm(
    actionId: string,
    args?: string[],
    skipConfirm: boolean = false
  ): Promise<SudoExecuteResult | null> {
    try {
      // 先预览
      const preview = await sudoService.preview(actionId, args);

      // 非危险操作或跳过确认直接执行
      if (!preview.action.dangerous || skipConfirm) {
        return await sudoService.execute(actionId, args, true);
      }

      // 危险操作需要确认
      const confirmed = await showConfirm({
        action: preview.action,
        args,
        command: preview.command,
      });

      if (!confirmed) {
        return null;
      }

      return await sudoService.execute(actionId, args, true);
    } catch (error) {
      console.error("Sudo execution failed:", error);
      throw error;
    }
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
    if (!open) return;
    if (e.key === "Escape") {
      handleCancel();
    }
  }

  // 设置上下文
  setSudoContext({
    confirm: showConfirm,
    execute: executeWithConfirm,
  });
</script>

<svelte:window onkeydown={handleKeydown} />

{#if children}
  {@render children()}
{/if}

{#if open && options}
  <!-- 背景遮罩 -->
  <div
    class="fixed inset-0 bg-black/50 z-[9999] flex items-center justify-center"
    onclick={handleCancel}
    role="dialog"
    aria-modal="true"
  >
    <!-- 弹窗内容 -->
    <div
      class="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden"
      onclick={(e) => e.stopPropagation()}
    >
      <!-- 头部 -->
      <div
        class="bg-amber-500 text-white px-6 py-4 flex items-center gap-3"
      >
        <Icon icon="mdi:shield-alert" class="w-8 h-8" />
        <div>
          <h3 class="text-lg font-bold">{$t('auth.adminRequired')}</h3>
          <p class="text-sm opacity-90">{$t('auth.confirmToExecute')}</p>
        </div>
      </div>

      <!-- 内容 -->
      <div class="p-6 space-y-4">
        <!-- 操作信息 -->
        <div class="space-y-2">
          <div class="flex items-center gap-2 text-gray-700 dark:text-gray-300">
            <Icon icon="mdi:information" class="w-5 h-5" />
            <span class="font-medium">{options.action.name}</span>
          </div>
          <p class="text-sm text-gray-500 dark:text-gray-400 ml-7">
            {options.action.description}
          </p>
        </div>

        <!-- 命令预览 -->
        <div class="bg-gray-900 rounded-lg p-4 font-mono text-sm">
          <div class="flex items-center gap-2 text-gray-400 mb-2">
            <Icon icon="mdi:console" class="w-4 h-4" />
            <span>{$t('auth.commandToRun')}</span>
          </div>
          <code class="text-green-400 break-all">{options.command}</code>
        </div>

        <!-- 危险警告 -->
        {#if options.action.dangerous}
          <div
            class="bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-800 rounded-lg p-4 flex items-start gap-3"
          >
            <Icon
              icon="mdi:alert-circle"
              class="w-5 h-5 text-red-500 flex-shrink-0 mt-0.5"
            />
            <div class="text-sm text-red-700 dark:text-red-300">
              <p class="font-medium">{$t('auth.dangerWarning')}</p>
              <p class="mt-1 opacity-90">
                {$t('auth.dangerDescription')}
              </p>
            </div>
          </div>
        {/if}
      </div>

      <!-- 底部按钮 -->
      <div
        class="px-6 py-4 bg-gray-50 dark:bg-gray-900 flex justify-end gap-3"
      >
        <button
          class="px-4 py-2 text-gray-600 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700 rounded-lg transition-colors"
          onclick={handleCancel}
          disabled={loading}
        >
          {$t('common.cancel')}
        </button>
        <button
          class="px-4 py-2 bg-amber-500 hover:bg-amber-600 text-white rounded-lg transition-colors flex items-center gap-2"
          onclick={handleConfirm}
          disabled={loading}
        >
          {#if loading}
            <Icon icon="mdi:loading" class="w-4 h-4 animate-spin" />
          {:else}
            <Icon icon="mdi:shield-check" class="w-4 h-4" />
          {/if}
          {$t('auth.confirmExecute')}
        </button>
      </div>
    </div>
  </div>
{/if}
