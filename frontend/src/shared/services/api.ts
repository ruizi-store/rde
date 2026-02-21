// API 客户端基础封装
// 统一处理请求、响应、错误、Token

import { isTokenExpired } from "$shared/utils/auth";

export interface ApiResponse<T = unknown> {
  success: boolean;
  message: string;
  data?: T;
}

export interface ApiError {
  status: number;
  message: string;
  code?: string;
}

// 全局 401 处理器（由应用层设置）
let onUnauthorized: (() => void) | null = null;

export function setUnauthorizedHandler(handler: () => void): void {
  onUnauthorized = handler;
}

export class ApiClient {
  private baseUrl: string;
  private token: string | null = null;
  private _refreshing: Promise<boolean> | null = null;

  constructor(baseUrl: string = "/api/v1") {
    this.baseUrl = baseUrl;
    // 从 localStorage 恢复 token
    if (typeof window !== "undefined") {
      this.token = localStorage.getItem("auth_token");
    }
  }

  // 设置 Token
  setToken(token: string | null): void {
    this.token = token;
    if (typeof window !== "undefined") {
      if (token) {
        localStorage.setItem("auth_token", token);
      } else {
        localStorage.removeItem("auth_token");
      }
    }
  }

  // 获取 Token
  getToken(): string | null {
    return this.token;
  }

  // 检查是否已登录
  isAuthenticated(): boolean {
    return !!this.token;
  }

  // 清除认证并触发登出
  clearAuthAndRedirect(): void {
    this.setToken(null);
    if (onUnauthorized) {
      onUnauthorized();
    }
  }

  // 构建请求头
  private getHeaders(contentType?: string): HeadersInit {
    const headers: HeadersInit = {};

    if (contentType) {
      headers["Content-Type"] = contentType;
    }

    // 每次请求时从 localStorage 读取最新的 token，并检查是否过期
    const token = typeof window !== "undefined" ? localStorage.getItem("auth_token") : this.token;
    if (token) {
      if (isTokenExpired(token)) {
        // token 过期，不发送请求，直接清除并跳转
        this.clearAuthAndRedirect();
        return headers;
      }
      headers["Authorization"] = `Bearer ${token}`;
    }

    return headers;
  }

  // 处理响应
  private async handleResponse<T>(response: Response, retryFn?: () => Promise<Response>): Promise<T> {
    if (!response.ok) {
      let errorMessage = `HTTP Error: ${response.status}`;
      try {
        const errorData = await response.json();
        errorMessage = errorData.error || errorData.message || errorMessage;
      } catch {
        // 忽略解析错误
      }

      // 处理 401 未授权：尝试刷新 token，成功则重试原请求
      if (response.status === 401 && retryFn) {
        const refreshed = await this.tryRefreshToken();
        if (refreshed) {
          const retryResponse = await retryFn();
          return this.handleResponse<T>(retryResponse);
        }
        this.clearAuthAndRedirect();
      } else if (response.status === 401) {
        this.clearAuthAndRedirect();
      }

      const error: ApiError = {
        status: response.status,
        message: errorMessage,
      };
      throw error;
    }

    // 204 No Content
    if (response.status === 204) {
      return {} as T;
    }

    return response.json();
  }

  // 尝试刷新 token（去重：多个并发 401 只刷新一次）
  private async tryRefreshToken(): Promise<boolean> {
    if (this._refreshing) return this._refreshing;
    this._refreshing = (async () => {
      try {
        const refreshToken = typeof window !== "undefined" ? localStorage.getItem("refresh_token") : null;
        if (!refreshToken) return false;
        const res = await fetch(`${this.baseUrl}/auth/refresh`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ refresh_token: refreshToken }),
        });
        if (!res.ok) return false;
        const json = await res.json();
        if (json.data?.access_token) {
          this.setToken(json.data.access_token);
          if (json.data.refresh_token) {
            localStorage.setItem("refresh_token", json.data.refresh_token);
          }
          return true;
        }
        return false;
      } catch {
        return false;
      } finally {
        this._refreshing = null;
      }
    })();
    return this._refreshing;
  }

  // GET 请求
  async get<T>(endpoint: string, params?: Record<string, string | number | boolean>): Promise<T> {
    let url = `${this.baseUrl}${endpoint}`;

    if (params) {
      const searchParams = new URLSearchParams();
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value));
        }
      });
      const queryString = searchParams.toString();
      if (queryString) {
        url += `?${queryString}`;
      }
    }

    const doFetch = () => fetch(url, {
      method: "GET",
      headers: this.getHeaders(),
    });
    const response = await doFetch();

    return this.handleResponse<T>(response, doFetch);
  }

  // POST 请求
  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    const doFetch = () => fetch(`${this.baseUrl}${endpoint}`, {
      method: "POST",
      headers: this.getHeaders("application/json"),
      body: data ? JSON.stringify(data) : undefined,
    });
    const response = await doFetch();

    return this.handleResponse<T>(response, doFetch);
  }

  // PUT 请求
  async put<T>(endpoint: string, data?: unknown): Promise<T> {
    const doFetch = () => fetch(`${this.baseUrl}${endpoint}`, {
      method: "PUT",
      headers: this.getHeaders("application/json"),
      body: data ? JSON.stringify(data) : undefined,
    });
    const response = await doFetch();

    return this.handleResponse<T>(response, doFetch);
  }

  // PATCH 请求
  async patch<T>(endpoint: string, data?: unknown): Promise<T> {
    const doFetch = () => fetch(`${this.baseUrl}${endpoint}`, {
      method: "PATCH",
      headers: this.getHeaders("application/json"),
      body: data ? JSON.stringify(data) : undefined,
    });
    const response = await doFetch();

    return this.handleResponse<T>(response, doFetch);
  }

  // DELETE 请求
  async delete<T>(endpoint: string, data?: unknown): Promise<T> {
    const doFetch = () => fetch(`${this.baseUrl}${endpoint}`, {
      method: "DELETE",
      headers: data ? this.getHeaders("application/json") : this.getHeaders(),
      body: data ? JSON.stringify(data) : undefined,
    });
    const response = await doFetch();

    return this.handleResponse<T>(response, doFetch);
  }

  // 上传文件
  async upload<T>(endpoint: string, formData: FormData): Promise<T> {
    const headers: HeadersInit = {};
    if (this.token) {
      headers["Authorization"] = `Bearer ${this.token}`;
    }
    // 不设置 Content-Type，让浏览器自动处理 multipart/form-data

    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: "POST",
      headers,
      body: formData,
    });

    return this.handleResponse<T>(response);
  }

  // 下载文件
  async download(endpoint: string, filename?: string): Promise<void> {
    const response = await fetch(`${this.baseUrl}${endpoint}`, {
      method: "GET",
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      throw new Error(`Download failed: ${response.status}`);
    }

    const blob = await response.blob();
    const url = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename || "download";
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(url);
    document.body.removeChild(a);
  }
}

// 导出单例
export const api = new ApiClient();
export const apiClient = api; // 别名
