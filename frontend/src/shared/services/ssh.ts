// SSH 远程连接服务
import { api } from "./api";

// SSH 连接配置
export interface SSHConnection {
  id: string;
  name: string;
  host: string;
  port: number;
  username: string;
  auth_type: "password" | "key";
  private_key?: string;
  group?: string;
  description?: string;
  created_at: string;
  updated_at: string;
}

// SSH 会话信息
export interface SSHSession {
  id: string;
  connection_id: string;
  connection_name: string;
  host: string;
  cols: number;
  rows: number;
  status: string;
  connected_at: string;
}

// 文件信息
export interface SFTPFile {
  name: string;
  path: string;
  is_dir: boolean;
  size: number;
  mode: string;
  mod_time: string;
}

// 传输任务
export interface TransferTask {
  id: string;
  session_id: string;
  type: "upload" | "download";
  remote_path: string;
  local_path: string;
  status: "pending" | "running" | "completed" | "failed" | "cancelled";
  progress: number;
  transferred: number;
  total_size: number;
  error?: string;
  created_at: string;
  started_at?: string;
  completed_at?: string;
}

// 传输进度更新
export interface TransferProgress {
  task_id: string;
  status: string;
  progress: number;
  transferred: number;
  total_size: number;
  error?: string;
}

// API 响应类型
interface ApiResponse<T> {
  success: boolean;
  data?: T;
  message?: string;
}

// 创建连接请求
export interface CreateConnectionRequest {
  name: string;
  host: string;
  port?: number;
  username: string;
  password?: string;
  auth_type: "password" | "key";
  private_key?: string;
  passphrase?: string;
  group?: string;
  description?: string;
}

// 更新连接请求
export interface UpdateConnectionRequest {
  name?: string;
  host?: string;
  port?: number;
  username?: string;
  password?: string;
  auth_type?: "password" | "key";
  private_key?: string;
  passphrase?: string;
  group?: string;
  description?: string;
}

// 测试连接请求
export interface TestConnectionRequest {
  host: string;
  port?: number;
  username: string;
  password?: string;
  auth_type: "password" | "key";
  private_key?: string;
  passphrase?: string;
}

class SSHService {
  private baseUrl = "/ssh";
  private sftpUrl = "/sftp";

  // ==================== 连接配置管理 ====================

  // 创建连接配置
  async createConnection(
    req: CreateConnectionRequest
  ): Promise<ApiResponse<SSHConnection>> {
    try {
      return await api.post<ApiResponse<SSHConnection>>(
        `${this.baseUrl}/connections`,
        req
      );
    } catch (error) {
      console.error("创建SSH连接失败", error);
      return { success: false, message: "创建SSH连接失败" };
    }
  }

  // 获取所有连接配置
  async listConnections(): Promise<ApiResponse<SSHConnection[]>> {
    try {
      return await api.get<ApiResponse<SSHConnection[]>>(
        `${this.baseUrl}/connections`
      );
    } catch (error) {
      console.error("获取SSH连接列表失败", error);
      return { success: false, message: "获取SSH连接列表失败" };
    }
  }

  // 获取单个连接配置
  async getConnection(id: string): Promise<ApiResponse<SSHConnection>> {
    try {
      return await api.get<ApiResponse<SSHConnection>>(
        `${this.baseUrl}/connections/${id}`
      );
    } catch (error) {
      console.error("获取SSH连接失败", error);
      return { success: false, message: "获取SSH连接失败" };
    }
  }

  // 更新连接配置
  async updateConnection(
    id: string,
    req: UpdateConnectionRequest
  ): Promise<ApiResponse<SSHConnection>> {
    try {
      return await api.put<ApiResponse<SSHConnection>>(
        `${this.baseUrl}/connections/${id}`,
        req
      );
    } catch (error) {
      console.error("更新SSH连接失败", error);
      return { success: false, message: "更新SSH连接失败" };
    }
  }

  // 删除连接配置
  async deleteConnection(id: string): Promise<ApiResponse<void>> {
    try {
      return await api.delete<ApiResponse<void>>(
        `${this.baseUrl}/connections/${id}`
      );
    } catch (error) {
      console.error("删除SSH连接失败", error);
      return { success: false, message: "删除SSH连接失败" };
    }
  }

  // 测试连接
  async testConnection(req: TestConnectionRequest): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(
        `${this.baseUrl}/connections/test`,
        req
      );
    } catch (error) {
      console.error("测试SSH连接失败", error);
      return { success: false, message: "测试SSH连接失败" };
    }
  }

  // 测试已保存的连接
  async testConnectionById(id: string): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(
        `${this.baseUrl}/connections/${id}/test`
      );
    } catch (error) {
      console.error("测试SSH连接失败", error);
      return { success: false, message: "测试SSH连接失败" };
    }
  }

  // ==================== SSH 会话管理 ====================

  // 创建SSH会话
  async createSession(
    connectionId: string,
    cols: number = 80,
    rows: number = 24
  ): Promise<ApiResponse<SSHSession>> {
    try {
      return await api.post<ApiResponse<SSHSession>>(`${this.baseUrl}/sessions`, {
        connection_id: connectionId,
        cols,
        rows,
      });
    } catch (error) {
      console.error("创建SSH会话失败", error);
      return { success: false, message: "创建SSH会话失败" };
    }
  }

  // 获取所有会话
  async listSessions(): Promise<ApiResponse<SSHSession[]>> {
    try {
      return await api.get<ApiResponse<SSHSession[]>>(`${this.baseUrl}/sessions`);
    } catch (error) {
      console.error("获取SSH会话列表失败", error);
      return { success: false, message: "获取SSH会话列表失败" };
    }
  }

  // 关闭会话
  async closeSession(id: string): Promise<ApiResponse<void>> {
    try {
      return await api.delete<ApiResponse<void>>(`${this.baseUrl}/sessions/${id}`);
    } catch (error) {
      console.error("关闭SSH会话失败", error);
      return { success: false, message: "关闭SSH会话失败" };
    }
  }

  // 调整终端大小
  async resizeSession(
    id: string,
    cols: number,
    rows: number
  ): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(
        `${this.baseUrl}/sessions/${id}/resize`,
        { cols, rows }
      );
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
    return `${protocol}//${host}/api/v1/ssh/sessions/${sessionId}/ws?token=${token}`;
  }

  // ==================== SFTP 操作 ====================

  // 列出目录
  async listDir(
    sessionId: string,
    remotePath: string = "/"
  ): Promise<ApiResponse<SFTPFile[]>> {
    try {
      const params = new URLSearchParams({ path: remotePath });
      return await api.get<ApiResponse<SFTPFile[]>>(
        `${this.sftpUrl}/${sessionId}/list?${params}`
      );
    } catch (error) {
      console.error("列出目录失败", error);
      return { success: false, message: "列出目录失败" };
    }
  }

  // 获取文件信息
  async stat(sessionId: string, remotePath: string): Promise<ApiResponse<SFTPFile>> {
    try {
      const params = new URLSearchParams({ path: remotePath });
      return await api.get<ApiResponse<SFTPFile>>(
        `${this.sftpUrl}/${sessionId}/stat?${params}`
      );
    } catch (error) {
      console.error("获取文件信息失败", error);
      return { success: false, message: "获取文件信息失败" };
    }
  }

  // 创建目录
  async mkdir(sessionId: string, remotePath: string): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(`${this.sftpUrl}/${sessionId}/mkdir`, {
        path: remotePath,
      });
    } catch (error) {
      console.error("创建目录失败", error);
      return { success: false, message: "创建目录失败" };
    }
  }

  // 重命名/移动文件
  async rename(
    sessionId: string,
    oldPath: string,
    newPath: string
  ): Promise<ApiResponse<void>> {
    try {
      return await api.post<ApiResponse<void>>(`${this.sftpUrl}/${sessionId}/rename`, {
        old_path: oldPath,
        new_path: newPath,
      });
    } catch (error) {
      console.error("重命名失败", error);
      return { success: false, message: "重命名失败" };
    }
  }

  // 删除文件/目录
  async delete(sessionId: string, paths: string[]): Promise<ApiResponse<void>> {
    try {
      return await api.delete<ApiResponse<void>>(
        `${this.sftpUrl}/${sessionId}/delete`,
        { paths }
      );
    } catch (error) {
      console.error("删除失败", error);
      return { success: false, message: "删除失败" };
    }
  }

  // 上传文件
  async uploadFiles(
    sessionId: string,
    remoteDir: string,
    files: File[]
  ): Promise<ApiResponse<{ uploaded: string[] }>> {
    try {
      const formData = new FormData();
      formData.append("remote_dir", remoteDir);
      for (const file of files) {
        formData.append("files", file);
      }
      return await api.upload<ApiResponse<{ uploaded: string[] }>>(
        `${this.sftpUrl}/${sessionId}/upload`,
        formData
      );
    } catch (error) {
      console.error("上传文件失败", error);
      return { success: false, message: "上传文件失败" };
    }
  }

  // 下载文件到本地路径
  async downloadFiles(
    sessionId: string,
    remotePaths: string[],
    localDir: string
  ): Promise<ApiResponse<{ downloaded: string[] }>> {
    try {
      return await api.post<ApiResponse<{ downloaded: string[] }>>(
        `${this.sftpUrl}/${sessionId}/download`,
        {
          remote_paths: remotePaths,
          local_dir: localDir,
        }
      );
    } catch (error) {
      console.error("下载文件失败", error);
      return { success: false, message: "下载文件失败" };
    }
  }

  // ==================== 传输队列 ====================

  // 创建传输任务
  async createTransfer(
    sessionId: string,
    type: "upload" | "download",
    remotePath: string,
    localPath: string
  ): Promise<ApiResponse<TransferTask>> {
    try {
      return await api.post<ApiResponse<TransferTask>>(`${this.sftpUrl}/transfers`, {
        session_id: sessionId,
        type,
        remote_path: remotePath,
        local_path: localPath,
      });
    } catch (error) {
      console.error("创建传输任务失败", error);
      return { success: false, message: "创建传输任务失败" };
    }
  }

  // 获取传输任务列表
  async listTransfers(sessionId?: string): Promise<ApiResponse<TransferTask[]>> {
    try {
      const params = sessionId ? `?session_id=${sessionId}` : "";
      return await api.get<ApiResponse<TransferTask[]>>(
        `${this.sftpUrl}/transfers${params}`
      );
    } catch (error) {
      console.error("获取传输列表失败", error);
      return { success: false, message: "获取传输列表失败" };
    }
  }

  // 取消传输任务
  async cancelTransfer(taskId: string): Promise<ApiResponse<void>> {
    try {
      return await api.delete<ApiResponse<void>>(
        `${this.sftpUrl}/transfers/${taskId}`
      );
    } catch (error) {
      console.error("取消传输任务失败", error);
      return { success: false, message: "取消传输任务失败" };
    }
  }

  // 清理已完成的传输任务
  async clearCompletedTransfers(): Promise<ApiResponse<{ cleared: number }>> {
    try {
      return await api.delete<ApiResponse<{ cleared: number }>>(
        `${this.sftpUrl}/transfers/completed`
      );
    } catch (error) {
      console.error("清理传输任务失败", error);
      return { success: false, message: "清理传输任务失败" };
    }
  }

  // 获取传输进度 WebSocket URL
  getTransferProgressUrl(): string {
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const host = window.location.host;
    const token = localStorage.getItem("auth_token") || "";
    return `${protocol}//${host}/api/v1/sftp/transfers/ws?token=${token}`;
  }
}

export const sshService = new SSHService();
