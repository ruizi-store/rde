/**
 * Android Service - 安卓设备管理
 *
 * 后端路由: /api/v1/android/...
 */

import { api, type ApiResponse } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface AndroidDevice {
  id: string;
  name: string;
  model?: string;
  product?: string;
  brand?: string;
  android_version?: string;
  api_level?: number;
  screen_size?: string;
  status: "device" | "offline" | "unauthorized" | "mirroring" | "connected" | "disconnected";
  transport?: string;
  address?: string;
  mirror_port?: number;
  battery?: number;
  is_charging?: boolean;
  connected_at?: string;
}

export interface AndroidApp {
  package_name: string;
  app_name: string;
  version: string;
  version_code?: number;
  icon?: string;
  installed: boolean;
  is_system?: boolean;
  is_enabled?: boolean;
  size?: number;
}

export interface APKInfo {
  package_name: string;
  app_name: string;
  version_name?: string;
  version_code?: number;
  icon?: string;
  size: number;
  min_sdk?: number;
  target_sdk?: number;
}

export interface AndroidFile {
  name: string;
  path: string;
  size: number;
  is_dir: boolean;
  mod_time?: string;
  perm?: string;
}

export interface MirrorSession {
  id: string;
  device_id: string;
  port: number;
  pid?: number;
  status: string;
}

export interface MirrorOptions {
  max_size?: number;
  bit_rate?: number;
  max_fps?: number;
  crop?: string;
  no_audio?: boolean;
}

// ==================== 环境安装类型 ====================

export interface EnvStatus {
  dkms_installed: boolean;
  headers_installed: boolean;
  binder_installed: boolean;
  binder_loaded: boolean;
  kernel_version: string;
  docker_installed: boolean;
  image_exists: boolean;
  container_running: boolean;
  container_exists: boolean;
  is_ready: boolean;
  only_need_start_container: boolean;
  required_steps: string[];
}

// ==================== 服务实现 ====================

class AndroidService {
  // ---- 设备管理 ----

  async getDevices(): Promise<AndroidDevice[]> {
    const res = await api.get<ApiResponse<AndroidDevice[]>>("/android/devices");
    return res.data ?? [];
  }

  async getDevice(serial: string): Promise<AndroidDevice> {
    return api.get<AndroidDevice>(`/android/devices/${serial}`);
  }

  async connect(address: string): Promise<void> {
    await api.post("/android/devices/connect", { serial: address });
  }

  async disconnect(serial: string): Promise<void> {
    await api.post("/android/devices/disconnect", { serial });
  }

  async reboot(serial: string): Promise<void> {
    await api.post(`/android/devices/${serial}/reboot`);
  }

  // ---- 应用管理 ----

  async getApps(serial: string): Promise<AndroidApp[]> {
    return api.get<AndroidApp[]>(`/android/devices/${serial}/apps`);
  }

  async installApp(serial: string, apkPath: string): Promise<void> {
    await api.post(`/android/devices/${serial}/apps/install`, { path: apkPath });
  }

  async uninstallApp(serial: string, packageName: string): Promise<void> {
    await api.post(`/android/devices/${serial}/apps/uninstall`, { package: packageName });
  }

  async launchApp(serial: string, packageName: string): Promise<void> {
    await api.post(`/android/devices/${serial}/apps/${packageName}/launch`);
  }

  async stopApp(serial: string, packageName: string): Promise<void> {
    await api.post(`/android/devices/${serial}/apps/${packageName}/stop`);
  }

  // ---- 文件管理 ----

  async listFiles(serial: string, path = "/"): Promise<AndroidFile[]> {
    return api.get<AndroidFile[]>(`/android/devices/${serial}/files?path=${encodeURIComponent(path)}`);
  }

  async pushFile(serial: string, localPath: string, remotePath: string): Promise<void> {
    await api.post(`/android/devices/${serial}/files/push`, { local_path: localPath, remote_path: remotePath });
  }

  async pullFile(serial: string, remotePath: string, localPath: string): Promise<void> {
    await api.post(`/android/devices/${serial}/files/pull`, { remote_path: remotePath, local_path: localPath });
  }

  // ---- Shell & 控制 ----

  async shell(serial: string, command: string): Promise<string> {
    const res = await api.post<{ output: string }>(`/android/devices/${serial}/shell`, { command });
    return res.output;
  }

  async input(serial: string, inputType: string, args: string): Promise<void> {
    await api.post(`/android/devices/${serial}/input`, { type: inputType, args });
  }

  async screenshot(serial: string): Promise<string> {
    const res = await api.post<ApiResponse<{ image: string }>>(`/android/devices/${serial}/screenshot`);
    return res.data?.image ?? "";
  }

  // ---- 投屏 ----

  async startMirror(serial: string, options?: MirrorOptions): Promise<MirrorSession> {
    return api.post<MirrorSession>(`/android/devices/${serial}/mirror/start`, options || {});
  }

  async stopMirror(serial: string): Promise<void> {
    await api.post(`/android/devices/${serial}/mirror/stop`);
  }

  // ---- 环境安装 ----

  async getEnvStatus(): Promise<EnvStatus> {
    const res = await api.get<ApiResponse<EnvStatus>>("/android/env/status");
    return res.data ?? ({} as EnvStatus);
  }

  /** 启动统一安装流程（binder + docker + 镜像 + 容器） */
  async startInstall(): Promise<void> {
    await api.post("/android/env/install");
  }

  /** 取消安装 */
  async cancelInstall(): Promise<void> {
    await api.post("/android/env/cancel");
  }

  // ---- 容器管理 ----

  /** 启动 Android 容器 */
  async startContainer(): Promise<void> {
    await api.post("/android/container/start");
  }

  /** 停止 Android 容器 */
  async stopContainer(): Promise<void> {
    await api.post("/android/container/stop");
  }

  /** 获取安装步骤列表 */
  async getInstallSteps(): Promise<{ steps: any[]; is_running: boolean; current: string }> {
    const res = await api.get<ApiResponse<{ steps: any[]; is_running: boolean; current: string }>>("/android/env/steps");
    return res.data ?? { steps: [], is_running: false, current: "" };
  }

  /** 连接安装进度 WebSocket */
  connectInstallWS(onMessage: (data: any) => void): WebSocket {
    const protocol = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${protocol}//${location.host}/api/v1/android/ws/install`);
    ws.onmessage = (e) => {
      try { onMessage(JSON.parse(e.data)); } catch {}
    };
    return ws;
  }

  // ---- 兼容 FileManager 的 APK 功能（回退到旧 pkg-api 如果可用）----

  async parseAPK(path: string): Promise<APKInfo> {
    const res = await fetch(`/api/v1/android/apk/parse`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({ path }),
    });
    const data = await res.json();
    if (!data.success) throw new Error(data.error || "解析 APK 失败");
    return data.data;
  }

  async installAPK(path: string): Promise<AndroidApp> {
    const res = await fetch(`/api/v1/android/apk/install`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...authHeaders() },
      body: JSON.stringify({ path }),
    });
    const data = await res.json();
    if (!data.success) throw new Error(data.error || "安装 APK 失败");
    return data.data;
  }

  /** 上传并安装 APK 文件（从本地电脑） */
  async uploadAPK(file: File): Promise<AndroidApp> {
    const formData = new FormData();
    formData.append("file", file);
    
    const res = await fetch(`/api/v1/android/apk/upload`, {
      method: "POST",
      headers: authHeaders(),
      body: formData,
    });
    const data = await res.json();
    if (!data.success) throw new Error(data.error || "上传安装 APK 失败");
    return data.data;
  }
}

function authHeaders(): Record<string, string> {
  const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : null;
  return token ? { Authorization: `Bearer ${token}` } : {};
}

// 导出单例
export const androidService = new AndroidService();
