// 应用注册表
// 在这里注册所有可用的桌面应用

import { apps, type ExtendedAppDefinition } from "$desktop/stores/apps.svelte";
import type { StartMenuCategory } from "$shared/types/apps";
import { pluginAppsService } from "$shared/services/plugin-apps";

export function registerApps() {
  apps.registerAll([
    // === 系统工具 (system) ===
    {
      id: "file",
      name: "文件管理",
      icon: "/icons/file-manager.svg",
      component: () => import("$apps/file-manager/FileManager.svelte"),
      defaultWidth: 900,
      defaultHeight: 600,
      minWidth: 600,
      minHeight: 400,
      source: "system",
      category: "system" as StartMenuCategory,
    },
    {
      id: "settings",
      name: "设置",
      icon: "/icons/settings.svg",
      component: () => import("$apps/settings/Settings.svelte"),
      defaultWidth: 800,
      defaultHeight: 600,
      minWidth: 600,
      minHeight: 400,
      singleton: true,
      source: "system",
      category: "system" as StartMenuCategory,
    },
    {
      id: "terminal",
      name: "终端",
      icon: "/icons/terminal.svg",
      component: () => import("$apps/terminal/Terminal.svelte"),
      defaultWidth: 700,
      defaultHeight: 450,
      minWidth: 500,
      minHeight: 300,
      source: "system",
      category: "system" as StartMenuCategory,
    },
    // === 模块应用 (module) ===
    {
      id: "download",
      name: "下载管理",
      icon: "/icons/download.svg",
      component: () => import("$apps/download/Download.svelte"),
      defaultWidth: 900,
      defaultHeight: 600,
      minWidth: 600,
      minHeight: 400,
      singleton: true,
      source: "module",
      category: "tools" as StartMenuCategory,
    },
    {
      id: "sync",
      name: "同步",
      icon: "/icons/sync.svg",
      component: () => import("$apps/sync/Sync.svelte"),
      defaultWidth: 900,
      defaultHeight: 650,
      minWidth: 600,
      minHeight: 400,
      singleton: true,
      source: "module",
      category: "tools" as StartMenuCategory,
    },
    {
      id: "photos",
      name: "相册",
      icon: "/icons/photos.svg",
      component: () => import("$apps/photos/Photos.svelte"),
      defaultWidth: 1000,
      defaultHeight: 700,
      minWidth: 700,
      minHeight: 500,
      singleton: true,
      source: "module",
      category: "multimedia" as StartMenuCategory,
    },
    {
      id: "music",
      name: "音乐播放器",
      icon: "/icons/music.svg",
      component: () => import("$apps/music/MusicPlayer.svelte"),
      defaultWidth: 900,
      defaultHeight: 600,
      minWidth: 700,
      minHeight: 450,
      singleton: true,
      source: "module",
      category: "multimedia" as StartMenuCategory,
    },
    {
      id: "video",
      name: "视频播放器",
      icon: "/icons/video-player.svg",
      component: () => import("$apps/video/VideoPlayer.svelte"),
      defaultWidth: 1000,
      defaultHeight: 700,
      minWidth: 640,
      minHeight: 480,
      singleton: false,
      source: "module",
      category: "multimedia" as StartMenuCategory,
    },
    {
      id: "backup",
      name: "备份管理",
      icon: "/icons/backup.svg",
      component: () => import("$apps/backup/Backup.svelte"),
      defaultWidth: 900,
      defaultHeight: 600,
      minWidth: 600,
      minHeight: 400,
      singleton: true,
      source: "module",
      category: "system" as StartMenuCategory,
    },
    {
      id: "samba",
      name: "文件共享",
      icon: "/icons/samba.svg",
      component: () => import("$apps/samba/Samba.svelte"),
      defaultWidth: 850,
      defaultHeight: 600,
      minWidth: 600,
      minHeight: 400,
      singleton: true,
      source: "module",
      category: "network" as StartMenuCategory,
    },
    {
      id: "flatpak",
      name: "Flatpak 应用",
      icon: "/icons/flatpak.svg",
      component: () => import("$apps/flatpak/Flatpak.svelte"),
      defaultWidth: 1000,
      defaultHeight: 700,
      minWidth: 700,
      minHeight: 500,
      singleton: true,
      source: "module",
      category: "system" as StartMenuCategory,
    },

    {
      id: "retrogame",
      name: "复古游戏",
      icon: "/icons/retrogame.svg",
      component: () => import("$apps/retrogame/RetroGame.svelte"),
      defaultWidth: 900,
      defaultHeight: 650,
      minWidth: 700,
      minHeight: 500,
      singleton: true,
      source: "module",
      category: "multimedia" as StartMenuCategory,
    },
    {
      id: "docker",
      name: "Docker 应用",
      icon: "/icons/docker.svg",
      component: () => import("$apps/docker/Docker.svelte"),
      defaultWidth: 950,
      defaultHeight: 650,
      minWidth: 700,
      minHeight: 500,
      singleton: true,
      source: "module",
      category: "system" as StartMenuCategory,
    },
    {
      id: "linuxlab",
      name: "Linux Lab",
      icon: "/icons/linuxlab.svg",
      component: () => import("$apps/linuxlab/LinuxLab.svelte"),
      defaultWidth: 1000,
      defaultHeight: 700,
      minWidth: 750,
      minHeight: 500,
      singleton: true,
      source: "module",
      category: "tools" as StartMenuCategory,
    },

  ]);
}

// 初始化应用 store（从后端加载用户偏好）
export async function initApps(): Promise<void> {
  const { remoteAccessStore } = await import("$desktop/stores/remote-access.svelte");
  // 并行加载应用配置和远程访问设置
  await Promise.all([apps.init(), remoteAccessStore.load()]);

  // 并行加载外部应用和插件应用
  await Promise.all([
    loadExternalApps(),
    loadPluginApps(),
  ]);
}

// 加载外部应用到 appsStore
async function loadExternalApps(): Promise<void> {
  try {
    const dockerApps = await loadDockerApps();

    console.log(`[Apps] Loaded ${dockerApps} Docker apps`);
  } catch (e) {
    console.error("[Apps] Failed to load external apps:", e);
  }
}

// 加载 Docker 已安装应用
async function loadDockerApps(): Promise<number> {
  try {
    const { dockerStoreService } = await import("./docker/store-service");
    const installedApps = await dockerStoreService.getInstalledApps();

    // 清除旧的 Docker 应用
    apps.clearExternalApps("docker");

    // 注册每个已安装的 Docker 应用
    for (const app of installedApps) {
      apps.registerExternalApp({
        id: `docker:${app.name}`,
        name: app.name,
        icon: app.icon ? dockerStoreService.getIconUrl(app.icon) : "mdi:docker",
        type: "docker",
        externalAppId: app.name,
        keywords: `docker ${app.app_id}`,
        // Docker 应用启动回调：启动/打开应用
        launchCallback: async (appId, appName) => {
          const { dockerStoreService: svc } = await import("./docker/store-service");
          try {
            // 如果应用已停止，先启动它
            const currentApps = await svc.getInstalledApps();
            const currentApp = currentApps.find(a => a.name === appName);
            if (currentApp?.status === "stopped") {
              await svc.startApp(appName);
            }
            // 打开 Docker 应用管理器并定位到该应用
            const { apps: appsStore } = await import("$desktop/stores/apps.svelte");
            await appsStore.launch("docker", { tab: "my-apps", highlight: appName });
          } catch (e) {
            console.error(`Failed to launch Docker app ${appName}:`, e);
          }
        },
      });
    }

    return installedApps.length;
  } catch (e) {
    console.error("[Apps] Failed to load Docker apps:", e);
    return 0;
  }
}

// 导出刷新外部应用的函数（供应用安装/卸载后调用）
export { loadExternalApps as refreshExternalApps };

// 加载插件提供的前端应用
async function loadPluginApps(): Promise<void> {
  try {
    const pluginApps = await pluginAppsService.getPluginApps();
    if (pluginApps.length === 0) return;

    for (const { plugin_id, app } of pluginApps) {
      const pluginUrl = pluginAppsService.buildAppUrl(app);

      // 注册为外部应用，点击后在新标签页打开独立 SPA
      apps.registerExternalApp({
        id: app.id,
        name: app.name,
        icon: app.icon,
        type: "docker",
        externalAppId: app.id,
        keywords: `plugin ${plugin_id} ${app.id}`,
        launchCallback: async () => {
          window.open(pluginUrl, `_plugin_${app.id}`);
        },
      });
    }

    console.log(`[Apps] Loaded ${pluginApps.length} plugin apps`);
  } catch (e) {
    console.error("[Apps] Failed to load plugin apps:", e);
  }
}

// 导出刷新插件应用的函数
export { loadPluginApps as refreshPluginApps };
