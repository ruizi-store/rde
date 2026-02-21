// 通知服务 API
import { api } from "./api";
import { getValidToken } from "$shared/utils/auth";

// ===================== 类型定义 =====================

// 通知类别
export type NotificationCategory = "system" | "security" | "storage" | "backup" | "app" | "update";

// 严重级别
export type NotificationSeverity = "info" | "warning" | "error" | "critical";

// 站内通知
export interface Notification {
  id: string;
  user_id: string;
  category: NotificationCategory;
  severity: NotificationSeverity;
  title: string;
  content: string;
  link?: string;
  icon?: string;
  source?: string;
  is_read: boolean;
  read_at?: string;
  created_at: string;
  expires_at?: string;
}

// 通知列表响应
export interface NotificationListResponse {
  items: Notification[];
  total: number;
  page: number;
  page_size: number;
  unread_count: number;
}

// 通知设置
export interface NotificationSettings {
  enabled: boolean;
  desktop_notify: boolean;
  sound_enabled: boolean;
  dnd_enabled: boolean;
  dnd_from: string;
  dnd_to: string;
  filter_categories: string[];
  filter_severities: string[];
}

// 推送渠道类型
export type ChannelType = "email" | "webhook" | "telegram" | "wechat" | "bark" | "dingtalk";

// 推送渠道
export interface NotificationChannel {
  id: string;
  user_id: string;
  name: string;
  type: ChannelType;
  enabled: boolean;
  config: string;
  description: string;
  created_at: string;
  updated_at: string;
}

// 推送规则
export interface NotificationRule {
  id: string;
  user_id: string;
  name: string;
  channel_id: string;
  categories: string;
  severities: string;
  enabled: boolean;
  cooldown: number;
  last_sent_at?: string;
  created_at: string;
  updated_at: string;
}

// 推送历史
export interface NotificationHistory {
  id: string;
  channel_id: string;
  channel_type: ChannelType;
  category: NotificationCategory;
  title: string;
  content: string;
  recipient: string;
  status: "pending" | "sent" | "failed";
  error_msg?: string;
  sent_at?: string;
  created_at: string;
}

// 渠道配置
export interface EmailConfig {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  use_tls: boolean;
  from_address: string;
  from_name: string;
  to_addresses: string[];
}

export interface TelegramConfig {
  bot_token: string;
  chat_id: string;
}

export interface BarkConfig {
  server_url: string;
  device_key: string;
}

export interface WeChatConfig {
  webhook_url: string;
}

export interface DingTalkConfig {
  webhook_url: string;
  secret: string;
}

export interface WebhookConfig {
  url: string;
  method: string;
  headers: Record<string, string>;
  content_type: string;
  template: string;
}

// ===================== API 响应 =====================

interface ApiResponse<T> {
  success: number;
  message: string;
  data?: T;
  total?: number;
}

// ===================== 通知服务 =====================

class NotificationService {
  private baseUrl = "/notifications";

  // ==================== 站内通知 ====================

  // 获取通知列表
  async getNotifications(params?: {
    page?: number;
    page_size?: number;
    category?: string;
    severity?: string;
    is_read?: boolean;
    start_date?: string;
    end_date?: string;
  }): Promise<{ success: boolean; data?: NotificationListResponse; message?: string }> {
    try {
      const searchParams = new URLSearchParams();
      if (params?.page) searchParams.set("page", params.page.toString());
      if (params?.page_size) searchParams.set("page_size", params.page_size.toString());
      if (params?.category) searchParams.set("category", params.category);
      if (params?.severity) searchParams.set("severity", params.severity);
      if (params?.is_read !== undefined) searchParams.set("is_read", params.is_read.toString());
      if (params?.start_date) searchParams.set("start_date", params.start_date);
      if (params?.end_date) searchParams.set("end_date", params.end_date);

      const query = searchParams.toString();
      const url = query ? `${this.baseUrl}?${query}` : this.baseUrl;
      const response = await api.get<ApiResponse<NotificationListResponse>>(url);

      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取通知列表失败", error);
      return { success: false, message: "获取通知列表失败" };
    }
  }

  // 获取未读数量
  async getUnreadCount(): Promise<{ success: boolean; data?: number; message?: string }> {
    try {
      const response = await api.get<ApiResponse<{ count: number }>>(
        `${this.baseUrl}/unread-count`,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data.count };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取未读数量失败", error);
      return { success: false, message: "获取未读数量失败" };
    }
  }

  // 标记为已读
  async markAsRead(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.put<ApiResponse<void>>(`${this.baseUrl}/${id}/read`, {});
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("标记已读失败", error);
      return { success: false, message: "标记已读失败" };
    }
  }

  // 全部标记为已读
  async markAllAsRead(): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.put<ApiResponse<void>>(`${this.baseUrl}/read-all`, {});
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("全部标记已读失败", error);
      return { success: false, message: "全部标记已读失败" };
    }
  }

  // 删除通知
  async deleteNotification(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.delete<ApiResponse<void>>(`${this.baseUrl}/${id}`);
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("删除通知失败", error);
      return { success: false, message: "删除通知失败" };
    }
  }

  // 删除已读通知
  async deleteReadNotifications(): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.delete<ApiResponse<void>>(`${this.baseUrl}/read`);
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("删除已读通知失败", error);
      return { success: false, message: "删除已读通知失败" };
    }
  }

  // 清空所有通知
  async clearAll(): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.delete<ApiResponse<void>>(this.baseUrl);
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("清空通知失败", error);
      return { success: false, message: "清空通知失败" };
    }
  }

  // ==================== 通知设置 ====================

  // 获取通知设置
  async getSettings(): Promise<{
    success: boolean;
    data?: NotificationSettings;
    message?: string;
  }> {
    try {
      const response = await api.get<ApiResponse<NotificationSettings>>(`${this.baseUrl}/settings`);
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取通知设置失败", error);
      return { success: false, message: "获取通知设置失败" };
    }
  }

  // 更新通知设置
  async updateSettings(
    settings: Partial<NotificationSettings>,
  ): Promise<{ success: boolean; data?: NotificationSettings; message?: string }> {
    try {
      const response = await api.put<ApiResponse<NotificationSettings>>(
        `${this.baseUrl}/settings`,
        settings,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("更新通知设置失败", error);
      return { success: false, message: "更新通知设置失败" };
    }
  }

  // ==================== 推送渠道 ====================

  // 获取渠道列表
  async getChannels(): Promise<{
    success: boolean;
    data?: NotificationChannel[];
    message?: string;
  }> {
    try {
      const response = await api.get<ApiResponse<NotificationChannel[]>>(
        `${this.baseUrl}/channels`,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取渠道列表失败", error);
      return { success: false, message: "获取渠道列表失败" };
    }
  }

  // 获取渠道详情
  async getChannel(
    id: string,
  ): Promise<{ success: boolean; data?: NotificationChannel; message?: string }> {
    try {
      const response = await api.get<ApiResponse<NotificationChannel>>(
        `${this.baseUrl}/channels/${id}`,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取渠道详情失败", error);
      return { success: false, message: "获取渠道详情失败" };
    }
  }

  // 创建渠道
  async createChannel(data: {
    name: string;
    type: ChannelType;
    config: object;
    description?: string;
  }): Promise<{ success: boolean; data?: NotificationChannel; message?: string }> {
    try {
      const response = await api.post<ApiResponse<NotificationChannel>>(
        `${this.baseUrl}/channels`,
        data,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("创建渠道失败", error);
      return { success: false, message: "创建渠道失败" };
    }
  }

  // 更新渠道
  async updateChannel(
    id: string,
    data: {
      name?: string;
      config?: object;
      description?: string;
      enabled?: boolean;
    },
  ): Promise<{ success: boolean; data?: NotificationChannel; message?: string }> {
    try {
      const response = await api.put<ApiResponse<NotificationChannel>>(
        `${this.baseUrl}/channels/${id}`,
        data,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("更新渠道失败", error);
      return { success: false, message: "更新渠道失败" };
    }
  }

  // 删除渠道
  async deleteChannel(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.delete<ApiResponse<void>>(`${this.baseUrl}/channels/${id}`);
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("删除渠道失败", error);
      return { success: false, message: "删除渠道失败" };
    }
  }

  // 测试渠道
  async testChannel(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.post<ApiResponse<void>>(`${this.baseUrl}/channels/${id}/test`, {});
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("测试渠道失败", error);
      return { success: false, message: "测试渠道失败" };
    }
  }

  // ==================== 推送规则 ====================

  // 获取规则列表
  async getRules(): Promise<{ success: boolean; data?: NotificationRule[]; message?: string }> {
    try {
      const response = await api.get<ApiResponse<NotificationRule[]>>(`${this.baseUrl}/rules`);
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取规则列表失败", error);
      return { success: false, message: "获取规则列表失败" };
    }
  }

  // 创建规则
  async createRule(data: {
    name: string;
    channel_id: string;
    categories?: string[];
    severities?: string[];
    cooldown?: number;
  }): Promise<{ success: boolean; data?: NotificationRule; message?: string }> {
    try {
      const response = await api.post<ApiResponse<NotificationRule>>(`${this.baseUrl}/rules`, data);
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("创建规则失败", error);
      return { success: false, message: "创建规则失败" };
    }
  }

  // 更新规则
  async updateRule(
    id: string,
    data: {
      name?: string;
      categories?: string[];
      severities?: string[];
      cooldown?: number;
      enabled?: boolean;
    },
  ): Promise<{ success: boolean; data?: NotificationRule; message?: string }> {
    try {
      const response = await api.put<ApiResponse<NotificationRule>>(
        `${this.baseUrl}/rules/${id}`,
        data,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("更新规则失败", error);
      return { success: false, message: "更新规则失败" };
    }
  }

  // 删除规则
  async deleteRule(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      const response = await api.delete<ApiResponse<void>>(`${this.baseUrl}/rules/${id}`);
      return { success: response.success === 200, message: response.message };
    } catch (error) {
      console.error("删除规则失败", error);
      return { success: false, message: "删除规则失败" };
    }
  }

  // ==================== 推送历史 ====================

  // 获取推送历史
  async getHistory(
    page = 1,
    pageSize = 20,
  ): Promise<{ success: boolean; data?: NotificationHistory[]; total?: number; message?: string }> {
    try {
      const response = await api.get<ApiResponse<NotificationHistory[]>>(
        `${this.baseUrl}/history?page=${page}&page_size=${pageSize}`,
      );
      if (response.success === 200 && response.data) {
        return { success: true, data: response.data, total: response.total };
      }
      return { success: false, message: response.message };
    } catch (error) {
      console.error("获取推送历史失败", error);
      return { success: false, message: "获取推送历史失败" };
    }
  }

  // ==================== WebSocket ====================

  // 连接 WebSocket
  connectWebSocket(onMessage: (data: { type: string; data: unknown }) => void): WebSocket | null {
    try {
      const token = getValidToken();
      if (!token) {
        return null;
      }

      const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
      const ws = new WebSocket(
        `${protocol}//${window.location.host}/api/v1/notifications/ws?token=${encodeURIComponent(token)}`,
      );

      ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          onMessage(data);
        } catch (e) {
          console.error("解析 WebSocket 消息失败", e);
        }
      };

      ws.onerror = (error) => {
        console.error("WebSocket 错误", error);
      };

      ws.onclose = (event) => {
        // 检查是否是认证失败（HTTP 401 会导致 WebSocket 握手失败）
        // WebSocket 关闭代码 1008 表示 Policy Violation，通常用于认证失败
        if (event.code === 1008 || event.reason?.toLowerCase().includes("unauthorized") || event.reason?.toLowerCase().includes("invalid token")) {
          console.warn("WebSocket 认证失败，跳转登录页");
          // 触发全局 401 处理
          import("./api").then(({ api }) => api.clearAuthAndRedirect());
        }
      };

      return ws;
    } catch (error) {
      console.error("连接 WebSocket 失败", error);
      return null;
    }
  }
}

export const notificationService = new NotificationService();

// ===================== 辅助函数 =====================

// 类别信息
export const categoryInfo: Record<
  NotificationCategory,
  { name: string; icon: string; color: string }
> = {
  system: { name: "系统", icon: "mdi:monitor", color: "#3b82f6" },
  security: { name: "安全", icon: "mdi:shield-check", color: "#ef4444" },
  storage: { name: "存储", icon: "mdi:harddisk", color: "#f97316" },
  backup: { name: "备份", icon: "mdi:archive", color: "#22c55e" },
  app: { name: "应用", icon: "mdi:package-variant", color: "#a855f7" },
  update: { name: "更新", icon: "mdi:refresh", color: "#06b6d4" },
};

// 严重级别信息
export const severityInfo: Record<
  NotificationSeverity,
  { name: string; icon: string; color: string }
> = {
  info: { name: "信息", icon: "mdi:information", color: "#3b82f6" },
  warning: { name: "警告", icon: "mdi:alert", color: "#eab308" },
  error: { name: "错误", icon: "mdi:alert-circle", color: "#ef4444" },
  critical: { name: "紧急", icon: "mdi:alert-octagon", color: "#dc2626" },
};

// 渠道类型信息
export const channelTypeInfo: Record<ChannelType, { name: string; icon: string; color: string }> = {
  email: { name: "邮件", icon: "mdi:email", color: "#3b82f6" },
  telegram: { name: "Telegram", icon: "mdi:telegram", color: "#0088cc" },
  bark: { name: "Bark (iOS)", icon: "mdi:bell-ring", color: "#f97316" },
  wechat: { name: "企业微信", icon: "mdi:wechat", color: "#07c160" },
  dingtalk: { name: "钉钉", icon: "mdi:message-text", color: "#2196f3" },
  webhook: { name: "Webhook", icon: "mdi:webhook", color: "#6366f1" },
};

// 所有类别
export const allCategories: NotificationCategory[] = [
  "system",
  "security",
  "storage",
  "backup",
  "app",
  "update",
];

// 所有严重级别
export const allSeverities: NotificationSeverity[] = ["info", "warning", "error", "critical"];

// 所有渠道类型
export const allChannelTypes: ChannelType[] = [
  "email",
  "telegram",
  "bark",
  "wechat",
  "dingtalk",
  "webhook",
];
