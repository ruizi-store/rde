/**
 * Download Service - 下载管理
 * API: /api/v1/download/...
 */

import { api } from "$shared/services/api";

// ==================== 类型定义 ====================

export interface DownloadTask {
  gid: string;
  name: string;
  status: "active" | "paused" | "waiting" | "complete" | "error" | "removed";
  total_length: number;
  completed_length: number;
  download_speed: number;
  upload_speed: number;
  progress: number;
  connections: number;
  dir?: string;
  error_code?: string;
  error_message?: string;
  files?: DownloadFile[];
  bittorrent?: BTInfo;
}

export interface DownloadFile {
  index: string;
  path: string;
  length: number;
  completed_length: number;
  selected: boolean;
}

export interface BTInfo {
  name?: string;
  comment?: string;
  mode?: string;
}

export interface DownloadStats {
  download_speed: number;
  upload_speed: number;
  num_active: number;
  num_waiting: number;
  num_stopped: number;
  num_stopped_total: number;
}

export interface TaskListResponse {
  active: DownloadTask[];
  waiting: DownloadTask[];
  stopped: DownloadTask[];
  stats: DownloadStats;
}

export interface DownloadHistory {
  id: number;
  gid: string;
  name: string;
  url?: string;
  size: number;
  save_path: string;
  status: string;
  error_message?: string;
  created_at: string;
  completed_at?: string;
  duration: number;
  avg_speed: number;
}

export interface DownloadSettings {
  download_dir: string;
  max_concurrent: number;
  max_conn_per_server: number;
  split: number;
  global_download_limit: number;
  global_upload_limit: number;
  seed_ratio: number;
  seed_time: number;
  enable_dht: boolean;
  notify_on_complete: boolean;
  auto_start: boolean;
}

export interface DownloadStatistics {
  total_downloads: number;
  completed_count: number;
  failed_count: number;
  total_size: number;
  today_downloads: number;
  today_size: number;
  week_downloads: number;
  week_size: number;
  average_speed: number;
  fastest_download: number;
}

export interface AddUriRequest {
  uris: string[];
  dir?: string;
  out?: string;
}

export interface AddTorrentRequest {
  torrent: string; // base64 encoded
  dir?: string;
}

// WebSocket 事件类型
export type DownloadEventType =
  | "task:added"
  | "task:progress"
  | "task:completed"
  | "task:paused"
  | "task:resumed"
  | "task:error"
  | "task:removed"
  | "stats:update"
  | "service:status";

export interface DownloadEvent {
  type: DownloadEventType;
  payload: unknown;
  time: number;
}

// ==================== 服务实现 ====================

class DownloadService {
  private base = "/download";

  // ========== 任务管理 ==========

  async getTasks(): Promise<TaskListResponse> {
    return api.get<TaskListResponse>(`${this.base}/tasks`);
  }

  async getTask(gid: string): Promise<DownloadTask> {
    return api.get<DownloadTask>(`${this.base}/tasks/${gid}`);
  }

  async addUri(request: AddUriRequest): Promise<{ gid: string }> {
    return api.post(`${this.base}/tasks/uri`, request);
  }

  async addTorrent(request: AddTorrentRequest): Promise<{ gid: string }> {
    return api.post(`${this.base}/tasks/torrent`, request);
  }

  async pauseTask(gid: string): Promise<void> {
    await api.post(`${this.base}/tasks/${gid}/pause`);
  }

  async resumeTask(gid: string): Promise<void> {
    await api.post(`${this.base}/tasks/${gid}/resume`);
  }

  async removeTask(gid: string): Promise<void> {
    await api.delete(`${this.base}/tasks/${gid}`);
  }

  async pauseAll(): Promise<void> {
    await api.post(`${this.base}/pause-all`);
  }

  async resumeAll(): Promise<void> {
    await api.post(`${this.base}/resume-all`);
  }

  async purgeResults(): Promise<void> {
    await api.delete(`${this.base}/results`);
  }

  // ========== 统计 ==========

  async getStats(): Promise<DownloadStats> {
    return api.get<DownloadStats>(`${this.base}/stats`);
  }

  async getStatistics(): Promise<DownloadStatistics> {
    return api.get<DownloadStatistics>(`${this.base}/statistics`);
  }

  // ========== 历史记录 ==========

  async getHistory(
    limit = 50,
    offset = 0,
    status?: string
  ): Promise<{ items: DownloadHistory[]; total: number }> {
    const params: Record<string, string> = {
      limit: String(limit),
      offset: String(offset),
    };
    if (status && status !== "all") {
      params.status = status;
    }
    return api.get(`${this.base}/history`, params);
  }

  async searchHistory(
    keyword: string,
    limit = 50
  ): Promise<{ items: DownloadHistory[] }> {
    return api.get(`${this.base}/history/search`, { q: keyword, limit: String(limit) });
  }

  async deleteHistoryItem(id: number): Promise<void> {
    await api.delete(`${this.base}/history/${id}`);
  }

  async clearHistory(): Promise<void> {
    await api.delete(`${this.base}/history`);
  }

  // ========== 设置 ==========

  async getSettings(): Promise<DownloadSettings> {
    return api.get<DownloadSettings>(`${this.base}/settings`);
  }

  async updateSettings(settings: Partial<DownloadSettings>): Promise<void> {
    await api.put(`${this.base}/settings`, settings);
  }

  // ========== 服务控制 ==========

  async startService(): Promise<void> {
    await api.post(`${this.base}/start`);
  }

  async stopService(): Promise<void> {
    await api.post(`${this.base}/stop`);
  }

  // ========== WebSocket ==========

  /**
   * 连接下载管理 WebSocket
   * @param callbacks 回调函数
   * @returns WebSocket 实例
   */
  connectWebSocket(callbacks: {
    onTask?: (type: DownloadEventType, task: DownloadTask) => void;
    onStats?: (stats: DownloadStats) => void;
    onServiceStatus?: (running: boolean) => void;
    onError?: (error: Event) => void;
    onClose?: () => void;
    onOpen?: () => void;
  }): WebSocket | null {
    if (typeof window === "undefined") return null;

    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const wsUrl = `${protocol}//${window.location.host}/api/v1/download/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      callbacks.onOpen?.();
    };

    ws.onmessage = (event) => {
      try {
        // 支持批量消息（用换行分隔）
        const messages = event.data.split("\n").filter(Boolean);
        for (const msg of messages) {
          const data: DownloadEvent = JSON.parse(msg);
          this.handleEvent(data, callbacks);
        }
      } catch (e) {
        console.error("解析下载事件失败", e);
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

  private handleEvent(
    event: DownloadEvent,
    callbacks: {
      onTask?: (type: DownloadEventType, task: DownloadTask) => void;
      onStats?: (stats: DownloadStats) => void;
      onServiceStatus?: (running: boolean) => void;
    }
  ) {
    switch (event.type) {
      case "task:added":
      case "task:progress":
      case "task:completed":
      case "task:paused":
      case "task:resumed":
      case "task:error":
      case "task:removed":
        callbacks.onTask?.(event.type, event.payload as DownloadTask);
        break;
      case "stats:update":
        callbacks.onStats?.(event.payload as DownloadStats);
        break;
      case "service:status":
        callbacks.onServiceStatus?.((event.payload as { running: boolean }).running);
        break;
    }
  }
}

export const downloadService = new DownloadService();
