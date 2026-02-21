// 特权操作服务 API
import { api } from "./api";
import { getValidToken } from "$shared/utils/auth";

// ===================== 类型定义 =====================

// 操作类别
export type ActionCategory = "apt" | "dkms" | "modprobe" | "docker" | "systemd";

// 风险等级
export type RiskLevel = "low" | "medium" | "high";

// 授权请求（来自后端 WebSocket 推送）
export interface AuthorizationRequest {
  id: string;
  package_id: string;
  package_name: string;
  category: ActionCategory;
  command: string;
  title: string;
  description: string;
  risk_level: RiskLevel;
  created_at: string;
  expires_at: string;
  stream_mode?: boolean; // 是否支持流式模式
  task_id?: string;      // 流式模式的任务 ID
}

// 执行请求
export interface ExecuteRequest {
  category: ActionCategory;
  args: Record<string, string>;
  title?: string;
  description?: string;
  timeout?: number;
}

// 执行响应
export interface ExecuteResponse {
  request_id: string;
  status: "pending" | "approved" | "rejected" | "completed" | "failed" | "timeout";
  exit_code?: number;
  stdout?: string;
  stderr?: string;
  error?: string;
}

// 授权响应（发送给后端）
export interface AuthorizationResponse {
  request_id: string;
  approved: boolean;
  remember?: boolean;
  stream_mode?: boolean; // 是否使用流式模式
}

// 授权处理结果
export interface AuthorizeResult {
  success: boolean;
  message?: string;
  approved?: boolean;
  stream_mode?: boolean;
  task_id?: string;
}

// ===================== 服务类 =====================

class PrivilegeService {
  // 获取等待中的授权请求
  async getPendingRequests(): Promise<{ success: boolean; data?: AuthorizationRequest[] }> {
    return api.get("/privilege/pending");
  }

  // 处理授权响应
  async authorize(response: AuthorizationResponse): Promise<AuthorizeResult> {
    const result = await api.post("/privilege/authorize", response);
    return {
      success: result.success === 200 || result.success === true,
      message: result.message,
      approved: result.data?.approved,
      stream_mode: result.data?.stream_mode,
      task_id: result.data?.task_id,
    };
  }

  // 清除记住的授权
  async clearRemembered(packageId?: string): Promise<{ success: boolean }> {
    const url = packageId
      ? `/privilege/remembered?package_id=${encodeURIComponent(packageId)}`
      : "/privilege/remembered";
    return api.delete(url);
  }

  // 连接 WebSocket 监听授权请求（复用 notification WebSocket）
  connectWebSocket(callback: (request: AuthorizationRequest) => void): WebSocket | null {
    if (typeof window === "undefined") return null;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const token = getValidToken();
    if (!token) {
      return null;
    }

    // 复用 notification 的 WebSocket 端点
    const wsUrl = `${protocol}//${window.location.host}/api/v1/notifications/ws?token=${encodeURIComponent(token)}`;

    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        // 监听特权授权请求事件
        if (data.type === "privilege.authorization_request") {
          callback(data.data as AuthorizationRequest);
        }
      } catch (e) {
        console.error("解析 WebSocket 消息失败", e);
      }
    };

    ws.onerror = () => {
      // WebSocket 错误不打印到控制台，认证失败由 onclose 处理
    };

    ws.onclose = (event) => {
      if (event.code === 1008 || event.reason?.toLowerCase().includes("unauthorized")) {
        console.debug("Privilege WebSocket：未认证，跳过连接");
      }
    };

    return ws;
  }
}

// ===================== 辅助信息 =====================

// 类别显示信息
export const categoryInfo: Record<ActionCategory, { name: string; icon: string; color: string }> = {
  apt: {
    name: "安装系统软件包",
    icon: "mdi:package-variant-closed",
    color: "#22c55e", // green
  },
  dkms: {
    name: "DKMS 内核模块",
    icon: "mdi:cogs",
    color: "#f59e0b", // amber
  },
  modprobe: {
    name: "加载内核模块",
    icon: "mdi:memory",
    color: "#f59e0b", // amber
  },
  docker: {
    name: "Docker 容器",
    icon: "mdi:docker",
    color: "#3b82f6", // blue
  },
  systemd: {
    name: "系统服务",
    icon: "mdi:cog-play",
    color: "#8b5cf6", // purple
  },
};

// 风险等级显示信息
export const riskLevelInfo: Record<RiskLevel, { name: string; color: string; bgColor: string }> = {
  low: {
    name: "低风险",
    color: "#22c55e",
    bgColor: "rgba(34, 197, 94, 0.1)",
  },
  medium: {
    name: "中风险",
    color: "#f59e0b",
    bgColor: "rgba(245, 158, 11, 0.1)",
  },
  high: {
    name: "高风险",
    color: "#ef4444",
    bgColor: "rgba(239, 68, 68, 0.1)",
  },
};

// ===================== 任务相关类型 =====================

// 任务状态
export type TaskStatus = "pending" | "running" | "completed" | "failed" | "cancelled";

// 后台任务
export interface BackgroundTask {
  id: string;
  package_id: string;
  package_name: string;
  category: ActionCategory;
  title: string;
  description: string;
  command: string;
  status: TaskStatus;
  exit_code?: number;
  error?: string;
  progress: number;
  created_at: string;
  started_at?: string;
  finished_at?: string;
}

// 任务输出消息
export interface TaskOutputMessage {
  type: "init" | "output" | "status" | "ping";
  data: BackgroundTask | string;
}

// ===================== 任务服务扩展 =====================

class TaskService {
  // 获取所有任务
  async getTasks(): Promise<{ success: boolean; data?: BackgroundTask[] }> {
    return api.get("/privilege/stream/tasks");
  }

  // 获取单个任务
  async getTask(taskId: string): Promise<{ success: boolean; data?: BackgroundTask }> {
    return api.get(`/privilege/stream/tasks/${taskId}`);
  }

  // 取消任务
  async cancelTask(taskId: string): Promise<{ success: boolean }> {
    return api.post(`/privilege/stream/tasks/${taskId}/cancel`);
  }

  // 移除任务
  async removeTask(taskId: string): Promise<{ success: boolean }> {
    return api.delete(`/privilege/stream/tasks/${taskId}`);
  }

  // 连接任务输出 WebSocket
  connectTaskOutput(
    taskId: string,
    callbacks: {
      onInit?: (task: BackgroundTask) => void;
      onOutput?: (data: string) => void;
      onStatus?: (task: BackgroundTask) => void;
      onError?: (error: Event) => void;
      onClose?: () => void;
    }
  ): WebSocket | null {
    if (typeof window === "undefined") return null;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/v1/privilege/stream/tasks/${taskId}/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as TaskOutputMessage;
        switch (msg.type) {
          case "init":
            callbacks.onInit?.(msg.data as BackgroundTask);
            break;
          case "output":
            callbacks.onOutput?.(msg.data as string);
            break;
          case "status":
            callbacks.onStatus?.(msg.data as BackgroundTask);
            break;
          case "ping":
            // 心跳，忽略
            break;
        }
      } catch (e) {
        console.error("解析任务输出消息失败", e);
      }
    };

    ws.onerror = (error) => {
      callbacks.onError?.(error);
    };

    ws.onclose = () => {
      callbacks.onClose?.();
    };

    return ws;
  }

  // 连接全局任务更新 WebSocket
  connectTaskUpdates(
    callbacks: {
      onInit?: (tasks: BackgroundTask[]) => void;
      onUpdate?: (task: BackgroundTask) => void;
      onError?: (error: Event) => void;
      onClose?: () => void;
    }
  ): WebSocket | null {
    if (typeof window === "undefined") return null;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/v1/privilege/stream/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        if (msg.type === "init") {
          callbacks.onInit?.(msg.data as BackgroundTask[]);
        } else if (msg.type === "update") {
          callbacks.onUpdate?.(msg.data as BackgroundTask);
        }
      } catch (e) {
        console.error("解析任务更新消息失败", e);
      }
    };

    ws.onerror = callbacks.onError;
    ws.onclose = callbacks.onClose;

    return ws;
  }
}

// 导出单例
export const privilegeService = new PrivilegeService();
export const taskService = new TaskService();
