<script lang="ts">
  import Icon from "@iconify/svelte";

  interface Props {
    value?: string;
    options?: string[];
    label?: string;
    placeholder?: string;
    size?: "sm" | "md" | "lg";
    error?: string;
    disabled?: boolean;
    onchange?: (value: string) => void;
  }

  let {
    value = $bindable(""),
    options = [],
    label = "",
    placeholder = "",
    size = "md",
    error = "",
    disabled = false,
    onchange,
  }: Props = $props();

  let open = $state(false);
  let inputEl: HTMLInputElement | undefined = $state();
  let containerEl: HTMLDivElement | undefined = $state();

  const filtered = $derived(
    value
      ? options.filter((o) => o.toLowerCase().includes(value.toLowerCase()))
      : options,
  );

  function handleInput(e: Event) {
    const target = e.target as HTMLInputElement;
    value = target.value;
    open = true;
    onchange?.(value);
  }

  function handleFocus() {
    open = true;
  }

  function select(option: string) {
    value = option;
    open = false;
    onchange?.(option);
    inputEl?.focus();
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      open = false;
    }
  }

  function handleClickOutside(e: MouseEvent) {
    if (containerEl && !containerEl.contains(e.target as Node)) {
      open = false;
    }
  }
</script>

<svelte:window onclick={handleClickOutside} />

<div class="combobox-container" bind:this={containerEl}>
  {#if label}
    <label class="combobox-label">{label}</label>
  {/if}

  <div class="combobox-wrapper {size}" class:has-error={!!error} class:disabled>
    <input
      bind:this={inputEl}
      type="text"
      {value}
      {placeholder}
      {disabled}
      oninput={handleInput}
      onfocus={handleFocus}
      onkeydown={handleKeydown}
      autocomplete="off"
    />
    <button
      class="combobox-toggle"
      type="button"
      tabindex="-1"
      onclick={() => { open = !open; inputEl?.focus(); }}
    >
      <Icon icon={open ? "mdi:chevron-up" : "mdi:chevron-down"} />
    </button>
  </div>

  {#if open && filtered.length > 0}
    <ul class="combobox-dropdown">
      {#each filtered as option}
        <li>
          <button type="button" onclick={() => select(option)}>
            <Icon icon="mdi:account-outline" width="16" />
            {option}
          </button>
        </li>
      {/each}
    </ul>
  {/if}

  {#if error}
    <span class="error-message">{error}</span>
  {/if}
</div>

<style>
  .combobox-container {
    display: flex;
    flex-direction: column;
    gap: 6px;
    position: relative;
  }

  .combobox-label {
    font-size: 13px;
    font-weight: 500;
    color: var(--text-secondary, #666);
  }

  .combobox-wrapper {
    position: relative;
    display: flex;
    align-items: center;

    &.disabled {
      opacity: 0.5;
      pointer-events: none;
    }

    &.has-error input {
      border-color: #dc3545;
    }

    &.sm input {
      padding: 6px 32px 6px 10px;
      font-size: 12px;
    }

    &.md input {
      padding: 10px 36px 10px 12px;
      font-size: 14px;
    }

    &.lg input {
      padding: 12px 40px 12px 16px;
      font-size: 16px;
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
  }

  .combobox-toggle {
    position: absolute;
    right: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: none;
    border: none;
    padding: 0;
    cursor: pointer;
    color: var(--text-muted, #adb5bd);

    &:hover {
      color: var(--text-secondary, #666);
    }
  }

  .combobox-dropdown {
    position: absolute;
    top: 100%;
    left: 0;
    right: 0;
    z-index: 1000;
    margin-top: 4px;
    padding: 4px;
    background: var(--bg-surface, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: var(--radius-md);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
    max-height: 200px;
    overflow-y: auto;
    list-style: none;

    li button {
      width: 100%;
      display: flex;
      align-items: center;
      gap: 8px;
      padding: 8px 10px;
      border: none;
      border-radius: var(--radius-sm, 4px);
      background: none;
      color: var(--text-primary, #333);
      font-size: 13px;
      cursor: pointer;
      text-align: left;

      &:hover {
        background: var(--bg-hover, #f0f0f0);
      }
    }
  }

  .error-message {
    font-size: 12px;
    color: #dc3545;
    margin-top: 4px;
  }
</style>
