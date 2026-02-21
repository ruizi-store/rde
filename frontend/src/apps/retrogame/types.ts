/**
 * RetroGame 类型定义
 */

// 游戏平台
export type Platform = 
  | "nes" 
  | "snes" 
  | "gb" 
  | "gbc" 
  | "gba" 
  | "n64" 
  | "nds" 
  | "psx" 
  | "psp" 
  | "genesis" 
  | "saturn" 
  | "arcade";

// 平台信息
export interface PlatformInfo {
  id: Platform;
  name: string;
  core: string;
  extensions: string[];
  needsBios?: boolean;
  biosFile?: string;
  icon: string;
}

// ROM 文件
export interface RomFile {
  name: string;
  path: string;
  size: number;
  platform: Platform;
  lastPlayed?: string;
  playCount?: number;
  coverUrl?: string;
  /** 封面候选 URL 列表（带不同区域后缀） */
  coverUrls?: string[];
  /** 本地文件对象（通过浏览器 File API 选择，跳过服务器下载） */
  localFile?: File;
}

// 游戏存档
export interface SaveState {
  id: string;
  romPath: string;
  slot: number;
  timestamp: string;
  screenshot?: string;
}

// 按键映射
export interface KeyMapping {
  up: string;
  down: string;
  left: string;
  right: string;
  a: string;
  b: string;
  x: string;
  y: string;
  start: string;
  select: string;
  l: string;
  r: string;
  l2?: string;
  r2?: string;
}

// 设置
export interface RetroGameSettings {
  romDirectory: string;
  autoSave: boolean;
  showFps: boolean;
  audioVolume: number;
  videoFilter: VideoFilter;
  keyMapping: KeyMapping;
}

// 视频滤镜
export type VideoFilter = "none" | "crt" | "scanlines" | "smooth" | "pixelated";

// 手柄状态
export interface GamepadState {
  connected: boolean;
  id: string;
  index: number;
  mapping: string;
}

// EmulatorJS 配置
export interface EmulatorConfig {
  core: string;
  gameUrl: string;
  biosUrl?: string;
  startOnLoaded?: boolean;
  fullscreenOnLoaded?: boolean;
  color?: string;
  defaultControls?: boolean;
}

// EmulatorJS 安装状态
export interface EmulatorSetupStatus {
  installed: boolean;
  version: string;
  emulator_dir: string;
}

// EmulatorJS 安装进度事件
export interface SetupProgress {
  status: "downloading" | "extracting" | "completed" | "failed";
  message: string;
  progress: number;
}
