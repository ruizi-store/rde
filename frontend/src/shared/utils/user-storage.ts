// 用户隔离的 localStorage 工具
// 通过 JWT token 中的 user_id 为每个用户生成独立的存储 key
// 确保多用户环境下个性化设置（壁纸、主题、图标布局等）互不影响

const TOKEN_KEY = "auth_token";

/**
 * 从 localStorage 中的 JWT token 提取当前用户 ID
 * 纯客户端解码，不发网络请求
 */
export function getCurrentUserId(): string | null {
  if (typeof window === "undefined") return null;

  const token = localStorage.getItem(TOKEN_KEY);
  if (!token) return null;

  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const payload = JSON.parse(atob(base64));
    // JWT 的 sub 字段存储 user_id
    return payload.sub || payload.user_id || null;
  } catch {
    return null;
  }
}

/**
 * 获取用户专属的 localStorage key
 * 格式: baseKey:userId
 * 若未登录则返回原始 key
 */
export function userStorageKey(baseKey: string): string {
  const userId = getCurrentUserId();
  return userId ? `${baseKey}:${userId}` : baseKey;
}

/**
 * 从 localStorage 读取用户专属数据
 * 带自动迁移：若用户专属 key 无数据，自动从旧的全局 key 迁移
 */
export function loadUserData<T>(baseKey: string, fallback?: T): T | null {
  if (typeof localStorage === "undefined") return fallback ?? null;

  const key = userStorageKey(baseKey);

  try {
    const data = localStorage.getItem(key);
    if (data) {
      return JSON.parse(data) as T;
    }

    // 用户专属 key 无数据 → 尝试从全局 key 迁移
    if (key !== baseKey) {
      const legacyData = localStorage.getItem(baseKey);
      if (legacyData) {
        // 复制到用户专属 key（不删除旧数据，其他用户可能需要迁移）
        localStorage.setItem(key, legacyData);
        return JSON.parse(legacyData) as T;
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
 * 带自动迁移
 */
export function loadUserString(baseKey: string): string | null {
  if (typeof localStorage === "undefined") return null;

  const key = userStorageKey(baseKey);
  let value = localStorage.getItem(key);

  // 迁移旧数据
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

/**
 * 为当前用户重新加载所有个性化 store
 * 在登录后或页面加载时调用，确保各 store 使用正确的用户专属数据
 */
export function reloadAllStoresForUser(): void {
  // 动态导入避免循环依赖
  import("$shared/stores/theme.svelte").then(({ theme }) => theme.reloadForUser());
  import("$shared/stores/settings.svelte").then(({ settings }) => settings.reloadForUser());
  import("$desktop/stores/desktop.svelte").then(({ desktop }) => desktop.reloadForUser());
  import("$desktop/stores/apps.svelte").then(({ apps }) => apps.reloadForUser());
}
