/**
 * Flatpak Apps Service
 *
 * 通过 KasmVNC 实现 Flatpak GUI 应用远程显示
 * 后端路由: /api/v1/flatpak/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

/** 桌面状态 */
export interface DesktopStatus {
  running: boolean;
  display: number;
  websocket_port: number;
  vnc_url: string;
  pid: number;
  uptime: number;
  resolution: string;
  running_apps: string[];
  kasmvnc_version: string;
}

/** 桌面配置 */
export interface DesktopConfig {
  display: number;
  websocket_port: number;
  default_resolution: string;
  audio_enabled: boolean;
  clipboard_sync: boolean;
  auto_start: boolean;
}

/** Flatpak 应用 */
export interface FlatpakApp {
  app_id: string;
  name: string;
  description: string;
  version: string;
  icon?: string;
  category?: string;
  installed: boolean;
  size?: string;
  runtime?: string;
  remote?: string;
  running: boolean;
}

/** 环境状态 */
export interface SetupStatus {
  kasmvnc_installed: boolean;
  kasmvnc_version: string;
  kasmvnc_expected: string;
  flatpak_installed: boolean;
  flatpak_remote_ok: boolean;
  openbox_installed: boolean;
  pulseaudio_installed: boolean;
  pulseaudio_running: boolean;
  virtual_sink_ready: boolean;
  ready: boolean;
}

/** SSE 事件 */
export interface SSEEvent {
  type: "start" | "progress" | "error" | "complete";
  message: string;
}

// ==================== SSE 流式请求辅助 ====================

function streamRequest(
  url: string,
  options: RequestInit,
  onProgress: (line: string) => void,
  onComplete: (success: boolean, error?: string) => void
): () => void {
  const controller = new AbortController();
  const token =
    typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  if (token) headers["Authorization"] = `Bearer ${token}`;

  fetch(url, {
    ...options,
    headers: { ...headers, ...((options.headers as Record<string, string>) || {}) },
    signal: controller.signal,
  })
    .then(async (response) => {
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
      const reader = response.body?.getReader();
      if (!reader) throw new Error("No reader available");

      const decoder = new TextDecoder();
      let buffer = "";

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split("\n\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          if (line.startsWith("data: ")) {
            try {
              const data: SSEEvent = JSON.parse(line.slice(6));
              if (data.type === "progress") {
                onProgress(data.message);
              } else if (data.type === "complete") {
                onComplete(true);
              } else if (data.type === "error") {
                onComplete(false, data.message);
              }
            } catch {
              // ignore parse errors
            }
          }
        }
      }
    })
    .catch((err) => {
      if (err.name !== "AbortError") {
        onComplete(false, err.message);
      }
    });

  return () => controller.abort();
}

// ==================== 服务类 ====================

class FlatpakService {
  // ========== Setup ==========

  /** 获取环境检测状态 */
  async getSetupStatus(): Promise<SetupStatus> {
    return api.get<SetupStatus>("/flatpak/setup/status");
  }

  /** 执行环境安装（SSE 流式） */
  runSetupStream(
    onProgress: (line: string) => void,
    onComplete: (success: boolean, error?: string) => void
  ): () => void {
    return streamRequest(
      "/api/v1/flatpak/setup/run",
      { method: "POST" },
      onProgress,
      onComplete
    );
  }

  // ========== Desktop ==========

  /** 获取桌面状态 */
  async getDesktopStatus(): Promise<DesktopStatus> {
    return api.get<DesktopStatus>("/flatpak/desktop/status");
  }

  /** 启动桌面 */
  async startDesktop(): Promise<void> {
    await api.post("/flatpak/desktop/start");
  }

  /** 停止桌面 */
  async stopDesktop(): Promise<void> {
    await api.post("/flatpak/desktop/stop");
  }

  /** 重启桌面 */
  async restartDesktop(): Promise<void> {
    await api.post("/flatpak/desktop/restart");
  }

  /** 获取桌面配置 */
  async getDesktopConfig(): Promise<DesktopConfig> {
    return api.get<DesktopConfig>("/flatpak/desktop/config");
  }

  /** 更新桌面配置 */
  async updateDesktopConfig(config: Partial<DesktopConfig>): Promise<void> {
    await api.put("/flatpak/desktop/config", config);
  }

  /** 获取 VNC iframe URL */
  getVNCUrl(): string {
    return "/api/v1/flatpak/vnc/vnc.html?autoconnect=true&resize=remote&path=api/v1/flatpak/vnc/websockify";
  }

  // ========== Apps ==========

  /** 获取已安装应用 */
  async getInstalledApps(): Promise<FlatpakApp[]> {
    return api.get<FlatpakApp[]>("/flatpak/apps");
  }

  /** 搜索 Flathub 应用 */
  async searchApps(query: string): Promise<FlatpakApp[]> {
    return api.get<FlatpakApp[]>(`/flatpak/apps/search?q=${encodeURIComponent(query)}`);
  }

  /** 获取推荐应用 */
  async getRecommendedApps(category?: string): Promise<FlatpakApp[]> {
    const params = category ? `?category=${encodeURIComponent(category)}` : "";
    return api.get<FlatpakApp[]>(`/flatpak/apps/recommended${params}`);
  }

  /** 获取推荐分类 */
  async getRecommendedCategories(): Promise<string[]> {
    return api.get<string[]>("/flatpak/apps/categories");
  }

  /** 流式安装应用 */
  installAppStream(
    appId: string,
    onProgress: (line: string) => void,
    onComplete: (success: boolean, error?: string) => void
  ): () => void {
    return streamRequest(
      "/api/v1/flatpak/apps/install-stream",
      {
        method: "POST",
        body: JSON.stringify({ app_id: appId }),
      },
      onProgress,
      onComplete
    );
  }

  /** 卸载应用 */
  async uninstallApp(appId: string): Promise<void> {
    await api.post("/flatpak/apps/uninstall", { app_id: appId });
  }

  /** 运行应用 */
  async runApp(appId: string, args?: string[]): Promise<void> {
    await api.post("/flatpak/apps/run", { app_id: appId, args });
  }

  /** 获取应用图标 URL */
  getIconUrl(appId: string): string {
    return `/api/v1/flatpak/icons/${appId}`;
  }
}

export const flatpakService = new FlatpakService();
