// 终端 WebSocket 服务
import { api } from "./api";

export interface TerminalSession {
  id: string;
  name: string;
  cols: number;
  rows: number;
  createdAt: string;
}

export interface TerminalSize {
  cols: number;
  rows: number;
}

interface ApiResponse<T> {
  success: boolean;
  data?: T;
  message?: string;
}

class TerminalService {
  private baseUrl = "/terminal";

  // 创建终端会话
  async createSession(cols: number = 80, rows: number = 24): Promise<ApiResponse<TerminalSession>> {
    try {
      return await api.post<ApiResponse<TerminalSession>>(this.baseUrl, { cols, rows });
    } catch (error) {
      console.error("创建终端会话失败", error);
      return { success: false, message: "创建终端会话失败" };
    }
  }

  // 获取终端会话列表
  async getSessions(): Promise<ApiResponse<TerminalSession[]>> {
    try {
      return await api.get<ApiResponse<TerminalSession[]>>(this.baseUrl);
    } catch (error) {
      console.error("获取终端会话列表失败", error);
      return { success: false, message: "获取终端会话列表失败" };
    }
  }

  // 关闭终端会话
  async closeSession(id: string): Promise<ApiResponse<void>> {
    try {
      return await api.delete<ApiResponse<void>>(`${this.baseUrl}/${id}`);
    } catch (error) {
      console.error("关闭终端会话失败", error);
      return { success: false, message: "关闭终端会话失败" };
    }
  }

  // 调整终端大小
  async resizeSession(id: string, cols: number, rows: number): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(`${this.baseUrl}/${id}/resize`, { cols, rows });
    } catch (error) {
      console.error("调整终端大小失败", error);
      return { success: false, message: "调整终端大小失败" };
    }
  }

  // 获取 WebSocket 连接 URL
  getWebSocketUrl(sessionId: string): string {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    const token = localStorage.getItem("auth_token") || "";
    return `${protocol}//${host}/api/v1/terminal/${sessionId}/ws?token=${token}`;
  }

  // 创建 WebSocket 连接
  createConnection(
    sessionId: string,
    onData: (data: string) => void,
    onClose?: () => void,
    onError?: (error: Event) => void,
  ): WebSocket {
    const url = this.getWebSocketUrl(sessionId);
    const ws = new WebSocket(url);

    ws.onmessage = (event) => {
      onData(event.data);
    };

    ws.onclose = () => {
      if (onClose) onClose();
    };

    ws.onerror = (error) => {
      console.error("WebSocket 错误", error);
      if (onError) onError(error);
    };

    return ws;
  }
}

export const terminalService = new TerminalService();
