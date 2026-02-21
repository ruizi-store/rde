// Desktop Store - Svelte 5 Runes
// 桌面状态管理
// 参考文档: docs/frontend/APP_ICON_MANAGEMENT.md

import { generateUUID } from "$shared/utils/uuid";
import { remoteAccessStore } from "./remote-access.svelte";
import { loadUserData, saveUserData } from "$shared/utils/user-storage";

const STORAGE_KEY = "rde_desktop";

// 网格配置
const GRID_CELL_W = 88; // 80px icon + 8px gap
const GRID_CELL_H = 98; // 90px icon + 8px gap
const GRID_PADDING = 16;
const TASKBAR_HEIGHT = 48;

export type SortMode = "name" | "category" | "type";

export interface DesktopIcon {
  id: string;
  name: string;
  icon: string;
  appId: string;
  x: number;
  y: number;
}

interface DesktopData {
  wallpaper: string;
  icons: DesktopIcon[];
  snapToGrid?: boolean;
}

class DesktopStore {
  // 壁纸
  wallpaper = $state("/wallpapers/default.svg");

  // 桌面图标
  icons = $state<DesktopIcon[]>([]);

  // 选中的图标
  selectedIconId = $state<string | null>(null);

  // 网格选项
  snapToGrid = $state(true);

  // 是否已初始化
  private _initialized = false;

  // 计算当前屏幕可用的网格列/行数
  getGridDimensions(): { cols: number; rows: number } {
    if (typeof window === "undefined") return { cols: 20, rows: 10 };
    const w = window.innerWidth - GRID_PADDING * 2;
    const h = window.innerHeight - TASKBAR_HEIGHT - GRID_PADDING * 2;
    return {
      cols: Math.max(1, Math.floor(w / GRID_CELL_W)),
      rows: Math.max(1, Math.floor(h / GRID_CELL_H)),
    };
  }

  // 设置壁纸
  setWallpaper(url: string): void {
    this.wallpaper = url;
    this.save();
  }

  // 添加图标
  addIcon(icon: Omit<DesktopIcon, "id">): void {
    // 检查是否已存在
    if (this.icons.some((i) => i.appId === icon.appId)) {
      return;
    }

    const pos = this.findNextPosition();

    this.icons = [
      ...this.icons,
      {
        ...icon,
        ...pos,
        id: generateUUID(),
      },
    ];
    this.save();
  }

  // 检查应用是否在桌面上
  hasIcon(appId: string): boolean {
    return this.icons.some((i) => i.appId === appId);
  }

  // 移除图标
  removeIcon(id: string): void {
    this.icons = this.icons.filter((i) => i.id !== id);
    this.save();
  }

  // 按 appId 移除图标
  removeIconByAppId(appId: string): void {
    this.icons = this.icons.filter((i) => i.appId !== appId);
    this.save();
  }

  // 移动图标到网格坐标
  moveIcon(id: string, x: number, y: number): void {
    // 吸附到网格
    x = Math.max(0, Math.round(x));
    y = Math.max(0, Math.round(y));

    // 检测碰撞：如果目标位置有其他图标则交换
    const targetIcon = this.icons.find((i) => i.id !== id && i.x === x && i.y === y);
    const movingIcon = this.icons.find((i) => i.id === id);
    if (!movingIcon) return;

    if (targetIcon) {
      // 交换位置
      targetIcon.x = movingIcon.x;
      targetIcon.y = movingIcon.y;
    }

    movingIcon.x = x;
    movingIcon.y = y;
    // 触发响应式更新
    this.icons = [...this.icons];
    this.save();
  }

  // 选中图标
  selectIcon(id: string | null): void {
    this.selectedIconId = id;
  }

  // ========== 排列功能 ==========

  // 自动排列所有图标（按列优先，从上到下，从左到右）
  arrangeIcons(): void {
    const { rows } = this.getGridDimensions();
    const maxRows = Math.max(rows, 6);

    this.icons = this.icons.map((icon, index) => ({
      ...icon,
      x: Math.floor(index / maxRows),
      y: index % maxRows,
    }));
    this.save();
  }

  // 按名称排序并排列
  sortByName(): void {
    this.icons = [...this.icons].sort((a, b) => a.name.localeCompare(b.name, "zh-CN"));
    this.arrangeIcons();
  }

  // 按类型排序（系统→套件→其他）
  sortByType(): void {
    const typeOrder = (appId: string): number => {
      if (appId === "file") return 0;
      if (appId === "settings") return 1;
      if (appId === "terminal") return 2;
      if (appId === "storage-manager") return 3;
      if (appId.startsWith("native-")) return 20;
      return 50;
    };
    this.icons = [...this.icons].sort((a, b) => typeOrder(a.appId) - typeOrder(b.appId));
    this.arrangeIcons();
  }

  // 设置网格吸附
  setSnapToGrid(enabled: boolean): void {
    this.snapToGrid = enabled;
    this.save();
  }

  // 默认图标
  private getDefaultIcons(): DesktopIcon[] {
    return [
      // 第一列：核心工具
      { id: "1", name: "文件管理", icon: "/icons/file-manager.svg", appId: "file", x: 0, y: 0, },
      { id: "22", name: "音乐播放器", icon: "/icons/music.svg", appId: "music", x: 0, y: 1, },
      { id: "23", name: "视频播放器", icon: "/icons/video-player.svg", appId: "video", x: 0, y: 2, },
      { id: "2", name: "设置", icon: "/icons/settings.svg", appId: "settings", x: 0, y: 3, },
      // 第二列：应用管理
      { id: "10", name: "Docker 应用", icon: "/icons/docker.svg", appId: "docker", x: 1, y: 0, },
      { id: "24", name: "Flatpak 应用", icon: "/icons/flatpak.svg", appId: "flatpak", x: 1, y: 1, },
      { id: "18", name: "文件共享", icon: "/icons/samba.svg", appId: "samba", x: 1, y: 2, },
      { id: "19", name: "同步", icon: "/icons/sync.svg", appId: "sync", x: 1, y: 3, },
      // 第三列：工具与娱乐
      { id: "12", name: "下载管理", icon: "/icons/download.svg", appId: "download", x: 2, y: 0, },
      { id: "20", name: "复古游戏", icon: "/icons/retrogame.svg", appId: "retrogame", x: 2, y: 1, },
      { id: "13", name: "备份管理", icon: "/icons/backup.svg", appId: "backup", x: 2, y: 2, },
      { id: "21", name: "相册", icon: "/icons/photos.svg", appId: "photos", x: 2, y: 3, },
    ];
  }

  // 找到下一个可用位置
  findNextPosition(): { x: number; y: number } {
    const existingPositions = new Set(this.icons.map((icon) => `${icon.x},${icon.y}`));
    const { rows } = this.getGridDimensions();
    const maxRows = Math.max(rows, 6);

    // 按列优先查找（Y 方向先增加）
    for (let x = 0; x < 30; x++) {
      for (let y = 0; y < maxRows; y++) {
        if (!existingPositions.has(`${x},${y}`)) {
          return { x, y };
        }
      }
    }

    return { x: 0, y: 0 };
  }

  // 初始化默认图标（仅在没有保存数据时使用）
  initDefaultIcons(): void {
    if (this._initialized) return;
    this._initialized = true;

    // 尝试从 localStorage 恢复
    if (this.load()) {
      return;
    }

    // 没有保存的数据，使用默认图标
    this.icons = this.getDefaultIcons();
  }

  // 从 localStorage 加载
  private load(): boolean {
    if (typeof localStorage === "undefined") return false;

    try {
      const data = loadUserData<DesktopData>(STORAGE_KEY);
      if (data) {
        if (data.wallpaper) {
          this.wallpaper = data.wallpaper;
        }
        if (data.icons && Array.isArray(data.icons)) {
          // 迁移旧的应用 ID
          this.icons = data.icons.map((icon) => {
            if (icon.appId === "syncthing") {
              return { ...icon, appId: "sync", icon: "/icons/sync.svg" };
            }
            return icon;
          });
          // 自动补充新增的桌面图标
          const newIcons: { appId: string; name: string; icon: string }[] = [
            { appId: "flatpak", name: "Flatpak 应用", icon: "/icons/flatpak.svg" },
          ];
          for (const ni of newIcons) {
            if (!this.icons.some((i) => i.appId === ni.appId)) {
              const pos = this.findNextPosition();
              this.icons.push({ id: `auto_${ni.appId}`, ...ni, ...pos });
            }
          }
        }
        if (data.snapToGrid !== undefined) {
          this.snapToGrid = data.snapToGrid;
        }
        return true;
      }
    } catch (e) {
      console.error("加载桌面数据失败:", e);
    }
    return false;
  }

  // 保存到 localStorage
  private save(): void {
    if (typeof localStorage === "undefined") return;

    try {
      const data: DesktopData = {
        wallpaper: this.wallpaper,
        icons: this.icons,
        snapToGrid: this.snapToGrid,
      };
      saveUserData(STORAGE_KEY, data);
    } catch (e) {
      console.error("保存桌面数据失败:", e);
    }
  }

  // 为当前用户重新加载数据（用户切换时调用）
  reloadForUser(): void {
    this._initialized = false;
    this.initDefaultIcons();
  }

  // 重置为默认
  reset(): void {
    this.wallpaper = "/wallpapers/default.svg";
    this.snapToGrid = true;
    this.icons = this.getDefaultIcons();
    this.save();
  }
}

export const desktop = new DesktopStore();
