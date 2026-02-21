<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";

  interface Props {
    open?: boolean;
    title?: string;
    size?: "sm" | "md" | "lg" | "xl" | "fullscreen";
    closable?: boolean;
    closeOnOverlay?: boolean;
    closeOnEsc?: boolean;
    showHeader?: boolean;
    showFooter?: boolean;
    onclose?: () => void;
    header?: Snippet;
    footer?: Snippet;
    children: Snippet;
  }

  let {
    open = $bindable(false),
    title = "",
    size = "md",
    closable = true,
    closeOnOverlay = false,
    closeOnEsc = true,
    showHeader = true,
    showFooter = false,
    onclose,
    header,
    footer,
    children,
  }: Props = $props();

  function close() {
    open = false;
    onclose?.();
  }

  let shaking = $state(false);
  let mounted = $state(false);

  function handleOverlayClick(e: MouseEvent) {
    if (e.target !== e.currentTarget) return;
    if (closeOnOverlay) {
      close();
    } else {
      // 模拟原生桌面行为：点击外部时闪烁/抖动弹窗
      shaking = true;
      setTimeout(() => { shaking = false; }, 300);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (closeOnEsc && e.key === "Escape") {
      close();
    }
  }

  $effect(() => {
    if (open) {
      mounted = false;
      document.addEventListener("keydown", handleKeydown);
      document.body.style.overflow = "hidden";
      // slideIn 动画结束后标记为 mounted，防止后续 shake 结束时重播 slideIn
      const timer = setTimeout(() => { mounted = true; }, 220);
      return () => {
        clearTimeout(timer);
        document.removeEventListener("keydown", handleKeydown);
        document.body.style.overflow = "";
      };
    }
  });
</script>

{#if open}
  <div class="modal-overlay" onclick={handleOverlayClick} role="dialog" aria-modal="true">
    <div class="modal-container {size}" class:shaking class:mounted>
      {#if showHeader}
        <div class="modal-header">
          {#if header}
            {@render header()}
          {:else}
            <h3 class="modal-title">{title}</h3>
          {/if}
          {#if closable}
            <button class="close-btn" onclick={close} aria-label={$t('common.close')}>
              <Icon icon="mdi:close" />
            </button>
          {/if}
        </div>
      {/if}

      <div class="modal-body">
        {@render children()}
      </div>

      {#if showFooter && footer}
        <div class="modal-footer">
          {@render footer()}
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .modal-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 10000;
    animation: fadeIn 0.15s ease-out;
    backdrop-filter: blur(2px);
  }

  .modal-container {
    background: var(--bg-card, white);
    border-radius: 12px;
    box-shadow: 0 20px 60px rgba(0, 0, 0, 0.3);
    display: flex;
    flex-direction: column;
    max-height: 90vh;
    animation: slideIn 0.2s ease-out;

    &.mounted {
      animation: none;
    }

    &.shaking {
      animation: shake 0.3s ease-in-out;
    }

    &.sm {
      width: 360px;
    }

    &.md {
      width: 480px;
    }

    &.lg {
      width: 640px;
    }

    &.xl {
      width: 800px;
    }

    &.fullscreen {
      width: calc(100vw - 40px);
      height: calc(100vh - 40px);
      max-height: calc(100vh - 40px);
      border-radius: 8px;
    }
  }

  .modal-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 16px 20px;
    border-bottom: 1px solid var(--border-color, #e0e0e0);
    flex-shrink: 0;
  }

  .modal-title {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: var(--text-primary, #333);
  }

  .close-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    background: none;
    border: none;
    border-radius: 6px;
    color: var(--text-muted, #888);
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, #f0f0f0);
      color: var(--text-primary, #333);
    }

    :global(svg) {
      font-size: 20px;
    }
  }

  .modal-body {
    padding: 20px;
    overflow-y: auto;
    flex: 1;
  }

  .modal-footer {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid var(--border-color, #e0e0e0);
    flex-shrink: 0;
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

  @keyframes shake {
    0%, 100% { transform: scale(1); }
    25% { transform: scale(1.02); }
    50% { transform: scale(0.98); }
    75% { transform: scale(1.01); }
  }
</style>
