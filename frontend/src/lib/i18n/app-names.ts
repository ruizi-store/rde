// 应用名称本地化辅助函数
import { get } from "svelte/store";
import { _, locale } from "svelte-i18n";

// 应用 ID 到翻译 key 的映射
const APP_NAME_KEYS: Record<string, string> = {
  "file": "apps.names.file",
  "settings": "apps.names.settings",
  "terminal": "apps.names.terminal",
  "download": "apps.names.download",
  "sync": "apps.names.sync",
  "photos": "apps.names.photos",
  "music": "apps.names.music",
  "video": "apps.names.video",
  "backup": "apps.names.backup",
  "samba": "apps.names.samba",
  "retrogame": "apps.names.retrogame",
  "docker": "apps.names.docker",
  "notification": "apps.names.notification",
};

/**
 * 获取应用的本地化名称
 * @param appId 应用 ID
 * @param fallback 兜底名称（通常是 app.name）
 * @returns 本地化后的应用名称
 */
export function getAppName(appId: string, fallback: string): string {
  const key = APP_NAME_KEYS[appId];
  if (!key) return fallback;
  
  const $_ = get(_);
  const translated = $_(key);
  
  // 如果翻译失败（返回 key 本身），使用 fallback
  if (translated === key || !translated) {
    return fallback;
  }
  
  return translated;
}

/**
 * 获取应用名称的翻译 Key
 * 用于在 Svelte 模板中直接使用 $t()
 */
export function getAppNameKey(appId: string): string | null {
  return APP_NAME_KEYS[appId] || null;
}
