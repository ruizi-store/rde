/**
 * Backup Service - 备份管理
 * API: /api/v1/backup/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export type BackupType = "full" | "incremental" | "config";
export type BackupStatus = "pending" | "running" | "success" | "failed" | "cancelled";
export type TargetType = "local" | "s3" | "webdav" | "sftp";

export interface BackupTask {
  id: string;
  name: string;
  description?: string;
  type: BackupType;
  sources: string[];
  target_type: TargetType;
  target_config: string;
  schedule?: string;
  retention: number;
  compression: boolean;
  encryption: boolean;
  enabled: boolean;
  last_run_at?: string;
  next_run_at?: string;
  created_at: string;
  updated_at: string;
}

export interface BackupRecord {
  id: string;
  task_id: string;
  task_name?: string;
  type: BackupType;
  size: number;
  file_count: number;
  file_path: string;
  checksum: string;
  status: BackupStatus;
  progress: number;
  message?: string;
  error?: string;
  started_at: string;
  completed_at?: string;
}

export interface RestoreStatus {
  id: string;
  record_id: string;
  status: BackupStatus;
  progress: number;
  current_file?: string;
  message?: string;
  error?: string;
  started_at: string;
  completed_at?: string;
}

export interface BackupOverview {
  total_tasks: number;
  enabled_tasks: number;
  total_records: number;
  total_size: number;
  last_backup_at?: string;
  next_backup_at?: string;
  success_count: number;
  failed_count: number;
}

export interface ExportableConfig {
  id: string;
  name: string;
  description: string;
  category: string;
}

export interface LocalTargetConfig {
  path: string;
}

export interface S3TargetConfig {
  endpoint?: string;
  region: string;
  bucket: string;
  prefix?: string;
  access_key_id: string;
  secret_access_key: string;
  use_ssl?: boolean;
}

export interface WebDAVTargetConfig {
  url: string;
  username: string;
  password: string;
  path?: string;
}

export interface SFTPTargetConfig {
  host: string;
  port: number;
  username: string;
  password?: string;
  private_key?: string;
  path?: string;
}

// ==================== P2P 迁移类型 ====================

export type MigrateRole = "source" | "target";
export type MigrateStatus = "pairing" | "connected" | "transferring" | "completed" | "failed" | "cancelled";

export interface MigrateSession {
  id: string;
  pair_code?: string;
  role: MigrateRole;
  status: MigrateStatus;
  remote_addr?: string;
  remote_host?: string;
  expires_at?: string;
  created_at: string;
}

export interface MigrateProgress {
  phase: string;
  total_size: number;
  transferred_size: number;
  total_files: number;
  transferred_files: number;
  current_file?: string;
  speed: number;
  eta: number;
  error?: string;
}

export interface MigrateContent {
  system_config: boolean;
  users: boolean;
  docker: boolean;
  network: boolean;
  samba: boolean;
  files: string[];
  apps: string[];
}

export interface CreateTaskRequest {
  name: string;
  description?: string;
  type: BackupType;
  sources: string[];
  target_type: TargetType;
  target_config: string;
  schedule?: string;
  retention?: number;
  compression?: boolean;
  encryption?: boolean;
}

export interface UpdateTaskRequest {
  name?: string;
  description?: string;
  sources?: string[];
  target_type?: TargetType;
  target_config?: string;
  schedule?: string;
  retention?: number;
  compression?: boolean;
  encryption?: boolean;
  enabled?: boolean;
}

export interface RestoreRequest {
  record_id: string;
  target_path?: string;
  selected_items?: string[];
  overwrite?: boolean;
}

export interface TargetTestRequest {
  type: TargetType;
  config: string;
}

export interface TargetTestResponse {
  success: boolean;
  message: string;
  free_space?: number;
}

// ==================== 服务实现 ====================

class BackupService {
  private base = "/backup";

  // 概览
  async getOverview(): Promise<BackupOverview> {
    const res = await api.get<any>(`${this.base}/overview`);
    return res.data ?? res;
  }

  // 任务管理
  async getTasks(page = 1, pageSize = 20): Promise<{ data: BackupTask[]; total: number }> {
    const res = await api.get<any>(`${this.base}/tasks`, { page, page_size: pageSize });
    return { data: res.data ?? [], total: res.total ?? 0 };
  }

  async getTask(id: string): Promise<BackupTask> {
    const res = await api.get<any>(`${this.base}/tasks/${id}`);
    return res.data ?? res;
  }

  async createTask(req: CreateTaskRequest): Promise<BackupTask> {
    const res = await api.post<any>(`${this.base}/tasks`, req);
    return res.data ?? res;
  }

  async updateTask(id: string, req: UpdateTaskRequest): Promise<BackupTask> {
    const res = await api.put<any>(`${this.base}/tasks/${id}`, req);
    return res.data ?? res;
  }

  async deleteTask(id: string): Promise<void> {
    await api.delete(`${this.base}/tasks/${id}`);
  }

  async runTask(id: string): Promise<BackupRecord> {
    const res = await api.post<any>(`${this.base}/tasks/${id}/run`);
    return res.data ?? res;
  }

  // 备份记录
  async getRecords(page = 1, pageSize = 20, taskId?: string): Promise<{ data: BackupRecord[]; total: number }> {
    const params: Record<string, string | number | boolean> = { page, page_size: pageSize };
    if (taskId) params.task_id = taskId;
    const res = await api.get<any>(`${this.base}/records`, params);
    return { data: res.data ?? [], total: res.total ?? 0 };
  }

  async getRecord(id: string): Promise<BackupRecord> {
    const res = await api.get<any>(`${this.base}/records/${id}`);
    return res.data ?? res;
  }

  async deleteRecord(id: string): Promise<void> {
    await api.delete(`${this.base}/records/${id}`);
  }

  // 还原
  async restore(req: RestoreRequest): Promise<RestoreStatus> {
    const res = await api.post<any>(`${this.base}/restore`, req);
    return res.data ?? res;
  }

  async getRestoreStatus(id: string): Promise<RestoreStatus> {
    const res = await api.get<any>(`${this.base}/restore/${id}/status`);
    return res.data ?? res;
  }

  // 目标测试
  async testTarget(req: TargetTestRequest): Promise<TargetTestResponse> {
    const res = await api.post<any>(`${this.base}/targets/test`, req);
    return res.data ?? res;
  }

  // 配置导入导出
  async getExportableConfigs(): Promise<ExportableConfig[]> {
    const res = await api.get<any>(`${this.base}/config/exportable`);
    return res.data ?? [];
  }

  async exportConfig(items: string[], encryption = false, password?: string): Promise<any> {
    const res = await api.post<any>(`${this.base}/config/export`, {
      include_items: items,
      encryption,
      password,
    });
    return res;
  }

  async importConfig(file: File, password?: string, overwrite = false): Promise<void> {
    const formData = new FormData();
    formData.append('file', file);
    if (password) formData.append('password', password);
    if (overwrite) formData.append('overwrite', 'true');
    await api.post(`${this.base}/config/import`, formData);
  }

  // P2P 迁移
  async generatePairCode(): Promise<{ session_id: string; pair_code: string; expires_at: string }> {
    const res = await api.post<any>(`${this.base}/migrate/pair`);
    return res.data ?? res;
  }

  async validatePairCode(pairCode: string): Promise<{ valid: boolean; session_id: string; expires_at: string }> {
    const res = await api.post<any>(`${this.base}/migrate/validate`, { pair_code: pairCode });
    return res.data ?? res;
  }

  async connectToSource(pairCode: string, targetUrl: string): Promise<MigrateSession> {
    const res = await api.post<any>(`${this.base}/migrate/connect`, {
      pair_code: pairCode,
      target_url: targetUrl,
    });
    return res.data ?? res;
  }

  async getMigrateSession(sessionId: string): Promise<MigrateSession> {
    const res = await api.get<any>(`${this.base}/migrate/session/${sessionId}`);
    return res.data ?? res;
  }

  async getMigrateProgress(sessionId: string): Promise<MigrateProgress> {
    const res = await api.get<any>(`${this.base}/migrate/session/${sessionId}/progress`);
    return res.data ?? res;
  }

  async startMigrateTransfer(sessionId: string, content: MigrateContent): Promise<void> {
    await api.post(`${this.base}/migrate/session/${sessionId}/start`, content);
  }

  async cancelMigrate(sessionId: string): Promise<void> {
    await api.post(`${this.base}/migrate/session/${sessionId}/cancel`);
  }

  getMigrateWsUrl(pairCode: string): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/api/v1${this.base}/migrate/ws/${pairCode}`;
  }
}

export const backupService = new BackupService();

// ==================== 工具函数 ====================

export function formatBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

export function getStatusColor(status: BackupStatus): string {
  switch (status) {
    case "success": return "text-green-500";
    case "failed": return "text-red-500";
    case "running": return "text-blue-500";
    case "pending": return "text-yellow-500";
    case "cancelled": return "text-gray-500";
    default: return "text-gray-400";
  }
}

export function getStatusLabel(status: BackupStatus): string {
  switch (status) {
    case "success": return "成功";
    case "failed": return "失败";
    case "running": return "运行中";
    case "pending": return "等待中";
    case "cancelled": return "已取消";
    default: return status;
  }
}

export function getTargetTypeLabel(type: TargetType): string {
  switch (type) {
    case "local": return "本地存储";
    case "s3": return "S3 / MinIO";
    case "webdav": return "WebDAV";
    case "sftp": return "SFTP";
    default: return type;
  }
}

export function getMigrateStatusLabel(status: MigrateStatus): string {
  switch (status) {
    case "pairing": return "等待配对";
    case "connected": return "已连接";
    case "transferring": return "传输中";
    case "completed": return "已完成";
    case "failed": return "失败";
    case "cancelled": return "已取消";
    default: return status;
  }
}

export function getMigrateStatusColor(status: MigrateStatus): string {
  switch (status) {
    case "pairing": return "text-yellow-500";
    case "connected": return "text-blue-500";
    case "transferring": return "text-blue-500";
    case "completed": return "text-green-500";
    case "failed": return "text-red-500";
    case "cancelled": return "text-gray-500";
    default: return "text-gray-400";
  }
}
