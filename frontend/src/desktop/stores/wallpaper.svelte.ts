// Wallpaper Store - 壁纸管理
// 支持静态、WebGL动态壁纸、Lottie动画

export type WallpaperType = "static" | "webgl" | "lottie" | "custom";

export interface WallpaperItem {
  id: string;
  name: string;
  type: WallpaperType;
  file: string;
  thumbnail: string;
  // WebGL/Lottie 壁纸特有
  init?: string;
}

export interface WallpaperIndex {
  static: WallpaperItem[];
  webgl: WallpaperItem[];
  lottie: WallpaperItem[];
}

const STORAGE_KEY = "rde_wallpaper";

interface WallpaperSettings {
  currentId: string;
  type: WallpaperType;
  customUrl?: string;
}

// 有效的壁纸类型
const VALID_TYPES: WallpaperType[] = ["static", "webgl", "lottie", "custom"];

// 同步读取初始值（仅在浏览器端有效）
function getStoredWallpaper(): WallpaperSettings | null {
  if (typeof window === "undefined") return null;
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (saved) {
      const data = JSON.parse(saved);
      // 验证类型是否有效（如 video 已废弃），无效则清除并返回 null
      if (data.type && !VALID_TYPES.includes(data.type)) {
        localStorage.removeItem(STORAGE_KEY);
        return null;
      }
      return data;
    }
  } catch {}
  return null;
}

// 在浏览器端同步读取
const initialData = typeof window !== "undefined" ? getStoredWallpaper() : null;

class WallpaperStore {
  // 当前壁纸设置 - SSR 时不设置默认值，避免闪烁
  currentId = $state(initialData?.currentId || "");
  type = $state<WallpaperType>(initialData?.type || "static");
  customUrl = $state<string | null>(initialData?.customUrl || null);

  // 是否已初始化（客户端）
  private _initialized = $state(false);

  // 壁纸索引
  index = $state<WallpaperIndex | null>(null);

  // 是否正在加载
  loading = $state(false);

  // 是否已初始化
  get initialized(): boolean {
    return this._initialized;
  }

  // 当前壁纸URL - 直接根据 currentId 和 type 构建，避免等待 index 加载
  get currentUrl(): string {
    // 未初始化时返回空，避免闪烁
    if (!this._initialized || !this.currentId) {
      return "";
    }

    if (this.type === "custom" && this.customUrl) {
      return this.customUrl;
    }

    // 直接根据 type 和 id 构建 URL
    if (this.type === "static") {
      return `/wallpapers/static/${this.currentId}.jpg`;
    }

    // 动态壁纸类型需要从 index 获取
    if (["webgl", "lottie"].includes(this.type)) {
      const item = this.findWallpaper(this.currentId);
      if (item) {
        return `/wallpapers/${item.file}`;
      }
    }

    return `/wallpapers/static/${this.currentId}.jpg`;
  }

  // 当前壁纸缩略图
  get currentThumbnail(): string {
    if (this.type === "custom" && this.customUrl) {
      return this.customUrl;
    }

    // 从 item 获取缩略图或构建默认 URL
    const item = this.findWallpaper(this.currentId);
    if (item?.thumbnail) {
      return `/wallpapers/${item.thumbnail}`;
    }

    if (this.type === "static") {
      return `/wallpapers/thumbnails/static/${this.currentId}.jpg`;
    }

    return `/wallpapers/thumbnails/static/${this.currentId}.jpg`;
  }

  // 当前壁纸项
  get currentItem(): WallpaperItem | null {
    return this.findWallpaper(this.currentId);
  }

  // 查找壁纸
  findWallpaper(id: string): WallpaperItem | null {
    if (!this.index) return null;

    for (const type of ["static", "webgl", "lottie"] as const) {
      const item = this.index[type]?.find((w) => w.id === id);
      if (item) return item;
    }

    return null;
  }

  // 检测 WebGL 支持
  isWebGLSupported(): boolean {
    if (typeof window === "undefined") return false;
    try {
      const canvas = document.createElement("canvas");
      return !!(
        window.WebGLRenderingContext &&
        (canvas.getContext("webgl") || canvas.getContext("experimental-webgl"))
      );
    } catch (e) {
      return false;
    }
  }

  // 获取随机静态壁纸（用于回退）
  getRandomStaticWallpaper(): WallpaperItem | null {
    if (!this.index?.static?.length) return null;
    const idx = Math.floor(Math.random() * this.index.static.length);
    return this.index.static[idx];
  }

  // 加载壁纸索引
  async loadIndex(): Promise<void> {
    if (this.index) return;

    this.loading = true;
    try {
      const res = await fetch("/wallpapers/index.json");
      if (res.ok) {
        this.index = await res.json();
      }
    } catch (e) {
      console.error("加载壁纸索引失败:", e);
    } finally {
      this.loading = false;
    }
  }

  // 设置壁纸
  setWallpaper(id: string, type: WallpaperType): void {
    this.currentId = id;
    this.type = type;
    this.customUrl = null;
    this.save();
  }

  // 设置自定义壁纸
  setCustomWallpaper(url: string): void {
    this.type = "custom";
    this.customUrl = url;
    this.currentId = "custom";
    this.save();
  }

  // 保存到 localStorage
  private save(): void {
    if (typeof localStorage === "undefined") return;

    try {
      const data: WallpaperSettings = {
        currentId: this.currentId,
        type: this.type,
        customUrl: this.customUrl || undefined,
      };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
    } catch (e) {
      console.error("保存壁纸设置失败:", e);
    }
  }

  // 初始化 - 从 localStorage 读取设置并加载索引
  async init(): Promise<void> {
    // 再次尝试从 localStorage 读取（确保客户端 hydrate 后正确读取）
    const stored = getStoredWallpaper();
    if (stored) {
      this.currentId = stored.currentId;
      this.type = stored.type;
      this.customUrl = stored.customUrl || null;
    } else {
      // 没有存储的设置，使用默认值（海底世界动画）
      this.currentId = "underwater";
      this.type = "lottie";
    }

    this._initialized = true;
    await this.loadIndex();
  }
}

export const wallpaper = new WallpaperStore();
