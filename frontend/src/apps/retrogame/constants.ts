/**
 * RetroGame 常量配置
 */

import type { PlatformInfo, KeyMapping, VideoFilter } from "./types";

// 支持的平台配置
// 核心名称必须与 EmulatorJS CDN 上的实际核心名称匹配
export const PLATFORMS: PlatformInfo[] = [
  {
    id: "nes",
    name: "NES / FC",
    core: "fceumm",
    extensions: [".nes"],
    icon: "mdi:gamepad-variant",
  },
  {
    id: "snes",
    name: "SNES / SFC",
    core: "snes9x",
    extensions: [".smc", ".sfc"],
    icon: "mdi:gamepad-square",
  },
  {
    id: "gb",
    name: "Game Boy",
    core: "gambatte",
    extensions: [".gb"],
    icon: "mdi:gamepad",
  },
  {
    id: "gbc",
    name: "Game Boy Color",
    core: "gambatte",
    extensions: [".gbc"],
    icon: "mdi:gamepad",
  },
  {
    id: "gba",
    name: "Game Boy Advance",
    core: "mgba",
    extensions: [".gba"],
    icon: "mdi:gamepad-round",
  },
  {
    id: "n64",
    name: "Nintendo 64",
    core: "mupen64plus_next",
    extensions: [".n64", ".z64", ".v64"],
    icon: "mdi:controller",
  },
  {
    id: "nds",
    name: "Nintendo DS",
    core: "melonds",
    extensions: [".nds"],
    icon: "mdi:nintendo-switch",
  },
  {
    id: "psx",
    name: "PlayStation",
    core: "pcsx_rearmed",
    extensions: [".bin", ".cue", ".iso", ".pbp"],
    needsBios: true,
    biosFile: "scph1001.bin",
    icon: "mdi:sony-playstation",
  },
  {
    id: "psp",
    name: "PlayStation Portable",
    core: "ppsspp",
    extensions: [".iso", ".cso"],
    icon: "mdi:sony-playstation",
  },
  {
    id: "genesis",
    name: "Sega Genesis / MD",
    core: "genesis_plus_gx",
    extensions: [".md", ".gen", ".bin"],
    icon: "mdi:controller-classic",
  },
  {
    id: "saturn",
    name: "Sega Saturn",
    core: "yabause",
    extensions: [".bin", ".cue", ".iso"],
    needsBios: true,
    biosFile: "saturn_bios.bin",
    icon: "mdi:controller-classic",
  },
  {
    id: "arcade",
    name: "Arcade",
    core: "fbneo",
    extensions: [".zip"],
    icon: "mdi:space-invaders",
  },
];

// 平台 ID 到信息的映射
export const PLATFORM_MAP = new Map(PLATFORMS.map((p) => [p.id, p]));

// 根据文件扩展名获取平台
export function getPlatformByExtension(filename: string): PlatformInfo | undefined {
  const ext = filename.toLowerCase().slice(filename.lastIndexOf("."));
  return PLATFORMS.find((p) => p.extensions.includes(ext));
}

// 默认按键映射
export const DEFAULT_KEY_MAPPING: KeyMapping = {
  up: "ArrowUp",
  down: "ArrowDown",
  left: "ArrowLeft",
  right: "ArrowRight",
  a: "z",
  b: "x",
  x: "a",
  y: "s",
  start: "Enter",
  select: "Shift",
  l: "q",
  r: "w",
};

// 视频滤镜选项
export const VIDEO_FILTERS: { id: VideoFilter; name: string }[] = [
  { id: "none", name: "无滤镜" },
  { id: "crt", name: "CRT 效果" },
  { id: "scanlines", name: "扫描线" },
  { id: "smooth", name: "平滑" },
  { id: "pixelated", name: "像素完美" },
];

// EmulatorJS 路径 (使用本地静态文件)
export const EMULATORJS_CDN = "/emulatorjs/";

// 默认 ROM 目录
export const DEFAULT_ROM_DIRECTORY = "~/Games/ROMs";

// 默认设置
export const DEFAULT_SETTINGS = {
  romDirectory: DEFAULT_ROM_DIRECTORY,
  autoSave: true,
  showFps: false,
  audioVolume: 100,
  videoFilter: "none" as VideoFilter,
  keyMapping: DEFAULT_KEY_MAPPING,
};
