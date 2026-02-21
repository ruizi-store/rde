<!-- Wallpaper.svelte - 壁纸组件 -->
<!-- 支持静态图片、WebGL动态壁纸、Lottie动画 -->
<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { browser } from "$app/environment";
  import { wallpaper, type WallpaperItem } from "$desktop/stores/wallpaper.svelte";
  import { notificationBubbleStore } from "$shared/stores/notification-bubble.svelte";

  let canvasElement = $state<HTMLCanvasElement | null>(null);
  let containerElement = $state<HTMLDivElement | null>(null);
  let currentWallpaperInstance: any = null;

  // 客户端就绪标志 - 防止 SSR 闪烁
  let ready = $state(false);

  // WebGL 支持状态
  let webglSupported = $state(true);

  // 清理当前壁纸实例
  function cleanupCurrentWallpaper() {
    if (currentWallpaperInstance) {
      if (typeof currentWallpaperInstance.destroy === "function") {
        currentWallpaperInstance.destroy();
      }
      currentWallpaperInstance = null;
    }
  }

  // 加载 WebGL 动态壁纸
  async function loadWebglWallpaper(file: string, initFn: string) {
    if (!canvasElement) return;

    // 检查 WebGL 支持
    if (!wallpaper.isWebGLSupported()) {
      console.warn("WebGL 不支持，回退到静态壁纸");
      webglSupported = false;
      fallbackToStatic();
      return;
    }

    try {
      cleanupCurrentWallpaper();
      // 使用动态脚本加载方式避免 vite 动态导入限制
      const scriptUrl = `/wallpapers/${file}`;
      const response = await fetch(scriptUrl);
      if (!response.ok) throw new Error(`Failed to load ${scriptUrl}`);
      const scriptText = await response.text();
      const blob = new Blob([scriptText], { type: "application/javascript" });
      const blobUrl = URL.createObjectURL(blob);
      const module = await import(/* @vite-ignore */ blobUrl);
      URL.revokeObjectURL(blobUrl);
      if (module[initFn]) {
        currentWallpaperInstance = module[initFn](canvasElement);
      }
    } catch (e) {
      console.error("加载 WebGL 壁纸失败:", e);
      fallbackToStatic();
    }
  }

  // 加载 Lottie 动画壁纸（Canvas 2D 实现）
  async function loadLottieWallpaper(file: string, initFn: string) {
    if (!containerElement) return;

    try {
      cleanupCurrentWallpaper();
      // 使用动态脚本加载方式避免 vite 动态导入限制
      const scriptUrl = `/wallpapers/${file}`;
      const response = await fetch(scriptUrl);
      if (!response.ok) throw new Error(`Failed to load ${scriptUrl}`);
      const scriptText = await response.text();
      const blob = new Blob([scriptText], { type: "application/javascript" });
      const blobUrl = URL.createObjectURL(blob);
      const module = await import(/* @vite-ignore */ blobUrl);
      URL.revokeObjectURL(blobUrl);
      if (module[initFn]) {
        currentWallpaperInstance = module[initFn](containerElement);
      }
    } catch (e) {
      console.error("加载 Lottie 壁纸失败:", e);
      fallbackToStatic();
    }
  }

  // 回退到随机静态壁纸
  function fallbackToStatic() {
    const staticItem = wallpaper.getRandomStaticWallpaper();
    if (staticItem) {
      wallpaper.setWallpaper(staticItem.id, "static");
    }
  }

  // 监听壁纸变化
  $effect(() => {
    const item = wallpaper.currentItem;
    if (!item || !ready) return;

    // 同步壁纸 ID 到通知泡泡 store
    notificationBubbleStore.setCurrentWallpaper(item.id);

    // 根据类型加载对应壁纸
    if (item.type === "webgl" && item.init && canvasElement) {
      loadWebglWallpaper(item.file, item.init);
    } else if (item.type === "lottie" && item.init && containerElement) {
      loadLottieWallpaper(item.file, item.init);
    } else if (item.type === "static" || item.type === "custom") {
      cleanupCurrentWallpaper();
    }
  });

  onMount(async () => {
    await wallpaper.init();
    webglSupported = wallpaper.isWebGLSupported();
    ready = true;
  });

  onDestroy(() => {
    cleanupCurrentWallpaper();
  });
</script>

<!-- 只在客户端就绪且有壁纸URL时渲染 -->
{#if ready && wallpaper.currentUrl}
  <div class="wallpaper-container" bind:this={containerElement}>
    {#if wallpaper.type === "webgl"}
      <!-- WebGL 动态壁纸 -->
      <canvas bind:this={canvasElement} class="wallpaper-canvas"></canvas>
    {:else if wallpaper.type === "lottie"}
      <!-- Lottie 动画壁纸 - 容器已经绑定到 containerElement -->
    {:else}
      <!-- 静态图片壁纸 -->
      <div class="wallpaper-static" style="background-image: url({wallpaper.currentUrl})"></div>
    {/if}
  </div>
{:else}
  <!-- 加载中显示背景色 -->
  <div class="wallpaper-container wallpaper-loading"></div>
{/if}

<style>
  .wallpaper-container {
    position: absolute;
    inset: 0;
    overflow: hidden;
    z-index: 0;
    background-color: #1a1a2e;
  }

  .wallpaper-static {
    width: 100%;
    height: 100%;
    background-size: cover;
    background-position: center;
    background-repeat: no-repeat;
  }

  .wallpaper-canvas {
    width: 100%;
    height: 100%;
    display: block;
  }
</style>
