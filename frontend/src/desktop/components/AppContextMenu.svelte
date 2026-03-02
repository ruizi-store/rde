<script lang="ts">
  import Icon from "@iconify/svelte";
  import { t } from "svelte-i18n";
  import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
  import { desktop } from "$desktop/stores/desktop.svelte";
  import { windows } from "$desktop/stores/windows.svelte";
  import { isUninstallable } from "$shared/types/apps";
  import { showToast } from "$shared/ui";

  // 获取应用的本地化名称
  function getAppDisplayName(app: ExtendedAppDefinition): string {
    const key = `apps.names.${app.id}`;
    const translated = $t(key);
    return translated === key ? app.name : translated;
  }

  interface Props {
    x: number;
    y: number;
    app: ExtendedAppDefinition;
    context: "desktop" | "startmenu" | "taskbar";
    onclose: () => void;
  }

  let { x, y, app, context, onclose }: Props = $props();

  interface MenuItem {
    label: string;
    icon?: string;
    action?: () => void;
    separator?: boolean;
    disabled?: boolean;
    danger?: boolean;
  }

  // 根据上下文生成菜单项
  let menuItems = $derived(() => {
    const items: MenuItem[] = [];

    // 打开
    items.push({
      label: $t("desktop.contextMenu.open"),
      icon: "mdi:open-in-new",
      action: () => openApp(),
    });

    items.push({ separator: true, label: "" });

    // 任务栏固定/取消固定
    if (apps.isPinned(app.id)) {
      items.push({
        label: $t("desktop.contextMenu.unpinFromTaskbar"),
        icon: "mdi:pin-off-outline",
        action: () => unpinFromTaskbar(),
      });
    } else {
      items.push({
        label: $t("desktop.contextMenu.pinToTaskbar"),
        icon: "mdi:pin-outline",
        action: () => pinToTaskbar(),
      });
    }

    // 开始菜单固定/取消固定
    if (apps.isPinnedInStartMenu(app.id)) {
      items.push({
        label: $t("desktop.contextMenu.unpinFromStart"),
        icon: "mdi:pin-off",
        action: () => unpinFromStartMenu(),
      });
    } else {
      items.push({
        label: $t("desktop.contextMenu.pinToStart"),
        icon: "mdi:pin",
        action: () => pinToStartMenu(),
      });
    }

    // 桌面快捷方式（仅在开始菜单和任务栏中显示）
    if (context !== "desktop") {
      const isOnDesktop = desktop.icons.some((icon) => icon.appId === app.id);
      if (isOnDesktop) {
        items.push({
          label: $t("desktop.contextMenu.removeFromDesktop"),
          icon: "mdi:desktop-mac-dashboard",
          action: () => removeFromDesktop(),
        });
      } else {
        items.push({
          label: $t("desktop.contextMenu.addToDesktop"),
          icon: "mdi:desktop-mac-dashboard",
          action: () => addToDesktop(),
        });
      }
    }

    // 从桌面移除（仅在桌面图标上显示）
    if (context === "desktop") {
      items.push({
        label: $t("desktop.contextMenu.removeFromDesktop"),
        icon: "mdi:close-circle-outline",
        action: () => removeFromDesktop(),
      });
    }

    // 卸载（仅第三方应用）
    if (
      app.source === "docker_store"
    ) {
      items.push({ separator: true, label: "" });
      items.push({
        label: $t("desktop.contextMenu.uninstall"),
        icon: "mdi:delete-outline",
        action: () => uninstallApp(),
        danger: true,
      });
    }

    return items;
  });

  function openApp() {
    apps.launch(app.id);
    onclose();
  }

  function pinToTaskbar() {
    apps.pin(app.id);
    onclose();
  }

  function unpinFromTaskbar() {
    apps.unpin(app.id);
    onclose();
  }

  function pinToStartMenu() {
    apps.pinToStartMenu(app.id);
    onclose();
  }

  function unpinFromStartMenu() {
    apps.unpinFromStartMenu(app.id);
    onclose();
  }

  function addToDesktop() {
    // 找到一个空位置
    const existingPositions = new Set(desktop.icons.map((icon) => `${icon.x},${icon.y}`));
    let x = 0,
      y = 0;
    while (existingPositions.has(`${x},${y}`)) {
      y++;
      if (y > 8) {
        y = 0;
        x++;
      }
    }

    desktop.addIcon({
      name: getAppDisplayName(app),
      icon: app.icon,
      appId: app.id,
      x,
      y,
    });
    onclose();
  }

  function removeFromDesktop() {
    const icon = desktop.icons.find((i) => i.appId === app.id);
    if (icon) {
      desktop.removeIcon(icon.id);
    }
    onclose();
  }

  async function uninstallApp() {
    const confirmed = confirm($t("desktop.contextMenu.confirmUninstall", { values: { appName: getAppDisplayName(app) } }));
    if (!confirmed) {
      onclose();
      return;
    }

    try {
      // TODO: Docker 应用卸载
      
      // 从桌面移除快捷方式
      desktop.removeIconByAppId(app.id);
      // 从前端状态移除
      apps.uninstallApp(app.id);
    } catch (e: any) {
      showToast($t("desktop.contextMenu.uninstallFailed", { values: { error: e.message } }), "error");
    }
    onclose();
  }

  function handleItemClick(item: MenuItem) {
    if (item.disabled) return;
    item.action?.();
  }

  /* 确保菜单不超出屏幕 */
  let menuStyle = $derived(() => {
    const menuWidth = 200;
    const menuHeight = menuItems().length * 32;
    const maxX = window.innerWidth - menuWidth - 8;
    const maxY = window.innerHeight - menuHeight - 56;

    return `left: ${Math.min(x, maxX)}px; top: ${Math.min(y, maxY)}px;`;
  });
</script>

<svelte:window onclick={onclose} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="app-context-menu" style={menuStyle()} onclick={(e) => e.stopPropagation()}>
  <!-- 应用信息头部 -->
  <div class="menu-header">
    <img
      src={app.icon}
      alt={getAppDisplayName(app)}
      class="app-icon"
      onerror={(e) => ((e.currentTarget as HTMLImageElement).src = "/icons/default.svg")}
    />
    <div class="app-info">
      <span class="app-name">{getAppDisplayName(app)}</span>
    </div>
  </div>

  <div class="separator"></div>

  {#each menuItems() as item}
    {#if item.separator}
      <div class="separator"></div>
    {:else}
      <button
        class="menu-item"
        class:disabled={item.disabled}
        class:danger={item.danger}
        onclick={() => handleItemClick(item)}
        disabled={item.disabled}
      >
        {#if item.icon}
          <Icon icon={item.icon} width="18" />
        {:else}
          <span class="icon-placeholder"></span>
        {/if}
        <span class="item-label">{item.label}</span>
      </button>
    {/if}
  {/each}
</div>

<style>
  .app-context-menu {
    position: fixed;
    min-width: 200px;
    background: rgba(40, 40, 44, 0.95);
    backdrop-filter: blur(20px);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 10px;
    padding: 6px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
    z-index: 99999;

    @media (prefers-color-scheme: light) {
      background: rgba(255, 255, 255, 0.95);
      border-color: rgba(0, 0, 0, 0.1);
    }
  }

  .menu-header {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 10px;

    .app-icon {
      width: 32px;
      height: 32px;
      border-radius: 8px;
      object-fit: contain;
      background: rgba(255, 255, 255, 0.1);
    }

    .app-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .app-name {
      font-size: 13px;
      font-weight: 500;
      color: rgba(255, 255, 255, 0.9);

      @media (prefers-color-scheme: light) {
        color: #333;
      }
    }

  }

  .menu-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 8px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: rgba(255, 255, 255, 0.9);
    font-size: 13px;
    text-align: left;
    cursor: pointer;

    @media (prefers-color-scheme: light) {
      color: #333;
    }

    &:hover:not(.disabled) {
      background: rgba(74, 144, 217, 0.3);

      @media (prefers-color-scheme: light) {
        background: rgba(0, 102, 204, 0.1);
      }
    }

    &.disabled {
      color: rgba(255, 255, 255, 0.3);
      cursor: not-allowed;

      @media (prefers-color-scheme: light) {
        color: rgba(0, 0, 0, 0.3);
      }
    }

    &.danger {
      color: #dc3545;

      &:hover:not(.disabled) {
        background: rgba(220, 53, 69, 0.2);
      }
    }

    .icon-placeholder {
      width: 18px;
    }

    .item-label {
      flex: 1;
    }
  }

  .separator {
    height: 1px;
    background: rgba(255, 255, 255, 0.1);
    margin: 4px 8px;

    @media (prefers-color-scheme: light) {
      background: rgba(0, 0, 0, 0.1);
    }
  }
</style>
