/**
 * RetroGame Service - ROM 管理和模拟器服务
 */

import { api } from "$shared/services/api";
import type { RomFile, Platform, RetroGameSettings, SaveState, GamepadState, EmulatorSetupStatus, SetupProgress } from "./types";
import { DEFAULT_SETTINGS } from "./constants";
import { userStore } from "$shared/stores/user.svelte";

// ==================== ROM 管理 ====================

/**
 * 扫描 ROM 目录（通过后端接口，支持 ZIP 内部平台检测）
 */
export async function scanRoms(directory: string): Promise<RomFile[]> {
  try {
    // 使用后端 ROM 扫描接口，能正确识别 .zip 内的 ROM 平台
    const roms = await api.get<RomFile[]>("/retrogame/scan-roms", { path: directory });
    return roms || [];
  } catch (error) {
    console.error("扫描 ROM 失败:", error);
    return [];
  }
}

/**
 * 获取文件的访问 URL
 */
export function getFileUrl(path: string): string {
  // 将文件路径转换为 API 可访问的 URL
  const encodedPath = encodeURIComponent(path);
  return `/api/v1/files/download?path=${encodedPath}`;
}

// LibRetro 缩略图仓库中的平台名称映射
const LIBRETRO_SYSTEM_NAMES: Record<string, string> = {
  nes: "Nintendo_-_Nintendo_Entertainment_System",
  snes: "Nintendo_-_Super_Nintendo_Entertainment_System",
  gb: "Nintendo_-_Game_Boy",
  gbc: "Nintendo_-_Game_Boy_Color",
  gba: "Nintendo_-_Game_Boy_Advance",
  n64: "Nintendo_-_Nintendo_64",
  nds: "Nintendo_-_Nintendo_DS",
  psx: "Sony_-_PlayStation",
  psp: "Sony_-_PlayStation_Portable",
  genesis: "Sega_-_Mega_Drive_-_Genesis",
  saturn: "Sega_-_Saturn",
  arcade: "MAME",
};

/**
 * 获取 ROM 封面候选 URL 列表 (从 libretro-thumbnails GitHub CDN)
 * 返回多个候选，依次尝试：原名、原名+(USA)、原名+(USA, Europe)、原名+(Europe) 等
 */
export function getCoverUrls(rom: RomFile): string[] {
  const system = LIBRETRO_SYSTEM_NAMES[rom.platform];
  if (!system) return [];
  
  const safeName = rom.name
    .replace(/&/g, "_")
    .replace(/\//g, "_");
  
  const base = `https://raw.githubusercontent.com/libretro-thumbnails/${system}/master/Named_Boxarts/`;
  
  // 如果文件名已经包含区域标记 (xxx)，只用原名
  if (/\(.+\)/.test(safeName)) {
    return [base + encodeURIComponent(safeName) + ".png"];
  }
  
  // 常见区域后缀 fallback 列表
  const regionSuffixes = [
    "",
    " (USA)",
    " (USA, Europe)",
    " (Europe)",
    " (Japan)",
    " (Japan, USA)",
    " (World)",
  ];
  
  return regionSuffixes.map(suffix => 
    base + encodeURIComponent(safeName + suffix) + ".png"
  );
}

/**
 * 获取 ROM 封面 URL（兼容旧接口，返回第一个候选）
 */
export function getCoverUrl(rom: RomFile): string {
  const urls = getCoverUrls(rom);
  return urls[0] || "";
}

// ==================== 设置管理 ====================

const SETTINGS_KEY = "retrogame_settings";

/**
 * 加载设置
 */
export function loadSettings(): RetroGameSettings {
  try {
    const saved = localStorage.getItem(SETTINGS_KEY);
    if (saved) {
      return { ...DEFAULT_SETTINGS, ...JSON.parse(saved) };
    }
  } catch (error) {
    console.error("加载设置失败:", error);
  }
  return { ...DEFAULT_SETTINGS };
}

/**
 * 保存设置
 */
export function saveSettings(settings: RetroGameSettings): void {
  try {
    localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
  } catch (error) {
    console.error("保存设置失败:", error);
  }
}

// ==================== 存档管理 ====================

const SAVES_KEY = "retrogame_saves";

/**
 * 获取存档列表
 */
export function getSaveStates(romPath: string): SaveState[] {
  try {
    const saved = localStorage.getItem(SAVES_KEY);
    if (saved) {
      const all: SaveState[] = JSON.parse(saved);
      return all.filter((s) => s.romPath === romPath);
    }
  } catch (error) {
    console.error("获取存档失败:", error);
  }
  return [];
}

/**
 * 保存存档信息
 */
export function saveSaveState(saveState: SaveState): void {
  try {
    const saved = localStorage.getItem(SAVES_KEY);
    let all: SaveState[] = saved ? JSON.parse(saved) : [];

    // 更新或添加
    const index = all.findIndex((s) => s.id === saveState.id);
    if (index >= 0) {
      all[index] = saveState;
    } else {
      all.push(saveState);
    }

    localStorage.setItem(SAVES_KEY, JSON.stringify(all));
  } catch (error) {
    console.error("保存存档失败:", error);
  }
}

// ==================== 手柄支持 ====================

let gamepadListeners: ((state: GamepadState | null) => void)[] = [];

/**
 * 监听手柄连接
 */
export function onGamepadChange(callback: (state: GamepadState | null) => void): () => void {
  gamepadListeners.push(callback);

  const handleConnect = (e: GamepadEvent) => {
    callback({
      connected: true,
      id: e.gamepad.id,
      index: e.gamepad.index,
      mapping: e.gamepad.mapping,
    });
  };

  const handleDisconnect = (e: GamepadEvent) => {
    callback(null);
  };

  window.addEventListener("gamepadconnected", handleConnect);
  window.addEventListener("gamepaddisconnected", handleDisconnect);

  // 检查是否已有连接的手柄
  const gamepads = navigator.getGamepads();
  for (const gamepad of gamepads) {
    if (gamepad) {
      callback({
        connected: true,
        id: gamepad.id,
        index: gamepad.index,
        mapping: gamepad.mapping,
      });
      break;
    }
  }

  return () => {
    gamepadListeners = gamepadListeners.filter((l) => l !== callback);
    window.removeEventListener("gamepadconnected", handleConnect);
    window.removeEventListener("gamepaddisconnected", handleDisconnect);
  };
}

// ==================== 历史记录 ====================

const HISTORY_KEY = "retrogame_history";

interface PlayHistory {
  romPath: string;
  lastPlayed: string;
  playCount: number;
}

/**
 * 记录游戏历史
 */
export function recordPlay(romPath: string): void {
  try {
    const saved = localStorage.getItem(HISTORY_KEY);
    let history: PlayHistory[] = saved ? JSON.parse(saved) : [];

    const index = history.findIndex((h) => h.romPath === romPath);
    if (index >= 0) {
      history[index].lastPlayed = new Date().toISOString();
      history[index].playCount++;
    } else {
      history.push({
        romPath,
        lastPlayed: new Date().toISOString(),
        playCount: 1,
      });
    }

    // 只保留最近 100 条
    if (history.length > 100) {
      history = history.slice(-100);
    }

    localStorage.setItem(HISTORY_KEY, JSON.stringify(history));
  } catch (error) {
    console.error("记录历史失败:", error);
  }
}

/**
 * 获取最近游玩的游戏
 */
export function getRecentlyPlayed(limit = 10): PlayHistory[] {
  try {
    const saved = localStorage.getItem(HISTORY_KEY);
    if (saved) {
      const history: PlayHistory[] = JSON.parse(saved);
      return history.sort((a, b) => b.lastPlayed.localeCompare(a.lastPlayed)).slice(0, limit);
    }
  } catch (error) {
    console.error("获取历史失败:", error);
  }
  return [];
}

// ==================== 自动存档 ====================

const AUTO_SAVE_KEY = "retrogame_autosave";

/**
 * 保存自动存档（save state 二进制数据存入 localStorage via base64）
 */
export function saveAutoState(romPath: string, stateData: Uint8Array): void {
  try {
    // 转换为 base64
    let binary = "";
    const bytes = new Uint8Array(stateData);
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    const base64 = btoa(binary);
    
    const saved = localStorage.getItem(AUTO_SAVE_KEY);
    const all: Record<string, { data: string; timestamp: string }> = saved ? JSON.parse(saved) : {};
    
    all[romPath] = {
      data: base64,
      timestamp: new Date().toISOString(),
    };
    
    // 限制总数防止 localStorage 溢出（最多保留 5 个自动存档）
    const keys = Object.keys(all);
    if (keys.length > 5) {
      const sorted = keys.sort((a, b) => all[a].timestamp.localeCompare(all[b].timestamp));
      delete all[sorted[0]];
    }
    
    localStorage.setItem(AUTO_SAVE_KEY, JSON.stringify(all));
  } catch (error) {
    console.error("保存自动存档失败:", error);
  }
}

/**
 * 加载自动存档
 */
export function loadAutoState(romPath: string): Uint8Array | null {
  try {
    const saved = localStorage.getItem(AUTO_SAVE_KEY);
    if (!saved) return null;
    
    const all: Record<string, { data: string; timestamp: string }> = JSON.parse(saved);
    const entry = all[romPath];
    if (!entry) return null;
    
    const binary = atob(entry.data);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  } catch (error) {
    console.error("加载自动存档失败:", error);
    return null;
  }
}

/**
 * 检查是否有自动存档
 */
export function hasAutoState(romPath: string): boolean {
  try {
    const saved = localStorage.getItem(AUTO_SAVE_KEY);
    if (!saved) return false;
    const all = JSON.parse(saved);
    return !!all[romPath];
  } catch {
    return false;
  }
}

/**
 * 删除自动存档
 */
export function clearAutoState(romPath: string): void {
  try {
    const saved = localStorage.getItem(AUTO_SAVE_KEY);
    if (!saved) return;
    const all = JSON.parse(saved);
    delete all[romPath];
    localStorage.setItem(AUTO_SAVE_KEY, JSON.stringify(all));
  } catch {}
}

// ==================== EmulatorJS 安装管理 ====================

/**
 * 检查 EmulatorJS 安装状态
 */
export async function checkEmulatorStatus(): Promise<EmulatorSetupStatus> {
  return await api.get<EmulatorSetupStatus>("/retrogame/status");
}

/**
 * 下载安装 EmulatorJS（SSE 流式进度）
 */
export async function setupEmulator(
  onProgress: (event: SetupProgress) => void,
): Promise<void> {
  const token = userStore.token || localStorage.getItem("auth_token");

  const response = await fetch("/api/v1/retrogame/setup", {
    method: "POST",
    headers: {
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
  });

  if (!response.ok) {
    throw new Error(`安装请求失败: ${response.status}`);
  }

  // 如果返回 JSON（已安装）
  const contentType = response.headers.get("Content-Type") || "";
  if (contentType.includes("application/json")) {
    const data = await response.json();
    onProgress({ status: "completed", message: data.message || "EmulatorJS 已安装", progress: 100 });
    return;
  }

  // SSE 流式读取进度
  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let lastEvent: SetupProgress | null = null;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });

    // SSE 格式: "event: xxx\ndata: {json}\n\n"
    // 按双换行分割事件块
    const blocks = buffer.split("\n\n");
    buffer = blocks.pop() || "";

    for (const block of blocks) {
      if (!block.trim()) continue;
      const lines = block.split("\n");
      for (const line of lines) {
        if (line.startsWith("data: ") || line.startsWith("data:")) {
          const jsonStr = line.startsWith("data: ") ? line.slice(6) : line.slice(5);
          try {
            const data: SetupProgress = JSON.parse(jsonStr.trim());
            lastEvent = data;
            onProgress(data);
          } catch {
            // 忽略解析错误
          }
        }
      }
    }
  }

  // 流结束后，如果没有收到 completed 事件，主动检查安装状态
  if (!lastEvent || lastEvent.status !== "completed") {
    try {
      const status = await checkEmulatorStatus();
      if (status.installed) {
        onProgress({ status: "completed", message: "EmulatorJS 安装完成！", progress: 100 });
      } else if (!lastEvent || lastEvent.status !== "failed") {
        onProgress({ status: "failed", message: "安装未完成，请重试", progress: 0 });
      }
    } catch {
      onProgress({ status: "failed", message: "无法检查安装状态", progress: 0 });
    }
  }
}

// ==================== 导出服务对象 ======================================

export const retroGameService = {
  scanRoms,
  getFileUrl,
  getCoverUrl,
  getCoverUrls,
  loadSettings,
  saveSettings,
  getSaveStates,
  saveSaveState,
  onGamepadChange,
  recordPlay,
  getRecentlyPlayed,
  saveAutoState,
  loadAutoState,
  hasAutoState,
  clearAutoState,
  checkEmulatorStatus,
  setupEmulator,
};
