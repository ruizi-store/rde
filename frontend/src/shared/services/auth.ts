// 认证服务
// 处理登录、登出、用户信息

import { api } from "./api";

const DEVICE_TOKEN_KEY = "rde_device_token";

export interface LoginRequest {
  username: string;
  password: string;
  device_token?: string;
}

export interface LoginResponse {
  success?: boolean;
  message?: string;
  error?: string;
  data?: {
    access_token: string;
    refresh_token: string;
    expires_at: number;
    token_type: string;
    require_2fa?: boolean;
    temp_token?: string;
    device_token?: string; // 受信任设备令牌
    user?: {
      id: string;
      username: string;
      nickname?: string;
      role: string;
      status: string;
    };
  };
}

export interface UserInfo {
  id: number;
  username: string;
  role: string;
  email?: string;
  avatar?: string;
  created_at: string;
}

export interface UserInfoResponse {
  success: boolean;
  message: string;
  data?: UserInfo;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

class AuthService {
  // 登录
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    // 自动附加保存的设备信任令牌
    const deviceToken = typeof window !== "undefined" ? localStorage.getItem(DEVICE_TOKEN_KEY) : null;
    const requestData = {
      ...credentials,
      device_token: deviceToken || undefined,
    };
    
    const response = await api.post<LoginResponse>("/auth/login", requestData);

    // 需要 2FA 验证
    if (response.data?.require_2fa && response.data?.temp_token) {
      return {
        success: true,
        data: {
          ...response.data,
          access_token: "",
          refresh_token: "",
          expires_at: 0,
          token_type: "",
        },
      };
    }

    // 后端返回 {data: {access_token, refresh_token, user}} 格式
    if (response.data?.access_token) {
      api.setToken(response.data.access_token);
      // 保存 refresh_token 用于刷新
      if (typeof window !== "undefined" && response.data.refresh_token) {
        localStorage.setItem("refresh_token", response.data.refresh_token);
      }
      return { success: true, data: response.data };
    }

    // 登录失败
    return { success: false, message: response.error || "登录失败" };
  }

  // 验证 2FA 码完成登录
  async verify2FA(code: string, tempToken: string, rememberDevice: boolean = false): Promise<LoginResponse> {
    const response = await api.post<LoginResponse>("/auth/verify-2fa", {
      code,
      temp_token: tempToken,
      remember_device: rememberDevice,
    });

    if (response.data?.access_token) {
      api.setToken(response.data.access_token);
      if (typeof window !== "undefined") {
        if (response.data.refresh_token) {
          localStorage.setItem("refresh_token", response.data.refresh_token);
        }
        // 保存设备信任令牌
        if (response.data.device_token) {
          localStorage.setItem(DEVICE_TOKEN_KEY, response.data.device_token);
        }
      }
      return { success: true, data: response.data };
    }

    return { success: false, message: response.error || "验证失败" };
  }

  // 登出
  async logout(): Promise<void> {
    try {
      await api.post("/auth/logout");
    } catch {
      // 忽略登出错误
    } finally {
      api.setToken(null);
    }
  }

  // 获取当前用户信息
  async getCurrentUser(): Promise<UserInfo | null> {
    try {
      const response = await api.get<UserInfoResponse>("/users/current");
      return response.data || null;
    } catch {
      return null;
    }
  }

  // 检查登录状态
  async checkAuth(): Promise<boolean> {
    if (!api.isAuthenticated()) {
      return false;
    }

    const user = await this.getCurrentUser();
    return user !== null;
  }

  // 修改密码
  async changePassword(
    data: ChangePasswordRequest,
  ): Promise<{ success: boolean; message: string }> {
    return api.post("/users/password", data);
  }

  // 刷新 Token
  async refreshToken(): Promise<boolean> {
    try {
      const refreshToken = localStorage.getItem("refresh_token");
      if (!refreshToken) return false;
      const response = await api.post<LoginResponse>("/auth/refresh", {
        refresh_token: refreshToken,
      });
      if (response.success && response.data?.access_token) {
        api.setToken(response.data.access_token);
        if (response.data.refresh_token) {
          localStorage.setItem("refresh_token", response.data.refresh_token);
        }
        return true;
      }
      return false;
    } catch {
      return false;
    }
  }

  // ----- 两步验证管理 -----

  // 获取 2FA 状态
  async get2FAStatus(): Promise<boolean> {
    try {
      const resp = await api.get<{ data: { enabled: boolean } }>("/users/2fa/status");
      return resp.data?.enabled ?? false;
    } catch {
      return false;
    }
  }

  // 开始设置 2FA（返回密钥和二维码 URL）
  async setup2FA(): Promise<{
    success: boolean;
    secret?: string;
    qr_code_url?: string;
    backup_codes?: string[];
    message?: string;
  }> {
    try {
      const resp = await api.post<{
        data: { secret: string; qr_code_url: string; backup_codes: string[] };
      }>("/users/2fa/setup");
      return { success: true, ...resp.data };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "设置失败";
      return { success: false, message: msg };
    }
  }

  // 验证码确认启用 2FA
  async enable2FA(code: string): Promise<{ success: boolean; message?: string }> {
    try {
      await api.post("/users/2fa/enable", { code });
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "启用失败";
      return { success: false, message: msg };
    }
  }

  // 关闭 2FA
  async disable2FA(): Promise<{ success: boolean; message?: string }> {
    try {
      await api.delete("/users/2fa");
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "关闭失败";
      return { success: false, message: msg };
    }
  }
}

export const authService = new AuthService();
