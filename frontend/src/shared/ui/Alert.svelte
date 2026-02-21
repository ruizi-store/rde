<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";
  import { t } from "$lib/i18n";

  interface Props {
    type?: "info" | "success" | "warning" | "error";
    variant?: "info" | "success" | "warning" | "error";
    title?: string;
    closable?: boolean;
    showIcon?: boolean;
    onclose?: () => void;
    children: Snippet;
  }

  let {
    type = "info",
    variant,
    title = "",
    closable = false,
    showIcon = true,
    onclose,
    children,
  }: Props = $props();

  // variant 优先于 type
  let alertType = $derived(variant || type);

  let visible = $state(true);

  const icons = {
    info: "mdi:information",
    success: "mdi:check-circle",
    warning: "mdi:alert",
    error: "mdi:alert-circle",
  };

  function close() {
    visible = false;
    onclose?.();
  }
</script>

{#if visible}
  <div class="alert alert-{alertType}">
    {#if showIcon}
      <span class="alert-icon">
        <Icon icon={icons[alertType]} />
      </span>
    {/if}
    <div class="alert-content">
      {#if title}
        <div class="alert-title">{title}</div>
      {/if}
      <div class="alert-message">
        {@render children()}
      </div>
    </div>
    {#if closable}
      <button class="alert-close" onclick={close} aria-label={$t('common.close')}>
        <Icon icon="mdi:close" />
      </button>
    {/if}
  </div>
{/if}

<style>
  .alert {
    display: flex;
    align-items: flex-start;
    gap: 12px;
    padding: 12px 16px;
    border-radius: 8px;
    animation: slideIn 0.2s ease-out;
  }

  .alert-info {
    background: #e7f3ff;
    border: 1px solid #91caff;
    color: #0958d9;

    .alert-icon {
      color: #1677ff;
    }
  }

  .alert-success {
    background: #f6ffed;
    border: 1px solid #b7eb8f;
    color: #389e0d;

    .alert-icon {
      color: #52c41a;
    }
  }

  .alert-warning {
    background: #fffbe6;
    border: 1px solid #ffe58f;
    color: #d48806;

    .alert-icon {
      color: #faad14;
    }
  }

  .alert-error {
    background: #fff2f0;
    border: 1px solid #ffccc7;
    color: #cf1322;

    .alert-icon {
      color: #ff4d4f;
    }
  }

  .alert-icon {
    display: flex;
    flex-shrink: 0;
    font-size: 20px;
    margin-top: 2px;
  }

  .alert-content {
    flex: 1;
    min-width: 0;
  }

  .alert-title {
    font-weight: 600;
    font-size: 14px;
    margin-bottom: 4px;
  }

  .alert-message {
    font-size: 14px;
    line-height: 1.5;
  }

  .alert-close {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 24px;
    height: 24px;
    background: none;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    opacity: 0.6;
    transition: opacity 0.15s;
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
      transform: translateY(-8px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
