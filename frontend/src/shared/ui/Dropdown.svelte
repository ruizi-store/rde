<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";

  interface MenuItem {
    id: string;
    label: string;
    icon?: string;
    disabled?: boolean;
    divider?: boolean;
    danger?: boolean;
  }

  interface Props {
    items: MenuItem[];
    trigger: Snippet;
    position?: "bottom-left" | "bottom-right" | "top-left" | "top-right";
    onselect?: (itemId: string) => void;
  }

  let { items, trigger, position = "bottom-left", onselect }: Props = $props();

  let open = $state(false);
  let menuRef: HTMLDivElement;

  function toggle(e: MouseEvent) {
    e.stopPropagation();
    open = !open;
  }

  function select(item: MenuItem) {
    if (!item.disabled && !item.divider) {
      onselect?.(item.id);
      open = false;
    }
  }

  function handleClickOutside(e: MouseEvent) {
    if (menuRef && !menuRef.contains(e.target as Node)) {
      open = false;
    }
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === "Escape") {
      open = false;
    }
  }

  $effect(() => {
    if (open) {
      document.addEventListener("click", handleClickOutside);
      document.addEventListener("keydown", handleKeydown);
    }

    return () => {
      document.removeEventListener("click", handleClickOutside);
      document.removeEventListener("keydown", handleKeydown);
    };
  });
</script>

<div class="dropdown" bind:this={menuRef}>
  <div
    class="dropdown-trigger"
    onclick={toggle}
    role="button"
    tabindex="0"
    aria-haspopup="true"
    aria-expanded={open}
  >
    {@render trigger()}
  </div>

  {#if open}
    <div class="dropdown-menu {position}">
      {#each items as item}
        {#if item.divider}
          <div class="menu-divider"></div>
        {:else}
          <button
            class="menu-item"
            class:disabled={item.disabled}
            class:danger={item.danger}
            onclick={() => select(item)}
            disabled={item.disabled}
          >
            {#if item.icon}
              <span class="item-icon">
                <Icon icon={item.icon} />
              </span>
            {/if}
            <span class="item-label">{item.label}</span>
          </button>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .dropdown {
    position: relative;
    display: inline-flex;
  }

  .dropdown-trigger {
    cursor: pointer;
  }

  .dropdown-menu {
    position: absolute;
    z-index: 1000;
    min-width: 160px;
    background: var(--bg-card, white);
    border: 1px solid var(--border-color, #e0e0e0);
    border-radius: 8px;
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.12);
    padding: 4px;
    animation: slideIn 0.15s ease-out;

    &.bottom-left {
      top: 100%;
      left: 0;
      margin-top: 4px;
    }

    &.bottom-right {
      top: 100%;
      right: 0;
      margin-top: 4px;
    }

    &.top-left {
      bottom: 100%;
      left: 0;
      margin-bottom: 4px;
    }

    &.top-right {
      bottom: 100%;
      right: 0;
      margin-bottom: 4px;
    }
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    background: none;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    color: var(--text-primary, #333);
    cursor: pointer;
    text-align: left;
    transition: background 0.1s;

    &:hover:not(.disabled) {
      background: var(--bg-hover, #f5f5f5);
    }

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    &.danger {
      color: #dc3545;

      &:hover:not(.disabled) {
        background: #fff5f5;
      }
    }
  }

  .item-icon {
    display: flex;
    color: var(--text-muted, #888);
    font-size: 16px;

    .danger & {
      color: #dc3545;
    }
  }

  .item-label {
    flex: 1;
  }

  .menu-divider {
    height: 1px;
    background: var(--border-color, #e0e0e0);
    margin: 4px 0;
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>
