/**
 * Iconify 离线图标配置
 * 预加载项目中使用的所有图标，支持离线环境
 */

import { addCollection } from "@iconify/svelte";

// 标记是否已初始化
let initialized = false;

/**
 * 初始化离线图标（异步按需加载）
 * 在应用启动时调用此函数
 */
export async function initOfflineIcons(): Promise<void> {
  // 避免重复初始化
  if (initialized) return;
  initialized = true;

  // 动态导入图标集
  const [mdiIcons, simpleIcons] = await Promise.all([
    import("@iconify-json/mdi/icons.json"),
    import("@iconify-json/simple-icons/icons.json"),
  ]);

  // 注册 MDI 图标集
  addCollection(mdiIcons.default as any);

  // 注册 Simple Icons 图标集
  addCollection(simpleIcons.default as any);

  console.log("[Icons] Offline icon collections loaded");
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
