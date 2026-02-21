<script lang="ts">
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import { windows, windowManager } from "$desktop/stores/windows.svelte";
  import { theme } from "$shared/stores/theme.svelte";
  import { desktop } from "$desktop/stores/desktop.svelte";
  import { remoteAccessStore } from "$desktop/stores/remote-access.svelte";

  let { x, y, onclose }: { x: number; y: number; onclose: () => void } = $props();

  interface MenuItem {
    label: string;
    icon?: string;
    action?: () => void;
    separator?: boolean;
    disabled?: boolean;
    shortcut?: string;
  }

  // 开发模式检测
  const isDev = import.meta.env.DEV;

  // 动态生成菜单项
  const menuItems = $derived.by(() => {
    const items: MenuItem[] = [
      { label: $t("desktop.refresh"), icon: "mdi:refresh", action: () => location.reload(), shortcut: "F5" },
      { separator: true, label: "" },
      {
        label: $t("desktop.autoArrangeIcons"),
        icon: "mdi:view-grid-plus-outline",
        action: () => desktop.arrangeIcons(),
      },
      {
        label: $t("desktop.sortByName"),
        icon: "mdi:sort-alphabetical-ascending",
        action: () => desktop.sortByName(),
      },
      {
        label: $t("desktop.sortByType"),
        icon: "mdi:sort-variant",
        action: () => desktop.sortByType(),
      },
      { separator: true, label: "" },
      { label: $t("desktop.changeWallpaper"), icon: "mdi:wallpaper", action: () => changeWallpaper() },
      { separator: true, label: "" },
    ];

    // 只有终端启用时才显示"打开终端"
    if (remoteAccessStore.terminalEnabled) {
      items.push({
        label: $t("desktop.openTerminal"),
        icon: "mdi:console",
        action: () => windows.open("terminal"),
        shortcut: "Ctrl+`",
      });
    }

    items.push({
      label: $t("desktop.openFileManager"),
      icon: "mdi:folder-outline",
      action: () => windows.open("file"),
    });

    // 开发者选项（仅在开发模式显示）
    if (isDev) {
      items.push(
        { separator: true, label: "" },
        {
          label: `🧪 ${$t("desktop.testPackage")}`,
          icon: "mdi:test-tube",
          action: () => openTestPackage(),
        },
      );
    }

    return items;
  });

  function openTestPackage() {
    windowManager.openIframe({
      url: "/test-package/index.html",
      title: $t("desktop.testPackage"),
      icon: "/icons/package.svg",
      packageId: "test-package",
      permissions: ["files:read", "system:read"],
      width: 600,
      height: 500,
    });
  }

  function changeWallpaper() {
    windows.open("settings", { section: "appearance" });
  }

  function handleItemClick(item: MenuItem) {
    if (item.disabled) return;
    item.action?.();
    onclose();
  }

  /* 确保菜单不超出屏幕 */
  let menuStyle = $derived(() => {
    const menuWidth = 220;
    const menuHeight = menuItems.length * 32;
    const maxX = window.innerWidth - menuWidth - 8;
    const maxY = window.innerHeight - menuHeight - 56;

    return `left: ${Math.min(x, maxX)}px; top: ${Math.min(y, maxY)}px;`;
  });
</script>

<svelte:window onclick={onclose} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="context-menu" style={menuStyle()} onclick={(e) => e.stopPropagation()}>
  {#each menuItems as item}
    {#if item.separator}
      <div class="separator"></div>
    {:else}
      <button
        class="menu-item"
        class:disabled={item.disabled}
        onclick={() => handleItemClick(item)}
        disabled={item.disabled}
      >
        {#if item.icon}
          <Icon icon={item.icon} width="18" />
        {:else}
          <span class="icon-placeholder"></span>
        {/if}
        <span class="item-label">{item.label}</span>
        {#if item.shortcut}
          <span class="shortcut">{item.shortcut}</span>
        {/if}
      </button>
    {/if}
  {/each}
</div>

<style>
  .context-menu {
    position: fixed;
    min-width: 220px;
    background: rgba(40, 40, 44, 0.95);
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 4px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    z-index: 99999;

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.95);
      border-color: rgba(0, 0, 0, 0.1);
      box-shadow: 0 8px 32px rgba(0, 0, 0, 0.15);
    }
  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 12px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: rgba(255, 255, 255, 0.9);
    font-size: 13px;
    text-align: left;
    cursor: pointer;

    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.85);
    }

    &:hover:not(.disabled) {
      background: rgba(74, 144, 217, 0.3);

      :global([data-theme="light"]) & {
        background: rgba(74, 144, 217, 0.15);
      }
    }

    &.disabled {
      color: rgba(255, 255, 255, 0.3);
      cursor: not-allowed;

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.3);
      }
    }

    .icon-placeholder {
      width: 18px;
    }

    .item-label {
      flex: 1;
    }

    .shortcut {
      color: rgba(255, 255, 255, 0.4);
      font-size: 11px;

      :global([data-theme="light"]) & {
        color: rgba(0, 0, 0, 0.4);
      }
    }
  }

  .separator {
    height: 1px;
    background: rgba(255, 255, 255, 0.1);
    margin: 4px 8px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.1);
    }
  }
</style>
