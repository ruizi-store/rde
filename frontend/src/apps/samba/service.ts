/**
 * Samba Service - Samba 共享管理
 * API: /api/v1/samba/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface SambaShare {
  name: string;
  path: string;
  comment?: string;
  browseable: boolean;
  writable: boolean;
  guest_ok: boolean;
  valid_users?: string[];
  write_list?: string[];
  read_only?: boolean;
}

export interface SambaUser {
  username: string;
  enabled: boolean;
}

export interface SambaGlobalConfig {
  workgroup: string;
  server_string?: string;
  netbios_name?: string;
  security?: string;
  map_to_guest?: string;
  log_file?: string;
  max_log_size?: number;
  passdb_backend?: string;
}

export interface ServiceStatus {
  running: boolean;
  enabled: boolean;
  version?: string;
}

export interface SambaSession {
  pid: number;
  username: string;
  group: string;
  machine: string;
  connected_at: string;
  protocol?: string;
}

export interface SessionsInfo {
  sessions: SambaSession[];
  total: number;
}

export interface CreateShareRequest {
  name: string;
  path: string;
  comment?: string;
  browseable?: boolean;
  writable?: boolean;
  guest_ok?: boolean;
  valid_users?: string[];
}

export interface UpdateShareRequest {
  path?: string;
  comment?: string;
  browseable?: boolean;
  writable?: boolean;
  guest_ok?: boolean;
  valid_users?: string[];
}

export interface AddUserRequest {
  username: string;
  password: string;
}

// ==================== 服务实现 ====================

class SambaService {
  private base = "/samba";

  // 服务状态
  async getServiceStatus(): Promise<ServiceStatus> {
    return api.get<ServiceStatus>(`${this.base}/status`);
  }

  async startService(): Promise<void> {
    await api.post(`${this.base}/start`);
  }

  async stopService(): Promise<void> {
    await api.post(`${this.base}/stop`);
  }

  async restartService(): Promise<void> {
    await api.post(`${this.base}/restart`);
  }

  async reloadService(): Promise<void> {
    await api.post(`${this.base}/reload`);
  }

  // 共享管理
  async listShares(): Promise<SambaShare[]> {
    const res = await api.get<any>(`${this.base}/shares`);
    return res.data ?? res ?? [];
  }

  async createShare(req: CreateShareRequest): Promise<void> {
    await api.post(`${this.base}/shares`, req);
  }

  async updateShare(name: string, req: UpdateShareRequest): Promise<void> {
    await api.put(`${this.base}/shares/${name}`, req);
  }

  async deleteShare(name: string): Promise<void> {
    await api.delete(`${this.base}/shares/${name}`);
  }

  // 用户管理
  async listUsers(): Promise<SambaUser[]> {
    const res = await api.get<any>(`${this.base}/users`);
    return res.data ?? res ?? [];
  }

  async getSystemUsers(): Promise<string[]> {
    const res = await api.get<any>(`${this.base}/system-users`);
    return res.data ?? res ?? [];
  }

  async addUser(req: AddUserRequest): Promise<void> {
    await api.post(`${this.base}/users`, req);
  }

  async deleteUser(username: string): Promise<void> {
    await api.delete(`${this.base}/users/${username}`);
  }

  async setUserPassword(username: string, password: string): Promise<void> {
    await api.put(`${this.base}/users/${username}/password`, { password });
  }

  // 全局配置
  async getGlobalConfig(): Promise<SambaGlobalConfig> {
    const res = await api.get<any>(`${this.base}/config`);
    return res.data ?? res;
  }

  async updateGlobalConfig(config: Partial<SambaGlobalConfig>): Promise<void> {
    await api.put(`${this.base}/config`, config);
  }

  // 会话管理
  async getSessions(): Promise<SessionsInfo> {
    const res = await api.get<any>(`${this.base}/sessions`);
    return res.data ?? res ?? { sessions: [], total: 0 };
  }

  async killSession(pid: number): Promise<void> {
    await api.delete(`${this.base}/sessions/${pid}`);
  }

  async killUserSessions(username: string): Promise<void> {
    await api.delete(`${this.base}/sessions/user/${username}`);
  }
}

export const sambaService = new SambaService();
