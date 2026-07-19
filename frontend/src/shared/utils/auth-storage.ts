/**
 * 认证相关 localStorage，按实例 ID 命名空间，避免同 IP 换机串会话。
 */
import { getInstanceId } from "./instance";

const AUTH_KEYS = ["auth_token", "refresh_token", "rde_device_token"] as const;
export type AuthStorageKey = (typeof AUTH_KEYS)[number];

export function authStorageKey(key: string): string {
  const id = getInstanceId();
  return id ? `rde:${id}:${key}` : key;
}

export function getAuthItem(key: string): string | null {
  if (typeof localStorage === "undefined") return null;
  const ns = authStorageKey(key);
  const v = localStorage.getItem(ns);
  if (v !== null) return v;
  // 兼容旧扁平 key（对账前/迁移前）
  return localStorage.getItem(key);
}

export function setAuthItem(key: string, value: string): void {
  if (typeof localStorage === "undefined") return;
  localStorage.setItem(authStorageKey(key), value);
  try {
    localStorage.removeItem(key);
  } catch {
    /* ignore */
  }
}

export function removeAuthItem(key: string): void {
  if (typeof localStorage === "undefined") return;
  localStorage.removeItem(authStorageKey(key));
  localStorage.removeItem(key);
}

export function clearAuthItems(): void {
  for (const k of AUTH_KEYS) {
    removeAuthItem(k);
  }
}

/** 将对账前扁平 auth key 迁到当前实例命名空间 */
export function migrateLegacyAuthKeys(): void {
  const id = getInstanceId();
  if (!id || typeof localStorage === "undefined") return;
  for (const k of AUTH_KEYS) {
    const bare = localStorage.getItem(k);
    const ns = `rde:${id}:${k}`;
    if (bare && !localStorage.getItem(ns)) {
      localStorage.setItem(ns, bare);
      localStorage.removeItem(k);
    }
  }
}
