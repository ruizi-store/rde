/**
 * Docker 应用商店服务
 * 调用后端 /docker/store/* API
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface LocaleText {
  en: string;
  zh: string;
}

/** 商店应用列表项（精简） */
export interface StoreAppItem {
  id: string;
  name: string;
  title: string;
  description: string;
  title_i18n: LocaleText;
  desc_i18n: LocaleText;
  category: string;
  icon: string;
  version: string;
  author?: string;
  tags: string[];
  architectures: string[];
}

/** 商店应用详情（含完整 compose 和 form） */
export interface StoreAppDetail extends StoreAppItem {
  license?: string;
  homepage: string;
  repository: string;
  form: FormField[];
  compose: string; // YAML 字符串
}

/** 表单字段 */
export interface FormField {
  key: string;
  label: LocaleText;
  default?: string | number;
  required: boolean;
  type: "text" | "number" | "password";
  env_key?: string;
}

/** 分类 */
export interface StoreCategory {
  id: string;
  name: string;
  count: number;
}

// ==================== 分类图标映射 ====================

export const categoryIcons: Record<string, string> = {
  database: "mdi:database",
  development: "mdi:code-braces",
  ai: "mdi:brain",
  storage: "mdi:harddisk",
  media: "mdi:play-circle",
  monitoring: "mdi:chart-line",
  productivity: "mdi:briefcase",
  utilities: "mdi:wrench",
  security: "mdi:shield-lock",
  network: "mdi:lan",
  communication: "mdi:chat",
  gaming: "mdi:gamepad-variant",
  automation: "mdi:robot",
  finance: "mdi:currency-usd",
  other: "mdi:package-variant",
};

// ==================== 已安装应用类型 ====================

/** 已安装应用 */
export interface InstalledApp {
  name: string;
  app_id: string;
  version: string;
  icon: string;
  status: "running" | "stopped" | "unknown";
  config: Record<string, unknown>;
  compose_path: string;
  installed_at: string;
}

/** 安装任务（异步） */
export interface InstallTask {
  id: string;
  app_id: string;
  name: string;
  status: "pending" | "running" | "success" | "failed";
  output: string;
  error?: string;
  created_at: string;
  done_at?: string;
}

/** 安装请求 */
export interface InstallRequest {
  app_id: string;
  config: Record<string, unknown>;
}

/** 端口检查结果 */
export interface PortCheckResult {
  port: string;
  available: boolean;
}

/** 端口建议结果 */
export interface PortSuggestResult {
  suggested: number;
  preferred: number;
}

// ==================== 服务实现 ====================

class DockerStoreService {
  // ---- 商店目录 ----

  /** 获取应用列表 */
  async getApps(category?: string, search?: string): Promise<StoreAppItem[]> {
    const params: Record<string, string> = {};
    if (category) params.category = category;
    if (search) params.search = search;

    const query = new URLSearchParams(params).toString();
    const url = `/docker/store/apps${query ? `?${query}` : ""}`;
    const res = await api.get<{ data: StoreAppItem[] }>(url);
    return (res as any).data ?? res;
  }

  /** 获取应用详情 */
  async getApp(id: string): Promise<StoreAppDetail> {
    const res = await api.get<{ data: StoreAppDetail }>(`/docker/store/apps/${id}`);
    return (res as any).data ?? res;
  }

  /** 获取分类列表 */
  async getCategories(): Promise<StoreCategory[]> {
    const res = await api.get<{ data: StoreCategory[] }>("/docker/store/categories");
    return (res as any).data ?? res;
  }

  /** 构建图标 URL */
  getIconUrl(icon: string): string {
    if (!icon) return "";
    if (icon.startsWith("http")) return icon;
    // 直接使用前端静态资源路径，避免经过 API 认证
    return `/docker-icons/${icon}`;
  }

  // ---- 已安装应用 ----

  /** 获取已安装应用列表 */
  async getInstalledApps(): Promise<InstalledApp[]> {
    const res = await api.get<{ data: InstalledApp[] }>("/docker/apps");
    return (res as any).data ?? res;
  }

  /** 安装应用（同步） */
  async installApp(
    appId: string,
    config: Record<string, unknown>,
  ): Promise<{ data: InstalledApp; output: string }> {
    const res = await api.post<{ data: InstalledApp; output: string }>("/docker/apps", {
      app_id: appId,
      config,
    });
    return res;
  }

  /** 异步安装应用（后台执行） */
  async installAppAsync(
    appId: string,
    config: Record<string, unknown>,
  ): Promise<InstallTask> {
    const res = await api.post<{ data: InstallTask }>("/docker/apps/async", {
      app_id: appId,
      config,
    });
    return (res as any).data ?? res;
  }

  /** 获取安装任务状态 */
  async getInstallTask(taskId: string): Promise<InstallTask> {
    const res = await api.get<{ data: InstallTask }>(`/docker/apps/tasks/${taskId}`);
    return (res as any).data ?? res;
  }

  /** 轮询安装任务直到完成 */
  async pollInstallTask(
    taskId: string,
    onProgress?: (task: InstallTask) => void,
    intervalMs = 2000,
  ): Promise<InstallTask> {
    return new Promise((resolve, reject) => {
      const poll = async () => {
        try {
          const task = await this.getInstallTask(taskId);
          onProgress?.(task);
          if (task.status === "success" || task.status === "failed") {
            resolve(task);
          } else {
            setTimeout(poll, intervalMs);
          }
        } catch (e) {
          reject(e);
        }
      };
      poll();
    });
  }

  /** 卸载应用 */
  async uninstallApp(name: string): Promise<void> {
    await api.delete(`/docker/apps/${name}`);
  }

  /** 启动应用 */
  async startApp(name: string): Promise<void> {
    await api.post(`/docker/apps/${name}/start`);
  }

  /** 停止应用 */
  async stopApp(name: string): Promise<void> {
    await api.post(`/docker/apps/${name}/stop`);
  }

  /** 重启应用 */
  async restartApp(name: string): Promise<void> {
    await api.post(`/docker/apps/${name}/restart`);
  }

  /** 获取应用日志 */
  async getAppLogs(name: string, tail = 100): Promise<string> {
    const res = await api.get<{ data: string }>(`/docker/apps/${name}/logs?tail=${tail}`);
    return (res as any).data ?? res;
  }

  // ---- 端口 ----

  /** 检查端口是否可用 */
  async checkPort(port: number): Promise<boolean> {
    const res = await api.get<PortCheckResult>(`/docker/ports/check?port=${port}`);
    return (res as any).available ?? false;
  }

  /** 获取建议端口 */
  async suggestPort(preferred: number): Promise<number> {
    const res = await api.get<PortSuggestResult>(`/docker/ports/suggest?preferred=${preferred}`);
    return (res as any).suggested ?? preferred;
  }
}

export const dockerStoreService = new DockerStoreService();
