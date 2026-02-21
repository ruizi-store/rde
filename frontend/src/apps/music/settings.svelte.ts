// 音乐播放器设置状态管理

import { fileService } from "$shared/services/files";

const STORAGE_KEY = "music_player_settings";

// 设置接口
export interface MusicSettings {
  // 默认音乐文件夹
  defaultMusicPath: string;
  // 是否已完成初始设置
  setupComplete: boolean;
  // 显示设置
  showFileExtension: boolean;
  // 播放结束后行为
  onPlaylistEnd: "stop" | "repeat" | "shuffle";
  // 记住上次播放位置
  rememberPosition: boolean;
  // 淡入淡出效果（秒）
  crossfadeDuration: number;
  // 均衡器预设
  equalizerPreset: "flat" | "bass" | "treble" | "vocal" | "custom";
  // 自动扫描音乐文件夹
  autoScan: boolean;
  // 扫描子文件夹
  scanSubfolders: boolean;
}

const defaultSettings: MusicSettings = {
  defaultMusicPath: "",
  setupComplete: false,
  showFileExtension: false,
  onPlaylistEnd: "stop",
  rememberPosition: true,
  crossfadeDuration: 0,
  equalizerPreset: "flat",
  autoScan: false,
  scanSubfolders: true,
};

class MusicSettingsStore {
  // 设置
  settings = $state<MusicSettings>({ ...defaultSettings });
  
  // 是否已加载
  loaded = $state(false);
  
  constructor() {
    if (typeof window !== "undefined") {
      this.load();
    }
  }
  
  // 从 localStorage 加载设置
  load() {
    try {
      const saved = localStorage.getItem(STORAGE_KEY);
      if (saved) {
        const parsed = JSON.parse(saved);
        this.settings = { ...defaultSettings, ...parsed };
      }
      this.loaded = true;
    } catch (e) {
      console.error("加载音乐播放器设置失败:", e);
      this.loaded = true;
    }
  }
  
  // 保存设置到 localStorage
  save() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.settings));
    } catch (e) {
      console.error("保存音乐播放器设置失败:", e);
    }
  }
  
  // 更新设置
  update(updates: Partial<MusicSettings>) {
    this.settings = { ...this.settings, ...updates };
    this.save();
  }
  
  // 标记设置完成
  completeSetup(musicPath: string) {
    this.settings.setupComplete = true;
    this.settings.defaultMusicPath = musicPath;
    this.save();
  }
  
  // 重置设置
  reset() {
    this.settings = { ...defaultSettings };
    this.save();
  }
  
  // 检测默认音乐路径
  async detectDefaultMusicPath(): Promise<string> {
    try {
      // 尝试获取用户主目录下的"音乐"文件夹
      const bookmarks = await fileService.getBookmarks();
      const homePath = bookmarks.home_path || "/";
      
      // 常见的音乐目录名称
      const musicDirNames = ["音乐", "Music", "music"];
      
      for (const dirName of musicDirNames) {
        const musicPath = homePath === "/" ? `/${dirName}` : `${homePath}/${dirName}`;
        try {
          const result = await fileService.list(musicPath, false);
          if (result.data?.content) {
            return musicPath;
          }
        } catch {
          // 目录不存在，继续尝试下一个
        }
      }
      
      // 回退到 ~/Music（即使目录不存在）
      return homePath === "/" ? "/Music" : `${homePath}/Music`;
    } catch {
      return "/Music";
    }
  }
}

// 导出单例
export const musicSettings = new MusicSettingsStore();
