<script lang="ts">
  import { t } from "svelte-i18n";
  import { windowManager, windows } from "$desktop/stores/windows.svelte";
  import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
  import { theme } from "$shared/stores/theme.svelte";
  import { uiState } from "$shared/stores/ui-state.svelte";
  import { musicPlayer } from "$shared/stores/music-player.svelte";
  import Icon from "@iconify/svelte";
  import AppLauncher from "./AppLauncher.svelte";
  import QuickSettings from "./QuickSettings.svelte";
  import CalendarPopup from "./CalendarPopup.svelte";
  import NotificationPanel from "./NotificationPanel.svelte";

  // 获取应用的本地化名称
  function getAppDisplayName(app: ExtendedAppDefinition): string {
    const key = `apps.names.${app.id}`;
    const translated = $t(key);
    return translated === key ? app.name : translated;
  }

  let time = $state(new Date());
  let showLauncher = $state(false);
  let showQuickSettings = $state(false);
  let showCalendar = $state(false);
  let showNotifications = $state(false);

  // 维护一个稳定的窗口 ID 顺序列表（按创建顺序）
  let windowIdOrder = $state<string[]>([]);

  // 监听窗口列表变化，维护稳定顺序
  $effect(() => {
    const currentIds = new Set(windowManager.windowList.map((w) => w.id));
    const existingIds = windowIdOrder.filter((id) => currentIds.has(id));
    const newIds = windowManager.windowList
      .filter((w) => !windowIdOrder.includes(w.id))
      .map((w) => w.id);

    // 只有当有新增或删除时才更新
    if (newIds.length > 0 || existingIds.length !== windowIdOrder.length) {
      windowIdOrder = [...existingIds, ...newIds];
    }
  });

  // 获取稳定排序的所有窗口列表（方案B：显示所有窗口）
  let stableWindowList = $derived(() => {
    // 按照 windowIdOrder 中的顺序排序
    return [...windowManager.windowList].sort((a, b) => {
      const aIndex = windowIdOrder.indexOf(a.id);
      const bIndex = windowIdOrder.indexOf(b.id);
      return aIndex - bIndex;
    });
  });

  /* 更新时间 */
  $effect(() => {
    const interval = setInterval(() => {
      time = new Date();
    }, 1000);
    return () => clearInterval(interval);
  });

  function formatTime(date: Date): string {
    return date.toLocaleTimeString("zh-CN", {
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function formatDate(date: Date): string {
    return date.toLocaleDateString("zh-CN", {
      month: "short",
      day: "numeric",
      weekday: "short",
    });
  }

  function handleWindowClick(windowId: string) {
    const win = windowManager.windowList.find((w) => w.id === windowId);
    if (!win) return;

    if (win.isFocused && !win.isMinimized) {
      // 如果窗口已聚焦且未最小化，则最小化
      windowManager.minimize(windowId);
    } else {
      // 否则聚焦并恢复窗口
      windowManager.focus(windowId);
    }
  }

  /* 方案B：固定图标作为启动器，始终启动新实例 */
  function handleAppClick(appId: string) {
    apps.launch(appId);
  }

  /* 切换启动器 */
  function toggleLauncher() {
    showLauncher = !showLauncher;
    showQuickSettings = false;
    showCalendar = false;
    showNotifications = false;
  }

  /* 切换快速设置 */
  function toggleQuickSettings() {
    showQuickSettings = !showQuickSettings;
    showLauncher = false;
    showCalendar = false;
    showNotifications = false;
  }

  /* 切换日历 */
  function toggleCalendar() {
    showCalendar = !showCalendar;
    showQuickSettings = false;
    showLauncher = false;
    showNotifications = false;
  }

  /* 切换通知面板 */
  function toggleNotifications() {
    showNotifications = !showNotifications;
    showQuickSettings = false;
    showLauncher = false;
    showCalendar = false;
  }

  /* 打开通知面板（由其他组件请求时） */
  function openNotifications() {
    showNotifications = true;
    showQuickSettings = false;
    showLauncher = false;
    showCalendar = false;
  }

  /* 监听 uiState 的通知面板打开请求 */
  $effect(() => {
    if (uiState.hasNotificationPanelRequest) {
      if (uiState.consumeNotificationPanelRequest()) {
        openNotifications();
      }
    }
  });

  /* 显示桌面 */
  function showDesktop() {
    windowManager.minimizeAll();
  }
</script>

<AppLauncher bind:visible={showLauncher} />
<QuickSettings bind:visible={showQuickSettings} />
<CalendarPopup bind:visible={showCalendar} />
<NotificationPanel bind:visible={showNotifications} />

<footer class="taskbar">
  <!-- 开始按钮 -->
  <button class="start-button" class:active={showLauncher} onclick={toggleLauncher} title={$t("desktop.launcherTitle")}>
    <Icon icon="mdi:apps" width="24" />
  </button>

  <div class="separator"></div>

  <!-- 固定的应用（方案B：作为启动器，无指示器） -->
  <div class="pinned-apps">
    {#each apps.pinned as app (app.id)}
      <button
        class="taskbar-item app-item launcher"
        onclick={() => handleAppClick(app.id)}
        title="{getAppDisplayName(app)}（{$t("desktop.clickToLaunch")}）"
      >
        <img
          src={app.icon}
          alt={getAppDisplayName(app)}
          onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = "none")}
        />
      </button>
    {/each}
  </div>

  <div class="separator"></div>

  <!-- 所有打开的窗口（方案B：包括固定应用的窗口） -->
  <div class="window-list">
    {#each stableWindowList() as win (win.id)}
      <button
        class="taskbar-item window-item"
        class:focused={win.isFocused}
        class:minimized={win.isMinimized}
        onclick={() => handleWindowClick(win.id)}
        title={win.title}
      >
        <img
          src={win.icon}
          alt=""
          onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = "none")}
        />
        <span class="title">{win.title}</span>
      </button>
    {/each}
  </div>

  <!-- 弹性空间 -->
  <div class="spacer"></div>

  <!-- 音乐播放器迷你控件（isActive 时一直显示） -->
  {#if musicPlayer.isActive}
    <div class="music-widget" class:wallpaper-mode={musicPlayer.wallpaperMode}>
      <!-- 滚动歌词/歌曲标题（点击打开窗口） -->
      <button 
        class="mini-lyrics-container"
        onclick={() => apps.launch("music")}
        title={$t("desktop.openMusicPlayer")}
      >
        {#if musicPlayer.currentLyricLine}
          <span class="mini-lyrics">{musicPlayer.currentLyricLine}</span>
        {:else}
          <span class="mini-title">{musicPlayer.displayTitle}</span>
        {/if}
      </button>
      <div class="music-controls">
        <button class="music-btn" onclick={() => musicPlayer.prev()} title={$t("desktop.prevTrack")}>
          <Icon icon="mdi:skip-previous" width="16" />
        </button>
        <button class="music-btn play" onclick={() => musicPlayer.toggle()} title={musicPlayer.isPlaying ? $t("desktop.pause") : $t("desktop.play")}>
          <Icon icon={musicPlayer.isPlaying ? "mdi:pause" : "mdi:play"} width="18" />
        </button>
        <button class="music-btn" onclick={() => musicPlayer.next()} title={$t("desktop.nextTrack")}>
          <Icon icon="mdi:skip-next" width="16" />
        </button>
        <!-- 切换特效按钮 -->
        <button 
          class="music-btn mode-btn"
          class:active={musicPlayer.wallpaperMode}
          onclick={() => musicPlayer.cycleVisualizerMode()} 
          title={$t("desktop.switchEffect", { values: { mode: musicPlayer.getVisualizerModeName() } })}
        >
          <Icon icon={musicPlayer.getVisualizerModeIcon()} width="16" />
        </button>
        <!-- 退出按钮 -->
        <button 
          class="music-btn close-btn" 
          onclick={() => musicPlayer.exit()} 
          title={$t("desktop.exitMusicPlayer")}
        >
          <Icon icon="mdi:close" width="16" />
        </button>
      </div>
    </div>
  {/if}

  <!-- 系统托盘 -->
  <div class="system-tray">
    <!-- 主题切换按钮 -->
    <button
      class="tray-item"
      onclick={() => theme.toggle()}
      title={theme.isDark ? $t("desktop.switchToLight") : $t("desktop.switchToDark")}
    >
      <Icon icon={theme.isDark ? "mdi:weather-night" : "mdi:weather-sunny"} width="18" />
    </button>
    <!-- 通知面板按钮 -->
    <button
      class="tray-item"
      class:active={showNotifications}
      onclick={toggleNotifications}
      title={$t("desktop.notifications")}
    >
      <Icon icon="mdi:bell-outline" width="18" />
    </button>
    <!-- 系统监控 -->
    <button
      class="tray-item"
      class:active={showQuickSettings}
      onclick={toggleQuickSettings}
      title={$t("desktop.systemMonitor")}
    >
      <Icon icon="mdi:chart-line" width="18" />
    </button>
  </div>

  <!-- 时间 - 点击显示日历 -->
  <button class="clock" class:active={showCalendar} onclick={toggleCalendar}>
    <span class="time">{formatTime(time)}</span>
    <span class="date">{formatDate(time)}</span>
  </button>

  <!-- 显示桌面按钮 -->
  <button class="show-desktop" title={$t("desktop.showDesktop")} onclick={showDesktop}>
    <div class="line"></div>
  </button>
</footer>

<style>
  .taskbar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    height: 48px;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 0 4px;
    background: var(--taskbar-bg, rgba(24, 24, 28, 0.92));
    backdrop-filter: blur(20px);
    border-top: 1px solid var(--taskbar-border, rgba(255, 255, 255, 0.08));
    z-index: 9999;
    color: var(--taskbar-text, rgba(255, 255, 255, 0.9));

    :global([data-theme="light"]) & {
      background: var(--taskbar-bg, rgba(243, 243, 243, 0.92));
      border-top-color: var(--taskbar-border, rgba(0, 0, 0, 0.08));
      color: var(--taskbar-text, rgba(0, 0, 0, 0.85));
    }
  }

  .start-button {
    width: 44px;
    height: 40px;
    border: none;
    border-radius: 6px;
    background: transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    color: inherit;

    &:hover {
      background: var(--taskbar-hover, rgba(255, 255, 255, 0.1));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.06);
      }
    }

    &:active,
    &.active {
      background: rgba(255, 255, 255, 0.15);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.1);
      }
    }
  }

  .separator {
    width: 1px;
    height: 20px;
    background: rgba(255, 255, 255, 0.15);
    margin: 0 4px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.1);
    }
  }

  .pinned-apps {
    display: flex;
    gap: 2px;
  }

  .taskbar-item {
    height: 40px;
    border: none;
    border-radius: 6px;
    background: transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    color: inherit;
    position: relative;

    img {
      width: 22px;
      height: 22px;
      flex-shrink: 0;
    }

    &:hover {
      background: var(--taskbar-hover, rgba(255, 255, 255, 0.1));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.06);
      }
    }

    &:active {
      background: var(--taskbar-active, rgba(255, 255, 255, 0.05));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.08);
      }
    }
  }

  .app-item {
    width: 44px;
    justify-content: center;

    /* 方案B: 启动器样式，无指示器 */
    &.launcher {
      opacity: 0.85;

      &:hover {
        opacity: 1;
      }
    }
  }

  .window-item {
    min-width: 44px;
    max-width: 180px;
    padding: 0 10px;
    gap: 8px;

    .title {
      font-size: 12px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
      opacity: 0.9;
    }

    &.focused {
      background: var(--taskbar-hover, rgba(255, 255, 255, 0.12));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.08);
      }
    }

    &.minimized {
      opacity: 0.6;
    }
  }

  .window-list {
    display: flex;
    gap: 2px;
    overflow-x: auto;
    flex-shrink: 1;

    &::-webkit-scrollbar {
      display: none;
    }
  }

  .spacer {
    flex: 1;
  }

  /* 音乐播放器迷你控件 */
  .music-widget {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 10px;
    background: var(--taskbar-hover, rgba(255, 255, 255, 0.08));
    border-radius: 6px;
    margin-right: 4px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.05);
    }

    .music-controls {
      display: flex;
      align-items: center;
      gap: 2px;
    }

    .music-btn {
      width: 26px;
      height: 26px;
      border: none;
      border-radius: 50%;
      background: transparent;
      color: inherit;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;

      &:hover {
        background: rgba(255, 255, 255, 0.15);

        :global([data-theme="light"]) & {
          background: rgba(0, 0, 0, 0.08);
        }
      }

      &.play {
        width: 28px;
        height: 28px;
        background: var(--accent-color, #4a90d9);
        color: white;

        &:hover {
          background: var(--accent-hover, #357abd);
        }
      }
      
      &.mode-btn {
        color: #a78bfa;
        
        &:hover {
          background: rgba(139, 92, 246, 0.2);
        }
        
        &.active {
          background: rgba(139, 92, 246, 0.3);
        }
      }
      
      &.close-btn {
        color: #f87171;
        
        &:hover {
          background: rgba(248, 113, 113, 0.2);
        }
      }
    }
  }

  .system-tray {
    display: flex;
    gap: 2px;
    padding: 0 4px;
  }

  .tray-item {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 4px;
    background: transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    color: inherit;

    &:hover {
      background: var(--taskbar-hover, rgba(255, 255, 255, 0.1));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.06);
      }
    }

    &.active {
      background: var(--taskbar-active, rgba(255, 255, 255, 0.15));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.1);
      }
    }
  }

  .clock {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 0 12px;
    color: inherit;
    font-size: 12px;
    line-height: 1.3;
    cursor: pointer;
    border-radius: 6px;
    height: 40px;
    border: none;
    background: transparent;

    &:hover,
    &.active {
      background: var(--taskbar-hover, rgba(255, 255, 255, 0.1));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.06);
      }
    }

    .time {
      font-weight: 500;
    }

    .date {
      font-size: 11px;
      opacity: 0.8;
    }
  }

  .show-desktop {
    width: 8px;
    height: 100%;
    border: none;
    background: transparent;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    border-left: 1px solid var(--taskbar-border, rgba(255, 255, 255, 0.1));
    margin-left: 4px;

    :global([data-theme="light"]) & {
      border-left-color: rgba(0, 0, 0, 0.08);
    }

    .line {
      width: 3px;
      height: 20px;
      background: var(--taskbar-dot, rgba(255, 255, 255, 0.2));
      border-radius: 2px;

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.15);
      }
    }

    &:hover .line {
      background: var(--taskbar-indicator, rgba(255, 255, 255, 0.4));

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.3);
      }
    }
  }
</style>
