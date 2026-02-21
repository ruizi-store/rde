// 视频播放器设置状态管理

import { fileService } from "$shared/services/files";

const STORAGE_KEY = "video_player_settings";
const HISTORY_KEY = "video_player_history";
const MAX_HISTORY = 50;

// 设置接口
export interface VideoSettings {
  // 默认视频文件夹
  defaultVideoPath: string;
  // 是否已完成初始设置
  setupComplete: boolean;
  // 记住播放位置
  rememberPosition: boolean;
  // 默认播放倍速
  defaultPlaybackRate: number;
  // 字幕字体大小(px)
  subtitleFontSize: number;
  // 字幕偏移(秒)
  subtitleOffset: number;
  // 自动检测字幕
  autoDetectSubtitle: boolean;
  // 自动播放下一个
  autoPlayNext: boolean;
  // 显示文件扩展名
  showFileExtension: boolean;
  // 循环模式: none | single | list
  loopMode: "none" | "single" | "list";
}

// 播放历史接口
export interface PlaybackHistory {
  path: string;
  position: number;
  duration: number;
  lastPlayed: string;
  name: string;
}

const defaultSettings: VideoSettings = {
  defaultVideoPath: "",
  setupComplete: false,
  rememberPosition: true,
  defaultPlaybackRate: 1,
  subtitleFontSize: 24,
  subtitleOffset: 0,
  autoDetectSubtitle: true,
  autoPlayNext: false,
  showFileExtension: false,
  loopMode: "none",
};

class VideoSettingsStore {
  // 设置
  settings = $state<VideoSettings>({ ...defaultSettings });
  
  // 播放历史
  history = $state<PlaybackHistory[]>([]);
  
  // 是否已加载
  loaded = $state(false);
  
  constructor() {
    if (typeof window !== "undefined") {
      this.load();
    }
  }
  
  // 从 localStorage 加载设置和历史
  load() {
    try {
      const savedSettings = localStorage.getItem(STORAGE_KEY);
      if (savedSettings) {
        const parsed = JSON.parse(savedSettings);
        this.settings = { ...defaultSettings, ...parsed };
      }
      
      const savedHistory = localStorage.getItem(HISTORY_KEY);
      if (savedHistory) {
        this.history = JSON.parse(savedHistory);
      }
      
      this.loaded = true;
    } catch (e) {
      console.error("加载视频播放器设置失败:", e);
      this.loaded = true;
    }
  }
  
  // 保存设置到 localStorage
  save() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.settings));
    } catch (e) {
      console.error("保存视频播放器设置失败:", e);
    }
  }
  
  // 保存历史到 localStorage
  saveHistory() {
    try {
      localStorage.setItem(HISTORY_KEY, JSON.stringify(this.history));
    } catch (e) {
      console.error("保存视频播放历史失败:", e);
    }
  }
  
  // 更新设置
  update(updates: Partial<VideoSettings>) {
    this.settings = { ...this.settings, ...updates };
    this.save();
  }
  
  // 标记设置完成
  completeSetup(videoPath: string) {
    this.settings.setupComplete = true;
    this.settings.defaultVideoPath = videoPath;
    this.save();
  }
  
  // 重置设置
  reset() {
    this.settings = { ...defaultSettings };
    this.save();
  }
  
  // 更新播放位置
  updatePosition(path: string, name: string, position: number, duration: number) {
    if (!this.settings.rememberPosition) return;
    
    const existingIndex = this.history.findIndex(h => h.path === path);
    const entry: PlaybackHistory = {
      path,
      name,
      position,
      duration,
      lastPlayed: new Date().toISOString(),
    };
    
    if (existingIndex >= 0) {
      this.history[existingIndex] = entry;
    } else {
      this.history.unshift(entry);
      // 限制历史数量
      if (this.history.length > MAX_HISTORY) {
        this.history = this.history.slice(0, MAX_HISTORY);
      }
    }
    
    this.saveHistory();
  }
  
  // 获取播放位置
  getPosition(path: string): number {
    if (!this.settings.rememberPosition) return 0;
    const entry = this.history.find(h => h.path === path);
    if (entry) {
      // 如果进度超过 95%，从头开始
      if (entry.duration > 0 && entry.position / entry.duration > 0.95) {
        return 0;
      }
      return entry.position;
    }
    return 0;
  }
  
  // 删除历史记录
  removeFromHistory(path: string) {
    this.history = this.history.filter(h => h.path !== path);
    this.saveHistory();
  }
  
  // 清空历史
  clearHistory() {
    this.history = [];
    this.saveHistory();
  }
  
  // 获取最近播放列表
  getRecentVideos(limit = 10): PlaybackHistory[] {
    return this.history.slice(0, limit);
  }
  
  // 检测默认视频路径
  async detectDefaultVideoPath(): Promise<string> {
    try {
      // 尝试获取用户主目录下的"视频"文件夹
      const bookmarks = await fileService.getBookmarks();
      const homePath = bookmarks.home_path || "/";
      
      // 常见的视频目录名称
      const videoDirNames = ["视频", "Videos", "videos", "Video", "video"];
      
      for (const dirName of videoDirNames) {
        const videoPath = homePath === "/" ? `/${dirName}` : `${homePath}/${dirName}`;
        try {
          const result = await fileService.list(videoPath, false);
          if (result.data?.content !== undefined) {
            return videoPath;
          }
        } catch {
          // 目录不存在，继续尝试下一个
        }
      }
      
      // 回退到 ~/Videos（即使目录不存在）
      return homePath === "/" ? "/Videos" : `${homePath}/Videos`;
    } catch {
      return "/Videos";
    }
  }
}

// 导出单例
export const videoSettings = new VideoSettingsStore();

// 视频格式判断
export function isVideoFile(filename: string): boolean {
  const ext = filename.split('.').pop()?.toLowerCase() || '';
  return ['mp4', 'webm', 'mkv', 'avi', 'mov', 'wmv', 'flv', 'm4v', 'ts', 'mts', 'ogv', '3gp'].includes(ext);
}

// 判断是否需要转码
export function needsTranscode(filename: string): boolean {
  const ext = filename.split('.').pop()?.toLowerCase() || '';
  // 浏览器原生支持的格式（H.264 容器）
  const nativeSupported = ['mp4', 'webm', 'ogv', 'm4v'];
  return !nativeSupported.includes(ext);
}
