<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";

  interface Props {
    id?: HTMLInputAttributes["id"];
    checked?: boolean;
    disabled?: boolean;
    label?: string;
    size?: "sm" | "md" | "lg";
    onchange?: (checked: boolean) => void;
    onclick?: (e: MouseEvent) => void; // 点击事件，如果调用 e.preventDefault() 则不会切换状态
  }

  let {
    checked = $bindable(false),
    disabled = false,
    label = "",
    size = "md",
    onchange,
    onclick,
  }: Props = $props();

  function toggle(e?: MouseEvent) {
    if (disabled) return;

    // 如果有 onclick 处理器，先调用它
    if (e && onclick) {
      onclick(e);
      // 如果调用了 preventDefault，则不切换状态
      if (e.defaultPrevented) return;
    }

    checked = !checked;
    onchange?.(checked);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === " " || e.key === "Enter") {
      e.preventDefault();
      toggle();
    }
  }
</script>

<label class="switch-wrapper {size}" class:disabled>
  <div
    class="switch"
    class:checked
    role="switch"
    aria-checked={checked}
    aria-disabled={disabled}
    tabindex={disabled ? -1 : 0}
    onclick={(e) => toggle(e)}
    onkeydown={handleKeydown}
  >
    <span class="switch-thumb"></span>
  </div>
  {#if label}
    <span class="switch-label" onclick={(e) => toggle(e)}>{label}</span>
  {/if}
</label>

<style>
  .switch-wrapper {
    display: inline-flex;
    align-items: center;
    gap: 8px;
    cursor: pointer;

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &.sm {
      .switch {
        width: 28px;
        height: 16px;
      }

      .switch-thumb {
        width: 12px;
        height: 12px;
      }

      .switch.checked .switch-thumb {
        transform: translateX(12px);
      }

      .switch-label {
        font-size: 12px;
      }
    }

    &.md {
      .switch {
        width: 40px;
        height: 22px;
      }

      .switch-thumb {
        width: 18px;
        height: 18px;
      }

      .switch.checked .switch-thumb {
        transform: translateX(18px);
      }

      .switch-label {
        font-size: 14px;
      }
    }

    &.lg {
      .switch {
        width: 52px;
        height: 28px;
      }

      .switch-thumb {
        width: 24px;
        height: 24px;
      }

      .switch.checked .switch-thumb {
        transform: translateX(24px);
      }

      .switch-label {
        font-size: 16px;
      }
    }
  }

  .switch {
    position: relative;
    background: var(--bg-tertiary, #e0e0e0);
    border-radius: 100px;
    transition: background 0.2s;
    flex-shrink: 0;

    &:focus-visible {
      outline: none;
      box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
    }

    &.checked {
      background: var(--color-primary, #4a90d9);
    }
  }

  .switch-thumb {
    position: absolute;
    top: 2px;
    left: 2px;
    background: white;
    border-radius: 50%;
    box-shadow: 0 2px 4px rgba(0, 0, 0, 0.15);
    transition: transform 0.2s;
  }

  .switch-label {
    color: var(--text-primary, #333);
    user-select: none;
  }
</style>
