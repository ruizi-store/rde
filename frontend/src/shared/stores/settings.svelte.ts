// 系统设置状态管理
// 持久化用户偏好设置

import { loadUserData, saveUserData } from "$shared/utils/user-storage";

export interface SystemSettings {
  // 外观
  appearance: {
    theme: "light" | "dark" | "auto";
    accentColor: string;
    wallpaper: string;
    wallpaperFit: "cover" | "contain" | "fill" | "tile";
    fontSize: "small" | "medium" | "large";
    animations: boolean;
    transparency: boolean;
  };

  // 桌面
  desktop: {
    showIcons: boolean;
    iconSize: "small" | "medium" | "large";
    snapEnabled: boolean;
    snapThreshold: number;
    doubleClickAction: "open" | "rename";
  };

  // 任务栏
  taskbar: {
    position: "bottom" | "top" | "left" | "right";
    autoHide: boolean;
    showClock: boolean;
    clockFormat: "12h" | "24h";
    showBattery: boolean;
    showNetwork: boolean;
  };

  // 文件管理器
  fileManager: {
    defaultView: "grid" | "list";
    showHiddenFiles: boolean;
    sortBy: "name" | "size" | "date" | "type";
    sortOrder: "asc" | "desc";
    previewPaneEnabled: boolean;
    confirmDelete: boolean;
  };

  // 通知
  notifications: {
    enabled: boolean;
    sound: boolean;
    desktop: boolean;
    duration: number; // 秒
    position: "top-right" | "top-left" | "bottom-right" | "bottom-left";
  };

  // 语言和区域
  locale: {
    language: string;
    timezone: string;
    dateFormat: string;
    timeFormat: "12h" | "24h";
    firstDayOfWeek: 0 | 1; // 0=周日, 1=周一
  };
}

// 默认设置
const defaultSettings: SystemSettings = {
  appearance: {
    theme: "auto",
    accentColor: "#0066cc",
    wallpaper: "/wallpapers/default.jpg",
    wallpaperFit: "cover",
    fontSize: "medium",
    animations: true,
    transparency: true,
  },
  desktop: {
    showIcons: true,
    iconSize: "medium",
    snapEnabled: true,
    snapThreshold: 20,
    doubleClickAction: "open",
  },
  taskbar: {
    position: "bottom",
    autoHide: false,
    showClock: true,
    clockFormat: "24h",
    showBattery: true,
    showNetwork: true,
  },
  fileManager: {
    defaultView: "grid",
    showHiddenFiles: false,
    sortBy: "name",
    sortOrder: "asc",
    previewPaneEnabled: false,
    confirmDelete: true,
  },
  notifications: {
    enabled: true,
    sound: true,
    desktop: true,
    duration: 5,
    position: "top-right",
  },
  locale: {
    language: "zh-CN",
    timezone: "Asia/Shanghai",
    dateFormat: "YYYY-MM-DD",
    timeFormat: "24h",
    firstDayOfWeek: 1,
  },
};

const STORAGE_KEY = "rde-settings";

class SettingsStore {
  // 使用深拷贝初始化
  private _settings = $state<SystemSettings>(structuredClone(defaultSettings));

  constructor() {
    this.load();
  }

  // 获取所有设置
  get all(): SystemSettings {
    return this._settings;
  }

  // 获取外观设置
  get appearance() {
    return this._settings.appearance;
  }

  // 获取桌面设置
  get desktop() {
    return this._settings.desktop;
  }

  // 获取任务栏设置
  get taskbar() {
    return this._settings.taskbar;
  }

  // 获取文件管理器设置
  get fileManager() {
    return this._settings.fileManager;
  }

  // 获取通知设置
  get notifications() {
    return this._settings.notifications;
  }

  // 获取语言区域设置
  get locale() {
    return this._settings.locale;
  }

  // 获取快捷键设置
  get shortcuts() {
    return this._settings.shortcuts;
  }

  // 更新外观设置
  updateAppearance(updates: Partial<SystemSettings["appearance"]>) {
    this._settings.appearance = { ...this._settings.appearance, ...updates };
    this.save();
  }

  // 更新桌面设置
  updateDesktop(updates: Partial<SystemSettings["desktop"]>) {
    this._settings.desktop = { ...this._settings.desktop, ...updates };
    this.save();
  }

  // 更新任务栏设置
  updateTaskbar(updates: Partial<SystemSettings["taskbar"]>) {
    this._settings.taskbar = { ...this._settings.taskbar, ...updates };
    this.save();
  }

  // 更新文件管理器设置
  updateFileManager(updates: Partial<SystemSettings["fileManager"]>) {
    this._settings.fileManager = { ...this._settings.fileManager, ...updates };
    this.save();
  }

  // 更新通知设置
  updateNotifications(updates: Partial<SystemSettings["notifications"]>) {
    this._settings.notifications = { ...this._settings.notifications, ...updates };
    this.save();
  }

  // 更新语言区域设置
  updateLocale(updates: Partial<SystemSettings["locale"]>) {
    this._settings.locale = { ...this._settings.locale, ...updates };
    this.save();
  }

  // 更新快捷键设置
  updateShortcuts(updates: Partial<SystemSettings["shortcuts"]>) {
    this._settings.shortcuts = { ...this._settings.shortcuts, ...updates };
    this.save();
  }

  // 批量更新
  update(updates: Partial<SystemSettings>) {
    this._settings = { ...this._settings, ...updates };
    this.save();
  }

  // 重置所有设置
  reset() {
    this._settings = structuredClone(defaultSettings);
    this.save();
  }

  // 重置某个分类的设置
  resetCategory(category: keyof SystemSettings) {
    (this._settings as any)[category] = structuredClone((defaultSettings as any)[category]);
    this.save();
  }

  // 从 localStorage 加载（用户专属 key）
  private load() {
    if (typeof window === "undefined") return;

    try {
      const parsed = loadUserData<Partial<SystemSettings>>(STORAGE_KEY);
      if (parsed) {
        // 深度合并，保留新增的默认值
        this._settings = this.deepMerge(structuredClone(defaultSettings), parsed);
      }
    } catch (err) {
      console.warn("加载设置失败，使用默认值", err);
    }
  }

  // 保存到 localStorage（用户专属 key）
  private save() {
    if (typeof window === "undefined") return;

    try {
      saveUserData(STORAGE_KEY, this._settings);
    } catch (err) {
      console.warn("保存设置失败", err);
    }
  }

  // 为当前用户重新加载设置（用户切换时调用）
  reloadForUser() {
    this._settings = structuredClone(defaultSettings);
    this.load();
  }

  // 深度合并对象
  private deepMerge<T extends object>(target: T, source: Partial<T>): T {
    const result = { ...target };

    for (const key in source) {
      const sourceValue = source[key];
      const targetValue = target[key];

      if (
        sourceValue &&
        typeof sourceValue === "object" &&
        !Array.isArray(sourceValue) &&
        targetValue &&
        typeof targetValue === "object" &&
        !Array.isArray(targetValue)
      ) {
        (result as any)[key] = this.deepMerge(targetValue as object, sourceValue as object);
      } else if (sourceValue !== undefined) {
        (result as any)[key] = sourceValue;
      }
    }

    return result;
  }

  // 导出设置
  export(): string {
    return JSON.stringify(this._settings, null, 2);
  }

  // 导入设置
  import(json: string): boolean {
    try {
      const parsed = JSON.parse(json);
      this._settings = this.deepMerge(structuredClone(defaultSettings), parsed);
      this.save();
      return true;
    } catch {
      return false;
    }
  }
}

export const settings = new SettingsStore();
