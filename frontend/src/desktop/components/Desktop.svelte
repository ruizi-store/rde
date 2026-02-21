<script lang="ts">
  import { windowManager } from "$desktop/stores/windows.svelte";
  import { desktop } from "$desktop/stores/desktop.svelte";
  import { workspaces } from "$desktop/stores/workspaces.svelte";
  import { remoteAccessStore } from "$desktop/stores/remote-access.svelte";
  import { musicPlayer } from "$shared/stores/music-player.svelte";
  import Window from "./Window.svelte";
  import TaskBar from "./TaskBar.svelte";
  import DesktopIcon from "./DesktopIcon.svelte";
  import ContextMenu from "./ContextMenu.svelte";
  import GlobalSearch from "./GlobalSearch.svelte";
  import ToastContainer from "$shared/components/ToastContainer.svelte";
  import SnapPreview from "./SnapPreview.svelte";
  import WorkspaceSwitcher from "./WorkspaceSwitcher.svelte";
  import Wallpaper from "./Wallpaper.svelte";
  import ThemeEffects from "./themes/ThemeEffects.svelte";
  import PrivilegeAuthDialog from "$shared/components/PrivilegeAuthDialog.svelte";
  import MusicWallpaper from "$apps/music/MusicWallpaper.svelte";

  let contextMenu = $state<{ x: number; y: number } | null>(null);
  let showSearch = $state(false);
  let showWorkspaceSwitcher = $state(false);

  // 吸附预览状态 - 全局共享
  let snapZone = $state<
    "left" | "right" | "top-left" | "top-right" | "bottom-left" | "bottom-right" | "maximize" | null
  >(null);

  // 过滤后的桌面图标（终端禁用时隐藏终端图标）
  let visibleIcons = $derived(
    desktop.icons.filter((icon) => {
      // 如果是终端图标且终端被禁用，则隐藏
      if (icon.appId === "terminal" && !remoteAccessStore.terminalEnabled) {
        return false;
      }
      return true;
    }),
  );

  // 当前工作区的窗口
  let visibleWindows = $derived(
    windowManager.windowList.filter(
      (w) =>
        workspaces.active.windowIds.includes(w.id) ||
        !workspaces.all.some((ws) => ws.windowIds.includes(w.id)), // 未分配工作区的窗口显示在所有工作区
    ),
  );

  function handleContextMenu(e: MouseEvent) {
    e.preventDefault();
    contextMenu = { x: e.clientX, y: e.clientY };
  }

  function closeContextMenu() {
    contextMenu = null;
  }

  function handleDesktopClick() {
    desktop.selectIcon(null);
    closeContextMenu();
  }

  /* 全局键盘快捷键 */
  function handleKeydown(e: KeyboardEvent) {
    /* Ctrl/Cmd + K: 打开搜索 */
    if ((e.ctrlKey || e.metaKey) && e.key === "k") {
      e.preventDefault();
      showSearch = true;
    }
    /* Ctrl/Cmd + Space: 打开搜索 (备选) */
    if ((e.ctrlKey || e.metaKey) && e.key === " ") {
      e.preventDefault();
      showSearch = true;
    }
    /* Ctrl/Cmd + Tab: 工作区切换器 */
    if ((e.ctrlKey || e.metaKey) && e.key === "Tab") {
      e.preventDefault();
      showWorkspaceSwitcher = true;
    }
    /* Super/Win + 数字: 快速切换工作区 */
    if (e.metaKey && e.key >= "1" && e.key <= "6") {
      e.preventDefault();
      const index = parseInt(e.key) - 1;
      if (index < workspaces.count) {
        workspaces.switchTo(workspaces.all[index].id);
      }
    }
  }

  /* 初始化桌面图标 */
  $effect(() => {
    if (desktop.icons.length === 0) {
      desktop.initDefaultIcons();
    }
  });

  /* 新窗口自动添加到当前工作区 */
  $effect(() => {
    windowManager.windowList.forEach((w) => {
      const assigned = workspaces.all.some((ws) => ws.windowIds.includes(w.id));
      if (!assigned) {
        workspaces.addWindow(w.id);
      }
    });
  });
</script>

<svelte:window onkeydown={handleKeydown} />

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="desktop" oncontextmenu={handleContextMenu} onclick={handleDesktopClick}>
  <!-- 壁纸 -->
  <Wallpaper />

  <!-- 音乐全屏壁纸模式 -->
  {#if musicPlayer.wallpaperMode}
    <MusicWallpaper />
  {:else}
    <!-- 主题特效层（泡泡等）- 音乐壁纸模式时隐藏 -->
    <ThemeEffects theme="aquarium" taskbarHeight={48} />
  {/if}

  <!-- 桌面图标 -->
  <div class="desktop-icons">
    {#each visibleIcons as icon (icon.id)}
      <DesktopIcon {icon} />
    {/each}
  </div>

  <!-- 窗口吸附预览 -->
  <SnapPreview visible={snapZone !== null} zone={snapZone} />

  <!-- 窗口 -->
  {#each visibleWindows as window (window.id)}
    <Window {window} bind:snapPreview={snapZone} />
  {/each}

  <!-- 右键菜单 -->
  {#if contextMenu}
    <ContextMenu x={contextMenu.x} y={contextMenu.y} onclose={closeContextMenu} />
  {/if}

  <!-- 全局搜索 -->
  <GlobalSearch bind:visible={showSearch} />

  <!-- 工作区切换器 -->
  <WorkspaceSwitcher bind:visible={showWorkspaceSwitcher} />

  <!-- 任务栏 -->
  <TaskBar />

  <!-- Toast 通知 -->
  <ToastContainer />

  <!-- 特权操作授权弹窗 -->
  <PrivilegeAuthDialog />
</div>

<style>
  .desktop {
    position: fixed;
    inset: 0;
    background: transparent;
    overflow: hidden;
  }

  .desktop-icons {
    position: relative;
    z-index: 1;
    display: grid;
    grid-template-columns: repeat(auto-fill, 80px);
    grid-auto-rows: minmax(90px, auto);
    gap: 8px;
    padding: 16px;
    height: calc(100% - 48px); /* 减去任务栏高度 */
    align-content: start;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: thin;
    scrollbar-color: rgba(255,255,255,0.2) transparent;
  }
</style>
