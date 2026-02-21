// 通知泡泡设置 Store
// 控制通知是否以泡泡形式显示在桌面上

import { browser } from "$app/environment";

// 泡泡通知模式
export type BubbleNotificationMode = "always" | "never" | "auto";

// 支持泡泡效果的壁纸主题 ID
export const BUBBLE_THEME_WALLPAPERS = [
  "underwater", // 海底世界
  "aquarium", // 水族馆
  "ocean", // 海洋
  "deep-sea", // 深海
];

const STORAGE_KEY = "rde_notification_bubble";

interface BubbleSettings {
  mode: BubbleNotificationMode;
  maxBubbles: number;
  autoHideSeconds: number;
}

// 默认设置
const defaultSettings: BubbleSettings = {
  mode: "auto",
  maxBubbles: 5,
  autoHideSeconds: 30,
};

// 从 localStorage 读取设置
function loadSettings(): BubbleSettings {
  if (!browser) return defaultSettings;

  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      return { ...defaultSettings, ...JSON.parse(saved) };
    }
  } catch (e) {
    console.error("加载泡泡通知设置失败:", e);
  }
  return defaultSettings;
}

class NotificationBubbleStore {
  // 设置
  private _mode = $state<BubbleNotificationMode>(defaultSettings.mode);
  private _maxBubbles = $state(defaultSettings.maxBubbles);
  private _autoHideSeconds = $state(defaultSettings.autoHideSeconds);

  // 当前壁纸 ID（用于 auto 模式判断）
  private _currentWallpaperId = $state("");

  // 初始化标志
  private _initialized = $state(false);

  // 清空信号（递增触发清空）
  private _clearSignal = $state(0);

  constructor() {
    if (browser) {
      const settings = loadSettings();
      this._mode = settings.mode;
      this._maxBubbles = settings.maxBubbles;
      this._autoHideSeconds = settings.autoHideSeconds;
      this._initialized = true;
    }
  }

  // Getters
  get mode(): BubbleNotificationMode {
    return this._mode;
  }

  get maxBubbles(): number {
    return this._maxBubbles;
  }

  get autoHideSeconds(): number {
    return this._autoHideSeconds;
  }

  get initialized(): boolean {
    return this._initialized;
  }

  // 清空信号（用于通知泡泡管理器清空所有泡泡）
  get clearSignal(): number {
    return this._clearSignal;
  }

  // 判断泡泡通知是否启用
  get isEnabled(): boolean {
    if (this._mode === "always") return true;
    if (this._mode === "never") return false;

    // auto 模式：根据当前壁纸判断
    return BUBBLE_THEME_WALLPAPERS.some((id) =>
      this._currentWallpaperId.toLowerCase().includes(id),
    );
  }

  // 设置模式
  setMode(mode: BubbleNotificationMode): void {
    this._mode = mode;
    this.save();
  }

  // 设置最大泡泡数
  setMaxBubbles(count: number): void {
    this._maxBubbles = Math.max(1, Math.min(10, count));
    this.save();
  }

  // 设置自动隐藏时间
  setAutoHideSeconds(seconds: number): void {
    this._autoHideSeconds = Math.max(10, Math.min(120, seconds));
    this.save();
  }

  // 更新当前壁纸 ID（由壁纸组件调用）
  setCurrentWallpaper(wallpaperId: string): void {
    this._currentWallpaperId = wallpaperId;
  }

  // 触发清空所有泡泡（通知被清空或全部标记已读时调用）
  triggerClear(): void {
    this._clearSignal++;
  }

  // 移除指定通知的泡泡
  removeNotification(notificationId: string): void {
    // 通过增加信号值并附带ID来触发移除
    // 但为了简单起见，我们使用事件
    this._removedNotificationIds = [...this._removedNotificationIds, notificationId];
  }

  // 获取待移除的通知ID列表
  private _removedNotificationIds = $state<string[]>([]);
  
  get removedNotificationIds(): string[] {
    return this._removedNotificationIds;
  }

  // 确认已处理移除
  clearRemovedIds(): void {
    this._removedNotificationIds = [];
  }

  // 保存设置
  private save(): void {
    if (!browser) return;

    try {
      const settings: BubbleSettings = {
        mode: this._mode,
        maxBubbles: this._maxBubbles,
        autoHideSeconds: this._autoHideSeconds,
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch (e) {
      console.error("保存泡泡通知设置失败:", e);
    }
  }
}

export const notificationBubbleStore = new NotificationBubbleStore();
