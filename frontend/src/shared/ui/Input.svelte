<script lang="ts">
  import type { HTMLInputAttributes } from "svelte/elements";
  import Icon from "@iconify/svelte";

  interface MyProps {
    type?: "text" | "password" | "email" | "number" | "search" | "url" | "tel" | "mono-code";
    value?: string;
    error?: string;
    icon?: string;
    size?: "sm" | "md" | "lg";
    clearable?: boolean;
    label?: string;
    onchange?: (value: string) => void;
    oninput?: (value: string) => void;
    onenter?: () => void;
  }

  type Props = MyProps & Omit<HTMLInputAttributes, keyof MyProps | "class">;

  let {
    id,
    type = "text",
    value = $bindable(""),
    disabled,
    error = "",
    icon = "",
    size = "md",
    clearable = false,
    label = "",
    onchange,
    oninput,
    onenter,
    ...restProps
  }: Props = $props();

  let showPassword = $state(false);

  const realType = $derived(
    type === "mono-code" ? "text" : type === "password" && showPassword ? "text" : type,
  );

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    oninput?.(value);
  }

  function handleChange(e: Event) {
    const target = e.target as HTMLInputElement;
    onchange?.(target.value);
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Enter") {
      onenter?.();
    }
  }

  function clear() {
    value = "";
    oninput?.("");
    onchange?.("");
  }

  function togglePassword() {
    showPassword = !showPassword;
  }
</script>

<div class="input-container">
  {#if label}
    <label for={id} class="input-label">{label}</label>
  {/if}

  <div class="input-wrapper {size}" class:has-error={!!error} class:disabled>
    {#if icon}
      <span class="input-icon left">
        <Icon {icon} />
      </span>
    {/if}

    <input
      {id}
      type={realType}
      {value}
      {disabled}
      class:has-icon={!!icon}
      class:has-suffix={clearable || type === "password"}
      class:mono-code={type === "mono-code"}
      oninput={handleInput}
      onchange={handleChange}
      onkeydown={handleKeydown}
      {...restProps}
    />

    {#if type === "password"}
      <button
        class="input-icon right toggle-password"
        onclick={togglePassword}
        type="button"
        tabindex="-1"
      >
        <Icon icon={showPassword ? "mdi:eye-off" : "mdi:eye"} />
      </button>
    {:else if clearable && value}
      <button class="input-icon right clear-btn" onclick={clear} type="button" tabindex="-1">
        <Icon icon="mdi:close-circle" />
      </button>
    {/if}
  </div>

  {#if error}
    <span class="error-message">{error}</span>
  {/if}
</div>

<style>
  .input-container {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }

  .input-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }

  .input-wrapper {
    position: relative;
    display: flex;
    align-items: center;

    &.disabled {
      opacity: 0.5;
      pointer-events: none;
    }

    &.has-error {
      input {
        border-color: #dc3545;

        &:focus {
          border-color: #dc3545;
          box-shadow: 0 0 0 3px rgba(220, 53, 69, 0.15);
        }
      }
    }

    &.sm {
      input {
        padding: 6px 10px;
        font-size: 12px;
      }

      .input-icon {
        font-size: 14px;
      }
    }

    &.md {
      input {
        padding: 10px 12px;
        font-size: 14px;
      }

      .input-icon {
        font-size: 18px;
      }
    }

    &.lg {
      input {
        padding: 12px 16px;
        font-size: 16px;
      }

      .input-icon {
        font-size: 20px;
      }
    }
  }

  input {
    width: 100%;
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: var(--radius-md);
    background: var(--bg-input, white);
    color: var(--text-primary, #333);
    outline: none;
    transition:
      border-color 0.15s,
      box-shadow 0.15s;

    &::placeholder {
      color: var(--text-muted, #adb5bd);
    }

    &:focus {
      border-color: var(--color-primary, #4a90d9);
      box-shadow: 0 0 0 3px rgba(74, 144, 217, 0.15);
    }

    &.has-icon {
      padding-left: 36px;
    }

    &.has-suffix {
      padding-right: 36px;
    }

    &.mono-code {
      font-family: var(--font-mono);
      text-align: center;
      letter-spacing: 0.25rem;
    }
  }

  .input-icon {
    position: absolute;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--text-muted, #adb5bd);

    &.left {
      left: 10px;
    }

    &.right {
      right: 10px;
      cursor: pointer;
      background: none;
      border: none;
      padding: 0;

      &:hover {
        color: var(--text-secondary, #666);
      }
    }
  }

  .error-message {
    font-size: 12px;
    color: #dc3545;
    margin-top: 4px;
  }
</style>
