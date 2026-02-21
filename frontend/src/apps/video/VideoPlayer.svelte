<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "svelte-i18n";
  import { fileService, type FileInfo } from "$shared/services/files";
  import { windowManager } from "$desktop/stores/windows.svelte";
  import Icon from "@iconify/svelte";
  import { Button, Spinner } from "$shared/ui";
  import LazyThumbnail from "$shared/components/LazyThumbnail.svelte";
  import { videoSettings, isVideoFile, needsTranscode } from "./settings.svelte";
  import VideoSetup from "./VideoSetup.svelte";
  import VideoSettings from "./VideoSettings.svelte";

  // 字幕信息接口
  interface SubtitleInfo {
    path: string;
    language: string;
    label: string;
    embedded: boolean;
    index?: number;
  }

  // 视频信息接口
  interface VideoInfoData {
    path: string;
    name: string;
    size: number;
    duration: number;
    width: number;
    height: number;
    codec: string;
    bitrate: number;
    fps: number;
    needs_transcode: boolean;
  }

  interface Props {
    windowId: string;
    filePath?: string;  // 从文件管理器传入的文件路径
  }

  let { windowId, filePath }: Props = $props();

  // 视图状态
  let showSetup = $state(false);
  let showSettings = $state(false);
  let initialized = $state(false);
  let view = $state<"browser" | "player">("browser");

  // 文件浏览状态
  let currentPath = $state("/");
  let files = $state<FileInfo[]>([]);
  let loading = $state(false);
  let error = $state("");

  // 播放器状态
  let videoElement: HTMLVideoElement | null = null;
  let currentVideo = $state<FileInfo | null>(null);
  let isPlaying = $state(false);
  let currentTime = $state(0);
  let duration = $state(0);
  let volume = $state(1);
  let isMuted = $state(false);
  let playbackRate = $state(1);
  let isFullscreen = $state(false);
  let showControls = $state(true);
  let controlsTimeout: ReturnType<typeof setTimeout> | null = null;
  let isDraggingProgress = $state(false);
  let dragProgress = $state(0);
  let videoError = $state("");
  let buffered = $state(0);

  // 新增功能状态
  let showVolumeSlider = $state(false);
  let showSpeedMenu = $state(false);
  let showSubtitleMenu = $state(false);
  let showVideoInfo = $state(false);
  let subtitles = $state<SubtitleInfo[]>([]);
  let currentSubtitle = $state<SubtitleInfo | null>(null);
  let subtitleText = $state("");
  let videoInfo = $state<VideoInfoData | null>(null);
  let isTranscoding = $state(false);
  let isPiP = $state(false);

  // 播放列表（当前目录的视频文件）
  let playlist = $derived(files.filter((f) => !f.is_dir && isVideoFile(f.name)));
  let currentIndex = $derived(
    currentVideo ? playlist.findIndex((f) => f.path === currentVideo.path) : -1
  );

  // 目录列表
  let directories = $derived(files.filter((f) => f.is_dir));

  // 面包屑
  let breadcrumbs = $derived.by(() => {
    const parts = currentPath.split("/").filter(Boolean);
    return [
      { name: $t("video.player.rootDirectory"), path: "/" },
      ...parts.map((part, i) => ({
        name: part,
        path: "/" + parts.slice(0, i + 1).join("/"),
      })),
    ];
  });

  // 格式化时间
  function formatTime(seconds: number): string {
    if (!isFinite(seconds)) return "0:00";
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) {
      return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
    }
    return `${m}:${s.toString().padStart(2, "0")}`;
  }

  // 获取文件名（不含扩展名）
  function getDisplayName(name: string): string {
    if (videoSettings.settings.showFileExtension) return name;
    const lastDot = name.lastIndexOf(".");
    return lastDot > 0 ? name.substring(0, lastDot) : name;
  }

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
    if (!videoSettings.settings.setupComplete) {
      showSetup = true;
    } else {
      currentPath = videoSettings.settings.defaultVideoPath || "/";
      await loadDirectory(currentPath);
    }
    initialized = true;
  });

  onDestroy(() => {
    // 保存播放进度
    saveProgress();
    if (controlsTimeout) {
      clearTimeout(controlsTimeout);
    }
    // 移除 PiP 事件监听
    if (videoElement) {
      videoElement.removeEventListener("leavepictureinpicture", handleLeavePiP);
    }
  });

  // PiP 退出事件处理
  function handleLeavePiP() {
    isPiP = false;
  }

  function handleSetupComplete() {
    showSetup = false;
    currentPath = videoSettings.settings.defaultVideoPath || "/";
    loadDirectory(currentPath);
  }

  function handleSetupSkip() {
    showSetup = false;
    currentPath = videoSettings.settings.defaultVideoPath || "/";
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
      error = e instanceof Error ? e.message : $t("video.player.loadFailed");
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

  // 播放视频文件
  function playFile(file: FileInfo) {
    currentVideo = file;
    view = "player";
    videoError = "";
    playbackRate = videoSettings.settings.defaultPlaybackRate;
    
    // 恢复播放位置
    const savedPosition = videoSettings.getPosition(file.path);
    
    // 获取认证 token
    const token = localStorage.getItem("auth_token") || "";
    
    // 等待 DOM 更新后设置视频源
    setTimeout(() => {
      if (videoElement) {
        // 根据是否需要转码选择不同的 URL
        const videoUrl = needsTranscode(file.name)
          ? `/api/v1/video/hls/playlist?path=${encodeURIComponent(file.path)}&token=${token}`
          : `/api/v1/video/stream?path=${encodeURIComponent(file.path)}&token=${token}`;
        
        // 对于需要转码的，使用 HLS.js
        if (needsTranscode(file.name)) {
          loadHLS(videoUrl, savedPosition);
        } else {
          videoElement.src = videoUrl;
          videoElement.currentTime = savedPosition;
          videoElement.playbackRate = playbackRate;
          videoElement.play().catch(e => {
            console.error("Playback failed:", e);
            videoError = $t("video.player.playFailed") + ": " + e.message;
          });
        }
      }
    }, 50);
  }

  // 加载 HLS 流
  async function loadHLS(url: string, startPosition: number) {
    if (!videoElement) return;
    
    // 动态导入 hls.js
    try {
      const Hls = (await import("hls.js")).default;
      
      if (Hls.isSupported()) {
        const hls = new Hls({
          startPosition: startPosition,
        });
        hls.loadSource(url);
        hls.attachMedia(videoElement);
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          videoElement!.playbackRate = playbackRate;
          videoElement!.play().catch(e => {
            console.error("HLS playback failed:", e);
            videoError = $t("video.player.playFailed") + ": " + e.message;
          });
        });
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (data.fatal) {
            videoError = $t("video.player.videoLoadFailed");
            console.error("HLS error:", data);
          }
        });
      } else if (videoElement.canPlayType("application/vnd.apple.mpegurl")) {
        // Safari 原生支持 HLS
        videoElement.src = url;
        videoElement.currentTime = startPosition;
        videoElement.playbackRate = playbackRate;
        videoElement.play();
      } else {
        videoError = $t("video.player.hlsNotSupported");
      }
    } catch (e) {
      console.error("Failed to load HLS.js:", e);
      // 尝试直接播放
      const fallbackToken = localStorage.getItem("auth_token") || "";
      videoElement.src = `/api/v1/video/stream?path=${encodeURIComponent(currentVideo!.path)}&token=${fallbackToken}`;
      videoElement.currentTime = startPosition;
      videoElement.play().catch(() => {
        videoError = $t("video.player.tryOtherFormat");
      });
    }
  }

  // 保存播放进度
  function saveProgress() {
    if (currentVideo && videoElement && duration > 0) {
      videoSettings.updatePosition(
        currentVideo.path,
        currentVideo.name,
        currentTime,
        duration
      );
    }
  }

  // 返回文件列表
  function backToBrowser() {
    saveProgress();
    view = "browser";
    currentVideo = null;
    if (videoElement) {
      videoElement.pause();
      videoElement.src = "";
    }
  }

  // 播放控制
  function togglePlay() {
    if (!videoElement) return;
    if (isPlaying) {
      videoElement.pause();
    } else {
      videoElement.play();
    }
  }

  function seek(seconds: number) {
    if (!videoElement) return;
    videoElement.currentTime = Math.max(0, Math.min(duration, currentTime + seconds));
  }

  function setPlaybackRate(rate: number) {
    playbackRate = rate;
    if (videoElement) {
      videoElement.playbackRate = rate;
    }
  }

  function toggleMute() {
    if (!videoElement) return;
    isMuted = !isMuted;
    videoElement.muted = isMuted;
  }

  function setVolume(v: number) {
    volume = Math.max(0, Math.min(1, v));
    if (videoElement) {
      videoElement.volume = volume;
    }
  }

  function toggleFullscreen() {
    const container = document.querySelector(`[data-window-id="${windowId}"]`);
    if (!container) return;

    if (!document.fullscreenElement) {
      container.requestFullscreen();
      isFullscreen = true;
    } else {
      document.exitFullscreen();
      isFullscreen = false;
    }
  }

  // 播放上一个/下一个
  function playPrevious() {
    if (currentIndex > 0) {
      playFile(playlist[currentIndex - 1]);
    }
  }

  function playNext() {
    if (currentIndex < playlist.length - 1) {
      playFile(playlist[currentIndex + 1]);
    }
  }

  // 视频事件处理
  function handleTimeUpdate() {
    if (!videoElement || isDraggingProgress) return;
    currentTime = videoElement.currentTime;
    
    // 更新缓冲进度
    if (videoElement.buffered.length > 0) {
      buffered = videoElement.buffered.end(videoElement.buffered.length - 1);
    }
  }

  function handleLoadedMetadata() {
    if (!videoElement) return;
    duration = videoElement.duration;
    volume = videoElement.volume;
    
    // 加载字幕和视频信息
    loadSubtitles();
    loadVideoInfo();
    
    // 监听 PiP 退出事件
    videoElement.addEventListener("leavepictureinpicture", handleLeavePiP);
  }

  function handlePlay() {
    isPlaying = true;
  }

  function handlePause() {
    isPlaying = false;
    saveProgress();
  }

  function handleEnded() {
    isPlaying = false;
    saveProgress();
    
    // 根据循环模式处理
    const loopMode = videoSettings.settings.loopMode;
    
    if (loopMode === "single") {
      // 单曲循环
      if (videoElement) {
        videoElement.currentTime = 0;
        videoElement.play();
      }
    } else if (loopMode === "list" || videoSettings.settings.autoPlayNext) {
      // 列表循环或自动播放下一个
      if (currentIndex < playlist.length - 1) {
        playNext();
      } else if (loopMode === "list" && playlist.length > 0) {
        // 列表循环到第一个
        playFile(playlist[0]);
      }
    }
  }

  function handleError() {
    videoError = $t("video.player.loadFailed");
  }

  // 进度条拖动
  function handleProgressMouseDown(e: MouseEvent) {
    isDraggingProgress = true;
    updateDragProgress(e);
  }

  function handleProgressMouseMove(e: MouseEvent) {
    if (!isDraggingProgress) return;
    updateDragProgress(e);
  }

  function handleProgressMouseUp() {
    if (!isDraggingProgress || !videoElement) return;
    videoElement.currentTime = dragProgress * duration;
    isDraggingProgress = false;
  }

  function updateDragProgress(e: MouseEvent) {
    const target = e.currentTarget as HTMLElement;
    const rect = target.getBoundingClientRect();
    const x = Math.max(0, Math.min(rect.width, e.clientX - rect.left));
    dragProgress = x / rect.width;
  }

  // 显示/隐藏控制栏
  function handleMouseMove() {
    showControls = true;
    if (controlsTimeout) {
      clearTimeout(controlsTimeout);
    }
    if (isPlaying) {
      controlsTimeout = setTimeout(() => {
        showControls = false;
      }, 3000);
    }
  }

  // 键盘快捷键
  function handleKeyDown(e: KeyboardEvent) {
    if (view !== "player" || !videoElement) return;
    
    switch (e.code) {
      case "Space":
        e.preventDefault();
        togglePlay();
        break;
      case "ArrowLeft":
        seek(-5);
        break;
      case "ArrowRight":
        seek(5);
        break;
      case "ArrowUp":
        e.preventDefault();
        setVolume(volume + 0.05);
        break;
      case "ArrowDown":
        e.preventDefault();
        setVolume(volume - 0.05);
        break;
      case "KeyM":
        toggleMute();
        break;
      case "KeyF":
        toggleFullscreen();
        break;
      case "Escape":
        if (isFullscreen) {
          toggleFullscreen();
        } else {
          backToBrowser();
        }
        break;
      case "BracketLeft":
        setPlaybackRate(Math.max(0.25, playbackRate - 0.25));
        break;
      case "BracketRight":
        setPlaybackRate(Math.min(2, playbackRate + 0.25));
        break;
    }
  }

  // 播放速度选项
  const playbackRates = [0.5, 0.75, 1, 1.25, 1.5, 2];

  // 加载字幕列表
  async function loadSubtitles() {
    if (!currentVideo) return;
    const token = localStorage.getItem("auth_token") || "";
    try {
      const res = await fetch(`/api/v1/video/subtitles?path=${encodeURIComponent(currentVideo.path)}&token=${token}`);
      if (res.ok) {
        const data = await res.json();
        subtitles = data.subtitles || [];
      }
    } catch (e) {
      console.error("Failed to load subtitles:", e);
    }
  }

  // 选择字幕
  async function selectSubtitle(sub: SubtitleInfo | null) {
    currentSubtitle = sub;
    showSubtitleMenu = false;
    
    if (!videoElement) return;
    
    // 移除现有字幕
    const tracks = videoElement.querySelectorAll("track");
    tracks.forEach(t => t.remove());
    
    if (!sub) return;
    
    const token = localStorage.getItem("auth_token") || "";
    const track = document.createElement("track");
    track.kind = "subtitles";
    track.label = sub.label;
    track.srclang = sub.language || "zh";
    track.default = true;
    
    if (sub.embedded) {
      track.src = `/api/v1/video/subtitle?path=${encodeURIComponent(currentVideo!.path)}&embedded=true&index=${sub.index}&token=${token}`;
    } else {
      track.src = `/api/v1/video/subtitle?path=${encodeURIComponent(sub.path)}&token=${token}`;
    }
    
    videoElement.appendChild(track);
    track.track.mode = "showing";
  }

  // 加载视频信息
  async function loadVideoInfo() {
    if (!currentVideo) return;
    const token = localStorage.getItem("auth_token") || "";
    try {
      const res = await fetch(`/api/v1/video/info?path=${encodeURIComponent(currentVideo.path)}&token=${token}`);
      if (res.ok) {
        videoInfo = await res.json();
        
        // 根据视频尺寸调整窗口大小
        if (videoInfo && videoInfo.width > 0 && videoInfo.height > 0) {
          adjustWindowSize(videoInfo.width, videoInfo.height);
        }
      }
    } catch (e) {
      console.error("Failed to load video info:", e);
    }
  }

  // 调整窗口大小以适应视频
  function adjustWindowSize(videoWidth: number, videoHeight: number) {
    // 获取屏幕可用尺寸
    const maxWidth = window.innerWidth * 0.85;
    const maxHeight = window.innerHeight * 0.85;
    const minWidth = 640;
    const minHeight = 480;
    
    // 计算适合的窗口尺寸 (保持视频比例)
    const aspectRatio = videoWidth / videoHeight;
    let newWidth = videoWidth;
    let newHeight = videoHeight;
    
    // 控制栏和标题栏的高度
    const controlsHeight = 120;
    
    // 如果视频太大，缩小到可用范围
    if (newWidth > maxWidth) {
      newWidth = maxWidth;
      newHeight = newWidth / aspectRatio;
    }
    if (newHeight + controlsHeight > maxHeight) {
      newHeight = maxHeight - controlsHeight;
      newWidth = newHeight * aspectRatio;
    }
    
    // 确保不小于最小尺寸
    newWidth = Math.max(minWidth, Math.round(newWidth));
    newHeight = Math.max(minHeight, Math.round(newHeight + controlsHeight));
    
    windowManager.resize(windowId, newWidth, newHeight);
  }

  // 视频截图
  async function takeScreenshot() {
    if (!videoElement) return;
    
    try {
      const canvas = document.createElement("canvas");
      canvas.width = videoElement.videoWidth;
      canvas.height = videoElement.videoHeight;
      
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      
      ctx.drawImage(videoElement, 0, 0);
      
      // 下载截图
      const link = document.createElement("a");
      const filename = `${getDisplayName(currentVideo?.name || "screenshot")}_${formatTime(currentTime).replace(/:/g, "-")}.png`;
      link.download = filename;
      link.href = canvas.toDataURL("image/png");
      link.click();
    } catch (e) {
      console.error("Screenshot failed:", e);
    }
  }

  // 画中画模式
  async function togglePiP() {
    if (!videoElement) return;
    
    try {
      if (document.pictureInPictureElement) {
        await document.exitPictureInPicture();
        isPiP = false;
      } else if (document.pictureInPictureEnabled) {
        await videoElement.requestPictureInPicture();
        isPiP = true;
      }
    } catch (e) {
      console.error("PiP failed:", e);
    }
  }

  // 循环模式图标
  function getLoopIcon(): string {
    switch (videoSettings.settings.loopMode) {
      case "single": return "mdi:repeat-once";
      case "list": return "mdi:repeat";
      default: return "mdi:repeat-off";
    }
  }

  // 切换循环模式
  function toggleLoopMode() {
    const modes: Array<"none" | "single" | "list"> = ["none", "single", "list"];
    const currentIdx = modes.indexOf(videoSettings.settings.loopMode);
    videoSettings.update({ loopMode: modes[(currentIdx + 1) % 3] });
  }

  // 获取最近播放缩略图URL
  function getHistoryThumbnail(path: string): string {
    const token = localStorage.getItem("auth_token") || "";
    return fileService.getThumbnailUrl(path, 128);
  }

  // 格式化文件大小
  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + " MB";
    return (bytes / (1024 * 1024 * 1024)).toFixed(2) + " GB";
  }

  // 格式化时长(秒)为 hh:mm:ss
  function formatDuration(seconds: number): string {
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    const s = Math.floor(seconds % 60);
    if (h > 0) {
      return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
    }
    return `${m}:${String(s).padStart(2, "0")}`;
  }
</script>

<svelte:window on:keydown={handleKeyDown} />

<div class="video-player-container" data-window-id={windowId}>
  {#if !initialized}
    <div class="loading-container">
      <Spinner size="lg" />
    </div>
  {:else if showSetup}
    <VideoSetup onComplete={handleSetupComplete} onSkip={handleSetupSkip} />
  {:else if showSettings}
    <VideoSettings onClose={() => showSettings = false} />
  {:else if view === "browser"}
    <!-- 文件浏览视图 -->
    <div class="browser-view">
      <div class="browser-header">
        <div class="nav-buttons">
          <button class="nav-btn" onclick={goUp} disabled={currentPath === "/"}>
            <Icon icon="mdi:arrow-up" width={18} />
          </button>
          <button class="nav-btn" onclick={() => loadDirectory(currentPath)}>
            <Icon icon="mdi:refresh" width={18} />
          </button>
        </div>

        <div class="breadcrumbs">
          {#each breadcrumbs as crumb, i}
            {#if i > 0}
              <span class="separator">/</span>
            {/if}
            <button class="crumb" onclick={() => navigateTo(crumb.path)}>
              {crumb.name}
            </button>
          {/each}
        </div>

        <button class="settings-btn" onclick={() => showSettings = true}>
          <Icon icon="mdi:cog" width={20} />
        </button>
      </div>

      <div class="browser-content">
        {#if loading}
          <div class="loading-state">
            <Spinner size="md" />
            <span>{$t("common.loading")}</span>
          </div>
        {:else if error}
          <div class="error-state">
            <Icon icon="mdi:alert-circle" width={48} />
            <p>{error}</p>
            <Button variant="outline" onclick={() => loadDirectory(currentPath)}>
              {$t("video.player.retry")}
            </Button>
          </div>
        {:else if files.length === 0}
          <div class="empty-state">
            <Icon icon="mdi:folder-open" width={64} />
            <p>{$t("video.player.folderEmpty")}</p>
          </div>
        {:else}
          <div class="file-list">
            <!-- 文件夹 -->
            {#each directories as dir}
              <button class="file-item" ondblclick={() => navigateTo(dir.path)}>
                <Icon icon="mdi:folder" width={32} class="folder-icon" />
                <span class="file-name">{dir.name}</span>
              </button>
            {/each}
            <!-- 视频文件 -->
            {#each playlist as file}
              <button class="file-item video-item" ondblclick={() => playFile(file)}>
                <div class="video-thumb">
                  <LazyThumbnail
                    src={fileService.getThumbnailUrl(file.path, 256)}
                    alt={file.name}
                    fallbackIcon="mdi:file-video"
                    isVideo={true}
                    size={80}
                  />
                </div>
                <span class="file-name">{getDisplayName(file.name)}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>

      <!-- 最近播放 -->
      {#if videoSettings.history.length > 0 && currentPath === videoSettings.settings.defaultVideoPath}
        <div class="recent-section">
          <h3>
            <Icon icon="mdi:history" width={18} />
            {$t("video.player.recentPlayed")}
          </h3>
          <div class="recent-list">
            {#each videoSettings.getRecentVideos(5) as item}
              <button
                class="recent-item"
                onclick={() => {
                  const pathParts = item.path.split("/");
                  pathParts.pop();
                  loadDirectory(pathParts.join("/") || "/").then(() => {
                    const file = files.find(f => f.path === item.path);
                    if (file) playFile(file);
                  });
                }}
              >
                <div class="recent-thumb">
                  <LazyThumbnail
                    src={getHistoryThumbnail(item.path)}
                    alt={item.name}
                    fallbackIcon="mdi:file-video"
                    isVideo={true}
                    size={48}
                  />
                </div>
                <div class="recent-info">
                  <span class="recent-name">{getDisplayName(item.name)}</span>
                  <span class="recent-progress">
                    {formatTime(item.position)} / {formatTime(item.duration)}
                  </span>
                </div>
              </button>
            {/each}
          </div>
        </div>
      {/if}
    </div>
  {:else}
    <!-- 播放器视图 -->
    <div 
      class="player-view" 
      class:controls-hidden={!showControls}
      onmousemove={handleMouseMove}
    >
      <video
        bind:this={videoElement}
        ontimeupdate={handleTimeUpdate}
        onloadedmetadata={handleLoadedMetadata}
        onplay={handlePlay}
        onpause={handlePause}
        onended={handleEnded}
        onerror={handleError}
        onclick={togglePlay}
        ondblclick={toggleFullscreen}
      >
        <track kind="captions" />
      </video>

      {#if videoError}
        <div class="video-error">
          <Icon icon="mdi:alert-circle" width={48} />
          <p>{videoError}</p>
          <Button variant="outline" onclick={backToBrowser}>
            {$t("video.player.backToList")}
          </Button>
        </div>
      {/if}

      <!-- 视频信息面板 -->
      {#if showVideoInfo && videoInfo}
        <div class="video-info-panel">
          <div class="info-header">
            <span>{$t("video.player.videoInfo")}</span>
            <button class="info-close" onclick={() => showVideoInfo = false}>
              <Icon icon="mdi:close" width={18} />
            </button>
          </div>
          <div class="info-content">
            <div class="info-row">
              <span class="info-label">{$t("video.player.resolution")}</span>
              <span class="info-value">{videoInfo.width} × {videoInfo.height}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("video.player.codec")}</span>
              <span class="info-value">{videoInfo.codec}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("video.player.duration")}</span>
              <span class="info-value">{formatDuration(videoInfo.duration)}</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("video.player.bitrate")}</span>
              <span class="info-value">{Math.round(videoInfo.bitrate / 1000)} kbps</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("video.player.frameRate")}</span>
              <span class="info-value">{videoInfo.fps.toFixed(2)} fps</span>
            </div>
            <div class="info-row">
              <span class="info-label">{$t("video.player.fileSize")}</span>
              <span class="info-value">{formatSize(videoInfo.size)}</span>
            </div>
          </div>
        </div>
      {/if}

      <!-- 字幕显示 -->
      {#if currentSubtitle && subtitleText}
        <div class="subtitle-display">
          {subtitleText}
        </div>
      {/if}

      <!-- 画中画提示 -->
      {#if isPiP}
        <div class="pip-placeholder">
          <Icon icon="mdi:picture-in-picture-bottom-right" width={64} />
          <p>{$t("video.player.pipMode")}</p>
          <Button variant="outline" onclick={togglePiP}>{$t("video.player.exitPip")}</Button>
        </div>
      {/if}

      <!-- 顶部栏 -->
      <div class="player-top-bar" class:hidden={!showControls}>
        <button class="back-btn" onclick={backToBrowser}>
          <Icon icon="mdi:arrow-left" width={24} />
        </button>
        <span class="video-title">{currentVideo?.name || ""}</span>
      </div>

      <!-- 底部控制栏 -->
      <div class="player-controls" class:hidden={!showControls}>
        <!-- 进度条 -->
        <div
          class="progress-bar"
          onmousedown={handleProgressMouseDown}
          onmousemove={handleProgressMouseMove}
          onmouseup={handleProgressMouseUp}
          onmouseleave={handleProgressMouseUp}
          role="slider"
          aria-label={$t("video.player.videoProgress")}
          aria-valuenow={currentTime}
          aria-valuemin={0}
          aria-valuemax={duration}
          tabindex="0"
        >
          <div class="progress-buffered" style="width: {(buffered / duration) * 100}%"></div>
          <div class="progress-played" style="width: {(isDraggingProgress ? dragProgress : currentTime / duration) * 100}%"></div>
          <div class="progress-thumb" style="left: {(isDraggingProgress ? dragProgress : currentTime / duration) * 100}%"></div>
        </div>

        <div class="controls-row">
          <div class="controls-left">
            <!-- 播放/暂停 -->
            <button class="control-btn" onclick={togglePlay}>
              <Icon icon={isPlaying ? "mdi:pause" : "mdi:play"} width={28} />
            </button>
            <!-- 上一个/下一个 -->
            <button class="control-btn" onclick={playPrevious} disabled={currentIndex <= 0}>
              <Icon icon="mdi:skip-previous" width={24} />
            </button>
            <button class="control-btn" onclick={playNext} disabled={currentIndex >= playlist.length - 1}>
              <Icon icon="mdi:skip-next" width={24} />
            </button>
            <!-- 音量 -->
            <div class="volume-control">
              <button class="control-btn" onclick={toggleMute}>
                <Icon icon={isMuted || volume === 0 ? "mdi:volume-off" : volume < 0.5 ? "mdi:volume-medium" : "mdi:volume-high"} width={22} />
              </button>
              <input
                type="range"
                class="volume-slider"
                min="0"
                max="1"
                step="0.01"
                value={volume}
                oninput={(e) => setVolume(parseFloat((e.target as HTMLInputElement).value))}
              />
            </div>
            <!-- 时间显示 -->
            <span class="time-display">
              {formatTime(currentTime)} / {formatTime(duration)}
            </span>
          </div>

          <div class="controls-right">
            <!-- 快退/快进按钮 -->
            <button class="control-btn" onclick={() => seek(-10)} title={$t("video.player.rewind10")}>
              <Icon icon="mdi:rewind-10" width={22} />
            </button>
            <button class="control-btn" onclick={() => seek(10)} title={$t("video.player.forward10")}>
              <Icon icon="mdi:fast-forward-10" width={22} />
            </button>
            <!-- 字幕选择 -->
            <div class="subtitle-control">
              <button 
                class="control-btn" 
                class:active={currentSubtitle !== null}
                onclick={() => showSubtitleMenu = !showSubtitleMenu}
                title={$t("video.player.subtitle")}
              >
                <Icon icon="mdi:subtitles" width={22} />
              </button>
              {#if showSubtitleMenu}
                <div class="popup-menu subtitle-menu">
                  <button 
                    class="menu-option" 
                    class:active={currentSubtitle === null}
                    onclick={() => selectSubtitle(null)}
                  >
                    {$t("video.player.closeSubtitle")}
                  </button>
                  {#each subtitles as sub}
                    <button 
                      class="menu-option"
                      class:active={currentSubtitle?.path === sub.path && currentSubtitle?.index === sub.index}
                      onclick={() => selectSubtitle(sub)}
                    >
                      {sub.label}
                    </button>
                  {/each}
                  {#if subtitles.length === 0}
                    <span class="menu-hint">{$t("video.player.noSubtitleDetected")}</span>
                  {/if}
                </div>
              {/if}
            </div>
            <!-- 循环模式 -->
            <button class="control-btn" onclick={toggleLoopMode} title="{$t("video.player.loop")}: {videoSettings.settings.loopMode}">
              <Icon icon={getLoopIcon()} width={22} />
            </button>
            <!-- 播放速度 -->
            <div class="speed-control">
              <button class="control-btn speed-btn" onclick={() => showSpeedMenu = !showSpeedMenu}>
                {playbackRate}x
              </button>
              {#if showSpeedMenu}
                <div class="popup-menu speed-menu">
                  {#each playbackRates as rate}
                    <button
                      class="menu-option"
                      class:active={playbackRate === rate}
                      onclick={() => { setPlaybackRate(rate); showSpeedMenu = false; }}
                    >
                      {rate}x
                    </button>
                  {/each}
                </div>
              {/if}
            </div>
            <!-- 画中画 -->
            {#if document.pictureInPictureEnabled}
              <button class="control-btn" onclick={togglePiP} title={$t("video.player.pip")}>
                <Icon icon={isPiP ? "mdi:picture-in-picture-bottom-right-outline" : "mdi:picture-in-picture-bottom-right"} width={22} />
              </button>
            {/if}
            <!-- 截图 -->
            <button class="control-btn" onclick={takeScreenshot} title={$t("video.player.screenshot")}>
              <Icon icon="mdi:camera" width={22} />
            </button>
            <!-- 视频信息 -->
            <button class="control-btn" onclick={() => showVideoInfo = !showVideoInfo} title={$t("video.player.videoInfo")}>
              <Icon icon="mdi:information" width={22} />
            </button>
            <!-- 全屏 -->
            <button class="control-btn" onclick={toggleFullscreen}>
              <Icon icon={isFullscreen ? "mdi:fullscreen-exit" : "mdi:fullscreen"} width={24} />
            </button>
          </div>
        </div>
      </div>

      <!-- 中央播放按钮（暂停时显示） -->
      {#if !isPlaying && !videoError}
        <button class="center-play-btn" onclick={togglePlay}>
          <Icon icon="mdi:play" width={64} />
        </button>
      {/if}
    </div>
  {/if}
</div>

<style>
  .video-player-container {
    height: 100%;
    display: flex;
    flex-direction: column;
    background: var(--bg-primary, #1a1a2e);
    color: var(--text-primary, #fff);

    :global([data-theme="light"]) & {
      background: #f8f9fa;
      color: #333;
    }
  }

  .loading-container {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  /* 文件浏览视图 */
  .browser-view {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }

  .browser-header {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
  }

  .nav-buttons {
    display: flex;
    gap: 4px;
  }

  .nav-btn {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 6px;
    background: var(--bg-secondary, rgba(255, 255, 255, 0.1));
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover:not(:disabled) {
      background: var(--bg-hover, rgba(255, 255, 255, 0.15));
    }

    &:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }
  }

  .breadcrumbs {
    flex: 1;
    display: flex;
    align-items: center;
    gap: 4px;
    overflow: hidden;
    font-size: 14px;
  }

  .separator {
    color: var(--text-secondary, rgba(255, 255, 255, 0.5));
  }

  .crumb {
    background: none;
    border: none;
    color: var(--text-secondary, rgba(255, 255, 255, 0.7));
    cursor: pointer;
    padding: 4px 8px;
    border-radius: 4px;
    white-space: nowrap;

    &:hover {
      background: var(--bg-secondary, rgba(255, 255, 255, 0.1));
      color: var(--text-primary);
    }

    &:last-child {
      color: var(--text-primary);
    }
  }

  .settings-btn {
    width: 36px;
    height: 36px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: inherit;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: var(--bg-secondary, rgba(255, 255, 255, 0.1));
    }
  }

  .browser-content {
    flex: 1;
    overflow: auto;
    padding: 16px;
  }

  .loading-state,
  .error-state,
  .empty-state {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    color: var(--text-secondary);
  }

  .file-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 16px;
  }

  .file-item {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px 12px;
    border: none;
    border-radius: 12px;
    background: var(--bg-secondary, rgba(255, 255, 255, 0.05));
    color: inherit;
    cursor: pointer;
    transition: all 0.15s;

    &:hover {
      background: var(--bg-hover, rgba(255, 255, 255, 0.1));
      transform: translateY(-2px);
    }

    :global(.folder-icon) {
      color: #ffc107;
    }
  }

  .video-item {
    .video-thumb {
      position: relative;
      width: 80px;
      height: 80px;
      background: rgba(0, 0, 0, 0.2);
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      overflow: hidden;
    }
  }

  .file-name {
    font-size: 13px;
    text-align: center;
    word-break: break-word;
    line-height: 1.3;
    max-height: 2.6em;
    overflow: hidden;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
  }

  .recent-section {
    padding: 16px;
    border-top: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));

    h3 {
      display: flex;
      align-items: center;
      gap: 8px;
      margin: 0 0 12px;
      font-size: 14px;
      font-weight: 500;
      color: var(--text-secondary);
    }
  }

  .recent-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .recent-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 12px;
    border: none;
    border-radius: 8px;
    background: var(--bg-secondary, rgba(255, 255, 255, 0.05));
    color: inherit;
    cursor: pointer;
    text-align: left;

    &:hover {
      background: var(--bg-hover, rgba(255, 255, 255, 0.1));
    }
  }

  .recent-info {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    overflow: hidden;
  }

  .recent-name {
    font-size: 14px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .recent-progress {
    font-size: 12px;
    color: var(--text-secondary);
  }

  /* 播放器视图 */
  .player-view {
    flex: 1;
    position: relative;
    background: #000;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: default;

    &.controls-hidden {
      cursor: none;
    }
  }

  video {
    width: 100%;
    height: 100%;
    object-fit: contain;
  }

  .video-error {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: rgba(0, 0, 0, 0.8);
    color: #fff;
  }

  .player-top-bar {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 12px 16px;
    background: linear-gradient(to bottom, rgba(0, 0, 0, 0.7), transparent);
    transition: opacity 0.3s;

    &.hidden {
      opacity: 0;
      pointer-events: none;
    }
  }

  .back-btn {
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 8px;
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover {
      background: rgba(255, 255, 255, 0.2);
    }
  }

  .video-title {
    flex: 1;
    font-size: 16px;
    font-weight: 500;
    color: #fff;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .player-controls {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 16px;
    background: linear-gradient(to top, rgba(0, 0, 0, 0.8), transparent);
    transition: opacity 0.3s;

    &.hidden {
      opacity: 0;
      pointer-events: none;
    }
  }

  .progress-bar {
    position: relative;
    height: 4px;
    background: rgba(255, 255, 255, 0.2);
    border-radius: 2px;
    margin-bottom: 12px;
    cursor: pointer;

    &:hover {
      height: 6px;
      margin-bottom: 10px;
    }
  }

  .progress-buffered {
    position: absolute;
    height: 100%;
    background: rgba(255, 255, 255, 0.3);
    border-radius: 2px;
  }

  .progress-played {
    position: absolute;
    height: 100%;
    background: var(--accent-color, #7E57C2);
    border-radius: 2px;
  }

  .progress-thumb {
    position: absolute;
    top: 50%;
    width: 12px;
    height: 12px;
    background: #fff;
    border-radius: 50%;
    transform: translate(-50%, -50%);
    opacity: 0;
    transition: opacity 0.15s;

    .progress-bar:hover & {
      opacity: 1;
    }
  }

  .controls-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 4px;
    min-width: 0;
  }

  .controls-left,
  .controls-right {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-shrink: 0;
  }

  .controls-left {
    flex-shrink: 1;
    min-width: 0;
    overflow: hidden;
  }

  .control-btn {
    width: 32px;
    height: 32px;
    flex-shrink: 0;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;

    &:hover:not(:disabled) {
      background: rgba(255, 255, 255, 0.1);
    }

    &:disabled {
      opacity: 0.4;
      cursor: not-allowed;
    }
  }

  .volume-control {
    display: flex;
    align-items: center;
    gap: 2px;
    flex-shrink: 1;
    min-width: 0;
  }

  .volume-slider {
    width: 60px;
    min-width: 40px;
    height: 4px;
    -webkit-appearance: none;
    appearance: none;
    background: rgba(255, 255, 255, 0.3);
    border-radius: 2px;
    outline: none;
    flex-shrink: 1;

    &::-webkit-slider-thumb {
      -webkit-appearance: none;
      width: 12px;
      height: 12px;
      background: #fff;
      border-radius: 50%;
      cursor: pointer;
    }
  }

  .time-display {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.9);
    margin-left: 4px;
    white-space: nowrap;
    flex-shrink: 0;
  }

  .speed-control {
    position: relative;
  }

  .speed-menu {
    min-width: 70px;
  }

  .speed-btn {
    width: auto;
    min-width: 32px;
    padding: 0 6px;
    font-size: 12px;
    font-weight: 500;
  }

  .center-play-btn {
    position: absolute;
    width: 80px;
    height: 80px;
    border: none;
    border-radius: 50%;
    background: rgba(0, 0, 0, 0.6);
    color: #fff;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: transform 0.15s;

    &:hover {
      transform: scale(1.1);
      background: rgba(0, 0, 0, 0.8);
    }
  }

  /* 字幕控制 */
  .subtitle-control {
    position: relative;
  }

  .popup-menu {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 8px;
    padding: 8px;
    background: rgba(0, 0, 0, 0.95);
    border-radius: 8px;
    min-width: 150px;
    display: flex;
    flex-direction: column;
    gap: 4px;
    z-index: 20;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.5);
  }

  .menu-option {
    padding: 8px 12px;
    border: none;
    border-radius: 4px;
    background: transparent;
    color: #fff;
    cursor: pointer;
    font-size: 13px;
    text-align: left;
    white-space: nowrap;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
    }

    &.active {
      background: var(--accent-color, #7E57C2);
    }
  }

  .menu-hint {
    padding: 8px 12px;
    color: rgba(255, 255, 255, 0.5);
    font-size: 12px;
  }

  .control-btn.active {
    color: var(--accent-color, #7E57C2);
  }

  /* 视频信息面板 */
  .video-info-panel {
    position: absolute;
    top: 80px;
    right: 16px;
    background: rgba(0, 0, 0, 0.85);
    border-radius: 12px;
    padding: 16px;
    min-width: 240px;
    z-index: 15;
    backdrop-filter: blur(8px);
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
  }

  .info-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    color: #fff;
    font-size: 14px;
    font-weight: 600;
  }

  .info-close {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;

    &:hover {
      background: rgba(255, 255, 255, 0.1);
      color: #fff;
    }
  }

  .info-content {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    font-size: 13px;
  }

  .info-label {
    color: rgba(255, 255, 255, 0.6);
  }

  .info-value {
    color: #fff;
  }

  /* 字幕显示 */
  .subtitle-display {
    position: absolute;
    bottom: 100px;
    left: 50%;
    transform: translateX(-50%);
    max-width: 80%;
    padding: 8px 16px;
    background: rgba(0, 0, 0, 0.75);
    border-radius: 8px;
    color: #fff;
    font-size: 18px;
    text-align: center;
    line-height: 1.5;
    z-index: 10;
  }

  /* 最近播放缩略图 */
  .recent-thumb {
    width: 80px;
    height: 45px;
    border-radius: 6px;
    overflow: hidden;
    background: var(--bg-secondary, #1a1a1a);
    flex-shrink: 0;
  }

  /* 画中画占位提示 */
  .pip-placeholder {
    position: absolute;
    inset: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 16px;
    background: rgba(0, 0, 0, 0.9);
    color: #fff;
    z-index: 25;

    p {
      font-size: 16px;
      color: rgba(255, 255, 255, 0.8);
    }
  }
</style>
