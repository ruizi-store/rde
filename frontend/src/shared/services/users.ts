// 用户管理服务

import { api } from "./api";

export interface User {
  id: string;
  username: string;
  email?: string;
  nickname?: string;
  avatar?: string;
  role: string;
  status: string;
  last_login?: string;
  created_at: string;
  is_online?: boolean;
}

export interface CreateUserRequest {
  username: string;
  password: string;
  nickname?: string;
  email?: string;
}

export interface UpdateUserRequest {
  nickname?: string;
  email?: string;
  avatar?: string;
  settings?: string;
}

export interface ChangePasswordRequest {
  old_password: string;
  new_password: string;
}

class UsersService {
  async list(): Promise<User[]> {
    const resp = await api.get<{ data: User[]; total: number }>("/users");
    return resp.data || [];
  }

  async get(id: string): Promise<User | null> {
    const resp = await api.get<{ data: User }>(`/users/${id}`);
    return resp.data || null;
  }

  async create(data: CreateUserRequest): Promise<{ success: boolean; message?: string }> {
    try {
      await api.post("/users", data);
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "创建失败";
      return { success: false, message: msg };
    }
  }

  async update(id: string, data: UpdateUserRequest): Promise<{ success: boolean; message?: string }> {
    try {
      await api.put(`/users/${id}`, data);
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "更新失败";
      return { success: false, message: msg };
    }
  }

  async delete(id: string): Promise<{ success: boolean; message?: string }> {
    try {
      await api.delete(`/users/${id}`);
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "删除失败";
      return { success: false, message: msg };
    }
  }

  async changePassword(
    id: string,
    data: ChangePasswordRequest,
  ): Promise<{ success: boolean; message?: string }> {
    try {
      await api.put(`/users/${id}/password`, data);
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "修改失败";
      return { success: false, message: msg };
    }
  }

  async resetPassword(
    id: string,
    newPassword: string,
  ): Promise<{ success: boolean; message?: string }> {
    try {
      await api.put(`/users/${id}/reset-password`, { new_password: newPassword });
      return { success: true };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "重置失败";
      return { success: false, message: msg };
    }
  }

  async uploadAvatar(
    id: string,
    file: File,
  ): Promise<{ success: boolean; avatar?: string; message?: string }> {
    try {
      const formData = new FormData();
      formData.append("file", file);
      const resp = await api.upload<{ data: { avatar: string } }>(`/users/${id}/avatar`, formData);
      return { success: true, avatar: resp.data?.avatar };
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : "上传失败";
      return { success: false, message: msg };
    }
  }
}

export const usersService = new UsersService();
