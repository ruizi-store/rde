// Sudo 特权操作服务
// 通过白名单机制安全执行需要 sudo 权限的操作

import { api } from "./api";

// API 响应包装
interface ApiResponse<T> {
  success: boolean;
  message?: string;
  data?: T;
}

// 操作信息
export interface SudoAction {
  id: string;
  name: string;
  description: string;
  arg_count: number;
  dangerous: boolean;
}

// 执行请求
export interface SudoExecuteRequest {
  action_id: string;
  args?: string[];
  confirmed?: boolean;
}

// 执行响应
export interface SudoExecuteResponse {
  success: boolean;
  output?: string;
  error?: string;
  exit_code: number;
  duration_ms: number;
}

// 预览响应
export interface SudoPreviewResponse {
  action: SudoAction;
  command: string;
}

// 审计日志
export interface SudoAuditLog {
  id: number;
  timestamp: string;
  user_id: string;
  username: string;
  action_id: string;
  args: string;
  command: string;
  success: boolean;
  output?: string;
  error?: string;
  exit_code: number;
  duration_ms: number;
  client_ip: string;
}

// 审计日志列表响应
export interface SudoLogsResponse {
  items: SudoAuditLog[];
  total: number;
  page: number;
  size: number;
}

class SudoService {
  // 获取所有可用操作
  async getActions(): Promise<SudoAction[]> {
    const response = await api.get<ApiResponse<SudoAction[]>>("/sudo/actions");
    if (!response.success || !response.data) {
      throw new Error(response.message || "Failed to get sudo actions");
    }
    return response.data;
  }

  // 预览命令
  async preview(actionId: string, args?: string[]): Promise<SudoPreviewResponse> {
    const response = await api.post<ApiResponse<SudoPreviewResponse>>("/sudo/preview", {
      action_id: actionId,
      args: args || [],
    });
    if (!response.success || !response.data) {
      throw new Error(response.message || "Failed to preview command");
    }
    return response.data;
  }

  // 执行操作
  async execute(
    actionId: string,
    args?: string[],
    confirmed: boolean = false
  ): Promise<SudoExecuteResponse> {
    const response = await api.post<ApiResponse<SudoExecuteResponse>>("/sudo/execute", {
      action_id: actionId,
      args: args || [],
      confirmed,
    });
    if (!response.success || !response.data) {
      throw new Error(response.message || "Failed to execute command");
    }
    return response.data;
  }

  // 获取审计日志
  async getLogs(page: number = 1, size: number = 20): Promise<SudoLogsResponse> {
    const response = await api.get<ApiResponse<SudoLogsResponse>>(
      `/sudo/logs?page=${page}&size=${size}`
    );
    if (!response.success || !response.data) {
      throw new Error(response.message || "Failed to get audit logs");
    }
    return response.data;
  }

  // 便捷方法：重启服务
  async restartService(serviceName: string, confirmed: boolean = false): Promise<SudoExecuteResponse> {
    return this.execute("service_restart", [serviceName], confirmed);
  }

  // 便捷方法：启动服务
  async startService(serviceName: string): Promise<SudoExecuteResponse> {
    return this.execute("service_start", [serviceName]);
  }

  // 便捷方法：停止服务
  async stopService(serviceName: string, confirmed: boolean = false): Promise<SudoExecuteResponse> {
    return this.execute("service_stop", [serviceName], confirmed);
  }

  // 便捷方法：安装软件包
  async installPackage(packageName: string): Promise<SudoExecuteResponse> {
    return this.execute("apt_install", [packageName]);
  }

  // 便捷方法：Docker 启动容器
  async dockerStart(container: string): Promise<SudoExecuteResponse> {
    return this.execute("docker_start", [container]);
  }

  // 便捷方法：Docker 停止容器
  async dockerStop(container: string): Promise<SudoExecuteResponse> {
    return this.execute("docker_stop", [container]);
  }

  // 便捷方法：Docker 重启容器
  async dockerRestart(container: string): Promise<SudoExecuteResponse> {
    return this.execute("docker_restart", [container]);
  }

  // 便捷方法：重启系统
  async rebootSystem(confirmed: boolean = false): Promise<SudoExecuteResponse> {
    return this.execute("system_reboot", [], confirmed);
  }

  // 便捷方法：关机
  async shutdownSystem(confirmed: boolean = false): Promise<SudoExecuteResponse> {
    return this.execute("system_shutdown", [], confirmed);
  }
}

export const sudoService = new SudoService();
