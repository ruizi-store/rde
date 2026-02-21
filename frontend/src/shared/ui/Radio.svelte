<script lang="ts">
  import type { SelectOption } from "./common.ts";

  interface Props {
    name?: HTMLInputElement["name"];
    options: SelectOption[];
    value?: string;
    disabled?: boolean;
    error?: string;
    size?: "sm" | "md" | "lg";
    onchange?: (value: string) => void;
  }

  let {
    name,
    options,
    value = $bindable(""),
    disabled = false,
    error = "",
    size = "md",
    onchange,
  }: Props = $props();

  function select(v: string) {
    value = v;
    onchange?.(v);
  }

  const uid = $props.id();
  name ??= uid;
</script>

<div class="radio-container {size}" class:has-error={!!error} class:disabled>
  {#each options as option}
    <input
      id={uid + option.value}
      type="radio"
      {name}
      checked={value == option.value}
      onchange={() => select(option.value)}
    />
    <label for={uid + option.value} class:disabled={option.disabled}>
      {option.label}
    </label>
  {/each}
  {#if error}
    <span class="error-message">{error}</span>
  {/if}
</div>

<style>
  .radio-container {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1rem;

    &.disabled {
      opacity: 0.5;
      pointer-events: none;
    }

    &.has-error {
      border-color: #dc3545;

      &:focus {
        border-color: #dc3545;
        box-shadow: 0 0 0 3px rgba(220, 53, 69, 0.15);
      }
    }

    &.sm {
      label {
        padding: 6px 28px 6px 10px;
        font-size: 12px;
      }
    }

    &.md {
      label {
        padding: 10px 32px 10px 12px;
        font-size: 14px;
      }
    }

    &.lg {
      label {
        padding: 12px 36px 12px 16px;
        font-size: 16px;
      }
    }
  }

  input[type="radio"] {
    display: none;
  }

  label {
    font-weight: 500;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 6px;
    background: var(--bg-input, white);
    color: var(--text-primary, #333);
    cursor: pointer;
    transition:
      background 0.15s,
      opacity 0.15s,
      transform 0.1s;

    &:hover {
      background: var(--color-primary-dark, #357abd);
    }
  }

  input:checked + label {
    background: var(--color-primary, #4a90d9);
    color: white;
  }

  .error-message {
    color: var(--color-danger);
    grid-column: 1 / -1;
    justify-self: center;
  }
</style>
