<!-- LyricsRenderer.svelte - 歌词渲染器 -->
<!-- 支持: perspective(3D透视), scroll(滚动) -->
<script lang="ts">
  import type { LyricLine, LyricsData } from "./lyrics";

  // 歌词显示模式
  type LyricsMode = "perspective" | "scroll";

  let { 
    mode = "perspective" as LyricsMode,
    lyrics,
    currentLineIndex = -1,
    currentTime = 0,
    onlineclick,
  }: {
    mode?: LyricsMode;
    lyrics: LyricsData;
    currentLineIndex: number;
    currentTime: number;
    onlineclick?: (line: LyricLine) => void;
  } = $props();

  // 滚动容器引用
  let scrollContainer: HTMLDivElement | null = null;
  let autoScroll = $state(true);
  let scrollTimeout: ReturnType<typeof setTimeout> | null = null;

  // 监听当前行变化，自动滚动
  $effect(() => {
    if (mode === "scroll" && autoScroll && scrollContainer && currentLineIndex >= 0) {
      scrollToLine(currentLineIndex);
    }
  });

  function scrollToLine(index: number) {
    if (!scrollContainer) return;
    const lineEl = scrollContainer.querySelector(`[data-line="${index}"]`);
    if (lineEl) {
      lineEl.scrollIntoView({ behavior: "smooth", block: "center" });
    }
  }

  function handleScroll() {
    // 用户滚动时暂停自动滚动
    autoScroll = false;
    if (scrollTimeout) clearTimeout(scrollTimeout);
    scrollTimeout = setTimeout(() => {
      autoScroll = true;
    }, 3000);
  }

  function handleLineClick(line: LyricLine) {
    onlineclick?.(line);
    autoScroll = true;
  }

  // 获取3D透视模式下每行的样式
  function getPerspectiveStyle(index: number): string {
    const offset = index - currentLineIndex;
    const absOffset = Math.abs(offset);
    
    // Z轴距离
    const z = -absOffset * 100;
    // Y轴位置
    const y = offset * 60;
    // 透明度
    const opacity = Math.max(0, 1 - absOffset * 0.25);
    // 缩放
    const scale = Math.max(0.6, 1 - absOffset * 0.1);
    // 模糊
    const blur = absOffset * 2;
    
    return `transform: translateY(${y}px) translateZ(${z}px) scale(${scale}); 
            opacity: ${opacity}; 
            filter: blur(${blur}px);`;
  }
</script>

<div class="lyrics-renderer" class:perspective={mode === "perspective"} class:scroll={mode === "scroll"}>
  {#if mode === "perspective"}
    <!-- 3D透视模式：歌词从远到近滚动 -->
    <div class="perspective-container">
      <div class="perspective-wrapper">
        {#each lyrics.lines as line, i}
          <div 
            class="perspective-line"
            class:active={i === currentLineIndex}
            style={getPerspectiveStyle(i)}
            data-line={i}
            onclick={() => handleLineClick(line)}
            role="button"
            tabindex="0"
          >
            {line.text}
          </div>
        {/each}
      </div>
    </div>
  {:else if mode === "scroll"}
    <!-- 滚动模式：多行歌词，当前行高亮放大 -->
    <div 
      class="scroll-container" 
      bind:this={scrollContainer}
      onscroll={handleScroll}
    >
      <div class="scroll-spacer"></div>
      {#each lyrics.lines as line, i}
        <div 
          class="scroll-line"
          class:active={i === currentLineIndex}
          class:near={Math.abs(i - currentLineIndex) <= 2}
          class:passed={i < currentLineIndex}
          data-line={i}
          onclick={() => handleLineClick(line)}
          role="button"
          tabindex="0"
        >
          {line.text}
        </div>
      {/each}
      <div class="scroll-spacer"></div>
    </div>
  {/if}
</div>

<style>
  .lyrics-renderer {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    pointer-events: none;
    padding: 100px 50px;
  }

  /* 3D透视模式 */
  .perspective-container {
    width: 100%;
    height: 100%;
    perspective: 800px;
    perspective-origin: center center;
  }

  .perspective-wrapper {
    position: relative;
    width: 100%;
    height: 100%;
    transform-style: preserve-3d;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
  }

  .perspective-line {
    position: absolute;
    text-align: center;
    font-size: 32px;
    color: rgba(255, 255, 255, 0.8);
    cursor: pointer;
    pointer-events: auto;
    white-space: nowrap;
    transition: all 0.5s ease;
    will-change: transform, opacity, filter;
  }

  .perspective-line.active {
    font-size: 48px;
    font-weight: 600;
    color: white;
    text-shadow: 
      0 0 20px rgba(74, 144, 217, 0.8),
      0 0 40px rgba(139, 92, 246, 0.5);
  }

  .perspective-line:hover {
    color: white;
  }

  /* 滚动模式 */
  .scroll-container {
    max-height: 100%;
    overflow-y: auto;
    scroll-behavior: smooth;
    pointer-events: auto;
    padding: 0 50px;
    scrollbar-width: none;
  }

  .scroll-container::-webkit-scrollbar {
    display: none;
  }

  .scroll-spacer {
    height: 40vh;
  }

  .scroll-line {
    text-align: center;
    font-size: 24px;
    color: rgba(255, 255, 255, 0.35);
    padding: 12px 0;
    cursor: pointer;
    transition: all 0.4s ease;
    transform: scale(0.95);
  }

  .scroll-line:hover {
    color: rgba(255, 255, 255, 0.6);
  }

  .scroll-line.near {
    color: rgba(255, 255, 255, 0.5);
  }

  .scroll-line.passed {
    color: rgba(255, 255, 255, 0.25);
  }

  .scroll-line.active {
    font-size: 36px;
    font-weight: 600;
    color: white;
    transform: scale(1);
    text-shadow: 
      0 0 20px rgba(74, 222, 128, 0.6),
      0 0 40px rgba(34, 211, 238, 0.4);
    padding: 20px 0;
  }
</style>
