<script lang="ts">
  import type { HTMLSelectAttributes } from "svelte/elements";
  import { t } from "$lib/i18n";

  import type { SelectOption } from "./common.ts";

  interface Props {
    id?: HTMLSelectAttributes["id"];
    options: SelectOption[];
    value?: string;
    placeholder?: string;
    disabled?: boolean;
    error?: string;
    size?: "sm" | "md" | "lg";
    onchange?: (value: string) => void;
  }

  let {
    id,
    options,
    value = $bindable(""),
    placeholder = $t('common.select'),
    disabled = false,
    error = "",
    size = "md",
    onchange,
  }: Props = $props();

  function handleChange(e: Event) {
    const target = e.target as HTMLSelectElement;
    value = target.value;
    onchange?.(value);
  }
</script>

<div class="select-wrapper {size}" class:has-error={!!error} class:disabled>
  <select {id} {value} {disabled} onchange={handleChange} class:placeholder={!value}>
    {#if placeholder}
      <option value="" disabled>{placeholder}</option>
    {/if}
    {#each options as option}
      <option value={option.value} disabled={option.disabled}>
        {option.label}
      </option>
    {/each}
  </select>
  <span class="select-arrow">
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none">
      <path
        d="M3 4.5L6 7.5L9 4.5"
        stroke="currentColor"
        stroke-width="1.5"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </svg>
  </span>
  {#if error}
    <span class="error-message">{error}</span>
  {/if}
</div>

<style>
  .select-wrapper {
    position: relative;
    display: inline-flex;

    &.disabled {
      opacity: 0.5;
      pointer-events: none;
    }

    &.has-error {
      select {
        border-color: #dc3545;

        &:focus {
          border-color: #dc3545;
          box-shadow: 0 0 0 3px rgba(220, 53, 69, 0.15);
        }
      }
    }

    &.sm {
      select {
        padding: 6px 28px 6px 10px;
        font-size: 12px;
      }
    }

    &.md {
      select {
        padding: 10px 32px 10px 12px;
        font-size: 14px;
      }
    }

    &.lg {
      select {
        padding: 12px 36px 12px 16px;
        font-size: 16px;
      }
    }
  }

  select {
    appearance: none;
    width: 100%;
    min-width: 120px;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    background: var(--bg-input, white);
    color: var(--text-primary, #333);
    outline: none;
    cursor: pointer;
    transition:
      border-color 0.15s,
      box-shadow 0.15s;

    &:focus {
      border-color: var(--color-primary, #4a90d9);
      box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
    }

    &.placeholder {
      color: var(--text-muted, #adb5bd);
    }
  }

  .select-arrow {
    position: absolute;
    right: 10px;
    top: 50%;
    transform: translateY(-50%);
    pointer-events: none;
    color: var(--text-muted, #adb5bd);
    display: flex;
    align-items: center;
  }

  .error-message {
    position: absolute;
    bottom: -18px;
    left: 0;
    font-size: 12px;
    color: #dc3545;
  }
</style>
