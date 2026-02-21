// 系统服务
// 处理系统信息、设置、通知

import { api } from "./api";

// 系统信息
export interface SystemInfo {
  hostname: string;
  os: string;
  arch: string;
  kernel: string;
  uptime: number;
  cpu_count: number;
  memory_total: number;
  memory_used: number;
  disk_total: number;
  disk_used: number;
  version: string;
  build_time?: string;
}

// CPU 使用率
export interface CpuUsage {
  percent: number;
  cores: number[];
  load_avg: [number, number, number];
}

// 内存使用
export interface MemoryUsage {
  total: number;
  used: number;
  free: number;
  cached: number;
  percent: number;
  swap_total: number;
  swap_used: number;
}

// 磁盘信息
export interface DiskInfo {
  path: string;
  total: number;
  used: number;
  free: number;
  percent: number;
  fs_type: string;
}

// 网络接口
export interface NetworkInterface {
  name: string;
  mac: string;
  ipv4: string[];
  ipv6: string[];
  rx_bytes: number;
  tx_bytes: number;
  speed?: number;
}

// 代理认证
export interface ProxyAuth {
  username: string;
  password: string;
}

// 系统代理配置
export type ProxyMode = "off" | "manual" | "pac";

export interface SystemProxyConfig {
  mode: ProxyMode;
  http_proxy: string;
  https_proxy: string;
  socks5: string;
  no_proxy: string;
  pac_url: string;
  auth?: ProxyAuth | null;
  docker_mirror?: string;
  docker_proxy_enabled?: boolean;
}

// 系统设置
export interface SystemSettings {
  language: string;
  timezone: string;
  hostname: string;
  auto_update: boolean;
  telemetry: boolean;
  ssh_enabled: boolean;
  ssh_port: number;
}

// 通知设置
export interface NotificationSettings {
  email_enabled: boolean;
  email_smtp_host?: string;
  email_smtp_port?: number;
  email_smtp_user?: string;
  email_from?: string;
  webhook_enabled: boolean;
  webhook_url?: string;
}

// 通知消息
export interface Notification {
  id: string;
  type: "info" | "warning" | "error" | "success";
  title: string;
  message: string;
  read: boolean;
  created_at: string;
}

class SystemService {
  // ==================== 系统信息 ====================

  // 获取系统信息
  async getInfo(): Promise<{ success: boolean; data?: SystemInfo }> {
    return api.get("/system/info");
  }

  // 获取 CPU 使用率
  async getCpuUsage(): Promise<{ success: boolean; data?: CpuUsage }> {
    return api.get("/system/cpu");
  }

  // 获取内存使用
  async getMemoryUsage(): Promise<{ success: boolean; data?: MemoryUsage }> {
    return api.get("/system/memory");
  }

  // 获取磁盘信息
  async getDiskInfo(): Promise<{ success: boolean; data?: DiskInfo[] }> {
    return api.get("/system/disks");
  }

  // 获取网络接口
  async getNetworkInterfaces(): Promise<{ success: boolean; data?: NetworkInterface[] }> {
    return api.get("/system/network");
  }

  // ==================== 代理设置 ====================

  // 获取代理配置
  async getProxyConfig(): Promise<{ success: boolean; data?: SystemProxyConfig }> {
    return api.get("/system/proxy");
  }

  // 保存代理配置
  async saveProxyConfig(config: SystemProxyConfig): Promise<{ success: boolean; message?: string }> {
    return api.post("/system/proxy", config);
  }

  // 测试代理连接
  async testProxy(params: { proxy_url: string; test_url: string }): Promise<{ success: boolean; message?: string }> {
    return api.post("/system/proxy/test", params);
  }

  // 获取实时资源监控数据
  async getResourceStats(): Promise<{
    success: boolean;
    data?: { cpu: CpuUsage; memory: MemoryUsage; disks: DiskInfo[] };
  }> {
    return api.get("/system/stats");
  }

  // ==================== 系统设置 ====================

  // 获取系统设置
  async getSettings(): Promise<{ success: boolean; data?: SystemSettings }> {
    return api.get("/system/settings");
  }

  // 更新系统设置
  async updateSettings(
    settings: Partial<SystemSettings>,
  ): Promise<{ success: boolean; message: string }> {
    return api.put("/system/settings", settings);
  }

  // 获取通知设置
  async getNotificationSettings(): Promise<{ success: boolean; data?: NotificationSettings }> {
    return api.get("/system/notification-settings");
  }

  // 更新通知设置
  async updateNotificationSettings(
    settings: Partial<NotificationSettings>,
  ): Promise<{ success: boolean; message: string }> {
    return api.put("/system/notification-settings", settings);
  }

  // ==================== 系统操作 ====================

  // 重启系统
  async reboot(): Promise<{ success: boolean; message: string }> {
    return api.post("/system/reboot");
  }

  // 关机
  async shutdown(): Promise<{ success: boolean; message: string }> {
    return api.post("/system/shutdown");
  }

  // ==================== SSH 服务管理 ====================

  // 获取 SSH 服务状态
  async getSSHStatus(): Promise<{
    success: number;
    data?: { running: boolean; enabled: boolean; port: number };
  }> {
    return api.get("/system/ssh");
  }

  // 启用 SSH 服务
  async enableSSH(): Promise<{ success: number; message: string }> {
    return api.post("/system/ssh/enable");
  }

  // 禁用 SSH 服务
  async disableSSH(): Promise<{ success: number; message: string }> {
    return api.post("/system/ssh/disable");
  }

  // 设置 SSH 开机自启
  async setSSHAutoStart(enabled: boolean): Promise<{ success: number; message: string }> {
    return api.post("/system/ssh/autostart", { enabled });
  }

  // ==================== 远程访问安全设置 ====================

  // 远程访问设置类型
  // 获取远程访问设置
  async getRemoteAccessSettings(): Promise<{
    success: number;
    data?: {
      ssh_enabled: boolean;
      ssh_running: boolean;
      ssh_port: number;
      terminal_enabled: boolean;
    };
  }> {
    return api.get("/system/security/remote-access");
  }

  // 更新远程访问设置
  async updateRemoteAccessSettings(settings: {
    ssh_enabled?: boolean;
    terminal_enabled?: boolean;
    password: string;
  }): Promise<{
    success: number;
    message: string;
    settings?: {
      ssh_enabled: boolean;
      ssh_running: boolean;
      ssh_port: number;
      terminal_enabled: boolean;
    };
  }> {
    return api.put("/system/security/remote-access", settings);
  }

  // ==================== 更新 ====================

  // 检查更新
  async checkUpdate(): Promise<{
    success: boolean;
    data?: { available: boolean; version?: string; changelog?: string };
  }> {
    return api.get("/system/update/check");
  }

  // 执行更新
  async performUpdate(): Promise<{ success: boolean; message: string }> {
    return api.post("/system/update/perform");
  }

  // ==================== 通知 ====================

  // 获取通知列表
  async getNotifications(
    unreadOnly: boolean = false,
  ): Promise<{ success: boolean; data?: Notification[] }> {
    return api.get("/notifications", { unread_only: unreadOnly });
  }

  // 标记通知已读
  async markNotificationRead(id: string): Promise<{ success: boolean; message: string }> {
    return api.post(`/notifications/${id}/read`);
  }

  // 标记所有通知已读
  async markAllNotificationsRead(): Promise<{ success: boolean; message: string }> {
    return api.post("/notifications/read-all");
  }

  // 删除通知
  async deleteNotification(id: string): Promise<{ success: boolean; message: string }> {
    return api.delete(`/notifications/${id}`);
  }

  // 清空所有通知
  async clearNotifications(): Promise<{ success: boolean; message: string }> {
    return api.delete("/notifications");
  }

  // ==================== 日志 ====================

  // 获取系统日志
  async getLogs(
    level?: string,
    limit: number = 100,
  ): Promise<{ success: boolean; data?: LogEntry[] }> {
    const params: Record<string, string | number> = { limit };
    if (level) params.level = level;
    return api.get("/system/logs", params);
  }

  // 获取审计日志
  async getAuditLogs(limit: number = 100): Promise<{ success: boolean; data?: AuditLog[] }> {
    return api.get("/system/audit-logs", { limit });
  }
}

export interface LogEntry {
  timestamp: string;
  level: "debug" | "info" | "warn" | "error";
  message: string;
  source?: string;
}

export interface AuditLog {
  id: string;
  timestamp: string;
  user: string;
  action: string;
  resource: string;
  ip: string;
  details?: string;
}

export const systemService = new SystemService();
