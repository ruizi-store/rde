// 用户 + 实例隔离的 localStorage 工具
// 格式: rde:{instanceId}:{baseKey}:{userId}

import { getAuthItem } from "./auth-storage";
import { getInstanceId } from "./instance";

const TOKEN_KEY = "auth_token";

/**
 * 从 JWT token 提取当前用户 ID
 */
export function getCurrentUserId(): string | null {
  if (typeof window === "undefined") return null;

  const token = getAuthItem(TOKEN_KEY);
  if (!token) return null;

  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const payload = JSON.parse(atob(base64));
    return payload.sub || payload.user_id || null;
  } catch {
    return null;
  }
}

/**
 * 获取实例+用户专属的 localStorage key
 */
export function userStorageKey(baseKey: string): string {
  const instanceId = getInstanceId();
  const userId = getCurrentUserId();
  let key = baseKey;
  if (instanceId) {
    key = `rde:${instanceId}:${key}`;
  }
  if (userId) {
    key = `${key}:${userId}`;
  }
  return key;
}

/**
 * 从 localStorage 读取用户专属数据（带旧 key 迁移）
 */
export function loadUserData<T>(baseKey: string, fallback?: T): T | null {
  if (typeof localStorage === "undefined") return fallback ?? null;

  const key = userStorageKey(baseKey);
  const candidates = [key];
  // 迁移：仅用户后缀 / 全局 key
  const userId = getCurrentUserId();
  if (userId) {
    candidates.push(`${baseKey}:${userId}`);
  }
  candidates.push(baseKey);

  try {
    for (const candidate of candidates) {
      const data = localStorage.getItem(candidate);
      if (data) {
        if (candidate !== key) {
          localStorage.setItem(key, data);
        }
        return JSON.parse(data) as T;
      }
    }
  } catch (e) {
    console.error(`Failed to load user data for key "${baseKey}":`, e);
  }

  return fallback ?? null;
}

/**
 * 保存数据到用户专属的 localStorage key
 */
export function saveUserData(baseKey: string, data: unknown): void {
  if (typeof localStorage === "undefined") return;

  const key = userStorageKey(baseKey);
  try {
    localStorage.setItem(key, JSON.stringify(data));
  } catch (e) {
    console.error(`Failed to save user data for key "${baseKey}":`, e);
  }
}

/**
 * 读取用户专属的简单字符串值（非 JSON）
 */
export function loadUserString(baseKey: string): string | null {
  if (typeof localStorage === "undefined") return null;

  const key = userStorageKey(baseKey);
  let value = localStorage.getItem(key);

  if (value === null && key !== baseKey) {
    value = localStorage.getItem(baseKey);
    if (value !== null) {
      localStorage.setItem(key, value);
    }
  }

  return value;
}

/**
 * 保存用户专属的简单字符串值
 */
export function saveUserString(baseKey: string, value: string): void {
  if (typeof localStorage === "undefined") return;

  const key = userStorageKey(baseKey);
  localStorage.setItem(key, value);
}
