// JWT token 工具函数
// 纯客户端解码，不发网络请求

const TOKEN_KEY = "auth_token";

interface JwtPayload {
  exp?: number;
  iat?: number;
  sub?: string;
  [key: string]: unknown;
}

/**
 * 解码 JWT payload（不验证签名，仅解析）
 */
function decodeJwtPayload(token: string): JwtPayload | null {
  try {
    const parts = token.split(".");
    if (parts.length !== 3) return null;

    // base64url → base64 → 解码
    const base64 = parts[1].replace(/-/g, "+").replace(/_/g, "/");
    const json = atob(base64);
    return JSON.parse(json);
  } catch {
    return null;
  }
}

/**
 * 检查 token 是否已过期
 * 提前 30 秒视为过期，避免边界情况
 */
export function isTokenExpired(token: string, bufferSeconds = 30): boolean {
  const payload = decodeJwtPayload(token);
  if (!payload?.exp) return true; // 无 exp 视为过期

  const now = Math.floor(Date.now() / 1000);
  return payload.exp - bufferSeconds <= now;
}

/**
 * 从 localStorage 获取 token 并检查有效性
 * 返回有效 token 或 null（过期/不存在时自动清除）
 */
export function getValidToken(): string | null {
  if (typeof window === "undefined") return null;

  const token = localStorage.getItem(TOKEN_KEY);
  if (!token) return null;

  if (isTokenExpired(token)) {
    localStorage.removeItem(TOKEN_KEY);
    return null;
  }

  return token;
}

/**
 * 快速检查当前是否有有效的 token（不发请求）
 */
export function hasValidToken(): boolean {
  return getValidToken() !== null;
}
