<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import { lyricsService, type LyricLine, type LyricsData } from "./lyrics";
  import { musicPlayer } from "$shared/stores/music-player.svelte";
  import Icon from "@iconify/svelte";
  import { Spinner } from "$shared/ui";
  import AudioVisualizer from "./AudioVisualizer.svelte";
  import VisualizerEffects from "./VisualizerEffects.svelte";

  // 歌词数据
  let lyrics = $state<LyricsData | null>(null);
  let loading = $state(false);
  let error = $state("");
  
  // 当前高亮行
  let currentLineIndex = $state(-1);
  
  // 滚动容器
  let container: HTMLDivElement | null = null;
  let autoScroll = $state(true);
  
  // 是否显示特效
  let showVisualizer = $derived(musicPlayer.visualizerMode !== "off");
  
  // 监听当前曲目变化
  let lastTrackPath = $state("");
  
  $effect(() => {
    const track = musicPlayer.currentTrack;
    if (track && track.path !== lastTrackPath) {
      lastTrackPath = track.path;
      loadLyrics(track.path, track.title, track.artist);
    }
  });
  
  // 监听播放进度，更新当前行
  $effect(() => {
    if (lyrics && lyrics.lines.length > 0 && lyrics.format === "lrc") {
      const newIndex = lyricsService.getCurrentLineIndex(lyrics.lines, musicPlayer.currentTime);
      if (newIndex !== currentLineIndex) {
        currentLineIndex = newIndex;
        if (autoScroll && container) {
          scrollToLine(newIndex);
        }
        // 更新 store 中的当前歌词行（用于迷你模式）
        if (newIndex >= 0 && newIndex < lyrics.lines.length) {
          musicPlayer.currentLyricLine = lyrics.lines[newIndex].text;
        } else {
          musicPlayer.currentLyricLine = "";
        }
      }
    } else {
      if (musicPlayer.currentLyricLine && !lyrics) {
        musicPlayer.currentLyricLine = "";
      }
    }
  });
  
  async function loadLyrics(path: string, title?: string, artist?: string) {
    loading = true;
    error = "";
    lyrics = null;
    currentLineIndex = -1;
    
    try {
      lyrics = await lyricsService.getLyrics(path, title, artist);
    } catch (e) {
      error = e instanceof Error ? e.message : $t('music.loadLyricsFailed');
    } finally {
      loading = false;
    }
  }
  
  function scrollToLine(index: number) {
    if (!container || index < 0) return;
    
    const lineElement = container.querySelector(`[data-line-index="${index}"]`);
    if (lineElement) {
      lineElement.scrollIntoView({
        behavior: "smooth",
        block: "center",
      });
    }
  }
  
  // 点击歌词行跳转
  function handleLineClick(line: LyricLine) {
    if (line.time >= 0) {
      musicPlayer.seek(line.time);
      autoScroll = true;
    }
  }
  
  // 手动滚动时暂停自动滚动
  function handleScroll() {
    // 用户手动滚动时暂停自动滚动
    // 3秒后恢复
    autoScroll = false;
    setTimeout(() => {
      autoScroll = true;
    }, 3000);
  }
  
  function getSourceText(source: string): string {
    switch (source) {
      case "embedded": return $t('music.embeddedLyrics');
      case "online": return $t('music.onlineLyrics');
      case "lrc_file": return $t('music.lrcFile');
      default: return "";
    }
  }
</script>

<div class="lyrics-container">
  <!-- 特效背景层 -->
  {#if showVisualizer}
    <div class="visualizer-background">
      <VisualizerEffects mode={musicPlayer.visualizerMode} initialized={musicPlayer.visualizerReady} />
    </div>
  {/if}
  
  <!-- 音频可视化（顶部小型频谱） -->
  <div class="visualizer-section">
    <AudioVisualizer mode="bars" height={60} />
  </div>
  
  {#if loading}
    <div class="loading">
      <Spinner size="md" />
      <span>{$t('music.loadingLyrics')}</span>
    </div>
  {:else if error}
    <div class="error">
      <Icon icon="mdi:alert-circle-outline" width="32" />
      <span>{error}</span>
    </div>
  {:else if !lyrics || lyrics.source === "none" || lyrics.lines.length === 0}
    <div class="no-lyrics">
      <Icon icon="mdi:music-note-off" width="48" />
      <span>{$t('music.noLyrics')}</span>
    </div>
  {:else}
    <div class="lyrics-header">
      <span class="source">{getSourceText(lyrics.source)}</span>
    </div>
    <div
      class="lyrics-content"
      bind:this={container}
      onscroll={handleScroll}
    >
      {#each lyrics.lines as line, index}
        <div
          class="lyric-line"
          class:active={index === currentLineIndex}
          class:past={index < currentLineIndex}
          class:plain={lyrics.format !== "lrc"}
          data-line-index={index}
          onclick={() => handleLineClick(line)}
          role="button"
          tabindex={0}
        >
          <span class="text">{line.text}</span>
          {#if line.translation}
            <span class="translation">{line.translation}</span>
          {/if}
        </div>
      {/each}
      <!-- 底部填充，确保最后几行可以滚动到中间 -->
      <div class="bottom-padding"></div>
    </div>
  {/if}
</div>

<style>
  .lyrics-container {
    display: flex;
    flex-direction: column;
    height: 100%;
    overflow: hidden;
    position: relative;
  }
  
  .visualizer-background {
    position: absolute;
    inset: 0;
    z-index: 0;
    opacity: 0.3;
    pointer-events: none;
  }
  
  .visualizer-section {
    position: relative;
    z-index: 1;
    flex-shrink: 0;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    background: linear-gradient(to bottom, rgba(0, 0, 0, 0.2), transparent);
    
    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.1);
      background: linear-gradient(to bottom, rgba(255, 255, 255, 0.5), transparent);
    }
  }
  
  .loading,
  .error,
  .no-lyrics {
    position: relative;
    z-index: 1;
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    color: var(--text-secondary, rgba(255, 255, 255, 0.6));
    
    :global([data-theme="light"]) & {
      color: rgba(0, 0, 0, 0.5);
    }
  }
  
  .error {
    color: var(--color-error, #ef4444);
  }
  
  .lyrics-header {
    position: relative;
    z-index: 1;
    padding: 8px 16px;
    border-bottom: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
    
    :global([data-theme="light"]) & {
      border-bottom-color: rgba(0, 0, 0, 0.1);
    }
  }
  
  .source {
    font-size: 11px;
    opacity: 0.5;
  }
  
  .lyrics-content {
    position: relative;
    z-index: 1;
    flex: 1;
    overflow-y: auto;
    padding: 40px 16px;
    scroll-behavior: smooth;
    
    /* 隐藏滚动条 */
    scrollbar-width: none;
    &::-webkit-scrollbar {
      display: none;
    }
  }
  
  .lyric-line {
    padding: 12px 16px;
    margin: 4px 0;
    border-radius: 8px;
    text-align: center;
    cursor: pointer;
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
    transform-origin: center;
    
    display: flex;
    flex-direction: column;
    gap: 4px;
    
    &:hover {
      background: rgba(255, 255, 255, 0.05);
      
      :global([data-theme="light"]) & {
        background: rgba(0, 0, 0, 0.03);
      }
    }
    
    &.plain {
      cursor: default;
    }
    
    &.past {
      opacity: 0.4;
      transform: scale(0.95) translateY(-2px);
    }
    
    &.active {
      opacity: 1;
      transform: scale(1.05);
      background: linear-gradient(90deg, 
        rgba(74, 144, 217, 0.1) 0%, 
        rgba(139, 92, 246, 0.1) 50%, 
        rgba(236, 72, 153, 0.1) 100%
      );
      border-radius: 8px;
      animation: pulse 2s ease-in-out infinite;
      
      .text {
        color: transparent;
        background: linear-gradient(90deg, #4a90d9, #8b5cf6, #ec4899);
        background-clip: text;
        -webkit-background-clip: text;
        font-size: 20px;
        font-weight: 700;
        text-shadow: 
          0 0 10px rgba(74, 144, 217, 0.5),
          0 0 20px rgba(139, 92, 246, 0.3),
          0 0 30px rgba(236, 72, 153, 0.2);
        animation: glow 2s ease-in-out infinite alternate;
      }
    }
  }
  
  @keyframes pulse {
    0%, 100% {
      box-shadow: 0 0 0 0 rgba(74, 144, 217, 0.1);
    }
    50% {
      box-shadow: 0 0 20px 5px rgba(74, 144, 217, 0.2);
    }
  }
  
  @keyframes glow {
    0% {
      filter: brightness(1);
    }
    100% {
      filter: brightness(1.2);
    }
  }
  
  .text {
    font-size: 15px;
    line-height: 1.6;
    transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
  }
  
  .translation {
    font-size: 12px;
    opacity: 0.6;
  }
  
  .bottom-padding {
    height: 40vh;
  }
</style>
