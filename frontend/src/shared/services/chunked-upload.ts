/**
 * 分片上传服务
 * 支持大文件上传（最大10GB）
 * - 分片大小: 50MB
 * - 支持断点续传
 * - 自动重试
 * - 进度回调
 */

export interface ChunkedUploadOptions {
  /** 目标上传路径 */
  path: string;
  /** 要上传的文件 */
  file: File;
  /** 子路径（用于文件夹上传，如 "folder/sub/file.txt"），会拼接到 path 后面 */
  subPath?: string;
  /** 分片大小 (字节)，默认 50MB */
  chunkSize?: number;
  /** 每个分片的最大重试次数，默认 3 */
  maxRetries?: number;
  /** 单个分片超时时间 (毫秒)，默认 10 分钟 */
  timeout?: number;
  /** 进度回调 */
  onProgress?: (progress: UploadProgress) => void;
  /** 状态变化回调 */
  onStateChange?: (state: UploadState) => void;
  /** 错误回调 */
  onError?: (error: Error, chunk: number) => void;
}

export interface UploadProgress {
  /** 已上传字节数 */
  loaded: number;
  /** 总字节数 */
  total: number;
  /** 百分比 0-100 */
  percent: number;
  /** 当前分片索引 */
  currentChunk: number;
  /** 总分片数 */
  totalChunks: number;
  /** 上传速度 (bytes/s) */
  speed: number;
  /** 预计剩余时间 (秒) */
  remainingTime: number;
}

export type UploadState = "idle" | "preparing" | "uploading" | "paused" | "completed" | "error";

export interface ChunkedUploadResult {
  success: boolean;
  message: string;
  fileId?: string;
  path?: string;
}

// 最大文件大小 10GB
const MAX_FILE_SIZE = 10 * 1024 * 1024 * 1024;
// 默认分片大小 50MB
const DEFAULT_CHUNK_SIZE = 50 * 1024 * 1024;
// 默认超时 10 分钟
const DEFAULT_TIMEOUT = 10 * 60 * 1000;
// 默认重试次数
const DEFAULT_MAX_RETRIES = 3;

/**
 * 分片上传控制器
 */
export class ChunkedUploader {
  private options: Required<
    Omit<ChunkedUploadOptions, "onProgress" | "onStateChange" | "onError" | "subPath">
  > &
    Pick<ChunkedUploadOptions, "onProgress" | "onStateChange" | "onError" | "subPath">;

  private state: UploadState = "idle";
  private uploadedChunks: Set<number> = new Set();
  private uploadId: string = "";
  private abortController: AbortController | null = null;
  private isPaused = false;

  // 速度计算
  private startTime = 0;
  private lastProgressTime = 0;
  private lastProgressLoaded = 0;
  private currentSpeed = 0;

  constructor(options: ChunkedUploadOptions) {
    this.options = {
      chunkSize: DEFAULT_CHUNK_SIZE,
      maxRetries: DEFAULT_MAX_RETRIES,
      timeout: DEFAULT_TIMEOUT,
      ...options,
    };
  }

  /**
   * 获取当前状态
   */
  getState(): UploadState {
    return this.state;
  }

  /**
   * 开始上传
   */
  async start(): Promise<ChunkedUploadResult> {
    const { file, path, chunkSize } = this.options;

    // 验证文件大小
    if (file.size > MAX_FILE_SIZE) {
      return {
        success: false,
        message: `文件大小超过限制 (最大 ${formatSize(MAX_FILE_SIZE)})`,
      };
    }

    // 如果文件小于分片大小，使用普通上传
    if (file.size <= chunkSize) {
      return this.simpleUpload();
    }

    try {
      this.setState("preparing");
      this.startTime = Date.now();
      this.abortController = new AbortController();

      // 初始化分片上传
      const initResult = await this.initUpload();
      if (!initResult.success) {
        this.setState("error");
        return { success: false, message: initResult.message || "初始化上传失败" };
      }

      this.uploadId = initResult.uploadId!;
      this.setState("uploading");

      // 计算分片
      const totalChunks = Math.ceil(file.size / chunkSize);

      // 上传所有分片
      for (let i = 0; i < totalChunks; i++) {
        if (this.isPaused) {
          this.setState("paused");
          return { success: false, message: "上传已暂停" };
        }

        if (this.uploadedChunks.has(i)) {
          continue; // 跳过已上传的分片
        }

        const success = await this.uploadChunkWithRetry(i, totalChunks);
        if (!success) {
          this.setState("error");
          return { success: false, message: `分片 ${i + 1} 上传失败` };
        }

        this.uploadedChunks.add(i);
      }

      // 完成上传
      const completeResult = await this.completeUpload();
      if (completeResult.success) {
        this.setState("completed");
      } else {
        this.setState("error");
      }

      return completeResult;
    } catch (error) {
      this.setState("error");
      return {
        success: false,
        message: error instanceof Error ? error.message : "上传失败",
      };
    }
  }

  /**
   * 暂停上传
   */
  pause(): void {
    this.isPaused = true;
    this.abortController?.abort();
    this.setState("paused");
  }

  /**
   * 恢复上传
   */
  async resume(): Promise<ChunkedUploadResult> {
    this.isPaused = false;
    this.abortController = new AbortController();
    return this.start();
  }

  /**
   * 取消上传
   */
  async cancel(): Promise<void> {
    this.abortController?.abort();
    this.isPaused = false;

    if (this.uploadId) {
      try {
        await this.abortUpload();
      } catch {
        // 忽略取消错误
      }
    }

    this.setState("idle");
    this.reset();
  }

  /**
   * 普通上传（小文件）
   */
  private async simpleUpload(): Promise<ChunkedUploadResult> {
    const { file, path, subPath, timeout, maxRetries } = this.options;

    this.setState("uploading");
    this.startTime = Date.now();

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        const formData = new FormData();
        formData.append("path", path);
        // 如果有 subPath，传递完整的相对路径，让后端创建目录
        formData.append("filename", subPath || file.name);
        formData.append("file", file);

        const result = await this.fetchWithProgress(
          "/api/v1/files/upload",
          {
            method: "POST",
            body: formData,
            signal: AbortSignal.timeout(timeout),
          },
          (loaded) => this.updateProgress(loaded, file.size, 0, 1),
        );

        if (result.success === 200 || result.success === true) {
          this.setState("completed");
          return { success: true, message: "上传成功", path: result.data?.path };
        }

        if (attempt < maxRetries - 1) {
          await this.delay(1000 * (attempt + 1));
        }
      } catch (error) {
        if (attempt >= maxRetries - 1) {
          this.setState("error");
          return {
            success: false,
            message: error instanceof Error ? error.message : "上传失败",
          };
        }
        await this.delay(1000 * (attempt + 1));
      }
    }

    this.setState("error");
    return { success: false, message: "上传失败，已达最大重试次数" };
  }

  /**
   * 初始化分片上传
   */
  private async initUpload(): Promise<{ success: boolean; uploadId?: string; message?: string }> {
    const { file, path, subPath } = this.options;

    try {
      const response = await fetch("/api/v1/files/upload/init", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...this.getAuthHeader(),
        },
        body: JSON.stringify({
          path,
          // 如果有 subPath，传递完整的相对路径而不是文件名
          filename: subPath || file.name,
          size: file.size,
          chunk_size: this.options.chunkSize,
        }),
        signal: this.abortController?.signal,
      });

      const result = await response.json();

      if (result.success === 200 || result.success === true) {
        return { success: true, uploadId: result.data?.upload_id };
      }

      return { success: false, message: result.message || "初始化上传失败" };
    } catch (error) {
      return {
        success: false,
        message: error instanceof Error ? error.message : "初始化上传失败",
      };
    }
  }

  /**
   * 上传单个分片（带重试）
   */
  private async uploadChunkWithRetry(chunkIndex: number, totalChunks: number): Promise<boolean> {
    const { maxRetries } = this.options;

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        const success = await this.uploadChunk(chunkIndex, totalChunks);
        if (success) return true;

        if (attempt < maxRetries - 1) {
          this.options.onError?.(
            new Error(`分片 ${chunkIndex + 1} 上传失败，重试中...`),
            chunkIndex,
          );
          await this.delay(1000 * (attempt + 1));
        }
      } catch (error) {
        if (attempt >= maxRetries - 1) {
          this.options.onError?.(
            error instanceof Error ? error : new Error("上传失败"),
            chunkIndex,
          );
          return false;
        }
        await this.delay(1000 * (attempt + 1));
      }
    }

    return false;
  }

  /**
   * 上传单个分片
   */
  private async uploadChunk(chunkIndex: number, totalChunks: number): Promise<boolean> {
    const { file, chunkSize, timeout } = this.options;

    const start = chunkIndex * chunkSize;
    const end = Math.min(start + chunkSize, file.size);
    const chunk = file.slice(start, end);

    const formData = new FormData();
    formData.append("upload_id", this.uploadId);
    formData.append("chunk_index", String(chunkIndex));
    formData.append("chunk", chunk);

    const result = await this.fetchWithProgress(
      "/api/v1/files/upload/chunk",
      {
        method: "POST",
        body: formData,
        signal: AbortSignal.timeout(timeout),
      },
      (loaded) => {
        const totalLoaded = chunkIndex * chunkSize + loaded;
        this.updateProgress(totalLoaded, file.size, chunkIndex, totalChunks);
      },
    );

    return result.success === 200 || result.success === true;
  }

  /**
   * 完成分片上传
   */
  private async completeUpload(): Promise<ChunkedUploadResult> {
    try {
      const response = await fetch("/api/v1/files/upload/complete", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...this.getAuthHeader(),
        },
        body: JSON.stringify({ upload_id: this.uploadId }),
        signal: this.abortController?.signal,
      });

      const result = await response.json();

      if (result.success === 200 || result.success === true) {
        return {
          success: true,
          message: "上传成功",
          path: result.data?.path,
          fileId: result.data?.file_id,
        };
      }

      return { success: false, message: result.message || "完成上传失败" };
    } catch (error) {
      return {
        success: false,
        message: error instanceof Error ? error.message : "完成上传失败",
      };
    }
  }

  /**
   * 取消分片上传
   */
  private async abortUpload(): Promise<void> {
    await fetch("/api/v1/files/upload/abort", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...this.getAuthHeader(),
      },
      body: JSON.stringify({ upload_id: this.uploadId }),
    });
  }

  /**
   * 带进度的 fetch
   */
  private async fetchWithProgress(
    url: string,
    options: RequestInit,
    onProgress: (loaded: number) => void,
  ): Promise<any> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();

      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable) {
          onProgress(e.loaded);
        }
      });

      xhr.addEventListener("load", () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText));
          } catch {
            resolve({ success: true });
          }
        } else {
          reject(new Error(`HTTP ${xhr.status}: ${xhr.statusText}`));
        }
      });

      xhr.addEventListener("error", () => reject(new Error("网络错误")));
      xhr.addEventListener("timeout", () => reject(new Error("请求超时")));
      xhr.addEventListener("abort", () => reject(new Error("请求已取消")));

      xhr.open(options.method || "GET", url);

      // 设置认证头
      const token = localStorage.getItem("auth_token");
      if (token) {
        xhr.setRequestHeader("Authorization", `Bearer ${token}`);
      }

      // 设置超时
      xhr.timeout = this.options.timeout;

      // 绑定 abort signal
      if (options.signal) {
        options.signal.addEventListener("abort", () => xhr.abort());
      }

      xhr.send(options.body as any);
    });
  }

  /**
   * 更新进度
   */
  private updateProgress(
    loaded: number,
    total: number,
    currentChunk: number,
    totalChunks: number,
  ): void {
    const now = Date.now();

    // 计算速度
    if (now - this.lastProgressTime > 500) {
      const timeDiff = (now - this.lastProgressTime) / 1000;
      const loadedDiff = loaded - this.lastProgressLoaded;
      this.currentSpeed = loadedDiff / timeDiff;
      this.lastProgressTime = now;
      this.lastProgressLoaded = loaded;
    }

    // 计算剩余时间
    const remaining = total - loaded;
    const remainingTime = this.currentSpeed > 0 ? remaining / this.currentSpeed : 0;

    this.options.onProgress?.({
      loaded,
      total,
      percent: Math.round((loaded / total) * 100),
      currentChunk,
      totalChunks,
      speed: this.currentSpeed,
      remainingTime,
    });
  }

  /**
   * 设置状态
   */
  private setState(state: UploadState): void {
    this.state = state;
    this.options.onStateChange?.(state);
  }

  /**
   * 获取认证头
   */
  private getAuthHeader(): Record<string, string> {
    const token = localStorage.getItem("auth_token");
    return token ? { Authorization: `Bearer ${token}` } : {};
  }

  /**
   * 延迟
   */
  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }

  /**
   * 重置状态
   */
  private reset(): void {
    this.uploadedChunks.clear();
    this.uploadId = "";
    this.startTime = 0;
    this.lastProgressTime = 0;
    this.lastProgressLoaded = 0;
    this.currentSpeed = 0;
  }
}

/**
 * 格式化文件大小
 */
export function formatSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

/**
 * 格式化时间
 */
export function formatTime(seconds: number): string {
  if (seconds < 60) return `${Math.round(seconds)}秒`;
  if (seconds < 3600) return `${Math.round(seconds / 60)}分钟`;
  return `${Math.round(seconds / 3600)}小时${Math.round((seconds % 3600) / 60)}分钟`;
}

/**
 * 创建上传器实例
 */
export function createUploader(options: ChunkedUploadOptions): ChunkedUploader {
  return new ChunkedUploader(options);
}
