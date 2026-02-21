// Window Manager Store - Svelte 5 Runes
// 窗口管理器，负责所有窗口的状态管理

import type { Component } from "svelte";
import type { InstalledPackage, PackageWindowConfig } from "$shared/types/package";
import { generateUUID } from "$shared/utils/uuid";
import { getAppName } from "$lib/i18n/app-names";

// 使用更宽松的组件类型以兼容各种 props
// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyComponent = Component<any, any, any>;

// 支持同步组件和动态导入的懒加载组件
export type LazyComponent = AnyComponent | (() => Promise<{ default: AnyComponent }>);

/**
 * 窗口类型
 * - component: Svelte 组件窗口（内置应用）
 * - iframe: iframe 嵌入窗口（套件应用）
 */
export type WindowType = "component" | "iframe";

/**
 * iframe 配置
 */
export interface IframeConfig {
  url: string;
  sandbox?: string;
  allow?: string; // Permission Policy (如 "fullscreen; camera; microphone")
  allowFullscreen?: boolean;
  packageId?: string; // 关联的套件 ID
  permissions?: string[]; // 套件权限
}

export interface WindowState {
  id: string;
  appId: string;
  title: string;
  icon: string;
  x: number;
  y: number;
  width: number;
  height: number;
  minWidth: number;
  minHeight: number;
  isMaximized: boolean;
  isMinimized: boolean;
  isFocused: boolean;
  zIndex: number;

  // 窗口类型
  type: WindowType;

  // 组件模式
  component?: AnyComponent;
  props?: Record<string, unknown>;

  // iframe 模式
  iframeConfig?: IframeConfig;
}

export interface AppDefinition {
  id: string;
  name: string;
  icon: string;
  component: LazyComponent;
  defaultWidth?: number;
  defaultHeight?: number;
  minWidth?: number;
  minHeight?: number;
  singleton?: boolean; // 是否只允许打开一个实例
}

class WindowManager {
  // 使用 Map 存储所有窗口
  private _windows = $state<Map<string, WindowState>>(new Map());
  private _activeWindowId = $state<string | null>(null);
  private _maxZIndex = $state(100);

  // 获取窗口列表（按 z-index 排序）
  get windowList(): WindowState[] {
    return [...this._windows.values()].sort((a, b) => a.zIndex - b.zIndex);
  }

  // 获取活动窗口
  get activeWindow(): WindowState | null {
    if (!this._activeWindowId) return null;
    return this._windows.get(this._activeWindowId) ?? null;
  }

  // 获取活动窗口 ID
  get activeWindowId(): string | null {
    return this._activeWindowId;
  }

  // 已注册的应用 (延迟绑定，避免循环依赖)
  private _registeredApps: Map<string, AppDefinition> | null = null;

  // 设置应用注册表 (由 apps store 调用)
  setAppRegistry(apps: Map<string, AppDefinition>): void {
    this._registeredApps = apps;
  }

  // 获取已注册的应用
  private _getApp(appId: string): AppDefinition | undefined {
    return this._registeredApps?.get(appId);
  }

  // 打开新窗口 - 支持 AppDefinition 或 appId 字符串
  async open(appOrId: AppDefinition | string, props?: Record<string, unknown>): Promise<string> {
    let app: AppDefinition;

    if (typeof appOrId === "string") {
      const foundApp = this._getApp(appOrId);
      if (!foundApp) {
        console.warn(`App not found: ${appOrId}`);
        return "";
      }
      app = foundApp;
    } else {
      app = appOrId;
    }

    // 检查是否是单例应用
    if (app.singleton) {
      const existing = this.findByAppId(app.id);
      if (existing) {
        this.focus(existing.id);
        return existing.id;
      }
    }

    // 解析懒加载组件
    let resolvedComponent: AnyComponent;
    if (typeof app.component === "function" && app.component.length === 0) {
      try {
        const mod = await (app.component as () => Promise<{ default: AnyComponent }>)();
        resolvedComponent = mod.default;
      } catch (e) {
        console.error(`[WindowManager] Failed to load component for ${app.id}:`, e);

        // 动态 import 失败通常意味着前端资源版本不匹配（部署了新版本但浏览器仍缓存旧代码）
        // 自动刷新页面以加载最新版本
        if (
          e instanceof TypeError &&
          e.message.includes("Failed to fetch dynamically imported module")
        ) {
          console.warn("[WindowManager] Asset version mismatch detected, reloading page...");
          window.location.reload();
        }

        return "";
      }
    } else {
      resolvedComponent = app.component as AnyComponent;
    }

    const id = generateUUID();
    const offset = (this._windows.size % 10) * 30;

    const window: WindowState = {
      id,
      appId: app.id,
      title: getAppName(app.id, app.name),
      icon: app.icon,
      x: 100 + offset,
      y: 80 + offset,
      width: app.defaultWidth ?? 800,
      height: app.defaultHeight ?? 600,
      minWidth: app.minWidth ?? 400,
      minHeight: app.minHeight ?? 300,
      isMaximized: false,
      isMinimized: false,
      isFocused: true,
      zIndex: ++this._maxZIndex,
      type: "component",
      component: resolvedComponent,
      props,
    };

    // 取消其他窗口焦点
    this._unfocusAll();

    this._windows.set(id, window);
    // 触发响应式更新 - Svelte 5 对 Map.set 不会自动追踪
    this._windows = new Map(this._windows);
    this._activeWindowId = id;

    return id;
  }

  /**
   * 打开 iframe 窗口 - 用于套件应用
   */
  openIframe(options: {
    packageId: string;
    url: string;
    title: string;
    icon: string;
    width?: number;
    height?: number;
    minWidth?: number;
    minHeight?: number;
    singleton?: boolean;
    permissions?: string[];
    sandbox?: string; // 自定义沙箱策略
    allow?: string; // Permission Policy
  }): string {
    // 检查是否是单例
    if (options.singleton) {
      const existing = this.findByAppId(options.packageId);
      if (existing) {
        this.focus(existing.id);
        return existing.id;
      }
    }

    const id = generateUUID();
    const offset = (this._windows.size % 10) * 30;

    const window: WindowState = {
      id,
      appId: options.packageId,
      title: options.title,
      icon: options.icon,
      x: 100 + offset,
      y: 80 + offset,
      width: options.width ?? 1000,
      height: options.height ?? 700,
      minWidth: options.minWidth ?? 400,
      minHeight: options.minHeight ?? 300,
      isMaximized: false,
      isMinimized: false,
      isFocused: true,
      zIndex: ++this._maxZIndex,
      type: "iframe",
      iframeConfig: {
        url: options.url,
        // 注意: allow-scripts + allow-same-origin 组合会触发浏览器安全警告
        // 这是设计权衡：套件需要执行脚本并与主窗口通信（postMessage）
        // 对于本地受信任的套件，这是可接受的；生产环境应只加载经过审核的套件
        sandbox:
          options.sandbox ??
          "allow-scripts allow-same-origin allow-forms allow-popups allow-modals",
        allow: options.allow ?? "fullscreen; autoplay",
        allowFullscreen: true,
        packageId: options.packageId,
        permissions: options.permissions,
      },
    };

    // 取消其他窗口焦点
    this._unfocusAll();

    this._windows.set(id, window);
    this._windows = new Map(this._windows);
    this._activeWindowId = id;

    return id;
  }

  // 关闭窗口
  close(id: string): void {
    this._windows.delete(id);
    // 触发响应式更新
    this._windows = new Map(this._windows);

    if (this._activeWindowId === id) {
      const topWindow = this._getTopWindow();
      this._activeWindowId = topWindow?.id ?? null;
      if (topWindow) {
        this._updateWindow(topWindow.id, { isFocused: true });
      }
    }
  }

  // 聚焦窗口
  focus(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    this._unfocusAll();

    this._updateWindow(id, {
      isFocused: true,
      isMinimized: false,
      zIndex: ++this._maxZIndex,
    });

    this._activeWindowId = id;
  }

  // 最小化窗口
  minimize(id: string): void {
    this._updateWindow(id, {
      isMinimized: true,
      isFocused: false,
    });

    if (this._activeWindowId === id) {
      const topWindow = this._getTopWindow();
      this._activeWindowId = topWindow?.id ?? null;
      if (topWindow) {
        this._updateWindow(topWindow.id, { isFocused: true });
      }
    }
  }

  // 最小化所有窗口（显示桌面）
  minimizeAll(): void {
    for (const win of this._windows.values()) {
      if (!win.isMinimized) {
        this._updateWindow(win.id, {
          isMinimized: true,
          isFocused: false,
        });
      }
    }
    this._activeWindowId = null;
  }

  // 切换最大化
  toggleMaximize(id: string): void {
    const win = this._windows.get(id);
    if (win) {
      this._updateWindow(id, { isMaximized: !win.isMaximized });
    }
  }

  // 移动窗口
  move(id: string, x: number, y: number): void {
    const win = this._windows.get(id);
    if (win && !win.isMaximized) {
      this._updateWindow(id, { x: Math.max(0, x), y: Math.max(0, y) });
    }
  }

  // 调整窗口大小
  resize(id: string, width: number, height: number, x?: number, y?: number): void {
    const win = this._windows.get(id);
    if (win && !win.isMaximized) {
      const updates: Partial<WindowState> = {
        width: Math.max(width, win.minWidth),
        height: Math.max(height, win.minHeight),
      };
      if (x !== undefined) updates.x = Math.max(0, x);
      if (y !== undefined) updates.y = Math.max(0, y);
      this._updateWindow(id, updates);
    }
  }

  // 窗口吸附 - 左半屏
  snapLeft(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48; // 减去任务栏高度

    this._updateWindow(id, {
      x: 0,
      y: 0,
      width: Math.floor(screenWidth / 2),
      height: screenHeight,
      isMaximized: false,
    });
    this.focus(id);
  }

  // 窗口吸附 - 右半屏
  snapRight(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48;

    this._updateWindow(id, {
      x: Math.floor(screenWidth / 2),
      y: 0,
      width: Math.floor(screenWidth / 2),
      height: screenHeight,
      isMaximized: false,
    });
    this.focus(id);
  }

  // 窗口吸附 - 左上角
  snapTopLeft(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48;

    this._updateWindow(id, {
      x: 0,
      y: 0,
      width: Math.floor(screenWidth / 2),
      height: Math.floor(screenHeight / 2),
      isMaximized: false,
    });
    this.focus(id);
  }

  // 窗口吸附 - 右上角
  snapTopRight(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48;

    this._updateWindow(id, {
      x: Math.floor(screenWidth / 2),
      y: 0,
      width: Math.floor(screenWidth / 2),
      height: Math.floor(screenHeight / 2),
      isMaximized: false,
    });
    this.focus(id);
  }

  // 窗口吸附 - 左下角
  snapBottomLeft(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48;

    this._updateWindow(id, {
      x: 0,
      y: Math.floor(screenHeight / 2),
      width: Math.floor(screenWidth / 2),
      height: Math.floor(screenHeight / 2),
      isMaximized: false,
    });
    this.focus(id);
  }

  // 窗口吸附 - 右下角
  snapBottomRight(id: string): void {
    const win = this._windows.get(id);
    if (!win) return;

    const screenWidth = window.innerWidth;
    const screenHeight = window.innerHeight - 48;

    this._updateWindow(id, {
      x: Math.floor(screenWidth / 2),
      y: Math.floor(screenHeight / 2),
      width: Math.floor(screenWidth / 2),
      height: Math.floor(screenHeight / 2),
      isMaximized: false,
    });
    this.focus(id);
  }

  // 更新窗口标题
  setTitle(id: string, title: string): void {
    this._updateWindow(id, { title });
  }

  // 根据 appId 查找窗口
  findByAppId(appId: string): WindowState | undefined {
    return [...this._windows.values()].find((w) => w.appId === appId);
  }

  // 获取窗口
  get(id: string): WindowState | undefined {
    return this._windows.get(id);
  }

  // 关闭所有窗口
  closeAll(): void {
    this._windows.clear();
    this._activeWindowId = null;
  }

  // 私有方法：取消所有窗口焦点
  private _unfocusAll(): void {
    for (const [id, win] of this._windows) {
      if (win.isFocused) {
        this._windows.set(id, { ...win, isFocused: false });
      }
    }
  }

  // 私有方法：获取最顶层窗口
  private _getTopWindow(): WindowState | null {
    let top: WindowState | null = null;
    for (const win of this._windows.values()) {
      if (!win.isMinimized && (!top || win.zIndex > top.zIndex)) {
        top = win;
      }
    }
    return top;
  }

  // 私有方法：更新窗口
  private _updateWindow(id: string, updates: Partial<WindowState>): void {
    const win = this._windows.get(id);
    if (win) {
      this._windows.set(id, { ...win, ...updates });
      // 触发响应式更新
      this._windows = new Map(this._windows);
    }
  }
}

// 导出单例
export const windowManager = new WindowManager();
// 别名导出 (向后兼容)
export const windows = windowManager;
