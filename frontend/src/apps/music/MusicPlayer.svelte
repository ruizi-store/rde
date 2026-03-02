<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import { musicPlayer, isAudioFile, type Track } from "$shared/stores/music-player.svelte";
  import { fileService, type FileInfo } from "$shared/services/files";
  import { windowManager } from "$desktop/stores/windows.svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import { musicSettings } from "./settings.svelte";
  import MusicSetup from "./MusicSetup.svelte";
  import MusicSettings from "./MusicSettings.svelte";
  import LyricsPanel from "./LyricsPanel.svelte";

  interface Props {
    windowId: string;
    filePath?: string;  // 从文件管理器传入的文件路径
  }

  let { windowId, filePath }: Props = $props();

  // 视图状态
  let showSetup = $state(false);
  let showSettings = $state(false);
  let initialized = $state(false);
  
  // 左侧面板视图: "files" | "lyrics"
  let leftPanelView = $state<"files" | "lyrics">("files");

  // 文件浏览状态
  let currentPath = $state("/");
  let files = $state<FileInfo[]>([]);
  let loading = $state(false);
  let error = $state("");

  // UI 状态
  let showPlaylist = $state(true);
  let isDraggingProgress = $state(false);
  let dragProgress = $state(0);

  // 进度条
  let progressBar: HTMLDivElement | null = null;

  // 过滤后的音频文件
  let audioFiles = $derived(files.filter((f) => !f.is_dir && isAudioFile(f.name)));

  // 目录列表
  let directories = $derived(files.filter((f) => f.is_dir));

  // 面包屑
  let breadcrumbs = $derived.by(() => {
    const parts = currentPath.split("/").filter(Boolean);
    return [
      { name: $t("music.rootDirectory"), path: "/" },
      ...parts.map((part, i) => ({
        name: part,
        path: "/" + parts.slice(0, i + 1).join("/"),
      })),
    ];
  });

  onMount(async () => {
    // 如果传入了文件路径，直接播放
    if (filePath) {
      const pathParts = filePath.split("/");
      const fileName = pathParts.pop() || "";
      const dirPath = pathParts.join("/") || "/";

      currentPath = dirPath;
      await loadDirectory(dirPath);

      const file = files.find(f => f.path === filePath);
      if (file) {
        playFile(file);
      }
      initialized = true;
      return;
    }

    // 检查是否需要显示设置向导
    if (!musicSettings.settings.setupComplete) {
      showSetup = true;
    } else {
      // 使用默认音乐文件夹
      currentPath = musicSettings.settings.defaultMusicPath || "/";
      await loadDirectory(currentPath);
    }
    initialized = true;
  });

  // 设置完成回调
  function handleSetupComplete() {
    showSetup = false;
    currentPath = musicSettings.settings.defaultMusicPath || "/";
    loadDirectory(currentPath);
  }

  // 跳过设置回调
  function handleSetupSkip() {
    showSetup = false;
    currentPath = musicSettings.settings.defaultMusicPath || "/";
    loadDirectory(currentPath);
  }

  async function loadDirectory(path: string) {
    loading = true;
    error = "";

    try {
      const result = await fileService.list(path, false);
      files = result.data?.content || [];
      currentPath = path;
    } catch (e) {
      error = e instanceof Error ? e.message : $t("music.loadFailed");
      files = [];
    } finally {
      loading = false;
    }
  }

  function navigateTo(path: string) {
    loadDirectory(path);
  }

  function goUp() {
    const parentPath = currentPath.split("/").slice(0, -1).join("/") || "/";
    loadDirectory(parentPath);
  }

  // 播放单个文件
  function playFile(file: FileInfo) {
    const track = musicPlayer.createTrackFromPath(
      currentPath === "/" ? `/${file.name}` : `${currentPath}/${file.name}`,
      file.name
    );
    
    // 创建当前目录所有音频文件的播放列表
    const tracks: Track[] = audioFiles.map((f) =>
      musicPlayer.createTrackFromPath(
        currentPath === "/" ? `/${f.name}` : `${currentPath}/${f.name}`,
        f.name
      )
    );
    
    const startIndex = audioFiles.findIndex((f) => f.name === file.name);
    musicPlayer.playNewList(tracks, startIndex >= 0 ? startIndex : 0);
  }

  // 添加到播放列表
  function addToPlaylist(file: FileInfo) {
    const track = musicPlayer.createTrackFromPath(
      currentPath === "/" ? `/${file.name}` : `${currentPath}/${file.name}`,
      file.name
    );
    musicPlayer.addTrack(track);
  }

  // 添加目录下所有音频文件
  function addAllToPlaylist() {
    const tracks: Track[] = audioFiles.map((f) =>
      musicPlayer.createTrackFromPath(
        currentPath === "/" ? `/${f.name}` : `${currentPath}/${f.name}`,
        f.name
      )
    );
    musicPlayer.addTracks(tracks);
  }

  // 播放目录下所有音频文件
  function playAll() {
    if (audioFiles.length === 0) return;
    
    const tracks: Track[] = audioFiles.map((f) =>
      musicPlayer.createTrackFromPath(
        currentPath === "/" ? `/${f.name}` : `${currentPath}/${f.name}`,
        f.name
      )
    );
    musicPlayer.playNewList(tracks, 0);
  }

  // 进度条拖拽
  function handleProgressMouseDown(e: MouseEvent) {
    if (!progressBar) return;
    
    // 立即计算并跳转到点击位置
    const rect = progressBar.getBoundingClientRect();
    const percent = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    const seekTime = percent * musicPlayer.duration;
    
    // 立即跳转
    musicPlayer.seek(seekTime);
    
    // 开始拖拽跟踪
    isDraggingProgress = true;
    dragProgress = seekTime;
    
    const handleMouseMove = (e: MouseEvent) => {
      if (!progressBar) return;
      const rect = progressBar.getBoundingClientRect();
      const percent = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      dragProgress = percent * musicPlayer.duration;
      musicPlayer.seek(dragProgress);
    };
    
    const handleMouseUp = () => {
      isDraggingProgress = false;
      document.removeEventListener("mousemove", handleMouseMove);
      document.removeEventListener("mouseup", handleMouseUp);
    };
    
    document.addEventListener("mousemove", handleMouseMove);
    document.addEventListener("mouseup", handleMouseUp);
  }

  function updateDragProgress(e: MouseEvent) {
    if (!progressBar) return;
    const rect = progressBar.getBoundingClientRect();
    const percent = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
    dragProgress = percent * musicPlayer.duration;
  }

  // 格式化文件大小
  function formatSize(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }

  // 获取播放进度百分比
  function getProgressPercent(): number {
    if (isDraggingProgress) {
      return musicPlayer.duration > 0 ? (dragProgress / musicPlayer.duration) * 100 : 0;
    }
    return musicPlayer.duration > 0 ? (musicPlayer.currentTime / musicPlayer.duration) * 100 : 0;
  }

  // 获取当前显示时间
  function getCurrentDisplayTime(): string {
    if (isDraggingProgress) {
      return musicPlayer.formatTime(dragProgress);
    }
    return musicPlayer.formatTime(musicPlayer.currentTime);
  }
</script>

{#if !initialized}
  <div class="loading-container">
    <Spinner size="lg" />
    <span>{$t("music.loading")}</span>
  </div>
{:else if showSetup}
  <MusicSetup onComplete={handleSetupComplete} onSkip={handleSetupSkip} />
{:else if showSettings}
  <MusicSettings onClose={() => showSettings = false} />
{:else}
  <div class="music-player">
    <!-- 左侧面板 -->
    <div class="left-panel" class:collapsed={!showPlaylist}>
      <!-- 面板标签 -->
      <div class="panel-tabs">
        <button
          class="tab"
          class:active={leftPanelView === "files"}
          onclick={() => leftPanelView = "files"}
        >
          <Icon icon="mdi:folder-music" width="16" />
          <span>{$t("music.files")}</span>
        </button>
        <button
          class="tab"
          class:active={leftPanelView === "lyrics"}
          onclick={() => leftPanelView = "lyrics"}
        >
          <Icon icon="mdi:text" width="16" />
          <span>{$t("music.lyrics")}</span>
        </button>
        <div class="tab-spacer"></div>
        <button class="icon-btn" onclick={() => showSettings = true} title={$t("music.settings.title")}>
          <Icon icon="mdi:cog" width="18" />
        </button>
      </div>
      
      {#if leftPanelView === "files"}
      <!-- 文件浏览器 -->
      <div class="file-browser">
        <!-- 工具栏 -->
        <div class="browser-toolbar">
          <button class="icon-btn" onclick={goUp} disabled={currentPath === "/"} title={$t("music.goUp")}>
            <Icon icon="mdi:arrow-up" width="18" />
          </button>
          <div class="breadcrumbs">
            {#each breadcrumbs as crumb, i}
              {#if i > 0}<span class="sep">/</span>{/if}
              <button class="crumb" onclick={() => navigateTo(crumb.path)}>{crumb.name}</button>
            {/each}
          </div>
          <button class="icon-btn" onclick={() => loadDirectory(currentPath)} title={$t("music.refresh")}>
            <Icon icon="mdi:refresh" width="18" />
          </button>
        </div>

    <!-- 文件列表 -->
    <div class="file-list">
      {#if loading}
        <div class="loading">
          <Icon icon="mdi:loading" width="24" class="spin" />
          <span>{$t("music.loadingText")}</span>
        </div>
      {:else if error}
        <div class="error">
          <Icon icon="mdi:alert-circle" width="24" />
          <span>{error}</span>
        </div>
      {:else}
        <!-- 目录 -->
        {#each directories as dir}
          <button class="file-item folder" ondblclick={() => navigateTo(currentPath === "/" ? `/${dir.name}` : `${currentPath}/${dir.name}`)}>
            <Icon icon="mdi:folder" width="20" class="folder-icon" />
            <span class="name">{dir.name}</span>
          </button>
        {/each}

        <!-- 音频文件 -->
        {#each audioFiles as file}
          <div
            class="file-item audio"
            class:playing={musicPlayer.currentTrack?.path === (currentPath === "/" ? `/${file.name}` : `${currentPath}/${file.name}`)}
            ondblclick={() => playFile(file)}
            role="button"
            tabindex="0"
          >
            <Icon 
              icon={musicPlayer.currentTrack?.path === (currentPath === "/" ? `/${file.name}` : `${currentPath}/${file.name}`) && musicPlayer.isPlaying ? "mdi:music-note" : "mdi:file-music"} 
              width="20" 
              class="music-icon"
            />
            <span class="name">{file.name}</span>
            <span class="size">{formatSize(file.size || 0)}</span>
            <button class="add-btn" onclick={(e) => { e.stopPropagation(); addToPlaylist(file); }} title={$t("music.addToPlaylist")}>
              <Icon icon="mdi:plus" width="16" />
            </button>
          </div>
        {/each}

        {#if directories.length === 0 && audioFiles.length === 0}
          <div class="empty">
            <Icon icon="mdi:music-off" width="48" />
            <span>{$t("music.noAudioFiles")}</span>
          </div>
        {/if}
      {/if}
    </div>

    <!-- 操作按钮 -->
    {#if audioFiles.length > 0}
      <div class="browser-actions">
        <Button variant="primary" size="sm" onclick={playAll}>
          <Icon icon="mdi:play" width="16" />
          {$t("music.playAll")} ({audioFiles.length})
        </Button>
        <Button variant="ghost" size="sm" onclick={addAllToPlaylist}>
          <Icon icon="mdi:playlist-plus" width="16" />
          {$t("music.addAll")}
        </Button>
      </div>
    {/if}
      </div>
      {:else}
      <!-- 歌词面板 -->
      <div class="lyrics-view">
        <LyricsPanel />
      </div>
      {/if}
    </div>

  <!-- 右侧：播放列表 -->
  <div class="playlist-panel" class:hidden={!showPlaylist}>
    <div class="playlist-header">
      <h3>{$t("music.playlist")}</h3>
      <span class="count">{musicPlayer.playlist.length} {$t("music.tracks")}</span>
      {#if musicPlayer.playlist.length > 0}
        <button class="icon-btn" onclick={() => musicPlayer.clearPlaylist()} title={$t("music.clear")}>
          <Icon icon="mdi:delete-outline" width="18" />
        </button>
      {/if}
    </div>

    <div class="playlist-items">
      {#each musicPlayer.playlist as track, i}
        <div
          class="playlist-item"
          class:active={i === musicPlayer.currentIndex}
          ondblclick={() => musicPlayer.playTrack(i)}
          role="button"
          tabindex="0"
        >
          <span class="index">{i + 1}</span>
          {#if i === musicPlayer.currentIndex && musicPlayer.isPlaying}
            <Icon icon="mdi:volume-high" width="16" class="playing-icon" />
          {:else}
            <Icon icon="mdi:music-note" width="16" class="note-icon" />
          {/if}
          <span class="title">{track.title || track.name.replace(/\.[^.]+$/, "")}</span>
          <button class="remove-btn" onclick={(e) => { e.stopPropagation(); musicPlayer.removeTrack(i); }} title={$t("music.remove")}>
            <Icon icon="mdi:close" width="14" />
          </button>
        </div>
      {/each}

      {#if musicPlayer.playlist.length === 0}
        <div class="empty-playlist">
          <Icon icon="mdi:playlist-music" width="48" />
          <span>{$t("music.playlistEmpty")}</span>
          <span class="hint">{$t("music.doubleClickToPlay")}</span>
        </div>
      {/if}
    </div>
  </div>

  <!-- 底部：播放控制 -->
  <div class="player-controls">
    <!-- 切换播放列表按钮 -->
    <button class="toggle-playlist" onclick={() => showPlaylist = !showPlaylist} title={showPlaylist ? $t("music.hidePlaylist") : $t("music.showPlaylist")}>
      <Icon icon={showPlaylist ? "mdi:playlist-music" : "mdi:playlist-music-outline"} width="20" />
    </button>

    <!-- 当前曲目信息 -->
    <div class="now-playing">
      <div class="track-info">
        {#if musicPlayer.currentTrack}
          <span class="title">{musicPlayer.displayTitle}</span>
          <span class="artist">{musicPlayer.displayArtist}</span>
        {:else}
          <span class="title">{$t("music.notPlaying")}</span>
          <span class="artist">{$t("music.selectToPlay")}</span>
        {/if}
      </div>
    </div>

    <!-- 播放按钮 -->
    <div class="main-controls">
      <button class="control-btn" onclick={() => musicPlayer.prev()} disabled={musicPlayer.playlist.length === 0} title={$t("music.prevTrack")}>
        <Icon icon="mdi:skip-previous" width="28" />
      </button>
      <button class="control-btn play-btn" onclick={() => musicPlayer.toggle()} disabled={musicPlayer.playlist.length === 0} title={musicPlayer.isPlaying ? $t("music.pause") : $t("music.play")}>
        <Icon icon={musicPlayer.isPlaying ? "mdi:pause" : "mdi:play"} width="32" />
      </button>
      <button class="control-btn" onclick={() => musicPlayer.next()} disabled={musicPlayer.playlist.length === 0} title={$t("music.nextTrack")}>
        <Icon icon="mdi:skip-next" width="28" />
      </button>
    </div>

    <!-- 进度条 -->
    <div class="progress-section">
      <span class="time">{getCurrentDisplayTime()}</span>
      <div
        class="progress-bar"
        bind:this={progressBar}
        onmousedown={handleProgressMouseDown}
        role="slider"
        tabindex="0"
        aria-valuenow={musicPlayer.currentTime}
        aria-valuemin={0}
        aria-valuemax={musicPlayer.duration}
      >
        <div class="progress-track">
          <div class="progress-fill" style="width: {getProgressPercent()}%"></div>
          <div class="progress-thumb" style="left: {getProgressPercent()}%"></div>
        </div>
      </div>
      <span class="time">{musicPlayer.formatTime(musicPlayer.duration)}</span>
    </div>

    <!-- 附加控制 -->
    <div class="extra-controls">
      <button class="control-btn small" onclick={() => musicPlayer.togglePlayMode()} title={musicPlayer.getPlayModeName()}>
        <Icon icon={musicPlayer.getPlayModeIcon()} width="20" />
      </button>
      <button class="control-btn small" onclick={() => musicPlayer.toggleMute()} title={musicPlayer.isMuted ? $t("music.unmute") : $t("music.mute")}>
        <Icon icon={musicPlayer.isMuted || musicPlayer.volume === 0 ? "mdi:volume-off" : musicPlayer.volume < 0.5 ? "mdi:volume-medium" : "mdi:volume-high"} width="20" />
      </button>
      <input
        type="range"
        class="volume-slider"
        min="0"
        max="1"
        step="0.01"
        value={musicPlayer.volume}
        style="--volume-percent: {musicPlayer.volume * 100}%"
        oninput={(e) => musicPlayer.setVolume(parseFloat((e.target as HTMLInputElement).value))}
      />
    </div>
  </div>
</div>
{/if}

<style>
  .loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    height: 100%;
    gap: 16px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.6));
    background: var(--bg-primary, #1a1a2e);

    :global([data-theme="light"]) & {
      background: #f8f9fa;
      color: #666;
    }
  }

  .music-player {
    display: grid;
    grid-template-columns: 1fr 280px;
    grid-template-rows: 1fr auto;
    height: 100%;
    background: var(--bg-primary, #1a1a1e);
    color: var(--text-primary, #fff);
    overflow: hidden;

    :global([data-theme="light"]) & {
      background: var(--bg-primary, #f5f5f5);
      color: var(--text-primary, #333);
    }
  }

  /* 左侧面板 */
  .left-panel {
    display: flex;
    flex-direction: column;
    border-right: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    overflow: hidden;

    &.collapsed {
      grid-column: span 2;
      border-right: none;
    }

    :global([data-theme="light"]) & {
      border-right-color: rgba(0, 0, 0, 0.1);
    }
  }

  /* 面板标签 */
  .panel-tabs {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.1);
    }
  }

  .tab {
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 6px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: var(--text-secondary, rgba(255, 255, 255, 0.6));
    font-size: 13px;
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      background: rgba(255, 255, 255, 0.05);
      color: var(--text-primary, #fff);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.05);
        color: var(--text-primary, #333);
      }
    }

    &.active {
      background: var(--accent-color, #4a90d9);
      color: white;
    }
  }

  .tab-spacer {
    flex: 1;
  }

  /* 歌词视图 */
  .lyrics-view {
    flex: 1;
    overflow: hidden;
  }

  .browser-toolbar {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    background: var(--bg-secondary, rgba(255, 255, 255, 0.03));

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.02);
      border-bottom-color: rgba(0, 0, 0, 0.1);
    }
  }

  .breadcrumbs {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    font-size: 13px;

    .sep {
      opacity: 0.5;
    }

    .crumb {
      background: none;
      border: none;
      color: inherit;
      cursor: pointer;
      padding: 2px 4px;
      border-radius: 4px;
      white-space: nowrap;

      &:hover {
        background: rgba(255, 255, 255, 0.1);

        :global([data-theme="light"]) & {
          background: rgba(0, 0, 0, 0.05);
        }
      }
    }
  }

  .icon-btn {
    width: 28px;
    height: 28px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.1);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.05);
      }
    }

    &:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
  }

  .file-list {
    flex: 1;
    overflow-y: auto;
    padding: 4px;
  }

  .file-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px 12px;
    border: none;
    border-radius: 6px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;

    &:hover {
      background: rgba(255, 255, 255, 0.08);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.04);
      }

      .add-btn {
        opacity: 1;
      }
    }

    &.playing {
      background: var(--accent-color, #4a90d9);
      color: white;

      :global(.music-icon) {
        animation: pulse 1s ease-in-out infinite;
      }
    }

    .name {
      flex: 1;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .size {
      font-size: 12px;
      opacity: 0.6;
    }

    .add-btn {
      opacity: 0;
      width: 24px;
      height: 24px;
      border: none;
      border-radius: 4px;
      background: rgba(255, 255, 255, 0.15);
      color: inherit;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;

      &:hover {
        background: rgba(255, 255, 255, 0.25);
      }
    }
  }

  :global(.folder-icon) {
    color: #ffc107;
  }

  :global(.music-icon) {
    color: var(--accent-color, #4a90d9);
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.5; }
  }

  .loading, .error, .empty {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px;
    opacity: 0.6;
  }

  .error {
    color: #f44336;
  }

  :global(.spin) {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }

  .browser-actions {
    display: flex;
    gap: 8px;
    padding: 8px 12px;
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    :global([data-theme="light"]) & {
      border-top-color: rgba(0, 0, 0, 0.1);
    }
  }

  /* 播放列表 */
  .playlist-panel {
    display: flex;
    flex-direction: column;
    background: var(--bg-secondary, rgba(0, 0, 0, 0.2));
    overflow: hidden;

    &.hidden {
      display: none;
    }

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.02);
    }
  }

  .playlist-header {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 12px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    h3 {
      margin: 0;
      font-size: 14px;
      font-weight: 500;
    }

    .count {
      flex: 1;
      font-size: 12px;
      opacity: 0.6;
    }

    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.1);
    }
  }

  .playlist-items {
    flex: 1;
    overflow-y: auto;
    padding: 4px;
  }

  .playlist-item {
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 8px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    text-align: left;

    &:hover {
      background: rgba(255, 255, 255, 0.08);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.04);
      }

      .remove-btn {
        opacity: 1;
      }
    }

    &.active {
      background: var(--accent-color, #4a90d9);
      color: white;
    }

    .index {
      width: 20px;
      font-size: 11px;
      opacity: 0.6;
      text-align: center;
    }

    :global(.playing-icon) {
      color: white;
      animation: pulse 1s ease-in-out infinite;
    }

    :global(.note-icon) {
      opacity: 0.5;
    }

    .title {
      flex: 1;
      font-size: 13px;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .remove-btn {
      opacity: 0;
      width: 20px;
      height: 20px;
      border: none;
      border-radius: 3px;
      background: transparent;
      color: inherit;
      cursor: pointer;
      display: flex;
      align-items: center;
      justify-content: center;

      &:hover {
        background: rgba(255, 255, 255, 0.2);
      }
    }
  }

  .empty-playlist {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 40px 20px;
    opacity: 0.5;
    text-align: center;

    .hint {
      font-size: 12px;
    }
  }

  /* 播放控制 */
  .player-controls {
    grid-column: span 2;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    background: var(--bg-tertiary, rgba(0, 0, 0, 0.3));
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    :global([data-theme="light"]) & {
      background: rgba(255, 255, 255, 0.8);
      border-top-color: rgba(0, 0, 0, 0.1);
    }
  }

  .toggle-playlist {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: rgba(255, 255, 255, 0.1);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.05);
      }
    }
  }

  .now-playing {
    width: 180px;
    overflow: hidden;

    .track-info {
      display: flex;
      flex-direction: column;
      gap: 2px;
    }

    .title {
      font-size: 13px;
      font-weight: 500;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }

    .artist {
      font-size: 11px;
      opacity: 0.6;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
  }

  .main-controls {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .control-btn {
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 50%;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.1);

      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.05);
      }
    }

    &:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }

    &.play-btn {
      width: 48px;
      height: 48px;
      background: var(--accent-color, #4a90d9);
      color: white;

      &:hover:not(:disabled) {
        background: var(--accent-hover, #357abd);
      }
    }

    &.small {
      width: 32px;
      height: 32px;
    }
  }

  .progress-section {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 8px;

    .time {
      font-size: 11px;
      opacity: 0.7;
      min-width: 36px;
      text-align: center;
    }
  }

  .progress-bar {
    flex: 1;
    height: 20px;
    cursor: pointer;
    display: flex;
    align-items: center;
  }

  .progress-track {
    position: relative;
    width: 100%;
    height: 4px;
    background: rgba(255, 255, 255, 0.2);
    border-radius: 2px;

    :global([data-theme="light"]) & {
      background: rgba(0, 0, 0, 0.15);
    }

    .progress-fill {
      height: 100%;
      background: var(--accent-color, #4a90d9);
      border-radius: 2px;
      transition: width 0.1s ease;
    }

    .progress-thumb {
      position: absolute;
      top: 50%;
      width: 12px;
      height: 12px;
      background: white;
      border-radius: 50%;
      transform: translate(-50%, -50%);
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
      opacity: 0;
      transition: opacity 0.2s;
    }

    &:hover .progress-thumb,
    .progress-bar:focus-within & .progress-thumb {
      opacity: 1;
    }
  }

  .extra-controls {
    display: flex;
    align-items: center;
    gap: 4px;
  }

  .volume-slider {
    width: 80px;
    height: 6px;
    -webkit-appearance: none;
    appearance: none;
    background: linear-gradient(to right, var(--accent-color, #4a90d9) var(--volume-percent, 100%), rgba(255, 255, 255, 0.3) var(--volume-percent, 100%));
    border-radius: 3px;
    cursor: pointer;

    :global([data-theme="light"]) & {
      background: linear-gradient(to right, var(--accent-color, #4a90d9) var(--volume-percent, 100%), rgba(0, 0, 0, 0.2) var(--volume-percent, 100%));
    }

    &::-webkit-slider-thumb {
      -webkit-appearance: none;
      width: 14px;
      height: 14px;
      background: white;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
      border: 2px solid var(--accent-color, #4a90d9);
    }

    &::-moz-range-thumb {
      width: 14px;
      height: 14px;
      background: white;
      border-radius: 50%;
      cursor: pointer;
      box-shadow: 0 1px 4px rgba(0, 0, 0, 0.3);
      border: 2px solid var(--accent-color, #4a90d9);
    }

    &::-webkit-slider-runnable-track {
      height: 6px;
      border-radius: 3px;
    }

    &::-moz-range-track {
      height: 6px;
      border-radius: 3px;
    }
  }
</style>
