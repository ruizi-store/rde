<script lang="ts">
  import Icon from "@iconify/svelte";

  interface Props {
    src: string;
    alt: string;
    fallbackIcon?: string;
    isVideo?: boolean;
    size?: number;
  }

  let { src, alt, fallbackIcon = "mdi:file", isVideo = false, size = 80 }: Props = $props();

  let loaded = $state(false);
  let error = $state(false);
  let visible = $state(false);
  let containerRef: HTMLDivElement | null = $state(null);

  // 使用 Intersection Observer 实现懒加载
  $effect(() => {
    if (!containerRef) return;

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            visible = true;
            observer.disconnect();
          }
        });
      },
      {
        rootMargin: "100px", // 提前 100px 开始加载
        threshold: 0.01,
      }
    );

    observer.observe(containerRef);

    return () => observer.disconnect();
  });

  function handleLoad() {
    loaded = true;
  }

  function handleError() {
    error = true;
  }
</script>

<div bind:this={containerRef} class="lazy-thumbnail" style="width: {size}px; height: {size}px;">
  {#if error}
    <!-- 加载失败显示图标 -->
    <div class="fallback">
      <Icon icon={fallbackIcon} width={size * 0.6} />
    </div>
  {:else if visible}
    <!-- 骨架屏占位 -->
    {#if !loaded}
      <div class="skeleton"></div>
    {/if}
    
    <!-- 图片元素 -->
    <img
      src={src}
      {alt}
      class="thumbnail"
      class:loaded
      onload={handleLoad}
      onerror={handleError}
    />
    
    {#if isVideo && loaded}
      <div class="video-indicator">
        <Icon icon="mdi:play-circle" width={Math.max(20, size * 0.3)} />
      </div>
    {/if}
  {:else}
    <!-- 等待进入视口时显示骨架屏 -->
    <div class="skeleton"></div>
  {/if}
</div>

<style>
  .lazy-thumbnail {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    overflow: hidden;
    border-radius: 6px;
    background: var(--color-surface-2, #f0f0f0);
  }

  .skeleton {
    position: absolute;
    inset: 0;
    background: linear-gradient(
      90deg,
      var(--color-surface-2, #f0f0f0) 25%,
      var(--color-surface-3, #e0e0e0) 50%,
      var(--color-surface-2, #f0f0f0) 75%
    );
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
  }

  @keyframes shimmer {
    0% {
      background-position: 200% 0;
    }
    100% {
      background-position: -200% 0;
    }
  }

  .thumbnail {
    width: 100%;
    height: 100%;
    object-fit: cover;
    opacity: 0;
    transition: opacity 0.3s ease;
  }

  .thumbnail.loaded {
    opacity: 1;
  }

  .fallback {
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--color-text-secondary, #666);
  }

  .video-indicator {
    position: absolute;
    bottom: 4px;
    right: 4px;
    background: rgba(0, 0, 0, 0.6);
    border-radius: 50%;
    padding: 2px;
    color: white;
    display: flex;
    align-items: center;
    justify-content: center;
  }
</style>
