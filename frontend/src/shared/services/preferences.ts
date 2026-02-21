// 应用偏好服务
// 与后端 /api/v1/apps, /api/v1/users/preferences, /api/v1/desktop, /api/v1/taskbar 等 API 交互

import { api } from "./api";
import type {
  AppResponse,
  UserPreferencesResponse,
  TaskbarAppItem,
  DesktopIconItem,
  StartMenuPosition,
} from "$shared/types/apps";

/**
 * 应用列表响应
 */
interface AppsListResponse {
  apps: AppResponse[];
}

/**
 * 更新偏好请求
 */
interface UpdatePreferencesRequest {
  startMenuPosition?: StartMenuPosition;
  pinnedApps?: string[];
  taskbarApps?: TaskbarAppItem[];
  recentApps?: string[];
}

/**
 * 应用服务 - 获取应用列表
 * 注意：后端 API 尚未实现，返回空数组
 */
export const appsService = {
  /**
   * 获取所有应用
   * TODO: 后端 API 待实现
   */
  async getApps(): Promise<AppResponse[]> {
    // 后端 API 尚未实现，返回空数组
    // 应用列表由前端内置的 apps.ts 定义
    return [];
  },

  /**
   * 记录应用启动（更新最近使用）
   * TODO: 后端 API 待实现
   */
  async recordLaunch(_appId: string): Promise<void> {
    // 后端 API 尚未实现，静默忽略
  },
};

/**
 * 用户偏好服务 - 管理开始菜单、任务栏、桌面配置
 * 注意：后端 API 尚未实现，使用本地存储
 */
export const preferencesService = {
  /**
   * 获取用户偏好
   * TODO: 后端 API 待实现，当前返回 null
   */
  async getPreferences(): Promise<UserPreferencesResponse | null> {
    // 后端 API 尚未实现，返回 null
    // 偏好由前端本地存储管理
    return null;
  },

  /**
   * 更新用户偏好
   * TODO: 后端 API 待实现
   */
  async updatePreferences(_data: UpdatePreferencesRequest): Promise<void> {
    // 后端 API 尚未实现，静默忽略
  },

  /**
   * 更新开始菜单位置
   */
  async setStartMenuPosition(_position: StartMenuPosition): Promise<void> {
    // 后端 API 尚未实现
  },

  /**
   * 更新固定的应用列表
   */
  async setPinnedApps(_appIds: string[]): Promise<void> {
    // 后端 API 尚未实现
  },

  /**
   * 更新任务栏应用
   */
  async setTaskbarApps(_apps: TaskbarAppItem[]): Promise<void> {
    // 后端 API 尚未实现
  },
};

/**
 * 桌面图标服务
 * 注意：后端 API 尚未实现
 */
export const desktopService = {
  /**
   * 获取桌面图标
   */
  async getIcons(): Promise<DesktopIconItem[]> {
    // 后端 API 尚未实现
    return [];
  },

  /**
   * 批量更新桌面图标
   */
  async updateIcons(_icons: DesktopIconItem[]): Promise<void> {
    // 后端 API 尚未实现
  },

  /**
   * 添加桌面图标
   */
  async addIcon(_appId: string, _x: number, _y: number): Promise<void> {
    // 后端 API 尚未实现
  },

  /**
   * 移除桌面图标
   */
  async removeIcon(_appId: string): Promise<void> {
    // 后端 API 尚未实现
  },
};

/**
 * 开始菜单服务
 * 注意：后端 API 尚未实现
 */
export const startMenuService = {
  /**
   * 固定应用到开始菜单
   */
  async pin(_appId: string): Promise<void> {
    // 后端 API 尚未实现
  },

  /**
   * 从开始菜单取消固定
   */
  async unpin(_appId: string): Promise<void> {
    // 后端 API 尚未实现
  },
};

/**
 * 任务栏服务
 * 注意：后端 API 尚未实现
 */
export const taskbarService = {
  /**
   * 更新任务栏应用
   */
  async updateApps(_apps: TaskbarAppItem[]): Promise<void> {
    // 后端 API 尚未实现
  },
};
