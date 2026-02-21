<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { t } from "svelte-i18n";
  import Icon from "@iconify/svelte";
  import type { Photo } from "$shared/services/photos";
  import { photoService } from "$shared/services/photos";
  import { Button } from "$shared/ui";

  interface Props {
    photos: Photo[];
    initialIndex: number;
    onClose: () => void;
    onFavorite?: (photo: Photo) => void;
    onDelete?: (photo: Photo) => void;
  }

  let { photos, initialIndex, onClose, onFavorite, onDelete }: Props = $props();

  let currentIndex = $state(initialIndex);
  let showInfo = $state(false);
  let loading = $state(false);
  let zoom = $state(1);
  let panX = $state(0);
  let panY = $state(0);

  // 自动播放
  let autoPlay = $state(false);
  let autoPlayInterval = $state(3); // 秒
  let showIntervalMenu = $state(false);
  let autoPlayTimer: ReturnType<typeof setInterval> | null = null;
  type TransitionEffect = "fade" | "fly-left" | "fly-right" | "fly-up" | "fly-down" | "zoom" | "zoom-rotate" | "flip-x" | "flip-y" | "rotate" | "blur" | "bounce" | "swing" | "spiral" | "dissolve";
  let transitionEffect = $state<TransitionEffect>("fade");
  let transitioning = $state(false);
  let transitionClass = $state("");
  let showSettingsPanel = $state(false);
  const intervalOptions = [1, 2, 3, 5, 8, 10, 15, 20];
  
  let effectOptions = $derived([
    { value: "fade" as TransitionEffect, label: $t("viewer.effectFade") },
    { value: "fly-left" as TransitionEffect, label: $t("viewer.effectFlyLeft") },
    { value: "fly-right" as TransitionEffect, label: $t("viewer.effectFlyRight") },
    { value: "fly-up" as TransitionEffect, label: $t("viewer.effectFlyUp") },
    { value: "fly-down" as TransitionEffect, label: $t("viewer.effectFlyDown") },
    { value: "zoom" as TransitionEffect, label: $t("viewer.effectZoom") },
    { value: "zoom-rotate" as TransitionEffect, label: $t("viewer.effectZoomRotate") },
    { value: "flip-x" as TransitionEffect, label: $t("viewer.effectFlipX") },
    { value: "flip-y" as TransitionEffect, label: $t("viewer.effectFlipY") },
    { value: "rotate" as TransitionEffect, label: $t("viewer.effectRotate") },
    { value: "blur" as TransitionEffect, label: $t("viewer.effectBlur") },
    { value: "bounce" as TransitionEffect, label: $t("viewer.effectBounce") },
    { value: "swing" as TransitionEffect, label: $t("viewer.effectSwing") },
    { value: "spiral" as TransitionEffect, label: $t("viewer.effectSpiral") },
    { value: "dissolve" as TransitionEffect, label: $t("viewer.effectDissolve") },
  ]);

  let currentPhoto = $derived(photos[currentIndex]);

  // 缩略图条引用和自动滚动
  let thumbnailStripEl: HTMLDivElement | undefined = $state();

  $effect(() => {
    // 当 currentIndex 变化时自动滚动缩略图
    if (thumbnailStripEl) {
      const activeItem = thumbnailStripEl.children[currentIndex] as HTMLElement;
      if (activeItem) {
        activeItem.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "center" });
      }
    }
  });

  // 自动播放控制
  function toggleAutoPlay() {
    autoPlay = !autoPlay;
    if (autoPlay) {
      startAutoPlay();
    } else {
      stopAutoPlay();
    }
  }

  function startAutoPlay() {
    stopAutoPlay();
    autoPlayTimer = setInterval(() => {
      if (currentIndex < photos.length - 1) {
        transitionToNext();
      } else {
        // 循环回第一张
        transitionTo(0);
      }
    }, autoPlayInterval * 1000);
  }

  function stopAutoPlay() {
    if (autoPlayTimer) {
      clearInterval(autoPlayTimer);
      autoPlayTimer = null;
    }
  }

  function setInterval_(interval: number) {
    autoPlayInterval = interval;
    showIntervalMenu = false;
    if (autoPlay) {
      startAutoPlay(); // 重启以使用新间隔
    }
  }

  function setEffect(effect: TransitionEffect) {
    transitionEffect = effect;
  }

  function transitionToNext() {
    const nextIdx = currentIndex < photos.length - 1 ? currentIndex + 1 : 0;
    transitionTo(nextIdx);
  }

  function transitionTo(idx: number) {
    if (transitioning) return;
    transitioning = true;

    // 退出动画
    transitionClass = `exit-${transitionEffect}`;

    setTimeout(() => {
      currentIndex = idx;
      resetView();
      // 进入动画
      transitionClass = `enter-${transitionEffect}`;

      setTimeout(() => {
        transitionClass = "";
        transitioning = false;
      }, 400);
    }, 300);
  }

  // 键盘导航
  function handleKeydown(e: KeyboardEvent) {
    switch (e.key) {
      case "Escape":
        onClose();
        break;
      case "ArrowLeft":
        prev();
        break;
      case "ArrowRight":
        next();
        break;
      case "i":
        showInfo = !showInfo;
        break;
      case " ":
        e.preventDefault();
        toggleAutoPlay();
        break;
    }
  }

  function prev() {
    if (currentIndex > 0) {
      if (autoPlay) {
        transitionTo(currentIndex - 1);
      } else {
        currentIndex--;
        resetView();
      }
    }
  }

  function next() {
    if (currentIndex < photos.length - 1) {
      if (autoPlay) {
        transitionToNext();
      } else {
        currentIndex++;
        resetView();
      }
    }
  }

  function resetView() {
    zoom = 1;
    panX = 0;
    panY = 0;
  }

  function zoomIn() {
    zoom = Math.min(zoom * 1.5, 5);
  }

  function zoomOut() {
    zoom = Math.max(zoom / 1.5, 0.5);
    if (zoom <= 1) {
      panX = 0;
      panY = 0;
    }
  }

  function handleWheel(e: WheelEvent) {
    if (e.ctrlKey || e.metaKey) {
      e.preventDefault();
      if (e.deltaY < 0) {
        zoomIn();
      } else {
        zoomOut();
      }
    }
  }

  function formatDate(dateStr: string | null): string {
    if (!dateStr) return $t("viewer.unknown");
    const date = new Date(dateStr);
    return date.toLocaleDateString("zh-CN", {
      year: "numeric",
      month: "long",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  }

  function formatSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / 1024 / 1024).toFixed(1) + " MB";
  }

  onMount(() => {
    document.addEventListener("keydown", handleKeydown);
    document.body.style.overflow = "hidden";
  });

  onDestroy(() => {
    document.removeEventListener("keydown", handleKeydown);
    document.body.style.overflow = "";
    stopAutoPlay();
  });
</script>

<div class="photo-viewer" onwheel={handleWheel}>
  <!-- 背景遮罩 -->
  <div class="backdrop" onclick={onClose}></div>

  <!-- 顶部工具栏 -->
  <header class="viewer-toolbar">
    <div class="toolbar-left">
      <span class="counter">{currentIndex + 1} / {photos.length}</span>
    </div>
    <div class="toolbar-center">
      <span class="filename">{currentPhoto?.filename}</span>
    </div>
    <div class="toolbar-right">
      <!-- 自动播放控制 -->
      <div class="autoplay-controls">
        <button class="toolbar-btn" class:active={autoPlay} onclick={toggleAutoPlay} title={$t("viewer.autoplay")}>
          <Icon icon={autoPlay ? "mdi:pause" : "mdi:play"} width={22} />
        </button>
        <button class="toolbar-btn" onclick={(e) => { e.stopPropagation(); showSettingsPanel = !showSettingsPanel; }} title={$t("viewer.playSettings")}>
          <Icon icon="mdi:cog" width={20} />
          <span class="settings-badge">{autoPlayInterval}s</span>
        </button>
      </div>
      <button class="toolbar-btn" onclick={() => showInfo = !showInfo} title={$t("viewer.details")}>
        <Icon icon="mdi:information" width={22} />
      </button>
      <button
        class="toolbar-btn"
        class:active={currentPhoto?.is_favorite}
        onclick={() => onFavorite?.(currentPhoto)}
        title={$t("viewer.favorite")}
      >
        <Icon icon={currentPhoto?.is_favorite ? "mdi:heart" : "mdi:heart-outline"} width={22} />
      </button>
      <a
        class="toolbar-btn"
        href={photoService.getOriginalUrl(currentPhoto?.id || "")}
        download={currentPhoto?.filename}
        title={$t("viewer.download")}
      >
        <Icon icon="mdi:download" width={22} />
      </a>
      <button class="toolbar-btn" onclick={() => onDelete?.(currentPhoto)} title={$t("viewer.delete")}>
        <Icon icon="mdi:delete" width={22} />
      </button>
      <button class="toolbar-btn close-btn" onclick={onClose} title={$t("viewer.close")}>
        <Icon icon="mdi:close" width={24} />
      </button>
    </div>
  </header>

  <!-- 图片/视频区域 -->
  <div class="viewer-content">
    <!-- 上一张 -->
    {#if currentIndex > 0}
      <button class="nav-btn prev" onclick={prev}>
        <Icon icon="mdi:chevron-left" width={48} />
      </button>
    {/if}

    <!-- 媒体内容 -->
    <div class="media-container {transitionClass}">
      {#if currentPhoto}
        {#if currentPhoto.type === "video"}
          <video
            src={photoService.getOriginalUrl(currentPhoto.id)}
            controls
            autoplay
            class="media video"
          >
            <track kind="captions" />
          </video>
        {:else}
          <img
            src={photoService.getPreviewUrl(currentPhoto.id)}
            alt={currentPhoto.filename}
            class="media image"
            style="transform: scale({zoom}) translate({panX}px, {panY}px);"
            draggable="false"
          />
        {/if}
      {/if}
    </div>

    <!-- 下一张 -->
    {#if currentIndex < photos.length - 1}
      <button class="nav-btn next" onclick={next}>
        <Icon icon="mdi:chevron-right" width={48} />
      </button>
    {/if}
  </div>

  <!-- 缩放控制 -->
  {#if currentPhoto?.type !== "video"}
    <div class="zoom-controls">
      <button class="zoom-btn" onclick={zoomOut}>
        <Icon icon="mdi:minus" width={20} />
      </button>
      <span class="zoom-level">{Math.round(zoom * 100)}%</span>
      <button class="zoom-btn" onclick={zoomIn}>
        <Icon icon="mdi:plus" width={20} />
      </button>
      <button class="zoom-btn" onclick={resetView}>
        <Icon icon="mdi:fit-to-screen" width={20} />
      </button>
    </div>
  {/if}

  <!-- 详情面板 -->
  {#if showInfo && currentPhoto}
    <aside class="info-panel">
      <h3>{$t("photos.viewer.details")}</h3>
      <div class="info-section">
        <div class="info-row">
          <span class="label">{$t("photos.viewer.filename")}</span>
          <span class="value">{currentPhoto.filename}</span>
        </div>
        <div class="info-row">
          <span class="label">{$t("photos.viewer.takenAt")}</span>
          <span class="value">{formatDate(currentPhoto.taken_at)}</span>
        </div>
        <div class="info-row">
          <span class="label">{$t("photos.viewer.dimensions")}</span>
          <span class="value">{currentPhoto.width} × {currentPhoto.height}</span>
        </div>
        <div class="info-row">
          <span class="label">{$t("photos.viewer.size")}</span>
          <span class="value">{formatSize(currentPhoto.size)}</span>
        </div>
        <div class="info-row">
          <span class="label">{$t("photos.viewer.type")}</span>
          <span class="value">{currentPhoto.mime_type}</span>
        </div>
      </div>

      {#if currentPhoto.camera_make || currentPhoto.camera_model}
        <div class="info-section">
          <h4>{$t("photos.viewer.camera")}</h4>
          {#if currentPhoto.camera_make}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.make")}</span>
              <span class="value">{currentPhoto.camera_make}</span>
            </div>
          {/if}
          {#if currentPhoto.camera_model}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.model")}</span>
              <span class="value">{currentPhoto.camera_model}</span>
            </div>
          {/if}
          {#if currentPhoto.f_number}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.aperture")}</span>
              <span class="value">f/{currentPhoto.f_number}</span>
            </div>
          {/if}
          {#if currentPhoto.exposure_time}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.shutter")}</span>
              <span class="value">{currentPhoto.exposure_time}</span>
            </div>
          {/if}
          {#if currentPhoto.iso}
            <div class="info-row">
              <span class="label">ISO</span>
              <span class="value">{currentPhoto.iso}</span>
            </div>
          {/if}
          {#if currentPhoto.focal_length}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.focalLength")}</span>
              <span class="value">{currentPhoto.focal_length}mm</span>
            </div>
          {/if}
        </div>
      {/if}

      {#if currentPhoto.latitude && currentPhoto.longitude}
        <div class="info-section">
          <h4>{$t("photos.viewer.location")}</h4>
          <div class="info-row">
            <span class="label">{$t("photos.viewer.coordinates")}</span>
            <span class="value">{currentPhoto.latitude.toFixed(6)}, {currentPhoto.longitude.toFixed(6)}</span>
          </div>
          {#if currentPhoto.city || currentPhoto.country}
            <div class="info-row">
              <span class="label">{$t("photos.viewer.place")}</span>
              <span class="value">{[currentPhoto.city, currentPhoto.country].filter(Boolean).join(", ")}</span>
            </div>
          {/if}
        </div>
      {/if}

      <div class="info-section">
        <div class="info-row">
          <span class="label">{$t("photos.viewer.path")}</span>
          <span class="value path">{currentPhoto.path}</span>
        </div>
      </div>
    </aside>
  {/if}

  <!-- 播放设置面板 -->
  {#if showSettingsPanel}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="settings-overlay" onclick={() => showSettingsPanel = false}></div>
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="settings-panel" onclick={(e) => e.stopPropagation()}>
      <div class="settings-header">
        <h3>{$t("viewer.playSettings")}</h3>
        <button class="toolbar-btn" onclick={() => showSettingsPanel = false}>
          <Icon icon="mdi:close" width={18} />
        </button>
      </div>
      <div class="settings-body">
        <div class="settings-section">
          <div class="settings-label">{$t("viewer.playInterval")}</div>
          <div class="interval-grid">
            {#each intervalOptions as opt}
              <button class="interval-chip" class:active={autoPlayInterval === opt} onclick={() => setInterval_(opt)}>
                {opt}s
              </button>
            {/each}
          </div>
        </div>
        <div class="settings-section">
          <div class="settings-label">{$t("viewer.transitionEffect")}</div>
          <div class="effect-grid">
            {#each effectOptions as eff}
              <button class="effect-chip" class:active={transitionEffect === eff.value} onclick={() => setEffect(eff.value)}>
                {eff.label}
              </button>
            {/each}
          </div>
        </div>
      </div>
    </div>
  {/if}

  <!-- 缩略图导航条 -->
  {#if photos.length > 1}
    <div class="thumbnail-strip" bind:this={thumbnailStripEl}>
      {#each photos as photo, i (photo.id)}
        <button
          class="strip-item"
          class:active={i === currentIndex}
          onclick={() => { currentIndex = i; resetView(); }}
        >
          <img src={photo.thumbnail_url} alt="" />
        </button>
      {/each}
    </div>
  {/if}
</div>

<style>
  .photo-viewer {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 48px;
    z-index: 1000;
    display: flex;
    flex-direction: column;
    background: rgba(0, 0, 0, 0.95);
  }

  .backdrop {
    position: absolute;
    inset: 0;
    z-index: -1;
  }

  /* 工具栏 */
  .viewer-toolbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    background: rgba(0, 0, 0, 0.5);
    color: #fff;
    z-index: 10;
  }

  .toolbar-left,
  .toolbar-right {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .toolbar-center {
    flex: 1;
    text-align: center;
  }

  .counter {
    font-size: 13px;
    color: rgba(255, 255, 255, 0.7);
  }

  .filename {
    font-size: 14px;
    font-weight: 500;
  }

  .toolbar-btn {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.8);
    cursor: pointer;
    padding: 8px;
    border-radius: 6px;
    display: flex;
    align-items: center;
    justify-content: center;
    text-decoration: none;
  }

  .toolbar-btn:hover {
    background: rgba(255, 255, 255, 0.1);
    color: #fff;
  }

  .toolbar-btn.active {
    color: #ff4757;
  }

  .close-btn:hover {
    background: rgba(255, 0, 0, 0.3);
  }

  /* 内容区 */
  .viewer-content {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    position: relative;
    overflow: hidden;
    padding: 16px;
  }

  .media-container {
    max-width: 100%;
    max-height: 100%;
    display: flex;
    align-items: center;
    justify-content: center;
  }

  .media {
    max-width: 100%;
    max-height: calc(100vh - 248px);
    object-fit: contain;
    user-select: none;
  }

  .media.image {
    transition: transform 0.1s ease;
  }

  .media.video {
    max-height: calc(100vh - 248px);
  }

  /* 导航按钮 */
  .nav-btn {
    position: absolute;
    top: 50%;
    transform: translateY(-50%);
    background: rgba(0, 0, 0, 0.5);
    border: none;
    color: #fff;
    cursor: pointer;
    padding: 16px 8px;
    border-radius: 8px;
    z-index: 5;
    opacity: 0.7;
    transition: opacity 0.2s ease, background 0.2s ease;
  }

  .nav-btn:hover {
    opacity: 1;
    background: rgba(0, 0, 0, 0.7);
  }

  .nav-btn.prev {
    left: 16px;
  }

  .nav-btn.next {
    right: 16px;
  }

  /* 缩放控制 */
  .zoom-controls {
    position: absolute;
    bottom: 80px;
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(0, 0, 0, 0.6);
    padding: 8px 12px;
    border-radius: 8px;
  }

  .zoom-btn {
    background: none;
    border: none;
    color: #fff;
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
    display: flex;
  }

  .zoom-btn:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  .zoom-level {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.8);
    min-width: 48px;
    text-align: center;
  }

  /* 详情面板 */
  .info-panel {
    position: absolute;
    top: 60px;
    right: 0;
    bottom: 60px;
    width: 300px;
    background: rgba(0, 0, 0, 0.85);
    border-left: 1px solid rgba(255, 255, 255, 0.1);
    padding: 16px;
    overflow-y: auto;
    color: #fff;
  }

  .info-panel h3 {
    margin: 0 0 16px;
    font-size: 16px;
    font-weight: 600;
  }

  .info-panel h4 {
    margin: 0 0 8px;
    font-size: 13px;
    font-weight: 500;
    color: rgba(255, 255, 255, 0.7);
  }

  .info-section {
    margin-bottom: 20px;
    padding-bottom: 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .info-row {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 8px;
    font-size: 13px;
  }

  .info-row .label {
    color: rgba(255, 255, 255, 0.6);
  }

  .info-row .value {
    text-align: right;
    flex: 1;
    margin-left: 16px;
    word-break: break-all;
  }

  .info-row .value.path {
    font-size: 11px;
    color: rgba(255, 255, 255, 0.5);
  }

  /* 缩略图条 */
  .thumbnail-strip {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 10px 16px;
    background: rgba(0, 0, 0, 0.6);
    overflow-x: auto;
    justify-content: flex-start;
    scrollbar-width: thin;
    scrollbar-color: rgba(255,255,255,0.3) transparent;
  }

  .thumbnail-strip::-webkit-scrollbar {
    height: 4px;
  }

  .thumbnail-strip::-webkit-scrollbar-thumb {
    background: rgba(255,255,255,0.3);
    border-radius: 2px;
  }

  .strip-item {
    flex-shrink: 0;
    width: 80px;
    height: 60px;
    border: 2px solid transparent;
    border-radius: 6px;
    overflow: hidden;
    cursor: pointer;
    opacity: 0.5;
    transition: opacity 0.2s ease, border-color 0.2s ease, transform 0.2s ease;
    background: none;
    padding: 0;
  }

  .strip-item:hover {
    opacity: 0.85;
    transform: scale(1.05);
  }

  .strip-item.active {
    opacity: 1;
    border-color: var(--accent-color, #0066cc);
    transform: scale(1.1);
  }

  .strip-item img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }

  /* 自动播放控制 */
  .autoplay-controls {
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .settings-badge {
    font-size: 10px;
    color: rgba(255, 255, 255, 0.6);
    margin-left: 2px;
  }

  /* 播放设置面板 */
  .settings-overlay {
    position: fixed;
    inset: 0;
    z-index: 50;
  }

  .settings-panel {
    position: absolute;
    top: 56px;
    right: 16px;
    width: 320px;
    background: rgba(25, 25, 30, 0.96);
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 12px;
    backdrop-filter: blur(24px);
    z-index: 51;
    box-shadow: 0 12px 48px rgba(0,0,0,0.6);
    color: #fff;
    overflow: hidden;
  }

  .settings-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 12px 16px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  }

  .settings-header h3 {
    margin: 0;
    font-size: 14px;
    font-weight: 600;
  }

  .settings-body {
    padding: 16px;
  }

  .settings-section {
    margin-bottom: 16px;
  }

  .settings-section:last-child {
    margin-bottom: 0;
  }

  .settings-label {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.5);
    margin-bottom: 8px;
    font-weight: 500;
  }

  .interval-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .interval-chip {
    padding: 5px 12px;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    color: rgba(255, 255, 255, 0.75);
    font-size: 13px;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .interval-chip:hover {
    background: rgba(255, 255, 255, 0.15);
  }

  .interval-chip.active {
    background: rgba(66, 133, 244, 0.3);
    border-color: rgba(66, 133, 244, 0.6);
    color: #4dabf7;
  }

  .effect-grid {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .effect-chip {
    padding: 5px 10px;
    background: rgba(255, 255, 255, 0.08);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 16px;
    color: rgba(255, 255, 255, 0.75);
    font-size: 12px;
    cursor: pointer;
    transition: all 0.15s ease;
  }

  .effect-chip:hover {
    background: rgba(255, 255, 255, 0.15);
  }

  .effect-chip.active {
    background: rgba(66, 133, 244, 0.3);
    border-color: rgba(66, 133, 244, 0.6);
    color: #4dabf7;
  }

  /* 过渡动画 */
  .media-container {
    transition: opacity 0.3s ease, transform 0.3s ease;
  }

  /* 淡入淡出 */
  .media-container.exit-fade { opacity: 0; }
  .media-container.enter-fade { animation: fadeIn 0.4s ease forwards; }

  /* 左飞入 */
  .media-container.exit-fly-left { transform: translateX(-100%); opacity: 0; }
  .media-container.enter-fly-left { animation: flyInFromRight 0.4s ease forwards; }

  /* 右飞入 */
  .media-container.exit-fly-right { transform: translateX(100%); opacity: 0; }
  .media-container.enter-fly-right { animation: flyInFromLeft 0.4s ease forwards; }

  /* 上飞入 */
  .media-container.exit-fly-up { transform: translateY(-60%); opacity: 0; }
  .media-container.enter-fly-up { animation: flyInFromBottom 0.4s ease forwards; }

  /* 下飞入 */
  .media-container.exit-fly-down { transform: translateY(60%); opacity: 0; }
  .media-container.enter-fly-down { animation: flyInFromTop 0.4s ease forwards; }

  /* 放大进入 */
  .media-container.exit-zoom { transform: scale(0.3); opacity: 0; }
  .media-container.enter-zoom { animation: zoomEnter 0.4s ease forwards; }

  /* 旋转缩放 */
  .media-container.exit-zoom-rotate { transform: scale(0.3) rotate(180deg); opacity: 0; }
  .media-container.enter-zoom-rotate { animation: zoomRotateIn 0.5s ease forwards; }

  /* 水平翻转 */
  .media-container.exit-flip-x { transform: perspective(800px) rotateY(90deg); opacity: 0; }
  .media-container.enter-flip-x { animation: flipXIn 0.5s ease forwards; }

  /* 垂直翻转 */
  .media-container.exit-flip-y { transform: perspective(800px) rotateX(90deg); opacity: 0; }
  .media-container.enter-flip-y { animation: flipYIn 0.5s ease forwards; }

  /* 旋转 */
  .media-container.exit-rotate { transform: rotate(90deg) scale(0.5); opacity: 0; }
  .media-container.enter-rotate { animation: rotateIn 0.4s ease forwards; }

  /* 模糊渐现 */
  .media-container.exit-blur { filter: blur(20px); opacity: 0; }
  .media-container.enter-blur { animation: blurIn 0.5s ease forwards; }

  /* 弹跳 */
  .media-container.exit-bounce { transform: scale(0); opacity: 0; }
  .media-container.enter-bounce { animation: bounceIn 0.6s cubic-bezier(0.36, 0.07, 0.19, 0.97) forwards; }

  /* 摆动 */
  .media-container.exit-swing { transform: rotate(15deg) translateX(100%); opacity: 0; }
  .media-container.enter-swing { animation: swingIn 0.5s ease forwards; }

  /* 螺旋 */
  .media-container.exit-spiral { transform: rotate(360deg) scale(0); opacity: 0; }
  .media-container.enter-spiral { animation: spiralIn 0.6s ease forwards; }

  /* 溶解 */
  .media-container.exit-dissolve { filter: blur(8px) brightness(2); opacity: 0; }
  .media-container.enter-dissolve { animation: dissolveIn 0.5s ease forwards; }

  @keyframes fadeIn {
    from { opacity: 0; }
    to { opacity: 1; }
  }
  @keyframes flyInFromRight {
    from { transform: translateX(100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  @keyframes flyInFromLeft {
    from { transform: translateX(-100%); opacity: 0; }
    to { transform: translateX(0); opacity: 1; }
  }
  @keyframes flyInFromBottom {
    from { transform: translateY(60%); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
  @keyframes flyInFromTop {
    from { transform: translateY(-60%); opacity: 0; }
    to { transform: translateY(0); opacity: 1; }
  }
  @keyframes zoomEnter {
    from { transform: scale(0.3); opacity: 0; }
    to { transform: scale(1); opacity: 1; }
  }
  @keyframes zoomRotateIn {
    from { transform: scale(0.3) rotate(-180deg); opacity: 0; }
    to { transform: scale(1) rotate(0deg); opacity: 1; }
  }
  @keyframes flipXIn {
    from { transform: perspective(800px) rotateY(-90deg); opacity: 0; }
    to { transform: perspective(800px) rotateY(0deg); opacity: 1; }
  }
  @keyframes flipYIn {
    from { transform: perspective(800px) rotateX(-90deg); opacity: 0; }
    to { transform: perspective(800px) rotateX(0deg); opacity: 1; }
  }
  @keyframes rotateIn {
    from { transform: rotate(-90deg) scale(0.5); opacity: 0; }
    to { transform: rotate(0deg) scale(1); opacity: 1; }
  }
  @keyframes blurIn {
    from { filter: blur(20px); opacity: 0; }
    to { filter: blur(0); opacity: 1; }
  }
  @keyframes bounceIn {
    0% { transform: scale(0); opacity: 0; }
    50% { transform: scale(1.15); opacity: 0.8; }
    70% { transform: scale(0.95); opacity: 1; }
    100% { transform: scale(1); opacity: 1; }
  }
  @keyframes swingIn {
    0% { transform: rotate(-15deg) translateX(-100%); opacity: 0; }
    60% { transform: rotate(5deg) translateX(0); opacity: 1; }
    80% { transform: rotate(-3deg); }
    100% { transform: rotate(0deg); opacity: 1; }
  }
  @keyframes spiralIn {
    from { transform: rotate(-360deg) scale(0); opacity: 0; }
    to { transform: rotate(0deg) scale(1); opacity: 1; }
  }
  @keyframes dissolveIn {
    from { filter: blur(8px) brightness(2); opacity: 0; }
    to { filter: blur(0) brightness(1); opacity: 1; }
  }
</style>
