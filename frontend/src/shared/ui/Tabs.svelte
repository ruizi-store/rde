<script lang="ts">
  import type { Snippet } from "svelte";
  import Icon from "@iconify/svelte";

  interface Tab {
    id: string;
    label: string;
    icon?: string;
    disabled?: boolean;
  }

  interface Props {
    tabs: Tab[];
    activeTab?: string;
    variant?: "default" | "pills" | "underline";
    size?: "sm" | "md" | "lg";
    fullWidth?: boolean;
    onchange?: (tabId: string) => void;
    children: Snippet<[string]>;
  }

  let {
    tabs,
    activeTab = $bindable(tabs[0]?.id || ""),
    variant = "default",
    size = "md",
    fullWidth = false,
    onchange,
    children,
  }: Props = $props();

  function selectTab(tabId: string) {
    const tab = tabs.find((t) => t.id === tabId);
    if (tab && !tab.disabled) {
      activeTab = tabId;
      onchange?.(tabId);
    }
  }

  function handleKeydown(e: KeyboardEvent, index: number) {
    const enabledTabs = tabs.filter((t) => !t.disabled);
    const currentEnabledIndex = enabledTabs.findIndex((t) => t.id === tabs[index].id);

    if (e.key === "ArrowLeft" || e.key === "ArrowUp") {
      e.preventDefault();
      const prevIndex = currentEnabledIndex > 0 ? currentEnabledIndex - 1 : enabledTabs.length - 1;
      selectTab(enabledTabs[prevIndex].id);
    } else if (e.key === "ArrowRight" || e.key === "ArrowDown") {
      e.preventDefault();
      const nextIndex = currentEnabledIndex < enabledTabs.length - 1 ? currentEnabledIndex + 1 : 0;
      selectTab(enabledTabs[nextIndex].id);
    }
  }
</script>

<div class="tabs-container variant-{variant} size-{size}" class:full-width={fullWidth}>
  <div class="tabs-header" role="tablist">
    {#each tabs as tab, index}
      <button
        class="tab-item"
        class:active={activeTab === tab.id}
        class:disabled={tab.disabled}
        role="tab"
        aria-selected={activeTab === tab.id}
        aria-disabled={tab.disabled}
        tabindex={activeTab === tab.id ? 0 : -1}
        onclick={() => selectTab(tab.id)}
        onkeydown={(e) => handleKeydown(e, index)}
      >
        {#if tab.icon}
          <span class="tab-icon"><Icon icon={tab.icon} width="16" /></span>
        {/if}
        <span class="tab-label">{tab.label}</span>
      </button>
    {/each}
    {#if variant === "underline"}
      <div class="tab-indicator"></div>
    {/if}
  </div>

  <div class="tabs-content" role="tabpanel">
    {@render children(activeTab)}
  </div>
</div>

<style>
  .tabs-container {
    display: flex;
    flex-direction: column;

    &.full-width {
      .tabs-header {
        width: 100%;
      }

      .tab-item {
        flex: 1;
      }
    }
  }

  .tabs-header {
    display: flex;
    position: relative;
    gap: 4px;
  }

  .tab-item {
    display: flex;
    align-items: center;
    gap: 6px;
    background: none;
    border: none;
    cursor: pointer;
    transition: all 0.15s;
    white-space: nowrap;

    &.disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  }

  /* Size variants */
  .size-sm {
    .tab-item {
      padding: 6px 12px;
      font-size: 12px;
    }

    .tab-icon {
      font-size: 14px;
    }
  }

  .size-md {
    .tab-item {
      padding: 10px 16px;
      font-size: 14px;
    }

    .tab-icon {
      font-size: 16px;
    }
  }

  .size-lg {
    .tab-item {
      padding: 12px 20px;
      font-size: 16px;
    }

    .tab-icon {
      font-size: 18px;
    }
  }

  /* Style variants */
  .variant-default {
    .tabs-header {
      background: var(--bg-tertiary, #f5f5f5);
      border-radius: 8px;
      padding: 4px;
    }

    .tab-item {
      border-radius: 6px;
      color: var(--text-secondary, #666);

      &:hover:not(.disabled) {
        background: var(--bg-hover, rgba(0, 0, 0, 0.05));
      }

      &.active {
        background: var(--bg-card, white);
        color: var(--text-primary, #333);
        box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
      }
    }
  }

  .variant-pills {
    .tabs-header {
      gap: 8px;
    }

    .tab-item {
      border-radius: 20px;
      color: var(--text-secondary, #666);
      background: var(--bg-tertiary, #f5f5f5);

      &:hover:not(.disabled) {
        background: var(--bg-hover, #e8e8e8);
      }

      &.active {
        background: var(--color-primary, #4a90d9);
        color: white;
      }
    }
  }

  .variant-underline {
    .tabs-header {
      border-bottom: 1px solid var(--border-color, #e0e0e0);
      gap: 0;
    }

    .tab-item {
      border-radius: 0;
      color: var(--text-secondary, #666);
      border-bottom: 2px solid transparent;
      margin-bottom: -1px;

      &:hover:not(.disabled) {
        color: var(--text-primary, #333);
        border-bottom-color: var(--border-color, #e0e0e0);
      }

      &.active {
        color: var(--color-primary, #4a90d9);
        border-bottom-color: var(--color-primary, #4a90d9);
      }
    }
  }

  .tabs-content {
    padding: 16px 0;
  }

  .tab-icon {
    display: flex;
    align-items: center;
  }

  .tab-label {
    font-weight: 500;
  }
</style>
