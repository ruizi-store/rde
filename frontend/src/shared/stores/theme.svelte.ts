// 主题状态管理
// 处理深色/浅色模式切换和强调色

import { loadUserString, saveUserString } from "$shared/utils/user-storage";

type ThemeMode = "light" | "dark" | "auto";

// 预设强调色列表
const ACCENT_COLORS = [
  { id: "blue", value: "#4a90d9", hover: "#357abd" },
  { id: "green", value: "#52c41a", hover: "#3da50e" },
  { id: "gold", value: "#faad14", hover: "#d99312" },
  { id: "red", value: "#ff4d4f", hover: "#e63e40" },
  { id: "purple", value: "#722ed1", hover: "#5b25a7" },
  { id: "pink", value: "#eb2f96", hover: "#c41d7f" },
];

class ThemeStore {
  mode = $state<ThemeMode>("auto");
  isDark = $state(false);
  accentColor = $state("#4a90d9");

  private mediaQuery: MediaQueryList | null = null;

  constructor() {
    // 初始化时从用户专属 localStorage 读取
    if (typeof window !== "undefined") {
      const saved = loadUserString("theme") as ThemeMode | null;
      if (saved) {
        this.mode = saved;
      }

      const savedAccent = loadUserString("accentColor");
      if (savedAccent) {
        this.accentColor = savedAccent;
      }

      // 监听系统主题变化
      this.mediaQuery = window.matchMedia("(prefers-color-scheme: dark)");
      this.mediaQuery.addEventListener("change", this.handleSystemChange);

      this.updateTheme();
    }
  }

  private handleSystemChange = () => {
    if (this.mode === "auto") {
      this.updateTheme();
    }
  };

  private updateTheme() {
    if (this.mode === "auto") {
      this.isDark = this.mediaQuery?.matches ?? false;
    } else {
      this.isDark = this.mode === "dark";
    }

    // 更新 document 类名和 data-theme 属性
    if (typeof document !== "undefined") {
      document.documentElement.classList.toggle("dark", this.isDark);
      document.documentElement.classList.toggle("light", !this.isDark);
      // 设置 data-theme 属性，用于 CSS 变量选择器
      document.documentElement.setAttribute("data-theme", this.isDark ? "dark" : "light");

      // 应用强调色到 CSS 变量
      this.applyAccentColor();
    }
  }

  private applyAccentColor() {
    if (typeof document === "undefined") return;

    const accent = ACCENT_COLORS.find((c) => c.value === this.accentColor);
    const hoverColor = accent?.hover || this.adjustColor(this.accentColor, -20);
    const lightColor = this.hexToRgba(this.accentColor, 0.15);

    document.documentElement.style.setProperty("--color-primary", this.accentColor);
    document.documentElement.style.setProperty("--color-primary-hover", hoverColor);
    document.documentElement.style.setProperty("--color-primary-light", lightColor);
    document.documentElement.style.setProperty(
      "--bg-active",
      this.hexToRgba(this.accentColor, 0.2),
    );
  }

  // 调整颜色亮度
  private adjustColor(hex: string, amount: number): string {
    const num = parseInt(hex.replace("#", ""), 16);
    const r = Math.max(0, Math.min(255, (num >> 16) + amount));
    const g = Math.max(0, Math.min(255, ((num >> 8) & 0x00ff) + amount));
    const b = Math.max(0, Math.min(255, (num & 0x0000ff) + amount));
    return `#${((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1)}`;
  }

  // HEX 转 RGBA
  private hexToRgba(hex: string, alpha: number): string {
    const num = parseInt(hex.replace("#", ""), 16);
    const r = (num >> 16) & 255;
    const g = (num >> 8) & 255;
    const b = num & 255;
    return `rgba(${r}, ${g}, ${b}, ${alpha})`;
  }

  set(mode: ThemeMode) {
    this.mode = mode;
    saveUserString("theme", mode);
    this.updateTheme();
  }

  setAccent(color: string) {
    this.accentColor = color;
    saveUserString("accentColor", color);
    this.applyAccentColor();
  }

  toggle() {
    if (this.isDark) {
      this.set("light");
    } else {
      this.set("dark");
    }
  }

  // 获取当前主题的 CSS 变量
  get cssVariables() {
    return this.isDark ? darkTheme : lightTheme;
  }

  // 为当前用户重新加载主题设置（用户切换时调用）
  reloadForUser(): void {
    const saved = loadUserString("theme") as ThemeMode | null;
    this.mode = saved || "auto";

    const savedAccent = loadUserString("accentColor");
    this.accentColor = savedAccent || "#4a90d9";

    this.updateTheme();
  }
}

// 浅色主题变量
const lightTheme = {
  "--bg-window": "#ffffff",
  "--bg-window-header": "#f8f9fa",
  "--bg-sidebar": "#f5f5f5",
  "--bg-hover": "rgba(0, 0, 0, 0.05)",
  "--bg-active": "rgba(0, 102, 204, 0.1)",
  "--bg-menu": "#ffffff",
  "--bg-input": "#ffffff",
  "--bg-secondary": "#e9ecef",
  "--text-primary": "#333333",
  "--text-secondary": "#666666",
  "--text-muted": "#adb5bd",
  "--border-color": "#e0e0e0",
  "--color-primary": "#4a90d9",
  "--color-primary-dark": "#357abd",
  "--shadow": "0 2px 8px rgba(0, 0, 0, 0.1)",
};

// 深色主题变量
const darkTheme = {
  "--bg-window": "#1e1e2e",
  "--bg-window-header": "#313244",
  "--bg-sidebar": "#181825",
  "--bg-hover": "rgba(255, 255, 255, 0.08)",
  "--bg-active": "rgba(137, 180, 250, 0.15)",
  "--bg-menu": "#313244",
  "--bg-input": "#45475a",
  "--bg-secondary": "#45475a",
  "--text-primary": "#cdd6f4",
  "--text-secondary": "#a6adc8",
  "--text-muted": "#6c7086",
  "--border-color": "#45475a",
  "--color-primary": "#89b4fa",
  "--color-primary-dark": "#74a8f9",
  "--shadow": "0 2px 8px rgba(0, 0, 0, 0.3)",
};

export const theme = new ThemeStore();
