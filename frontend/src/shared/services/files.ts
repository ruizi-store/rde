// 文件服务
// 处理文件和目录操作

import { api } from "./api";

export interface FileInfo {
  name: string;
  path: string;
  size: number;
  is_dir: boolean;
  is_symlink?: boolean;
  link_target?: string;
  modified: string;
  created?: string;
  mode?: string;
  owner?: string;
  group?: string;
  mime_type?: string;
  extension?: string;
}

export interface FileListResponse {
  success: number | boolean;
  message: string;
  data?: {
    content: FileInfo[];
    total: number;
    index?: number;
    size?: number;
  };
}

export interface FileContentResponse {
  success: boolean;
  message: string;
  data?: {
    content: string;
    encoding: string;
  };
}

export interface CreateDirRequest {
  path: string;
  name: string;
}

export interface RenameRequest {
  path: string;
  new_name: string;
}

export interface MoveRequest {
  source: string[];
  destination: string;
}

export interface CopyRequest {
  source: string[];
  destination: string;
}

export interface DeleteRequest {
  paths: string[];
}

export interface SearchRequest {
  path: string;
  keyword: string;
  recursive?: boolean;
}

export interface DiskUsage {
  total: number;
  used: number;
  free: number;
  percent: number;
}

export interface BookmarkItem {
  icon: string;
  label: string;
  path: string;
}

export interface BookmarksResponse {
  default: BookmarkItem[];
  system: BookmarkItem[];
  home_path: string;
}

class FileService {
  // 获取文件列表
  async list(path: string = "/", showHidden: boolean = false): Promise<FileListResponse> {
    return api.get<FileListResponse>("/files/list", { path, show_hidden: showHidden });
  }

  // 获取文件内容（文本文件）
  async getContent(path: string): Promise<FileContentResponse> {
    const response = await api.get<{ success: number | boolean; message: string; data?: string }>(
      "/files/read",
      {
        path,
      },
    );
    // 转换格式以兼容前端
    return {
      success: response.success === 200 || response.success === true,
      message: response.message,
      data: response.data ? { content: response.data, encoding: "utf-8" } : undefined,
    };
  }

  // 保存文件内容
  async saveContent(path: string, content: string): Promise<{ success: boolean; message: string }> {
    return api.put("/files/update", { path, content });
  }

  // 创建目录
  async createDir(path: string, name: string): Promise<{ success: boolean; message: string }> {
    return api.post("/files/mkdir", { path, name });
  }

  // 创建文件
  async createFile(path: string, name: string): Promise<{ success: boolean; message: string }> {
    return api.post("/files/create", { path, name });
  }

  // 重命名
  async rename(oldPath: string, newName: string): Promise<{ success: boolean; message: string }> {
    // 计算新路径：保留父目录 + 新名称
    const parentDir = oldPath.substring(0, oldPath.lastIndexOf("/")) || "/";
    const newPath = parentDir + "/" + newName;
    return api.put("/files/rename", { old_path: oldPath, new_path: newPath });
  }

  // 移动文件/目录
  async move(
    source: string[],
    destination: string,
  ): Promise<{ success: boolean; message: string }> {
    return api.post("/files/move", { source, destination });
  }

  // 复制文件/目录
  async copy(
    source: string[],
    destination: string,
  ): Promise<{ success: boolean; message: string }> {
    return api.post("/files/copy", { source, destination });
  }

  // 删除文件/目录
  async delete(paths: string[]): Promise<{ success: boolean; message: string }> {
    return api.delete("/files/delete", { paths });
  }

  // 上传文件
  async upload(
    path: string,
    files: File[],
    onProgress?: (percent: number) => void,
  ): Promise<{ success: boolean; message: string }> {
    const formData = new FormData();
    formData.append("path", path);
    files.forEach((file) => {
      formData.append("filename", file.name);
      formData.append("file", file);
    });

    // 使用 XMLHttpRequest 以支持进度回调
    if (onProgress) {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest();

        xhr.upload.addEventListener("progress", (e) => {
          if (e.lengthComputable) {
            onProgress(Math.round((e.loaded / e.total) * 100));
          }
        });

        xhr.addEventListener("load", () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            resolve(JSON.parse(xhr.responseText));
          } else {
            reject(new Error(`Upload failed: ${xhr.status}`));
          }
        });

        xhr.addEventListener("error", () => reject(new Error("Upload failed")));

        xhr.open("POST", "/api/v1/files/upload");
        const token = localStorage.getItem("auth_token");
        if (token) {
          xhr.setRequestHeader("Authorization", `Bearer ${token}`);
        }
        xhr.send(formData);
      });
    }

    return api.upload("/files/upload", formData);
  }

  // 下载文件
  async download(path: string, filename?: string): Promise<void> {
    const encodedPath = encodeURIComponent(path);
    return api.download(`/files/download?path=${encodedPath}`, filename);
  }

  // 获取缩略图
  getThumbnailUrl(path: string, size: number = 128): string {
    const encodedPath = encodeURIComponent(path);
    const token = localStorage.getItem("auth_token");
    return `/api/v1/files/thumbnail?path=${encodedPath}&size=${size}&token=${token || ""}`;
  }

  // 搜索文件
  async search(
    path: string,
    keyword: string,
    recursive: boolean = true,
  ): Promise<FileListResponse> {
    return api.get<FileListResponse>("/files/search", { path, keyword, recursive });
  }

  // 获取磁盘使用情况
  async getDiskUsage(path: string = "/"): Promise<{ success: boolean; data?: DiskUsage }> {
    return api.get("/files/disk-usage", { path });
  }

  // 压缩文件
  async compress(
    paths: string[],
    archiveName: string,
    format: "zip" | "tar.gz" = "zip",
  ): Promise<{ success: boolean; message: string }> {
    return api.post("/files/compress", { paths, archive_name: archiveName, format });
  }

  // 解压文件
  async extract(
    archivePath: string,
    destination: string,
  ): Promise<{ success: boolean; message: string }> {
    return api.post("/files/extract", { archive_path: archivePath, destination });
  }

  // 获取文件信息
  async getInfo(path: string): Promise<{ success: boolean; data?: FileInfo }> {
    return api.get("/files/info", { path });
  }

  // 检查路径是否存在
  async exists(path: string): Promise<boolean> {
    try {
      const response = await api.get<{ success: boolean; data?: { exists: boolean } }>(
        "/files/exists",
        { path },
      );
      return response.data?.exists ?? false;
    } catch {
      return false;
    }
  }

  // 复制或移动文件
  async copyMove(
    source: string[],
    destination: string,
    move: boolean = false,
  ): Promise<{ success: boolean; message: string }> {
    if (move) {
      return this.move(source, destination);
    } else {
      return this.copy(source, destination);
    }
  }

  // 获取文件预览 URL（直接流式返回文件内容）
  getPreviewUrl(path: string): string {
    const encodedPath = encodeURIComponent(path);
    const token = localStorage.getItem("auth_token");
    // 使用 download 接口，浏览器会根据 Content-Type 决定是否内嵌显示
    return `/api/v1/files/download?path=${encodedPath}&token=${token || ""}&inline=1`;
  }

  // 获取快捷访问书签
  async getBookmarks(): Promise<BookmarksResponse> {
    const resp = await api.get<{ success: number; data: BookmarksResponse }>("/files/bookmarks");
    return (
      resp.data || {
        default: [{ icon: "mdi:home", label: "主目录", path: "/" }],
        system: [],
        home_path: "/",
      }
    );
  }

  // 管理员提权
  async elevate(password: string): Promise<{ success: number; data?: { elevated: boolean; expires_at: string; duration: number }; message?: string; need_elevate?: boolean }> {
    return api.post("/files/elevate", { password });
  }

  // 撤销提权
  async revokeElevation(): Promise<{ success: number }> {
    return api.delete("/files/elevate");
  }

  // 获取提权状态
  async getElevationStatus(): Promise<{ success: number; data?: { elevated: boolean; remaining: number; expires_at?: string } }> {
    return api.get("/files/elevate");
  }
}

export const fileService = new FileService();
