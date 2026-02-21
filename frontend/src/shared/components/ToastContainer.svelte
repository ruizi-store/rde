<script lang="ts">
  import Icon from "@iconify/svelte";
  import { toast, type ToastMessage } from "$shared/stores/toast.svelte";
  import { settings } from "$shared/stores/settings.svelte";

  // 获取图标
  function getIcon(type: ToastMessage["type"]): string {
    switch (type) {
      case "success":
        return "mdi:check-circle";
      case "warning":
        return "mdi:alert-circle";
      case "error":
        return "mdi:close-circle";
      default:
        return "mdi:information";
    }
  }

  // 获取位置类名
  let positionClass = $derived(settings.notifications.position.replace("-", " "));
</script>

<div class="toast-container {positionClass}">
  {#each toast.messages as item (item.id)}
    <div class="toast {item.type}" role="alert">
      <div class="toast-icon">
        <Icon icon={getIcon(item.type)} width="20" />
      </div>
      <div class="toast-content">
        <div class="toast-title">{item.title}</div>
        {#if item.message}
          <div class="toast-message">{item.message}</div>
        {/if}
      </div>
      {#if item.closable}
        <button class="toast-close" onclick={() => toast.remove(item.id)}>
          <Icon icon="mdi:close" width="16" />
        </button>
      {/if}
    </div>
  {/each}
</div>

<style>
  .toast-container {
    position: fixed;
    z-index: 10000;
    display: flex;
    flex-direction: column;
    gap: 8px;
    padding: 16px;
    pointer-events: none;

    /* 位置变体 */
    &.top {
      top: 0;
    }

    &.bottom {
      bottom: 0;
      flex-direction: column-reverse;
    }

    &.left {
      left: 0;
      align-items: flex-start;
    }

    &.right {
      right: 0;
      align-items: flex-end;
    }
  }

  .toast {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 16px;
    min-width: 300px;
    max-width: 400px;
    background: rgba(30, 30, 46, 0.95);
    backdrop-filter: blur(20px);
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
    pointer-events: auto;
    animation: slideIn 0.3s ease;

    @keyframes slideIn {
      from {
        opacity: 0;
        transform: translateX(100%);
      }
      to {
        opacity: 1;
        transform: translateX(0);
      }
    }

    /* 类型变体 */
    &.info {
      .toast-icon {
        color: #89b4fa;
      }
    }

    &.success {
      .toast-icon {
        color: #a6e3a1;
      }
    }

    &.warning {
      .toast-icon {
        color: #f9e2af;
      }
    }

    &.error {
      .toast-icon {
        color: #f38ba8;
      }
    }
  }

  .toast-icon {
    flex-shrink: 0;
    padding-top: 2px;
  }

  .toast-content {
    flex: 1;
    min-width: 0;
  }

  .toast-title {
    font-weight: 500;
    font-size: 14px;
    color: #cdd6f4;
  }

  .toast-message {
    margin-top: 4px;
    font-size: 13px;
    color: #a6adc8;
    line-height: 1.4;
  }

  .toast-close {
    flex-shrink: 0;
    padding: 4px;
    margin: -4px -4px -4px 0;
    border: none;
    background: transparent;
    color: #6c7086;
    cursor: pointer;
    border-radius: 4px;
    transition: all 0.15s ease;

    &:hover {
      color: #cdd6f4;
      background: rgba(255, 255, 255, 0.1);
    }
  }
</style>
