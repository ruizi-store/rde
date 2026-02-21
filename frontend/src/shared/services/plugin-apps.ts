/**
 * Plugin Apps Service
 * 从后端发现插件提供的前端应用，并动态注册到桌面环境
 */

import { api } from "./api";

/** 插件应用定义（对应后端 PluginApp） */
export interface PluginAppDef {
  id: string;
  name: string;
  icon: string;
  frontend_route: string;
  category?: string;
  default_width?: number;
  default_height?: number;
  min_width?: number;
  min_height?: number;
  singleton?: boolean;
  permissions?: string[];
}

/** 插件应用信息（对应后端 PluginAppInfo） */
export interface PluginAppInfo {
  plugin_id: string;
  app: PluginAppDef;
}

class PluginAppsService {
  /**
   * 获取所有运行中插件提供的前端应用
   */
  async getPluginApps(): Promise<PluginAppInfo[]> {
    try {
      const resp = await api.get<{ data: PluginAppInfo[] }>("/plugin-apps");
      return resp.data || [];
    } catch (e) {
      console.warn("[PluginApps] Failed to fetch plugin apps:", e);
      return [];
    }
  }

  /**
   * 构建插件应用的前端 URL
   * 插件前端资源通过 API 代理路由访问
   */
  buildAppUrl(app: PluginAppDef): string {
    // frontend_route 是相对路径，如 "/ai/app/"
    // 完整 URL 为 /api/v1/ai/app/
    return `/api/v1${app.frontend_route}`;
  }
}

export const pluginAppsService = new PluginAppsService();
