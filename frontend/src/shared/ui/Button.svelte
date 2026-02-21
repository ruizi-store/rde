<script lang="ts">
  import type { Snippet } from "svelte";

  interface Props {
    variant?: "primary" | "secondary" | "success" | "danger" | "ghost" | "outline" | "destructive";
    size?: "sm" | "md" | "lg";
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
    loading?: boolean;
    fullWidth?: boolean;
    icon?: Snippet;
    children: Snippet;
    onclick?: (e: MouseEvent) => void;
  }

  let {
    variant = "primary",
    size = "md",
    type = "button",
    disabled = false,
    loading = false,
    fullWidth = false,
    icon,
    children,
    onclick,
  }: Props = $props();
</script>

<button
  {type}
  class="btn {variant} {size}"
  class:full-width={fullWidth}
  disabled={disabled || loading}
  {onclick}
>
  {#if loading}
    <span class="loader"></span>
  {:else if icon}
    <span class="icon">
      {@render icon()}
    </span>
  {/if}
  <span class="label">
    {@render children()}
  </span>
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    border: none;
    border-radius: 6px;
    font-weight: 500;
    cursor: pointer;
    transition:
      background 0.15s,
      opacity 0.15s,
      transform 0.1s;
    white-space: nowrap;

    &:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &:active:not(:disabled) {
      transform: scale(0.98);
    }

    /* Sizes */
    &.sm {
      padding: 6px 12px;
      font-size: 12px;
    }

    &.md {
      padding: 8px 16px;
      font-size: 14px;
    }

    &.lg {
      padding: 12px 24px;
      font-size: 16px;
    }

    /* Variants */
    &.primary {
      background: var(--color-primary, #4a90d9);
      color: white;

      &:hover:not(:disabled) {
        background: var(--color-primary-dark, #357abd);
      }
    }

    &.secondary {
      background: var(--color-secondary, #e9ecef);
      color: var(--text-primary, #333);

      &:hover:not(:disabled) {
        background: var(--color-secondary-hover, #dee2e6);
      }
    }

    &.success {
      background: #03ae00;
      color: white;

      &:hover:not(:disabled) {
        background: #027d00;
      }
    }

    &.danger {
      background: #dc3545;
      color: white;

      &:hover:not(:disabled) {
        background: #c82333;
      }
    }

    &.ghost {
      background: transparent;
      color: var(--text-primary, #333);

      &:hover:not(:disabled) {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
      }
    }

    &.outline {
      background: transparent;
      border: 1px solid var(--border-color, #e0e0e0);
      color: var(--text-primary, #333);

      &:hover:not(:disabled) {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
        border-color: var(--color-primary, #4a90d9);
      }
    }

    &.destructive {
      background: #dc3545;
      color: white;

      &:hover:not(:disabled) {
        background: #c82333;
      }
    }

    &.full-width {
      width: 100%;
    }
  }

  .icon {
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }

  .loader {
    width: 14px;
    height: 14px;
    border: 2px solid transparent;
    border-top-color: currentColor;
    border-radius: 50%;
    animation: spin 0.8s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
