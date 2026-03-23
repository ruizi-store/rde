// Apps Store - Svelte 5 Runes
// 应用注册和管理
// 参考文档: docs/frontend/APP_ICON_MANAGEMENT.md

import { windowManager, type AppDefinition } from "./windows.svelte";
import {
  appsService,
  preferencesService,
  startMenuService,
  taskbarService,
} from "$shared/services/preferences";
import { modulesApi } from "$shared/services/setup";
import { remoteAccessStore } from "./remote-access.svelte";
import type {
  InstalledApp,
  AppSource,
  AppState,
  StartMenuCategory,
  AppPlacements,
  AppResponse,
  UserPreferencesResponse,
  TaskbarAppItem,
  DesktopIconItem,
  StartMenuPosition,
} from "$shared/types/apps";
import { CATEGORY_INFO, getDefaultCategory } from "$shared/types/apps";
import { loadUserData, saveUserData } from "$shared/utils/user-storage";

// 防抖函数
function debounce<T extends (...args: unknown[]) => unknown>(fn: T, delay: number): T {
  let timeoutId: ReturnType<typeof setTimeout>;
  return ((...args: unknown[]) => {
    clearTimeout(timeoutId);
    timeoutId = setTimeout(() => fn(...args), delay);
  }) as T;
}

const STORAGE_KEY = "rde_apps_config";
const CONFIG_VERSION = 4; // 增加版本号以清除旧配置（移除套件系统）

// 外部应用启动回调类型
export type ExternalAppLauncher = (appId: string, appName: string) => Promise<void>;

// 扩展 AppDefinition 以支持更多应用类型
export interface ExtendedAppDefinition extends AppDefinition {
  type?: "builtin" | "docker" | "native"; // docker = 外部应用
  externalAppId?: string; // 外部应用 ID（docker:xxx）
  nativeAppId?: string; // 后端原生应用 ID（兼容旧代码）
  source?: AppSource; // 应用来源
  state?: AppState; // 应用状态
  category?: StartMenuCategory; // 开始菜单分类
  keywords?: string; // 搜索关键词
  launchCallback?: ExternalAppLauncher; // 外部应用自定义启动回调
  webUI?: string; // Docker 应用的 Web UI 地址
}

class AppsStore {
  // 已注册的应用
  private _apps = $state<Map<string, ExtendedAppDefinition>>(new Map());

  // 固定在开始菜单的应用 ID
  pinnedStartMenuAppIds = $state<string[]>([
    "file",
    "settings",
    "terminal",
    "notification",
    "music",
    "translate",
  ]);

  // 固定在任务栏的应用 ID (已排序) - 保持简洁，只留文件管理
  pinnedAppIds = $state<string[]>(["file"]);

  // 桌面上的应用 ID（通过 desktop store 管理）
  // 新用户默认显示翻译应用
  desktopAppIds = $state<string[]>(["translate"]);

  // 最近使用的应用
  recentApps = $state<string[]>([]);

  // 开始菜单位置
  startMenuPosition = $state<StartMenuPosition>("left");

  // 是否已从后端加载
  private _loaded = $state(false);

  // 是否正在同步
  private _syncing = $state(false);

  constructor() {
    // 连接窗口管理器的应用注册表
    windowManager.setAppRegistry(this._apps as Map<string, AppDefinition>);
    // 从本地缓存加载配置（快速显示）
    this.loadLocalConfig();
  }

  // 是否已加载完成
  get loaded(): boolean {
    return this._loaded;
  }

  // 从后端初始化（应用启动时调用）
  async init(): Promise<void> {
    if (this._loaded) return;

    try {
      // 并行加载应用列表和用户偏好
      const [appsList, preferences] = await Promise.all([
        appsService.getApps().catch(() => [] as AppResponse[]),
        preferencesService.getPreferences().catch(() => null),
      ]);

      // 注册从后端获取的应用
      if (appsList.length > 0) {
        for (const app of appsList) {
          this.registerFromBackend(app);
        }
      }

      // 应用用户偏好
      if (preferences) {
        this.applyPreferences(preferences);
      }

      this._loaded = true;

      // 保存到本地缓存
      this.saveLocalConfig();

    } catch (e) {
      console.error("初始化应用 Store 失败:", e);
      // 使用本地缓存的数据
      this._loaded = true;
    }
  }

  // 从后端应用响应注册
  private registerFromBackend(app: AppResponse): void {
    const extApp: ExtendedAppDefinition = {
      id: app.id,
      name: app.name,
      icon: app.icon,
      source: app.source,
      state: app.state || "active",
      category:
        (app.category as StartMenuCategory) ||
        this.inferCategory({ id: app.id, source: app.source } as ExtendedAppDefinition),
      keywords: app.keywords,
      component: () => import(`$lib/components/apps/${this.getComponentName(app.id)}.svelte`),
    };

    this._apps.set(app.id, extApp);
  }

  // 获取组件名称
  private getComponentName(appId: string): string {
    // 将 kebab-case 转换为 PascalCase
    return appId
      .split("-")
      .map((s) => s.charAt(0).toUpperCase() + s.slice(1))
      .join("");
  }

  // 应用用户偏好
  private applyPreferences(prefs: UserPreferencesResponse): void {
    this.startMenuPosition = prefs.startMenuPosition || "left";
    this.pinnedStartMenuAppIds = prefs.pinnedApps || this.pinnedStartMenuAppIds;
    this.pinnedAppIds = prefs.taskbarApps?.map((a) => a.id) || this.pinnedAppIds;
    this.recentApps = prefs.recentApps || [];
  }

  // 获取所有应用
  get all(): ExtendedAppDefinition[] {
    return [...this._apps.values()];
  }

  // 别名 - 兼容旧代码
  get list(): ExtendedAppDefinition[] {
    return this.all;
  }

  // 获取活跃状态的应用
  get activeApps(): ExtendedAppDefinition[] {
    return this.all;
  }

  // 获取固定在任务栏的应用
  get pinned(): ExtendedAppDefinition[] {
    return this.pinnedAppIds
      .map((id) => this._apps.get(id))
      .filter((app): app is ExtendedAppDefinition => app !== undefined);
  }

  // 按分类获取开始菜单应用
  get startMenuByCategory(): Map<StartMenuCategory, ExtendedAppDefinition[]> {
    const result = new Map<StartMenuCategory, ExtendedAppDefinition[]>();

    // 初始化分类
    for (const cat of Object.keys(CATEGORY_INFO) as StartMenuCategory[]) {
      result.set(cat, []);
    }

    // 分配应用到分类（只包含已启用模块的应用）
    for (const app of this.activeApps) {
      const category = app.category || this.inferCategory(app);
      const apps = result.get(category) || [];
      apps.push(app);
      result.set(category, apps);
    }

    return result;
  }

  // 获取排序后的分类列表（用于开始菜单显示）
  get sortedCategories(): {
    category: StartMenuCategory;
    info: (typeof CATEGORY_INFO)[StartMenuCategory];
    apps: ExtendedAppDefinition[];
  }[] {
    const byCategory = this.startMenuByCategory;
    const result: {
      category: StartMenuCategory;
      info: (typeof CATEGORY_INFO)[StartMenuCategory];
      apps: ExtendedAppDefinition[];
    }[] = [];

    // 按 order 排序
    const sortedCategories = (Object.keys(CATEGORY_INFO) as StartMenuCategory[]).sort(
      (a, b) => CATEGORY_INFO[a].order - CATEGORY_INFO[b].order,
    );

    for (const cat of sortedCategories) {
      const apps = byCategory.get(cat) || [];
      if (apps.length > 0) {
        result.push({
          category: cat,
          info: CATEGORY_INFO[cat],
          apps,
        });
      }
    }

    return result;
  }

  // 推断应用分类
  private inferCategory(app: ExtendedAppDefinition): StartMenuCategory {
    // 系统核心应用
    if (["file", "terminal", "settings"].includes(app.id)) {
      return "system";
    }
    // 效率工具
    if (["docker", "backup", "notification"].includes(app.id)) {
      return "productivity";
    }
    // 根据来源推断
    if (app.source) {
      return getDefaultCategory(app.source);
    }
    return "other";
  }

  // 注册应用
  register(app: ExtendedAppDefinition): void {
    // 设置默认值
    if (!app.source) {
      app.source = "module";
    }
    if (!app.state) {
      app.state = "active";
    }
    if (!app.category) {
      app.category = this.inferCategory(app);
    }

    this._apps.set(app.id, app);
  }

  // 批量注册
  registerAll(apps: ExtendedAppDefinition[]): void {
    apps.forEach((app) => this.register(app));
  }

  // 注册外部应用（Docker）
  registerExternalApp(options: {
    id: string;
    name: string;
    icon: string;
    type: "docker";
    externalAppId: string;
    launchCallback?: ExternalAppLauncher;
    webUI?: string;
    keywords?: string;
  }): void {
    const appId = `${options.type}:${options.externalAppId}`;
    const category: StartMenuCategory = "docker_apps";

    const app: ExtendedAppDefinition = {
      id: appId,
      name: options.name,
      icon: options.icon || "mdi:docker",
      type: options.type,
      externalAppId: options.externalAppId,
      source: "docker_store",
      state: "active",
      category,
      keywords: options.keywords,
      launchCallback: options.launchCallback,
      webUI: options.webUI,
      // 外部应用没有组件，使用空的懒加载
      component: () => Promise.resolve({ default: null }),
    };

    this._apps.set(appId, app);
  }

  // 批量注册外部应用
  registerExternalApps(apps: Array<Parameters<typeof this.registerExternalApp>[0]>): void {
    apps.forEach((app) => this.registerExternalApp(app));
  }

  // 清除指定类型的外部应用（用于刷新）
  clearExternalApps(type: "docker"): void {
    const prefix = `${type}:`;
    for (const id of this._apps.keys()) {
      if (id.startsWith(prefix)) {
        this._apps.delete(id);
      }
    }
  }

  // 获取应用
  get(id: string): ExtendedAppDefinition | undefined {
    return this._apps.get(id);
  }

  // 检查应用是否可用
  isAvailable(appId: string): boolean {
    const app = this._apps.get(appId);
    return !!app;
  }

  // 启动应用
  async launch(appId: string, props?: Record<string, unknown>): Promise<string | null> {
    const app = this._apps.get(appId);

    // 记录应用启动
    this.recordAppLaunch(appId);

    // 如果是原生应用（以 native- 开头），在新浏览器窗口中打开
    if (appId.startsWith("native-")) {
      const nativeAppId = appId.replace("native-", "");
      return this.launchNativeAppInNewWindow(nativeAppId, app?.name);
    }

    if (!app) {
      console.warn(`App not found: ${appId}`);
      return null;
    }

    // 外部应用（Docker）使用自定义启动回调
    if (app.type === "docker") {
      if (app.launchCallback) {
        await app.launchCallback(app.externalAppId || appId, app.name);
        return null; // 外部应用不返回窗口 ID
      }
      // 如果有 webUI，在新标签页打开
      if (app.webUI) {
        window.open(app.webUI, "_blank");
        return null;
      }
      console.warn(`External app ${appId} has no launch callback or webUI`);
      return null;
    }

    return await windowManager.open(app, props);
  }

  // 在新窗口中启动原生应用（通过 Xpra）
  private launchNativeAppInNewWindow(nativeAppId: string, appName?: string): string | null {
    // 原生应用通过 Xpra HTML 客户端在新窗口中打开
    console.log(`[Native] Launching ${appName || nativeAppId} via Xpra`);
    // TODO: 实现 Xpra 启动逻辑
    return null;
  }

  // === 任务栏固定管理 ===

  // 固定应用到任务栏
  pin(appId: string): void {
    if (!this.pinnedAppIds.includes(appId)) {
      this.pinnedAppIds = [...this.pinnedAppIds, appId];
      this.saveLocalConfig();
      this.debouncedSyncTaskbar();
    }
  }

  // 从任务栏取消固定
  unpin(appId: string): void {
    this.pinnedAppIds = this.pinnedAppIds.filter((id) => id !== appId);
    this.saveLocalConfig();
    this.debouncedSyncTaskbar();
  }

  // 检查应用是否已固定到任务栏
  isPinned(appId: string): boolean {
    return this.pinnedAppIds.includes(appId);
  }

  // 设置任务栏顺序
  setTaskbarOrder(appIds: string[]): void {
    this.pinnedAppIds = appIds;
    this.saveLocalConfig();
    this.debouncedSyncTaskbar();
  }

  // === 第三方应用管理 ===

  // 安装第三方应用
  installApp(app: ExtendedAppDefinition, addToDesktop: boolean = false): void {
    app.state = "active";
    this._apps.set(app.id, app);

    if (addToDesktop) {
      this.desktopAppIds.push(app.id);
    }

    this.saveConfig();
  }

  // 卸载第三方应用
  uninstallApp(appId: string): boolean {
    const app = this._apps.get(appId);
    if (!app) return false;

    // 只能卸载第三方应用
    if (
      app.source !== "docker_store"
    ) {
      return false;
    }

    // 从所有位置移除
    this._apps.delete(appId);
    this.pinnedAppIds = this.pinnedAppIds.filter((id) => id !== appId);
    this.desktopAppIds = this.desktopAppIds.filter((id) => id !== appId);

    this.saveConfig();
    return true;
  }

  // === 持久化 ===

  // 防抖保存到后端
  private debouncedSyncPinnedApps = debounce(async () => {
    try {
      await preferencesService.setPinnedApps(this.pinnedStartMenuAppIds);
    } catch (e) {
      console.error("同步固定应用失败:", e);
    }
  }, 500);

  private debouncedSyncTaskbar = debounce(async () => {
    try {
      const apps: TaskbarAppItem[] = this.pinnedAppIds.map((id, order) => ({ id, order }));
      await taskbarService.updateApps(apps);
    } catch (e) {
      console.error("同步任务栏失败:", e);
    }
  }, 500);

  // 保存配置到 localStorage（本地缓存，用户专属 key）
  private saveLocalConfig(): void {
    if (typeof localStorage === "undefined") return;

    try {
      const config = {
        version: CONFIG_VERSION,
        pinnedAppIds: this.pinnedAppIds,
        pinnedStartMenuAppIds: this.pinnedStartMenuAppIds,
        desktopAppIds: this.desktopAppIds,
        startMenuPosition: this.startMenuPosition,
        recentApps: this.recentApps,
      };
      saveUserData(STORAGE_KEY, config);
    } catch (e) {
      console.error("保存应用配置失败:", e);
    }
  }

  // 从 localStorage 加载配置（本地缓存，用户专属 key）
  private loadLocalConfig(): void {
    if (typeof localStorage === "undefined") return;

    try {
      const config = loadUserData<{
        version: number;
        pinnedAppIds?: string[];
        pinnedStartMenuAppIds?: string[];
        desktopAppIds?: string[];
        startMenuPosition?: StartMenuPosition;
        recentApps?: string[];
      }>(STORAGE_KEY);

      if (config) {
        // 版本检查：如果版本不匹配，清除旧配置
        if (config.version !== CONFIG_VERSION) {
          console.log("[Apps] Config version mismatch, using defaults");
          return;
        }

        if (config.pinnedAppIds) {
          this.pinnedAppIds = config.pinnedAppIds;
        }
        if (config.pinnedStartMenuAppIds) {
          this.pinnedStartMenuAppIds = config.pinnedStartMenuAppIds;
        }
        if (config.desktopAppIds) {
          this.desktopAppIds = config.desktopAppIds;
        }
        if (config.startMenuPosition) {
          this.startMenuPosition = config.startMenuPosition;
        }
        if (config.recentApps) {
          this.recentApps = config.recentApps;
        }
      }
    } catch (e) {
      console.error("加载应用配置失败:", e);
    }
  }

  // 为当前用户重新加载配置（用户切换时调用）
  reloadForUser(): void {
    // 重置为默认值
    this.pinnedStartMenuAppIds = ["file", "settings", "terminal", "notification", "music", "translate"];
    this.pinnedAppIds = ["file"];
    this.desktopAppIds = ["translate"];
    this.recentApps = [];
    this.startMenuPosition = "left";
    this._loaded = false;
    // 从用户专属 key 重新加载
    this.loadLocalConfig();
  }

  // 同步所有配置到后端
  async syncToBackend(): Promise<void> {
    if (this._syncing) return;
    this._syncing = true;

    try {
      await preferencesService.updatePreferences({
        startMenuPosition: this.startMenuPosition,
        pinnedApps: this.pinnedStartMenuAppIds,
        taskbarApps: this.pinnedAppIds.map((id, order) => ({ id, order })),
      });
    } catch (e) {
      console.error("同步到后端失败:", e);
    } finally {
      this._syncing = false;
    }
  }

  // === 开始菜单固定管理 ===

  // 固定到开始菜单
  async pinToStartMenu(appId: string): Promise<void> {
    if (!this.pinnedStartMenuAppIds.includes(appId)) {
      this.pinnedStartMenuAppIds = [...this.pinnedStartMenuAppIds, appId];
      this.saveLocalConfig();
      this.debouncedSyncPinnedApps();
    }
  }

  // 从开始菜单取消固定
  async unpinFromStartMenu(appId: string): Promise<void> {
    this.pinnedStartMenuAppIds = this.pinnedStartMenuAppIds.filter((id) => id !== appId);
    this.saveLocalConfig();
    this.debouncedSyncPinnedApps();
  }

  // 检查应用是否固定在开始菜单
  isPinnedInStartMenu(appId: string): boolean {
    return this.pinnedStartMenuAppIds.includes(appId);
  }

  // 获取固定在开始菜单的应用
  get pinnedStartMenuApps(): ExtendedAppDefinition[] {
    const result: ExtendedAppDefinition[] = [];

    for (const id of this.pinnedStartMenuAppIds) {
      // 如果终端被禁用，跳过终端应用
      if (id === "terminal" && !remoteAccessStore.terminalEnabled) {
        continue;
      }

      const app = this._apps.get(id);
      if (!app) {
        // 应用不存在，跳过（可能是旧配置中的无效 ID）
        continue;
      }
      result.push(app);
    }

    return result;
  }

  // === 开始菜单位置 ===

  // 设置开始菜单位置
  async setStartMenuPosition(position: StartMenuPosition): Promise<void> {
    this.startMenuPosition = position;
    this.saveLocalConfig();

    try {
      await preferencesService.setStartMenuPosition(position);
    } catch (e) {
      console.error("保存开始菜单位置失败:", e);
    }
  }

  // === 最近使用 ===

  // 记录应用启动
  async recordAppLaunch(appId: string): Promise<void> {
    // 更新本地
    this.recentApps = [appId, ...this.recentApps.filter((id) => id !== appId)].slice(0, 10);
    this.saveLocalConfig();

    // 同步到后端
    try {
      await appsService.recordLaunch(appId);
    } catch (e) {
      // 忽略错误
    }
  }

  // 获取最近使用的应用
  get recentAppsList(): ExtendedAppDefinition[] {
    return this.recentApps
      .filter((id) => {
        // 如果终端被禁用，过滤掉终端
        if (id === "terminal" && !remoteAccessStore.terminalEnabled) {
          return false;
        }
        return true;
      })
      .map((id) => this._apps.get(id))
      .filter((app): app is ExtendedAppDefinition => app !== undefined)
      .slice(0, 12); // 显示最近 12 个（2 行）
  }

  // 保存配置到 localStorage（旧方法，保持兼容）
  private saveConfig(): void {
    this.saveLocalConfig();
  }

  // 从 localStorage 加载配置（旧方法，保持兼容）
  private loadConfig(): void {
    this.loadLocalConfig();
  }
}

export const apps = new AppsStore();

// 导出类型
export type { AppDefinition };
export type { ExternalAppLauncher };
export type {
  InstalledApp,
  AppSource,
  AppState,
  StartMenuCategory,
  StartMenuPosition,
  TaskbarAppItem,
  DesktopIconItem,
} from "$shared/types/apps";
