/**
 * 文件同步 Service - 基于 TUS 协议的断点续传上传
 * API: /api/v1/sync/...
 */

import { api } from "$shared/services/api";
import * as tus from "tus-js-client";

// ==================== 类型定义 ====================

export interface SyncStatus {
  running: boolean;
  storage_path: string;
  total_files: number;
  total_size: number;
  uploading: number;
}

export interface SyncFile {
  id: string;
  filename: string;
  size: number;
  mime_type: string;
  sha256: string;
  path: string;
  status: string;
  created_at: string;
  updated_at: string;
}

export interface UploadSession {
  id: string;
  filename: string;
  size: number;
  offset: number;
  progress: number;
  created_at: string;
  expires_at: string;
}

export interface UploadProgress {
  file: File;
  uploadId: string;
  progress: number;
  bytesUploaded: number;
  bytesTotal: number;
  status: "pending" | "uploading" | "completed" | "failed" | "paused";
  error?: string;
}

export type UploadEventCallback = {
  onProgress?: (progress: UploadProgress) => void;
  onComplete?: (fileId: string) => void;
  onError?: (error: Error) => void;
};

// ==================== TUS 上传器 ====================

export class TusUploader {
  private upload: tus.Upload | null = null;
  private file: File;
  private callbacks: UploadEventCallback;
  private _isPaused = false;

  constructor(file: File, callbacks: UploadEventCallback = {}) {
    this.file = file;
    this.callbacks = callbacks;
  }

  get isPaused(): boolean {
    return this._isPaused;
  }

  async start(): Promise<void> {
    const token = localStorage.getItem("auth_token") || "";

    this.upload = new tus.Upload(this.file, {
      endpoint: "/api/v1/sync/upload",
      headers: {
        Authorization: `Bearer ${token}`,
      },
      metadata: {
        filename: this.file.name,
        filetype: this.file.type || "application/octet-stream",
        filesize: String(this.file.size),
      },
      chunkSize: 5 * 1024 * 1024, // 5MB 分块
      retryDelays: [0, 1000, 3000, 5000, 10000], // 重试延迟
      storeFingerprintForResuming: true, // 启用断点续传
      removeFingerprintOnSuccess: true,
      onProgress: (bytesUploaded, bytesTotal) => {
        const progress = bytesTotal > 0 ? bytesUploaded / bytesTotal : 0;
        this.callbacks.onProgress?.({
          file: this.file,
          uploadId: this.upload?.url || "",
          progress,
          bytesUploaded,
          bytesTotal,
          status: "uploading",
        });
      },
      onSuccess: () => {
        const uploadUrl = this.upload?.url || "";
        const fileId = uploadUrl.split("/").pop() || "";
        this.callbacks.onComplete?.(fileId);
      },
      onError: (error) => {
        this.callbacks.onError?.(error);
      },
    });

    // 检查是否有未完成的上传
    const previousUploads = await this.upload.findPreviousUploads();
    if (previousUploads.length > 0) {
      // 从上次中断处继续
      this.upload.resumeFromPreviousUpload(previousUploads[0]);
    }

    this._isPaused = false;
    this.upload.start();
  }

  pause(): void {
    if (this.upload && !this._isPaused) {
      this.upload.abort();
      this._isPaused = true;
    }
  }

  resume(): void {
    if (this.upload && this._isPaused) {
      this._isPaused = false;
      this.upload.start();
    }
  }

  abort(): void {
    if (this.upload) {
      this.upload.abort();
      this.upload = null;
    }
  }
}

// ==================== 上传队列管理 ====================

export class UploadQueue {
  private queue: Map<string, TusUploader> = new Map();
  private activeCount = 0;
  private maxConcurrent = 3;
  private pending: Array<{ id: string; uploader: TusUploader }> = [];

  onProgressUpdate?: (uploads: Map<string, UploadProgress>) => void;
  onComplete?: (fileId: string) => void;
  onError?: (fileId: string, error: Error) => void;

  private progresses: Map<string, UploadProgress> = new Map();

  add(files: FileList | File[]): void {
    const fileArray = Array.from(files);
    
    for (const file of fileArray) {
      const id = `${file.name}-${file.size}-${Date.now()}`;
      
      const uploader = new TusUploader(file, {
        onProgress: (progress) => {
          this.progresses.set(id, progress);
          this.onProgressUpdate?.(new Map(this.progresses));
        },
        onComplete: (fileId) => {
          this.progresses.delete(id);
          this.queue.delete(id);
          this.activeCount--;
          this.processNext();
          this.onComplete?.(fileId);
          this.onProgressUpdate?.(new Map(this.progresses));
        },
        onError: (error) => {
          const progress = this.progresses.get(id);
          if (progress) {
            progress.status = "failed";
            progress.error = error.message;
            this.progresses.set(id, progress);
          }
          this.activeCount--;
          this.processNext();
          this.onError?.(id, error);
          this.onProgressUpdate?.(new Map(this.progresses));
        },
      });

      this.queue.set(id, uploader);
      this.progresses.set(id, {
        file,
        uploadId: id,
        progress: 0,
        bytesUploaded: 0,
        bytesTotal: file.size,
        status: "pending",
      });

      if (this.activeCount < this.maxConcurrent) {
        this.startUpload(id, uploader);
      } else {
        this.pending.push({ id, uploader });
      }
    }

    this.onProgressUpdate?.(new Map(this.progresses));
  }

  private startUpload(id: string, uploader: TusUploader): void {
    this.activeCount++;
    const progress = this.progresses.get(id);
    if (progress) {
      progress.status = "uploading";
      this.progresses.set(id, progress);
    }
    uploader.start();
  }

  private processNext(): void {
    if (this.pending.length > 0 && this.activeCount < this.maxConcurrent) {
      const next = this.pending.shift();
      if (next) {
        this.startUpload(next.id, next.uploader);
      }
    }
  }

  pause(id: string): void {
    const uploader = this.queue.get(id);
    if (uploader) {
      uploader.pause();
      const progress = this.progresses.get(id);
      if (progress) {
        progress.status = "paused";
        this.progresses.set(id, progress);
        this.onProgressUpdate?.(new Map(this.progresses));
      }
    }
  }

  resume(id: string): void {
    const uploader = this.queue.get(id);
    if (uploader) {
      uploader.resume();
      const progress = this.progresses.get(id);
      if (progress) {
        progress.status = "uploading";
        this.progresses.set(id, progress);
        this.onProgressUpdate?.(new Map(this.progresses));
      }
    }
  }

  cancel(id: string): void {
    const uploader = this.queue.get(id);
    if (uploader) {
      uploader.abort();
      this.queue.delete(id);
      this.progresses.delete(id);
      this.activeCount--;
      this.processNext();
      this.onProgressUpdate?.(new Map(this.progresses));
    }
  }

  cancelAll(): void {
    for (const [id, uploader] of this.queue) {
      uploader.abort();
    }
    this.queue.clear();
    this.progresses.clear();
    this.pending = [];
    this.activeCount = 0;
    this.onProgressUpdate?.(new Map(this.progresses));
  }

  getProgress(): Map<string, UploadProgress> {
    return new Map(this.progresses);
  }

  get size(): number {
    return this.queue.size;
  }

  get uploadingCount(): number {
    return this.activeCount;
  }
}

// ==================== 同步服务 ====================

class SyncService {
  private base = "/sync";

  async getStatus(): Promise<SyncStatus> {
    const res = await api.get<any>(`${this.base}/status`);
    return res.data ?? res;
  }

  async listFiles(params?: { limit?: number; offset?: number }): Promise<{ files: SyncFile[]; total: number }> {
    const res = await api.get<any>(`${this.base}/files`, params);
    return {
      files: res.data ?? [],
      total: res.total ?? 0,
    };
  }

  async getFile(id: string): Promise<SyncFile> {
    const res = await api.get<any>(`${this.base}/files/${id}`);
    return res.data ?? res;
  }

  async deleteFile(id: string): Promise<void> {
    await api.delete(`${this.base}/files/${id}`);
  }

  getDownloadUrl(id: string): string {
    const token = localStorage.getItem("auth_token") || "";
    return `/api/v1${this.base}/files/${id}/download?token=${encodeURIComponent(token)}`;
  }

  async listUploads(): Promise<UploadSession[]> {
    const res = await api.get<any>(`${this.base}/uploads`);
    return res.data ?? [];
  }

  async getUpload(id: string): Promise<UploadSession> {
    const res = await api.get<any>(`${this.base}/uploads/${id}`);
    return res.data ?? res;
  }

  // 创建上传队列
  createUploadQueue(): UploadQueue {
    return new UploadQueue();
  }

  // 创建单个上传器
  createUploader(file: File, callbacks?: UploadEventCallback): TusUploader {
    return new TusUploader(file, callbacks);
  }
}

export const syncService = new SyncService();
