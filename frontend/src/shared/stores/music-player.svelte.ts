// 音乐播放器状态管理
// 全局单例，支持任务栏控件和主窗口共享状态

import { fileService } from "$shared/services/files";

// 音乐曲目信息
export interface Track {
  id: string;
  path: string; // 文件路径
  name: string; // 文件名
  title?: string; // 曲目标题（从元数据解析）
  artist?: string; // 艺术家
  album?: string; // 专辑
  duration?: number; // 时长（秒）
  coverUrl?: string; // 封面 URL
}

// 播放模式
export type PlayMode = "sequence" | "repeat" | "repeat-one" | "shuffle";

// 可视化模式 (窗口模式和壁纸模式下的特效)
// off = 普通桌面（无壁纸模式），其他 = 壁纸模式
export type VisualizerMode = "bars" | "circle" | "kaleidoscope" | "off";

// 可视化模式列表
const VISUALIZER_MODES: VisualizerMode[] = ["bars", "circle", "kaleidoscope", "off"];

// 支持的音频格式
const AUDIO_EXTENSIONS = [".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a", ".wma", ".webm"];

export function isAudioFile(filename: string): boolean {
  const ext = filename.toLowerCase().slice(filename.lastIndexOf("."));
  return AUDIO_EXTENSIONS.includes(ext);
}

class MusicPlayerStore {
  // 播放列表
  playlist = $state<Track[]>([]);
  
  // 当前播放索引
  currentIndex = $state(-1);
  
  // 播放状态
  isPlaying = $state(false);
  
  // 当前播放时间（秒）
  currentTime = $state(0);
  
  // 总时长（秒）
  duration = $state(0);
  
  // 音量 (0-1)
  volume = $state(1);
  
  // 是否静音
  isMuted = $state(false);
  
  // 播放模式
  playMode = $state<PlayMode>("sequence");
  
  // 可视化模式 (off = 普通桌面，其他 = 壁纸模式)
  visualizerMode = $state<VisualizerMode>("off");
  
  // 壁纸模式: visualizerMode !== "off" 时激活
  get wallpaperMode(): boolean {
    return this.visualizerMode !== "off";
  }
  
  // 当前歌词行文本（用于迷你模式在任务栏显示）
  currentLyricLine = $state("");
  
  // 是否激活（有曲目在播放/暂停）
  get isActive(): boolean {
    return this.currentTrack !== null;
  }
  
  // 当前曲目
  get currentTrack(): Track | null {
    if (this.currentIndex >= 0 && this.currentIndex < this.playlist.length) {
      return this.playlist[this.currentIndex];
    }
    return null;
  }
  
  // 显示标题（优先使用元数据标题）
  get displayTitle(): string {
    const track = this.currentTrack;
    if (!track) return "";
    return track.title || track.name.replace(/\.[^.]+$/, "");
  }
  
  // 显示艺术家
  get displayArtist(): string {
    return this.currentTrack?.artist || "未知艺术家";
  }
  
  // Audio 元素
  private audio: HTMLAudioElement | null = null;
  
  // Web Audio API (用于可视化)
  private audioContext: AudioContext | null = null;
  private analyser: AnalyserNode | null = null;
  private sourceNode: MediaElementAudioSourceNode | null = null;
  private audioContextInitialized = false;
  
  // 随机播放历史（用于 shuffle 模式的回退）
  private shuffleHistory: number[] = [];
  
  constructor() {
    if (typeof window !== "undefined") {
      this.initAudio();
      this.loadState();
    }
  }
  
  private initAudio() {
    this.audio = new Audio();
    this.audio.volume = this.volume;
    
    // 监听事件
    this.audio.addEventListener("timeupdate", () => {
      this.currentTime = this.audio?.currentTime || 0;
    });
    
    this.audio.addEventListener("durationchange", () => {
      this.duration = this.audio?.duration || 0;
    });
    
    this.audio.addEventListener("ended", () => {
      this.handleTrackEnd();
    });
    
    this.audio.addEventListener("play", () => {
      this.isPlaying = true;
    });
    
    this.audio.addEventListener("pause", () => {
      this.isPlaying = false;
    });
    
    this.audio.addEventListener("error", (e) => {
      console.error("音频播放错误:", e);
      this.isPlaying = false;
    });
  }
  
  // 保存状态到 localStorage
  private saveState() {
    if (typeof localStorage === "undefined") return;
    
    const state = {
      playlist: this.playlist,
      currentIndex: this.currentIndex,
      volume: this.volume,
      playMode: this.playMode,
      currentTime: this.currentTime,
    };
    localStorage.setItem("music_player_state", JSON.stringify(state));
  }
  
  // 从 localStorage 恢复状态
  private loadState() {
    if (typeof localStorage === "undefined") return;
    
    try {
      const saved = localStorage.getItem("music_player_state");
      if (saved) {
        const state = JSON.parse(saved);
        this.playlist = state.playlist || [];
        this.currentIndex = state.currentIndex ?? -1;
        this.volume = state.volume ?? 1;
        this.playMode = state.playMode || "sequence";
        
        if (this.audio) {
          this.audio.volume = this.volume;
        }
        
        // 恢复曲目（不自动播放）
        if (this.currentTrack) {
          this.loadTrack(this.currentTrack, false);
          // 恢复播放位置
          if (state.currentTime && this.audio) {
            this.audio.currentTime = state.currentTime;
          }
        }
      }
    } catch (e) {
      console.error("恢复音乐播放器状态失败:", e);
    }
  }
  
  // 加载曲目
  private loadTrack(track: Track, autoPlay: boolean = true) {
    if (!this.audio) return;
    
    const url = fileService.getPreviewUrl(track.path);
    this.audio.src = url;
    this.audio.load();
    
    if (autoPlay) {
      this.audio.play().catch(console.error);
    }
    
    this.saveState();
  }
  
  // 播放/暂停切换
  toggle() {
    if (!this.audio) return;
    
    if (this.isPlaying) {
      this.audio.pause();
    } else {
      this.audio.play().catch(console.error);
    }
  }
  
  // 播放
  play() {
    this.audio?.play().catch(console.error);
  }
  
  // 暂停
  pause() {
    this.audio?.pause();
  }
  
  // 停止
  stop() {
    if (this.audio) {
      this.audio.pause();
      this.audio.currentTime = 0;
    }
    this.isPlaying = false;
  }
  
  // 上一曲
  prev() {
    if (this.playlist.length === 0) return;
    
    if (this.playMode === "shuffle" && this.shuffleHistory.length > 1) {
      // 从历史中回退
      this.shuffleHistory.pop(); // 移除当前
      const prevIndex = this.shuffleHistory[this.shuffleHistory.length - 1];
      this.currentIndex = prevIndex;
    } else {
      this.currentIndex = (this.currentIndex - 1 + this.playlist.length) % this.playlist.length;
    }
    
    if (this.currentTrack) {
      this.loadTrack(this.currentTrack);
    }
  }
  
  // 下一曲
  next() {
    if (this.playlist.length === 0) return;
    
    if (this.playMode === "shuffle") {
      // 随机选择（排除当前曲目）
      const available = this.playlist
        .map((_, i) => i)
        .filter((i) => i !== this.currentIndex);
      
      if (available.length > 0) {
        this.currentIndex = available[Math.floor(Math.random() * available.length)];
        this.shuffleHistory.push(this.currentIndex);
      }
    } else {
      this.currentIndex = (this.currentIndex + 1) % this.playlist.length;
    }
    
    if (this.currentTrack) {
      this.loadTrack(this.currentTrack);
    }
  }
  
  // 曲目结束处理
  private handleTrackEnd() {
    switch (this.playMode) {
      case "repeat-one":
        // 单曲循环
        if (this.audio) {
          this.audio.currentTime = 0;
          this.audio.play().catch(console.error);
        }
        break;
      case "repeat":
        // 列表循环
        this.next();
        break;
      case "shuffle":
        // 随机播放
        this.next();
        break;
      case "sequence":
      default:
        // 顺序播放
        if (this.currentIndex < this.playlist.length - 1) {
          this.next();
        } else {
          // 播放完毕
          this.isPlaying = false;
        }
        break;
    }
  }
  
  // 跳转到指定位置
  seek(time: number) {
    if (this.audio) {
      const newTime = Math.max(0, Math.min(time, this.duration));
      this.audio.currentTime = newTime;
      this.currentTime = newTime; // 立即更新状态，避免 UI 回弹
      this.saveState();
    }
  }
  
  // 设置音量
  setVolume(vol: number) {
    this.volume = Math.max(0, Math.min(1, vol));
    if (this.audio) {
      this.audio.volume = this.volume;
    }
    this.isMuted = this.volume === 0;
    this.saveState();
  }
  
  // 静音切换
  toggleMute() {
    this.isMuted = !this.isMuted;
    if (this.audio) {
      this.audio.muted = this.isMuted;
    }
  }
  
  // 切换播放模式
  togglePlayMode() {
    const modes: PlayMode[] = ["sequence", "repeat", "repeat-one", "shuffle"];
    const currentIdx = modes.indexOf(this.playMode);
    this.playMode = modes[(currentIdx + 1) % modes.length];
    
    if (this.playMode === "shuffle") {
      this.shuffleHistory = [this.currentIndex];
    }
    
    this.saveState();
  }
  
  // 播放指定曲目
  playTrack(index: number) {
    if (index < 0 || index >= this.playlist.length) return;
    
    this.currentIndex = index;
    if (this.playMode === "shuffle") {
      this.shuffleHistory.push(index);
    }
    
    if (this.currentTrack) {
      this.loadTrack(this.currentTrack);
    }
  }
  
  // 添加曲目到播放列表
  addTrack(track: Track) {
    this.playlist.push(track);
    this.saveState();
  }
  
  // 添加多个曲目（自动去重）
  addTracks(tracks: Track[]) {
    const existingPaths = new Set(this.playlist.map(t => t.path));
    const newTracks = tracks.filter(t => !existingPaths.has(t.path));
    if (newTracks.length > 0) {
      this.playlist.push(...newTracks);
      this.saveState();
    }
  }
  
  // 从文件路径创建曲目
  createTrackFromPath(path: string, name: string): Track {
    return {
      id: `${Date.now()}-${Math.random().toString(36).slice(2)}`,
      path,
      name,
      title: name.replace(/\.[^.]+$/, ""),
    };
  }
  
  // 清空播放列表并播放新列表
  playNewList(tracks: Track[], startIndex: number = 0) {
    if (tracks.length === 0) return;
    
    this.playlist = tracks;
    this.currentIndex = startIndex;
    this.shuffleHistory = [startIndex];
    
    // 直接使用 tracks 数组，避免 getter 更新延迟
    const track = tracks[startIndex];
    if (track) {
      this.loadTrack(track);
    }
  }
  
  // 移除曲目
  removeTrack(index: number) {
    if (index < 0 || index >= this.playlist.length) return;
    
    const wasPlaying = this.isPlaying && index === this.currentIndex;
    
    this.playlist.splice(index, 1);
    
    if (index < this.currentIndex) {
      this.currentIndex--;
    } else if (index === this.currentIndex) {
      if (this.playlist.length === 0) {
        this.currentIndex = -1;
        this.stop();
      } else {
        this.currentIndex = Math.min(this.currentIndex, this.playlist.length - 1);
        if (wasPlaying && this.currentTrack) {
          this.loadTrack(this.currentTrack);
        }
      }
    }
    
    this.saveState();
  }
  
  // 清空播放列表
  clearPlaylist() {
    this.playlist = [];
    this.currentIndex = -1;
    this.stop();
    this.shuffleHistory = [];
    this.saveState();
  }
  
  // ========== 音频可视化 ==========
  
  // 初始化 AudioContext（需要用户交互后调用）
  initAudioContext() {
    if (this.audioContextInitialized || !this.audio) return;
    
    try {
      this.audioContext = new AudioContext();
      this.analyser = this.audioContext.createAnalyser();
      this.analyser.fftSize = 256;
      this.analyser.smoothingTimeConstant = 0.8;
      
      this.sourceNode = this.audioContext.createMediaElementSource(this.audio);
      this.sourceNode.connect(this.analyser);
      this.analyser.connect(this.audioContext.destination);
      
      this.audioContextInitialized = true;
    } catch (e) {
      console.error("初始化 AudioContext 失败:", e);
    }
  }
  
  // 获取频谱数据（用于可视化）
  getFrequencyData(): Uint8Array | null {
    if (!this.analyser) return null;
    
    const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
    this.analyser.getByteFrequencyData(dataArray);
    return dataArray;
  }
  
  // 获取波形数据
  getWaveformData(): Uint8Array | null {
    if (!this.analyser) return null;
    
    const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
    this.analyser.getByteTimeDomainData(dataArray);
    return dataArray;
  }
  
  // 获取分析器节点数量
  get analyserBinCount(): number {
    return this.analyser?.frequencyBinCount || 0;
  }
  
  // 检查可视化是否可用
  get visualizerReady(): boolean {
    return this.audioContextInitialized && this.analyser !== null;
  }
  
  // 切换壁纸模式
  // 切换壁纸模式 (通过切换 visualizerMode)
  toggleWallpaperMode() {
    if (this.visualizerMode === "off") {
      this.visualizerMode = "circle";
    } else {
      this.visualizerMode = "off";
    }
  }
  
  // 进入壁纸模式
  enterWallpaperMode() {
    if (this.visualizerMode === "off") {
      this.visualizerMode = "circle";
    }
  }
  
  // 退出壁纸模式（关闭特效回到普通桌面）
  exitWallpaperMode() {
    this.visualizerMode = "off";
  }
  
  // 完全退出播放器
  exit() {
    this.stop();
    this.clearPlaylist();
    this.visualizerMode = "off";
    this.currentLyricLine = "";
  }
  
  // 切换可视化模式（循环切换，off 时回到普通桌面）
  cycleVisualizerMode() {
    const idx = VISUALIZER_MODES.indexOf(this.visualizerMode);
    this.visualizerMode = VISUALIZER_MODES[(idx + 1) % VISUALIZER_MODES.length];
  }
  
  // 获取可视化模式名称
  getVisualizerModeName(): string {
    const names: Record<VisualizerMode, string> = {
      bars: "频谱柱",
      circle: "环形",
      kaleidoscope: "万花筒",
      off: "关闭",
    };
    return names[this.visualizerMode];
  }
  
  // 获取可视化模式图标
  getVisualizerModeIcon(): string {
    const icons: Record<VisualizerMode, string> = {
      bars: "mdi:chart-bar",
      circle: "mdi:circle-outline",
      kaleidoscope: "mdi:flower",
      off: "mdi:eye-off",
    };
    return icons[this.visualizerMode];
  }
  
  // 格式化时间
  formatTime(seconds: number): string {
    if (!isFinite(seconds) || seconds < 0) return "0:00";
    const mins = Math.floor(seconds / 60);
    const secs = Math.floor(seconds % 60);
    return `${mins}:${secs.toString().padStart(2, "0")}`;
  }
  
  // 获取播放模式图标
  getPlayModeIcon(): string {
    switch (this.playMode) {
      case "repeat":
        return "mdi:repeat";
      case "repeat-one":
        return "mdi:repeat-once";
      case "shuffle":
        return "mdi:shuffle";
      default:
        return "mdi:repeat-off";
    }
  }
  
  // 获取播放模式名称
  getPlayModeName(): string {
    switch (this.playMode) {
      case "repeat":
        return "列表循环";
      case "repeat-one":
        return "单曲循环";
      case "shuffle":
        return "随机播放";
      default:
        return "顺序播放";
    }
  }
}

// 导出单例
export const musicPlayer = new MusicPlayerStore();
