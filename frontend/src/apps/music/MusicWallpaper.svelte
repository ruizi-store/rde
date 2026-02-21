<!-- MusicWallpaper.svelte - 音乐全屏壁纸模式 -->
<!-- 3D透视歌词 + 可视化特效 -->
<script lang="ts">
  import { onMount } from "svelte";
  import { t } from "svelte-i18n";
  import { musicPlayer, type VisualizerMode } from "$shared/stores/music-player.svelte";
  import { lyricsService, type LyricsData } from "./lyrics";
  import Icon from "@iconify/svelte";
  import VisualizerEffects from "./VisualizerEffects.svelte";
  import LyricsRenderer from "./LyricsRenderer.svelte";

  // 使用全局 store 的模式状态
  let visualizerMode = $derived(musicPlayer.visualizerMode);
  
  // 歌词数据
  let lyrics = $state<LyricsData | null>(null);
  let currentLineIndex = $state(-1);
  let lastTrackPath = $state("");
  
  // 可视化初始化状态
  let visualizerInitialized = $state(false);

  // 初始化可视化
  function initVisualizer() {
    if (visualizerInitialized) return;
    musicPlayer.initAudioContext();
    visualizerInitialized = true;
  }

  // 监听曲目变化加载歌词
  $effect(() => {
    const track = musicPlayer.currentTrack;
    if (track && track.path !== lastTrackPath) {
      lastTrackPath = track.path;
      loadLyrics(track.path, track.title, track.artist);
    }
  });
  
  // 监听播放进度更新当前行
  $effect(() => {
    if (lyrics && lyrics.lines.length > 0 && lyrics.format === "lrc") {
      currentLineIndex = lyricsService.getCurrentLineIndex(lyrics.lines, musicPlayer.currentTime);
      // 更新 store 中的当前歌词行（用于迷你模式）
      if (currentLineIndex >= 0 && currentLineIndex < lyrics.lines.length) {
        musicPlayer.currentLyricLine = lyrics.lines[currentLineIndex].text;
      } else {
        musicPlayer.currentLyricLine = "";
      }
    } else {
      musicPlayer.currentLyricLine = "";
    }
  });
  
  async function loadLyrics(path: string, title?: string, artist?: string) {
    lyrics = null;
    currentLineIndex = -1;
    
    try {
      lyrics = await lyricsService.getLyrics(path, title, artist);
    } catch (e) {
      console.error("加载歌词失败:", e);
    }
  }
  
  onMount(() => {
    // 自动初始化可视化
    initVisualizer();
  });
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="music-wallpaper">
  <!-- 背景渐变（半透明） -->
  <div class="background-gradient"></div>
  
  <!-- 可视化特效层 -->
  {#if visualizerMode !== "off"}
    <VisualizerEffects 
      mode={visualizerMode} 
      initialized={visualizerInitialized}
    />
  {/if}
  
  <!-- 歌词层（固定使用 3D 透视模式） -->
  {#if lyrics && lyrics.lines.length > 0}
    <LyricsRenderer
      mode="perspective"
      {lyrics}
      {currentLineIndex}
      currentTime={musicPlayer.currentTime}
    />
  {:else}
    <div class="no-lyrics">
      <Icon icon="mdi:music-note" width="48" />
      <span>{$t("music.noLyrics")}</span>
    </div>
  {/if}
  
  <!-- 简化的提示信息（不可交互，仅显示） -->
  <div class="wallpaper-hints">
    <div class="track-info">
      <div class="track-title">{musicPlayer.displayTitle || $t("music.notPlaying")}</div>
      <div class="track-artist">{musicPlayer.displayArtist}</div>
    </div>
    <div class="mode-info">
      <span class="mode-tag">
        <Icon icon={musicPlayer.getVisualizerModeIcon()} width="14" />
        {musicPlayer.getVisualizerModeName()}
      </span>
    </div>
    <div class="hints">
      <span>V {$t("music.switchEffect")}</span>
      <span>{$t("music.clickTaskbarToExit")}</span>
    </div>
  </div>
</div>

<style>
  .music-wallpaper {
    position: absolute;
    inset: 0;
    z-index: 0;
    overflow: hidden;
    user-select: none;
    pointer-events: none;
  }
  
  .background-gradient {
    position: absolute;
    inset: 0;
    background: linear-gradient(
      135deg,
      rgba(15, 12, 41, 0.85) 0%,
      rgba(48, 43, 99, 0.80) 50%,
      rgba(36, 36, 62, 0.85) 100%
    );
    animation: gradientShift 30s ease infinite;
    background-size: 400% 400%;
  }
  
  @keyframes gradientShift {
    0%, 100% { background-position: 0% 50%; }
    50% { background-position: 100% 50%; }
  }
  
  .no-lyrics {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 16px;
    color: rgba(255, 255, 255, 0.5);
    font-size: 24px;
  }
  
  .wallpaper-hints {
    position: absolute;
    bottom: 60px; /* 任务栏上方 */
    left: 50%;
    transform: translateX(-50%);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px 24px;
    background: rgba(0, 0, 0, 0.5);
    border-radius: 16px;
    backdrop-filter: blur(10px);
  }
  
  .track-info {
    text-align: center;
  }
  
  .track-title {
    font-size: 18px;
    font-weight: 600;
    color: white;
    text-shadow: 0 2px 8px rgba(0, 0, 0, 0.5);
  }
  
  .track-artist {
    font-size: 14px;
    color: rgba(255, 255, 255, 0.7);
    margin-top: 2px;
  }
  
  .mode-info {
    display: flex;
    gap: 12px;
  }
  
  .mode-tag {
    display: flex;
    align-items: center;
    gap: 4px;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.6);
    background: rgba(255, 255, 255, 0.1);
    padding: 4px 10px;
    border-radius: 12px;
  }
  
  .hints {
    display: flex;
    gap: 16px;
    font-size: 11px;
    color: rgba(255, 255, 255, 0.4);
  }
</style>
