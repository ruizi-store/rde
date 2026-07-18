/**
 * Iconify 离线图标配置
 * 构建期打包全量 mdi + simple-icons，运行时从同源 chunk 加载，不依赖外网 CDN
 */

import { addCollection } from "@iconify/svelte";

// 标记是否已初始化
let initialized = false;
let initPromise: Promise<void> | null = null;

/**
 * 初始化离线图标（全量 Iconify 集合）
 * 在应用启动时调用此函数
 */
export async function initOfflineIcons(): Promise<void> {
  if (initialized) return;
  if (initPromise) return initPromise;

  initPromise = (async () => {
    const [mdiIcons, simpleIcons] = await Promise.all([
      import("@iconify-json/mdi/icons.json"),
      import("@iconify-json/simple-icons/icons.json"),
    ]);

    addCollection(mdiIcons.default as any);
    addCollection(simpleIcons.default as any);
    initialized = true;
    console.log("[Icons] Full offline icon collections loaded");
  })();

  try {
    await initPromise;
  } catch (err) {
    initPromise = null;
    console.error("[Icons] Failed to load offline icon collections:", err);
    throw err;
  }
}

// 导出常用图标名称常量，便于代码补全和类型检查
export const Icons = {
  // 通用操作
  close: "mdi:close",
  check: "mdi:check",
  plus: "mdi:plus",
  minus: "mdi:minus",
  delete: "mdi:delete",
  edit: "mdi:pencil",
  save: "mdi:content-save",
  copy: "mdi:content-copy",
  cut: "mdi:content-cut",
  paste: "mdi:content-paste",
  refresh: "mdi:refresh",
  search: "mdi:magnify",
  loading: "mdi:loading",

  // 导航
  arrowLeft: "mdi:arrow-left",
  arrowRight: "mdi:arrow-right",
  arrowUp: "mdi:arrow-up",
  chevronDown: "mdi:chevron-down",
  chevronRight: "mdi:chevron-right",

  // 文件管理
  file: "mdi:file",
  folder: "mdi:folder-open",
  folderPlus: "mdi:folder-plus",
  upload: "mdi:upload",
  download: "mdi:download",

  // 系统
  settings: "mdi:cog",
  account: "mdi:account-circle",
  lock: "mdi:lock",
  power: "mdi:power",
  wifi: "mdi:wifi",

  // 通知
  bell: "mdi:bell",
  bellOff: "mdi:bell-off",

  // 状态
  success: "mdi:check-circle",
  warning: "mdi:alert-circle",
  error: "mdi:close-circle",
  info: "mdi:information-outline",

  // Docker
  docker: "mdi:docker",
  dockerLogo: "simple-icons:docker",
} as const;
