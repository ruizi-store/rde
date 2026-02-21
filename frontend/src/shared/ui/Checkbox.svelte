<script lang="ts">
  import Icon from "@iconify/svelte";
  import type { HTMLButtonAttributes } from "svelte/elements";

  interface Props {
    id?: HTMLButtonAttributes["id"];
    checked?: boolean;
    disabled?: boolean;
    label?: string;
    indeterminate?: boolean;
    size?: "sm" | "md" | "lg";
    onchange?: (checked: boolean) => void;
  }

  let {
    id,
    checked = $bindable(false),
    disabled = false,
    label = "",
    indeterminate = false,
    size = "md",
    onchange,
  }: Props = $props();

  function toggle() {
    if (!disabled) {
      checked = !checked;
      onchange?.(checked);
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === " " || e.key === "Enter") {
      e.preventDefault();
      toggle();
    }
  }
</script>

<button
  {id}
  role="checkbox"
  aria-checked={indeterminate ? "mixed" : checked}
  aria-disabled={disabled}
  tabindex={disabled ? -1 : 0}
  onclick={toggle}
  onkeydown={handleKeydown}
  class="checkbox-wrapper {size}"
  class:checked
  class:indeterminate
  class:disabled
>
  <span class="box">
    {#if indeterminate}
      <Icon icon="mdi:minus" />
    {:else if checked}
      <Icon icon="mdi:check" />
    {/if}
  </span>
  {#if label}
    <span class="label">{label}</span>
  {/if}
</button>

<style>
  .checkbox-wrapper {
    display: inline-flex;
    align-items: center;
    gap: calc(var(--spacing));
    cursor: pointer;

    .box {
      display: flex;
      align-items: center;
      justify-content: center;
      border: 2px solid var(--border-color, #d9d9d9);
      border-radius: 4px;
      background: var(--bg-input, white);
      transition: all 0.15s;
      flex-shrink: 0;

      &:focus-visible {
        outline: none;
        box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
      }

      :global(svg) {
        display: block;
      }
    }

    &.checked,
    &.indeterminate {
      .box {
        background: var(--color-primary, #4a90d9);
        border-color: var(--color-primary, #4a90d9);
        color: white;
      }
    }

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &:not(:disabled) {
      .box:hover {
        border-color: var(--color-primary, #4a90d9);
      }
    }

    &.sm {
      .box {
        width: var(--text-sm);
        height: var(--text-sm);

        :global(svg) {
          font-size: var(--text-xs);
        }
      }

      .label {
        font-size: var(--text-sm);
      }
    }

    &.md {
      .box {
        width: var(--text-base);
        height: var(--text-base);

        :global(svg) {
          font-size: var(--text-sm);
        }
      }

      .label {
        font-size: var(--text-base);
      }
    }

    &.lg {
      .box {
        width: var(--text-lg);
        height: var(--text-lg);

        :global(svg) {
          font-size: var(--text-base);
        }
      }

      .label {
        font-size: var(--text-lg);
      }
    }
  }
</style>
