// 应用类型定义
// 参考文档: docs/frontend/APP_ICON_MANAGEMENT.md

/**
 * 应用来源
 */
export type AppSource =
  | "system" // 系统核心应用（文件管理、设置、终端）
  | "module" // 内置模块应用（Docker、备份、存储管理等）
  | "docker_store"; // Docker 商店安装

/**
 * 应用状态
 */
export type AppState =
  | "active" // 正常可用
  | "installing" // 安装中
  | "uninstalling"; // 卸载中

/**
 * 应用类型
 */
export type AppType = "builtin" | "docker" | "native" | "internal";

/**
 * 开始菜单分类
 */
export type StartMenuCategory =
  | "system" // 系统工具
  | "productivity" // 效率工具
  | "multimedia" // 多媒体
  | "network" // 网络工具
  | "tools" // 实用工具
  | "other" // 其他
  | "docker_apps"; // Docker 应用

/**
 * 开始菜单位置
 */
export type StartMenuPosition = "left" | "center";

/**
 * 应用放置配置
 */
export interface AppPlacements {
  desktop: boolean; // 是否在桌面显示
  desktopPosition?: {
    // 桌面位置
    x: number;
    y: number;
  };
  startMenu: boolean; // 是否在开始菜单显示
  startMenuCategory?: StartMenuCategory; // 开始菜单分类
  taskbar: boolean; // 是否固定到任务栏
  taskbarOrder?: number; // 任务栏位置
}

/**
 * 应用启动配置
 */
export interface AppLaunchConfig {
  type: string;
  target: string; // 内部路由、容器ID、或应用路径
  args?: string[]; // 启动参数
}

/**
 * 已安装应用（完整信息）
 */
export interface InstalledApp {
  id: string; // 唯一标识，如 "docker-app-nginx"
  name: string; // 显示名称
  icon: string; // 图标（URL 或 Iconify 名称）
  description?: string; // 描述
  source: AppSource; // 来源

  placements: AppPlacements; // 放置配置
  state: AppState; // 当前状态
  installedAt?: string; // 安装时间 ISO 8601

  launchConfig: AppLaunchConfig; // 启动配置
  category?: StartMenuCategory; // 开始菜单分类
  keywords?: string; // 搜索关键词
}

/**
 * 后端返回的应用响应格式
 */
export interface AppResponse {
  id: string;
  name: string;
  icon: string;
  source: AppSource;
  launchConfig: AppLaunchConfig;
  state: AppState;
  category?: string;
  keywords?: string;
}

/**
 * 任务栏应用项
 */
export interface TaskbarAppItem {
  id: string;
  order: number;
}

/**
 * 桌面图标项
 */
export interface DesktopIconItem {
  appId: string;
  x: number;
  y: number;
}

/**
 * 用户偏好响应
 */
export interface UserPreferencesResponse {
  startMenuPosition: StartMenuPosition;
  pinnedApps: string[];
  taskbarApps: TaskbarAppItem[];
  desktopIcons: DesktopIconItem[];
  recentApps: string[];
}

/**
 * 分类信息
 */
export interface CategoryInfo {
  id: StartMenuCategory;
  name: string;
  icon?: string;
  order: number;
}

/**
 * 开始菜单分类映射
 */
export const CATEGORY_INFO: Record<StartMenuCategory, CategoryInfo> = {
  system: { id: "system", name: "系统工具", icon: "mdi:cog", order: 1 },
  productivity: { id: "productivity", name: "效率工具", icon: "mdi:briefcase", order: 2 },
  multimedia: { id: "multimedia", name: "多媒体", icon: "mdi:play-circle", order: 3 },
  network: { id: "network", name: "网络工具", icon: "mdi:lan", order: 4 },
  tools: { id: "tools", name: "实用工具", icon: "mdi:tools", order: 5 },
  other: { id: "other", name: "其他", icon: "mdi:apps", order: 6 },
  docker_apps: { id: "docker_apps", name: "Docker 应用", icon: "mdi:docker", order: 10 },
};

/**
 * 根据来源获取默认分类
 */
export function getDefaultCategory(source: AppSource): StartMenuCategory {
  switch (source) {
    case "system":
      return "system";
    case "module":
      return "other";
    case "docker_store":
      return "docker_apps";
    default:
      return "other";
  }
}

/**
 * 判断应用是否可卸载
 */
export function isUninstallable(app: InstalledApp): boolean {
  return (
    app.source === "docker_store"
  );
}
